package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestPostgresRepositoryIntegration is opt-in so the normal unit-test suite
// never needs a database. CI and local operators enable it with a disposable
// PostgreSQL database through DEVENGLISH_TEST_DATABASE_URL.
func TestPostgresRepositoryIntegration(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("DEVENGLISH_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set DEVENGLISH_TEST_DATABASE_URL to run PostgreSQL integration coverage")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}
	repository := &PostgresStore{Pool: pool}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userA := "postgres-it-a-" + suffix
	userB := "postgres-it-b-" + suffix
	ctxA := WithUser(ctx, userA)
	ctxB := WithUser(ctx, userB)
	cleanup := func(ctx context.Context, userID string) {
		if err := repository.DeleteUserData(WithUser(ctx, userID)); err != nil && !errors.Is(err, ErrNotFound) {
			t.Errorf("cleanup user %s: %v", userID, err)
		}
	}
	defer func() {
		cleanup(context.Background(), userA)
		cleanup(context.Background(), userB)
		pool.Close()
	}()

	for _, userID := range []string{userA, userB} {
		if err := repository.EnsureUser(WithUser(ctx, userID), domain.User{ID: userID, DisplayName: userID, CEFR: "B1", CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatalf("ensure user %s: %v", userID, err)
		}
	}

	createdAt := time.Now().UTC()
	mission := domain.Mission{
		ID:               "postgres-it-mission-" + suffix,
		Title:            "PostgreSQL integration mission",
		Mode:             domain.ModeWriting,
		Skill:            "technical_writing",
		SkillLabel:       "Technical Writing",
		Level:            "B1",
		Context:          "A disposable repository test",
		Prompt:           "Explain the persistence boundary.",
		TargetVocabulary: []string{"transaction"},
		ExpectedPoints:   []string{"observed behavior", "next step"},
		EstimatedMinutes: 5,
		Status:           "available",
		CreatedAt:        createdAt,
	}
	if err := repository.SaveMission(ctxA, mission); err != nil {
		t.Fatalf("save mission: %v", err)
	}
	if _, err := repository.Mission(ctxB, mission.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected per-user mission isolation, got %v", err)
	}

	completedAt := createdAt.Add(time.Minute)
	if err := repository.SaveWritingOutcome(ctxA, WritingOutcome{
		Attempt: domain.MissionAttempt{
			ID:          "postgres-it-attempt-" + suffix,
			MissionID:   mission.ID,
			Answer:      "The transaction preserves the learning state.",
			Score:       91,
			SubmittedAt: completedAt,
		},
		Evaluation: domain.Evaluation{
			Score:    91,
			Summary:  "The answer explains the persistence boundary.",
			Provider: "integration-test",
		},
		Mistakes: []domain.Mistake{{
			ID:         "postgres-it-mistake-" + suffix,
			Type:       "article",
			Original:   "write transaction",
			Corrected:  "write a transaction",
			Context:    "integration test",
			Severity:   1,
			Frequency:  1,
			LastSeen:   completedAt,
			NextReview: completedAt.Add(24 * time.Hour),
			Mastery:    0.2,
		}},
		MissionID:   mission.ID,
		CompletedAt: completedAt,
		Skill:       "technical_writing",
		SkillDelta:  3,
	}); err != nil {
		t.Fatalf("save atomic writing outcome: %v", err)
	}

	persistedMission, err := repository.Mission(ctxA, mission.ID)
	if err != nil {
		t.Fatalf("read persisted mission: %v", err)
	}
	if persistedMission.Status != "completed" || persistedMission.CompletedAt == nil {
		t.Fatalf("mission transition was not persisted atomically: %+v", persistedMission)
	}
	mistakes, err := repository.AllMistakes(ctxA)
	if err != nil {
		t.Fatalf("read persisted mistakes: %v", err)
	}
	if len(mistakes) != 1 || mistakes[0].ID == "" {
		t.Fatalf("expected one persisted mistake, got %+v", mistakes)
	}
	state, err := repository.LearningState(ctxA)
	if err != nil {
		t.Fatalf("read persisted learning state: %v", err)
	}
	var technicalWritingScore float64
	for _, skill := range state.Skills {
		if skill.Skill == "technical_writing" {
			technicalWritingScore = skill.Score
		}
	}
	if technicalWritingScore != 51 {
		t.Fatalf("expected persisted skill update to 51, got %v", technicalWritingScore)
	}
	if _, err := repository.Mission(ctxB, mission.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other user can read persisted mission: %v", err)
	}

	reservationMonth := time.Date(createdAt.Year(), createdAt.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstReservation := UsageReservationRequest{
		ID:         "postgres-it-reservation-first-" + suffix,
		UserID:     userA,
		MonthStart: reservationMonth,
		Feature:    "assistant",
		Metric:     UsageMetricTokens,
		Amount:     80,
		Limit:      100,
		ExpiresAt:  createdAt.Add(5 * time.Minute),
	}
	allowed, err := repository.ReserveUsage(ctxA, firstReservation)
	if err != nil || !allowed {
		t.Fatalf("reserve first usage window: allowed=%v err=%v", allowed, err)
	}
	// Retrying the same request is idempotent and must not consume another 80
	// tokens. This is the durable equivalent of a client retry after a timeout.
	if allowed, err := repository.ReserveUsage(ctxA, firstReservation); err != nil || !allowed {
		t.Fatalf("replay first usage reservation: allowed=%v err=%v", allowed, err)
	}
	secondReservation := firstReservation
	secondReservation.ID += "-second"
	secondReservation.Amount = 30
	if allowed, err := repository.ReserveUsage(ctxA, secondReservation); err != nil {
		t.Fatalf("reserve over-budget usage: %v", err)
	} else if allowed {
		t.Fatal("expected active reservation to consume the remaining budget")
	}
	if err := repository.CompleteUsageReservation(ctxA, firstReservation.ID, false); err != nil {
		t.Fatalf("release first usage reservation: %v", err)
	}
	if allowed, err := repository.ReserveUsage(ctxA, secondReservation); err != nil || !allowed {
		t.Fatalf("reserve after release: allowed=%v err=%v", allowed, err)
	}
	if err := repository.CompleteUsageReservation(ctxA, secondReservation.ID, true); err != nil {
		t.Fatalf("commit second usage reservation: %v", err)
	}
	if err := repository.SaveUsage(ctxA, domain.UsageRecord{
		Provider:       "integration-test",
		Model:          "usage-guard",
		Feature:        "assistant",
		InputTokens:    30,
		UsageAvailable: true,
		CreatedAt:      createdAt,
	}); err != nil {
		t.Fatalf("save reserved usage: %v", err)
	}
	thirdReservation := firstReservation
	thirdReservation.ID += "-third"
	thirdReservation.Amount = 71
	if allowed, err := repository.ReserveUsage(ctxA, thirdReservation); err != nil {
		t.Fatalf("reserve after committed usage: %v", err)
	} else if allowed {
		t.Fatal("expected recorded usage to count against the monthly limit")
	}
	if err := repository.SaveUsage(ctxA, domain.UsageRecord{
		Provider: "integration-test", Model: "usage-guard", Feature: "stt", EstimatedCost: 95, CreatedAt: createdAt,
	}); err != nil {
		t.Fatalf("save feature-scoped usage: %v", err)
	}
	featureScoped := firstReservation
	featureScoped.ID += "-feature-scoped"
	featureScoped.Metric = UsageMetricCost
	featureScoped.Feature = "assistant"
	featureScoped.Amount = 40
	featureScoped.Limit = 100
	if allowed, err := repository.ReserveUsage(ctxA, featureScoped); err != nil || !allowed {
		t.Fatalf("independent feature reservation: allowed=%v err=%v", allowed, err)
	}
	if err := repository.CompleteUsageReservation(ctxA, featureScoped.ID, false); err != nil {
		t.Fatalf("release independent feature reservation: %v", err)
	}
}
