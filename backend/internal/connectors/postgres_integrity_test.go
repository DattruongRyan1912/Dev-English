package connectors

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

func openConnectorIntegrityDatabase(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv("DEVENGLISH_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set DEVENGLISH_TEST_DATABASE_URL to run PostgreSQL connector integrity tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create PostgreSQL connector test pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL connector test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, ctx
}

func expectConnectorIntegrityRejected(t *testing.T, ctx context.Context, tx pgx.Tx, savepoint string, operation func() error) {
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

func TestPostgresSafeWriteChallengeAndReceiptScope(t *testing.T) {
	pool, ctx := openConnectorIntegrityDatabase(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin connector integrity transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	var constraintCount int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_constraint
		WHERE contype = 'f'
		  AND conname IN ('action_challenges_workspace_user_fk', 'action_receipts_challenge_scope_fk')
	`).Scan(&constraintCount); err != nil {
		t.Fatalf("inspect safe-write scope foreign keys: %v", err)
	}
	if constraintCount != 2 {
		t.Fatalf("safe-write scope foreign key count = %d, want 2", constraintCount)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userA := "connector-integrity-user-a-" + suffix
	userB := "connector-integrity-user-b-" + suffix
	workspaceA := "connector-integrity-workspace-a-" + suffix
	workspaceB := "connector-integrity-workspace-b-" + suffix
	challengeA := "connector-integrity-challenge-a-" + suffix
	challengeB := "connector-integrity-challenge-b-" + suffix
	statements := []struct {
		query string
		args  []any
	}{
		{"INSERT INTO users (id, display_name) VALUES ($1, $2)", []any{userA, userA}},
		{"INSERT INTO users (id, display_name) VALUES ($1, $2)", []any{userB, userB}},
		{"INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)", []any{workspaceA, userA, "Connector A", "connector-a-" + suffix}},
		{"INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)", []any{workspaceB, userB, "Connector B", "connector-b-" + suffix}},
		{"INSERT INTO action_challenges (id, workspace_id, user_id, action, target_type, target_id, action_hash, prompt) VALUES ($1, $2, $3, 'github.issue.create', 'github_repository', 'owner/repo', $4, 'Confirm issue')", []any{challengeA, workspaceA, userA, strings.Repeat("a", 64)}},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed connector integrity fixture: %v", err)
		}
	}

	expectConnectorIntegrityRejected(t, ctx, tx, "connector_cross_owner_challenge", func() error {
		_, err := tx.Exec(ctx, `INSERT INTO action_challenges (id, workspace_id, user_id, action, target_type, target_id, action_hash, prompt) VALUES ($1, $2, $3, 'github.issue.create', 'github_repository', 'owner/repo', $4, 'Invalid confirmation')`, "connector-invalid-challenge-"+suffix, workspaceA, userB, strings.Repeat("b", 64))
		return err
	})
	expectConnectorIntegrityRejected(t, ctx, tx, "connector_cross_scope_receipt", func() error {
		_, err := tx.Exec(ctx, `INSERT INTO action_receipts (id, challenge_id, workspace_id, user_id, action, target_type, target_id, idempotency_key, status) VALUES ($1, $2, $3, $4, 'github.issue.create', 'github_repository', 'owner/repo', 'cross-scope', 'accepted')`, "connector-invalid-receipt-"+suffix, challengeA, workspaceB, userB)
		return err
	})
	if _, err := tx.Exec(ctx, `INSERT INTO action_receipts (id, challenge_id, workspace_id, user_id, action, target_type, target_id, idempotency_key, status) VALUES ($1, $2, $3, $4, 'github.issue.create', 'github_repository', 'owner/repo', 'same-scope', 'accepted')`, "connector-valid-receipt-"+suffix, challengeA, workspaceA, userA); err != nil {
		t.Fatalf("same-scope receipt rejected: %v", err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO action_challenges (id, workspace_id, user_id, action, target_type, target_id, action_hash, prompt) VALUES ($1, $2, $3, 'github.issue.comment', 'github_issue', 'owner/repo#1', $4, 'Confirm comment')`, challengeB, workspaceA, userA, strings.Repeat("c", 64)); err != nil {
		t.Fatalf("seed second same-scope challenge: %v", err)
	}
	expectConnectorIntegrityRejected(t, ctx, tx, "connector_duplicate_receipt_tuple", func() error {
		_, err := tx.Exec(ctx, `INSERT INTO action_receipts (id, challenge_id, workspace_id, user_id, action, target_type, target_id, idempotency_key, status) VALUES ($1, $2, $3, $4, 'github.issue.create', 'github_repository', 'owner/repo', 'same-scope', 'accepted')`, "connector-duplicate-receipt-"+suffix, challengeA, workspaceA, userA)
		return err
	})
	if _, err := tx.Exec(ctx, `INSERT INTO action_receipts (id, challenge_id, workspace_id, user_id, action, target_type, target_id, idempotency_key, status) VALUES ($1, $2, $3, $4, 'github.issue.comment', 'github_issue', 'owner/repo#1', 'same-scope', 'accepted')`, "connector-different-operation-receipt-"+suffix, challengeB, workspaceA, userA); err != nil {
		t.Fatalf("same workspace/user/key with different operation rejected: %v", err)
	}
}
