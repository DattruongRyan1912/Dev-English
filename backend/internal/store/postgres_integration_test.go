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
}

// TestPostgresLegacyContextCompatibilityAcrossWorkspaceAmbiguity protects the
// legacy user-scoped tables while workspace resolution remains undefined. It
// deliberately does not choose a workspace or write canonical Knowledge rows.
func TestPostgresLegacyContextCompatibilityAcrossWorkspaceAmbiguity(t *testing.T) {
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
	userA := "legacy-compat-a-" + suffix
	userB := "legacy-compat-b-" + suffix
	ctxA := WithUser(ctx, userA)
	ctxB := WithUser(ctx, userB)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, userID := range []string{userA, userB} {
			if err := repository.DeleteUserData(WithUser(cleanupCtx, userID)); err != nil && !errors.Is(err, ErrNotFound) {
				t.Errorf("cleanup user %s: %v", userID, err)
			}
		}
		pool.Close()
	}()

	for _, userID := range []string{userA, userB} {
		if err := repository.EnsureUser(WithUser(ctx, userID), domain.User{
			ID:          userID,
			DisplayName: userID,
			CEFR:        "B1",
			CreatedAt:   time.Now().UTC(),
		}); err != nil {
			t.Fatalf("ensure user %s: %v", userID, err)
		}
	}

	workspaceIDs := []string{"legacy-compat-workspace-a1-" + suffix, "legacy-compat-workspace-a2-" + suffix}
	for index, workspaceID := range workspaceIDs {
		_, err := repository.Pool.Exec(ctx, `
			INSERT INTO workspaces (id, owner_user_id, name, slug, description)
			VALUES ($1, $2, $3, $4, $5)`,
			workspaceID,
			userA,
			"Legacy compatibility workspace "+fmt.Sprint(index+1),
			"legacy-compat-"+fmt.Sprint(index+1)+"-"+suffix,
			"synthetic workspace for compatibility isolation",
		)
		if err != nil {
			t.Fatalf("create workspace %s: %v", workspaceID, err)
		}
	}

	var workspaceCount int
	if err := repository.Pool.QueryRow(ctx, `SELECT count(*) FROM workspaces WHERE owner_user_id = $1 AND deleted_at IS NULL`, userA).Scan(&workspaceCount); err != nil {
		t.Fatalf("count ambiguous workspaces: %v", err)
	}
	if workspaceCount != 2 {
		t.Fatalf("expected user A to own two active workspaces, got %d", workspaceCount)
	}
	if err := repository.Pool.QueryRow(ctx, `SELECT count(*) FROM workspaces WHERE owner_user_id = $1 AND deleted_at IS NULL`, userB).Scan(&workspaceCount); err != nil {
		t.Fatalf("count workspace-free user: %v", err)
	}
	if workspaceCount != 0 {
		t.Fatalf("expected user B to own no active workspaces, got %d", workspaceCount)
	}

	createdAt := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	contexts := []struct {
		ctx  context.Context
		item domain.WorkContext
	}{
		{
			ctx: ctxA,
			item: domain.WorkContext{
				ID:         "legacy-compat-context-a-" + suffix,
				SourceType: "manual",
				SourceURL:  "https://example.invalid/legacy/a",
				Title:      "User A legacy sentinel",
				Content:    "user-a-content-must-stay-user-scoped",
				Domain:     "legacy-a",
				CreatedAt:  createdAt,
			},
		},
		{
			ctx: ctxB,
			item: domain.WorkContext{
				ID:         "legacy-compat-context-b-" + suffix,
				SourceType: "github",
				SourceURL:  "https://example.invalid/legacy/b",
				Title:      "User B legacy sentinel",
				Content:    "user-b-content-must-stay-user-scoped",
				Domain:     "legacy-b",
				CreatedAt:  createdAt.Add(time.Minute),
			},
		},
	}
	for _, entry := range contexts {
		if err := repository.SaveWorkContext(entry.ctx, entry.item); err != nil {
			t.Fatalf("save legacy work context %s: %v", entry.item.ID, err)
		}
	}

	for _, entry := range contexts {
		items, err := repository.AllWorkContexts(entry.ctx)
		if err != nil {
			t.Fatalf("read legacy work contexts for %s: %v", entry.item.ID, err)
		}
		if len(items) != 1 {
			t.Fatalf("expected one user-scoped legacy context for %s, got %+v", entry.item.ID, items)
		}
		got := items[0]
		if got.ID != entry.item.ID || got.SourceType != entry.item.SourceType || got.SourceURL != entry.item.SourceURL || got.Title != entry.item.Title || got.Content != entry.item.Content || got.Domain != entry.item.Domain || !got.CreatedAt.Equal(entry.item.CreatedAt) {
			t.Fatalf("legacy context changed or leaked for %s: got %+v, want %+v", entry.item.ID, items[0], entry.item)
		}
	}

	importedSources := []struct {
		id         string
		userID     string
		sourceType string
		title      string
		content    string
	}{
		{
			id:         "legacy-compat-source-a-" + suffix,
			userID:     userA,
			sourceType: "drive",
			title:      "User A imported sentinel",
			content:    "user-a-imported-content-must-stay-user-scoped",
		},
		{
			id:         "legacy-compat-source-b-" + suffix,
			userID:     userB,
			sourceType: "github",
			title:      "User B imported sentinel",
			content:    "user-b-imported-content-must-stay-user-scoped",
		},
	}
	for _, source := range importedSources {
		_, err := repository.Pool.Exec(ctx, `
			INSERT INTO imported_sources (id, user_id, source_type, title, content)
			VALUES ($1, $2, $3, $4, $5)`,
			source.id, source.userID, source.sourceType, source.title, source.content,
		)
		if err != nil {
			t.Fatalf("insert imported source %s: %v", source.id, err)
		}
	}

	for _, source := range importedSources {
		var got struct {
			ID         string
			UserID     string
			SourceType string
			Title      string
			Content    string
		}
		if err := repository.Pool.QueryRow(ctx, `
			SELECT id, user_id, source_type, title, content
			FROM imported_sources
			WHERE id = $1 AND user_id = $2`, source.id, source.userID).Scan(
			&got.ID, &got.UserID, &got.SourceType, &got.Title, &got.Content,
		); err != nil {
			t.Fatalf("read imported source %s in its user scope: %v", source.id, err)
		}
		if got.ID != source.id || got.UserID != source.userID || got.SourceType != source.sourceType || got.Title != source.title || got.Content != source.content {
			t.Fatalf("imported source changed for %s: got %+v, want %+v", source.id, got, source)
		}

		otherUser := userA
		if source.userID == userA {
			otherUser = userB
		}
		var otherUserCount int
		if err := repository.Pool.QueryRow(ctx, `SELECT count(*) FROM imported_sources WHERE id = $1 AND user_id = $2`, source.id, otherUser).Scan(&otherUserCount); err != nil {
			t.Fatalf("check imported source cross-user visibility %s: %v", source.id, err)
		}
		if otherUserCount != 0 {
			t.Fatalf("imported source %s leaked across user scope", source.id)
		}
	}
}
