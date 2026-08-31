package connectors

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type driveSyncStoreDecorator struct {
	*MemoryDriveRevisionStore
	loadErr     error
	loadState   DriveSyncState
	loadFound   bool
	startErr    error
	completeErr error
	removeErr   error
}

func (s *driveSyncStoreDecorator) LoadCursor(ctx context.Context, workspaceID string) (DriveSyncState, bool, error) {
	if s.loadErr != nil {
		return DriveSyncState{}, false, s.loadErr
	}
	if s.loadFound {
		return s.loadState, true, nil
	}
	return s.MemoryDriveRevisionStore.LoadCursor(ctx, workspaceID)
}

func (s *driveSyncStoreDecorator) StartSyncRun(ctx context.Context, provider, workspaceID, target, cursorBefore string, startedAt time.Time) (string, error) {
	if s.startErr != nil {
		return "", s.startErr
	}
	return s.MemoryDriveRevisionStore.StartSyncRun(ctx, provider, workspaceID, target, cursorBefore, startedAt)
}

func (s *driveSyncStoreDecorator) CompleteSyncRun(ctx context.Context, runID, status string, summary SyncRunSummary, cursorAfter, errorCode string, completedAt time.Time) error {
	if s.completeErr != nil {
		return s.completeErr
	}
	return s.MemoryDriveRevisionStore.CompleteSyncRun(ctx, runID, status, summary, cursorAfter, errorCode, completedAt)
}

func (s *driveSyncStoreDecorator) MarkRemoved(ctx context.Context, fileID string, removedAt time.Time) error {
	if s.removeErr != nil {
		return s.removeErr
	}
	return s.MemoryDriveRevisionStore.MarkRemoved(ctx, fileID, removedAt)
}

type githubSyncStoreDecorator struct {
	*MemoryGitHubRevisionStore
	loadErr     error
	loadState   GitHubSyncState
	loadFound   bool
	startErr    error
	completeErr error
}

func (s *githubSyncStoreDecorator) LoadCursor(ctx context.Context, workspaceID, repository string) (GitHubSyncState, bool, error) {
	if s.loadErr != nil {
		return GitHubSyncState{}, false, s.loadErr
	}
	if s.loadFound {
		return s.loadState, true, nil
	}
	return s.MemoryGitHubRevisionStore.LoadCursor(ctx, workspaceID, repository)
}

func (s *githubSyncStoreDecorator) StartSyncRun(ctx context.Context, provider, workspaceID, target, cursorBefore string, startedAt time.Time) (string, error) {
	if s.startErr != nil {
		return "", s.startErr
	}
	return s.MemoryGitHubRevisionStore.StartSyncRun(ctx, provider, workspaceID, target, cursorBefore, startedAt)
}

func (s *githubSyncStoreDecorator) CompleteSyncRun(ctx context.Context, runID, status string, summary SyncRunSummary, cursorAfter, errorCode string, completedAt time.Time) error {
	if s.completeErr != nil {
		return s.completeErr
	}
	return s.MemoryGitHubRevisionStore.CompleteSyncRun(ctx, runID, status, summary, cursorAfter, errorCode, completedAt)
}

type driveRevisionOnlyStore struct{ DriveRevisionStore }

