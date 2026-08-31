package usageguard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

// usageOnlyRepository deliberately embeds the broad legacy repository
// interface so the tests can exercise Guard's compatibility fallback without
// duplicating unrelated persistence methods.
type usageOnlyRepository struct {
	store.Repository
	records  []domain.UsageRecord
	err      error
	lastFrom time.Time
}

func (r *usageOnlyRepository) Usage(_ context.Context, from time.Time) ([]domain.UsageRecord, error) {
	r.lastFrom = from
	if r.err != nil {
		return nil, r.err
	}
	return r.records, nil
}

type reservationRepository struct {
	store.Repository
	allowed       bool
	reserveErr    error
	completeErr   error
	reserveInputs []store.UsageReservationRequest
	completed     []struct {
		id        string
		committed bool
	}
}

func (r *reservationRepository) ReserveUsage(_ context.Context, request store.UsageReservationRequest) (bool, error) {
	r.reserveInputs = append(r.reserveInputs, request)
	if r.reserveErr != nil {
		return false, r.reserveErr
	}
	return r.allowed, nil
}

func (r *reservationRepository) CompleteUsageReservation(_ context.Context, id string, committed bool) error {
	if r.completeErr != nil {
		return r.completeErr
	}
	r.completed = append(r.completed, struct {
		id        string
		committed bool
	}{id: id, committed: committed})
	return nil
}

