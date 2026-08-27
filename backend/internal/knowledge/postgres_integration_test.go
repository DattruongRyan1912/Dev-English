package knowledge

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

const knowledgeTestDatabaseEnv = "DEVENGLISH_TEST_DATABASE_URL"

func openKnowledgeTestDatabase(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv(knowledgeTestDatabaseEnv))
	if databaseURL == "" {
		t.Skip("set DEVENGLISH_TEST_DATABASE_URL to run PostgreSQL knowledge integrity tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create PostgreSQL test pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, ctx
}

func expectKnowledgeRejected(t *testing.T, ctx context.Context, tx pgx.Tx, savepoint string, operation func() error) {
	t.Helper()
	if _, err := tx.Exec(ctx, "SAVEPOINT "+savepoint); err != nil {
		t.Fatalf("create savepoint %s: %v", savepoint, err)
	}
	err := operation()
	if err == nil {
		_, err = tx.Exec(ctx, "SET CONSTRAINTS ALL IMMEDIATE")
	}
	if err == nil {
		t.Fatalf("operation unexpectedly succeeded")
	}
	if _, rollbackErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+savepoint); rollbackErr != nil {
		t.Fatalf("rollback rejected operation: %v", rollbackErr)
	}
	if _, releaseErr := tx.Exec(ctx, "RELEASE SAVEPOINT "+savepoint); releaseErr != nil {
		t.Fatalf("release savepoint %s: %v", savepoint, releaseErr)
	}
	if _, err := tx.Exec(ctx, "SET CONSTRAINTS ALL DEFERRED"); err != nil {
		t.Fatalf("restore deferred constraints: %v", err)
	}
}

func requireKnowledgeConstraintsImmediate(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()
	if _, err := tx.Exec(ctx, "SET CONSTRAINTS ALL IMMEDIATE"); err != nil {
		t.Fatalf("apply deferred knowledge constraints: %v", err)
	}
	if _, err := tx.Exec(ctx, "SET CONSTRAINTS ALL DEFERRED"); err != nil {
		t.Fatalf("restore deferred knowledge constraints: %v", err)
	}
}

type knowledgeFixtureIDs struct {
	workspaceA string
	workspaceB string
	sourceA    string
	sourceB    string
	itemA      string
	revisionA  string
	claimA     string
	claimB     string
	evidenceA  string
	evidenceA2 string
	evidenceB  string
}

func seedKnowledgeFixture(t *testing.T, ctx context.Context, tx pgx.Tx) knowledgeFixtureIDs {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ids := knowledgeFixtureIDs{
		workspaceA: "knowledge-workspace-a-" + suffix,
		workspaceB: "knowledge-workspace-b-" + suffix,
		sourceA:    "knowledge-source-a-" + suffix,
		sourceB:    "knowledge-source-b-" + suffix,
		itemA:      "knowledge-item-a-" + suffix,
		revisionA:  "knowledge-revision-a-" + suffix,
		claimA:     "knowledge-claim-a-" + suffix,
		claimB:     "knowledge-claim-b-" + suffix,
		evidenceA:  "knowledge-evidence-a-" + suffix,
		evidenceA2: "knowledge-evidence-a2-" + suffix,
		evidenceB:  "knowledge-evidence-b-" + suffix,
	}
	userA := "knowledge-user-a-" + suffix
	userB := "knowledge-user-b-" + suffix
	statements := []struct {
		query string
		args  []any
	}{
		{"INSERT INTO users (id, display_name) VALUES ($1, $2)", []any{userA, userA}},
		{"INSERT INTO users (id, display_name) VALUES ($1, $2)", []any{userB, userB}},
		{"INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)", []any{ids.workspaceA, userA, "Knowledge A", "knowledge-a-" + suffix}},
		{"INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)", []any{ids.workspaceB, userB, "Knowledge B", "knowledge-b-" + suffix}},
		{"INSERT INTO knowledge_sources (id, workspace_id, kind, name) VALUES ($1, $2, 'manual', 'Source A')", []any{ids.sourceA, ids.workspaceA}},
		{"INSERT INTO knowledge_sources (id, workspace_id, kind, name) VALUES ($1, $2, 'manual', 'Source B')", []any{ids.sourceB, ids.workspaceB}},
		{"INSERT INTO source_items (id, workspace_id, source_id, external_id) VALUES ($1, $2, $3, 'external-a')", []any{ids.itemA, ids.workspaceA, ids.sourceA}},
		{"INSERT INTO source_revisions (id, workspace_id, source_item_id, revision_key, content_hash, content) VALUES ($1, $2, $3, 'revision-a', 'hash-a', 'The deployment uses PostgreSQL.')", []any{ids.revisionA, ids.workspaceA, ids.itemA}},
		{"INSERT INTO knowledge_claims (id, workspace_id, statement, certainty, freshness) VALUES ($1, $2, 'The deployment uses PostgreSQL.', 'canonical', 'current')", []any{ids.claimA, ids.workspaceA}},
		{"INSERT INTO knowledge_claims (id, workspace_id, statement, certainty, freshness) VALUES ($1, $2, 'The database is relational.', 'canonical', 'current')", []any{ids.claimB, ids.workspaceA}},
		{"INSERT INTO claim_evidence (id, workspace_id, claim_id, source_revision_id, quote, freshness) VALUES ($1, $2, $3, $4, 'The deployment uses PostgreSQL.', 'current')", []any{ids.evidenceA, ids.workspaceA, ids.claimA, ids.revisionA}},
		{"INSERT INTO claim_evidence (id, workspace_id, claim_id, source_revision_id, quote, freshness) VALUES ($1, $2, $3, $4, 'The deployment uses PostgreSQL.', 'current')", []any{ids.evidenceA2, ids.workspaceA, ids.claimA, ids.revisionA}},
		{"INSERT INTO claim_evidence (id, workspace_id, claim_id, source_revision_id, quote, freshness) VALUES ($1, $2, $3, $4, 'The database is relational.', 'current')", []any{ids.evidenceB, ids.workspaceA, ids.claimB, ids.revisionA}},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed knowledge fixture: %v", err)
		}
	}
	requireKnowledgeConstraintsImmediate(t, ctx, tx)
	return ids
}