func TestDriveSyncFailureBoundaries(t *testing.T) {
	base := NewMemoryDriveRevisionStore()
	if _, err := NewDriveSyncService(nil, base); err == nil {
		t.Fatal("nil Drive reader was accepted")
	}
	if _, err := NewDriveSyncService(&FakeDriveReader{}, nil); err == nil {
		t.Fatal("nil Drive store was accepted")
	}

	service, err := NewDriveSyncService(&FakeDriveReader{}, base)
	if err != nil {
		t.Fatal(err)
	}
	for name, request := range map[string]DriveSyncRequest{
		"missing workspace":      {Cursor: DriveCursor{}},
		"non-empty blank cursor": {WorkspaceID: "workspace-1", Cursor: DriveCursor{Token: " "}},
		"negative page size":     {WorkspaceID: "workspace-1", PageSize: -1},
		"oversized page size":    {WorkspaceID: "workspace-1", PageSize: maxConnectorPageSize + 1},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := service.Sync(context.Background(), request)
			if err == nil {
				t.Fatal("invalid request was accepted")
			}
		})
	}

	loadFailure := &driveSyncStoreDecorator{MemoryDriveRevisionStore: NewMemoryDriveRevisionStore(), loadErr: errors.New("cursor store unavailable")}
	service.Reader = &FakeDriveReader{}
	service.Store = loadFailure
	if _, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"}); err == nil || !strings.Contains(err.Error(), "load cursor") {
		t.Fatalf("load cursor failure = %v", err)
	}

	invalidState := &driveSyncStoreDecorator{
		MemoryDriveRevisionStore: NewMemoryDriveRevisionStore(),
		loadState:                DriveSyncState{WorkspaceID: "other-workspace", HasMore: true, Cursor: DriveCursor{Token: "next"}},
		loadFound:                true,
	}
	service.Store = invalidState
	if _, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"}); !errors.Is(err, ErrInvalidSyncState) {
		t.Fatalf("invalid checkpoint = %v", err)
	}

	startFailure := &driveSyncStoreDecorator{MemoryDriveRevisionStore: NewMemoryDriveRevisionStore(), startErr: errors.New("audit store unavailable")}
	service.Store = startFailure
	if _, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"}); err == nil || !strings.Contains(err.Error(), "start sync run") {
		t.Fatalf("start sync failure = %v", err)
	}

	completeFailure := &driveSyncStoreDecorator{MemoryDriveRevisionStore: NewMemoryDriveRevisionStore(), completeErr: errors.New("audit completion unavailable")}
	service.Reader = &FakeDriveReader{Pages: []DrivePage{{HasMore: false}}}
	service.Store = completeFailure
	if _, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"}); err == nil || !strings.Contains(err.Error(), "complete sync run") {
		t.Fatalf("complete sync failure = %v", err)
	}

	removalUnsupported := NewMemoryDriveRevisionStore()
	service.Reader = &FakeDriveReader{Pages: []DrivePage{{Items: []DriveSourceItem{{FileID: "removed-file", RevisionID: "removed", Removed: true}}}}}
	service.Store = driveRevisionOnlyStore{DriveRevisionStore: removalUnsupported}
	if _, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"}); !errors.Is(err, ErrRemovalNotSupported) {
		t.Fatalf("unsupported removal = %v", err)
	}

	removalFailure := &driveSyncStoreDecorator{MemoryDriveRevisionStore: NewMemoryDriveRevisionStore(), removeErr: errors.New("removal persistence unavailable")}
	service.Reader = &FakeDriveReader{Pages: []DrivePage{{Items: []DriveSourceItem{{FileID: "removed-file", RevisionID: "removed", Removed: true}}}}}
	service.Store = removalFailure
	if _, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"}); err == nil || !strings.Contains(err.Error(), "mark removed") {
		t.Fatalf("removal failure = %v", err)
	}

	hasFailure := NewMemoryDriveRevisionStore()
	hasFailure.HasRevisionError = errors.New("revision lookup unavailable")
	service.Reader = &FakeDriveReader{Pages: []DrivePage{{Items: []DriveSourceItem{{FileID: "file-1", RevisionID: "revision-1"}}}}}
	service.Store = hasFailure
	if _, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"}); err == nil || !strings.Contains(err.Error(), "check revision") {
		t.Fatalf("revision lookup failure = %v", err)
	}

	saveFailure := NewMemoryDriveRevisionStore()
	saveFailure.SaveCursorError = errors.New("cursor persistence unavailable")
	service.Store = saveFailure
	if _, err := service.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace-1"}); err == nil || !strings.Contains(err.Error(), "save cursor") {
		t.Fatalf("cursor save failure = %v", err)
	}
}

