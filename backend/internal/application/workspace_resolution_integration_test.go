package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/testkit"
)

func TestPostgresWorkspaceResolutionCreatesOnlyAnUnambiguousFreshWorkspace(t *testing.T) {
	fixture := testkit.RequirePostgres(t)
	userID, workspaceID, ctx := seedWorkspaceResolutionUser(t, fixture, "fresh")
	repository := &PostgresWorkspaceRepository{Pool: fixture.Pool}

	if err := repository.EnsureWorkspace(ctx, workspaceID, userID); err != nil {
		t.Fatalf("first workspace resolution: %v", err)
	}
	if err := repository.EnsureWorkspace(ctx, workspaceID, userID); err != nil {
		t.Fatalf("idempotent workspace resolution: %v", err)
	}
	if got := countWorkspaceRows(t, fixture, userID, false); got != 1 {
		t.Fatalf("active workspace count = %d, want 1", got)
	}

	// The owner-row lock makes concurrent first requests converge on the same
	// deterministic workspace instead of creating competing scope roots.
	var waitGroup sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			results <- repository.EnsureWorkspace(ctx, workspaceID, userID)
		}()
	}
	waitGroup.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent workspace resolution: %v", err)
		}
	}
	if got := countWorkspaceRows(t, fixture, userID, false); got != 1 {
		t.Fatalf("concurrent active workspace count = %d, want 1", got)
	}
}

func TestPostgresWorkspaceResolutionRejectsLegacyAmbiguity(t *testing.T) {
	fixture := testkit.RequirePostgres(t)
	repository := &PostgresWorkspaceRepository{Pool: fixture.Pool}

	t.Run("active legacy workspace", func(t *testing.T) {
		userID, workspaceID, ctx := seedWorkspaceResolutionUser(t, fixture, "active-legacy")
		legacyID := "legacy-workspace-" + fmt.Sprint(time.Now().UnixNano())
		insertWorkspaceResolutionRow(t, fixture, ctx, legacyID, userID, nil)

		if err := repository.EnsureWorkspace(ctx, workspaceID, userID); !errors.Is(err, ErrWorkspaceMappingRequired) {
			t.Fatalf("resolution error = %v, want ErrWorkspaceMappingRequired", err)
		}
		if got := countWorkspaceRows(t, fixture, userID, true); got != 1 {
			t.Fatalf("workspace history count = %d, want 1", got)
		}
		if got := countWorkspaceRows(t, fixture, userID, false); got != 1 {
			t.Fatalf("active workspace count = %d, want 1", got)
		}
		if got := countWorkspaceRows(t, fixture, userID, false, workspaceID); got != 0 {
			t.Fatalf("canonical target rows = %d, want 0", got)
		}
	})

	t.Run("multiple active workspaces including target", func(t *testing.T) {
		userID, workspaceID, ctx := seedWorkspaceResolutionUser(t, fixture, "multiple-active")
		insertWorkspaceResolutionRow(t, fixture, ctx, workspaceID, userID, nil)
		insertWorkspaceResolutionRow(t, fixture, ctx, "second-workspace-"+fmt.Sprint(time.Now().UnixNano()), userID, nil)

		if err := repository.EnsureWorkspace(ctx, workspaceID, userID); !errors.Is(err, ErrWorkspaceMappingRequired) {
			t.Fatalf("resolution error = %v, want ErrWorkspaceMappingRequired", err)
		}
	})

	t.Run("deleted workspace history", func(t *testing.T) {
		userID, workspaceID, ctx := seedWorkspaceResolutionUser(t, fixture, "deleted")
		deletedAt := time.Now().UTC()
		insertWorkspaceResolutionRow(t, fixture, ctx, workspaceID, userID, &deletedAt)

		if err := repository.EnsureWorkspace(ctx, workspaceID, userID); !errors.Is(err, ErrWorkspaceUnavailable) {
			t.Fatalf("resolution error = %v, want ErrWorkspaceUnavailable", err)
		}
		if got := countWorkspaceRows(t, fixture, userID, false); got != 0 {
			t.Fatalf("active workspace count = %d, want 0", got)
		}
	})
}

