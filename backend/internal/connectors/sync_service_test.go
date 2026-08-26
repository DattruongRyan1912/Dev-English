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

	first, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: " workspace-1 "})
	if err != nil {
		t.Fatal(err)
	}
	if first.Seen != 2 || first.Upserted != 1 || first.Skipped != 1 || !first.HasMore || first.NextCursor.Token != "page-2" {
		t.Fatalf("first sync = %+v", first)
	}
	state, ok := store.Cursor("workspace-1")
	if !ok || state.Cursor.Token != "page-2" || !state.HasMore || !state.LastSynced.Equal(connectorTestNow) {
		t.Fatalf("checkpoint = %+v, exists=%v", state, ok)
	}

	second, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1", Cursor: first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if second.Seen != 1 || second.Upserted != 0 || second.Skipped != 1 || second.HasMore || second.NextCursor.Valid() {
		t.Fatalf("replayed sync = %+v", second)
	}
	if store.RevisionCount() != 1 {
		t.Fatalf("revision count = %d, want 1", store.RevisionCount())
	}
	requests := reader.RequestsSnapshot()
	if len(requests) != 2 || requests[0].PageSize != defaultConnectorPageSize || requests[1].Cursor.Token != "page-2" || requests[0].WorkspaceID != "workspace-1" {
		t.Fatalf("requests = %+v", requests)
	}
}

func TestDriveSyncRevisionDeduplicationIsWorkspaceScoped(t *testing.T) {
	item := DriveSourceItem{FileID: "shared-file", RevisionID: "shared-revision", Name: "brief", Text: "source"}
	store := NewMemoryDriveRevisionStore()
	for _, workspaceID := range []string{"workspace-a", "workspace-b"} {
		reader := &FakeDriveReader{Pages: []DrivePage{{Items: []DriveSourceItem{item}}}}
		service, err := NewDriveSyncService(reader, store)
		if err != nil {
			t.Fatal(err)
		}
		service.Clock = func() time.Time { return connectorTestNow }
		result, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: workspaceID})
		if err != nil || result.Upserted != 1 || result.Skipped != 0 {
			t.Fatalf("workspace %q sync=%+v err=%v", workspaceID, result, err)
		}
	}
	if store.RevisionCount() != 2 {
		t.Fatalf("workspace-scoped revision count=%d, want 2", store.RevisionCount())
	}
	for _, workspaceID := range []string{"workspace-a", "workspace-b"} {
		exists, err := store.HasRevision(context.Background(), workspaceID, item.RevisionKey())
		if err != nil || !exists {
			t.Fatalf("workspace %q revision exists=%v err=%v", workspaceID, exists, err)
		}
	}
}

func TestDriveSyncDoesNotCheckpointFailedPage(t *testing.T) {
	reader := &FakeDriveReader{Pages: []DrivePage{{Items: []DriveSourceItem{{FileID: "file-1", RevisionID: "rev-1"}}, HasMore: false}}}
	store := NewMemoryDriveRevisionStore()
	store.PutRevisionError = errors.New("authorization: Bearer fake-drive-secret")
	service, err := NewDriveSyncService(reader, store)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"})
	if err == nil || strings.Contains(err.Error(), "fake-drive-secret") {
		t.Fatalf("failed sync error = %v", err)
	}
	if _, ok := store.Cursor("workspace-1"); ok {
		t.Fatal("failed page must not checkpoint its cursor")
	}
}