func TestPostgresKnowledgeWorkspaceAndDeferredEvidenceIntegrity(t *testing.T) {
	pool, ctx := openKnowledgeTestDatabase(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin knowledge integrity transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	var rootForeignKeys int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_constraint
		WHERE contype = 'f'
		  AND conname IN ('knowledge_sources_workspace_fk', 'topics_workspace_fk', 'knowledge_claims_workspace_fk')
	`).Scan(&rootForeignKeys); err != nil {
		t.Fatalf("inspect root workspace foreign keys: %v", err)
	}
	if rootForeignKeys != 3 {
		t.Fatalf("root workspace foreign key count = %d, want 3", rootForeignKeys)
	}

	ids := seedKnowledgeFixture(t, ctx, tx)
	expectKnowledgeRejected(t, ctx, tx, "knowledge_missing_source_workspace", func() error {
		_, err := tx.Exec(ctx, `INSERT INTO knowledge_sources (id, workspace_id, kind, name) VALUES ($1, 'missing-workspace', 'manual', 'Invalid')`, "knowledge-invalid-source")
		return err
	})
	expectKnowledgeRejected(t, ctx, tx, "knowledge_cross_workspace_item", func() error {
		_, err := tx.Exec(ctx, `INSERT INTO source_items (id, workspace_id, source_id, external_id) VALUES ($1, $2, $3, 'cross-workspace')`, "knowledge-invalid-item", ids.workspaceB, ids.sourceA)
		return err
	})
	expectKnowledgeRejected(t, ctx, tx, "knowledge_cross_workspace_evidence", func() error {
		_, err := tx.Exec(ctx, `INSERT INTO claim_evidence (id, workspace_id, claim_id, source_revision_id, quote, freshness) VALUES ($1, $2, $3, $4, 'cross-workspace', 'current')`, "knowledge-invalid-evidence", ids.workspaceB, ids.claimA, ids.revisionA)
		return err
	})

	expectKnowledgeRejected(t, ctx, tx, "knowledge_stale_current_claim", func() error {
		if _, err := tx.Exec(ctx, `INSERT INTO knowledge_claims (id, workspace_id, statement, certainty, freshness) VALUES ($1, $2, 'A claim with stale evidence.', 'canonical', 'current')`, "knowledge-stale-claim", ids.workspaceA); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO claim_evidence (id, workspace_id, claim_id, source_revision_id, quote, freshness) VALUES ($1, $2, $3, $4, 'stale quote', 'stale')`, "knowledge-stale-evidence", ids.workspaceA, "knowledge-stale-claim", ids.revisionA)
		return err
	})

	if _, err := tx.Exec(ctx, `UPDATE claim_evidence SET claim_id = $1 WHERE id = $2`, ids.claimB, ids.evidenceA); err != nil {
		t.Fatalf("move evidence between valid claims: %v", err)
	}
	requireKnowledgeConstraintsImmediate(t, ctx, tx)
	expectKnowledgeRejected(t, ctx, tx, "knowledge_move_last_evidence", func() error {
		_, err := tx.Exec(ctx, `UPDATE claim_evidence SET claim_id = $1 WHERE id = $2`, ids.claimB, ids.evidenceA2)
		return err
	})
	expectKnowledgeRejected(t, ctx, tx, "knowledge_delete_last_evidence", func() error {
		_, err := tx.Exec(ctx, `DELETE FROM claim_evidence WHERE id = $1`, ids.evidenceA2)
		return err
	})
	expectKnowledgeRejected(t, ctx, tx, "knowledge_stale_mixed_evidence", func() error {
		_, err := tx.Exec(ctx, `UPDATE claim_evidence SET freshness = 'stale' WHERE id = $1`, ids.evidenceB)
		return err
	})
}

func TestPostgresKnowledgeRevisionsAreImmutable(t *testing.T) {
	pool, ctx := openKnowledgeTestDatabase(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin revision integrity transaction: %v", err)
	}
	defer tx.Rollback(ctx)
	ids := seedKnowledgeFixture(t, ctx, tx)

	newRevisionID := "knowledge-unreferenced-revision-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if _, err := tx.Exec(ctx, `INSERT INTO source_revisions (id, workspace_id, source_item_id, revision_key, content_hash, content) VALUES ($1, $2, $3, 'revision-unreferenced', 'hash-unreferenced', 'Immutable text')`, newRevisionID, ids.workspaceA, ids.itemA); err != nil {
		t.Fatalf("insert unreferenced revision: %v", err)
	}
	expectKnowledgeRejected(t, ctx, tx, "knowledge_revision_update", func() error {
		_, err := tx.Exec(ctx, `UPDATE source_revisions SET content = 'Changed text' WHERE id = $1`, newRevisionID)
		return err
	})
	expectKnowledgeRejected(t, ctx, tx, "knowledge_revision_delete", func() error {
		_, err := tx.Exec(ctx, `DELETE FROM source_revisions WHERE id = $1`, newRevisionID)
		return err
	})
}
