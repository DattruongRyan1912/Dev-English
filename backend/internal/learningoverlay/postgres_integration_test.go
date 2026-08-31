package learningoverlay

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/testkit"
)

func TestPostgresRepositoryPersistsScopedObservations(t *testing.T) {
	fixture := testkit.RequirePostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "learning-pg-user-" + suffix
	workspaceID := "learning-pg-workspace-" + suffix
	if _, err := fixture.Pool.Exec(ctx, `
		INSERT INTO users (id, display_name) VALUES ($1, $2)
	`, userID, userID); err != nil {
		t.Fatalf("create learning test user: %v", err)
	}
	if _, err := fixture.Pool.Exec(ctx, `
		INSERT INTO workspaces (id, owner_user_id, name, slug)
		VALUES ($1, $2, $3, $4)
	`, workspaceID, userID, "Learning PostgreSQL", "learning-pg-"+suffix); err != nil {
		t.Fatalf("create learning test workspace: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := fixture.Pool.Exec(cleanupCtx, `DELETE FROM workspaces WHERE id = $1`, workspaceID); err != nil {
			t.Errorf("cleanup learning test workspace: %v", err)
		}
		if _, err := fixture.Pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("cleanup learning test user: %v", err)
		}
	})

	repository, err := NewPostgresRepository(fixture.Pool)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{WorkspaceID: workspaceID, UserID: userID}
	first, err := service.Record(ctx, scope, ObservationInput{
		SourceType: "task",
		SourceID:   "task-1",
		Skill:      "technical_writing",
		Prompt:     "Explain the incident.",
		Response:   "I will inspect the request trace.",
		Feedback:   "Use a clearer next step.",
	})
	if err != nil {
		t.Fatalf("record first observation: %v", err)
	}
	second, err := service.Record(ctx, scope, ObservationInput{
		SourceType: "decision",
		SourceID:   "decision-1",
		Skill:      "speaking",
		Prompt:     "State the trade-off.",
		Response:   "We choose the bounded retry.",
	})
	if err != nil {
		t.Fatalf("record second observation: %v", err)
	}

	items, err := service.List(ctx, scope, 10)
	if err != nil {
		t.Fatalf("list observations: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("observation count = %d, want 2", len(items))
	}
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.ID] = true
		if item.WorkspaceID != workspaceID || item.UserID != userID {
			t.Fatalf("observation escaped scope: %#v", item)
		}
	}
	if !seen[first.ID] || !seen[second.ID] {
		t.Fatalf("listed ids = %#v, want %q and %q", seen, first.ID, second.ID)
	}

	other, err := repository.List(ctx, Scope{WorkspaceID: workspaceID, UserID: "learning-pg-other-" + suffix}, 10)
	if err != nil {
		t.Fatalf("list other user observations: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("cross-user observations leaked: %#v", other)
	}

	wrongScope := first
	wrongScope.WorkspaceID = "learning-pg-other-workspace-" + suffix
	if err := repository.Save(ctx, scope, wrongScope); err == nil {
		t.Fatal("scope-mismatched observation unexpectedly saved")
	} else if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("scope-mismatched save error = %v", err)
	}
}
