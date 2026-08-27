package work

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openWorkIntegrityDatabase(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv("DEVENGLISH_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set DEVENGLISH_TEST_DATABASE_URL to run PostgreSQL work integrity tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create PostgreSQL work test pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL work test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, ctx
}

func expectWorkIntegrityRejected(t *testing.T, ctx context.Context, tx pgx.Tx, savepoint string, operation func() error) {
	t.Helper()
	if _, err := tx.Exec(ctx, "SAVEPOINT "+savepoint); err != nil {
		t.Fatalf("create savepoint %s: %v", savepoint, err)
	}
	if err := operation(); err == nil {
		t.Fatalf("operation unexpectedly succeeded")
	}
	if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+savepoint); err != nil {
		t.Fatalf("rollback rejected operation: %v", err)
	}
	if _, err := tx.Exec(ctx, "RELEASE SAVEPOINT "+savepoint); err != nil {
		t.Fatalf("release savepoint %s: %v", savepoint, err)
	}
}

func TestPostgresWorkWorkspaceOwnerIntegrity(t *testing.T) {
	pool, ctx := openWorkIntegrityDatabase(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin work integrity transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	var constraintCount int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_constraint
		WHERE contype = 'f'
		  AND conname IN (
			'projects_workspace_owner_fk',
			'tasks_workspace_owner_fk',
			'decisions_workspace_owner_fk',
			'work_idempotency_workspace_user_fk'
		  )
	`).Scan(&constraintCount); err != nil {
		t.Fatalf("inspect work workspace-owner foreign keys: %v", err)
	}
	if constraintCount != 4 {
		t.Fatalf("work workspace-owner foreign key count = %d, want 4", constraintCount)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userA := "work-integrity-user-a-" + suffix
	userB := "work-integrity-user-b-" + suffix
	workspaceA := "work-integrity-workspace-a-" + suffix
	workspaceB := "work-integrity-workspace-b-" + suffix
	statements := []struct {
		query string
		args  []any
	}{
		{"INSERT INTO users (id, display_name) VALUES ($1, $2)", []any{userA, userA}},
		{"INSERT INTO users (id, display_name) VALUES ($1, $2)", []any{userB, userB}},
		{"INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)", []any{workspaceA, userA, "Work A", "work-a-" + suffix}},
		{"INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)", []any{workspaceB, userB, "Work B", "work-b-" + suffix}},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed work integrity fixture: %v", err)
		}
	}

	expectWorkIntegrityRejected(t, ctx, tx, "work_cross_owner_project", func() error {
		_, err := tx.Exec(ctx, `INSERT INTO projects (id, workspace_id, owner_user_id, name) VALUES ($1, $2, $3, 'Invalid project')`, "project-cross-"+suffix, workspaceA, userB)
		return err
	})
	expectWorkIntegrityRejected(t, ctx, tx, "work_cross_owner_task", func() error {
		_, err := tx.Exec(ctx, `INSERT INTO tasks (id, workspace_id, owner_user_id, title) VALUES ($1, $2, $3, 'Invalid task')`, "task-cross-"+suffix, workspaceA, userB)
		return err
	})
	expectWorkIntegrityRejected(t, ctx, tx, "work_cross_owner_decision", func() error {
		_, err := tx.Exec(ctx, `INSERT INTO decisions (id, workspace_id, owner_user_id, title, outcome) VALUES ($1, $2, $3, 'Invalid decision', 'Invalid')`, "decision-cross-"+suffix, workspaceA, userB)
		return err
	})
	expectWorkIntegrityRejected(t, ctx, tx, "work_cross_owner_idempotency", func() error {
		_, err := tx.Exec(ctx, `INSERT INTO work_idempotency (workspace_id, user_id, idempotency_key, operation, request_hash, entity_type, entity_id) VALUES ($1, $2, 'cross-owner', 'project.create', $3, 'project', 'project-cross')`, workspaceA, userB, strings.Repeat("a", 64))
		return err
	})
}