func TestPostgresWorkspaceResolutionRejectsOwnerMismatchAndMissingOwner(t *testing.T) {
	fixture := testkit.RequirePostgres(t)
	repository := &PostgresWorkspaceRepository{Pool: fixture.Pool}

	ownerID, targetID, ownerContext := seedWorkspaceResolutionUser(t, fixture, "owner-mismatch")
	otherID, _, otherContext := seedWorkspaceResolutionUser(t, fixture, "other-owner")
	insertWorkspaceResolutionRow(t, fixture, ownerContext, targetID, otherID, nil)
	if err := repository.EnsureWorkspace(ownerContext, targetID, ownerID); !errors.Is(err, ErrWorkspaceUnavailable) {
		t.Fatalf("owner mismatch error = %v, want ErrWorkspaceUnavailable", err)
	}

	missingID := "workspace-resolution-missing-" + fmt.Sprint(time.Now().UnixNano())
	missingContext := store.WithUser(context.Background(), missingID)
	if err := repository.EnsureWorkspace(missingContext, WorkspaceIDForUser(missingID), missingID); !errors.Is(err, ErrWorkspaceOwnerUnavailable) {
		t.Fatalf("missing owner error = %v, want ErrWorkspaceOwnerUnavailable", err)
	}

	// Keep the second context live until all queries have completed; this also
	// makes the ownership fixture explicit to readers of the test.
	_ = otherContext
}

func TestPostgresWorkspaceResolutionRejectsInvalidInputs(t *testing.T) {
	fixture := testkit.RequirePostgres(t)
	repository := &PostgresWorkspaceRepository{Pool: fixture.Pool}

	if err := repository.EnsureWorkspace(nil, "workspace", "user"); err == nil || err.Error() != "workspace context is required" {
		t.Fatalf("nil context error = %v, want explicit context error", err)
	}
	if err := repository.EnsureWorkspace(context.Background(), "", "user"); err == nil || err.Error() != "workspace and owner are required" {
		t.Fatalf("blank workspace error = %v, want explicit identifier error", err)
	}
	if err := repository.EnsureWorkspace(context.Background(), "workspace", ""); err == nil || err.Error() != "workspace and owner are required" {
		t.Fatalf("blank owner error = %v, want explicit identifier error", err)
	}
}

func seedWorkspaceResolutionUser(t *testing.T, fixture *testkit.Fixture, label string) (string, string, context.Context) {
	t.Helper()
	userID := fmt.Sprintf("workspace-resolution-%s-%d", label, time.Now().UnixNano())
	workspaceID := WorkspaceIDForUser(userID)
	ctx := store.WithUser(context.Background(), userID)
	legacyStore := &store.PostgresStore{Pool: fixture.Pool}
	if err := legacyStore.EnsureUser(ctx, domain.User{ID: userID, DisplayName: "Workspace resolution", CEFR: "B1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("seed workspace resolution user: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
	})
	return userID, workspaceID, ctx
}

func insertWorkspaceResolutionRow(t *testing.T, fixture *testkit.Fixture, ctx context.Context, workspaceID, userID string, deletedAt *time.Time) {
	t.Helper()
	_, err := fixture.Pool.Exec(ctx, `
		INSERT INTO workspaces (id, owner_user_id, name, slug, deleted_at)
		VALUES ($1,$2,'Legacy workspace',$3,$4)
	`, workspaceID, userID, "legacy-"+workspaceID, deletedAt)
	if err != nil {
		t.Fatalf("insert workspace resolution fixture: %v", err)
	}
}

func countWorkspaceRows(t *testing.T, fixture *testkit.Fixture, userID string, includeDeleted bool, ids ...string) int {
	t.Helper()
	query := `SELECT COUNT(*) FROM workspaces WHERE owner_user_id=$1`
	args := []any{userID}
	if !includeDeleted {
		query += ` AND deleted_at IS NULL`
	}
	if len(ids) == 1 {
		query += ` AND id=$2`
		args = append(args, ids[0])
	}
	var count int
	if err := fixture.Pool.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("count workspace resolution rows: %v", err)
	}
	return count
}