func TestDriveSyncFailureStagesAndValidation(t *testing.T) {
	if _, err := NewDriveSyncService(nil, NewMemoryDriveRevisionStore()); err == nil {
		t.Fatal("nil reader should be rejected")
	}
	if _, err := NewDriveSyncService(&FakeDriveReader{}, nil); err == nil {
		t.Fatal("nil store should be rejected")
	}
	store := NewMemoryDriveRevisionStore()
	service, err := NewDriveSyncService(&FakeDriveReader{}, store)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []DriveSyncRequest{
		{},
		{WorkspaceID: "workspace-1", Cursor: DriveCursor{Token: " "}},
		{WorkspaceID: "workspace-1", PageSize: -1},
		{WorkspaceID: "workspace-1", PageSize: maxConnectorPageSize + 1},
	} {
		if _, err := service.Sync(context.Background(), request); err == nil {
			t.Fatalf("invalid request unexpectedly succeeded: %+v", request)
		}
	}

	reader := &FakeDriveReader{Pages: []DrivePage{{HasMore: true}}}
	service.Reader = reader
	if _, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("missing continuation cursor error = %v", err)
	}
	reader = &FakeDriveReader{Errors: map[int]error{0: NewProviderError("google_drive", "list", http.StatusTooManyRequests, "retry later")}}
	service.Reader = reader
	if _, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"}); err == nil || ClassifyError(err) != RetryRateLimited {
		t.Fatalf("rate limit error class = %q, err=%v", ClassifyError(err), err)
	}

	item := DriveSourceItem{FileID: "file-1", RevisionID: "rev-1"}
	for _, setup := range []struct {
		name  string
		store *MemoryDriveRevisionStore
	}{
		{name: "has revision", store: &MemoryDriveRevisionStore{HasRevisionError: errors.New("query api_key=fake-query-secret")}},
		{name: "save cursor", store: &MemoryDriveRevisionStore{SaveCursorError: errors.New("save token=fake-save-secret")}},
	} {
		t.Run(setup.name, func(t *testing.T) {
			if setup.store.Revisions == nil {
				setup.store.Revisions = make(map[string]DriveSourceItem)
			}
			if setup.store.Cursors == nil {
				setup.store.Cursors = make(map[string]DriveSyncState)
			}
			service, err := NewDriveSyncService(&FakeDriveReader{Pages: []DrivePage{{Items: []DriveSourceItem{item}}}}, setup.store)
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"})
			if err == nil || strings.Contains(err.Error(), "fake-") {
				t.Fatalf("stage error = %v", err)
			}
		})
	}

	putStore := NewMemoryDriveRevisionStore()
	putStore.PutRevisionError = ErrRevisionAlreadyExists
	service, err = NewDriveSyncService(&FakeDriveReader{Pages: []DrivePage{{Items: []DriveSourceItem{item}}}}, putStore)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"})
	if err != nil || result.Skipped != 1 || result.Upserted != 0 {
		t.Fatalf("already-existing revision result=%+v err=%v", result, err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Sync(canceled, DriveSyncRequest{WorkspaceID: "workspace-1"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled sync error = %v", err)
	}
	if err := (DriveSyncState{WorkspaceID: "workspace-1", HasMore: true}).Validate(); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("invalid drive state = %v", err)
	}
}

func TestGitHubImportIsRevisionAndCursorIdempotent(t *testing.T) {
	issue := GitHubIssue{Repository: "Owner/Repo", Number: 7, Title: "Bug", Body: "details", State: "open", Revision: "etag-7", UpdatedAt: connectorTestNow}
	reader := &FakeGitHubReadClient{IssuePages: []GitHubIssuePage{
		{Issues: []GitHubIssue{issue, issue}, NextCursor: " next ", HasMore: true},
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
	state, ok := store.Cursor("workspace-1", "owner/repo")
	if !ok || state.Cursor != "next" || !state.LastSynced.Equal(connectorTestNow) {
		t.Fatalf("github checkpoint = %+v, exists=%v", state, ok)
	}
	second, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo", Cursor: first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if second.Seen != 1 || second.Upserted != 0 || second.Skipped != 1 || second.HasMore {
		t.Fatalf("replayed import = %+v", second)
	}
	if store.RevisionCount() != 1 {
		t.Fatalf("revision count = %d, want 1", store.RevisionCount())
	}
	got, err := service.GetIssue(context.Background(), "OWNER/REPO", 7)
	if err != nil || got.Repository != "owner/repo" || got.RevisionKey() != "github:owner/repo:issue:7:etag-7" {
		t.Fatalf("get issue = %+v, err=%v", got, err)
	}
	getRequests := reader.GetRequestsSnapshot()
	if len(getRequests) != 1 || getRequests[0] != "owner/repo#7" {
		t.Fatalf("get requests = %+v", getRequests)
	}
	listRequests := reader.ListRequestsSnapshot()
	if len(listRequests) != 2 || listRequests[0].Repository != "owner/repo" || listRequests[1].Cursor != "next" {
		t.Fatalf("list requests = %+v", listRequests)
	}
}

func TestGitHubImportRevisionDeduplicationIsWorkspaceScoped(t *testing.T) {
	issue := GitHubIssue{Repository: "owner/repo", Number: 7, Title: "Shared issue", Body: "details", State: "open", Revision: "shared-etag", UpdatedAt: connectorTestNow}
	store := NewMemoryGitHubRevisionStore()
	for _, workspaceID := range []string{"workspace-a", "workspace-b"} {
		reader := &FakeGitHubReadClient{IssuePages: []GitHubIssuePage{{Issues: []GitHubIssue{issue}}}}
		service, err := NewGitHubImportService(reader, store)
		if err != nil {
			t.Fatal(err)
		}
		service.Clock = func() time.Time { return connectorTestNow }
		result, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: workspaceID, Repository: "owner/repo"})
		if err != nil || result.Upserted != 1 || result.Skipped != 0 {
			t.Fatalf("workspace %q import=%+v err=%v", workspaceID, result, err)
		}
	}
	if store.RevisionCount() != 2 {
		t.Fatalf("workspace-scoped GitHub revision count=%d, want 2", store.RevisionCount())
	}
	item, err := NewGitHubImportItem(issue)
	if err != nil {
		t.Fatal(err)
	}
	for _, workspaceID := range []string{"workspace-a", "workspace-b"} {
		exists, err := store.HasRevision(context.Background(), workspaceID, item.RevisionKey())
		if err != nil || !exists {
			t.Fatalf("workspace %q GitHub revision exists=%v err=%v", workspaceID, exists, err)
		}
	}
}