func TestReservationPreventsConcurrentOvercommitAndReleasesOnFailure(t *testing.T) {
	repository := store.NewSeeded(time.Now().UTC())
	guard := New()
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	request := func(id string, amount float64) store.UsageReservationRequest {
		return store.UsageReservationRequest{
			ID: id, UserID: "user-1", MonthStart: month, Feature: "assistant",
			Metric: store.UsageMetricTokens, Amount: amount, Limit: 100,
		}
	}

	first, err := guard.Reserve(context.Background(), repository, request("first", 80))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := guard.Reserve(context.Background(), repository, request("second", 30)); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("second reservation error = %v, want budget exceeded", err)
	}
	if err := first.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	second, err := guard.Reserve(context.Background(), repository, request("second", 30))
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCostReservationCountsPersistedUsage(t *testing.T) {
	repository := store.NewSeeded(time.Now().UTC())
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if err := repository.SaveUsage(store.WithUser(context.Background(), "user-1"), domain.UsageRecord{
		Feature: "assistant", EstimatedCost: 75, CreatedAt: month.Add(24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	guard := New()
	if _, err := guard.Reserve(context.Background(), repository, store.UsageReservationRequest{
		ID: "too-large", UserID: "user-1", MonthStart: month, Feature: "assistant",
		Metric: store.UsageMetricCost, Amount: 30, Limit: 100,
	}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("cost reservation error = %v, want budget exceeded", err)
	}
}

func TestFeatureScopedReservationDoesNotConsumeAnotherFeature(t *testing.T) {
	repository := store.NewSeeded(time.Now().UTC())
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if err := repository.SaveUsage(store.WithUser(context.Background(), "user-1"), domain.UsageRecord{
		Feature: "stt", EstimatedCost: 90, CreatedAt: month.Add(24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	guard := New()
	assistantReservation, err := guard.Reserve(context.Background(), repository, store.UsageReservationRequest{
		ID: "assistant-feature", UserID: "user-1", MonthStart: month, Feature: "assistant",
		Metric: store.UsageMetricCost, Amount: 20, Limit: 100,
	})
	if err != nil {
		t.Fatalf("assistant feature should have an independent budget: %v", err)
	}
	if err := assistantReservation.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := guard.Reserve(context.Background(), repository, store.UsageReservationRequest{
		ID: "stt-feature", UserID: "user-1", MonthStart: month, Feature: "stt",
		Metric: store.UsageMetricCost, Amount: 20, Limit: 100,
	}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("stt feature error = %v, want budget exceeded", err)
	}
}

func TestLocalFallbackNormalizesMonthAndCoversTokenAndCostMetrics(t *testing.T) {
	month := time.Date(2026, 8, 19, 14, 30, 0, 0, time.FixedZone("ICT", 7*60*60))
	repository := &usageOnlyRepository{records: []domain.UsageRecord{
		{Feature: "assistant", InputTokens: 2, OutputTokens: 3},
		{Feature: "other", InputTokens: 999, OutputTokens: 999},
	}}
	guard := &Guard{}
	request := store.UsageReservationRequest{
		UserID: "context-user", MonthStart: month, Feature: "assistant",
		Metric: store.UsageMetricTokens, Amount: 4, Limit: 10,
	}

	first, err := guard.Reserve(context.Background(), repository, request)
	if err != nil {
		t.Fatal(err)
	}
	if want := monthStart(month); !repository.lastFrom.Equal(want) {
		t.Fatalf("usage query month = %v, want %v", repository.lastFrom, want)
	}
	if got := guard.local[reservationKey(store.UsageReservationRequest{
		UserID: "context-user", MonthStart: month, Feature: "assistant",
		Metric: store.UsageMetricTokens,
	})]; got != 4 {
		t.Fatalf("pending local amount = %v, want 4", got)
	}
	if _, err := guard.Reserve(context.Background(), repository, store.UsageReservationRequest{
		UserID: "context-user", MonthStart: month, Feature: "assistant",
		Metric: store.UsageMetricTokens, Amount: 2, Limit: 10,
	}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("over-limit local reservation error = %v, want budget exceeded", err)
	}
	if err := first.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	second, err := guard.Reserve(context.Background(), repository, store.UsageReservationRequest{
		UserID: "context-user", MonthStart: month, Feature: "assistant",
		Metric: store.UsageMetricTokens, Amount: 5, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}

	costRepository := &usageOnlyRepository{records: []domain.UsageRecord{
		{Feature: "assistant", EstimatedCost: 2},
		{Feature: "other", EstimatedCost: 99},
	}}
	costReservation, err := guard.Reserve(context.Background(), costRepository, store.UsageReservationRequest{
		UserID: "context-user", MonthStart: month, Feature: "assistant",
		Metric: store.UsageMetricCost, Amount: 7, Limit: 10,
	})
	if err != nil {
		t.Fatalf("cost reservation = %v", err)
	}
	if err := costReservation.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestReserveRejectsInvalidInputAndUsageReadErrors(t *testing.T) {
	guard := New()
	validMonth := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	valid := store.UsageReservationRequest{
		UserID: "user-1", MonthStart: validMonth, Metric: store.UsageMetricTokens,
		Amount: 1, Limit: 10,
	}
	cases := []struct {
		name string
		make func() store.UsageReservationRequest
	}{
		{name: "missing month", make: func() store.UsageReservationRequest {
			request := valid
			request.MonthStart = time.Time{}
			return request
		}},
		{name: "missing amount", make: func() store.UsageReservationRequest {
			request := valid
			request.Amount = 0
			return request
		}},
		{name: "missing limit", make: func() store.UsageReservationRequest {
			request := valid
			request.Limit = 0
			return request
		}},
		{name: "invalid metric", make: func() store.UsageReservationRequest {
			request := valid
			request.Metric = "requests"
			return request
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := guard.Reserve(context.Background(), &usageOnlyRepository{}, testCase.make()); err == nil {
				t.Fatal("Reserve returned nil error for invalid input")
			}
		})
	}
	if _, err := guard.Reserve(context.Background(), nil, valid); err == nil {
		t.Fatal("Reserve returned nil error for a missing repository")
	}

	readErr := errors.New("usage read failed")
	if _, err := guard.Reserve(context.Background(), &usageOnlyRepository{err: readErr}, valid); !errors.Is(err, readErr) {
		t.Fatalf("usage read error = %v, want %v", err, readErr)
	}
}

func TestDurableReservationLifecycleAndErrors(t *testing.T) {
	month := time.Date(2026, 8, 19, 14, 30, 0, 0, time.FixedZone("ICT", 7*60*60))
	repository := &reservationRepository{allowed: true}
	guard := New()
	request := store.UsageReservationRequest{
		ID: "durable-1", UserID: "user-1", MonthStart: month, Feature: "assistant",
		Metric: store.UsageMetricCost, Amount: 2, Limit: 10,
		ExpiresAt: month.Add(time.Hour),
	}
	reservation, err := guard.Reserve(context.Background(), repository, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(repository.reserveInputs) != 1 {
		t.Fatalf("ReserveUsage calls = %d, want 1", len(repository.reserveInputs))
	}
	if got, want := repository.reserveInputs[0].MonthStart, monthStart(month); !got.Equal(want) {
		t.Fatalf("durable month = %v, want %v", got, want)
	}
	if err := reservation.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := reservation.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repository.completed) != 1 || !repository.completed[0].committed {
		t.Fatalf("completed reservations = %#v, want one committed reservation", repository.completed)
	}

	denied := &reservationRepository{}
	if _, err := guard.Reserve(context.Background(), denied, request); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("denied reservation error = %v, want budget exceeded", err)
	}
	reserveErr := errors.New("reservation write failed")
	failedReserve := &reservationRepository{reserveErr: reserveErr}
	if _, err := guard.Reserve(context.Background(), failedReserve, request); !errors.Is(err, reserveErr) {
		t.Fatalf("reserve error = %v, want %v", err, reserveErr)
	}

	completeErr := errors.New("completion write failed")
	retryRepository := &reservationRepository{allowed: true, completeErr: completeErr}
	retryReservation, err := guard.Reserve(context.Background(), retryRepository, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := retryReservation.Release(context.Background()); !errors.Is(err, completeErr) {
		t.Fatalf("completion error = %v, want %v", err, completeErr)
	}
	retryRepository.completeErr = nil
	if err := retryReservation.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(retryRepository.completed) != 1 || retryRepository.completed[0].committed {
		t.Fatalf("retry completion = %#v, want one released reservation", retryRepository.completed)
	}

	var nilReservation *Reservation
	if err := nilReservation.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}
