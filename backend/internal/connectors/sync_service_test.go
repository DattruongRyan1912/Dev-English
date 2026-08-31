package connectors

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

var connectorTestNow = time.Date(2026, time.August, 26, 9, 0, 0, 0, time.UTC)

func TestDriveSyncIsRevisionAndCursorIdempotent(t *testing.T) {
	item := DriveSourceItem{FileID: "file-1", RevisionID: "rev-1", Name: "brief", Text: "source"}
	reader := &FakeDriveReader{Pages: []DrivePage{
		{Items: []DriveSourceItem{item, {FileID: " file-1 ", RevisionID: " rev-1 "}}, NextCursor: DriveCursor{Token: " page-2 "}, HasMore: true},
		{Items: []DriveSourceItem{item}, HasMore: false},
	}}
	store := NewMemoryDriveRevisionStore()
	service, err := NewDriveSyncService(reader, store)
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }

	first, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Seen != 2 || first.Upserted != 1 || first.Skipped != 1 || !first.HasMore || first.NextCursor.Token != "page-2" {
		t.Fatalf("first sync = %+v", first)
	}
	if store.SyncRunCount() != 1 {
		t.Fatalf("sync run count after first page = %d, want 1", store.SyncRunCount())
	}
	state, ok := store.Cursor("workspace-1")
	if !ok || state.Cursor.Token != "page-2" || !state.HasMore || !state.LastSynced.Equal(connectorTestNow) {
		t.Fatalf("checkpoint = %+v, exists=%v", state, ok)
	}

	// The second call deliberately omits the cursor. The service must resume
	// from the checkpoint written by the first successful page.
	second, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Seen != 1 || second.Upserted != 0 || second.Skipped != 1 || second.HasMore {
		t.Fatalf("replayed sync = %+v", second)
	}
	if store.SyncRunCount() != 2 {
		t.Fatalf("sync run count after resume = %d, want 2", store.SyncRunCount())
	}
	if store.RevisionCount() != 1 {
		t.Fatalf("revision count = %d, want 1", store.RevisionCount())
	}
	requests := reader.RequestsSnapshot()
	if len(requests) != 2 || requests[0].PageSize != defaultConnectorPageSize || requests[1].Cursor.Token != "page-2" {
		t.Fatalf("requests = %+v", requests)
	}
}

func TestDriveSyncDoesNotCheckpointFailedPage(t *testing.T) {
	reader := &FakeDriveReader{Pages: []DrivePage{{Items: []DriveSourceItem{{FileID: "file-1", RevisionID: "rev-1"}}, HasMore: false}}}
	store := NewMemoryDriveRevisionStore()
	store.PutRevisionError = errors.New("authorization: Bearer drive-secret")
	service, err := NewDriveSyncService(reader, store)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"})
	if err == nil || strings.Contains(err.Error(), "drive-secret") {
		t.Fatalf("failed sync error = %v", err)
	}
	if _, ok := store.Cursor("workspace-1"); ok {
		t.Fatal("failed page must not checkpoint its cursor")
	}
}

func TestMemorySyncRunCannotBeCompletedTwice(t *testing.T) {
	store := NewMemoryDriveRevisionStore()
	runID, err := store.StartSyncRun(context.Background(), ProviderGoogleDrive, "workspace-1", "", "", connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteSyncRun(context.Background(), runID, "succeeded", SyncRunSummary{Seen: 1}, "done", "", connectorTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteSyncRun(context.Background(), runID, "failed", SyncRunSummary{Seen: 99}, "overwritten", "late", connectorTestNow.Add(2*time.Minute)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second completion error = %v, want ErrNotFound", err)
	}
	run := store.SyncRuns[runID]
	if run.Status != "succeeded" || run.Summary.Seen != 1 || run.CursorAfter != "done" {
		t.Fatalf("terminal sync run was overwritten: %+v", run)
	}
}

func TestDriveSyncTombstonesRemovedFileWithoutCreatingSearchableRevision(t *testing.T) {
	item := DriveSourceItem{FileID: "file-removed", RevisionID: "rev-1", Name: "old notes", Text: "old source"}
	reader := &FakeDriveReader{Pages: []DrivePage{
		{Items: []DriveSourceItem{item}},
		{Items: []DriveSourceItem{{FileID: "file-removed", RevisionID: "removed:file-removed", Removed: true}}, CheckpointCursor: DriveCursor{Token: "start-after-remove"}},
	}}
	store := NewMemoryDriveRevisionStore()
	service, err := NewDriveSyncService(reader, store)
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }
	if _, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"}); err != nil {
		t.Fatal(err)
	}
	result, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Seen != 1 || result.Upserted != 1 || result.Skipped != 0 || result.HasMore {
		t.Fatalf("removal sync = %+v", result)
	}
	if store.RevisionCount() != 1 {
		t.Fatalf("removal must preserve history without adding a tombstone revision: %d", store.RevisionCount())
	}
	if removedAt, ok := store.RemovedAt("file-removed"); !ok || !removedAt.Equal(connectorTestNow) {
		t.Fatalf("removed marker = %v/%v", removedAt, ok)
	}
}