func TestGitHubImportFailsClosedAndPreservesCheckpointBoundary(t *testing.T) {
	reader := &FakeGitHubReadClient{IssuePages: []GitHubIssuePage{{Issues: []GitHubIssue{{Repository: "owner/repo", Number: 1}}}}}
	service, err := NewGitHubImportService(reader, NewMemoryGitHubRevisionStore())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"}); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("missing revision error = %v", err)
	}
	for _, request := range []GitHubImportRequest{{}, {WorkspaceID: "workspace-1"}, {WorkspaceID: "workspace-1", Repository: "owner/repo", Cursor: " "}, {WorkspaceID: "workspace-1", Repository: "owner/repo", PageSize: -1}} {
		if _, err := service.SyncIssues(context.Background(), request); err == nil {
			t.Fatalf("invalid github request unexpectedly succeeded: %+v", request)
		}
	}
	if err := (GitHubSyncState{WorkspaceID: "workspace-1", Repository: "owner/repo", HasMore: true}).Validate(); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("invalid github state = %v", err)
	}

	issue := GitHubIssue{Repository: "owner/repo", Number: 1, Revision: "rev-1"}
	for _, setup := range []struct {
		name  string
		store *MemoryGitHubRevisionStore
	}{
		{name: "has revision", store: &MemoryGitHubRevisionStore{HasRevisionError: errors.New("api_key=fake-has-secret")}},
		{name: "put revision", store: &MemoryGitHubRevisionStore{PutRevisionError: errors.New("token=fake-put-secret")}},
		{name: "save cursor", store: &MemoryGitHubRevisionStore{SaveCursorError: errors.New("password=fake-save-secret")}},
	} {
		t.Run(setup.name, func(t *testing.T) {
			if setup.store.Revisions == nil {
				setup.store.Revisions = make(map[string]GitHubImportItem)
			}
			if setup.store.Cursors == nil {
				setup.store.Cursors = make(map[string]GitHubSyncState)
			}
			service, err := NewGitHubImportService(&FakeGitHubReadClient{IssuePages: []GitHubIssuePage{{Issues: []GitHubIssue{issue}}}}, setup.store)
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"})
			if err == nil || strings.Contains(err.Error(), "fake-") {
				t.Fatalf("stage error = %v", err)
			}
			if _, ok := setup.store.Cursor("workspace-1", "owner/repo"); ok && setup.name != "save cursor" {
				t.Fatal("failed import unexpectedly checkpointed")
			}
		})
	}

	badCursorReader := &FakeGitHubReadClient{IssuePages: []GitHubIssuePage{{Issues: []GitHubIssue{issue}, HasMore: true}}}
	service, err = NewGitHubImportService(badCursorReader, NewMemoryGitHubRevisionStore())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("missing github continuation error = %v", err)
	}

	errReader := &FakeGitHubReadClient{ListErrors: map[int]error{0: NewProviderError("github", "list", http.StatusBadGateway, "body token=fake-list-secret")}}
	service.Reader = errReader
	if _, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"}); err == nil || ClassifyRetry(err) != RetryTransient || strings.Contains(err.Error(), "fake-list-secret") {
		t.Fatalf("github provider error = %v, class=%q", err, ClassifyRetry(err))
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.SyncIssues(canceled, GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled import error = %v", err)
	}
}

