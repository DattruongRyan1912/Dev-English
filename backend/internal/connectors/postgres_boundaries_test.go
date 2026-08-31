package connectors

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func seedConnectorBoundaryWorkspace(t *testing.T) (*pgxpool.Pool, context.Context, string, string) {
	t.Helper()
	pool, ctx := openConnectorDatabase(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "connector-boundary-user-" + suffix
	workspaceID := "connector-boundary-workspace-" + suffix
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, $1)`, userID); err != nil {
		t.Fatalf("seed connector boundary user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)`, workspaceID, userID, "Connector boundary", "connector-boundary-"+suffix); err != nil {
		t.Fatalf("seed connector boundary workspace: %v", err)
	}
	cleanup := func() error {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// Revisions are intentionally immutable and cannot be deleted by a test
		// cleanup query. Keep the fixture scoped to unique IDs, and remove the
		// mutable descendants so repeated runs do not retain action/checkpoint
		// state. This mirrors the existing PostgreSQL integration fixtures.
		for _, table := range []string{"action_receipts", "action_challenges", "knowledge_chunks", "connector_sync_runs", "connector_sync_cursors"} {
			_, _ = pool.Exec(cleanupCtx, "DELETE FROM "+table+" WHERE workspace_id=$1", workspaceID)
		}
		pool.Close()
		return nil
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Errorf("cleanup connector boundary workspace: %v", err)
		}
	})
	return pool, ctx, userID, workspaceID
}