func TestGitHubImportFailureBoundaries(t *testing.T) {
	base := NewMemoryGitHubRevisionStore()
	if _, err := NewGitHubImportService(nil, base); err == nil {
		t.Fatal("nil GitHub reader was accepted")
	}
	if _, err := NewGitHubImportService(&FakeGitHubReadClient{}, nil); err == nil {
		t.Fatal("nil GitHub store was accepted")
	}

	service, err := NewGitHubImportService(&FakeGitHubReadClient{}, base)
	if err != nil {
		t.Fatal(err)
	}
	for name, request := range map[string]GitHubImportRequest{
		"missing workspace":      {Repository: "owner/repo"},
		"non-empty blank cursor": {WorkspaceID: "workspace-1", Repository: "owner/repo", Cursor: " "},
		"negative page size":     {WorkspaceID: "workspace-1", Repository: "owner/repo", PageSize: -1},
		"oversized page size":    {WorkspaceID: "workspace-1", Repository: "owner/repo", PageSize: maxConnectorPageSize + 1},
		"invalid repository":     {WorkspaceID: "workspace-1", Repository: "not-a-repository"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := service.SyncIssues(context.Background(), request)
			if err == nil {
				t.Fatal("invalid request was accepted")
			}
		})
	}

	loadFailure := &githubSyncStoreDecorator{MemoryGitHubRevisionStore: NewMemoryGitHubRevisionStore(), loadErr: errors.New("cursor store unavailable")}
	service.Store = loadFailure
	if _, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"}); err == nil || !strings.Contains(err.Error(), "load cursor") {
		t.Fatalf("load cursor failure = %v", err)
	}

	invalidState := &githubSyncStoreDecorator{
		MemoryGitHubRevisionStore: NewMemoryGitHubRevisionStore(),
		loadState:                 GitHubSyncState{WorkspaceID: "other-workspace", Repository: "owner/repo", HasMore: true},
		loadFound:                 true,
	}
	service.Store = invalidState
	if _, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"}); !errors.Is(err, ErrInvalidSyncState) {
		t.Fatalf("invalid checkpoint = %v", err)
	}

	startFailure := &githubSyncStoreDecorator{MemoryGitHubRevisionStore: NewMemoryGitHubRevisionStore(), startErr: errors.New("audit store unavailable")}
	service.Store = startFailure
	if _, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"}); err == nil || !strings.Contains(err.Error(), "start sync run") {
		t.Fatalf("start sync failure = %v", err)
	}

	completeFailure := &githubSyncStoreDecorator{MemoryGitHubRevisionStore: NewMemoryGitHubRevisionStore(), completeErr: errors.New("audit completion unavailable")}
	service.Reader = &FakeGitHubReadClient{IssuePages: []GitHubIssuePage{{}}}
	service.Store = completeFailure
	if _, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"}); err == nil || !strings.Contains(err.Error(), "complete sync run") {
		t.Fatalf("complete sync failure = %v", err)
	}

	invalidIssue := NewMemoryGitHubRevisionStore()
	service.Reader = &FakeGitHubReadClient{IssuePages: []GitHubIssuePage{{Issues: []GitHubIssue{{Repository: "owner/repo", Number: 1}}}}}
	service.Store = invalidIssue
	if _, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"}); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("invalid issue revision = %v", err)
	}

	hasFailure := NewMemoryGitHubRevisionStore()
	hasFailure.HasRevisionError = errors.New("revision lookup unavailable")
	service.Reader = &FakeGitHubReadClient{IssuePages: []GitHubIssuePage{{Issues: []GitHubIssue{{Repository: "owner/repo", Number: 1, Revision: "etag-1"}}}}}
	service.Store = hasFailure
	if _, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"}); err == nil || !strings.Contains(err.Error(), "check revision") {
		t.Fatalf("revision lookup failure = %v", err)
	}

	saveFailure := NewMemoryGitHubRevisionStore()
	saveFailure.SaveCursorError = errors.New("cursor persistence unavailable")
	service.Store = saveFailure
	if _, err := service.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace-1", Repository: "owner/repo"}); err == nil || !strings.Contains(err.Error(), "save cursor") {
		t.Fatalf("cursor save failure = %v", err)
	}

	reader := &FakeGitHubReadClient{
		Issues: map[string]GitHubIssue{
			"owner/repo#1": {Repository: "owner/repo", Number: 1, Revision: "etag-1"},
			"owner/repo#2": {Number: 2, Revision: "etag-2"},
		},
		GetErrors: map[string]error{"owner/repo#3": errors.New("provider detail")},
	}
	service.Reader = reader
	if _, err := service.GetIssue(context.Background(), "bad", 1); !errors.Is(err, ErrInvalidRepository) {
		t.Fatalf("invalid GetIssue repository = %v", err)
	}
	if _, err := service.GetIssue(context.Background(), "owner/repo", 0); !errors.Is(err, ErrInvalidRepository) {
		t.Fatalf("invalid GetIssue number = %v", err)
	}
	if _, err := service.GetIssue(context.Background(), "owner/repo", 3); err == nil || !strings.Contains(err.Error(), "get issue") {
		t.Fatalf("provider GetIssue failure = %v", err)
	}
	if _, err := service.GetIssue(context.Background(), "owner/repo", 9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing GetIssue = %v", err)
	}
	if _, err := service.GetIssue(context.Background(), "owner/repo", 2); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("invalid provider GetIssue payload = %v", err)
	}
}