func TestGitHubImportConstructorAndGetValidation(t *testing.T) {
	if _, err := NewGitHubImportService(nil, NewMemoryGitHubRevisionStore()); err == nil {
		t.Fatal("nil reader should be rejected")
	}
	if _, err := NewGitHubImportService(&FakeGitHubReadClient{}, nil); err == nil {
		t.Fatal("nil store should be rejected")
	}
	service, err := NewGitHubImportService(&FakeGitHubReadClient{}, NewMemoryGitHubRevisionStore())
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range [][2]interface{}{{"bad", int64(1)}, {"owner/repo", int64(0)}} {
		if _, err := service.GetIssue(context.Background(), input[0].(string), input[1].(int64)); !errors.Is(err, ErrInvalidRepository) {
			t.Fatalf("invalid get input=%v error=%v", input, err)
		}
	}
	issue := GitHubIssue{Repository: "owner/repo", Number: 1, Revision: "rev-1"}
	reader := &FakeGitHubReadClient{Issues: map[string]GitHubIssue{"owner/repo#1": issue}, GetErrors: map[string]error{"owner/repo#2": errors.New("authorization api_key=fake-get-secret")}}
	service.Reader = reader
	if _, err := service.GetIssue(context.Background(), "owner/repo", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetIssue(context.Background(), "owner/repo", 2); err == nil || strings.Contains(err.Error(), "fake-get-secret") {
		t.Fatalf("get provider error = %v", err)
	}
	if _, err := service.GetIssue(context.Background(), "owner/repo", 3); !errors.Is(err, ErrNotFound) {
		t.Fatalf("not found error = %v", err)
	}
}

func TestGitHubRevisionValidationAndDedupe(t *testing.T) {
	valid := GitHubImportItem{Repository: "Owner/Repo", Kind: "Issue", ExternalID: 9, Revision: "rev"}
	if err := valid.Validate(); err != nil || valid.RevisionKey() != "github:owner/repo:issue:9:rev" {
		t.Fatalf("valid github item=%+v err=%v", valid, err)
	}
	if err := (GitHubImportItem{Repository: "owner/repo", Kind: "issue", Number: 0, Revision: "rev"}).Validate(); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("invalid github identity=%v", err)
	}
	unique, skipped, err := DeduplicateGitHubItems([]GitHubImportItem{valid, {Repository: "owner/repo", Kind: "issue", ExternalID: 9, Revision: "rev"}})
	if err != nil || len(unique) != 1 || skipped != 1 {
		t.Fatalf("github dedupe items=%d skipped=%d err=%v", len(unique), skipped, err)
	}
	if _, _, err := DeduplicateGitHubItems([]GitHubImportItem{{Repository: "owner/repo", Kind: "issue"}}); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("invalid github dedupe error=%v", err)
	}
	if _, err := NewGitHubImportItem(GitHubIssue{Repository: "owner/repo", Number: 1}); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("invalid issue error=%v", err)
	}
}
