package connectors

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func openConnectorDatabase(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv("DEVENGLISH_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set DEVENGLISH_TEST_DATABASE_URL to run PostgreSQL connector tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create connector PostgreSQL pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping connector PostgreSQL database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, ctx
}

func TestPostgresRevisionStoresPersistImmutableRevisionsAndCursors(t *testing.T) {
	pool, ctx := openConnectorDatabase(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "connector-user-" + suffix
	workspaceID := "connector-workspace-" + suffix
	secondUserID := "connector-user-second-" + suffix
	secondWorkspaceID := "connector-workspace-second-" + suffix
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, $1)`, userID); err != nil {
		t.Fatalf("seed connector user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)`, workspaceID, userID, "Connector test", "connector-test-"+suffix); err != nil {
		t.Fatalf("seed connector workspace: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, $1)`, secondUserID); err != nil {
		t.Fatalf("seed second connector user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)`, secondWorkspaceID, secondUserID, "Second connector test", "connector-test-second-"+suffix); err != nil {
		t.Fatalf("seed second connector workspace: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, workspace := range []string{workspaceID, secondWorkspaceID} {
			for _, table := range []string{"claim_evidence", "knowledge_claims", "knowledge_chunks", "source_revisions", "source_items", "knowledge_sources", "connector_sync_runs", "connector_sync_cursors", "work_history"} {
				_, _ = pool.Exec(cleanupCtx, "DELETE FROM "+table+" WHERE workspace_id=$1", workspace)
			}
		}
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM workspaces WHERE id=$1 AND owner_user_id=$2`, workspaceID, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM workspaces WHERE id=$1 AND owner_user_id=$2`, secondWorkspaceID, secondUserID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, secondUserID)
	})

	driveStore, err := NewPostgresDriveRevisionStore(pool, workspaceID)
	if err != nil {
		t.Fatalf("create Drive store: %v", err)
	}
	modified := time.Date(2026, 8, 28, 1, 0, 0, 0, time.UTC)
	driveItem := DriveSourceItem{
		FileID:       "drive-file-1",
		Name:         "Release runbook",
		MIMEType:     "text/plain",
		WebURL:       "https://drive.google.com/file/d/drive-file-1/view",
		RevisionID:   "drive-revision-1",
		ContentHash:  strings.Repeat("a", 64),
		ModifiedTime: modified,
		Text:         "The release requires evidence-backed answers.",
	}
	if err := driveStore.PutRevision(ctx, driveItem); err != nil {
		t.Fatalf("persist Drive revision: %v", err)
	}
	secondDriveStore, err := NewPostgresDriveRevisionStore(pool, secondWorkspaceID)
	if err != nil {
		t.Fatalf("create second Drive store: %v", err)
	}
	if err := secondDriveStore.PutRevision(ctx, driveItem); err != nil {
		t.Fatalf("persist the same Drive revision in a second workspace: %v", err)
	}
	if exists, err := driveStore.HasRevision(ctx, driveItem.RevisionKey()); err != nil || !exists {
		t.Fatalf("Drive revision lookup = %t/%v, want true/nil", exists, err)
	}
	if err := driveStore.PutRevision(ctx, driveItem); err == nil || !errors.Is(err, ErrRevisionAlreadyExists) {
		t.Fatalf("duplicate Drive revision error = %v, want ErrRevisionAlreadyExists", err)
	}
	removedAt := modified.Add(time.Minute)
	if err := driveStore.MarkRemoved(ctx, driveItem.FileID, removedAt); err != nil {
		t.Fatalf("mark Drive item removed: %v", err)
	}
	var deletedAt *time.Time
	var currentRevision *string
	if err := pool.QueryRow(ctx, `SELECT deleted_at, current_revision_id FROM source_items WHERE workspace_id=$1 AND external_id=$2`, workspaceID, driveItem.FileID).Scan(&deletedAt, &currentRevision); err != nil {
		t.Fatalf("read removed Drive item: %v", err)
	}
	if deletedAt == nil || currentRevision != nil {
		t.Fatalf("removed Drive item = deleted_at=%v current_revision=%v", deletedAt, currentRevision)
	}
	if exists, err := driveStore.HasRevision(ctx, driveItem.RevisionKey()); err != nil || !exists {
		t.Fatalf("immutable Drive revision after removal = %t/%v, want true/nil", exists, err)
	}
	var versionAfterFirst int64
	if err := pool.QueryRow(ctx, `SELECT version FROM source_items WHERE workspace_id=$1 AND external_id=$2`, workspaceID, driveItem.FileID).Scan(&versionAfterFirst); err != nil {
		t.Fatalf("read removal version: %v", err)
	}
	if err := driveStore.MarkRemoved(ctx, driveItem.FileID, removedAt.Add(time.Minute)); err != nil {
		t.Fatalf("replay Drive removal: %v", err)
	}
	var versionAfterReplay int64
	if err := pool.QueryRow(ctx, `SELECT version FROM source_items WHERE workspace_id=$1 AND external_id=$2`, workspaceID, driveItem.FileID).Scan(&versionAfterReplay); err != nil {
		t.Fatalf("read replayed removal version: %v", err)
	}
	if versionAfterReplay != versionAfterFirst {
		t.Fatalf("replayed removal changed version: first=%d replay=%d", versionAfterFirst, versionAfterReplay)
	}
	newDriveItem := driveItem
	newDriveItem.RevisionID = "drive-revision-2"
	newDriveItem.ContentHash = strings.Repeat("b", 64)
	newDriveItem.Text = "The refreshed release runbook is still source-backed."
	newDriveItem.ModifiedTime = removedAt.Add(time.Minute)
	if err := driveStore.PutRevision(ctx, newDriveItem); err != nil {
		t.Fatalf("restore Drive item with a newer revision: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT deleted_at, current_revision_id FROM source_items WHERE workspace_id=$1 AND external_id=$2`, workspaceID, driveItem.FileID).Scan(&deletedAt, &currentRevision); err != nil {
		t.Fatalf("read restored Drive item: %v", err)
	}
	if deletedAt != nil || currentRevision == nil {
		t.Fatalf("restored Drive item = deleted_at=%v current_revision=%v", deletedAt, currentRevision)
	}
	if err := driveStore.SaveCursor(ctx, DriveSyncState{WorkspaceID: workspaceID, Cursor: DriveCursor{Token: "drive-next"}, HasMore: true, LastSynced: modified}); err != nil {
		t.Fatalf("persist Drive cursor: %v", err)
	}
	var cursor string
	var hasMore bool
	if err := pool.QueryRow(ctx, `SELECT cursor, has_more FROM connector_sync_cursors WHERE workspace_id=$1 AND provider=$2 AND target=''`, workspaceID, ProviderGoogleDrive).Scan(&cursor, &hasMore); err != nil {
		t.Fatalf("read Drive cursor: %v", err)
	}
	if cursor != "drive-next" || !hasMore {
		t.Fatalf("Drive cursor = %q/%t, want drive-next/true", cursor, hasMore)
	}
	checkpoint, found, err := driveStore.LoadCursor(ctx, workspaceID)
	if err != nil || !found || checkpoint.Cursor.Token != "drive-next" || !checkpoint.HasMore {
		t.Fatalf("Drive checkpoint = %+v, found=%t, err=%v", checkpoint, found, err)
	}
	runID, err := driveStore.StartSyncRun(ctx, ProviderGoogleDrive, workspaceID, "", "drive-next", modified)
	if err != nil || strings.TrimSpace(runID) == "" {
		t.Fatalf("start Drive sync run = %q, err=%v", runID, err)
	}
	if err := driveStore.CompleteSyncRun(ctx, runID, "succeeded", SyncRunSummary{Seen: 2, Upserted: 1, Skipped: 1}, "drive-final", "", modified.Add(time.Minute)); err != nil {
		t.Fatalf("complete Drive sync run: %v", err)
	}
	var runStatus string
	var seen, upserted, skipped int
	var cursorAfter string
	if err := pool.QueryRow(ctx, `SELECT status, seen_count, upserted_count, skipped_count, cursor_after FROM connector_sync_runs WHERE id=$1 AND workspace_id=$2`, runID, workspaceID).Scan(&runStatus, &seen, &upserted, &skipped, &cursorAfter); err != nil {
		t.Fatalf("read Drive sync run: %v", err)
	}
	if runStatus != "succeeded" || seen != 2 || upserted != 1 || skipped != 1 || cursorAfter != "drive-final" {
		t.Fatalf("Drive sync run = %s/%d/%d/%d/%q", runStatus, seen, upserted, skipped, cursorAfter)
	}
	if err := driveStore.CompleteSyncRun(ctx, runID, "failed", SyncRunSummary{Seen: 99}, "overwritten", "late", modified.Add(90*time.Minute)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Drive sync completion error = %v, want ErrNotFound", err)
	}
	if err := pool.QueryRow(ctx, `SELECT status, seen_count, cursor_after FROM connector_sync_runs WHERE id=$1 AND workspace_id=$2`, runID, workspaceID).Scan(&runStatus, &seen, &cursorAfter); err != nil {
		t.Fatalf("read terminal Drive sync run: %v", err)
	}
	if runStatus != "succeeded" || seen != 2 || cursorAfter != "drive-final" {
		t.Fatalf("terminal Drive sync run was overwritten: %s/%d/%q", runStatus, seen, cursorAfter)
	}
	if err := driveStore.CompleteSyncRun(ctx, runID, "running", SyncRunSummary{}, "", "", modified.Add(2*time.Minute)); err == nil {
		t.Fatal("invalid sync run status was accepted")
	}

	githubStore, err := NewPostgresGitHubRevisionStore(pool, workspaceID)
	if err != nil {
		t.Fatalf("create GitHub store: %v", err)
	}
	issue := GitHubImportItem{Repository: "acme/api", Kind: "issue", Number: 42, Title: "Retry API", Body: "Bound retries", State: "open", Revision: "etag-1", UpdatedAt: modified}
	if err := githubStore.PutRevision(ctx, issue); err != nil {
		t.Fatalf("persist GitHub revision: %v", err)
	}
	if exists, err := githubStore.HasRevision(ctx, issue.RevisionKey()); err != nil || !exists {
		t.Fatalf("GitHub revision lookup = %t/%v, want true/nil", exists, err)
	}
	if err := githubStore.SaveCursor(ctx, GitHubSyncState{WorkspaceID: workspaceID, Repository: issue.Repository, Cursor: "2", HasMore: false, LastSynced: modified}); err != nil {
		t.Fatalf("persist GitHub cursor: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT cursor FROM connector_sync_cursors WHERE workspace_id=$1 AND provider=$2 AND target=$3`, workspaceID, ProviderGitHub, issue.Repository).Scan(&cursor); err != nil {
		t.Fatalf("read GitHub cursor: %v", err)
	}
	if cursor != "2" {
		t.Fatalf("GitHub cursor = %q, want 2", cursor)
	}
	githubCheckpoint, found, err := githubStore.LoadCursor(ctx, workspaceID, issue.Repository)
	if err != nil || !found || githubCheckpoint.Cursor != "2" || githubCheckpoint.HasMore {
		t.Fatalf("GitHub checkpoint = %+v, found=%t, err=%v", githubCheckpoint, found, err)
	}
}

func TestPostgresSafeWriteReceiptStoreScopesWorkspaces(t *testing.T) {
	pool, ctx := openConnectorDatabase(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "safe-write-user-" + suffix
	workspaceA := "safe-write-workspace-a-" + suffix
	workspaceB := "safe-write-workspace-b-" + suffix
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, $1)`, userID); err != nil {
		t.Fatalf("seed safe-write user: %v", err)
	}
	for _, workspace := range []string{workspaceA, workspaceB} {
		if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)`, workspace, userID, workspace, workspace); err != nil {
			t.Fatalf("seed safe-write workspace %s: %v", workspace, err)
		}
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM action_receipts WHERE workspace_id IN ($1, $2)`, workspaceA, workspaceB)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM action_challenges WHERE workspace_id IN ($1, $2)`, workspaceA, workspaceB)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM workspaces WHERE id IN ($1, $2) AND owner_user_id=$3`, workspaceA, workspaceB, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
	})

	challengeStore, err := NewPostgresSafeWriteChallengeStore(pool)
	if err != nil {
		t.Fatalf("create safe-write challenge store: %v", err)
	}
	now := time.Now().UTC()
	targetA := SafeWriteTarget{Operation: SafeWriteOperationCreateIssue, Repository: "acme/alpha", Title: "alpha"}
	targetB := SafeWriteTarget{Operation: SafeWriteOperationCreateIssue, Repository: "acme/beta", Title: "beta"}
	challengeA, err := NewWorkspaceSafeWriteChallenge(workspaceA, "safe-write-challenge-a-"+suffix, userID, targetA, now)
	if err != nil {
		t.Fatal(err)
	}
	challengeB, err := NewWorkspaceSafeWriteChallenge(workspaceB, "safe-write-challenge-b-"+suffix, userID, targetB, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := challengeStore.Put(challengeA); err != nil {
		t.Fatalf("persist alpha challenge: %v", err)
	}
	if err := challengeStore.Put(challengeB); err != nil {
		t.Fatalf("persist beta challenge: %v", err)
	}
	// The same user and canonical action are valid in another workspace. This
	// is the regression case for the workspace-scoped pending-challenge index.
	challengeSameAction, err := NewWorkspaceSafeWriteChallenge(workspaceB, "safe-write-challenge-same-action-"+suffix, userID, targetA, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := challengeStore.Put(challengeSameAction); err != nil {
		t.Fatalf("persist same action in second workspace: %v", err)
	}
	hashA, err := targetA.ActionHash()
	if err != nil {
		t.Fatal(err)
	}
	hashB, err := targetB.ActionHash()
	if err != nil {
		t.Fatal(err)
	}
	receipts, err := NewPostgresSafeWriteReceiptStore(pool)
	if err != nil {
		t.Fatalf("create safe-write receipt store: %v", err)
	}
	key := "same-idempotency-key"
	inputs := []SafeWriteReceipt{
		{ID: deterministicReceiptID(workspaceA, userID, SafeWriteOperationCreateIssue, key, hashA), WorkspaceID: workspaceA, UserID: userID, Operation: SafeWriteOperationCreateIssue, ChallengeID: challengeA.ID, IdempotencyKey: key, ActionHash: hashA, TargetType: "github_repository", TargetID: "acme/alpha", Status: SafeWriteReceiptPending, CreatedAt: now},
		{ID: deterministicReceiptID(workspaceB, userID, SafeWriteOperationCreateIssue, key, hashB), WorkspaceID: workspaceB, UserID: userID, Operation: SafeWriteOperationCreateIssue, ChallengeID: challengeB.ID, IdempotencyKey: key, ActionHash: hashB, TargetType: "github_repository", TargetID: "acme/beta", Status: SafeWriteReceiptPending, CreatedAt: now},
	}
	type reserveResult struct {
		receipt SafeWriteReceipt
		existed bool
		err     error
	}
	results := make([]reserveResult, len(inputs))
	var wg sync.WaitGroup
	for index := range inputs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index].receipt, results[index].existed, results[index].err = receipts.Reserve(ctx, inputs[index])
		}(index)
	}
	wg.Wait()
	for index, result := range results {
		if result.err != nil || result.existed || result.receipt.WorkspaceID != inputs[index].WorkspaceID {
			t.Fatalf("reserve[%d] = receipt:%+v existed:%t err:%v", index, result.receipt, result.existed, result.err)
		}
	}
	if results[0].receipt.ID == results[1].receipt.ID {
		t.Fatalf("workspace-scoped receipt IDs collided: %q", results[0].receipt.ID)
	}
	for index, workspace := range []string{workspaceA, workspaceB} {
		got, found, err := receipts.Lookup(ctx, workspace, userID, SafeWriteOperationCreateIssue, key)
		if err != nil || !found || got.ID != inputs[index].ID || got.WorkspaceID != workspace {
			t.Fatalf("lookup[%d] = receipt:%+v found:%t err:%v", index, got, found, err)
		}
	}
	loadedChallenge, err := challengeStore.Get(ctx, workspaceA, userID, challengeA.ID)
	if err != nil || loadedChallenge.ID != challengeA.ID || loadedChallenge.WorkspaceID != workspaceA || loadedChallenge.UsedAt != nil {
		t.Fatalf("loaded challenge = %+v, err=%v", loadedChallenge, err)
	}
	if _, err := challengeStore.Get(ctx, workspaceA, userID, challengeB.ID); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("cross-workspace challenge lookup error = %v, want ErrInvalidChallenge", err)
	}
	confirmedAt := now.Add(time.Minute)
	if err := challengeStore.Consume(ctx, challengeA.Confirmation(confirmedAt), targetA, confirmedAt); err != nil {
		t.Fatalf("consume challenge: %v", err)
	}
	if err := challengeStore.Consume(ctx, challengeA.Confirmation(confirmedAt), targetA, confirmedAt); !errors.Is(err, ErrChallengeUsed) {
		t.Fatalf("replayed challenge consume error = %v, want ErrChallengeUsed", err)
	}

	accepted := results[0].receipt
	accepted.Status = SafeWriteReceiptAccepted
	accepted.Issue = GitHubIssue{Repository: "acme/alpha", Number: 7, Title: "alpha"}
	if err := receipts.Complete(ctx, accepted); err != nil {
		t.Fatalf("complete pending receipt: %v", err)
	}
	if err := receipts.Complete(ctx, accepted); err != nil {
		t.Fatalf("replay complete accepted receipt: %v", err)
	}
	completed, found, err := receipts.Lookup(ctx, workspaceA, userID, SafeWriteOperationCreateIssue, key)
	if err != nil || !found || completed.Status != SafeWriteReceiptAccepted || completed.Issue.Number != 7 {
		t.Fatalf("completed receipt = %+v found=%t err=%v", completed, found, err)
	}

	saved := SafeWriteReceipt{
		ID:             deterministicReceiptID(workspaceB, userID, SafeWriteOperationCreateIssue, "save-key", hashB),
		WorkspaceID:    workspaceB,
		Provider:       ProviderGitHub,
		Operation:      SafeWriteOperationCreateIssue,
		UserID:         userID,
		ChallengeID:    challengeB.ID,
		IdempotencyKey: "save-key",
		ActionHash:     hashB,
		TargetType:     "github_repository",
		TargetID:       "acme/beta",
		Status:         SafeWriteReceiptAccepted,
		CreatedAt:      now,
		Issue:          GitHubIssue{Repository: "acme/beta", Number: 8, Title: "beta"},
	}
	if err := receipts.Save(ctx, saved); err != nil {
		t.Fatalf("save accepted receipt: %v", err)
	}
	if err := receipts.Save(ctx, saved); err != nil {
		t.Fatalf("replay save accepted receipt: %v", err)
	}
	savedLoaded, found, err := receipts.Lookup(ctx, workspaceB, userID, SafeWriteOperationCreateIssue, "save-key")
	if err != nil || !found || savedLoaded.Issue.Number != 8 {
		t.Fatalf("saved receipt = %+v found=%t err=%v", savedLoaded, found, err)
	}
}