func TestPostgresRevisionStoresRejectInvalidStatesAndUseDurableCheckpoints(t *testing.T) {
	pool, ctx, _, workspaceID := seedConnectorBoundaryWorkspace(t)
	drive, err := NewPostgresDriveRevisionStore(pool, workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	github, err := NewPostgresGitHubRevisionStore(pool, workspaceID)
	if err != nil {
		t.Fatal(err)
	}

	if _, found, err := drive.LoadCursor(ctx, workspaceID); err != nil || found {
		t.Fatalf("missing Drive cursor = found:%t err:%v", found, err)
	}
	if _, found, err := github.LoadCursor(ctx, workspaceID, "owner/repo"); err != nil || found {
		t.Fatalf("missing GitHub cursor = found:%t err:%v", found, err)
	}
	if _, err := drive.HasRevision(ctx, "missing"); err != nil {
		t.Fatalf("missing revision lookup = %v", err)
	}
	if err := drive.PutRevision(ctx, DriveSourceItem{}); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("invalid Drive item = %v", err)
	}
	if err := github.PutRevision(ctx, GitHubImportItem{Repository: "owner/repo", Kind: "issue"}); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("invalid GitHub item = %v", err)
	}
	if err := github.MarkRemoved(ctx, "issue", time.Now()); !errors.Is(err, ErrRemovalNotSupported) {
		t.Fatalf("GitHub removal = %v", err)
	}
	if err := drive.MarkRemoved(ctx, " ", time.Time{}); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("blank Drive removal = %v", err)
	}
	if err := drive.SaveCursor(ctx, DriveSyncState{WorkspaceID: "other", Cursor: DriveCursor{Token: "next"}}); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("cross-workspace Drive cursor = %v", err)
	}
	if _, _, err := drive.LoadCursor(ctx, "other"); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("cross-workspace Drive load = %v", err)
	}
	if err := github.SaveCursor(ctx, GitHubSyncState{WorkspaceID: "other", Repository: "owner/repo"}); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("cross-workspace GitHub cursor = %v", err)
	}
	if _, _, err := github.LoadCursor(ctx, workspaceID, " "); !errors.Is(err, ErrInvalidRepository) {
		t.Fatalf("blank GitHub repository = %v", err)
	}

	modified := time.Now().UTC().Add(-time.Minute)
	driveItem := DriveSourceItem{
		FileID:       "boundary-drive-file",
		Name:         "Boundary notes",
		MIMEType:     "text/plain",
		WebURL:       "https://drive.google.com/file/d/boundary-drive-file/view",
		RevisionID:   "boundary-revision-1",
		ModifiedTime: modified,
		Text:         "Durable source content",
	}
	if err := drive.PutRevision(ctx, driveItem); err != nil {
		t.Fatalf("Drive item with computed hash = %v", err)
	}
	if err := drive.PutRevision(ctx, driveItem); !errors.Is(err, ErrRevisionAlreadyExists) {
		t.Fatalf("duplicate Drive revision = %v", err)
	}
	embeddedItem := driveItem
	embeddedItem.FileID = "boundary-drive-embedded"
	embeddedItem.RevisionID = "boundary-revision-embedded"
	embeddedItem.Text = "Embedded source content"
	embedder := &connectorTestEmbedder{vector: make([]float32, 384)}
	embeddedStore, err := NewPostgresDriveRevisionStoreWithEmbedder(pool, workspaceID, embedder)
	if err != nil {
		t.Fatal(err)
	}
	if err := embeddedStore.PutRevision(ctx, embeddedItem); err != nil {
		t.Fatalf("Drive item with embedding = %v", err)
	}

	if err := drive.SaveCursor(ctx, DriveSyncState{WorkspaceID: workspaceID, Cursor: DriveCursor{Token: "  next-drive  "}, HasMore: true, LastSynced: time.Time{}}); err != nil {
		t.Fatalf("Drive cursor upsert = %v", err)
	}
	loadedDrive, found, err := drive.LoadCursor(ctx, workspaceID)
	if err != nil || !found || loadedDrive.Cursor.Token != "next-drive" || !loadedDrive.HasMore {
		t.Fatalf("Drive cursor = %+v/%t err=%v", loadedDrive, found, err)
	}
	if err := github.SaveCursor(ctx, GitHubSyncState{WorkspaceID: workspaceID, Repository: "Owner/Repo", Cursor: "  next-github  ", HasMore: true, LastSynced: time.Time{}}); err != nil {
		t.Fatalf("GitHub cursor upsert = %v", err)
	}
	loadedGitHub, found, err := github.LoadCursor(ctx, workspaceID, "owner/repo")
	if err != nil || !found || loadedGitHub.Cursor != "next-github" || !loadedGitHub.HasMore {
		t.Fatalf("GitHub cursor = %+v/%t err=%v", loadedGitHub, found, err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO connector_sync_cursors (workspace_id, provider, target, cursor, has_more) VALUES ($1, $2, '', '', true) ON CONFLICT (workspace_id, provider, target) DO UPDATE SET cursor='', has_more=true`, workspaceID, ProviderGoogleDrive); err != nil {
		t.Fatal(err)
	}
	if _, _, err := drive.LoadCursor(ctx, workspaceID); !errors.Is(err, ErrInvalidSyncState) {
		t.Fatalf("invalid Drive checkpoint = %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO connector_sync_cursors (workspace_id, provider, target, cursor, has_more) VALUES ($1, $2, 'owner/repo-invalid', '', true) ON CONFLICT (workspace_id, provider, target) DO UPDATE SET cursor='', has_more=true`, workspaceID, ProviderGitHub); err != nil {
		t.Fatal(err)
	}
	if _, _, err := github.LoadCursor(ctx, workspaceID, "owner/repo-invalid"); !errors.Is(err, ErrInvalidSyncState) {
		t.Fatalf("invalid GitHub checkpoint = %v", err)
	}

	if _, err := drive.StartSyncRun(ctx, ProviderGitHub, workspaceID, "", "", time.Time{}); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("wrong Drive provider run = %v", err)
	}
	if _, err := drive.StartSyncRun(ctx, ProviderGoogleDrive, "other", "", "", time.Time{}); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("cross-workspace sync run = %v", err)
	}
	runID, err := drive.StartSyncRun(ctx, ProviderGoogleDrive, workspaceID, "", "next", time.Time{})
	if err != nil || strings.TrimSpace(runID) == "" {
		t.Fatalf("Drive sync run = %q err=%v", runID, err)
	}
	for _, testCase := range []struct {
		name    string
		status  string
		runID   string
		summary SyncRunSummary
		want    error
	}{
		{"invalid status", "running", runID, SyncRunSummary{}, nil},
		{"missing run", "failed", "", SyncRunSummary{}, nil},
		{"negative seen", "failed", runID, SyncRunSummary{Seen: -1}, nil},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			err := drive.CompleteSyncRun(ctx, testCase.runID, testCase.status, testCase.summary, "", "", time.Time{})
			if err == nil {
				t.Fatalf("invalid completion was accepted")
			}
		})
	}
	if err := drive.CompleteSyncRun(ctx, runID, "succeeded", SyncRunSummary{Seen: 2, Upserted: 2}, "done", "", time.Time{}); err != nil {
		t.Fatalf("complete Drive run = %v", err)
	}
	if err := drive.CompleteSyncRun(ctx, runID, "failed", SyncRunSummary{}, "late", "late", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("replayed Drive run = %v", err)
	}

	issue := GitHubImportItem{Repository: "Owner/Repo", Kind: "issue", Number: 42, Title: "Boundary issue", Body: "Body", State: "open", Revision: "etag-boundary", UpdatedAt: modified}
	if err := github.PutRevision(ctx, issue); err != nil {
		t.Fatalf("GitHub item with number fallback = %v", err)
	}
	if exists, err := github.HasRevision(ctx, issue.RevisionKey()); err != nil || !exists {
		t.Fatalf("GitHub revision lookup = %t/%v", exists, err)
	}
}