func TestDriveSyncValidatesProviderCursorAndErrors(t *testing.T) {
	reader := &FakeDriveReader{Pages: []DrivePage{{HasMore: true}}}
	store := NewMemoryDriveRevisionStore()
	service, err := NewDriveSyncService(reader, store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("missing continuation cursor error = %v", err)
	}

	reader = &FakeDriveReader{Errors: map[int]error{0: NewProviderError("google_drive", "list", http.StatusTooManyRequests, "retry later")}}
	service.Reader = reader
	if err := func() error {
		_, syncErr := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"})
		return syncErr
	}(); err == nil || ClassifyError(err) != RetryRateLimited {
		t.Fatalf("rate limit error class = %q, err=%v", ClassifyError(err), err)
	}
}

func TestGitHubImportIsRevisionAndCursorIdempotent(t *testing.T) {
	issue := GitHubIssue{Repository: "Owner/Repo", Number: 7, Title: "Bug", Body: "details", State: "open", Revision: "etag-7", UpdatedAt: connectorTestNow}
	reader := &FakeGitHubReadClient{IssuePages: []GitHubIssuePage{
		{Issues: []GitHubIssue{issue, issue}, NextCursor: "next", HasMore: true},
		{Issues: []GitHubIssue{issue}, HasMore: false},
	}, Issues: map[string]GitHubIssue{"owner/repo#7": issue}}
	store := NewMemoryGitHubRevisionStore()
	service, err := NewGitHubImportService(reader, store)
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }

	first, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "Owner/Repo"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Seen != 2 || first.Upserted != 1 || first.Skipped != 1 || first.NextCursor != "next" || !first.HasMore {
		t.Fatalf("first import = %+v", first)
	}
	if store.SyncRunCount() != 1 {
		t.Fatalf("GitHub sync run count after first page = %d, want 1", store.SyncRunCount())
	}
	second, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Seen != 1 || second.Upserted != 0 || second.Skipped != 1 || second.HasMore {
		t.Fatalf("replayed import = %+v", second)
	}
	if store.SyncRunCount() != 2 {
		t.Fatalf("GitHub sync run count after resume = %d, want 2", store.SyncRunCount())
	}
	if store.RevisionCount() != 1 {
		t.Fatalf("revision count = %d, want 1", store.RevisionCount())
	}
	checkpoint, ok := store.Cursor("workspace-1", "owner/repo")
	if !ok || checkpoint.Cursor != githubRestartCursor || checkpoint.HasMore {
		t.Fatalf("completed GitHub scan checkpoint = %+v, exists=%v", checkpoint, ok)
	}
	got, err := service.GetIssue(context.Background(), "owner/repo", 7)
	if err != nil || got.RevisionKey() != "github:owner/repo:issue:7:etag-7" {
		t.Fatalf("get issue = %+v, err=%v", got, err)
	}
	requests := reader.ListRequestsSnapshot()
	if len(requests) != 2 || requests[1].Cursor != "next" {
		t.Fatalf("GitHub page requests = %+v", requests)
	}
}

func TestGitHubImportFailsClosedWithoutStableRevision(t *testing.T) {
	reader := &FakeGitHubReadClient{IssuePages: []GitHubIssuePage{{Issues: []GitHubIssue{{Repository: "owner/repo", Number: 1}}}}}
	service, err := NewGitHubImportService(reader, NewMemoryGitHubRevisionStore())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"}); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("missing revision error = %v", err)
	}
}
