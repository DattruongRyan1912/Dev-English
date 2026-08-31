package usageguard

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

var ErrBudgetExceeded = errors.New("usage budget exceeded")

// Guard coordinates a quota reservation across concurrent requests. Postgres
// stores use an atomic reservation row; the in-process map is a safe fallback
// for older adapters and deterministic tests.
type Guard struct {
	mu     sync.Mutex
	local  map[string]float64
	serial atomic.Uint64
}

func New() *Guard {
	return &Guard{local: make(map[string]float64)}
}

type Reservation struct {
	guard    *Guard
	durable  store.UsageReservationStore
	id       string
	localKey string
	userID   string
	amount   float64
	active   bool
	stateMu  sync.Mutex
}

func (g *Guard) Reserve(ctx context.Context, repository store.Repository, request store.UsageReservationRequest) (*Reservation, error) {
	if repository == nil {
		return nil, errors.New("usage repository is required")
	}
	if request.UserID == "" {
		request.UserID = store.UserID(ctx)
	}
	if request.UserID == "" || request.MonthStart.IsZero() ||
		request.Amount <= 0 || request.Limit <= 0 {
		return nil, errors.New("invalid usage reservation")
	}
	if request.Metric != store.UsageMetricTokens && request.Metric != store.UsageMetricCost {
		return nil, errors.New("invalid usage reservation metric")
	}
	if request.ID == "" {
		request.ID = fmt.Sprintf("usage-reservation-%d-%d", time.Now().UTC().UnixNano(), g.serial.Add(1))
	}
	request.MonthStart = monthStart(request.MonthStart)
	if request.ExpiresAt.IsZero() {
		request.ExpiresAt = time.Now().UTC().Add(5 * time.Minute)
	}

	if durable, ok := repository.(store.UsageReservationStore); ok {
		allowed, err := durable.ReserveUsage(store.WithUser(ctx, request.UserID), request)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, ErrBudgetExceeded
		}
		return &Reservation{
			durable: durable,
			id:      request.ID,
			userID:  request.UserID,
			amount:  request.Amount,
			active:  true,
		}, nil
	}

	key := reservationKey(request)
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.local == nil {
		g.local = make(map[string]float64)
	}
	records, err := repository.Usage(store.WithUser(ctx, request.UserID), request.MonthStart)
	if err != nil {
		return nil, err
	}
	used := 0.0
	for _, record := range records {
		if request.Feature != "" && record.Feature != request.Feature {
			continue
		}
		used += metricValue(request.Metric, record)
	}
	if used+g.local[key]+request.Amount > request.Limit {
		return nil, ErrBudgetExceeded
	}
	g.local[key] += request.Amount
	return &Reservation{
		guard:    g,
		id:       request.ID,
		localKey: key,
		userID:   request.UserID,
		amount:   request.Amount,
		active:   true,
	}, nil
}

func (r *Reservation) Commit(ctx context.Context) error {
	return r.finish(ctx, true)
}

func (r *Reservation) Release(ctx context.Context) error {
	return r.finish(ctx, false)
}

func (r *Reservation) finish(ctx context.Context, committed bool) error {
	if r == nil {
		return nil
	}
	r.stateMu.Lock()
	if !r.active {
		r.stateMu.Unlock()
		return nil
	}
	if r.durable != nil {
		finishCtx := store.WithUser(context.Background(), r.userID)
		if err := r.durable.CompleteUsageReservation(finishCtx, r.id, committed); err != nil {
			r.stateMu.Unlock()
			return err
		}
		r.active = false
		r.stateMu.Unlock()
		return nil
	}
	r.active = false
	r.stateMu.Unlock()
	if r.guard != nil {
		r.guard.mu.Lock()
		r.guard.local[r.localKey] -= r.amount
		if r.guard.local[r.localKey] <= 0 {
			delete(r.guard.local, r.localKey)
		}
		r.guard.mu.Unlock()
	}
	return nil
}

func reservationKey(request store.UsageReservationRequest) string {
	return fmt.Sprintf("%s|%s|%s|%s", request.UserID, request.MonthStart.UTC().Format("2006-01"), request.Feature, request.Metric)
}

func metricValue(metric store.UsageMetric, record domain.UsageRecord) float64 {
	if metric == store.UsageMetricTokens {
		return float64(record.InputTokens + record.OutputTokens)
	}
	return record.EstimatedCost
}

func monthStart(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.UTC)
}