func TestPostgresSafeWriteStoresRejectReplayAndPersistenceBoundaries(t *testing.T) {
	pool, ctx, userID, workspaceID := seedConnectorBoundaryWorkspace(t)
	challenges, err := NewPostgresSafeWriteChallengeStore(pool)
	if err != nil {
		t.Fatal(err)
	}
	receipts, err := NewPostgresSafeWriteReceiptStore(pool)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	target := SafeWriteTarget{Operation: SafeWriteOperationCreateIssue, Repository: "owner/repo", Title: "Boundary issue"}
	if err := challenges.Put(SafeWriteChallenge{}); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("invalid challenge Put = %v", err)
	}
	noWorkspace, err := NewSafeWriteChallenge("no-workspace", userID, target, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := challenges.Put(noWorkspace); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("workspace-less challenge Put = %v", err)
	}
	expired := noWorkspace
	expired.WorkspaceID = workspaceID
	expired.ID = "expired-challenge"
	expired.CreatedAt = now.Add(-2 * time.Minute)
	expired.ExpiresAt = now.Add(-time.Second)
	if err := challenges.Put(expired); !errors.Is(err, ErrChallengeExpired) {
		t.Fatalf("expired challenge Put = %v", err)
	}

	challenge, err := NewWorkspaceSafeWriteChallenge(workspaceID, "boundary-challenge", userID, target, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := challenges.Put(challenge); err != nil {
		t.Fatalf("challenge Put = %v", err)
	}
	if err := challenges.Put(challenge); err != nil {
		t.Fatalf("idempotent challenge Put = %v", err)
	}
	if _, err := challenges.Get(ctx, "other", userID, challenge.ID); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("cross-workspace challenge Get = %v", err)
	}
	if _, err := challenges.Get(ctx, workspaceID, userID, "missing"); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("missing challenge Get = %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := challenges.Get(canceled, workspaceID, userID, challenge.ID); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("canceled challenge Get = %v", err)
	}
	if err := challenges.Consume(canceled, challenge.Confirmation(now), target, now); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled challenge Consume = %v", err)
	}
	if err := challenges.Consume(ctx, challenge.Confirmation(now), SafeWriteTarget{Operation: "github.push", Repository: "owner/repo"}, now); !errors.Is(err, ErrUnsupportedWrite) {
		t.Fatalf("invalid target Consume = %v", err)
	}
	wrongConfirmation := challenge.Confirmation(now)
	wrongConfirmation.UserID = "other-user"
	if err := challenges.Consume(ctx, wrongConfirmation, target, now); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("wrong user Consume = %v", err)
	}
	if err := challenges.Consume(ctx, challenge.Confirmation(now), target, now); err != nil {
		t.Fatalf("challenge Consume = %v", err)
	}
	if err := challenges.Consume(ctx, challenge.Confirmation(now), target, now); !errors.Is(err, ErrChallengeUsed) {
		t.Fatalf("replayed challenge Consume = %v", err)
	}

	if _, found, err := receipts.Lookup(ctx, workspaceID, userID, target.Operation, "missing"); err != nil || found {
		t.Fatalf("missing receipt Lookup = found:%t err:%v", found, err)
	}
	if err := receipts.Save(ctx, SafeWriteReceipt{}); !errors.Is(err, ErrReceiptPersistence) {
		t.Fatalf("invalid receipt Save = %v", err)
	}
	hash, err := target.ActionHash()
	if err != nil {
		t.Fatal(err)
	}
	base := SafeWriteReceipt{ID: "boundary-receipt", WorkspaceID: workspaceID, Provider: ProviderGitHub, Operation: target.Operation, UserID: userID, ChallengeID: challenge.ID, IdempotencyKey: "boundary-key", ActionHash: hash, TargetType: "github_repository", TargetID: "owner/repo", Status: SafeWriteReceiptPending, CreatedAt: time.Time{}}
	reserved, existed, err := receipts.Reserve(ctx, base)
	if err != nil || existed || reserved.CreatedAt.IsZero() {
		t.Fatalf("receipt Reserve = %+v existed:%t err:%v", reserved, existed, err)
	}
	if _, existed, err := receipts.Reserve(ctx, base); err != nil || !existed {
		t.Fatalf("replayed receipt Reserve = existed:%t err:%v", existed, err)
	}
	accepted := base
	accepted.Status = SafeWriteReceiptAccepted
	accepted.Issue = GitHubIssue{Repository: "owner/repo", Number: 9, Title: "Boundary issue"}
	if err := receipts.Complete(ctx, accepted); err != nil {
		t.Fatalf("receipt Complete = %v", err)
	}
	if err := receipts.Complete(ctx, accepted); err != nil {
		t.Fatalf("idempotent receipt Complete = %v", err)
	}
	conflicting := accepted
	conflicting.ID = "different-receipt"
	conflicting.ActionHash = "different-hash"
	if err := receipts.Complete(ctx, conflicting); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("receipt Complete conflict = %v", err)
	}
	if err := receipts.Complete(ctx, SafeWriteReceipt{ID: "missing", WorkspaceID: workspaceID, UserID: userID, Operation: target.Operation, IdempotencyKey: "missing", ActionHash: hash, Status: SafeWriteReceiptAccepted}); !errors.Is(err, ErrReceiptPersistence) {
		t.Fatalf("missing receipt Complete = %v", err)
	}

	corruptChallenge, err := NewWorkspaceSafeWriteChallenge(workspaceID, "corrupt-challenge", userID, target, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := challenges.Put(corruptChallenge); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO action_receipts (id, challenge_id, workspace_id, user_id, action, action_hash, target_type, target_id, idempotency_key, status, output) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'"not-an-object"'::jsonb)`, "corrupt-receipt", corruptChallenge.ID, workspaceID, userID, string(target.Operation), hash, "github_repository", "owner/repo", "corrupt-key", SafeWriteReceiptAccepted); err != nil {
		t.Fatalf("seed corrupt receipt: %v", err)
	}
	if _, _, err := receipts.Lookup(ctx, workspaceID, userID, target.Operation, "corrupt-key"); !errors.Is(err, ErrReceiptPersistence) {
		t.Fatalf("corrupt receipt Lookup = %v", err)
	}
	if err := receipts.Save(ctx, accepted); err != nil {
		t.Fatalf("idempotent receipt Save = %v", err)
	}
	conflictingSave := accepted
	conflictingSave.ActionHash = "different-save-hash"
	if err := receipts.Save(ctx, conflictingSave); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("receipt Save conflict = %v", err)
	}
}
