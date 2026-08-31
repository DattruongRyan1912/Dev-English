package connectors

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryConnectorFixturesExerciseFailureAndCloneBoundaries(t *testing.T) {
	ctx := context.Background()
	canceled, cancel := context.WithCancel(ctx)
	cancel()

	reader := &FakeDriveReader{
		Pages:  []DrivePage{{Items: []DriveSourceItem{{FileID: "file-1", RevisionID: "rev-1"}}}},
		Errors: map[int]error{1: errors.New("drive read failed")},
	}
	page, err := reader.List(ctx, DriveListRequest{WorkspaceID: "workspace-1"})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("fixture Drive page = %+v, err=%v", page, err)
	}
	page.Items[0].Name = "mutated caller copy"
	if got, err := reader.List(ctx, DriveListRequest{}); err == nil || got.Items != nil {
		t.Fatalf("fixture Drive error = %+v, err=%v", got, err)
	}
	if _, err := reader.List(canceled, DriveListRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Drive fixture = %v", err)
	}
	if got := reader.RequestsSnapshot(); len(got) != 2 {
		t.Fatalf("Drive request snapshot length = %d, want 2", len(got))
	}

	drive := &MemoryDriveRevisionStore{}
	if _, err := drive.StartSyncRun(canceled, ProviderGoogleDrive, "workspace-1", "", "", time.Time{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Drive run = %v", err)
	}
	if _, err := drive.StartSyncRun(ctx, ProviderGoogleDrive, "", "", "", time.Time{}); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("invalid Drive run workspace = %v", err)
	}
	runID, err := drive.StartSyncRun(ctx, ProviderGoogleDrive, "workspace-1", "", "", time.Time{})
	if err != nil || runID == "" {
		t.Fatalf("Drive run = %q, err=%v", runID, err)
	}
	if err := drive.CompleteSyncRun(canceled, runID, "failed", SyncRunSummary{}, "", "canceled", time.Time{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Drive completion = %v", err)
	}
	if err := drive.CompleteSyncRun(ctx, "missing", "failed", SyncRunSummary{}, "", "", time.Time{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing Drive completion = %v", err)
	}
	if err := drive.CompleteSyncRun(ctx, runID, "failed", SyncRunSummary{Seen: 2}, "done", "provider", time.Time{}); err != nil {
		t.Fatalf("Drive completion = %v", err)
	}
	if err := drive.CompleteSyncRun(ctx, runID, "succeeded", SyncRunSummary{}, "again", "", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("replayed Drive completion = %v", err)
	}

	item := DriveSourceItem{FileID: "file-1", RevisionID: "rev-1", Text: "source"}
	if _, err := drive.HasRevision(canceled, item.RevisionKey()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Drive HasRevision = %v", err)
	}
	drive.HasRevisionError = errors.New("lookup failed")
	if _, err := drive.HasRevision(ctx, item.RevisionKey()); err == nil {
		t.Fatal("configured Drive HasRevision error was ignored")
	}
	drive.HasRevisionError = nil
	if err := drive.PutRevision(ctx, DriveSourceItem{}); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("invalid Drive revision = %v", err)
	}
	drive.PutRevisionError = errors.New("put failed")
	if err := drive.PutRevision(ctx, item); err == nil {
		t.Fatal("configured Drive PutRevision error was ignored")
	}
	drive.PutRevisionError = nil
	if err := drive.PutRevision(ctx, item); err != nil {
		t.Fatalf("Drive PutRevision = %v", err)
	}
	if err := drive.PutRevision(ctx, item); !errors.Is(err, ErrRevisionAlreadyExists) {
		t.Fatalf("duplicate Drive revision = %v", err)
	}
	if err := drive.MarkRemoved(canceled, "file-1", time.Time{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Drive removal = %v", err)
	}
	if err := drive.MarkRemoved(ctx, " ", time.Time{}); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("invalid Drive removal = %v", err)
	}
	if err := drive.MarkRemoved(ctx, "file-1", time.Time{}); err != nil {
		t.Fatalf("Drive removal = %v", err)
	}
	if removedAt, ok := drive.RemovedAt(" file-1 "); !ok || removedAt.IsZero() {
		t.Fatalf("Drive removal marker = %v/%t", removedAt, ok)
	}
	if err := drive.SaveCursor(ctx, DriveSyncState{}); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("invalid Drive cursor = %v", err)
	}
	drive.SaveCursorError = errors.New("cursor failed")
	if err := drive.SaveCursor(ctx, DriveSyncState{WorkspaceID: "workspace-1"}); err == nil {
		t.Fatal("configured Drive SaveCursor error was ignored")
	}
	drive.SaveCursorError = nil
	if err := drive.SaveCursor(ctx, DriveSyncState{WorkspaceID: "workspace-1", Cursor: DriveCursor{Token: "next"}}); err != nil {
		t.Fatalf("Drive SaveCursor = %v", err)
	}
	if _, _, err := drive.LoadCursor(canceled, "workspace-1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Drive LoadCursor = %v", err)
	}
	if _, _, err := drive.LoadCursor(ctx, " "); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("invalid Drive LoadCursor = %v", err)
	}
	if state, found, err := drive.LoadCursor(ctx, "workspace-1"); err != nil || !found || state.Cursor.Token != "next" {
		t.Fatalf("Drive LoadCursor = %+v/%t, err=%v", state, found, err)
	}
	if drive.RevisionCount() != 1 || drive.SyncRunCount() != 1 {
		t.Fatalf("Drive fixture counts = revisions:%d runs:%d", drive.RevisionCount(), drive.SyncRunCount())
	}

	githubReader := &FakeGitHubReadClient{
		IssuePages: []GitHubIssuePage{{Issues: []GitHubIssue{{Repository: "owner/repo", Number: 1}}}},
		ListErrors: map[int]error{1: errors.New("GitHub list failed")},
		Issues:     map[string]GitHubIssue{"owner/repo#1": {Repository: "owner/repo", Number: 1}},
		GetErrors:  map[string]error{"owner/repo#2": errors.New("GitHub get failed")},
	}
	if page, err := githubReader.ListIssues(ctx, GitHubIssueListRequest{Repository: "owner/repo"}); err != nil || len(page.Issues) != 1 {
		t.Fatalf("fixture GitHub page = %+v, err=%v", page, err)
	}
	if _, err := githubReader.ListIssues(ctx, GitHubIssueListRequest{}); err == nil {
		t.Fatal("configured GitHub list error was ignored")
	}
	if _, err := githubReader.ListIssues(canceled, GitHubIssueListRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled GitHub list = %v", err)
	}
	if _, err := githubReader.GetIssue(ctx, "owner/repo", 2); err == nil {
		t.Fatal("configured GitHub get error was ignored")
	}
	if _, err := githubReader.GetIssue(ctx, "owner/repo", 9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing GitHub issue = %v", err)
	}
	if _, err := githubReader.GetIssue(canceled, "owner/repo", 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled GitHub get = %v", err)
	}
	if len(githubReader.ListRequestsSnapshot()) != 2 {
		t.Fatalf("GitHub list snapshot length = %d", len(githubReader.ListRequestsSnapshot()))
	}

	github := &MemoryGitHubRevisionStore{}
	if _, err := github.StartSyncRun(ctx, ProviderGitHub, "", "owner/repo", "", time.Time{}); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("invalid GitHub run workspace = %v", err)
	}
	githubRun, err := github.StartSyncRun(ctx, ProviderGitHub, "workspace-1", "owner/repo", "", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if err := github.CompleteSyncRun(ctx, githubRun, "succeeded", SyncRunSummary{}, "done", "", time.Time{}); err != nil {
		t.Fatalf("GitHub completion = %v", err)
	}
	githubIssue := GitHubImportItem{Repository: "owner/repo", Kind: "issue", Number: 1, Revision: "etag-1", Title: "Issue"}
	if err := github.PutRevision(ctx, githubIssue); err != nil {
		t.Fatalf("GitHub PutRevision = %v", err)
	}
	if err := github.PutRevision(ctx, githubIssue); !errors.Is(err, ErrRevisionAlreadyExists) {
		t.Fatalf("duplicate GitHub revision = %v", err)
	}
	github.HasRevisionError = errors.New("GitHub lookup failed")
	if _, err := github.HasRevision(ctx, githubIssue.RevisionKey()); err == nil {
		t.Fatal("configured GitHub HasRevision error was ignored")
	}
	github.HasRevisionError = nil
	github.PutRevisionError = errors.New("GitHub put failed")
	if err := github.PutRevision(ctx, GitHubImportItem{Repository: "owner/repo", Kind: "issue", Number: 2, Revision: "etag-2"}); err == nil {
		t.Fatal("configured GitHub PutRevision error was ignored")
	}
	github.PutRevisionError = nil
	if err := github.SaveCursor(ctx, GitHubSyncState{WorkspaceID: "workspace-1", Repository: " "}); !errors.Is(err, ErrInvalidRepository) {
		t.Fatalf("invalid GitHub cursor = %v", err)
	}
	github.SaveCursorError = errors.New("GitHub cursor failed")
	if err := github.SaveCursor(ctx, GitHubSyncState{WorkspaceID: "workspace-1", Repository: "owner/repo"}); err == nil {
		t.Fatal("configured GitHub SaveCursor error was ignored")
	}
	github.SaveCursorError = nil
	if err := github.SaveCursor(ctx, GitHubSyncState{WorkspaceID: "workspace-1", Repository: "Owner/Repo", Cursor: "next", HasMore: true}); err != nil {
		t.Fatalf("GitHub SaveCursor = %v", err)
	}
	if state, found := github.Cursor("workspace-1", "OWNER/REPO"); !found || state.Cursor != "next" {
		t.Fatalf("GitHub cursor helper = %+v/%t", state, found)
	}
	if _, _, err := github.LoadCursor(canceled, "workspace-1", "owner/repo"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled GitHub LoadCursor = %v", err)
	}
	if _, _, err := github.LoadCursor(ctx, " ", "owner/repo"); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("invalid GitHub LoadCursor workspace = %v", err)
	}
	if _, _, err := github.LoadCursor(ctx, "workspace-1", " "); !errors.Is(err, ErrInvalidRepository) {
		t.Fatalf("invalid GitHub LoadCursor repository = %v", err)
	}
	if state, found, err := github.LoadCursor(ctx, "workspace-1", "owner/repo"); err != nil || !found || state.Cursor != "next" {
		t.Fatalf("GitHub LoadCursor = %+v/%t, err=%v", state, found, err)
	}
	if github.RevisionCount() != 1 || github.SyncRunCount() != 1 {
		t.Fatalf("GitHub fixture counts = revisions:%d runs:%d", github.RevisionCount(), github.SyncRunCount())
	}
}

func TestFakeGitHubWriterAndMemorySafeWriteFixturesCoverBoundaries(t *testing.T) {
	ctx := context.Background()
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	request, challenge := newConfirmedCreateRequest(t, "fixture-key", "fixture-challenge", "Fixture issue")

	writer := &FakeGitHubWriter{NextIssueNumber: 7}
	if _, err := writer.CreateIssue(canceled, request); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled fake issue = %v", err)
	}
	issue, err := writer.CreateIssue(ctx, request)
	if err != nil || issue.Number != 7 || issue.State != "open" {
		t.Fatalf("fake issue = %+v, err=%v", issue, err)
	}
	commentRequest := CommentIssueRequest{Repository: "owner/repo", Issue: 7, Body: "comment"}
	if _, err := writer.AddIssueComment(canceled, commentRequest); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled fake comment = %v", err)
	}
	comment, err := writer.AddIssueComment(ctx, commentRequest)
	if err != nil || comment.ID < 1 {
		t.Fatalf("fake comment = %+v, err=%v", comment, err)
	}
	labelRequest := LabelIssueRequest{Repository: "owner/repo", Issue: 7, Labels: []string{"bug", "urgent"}}
	if _, err := writer.SetIssueLabels(canceled, labelRequest); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled fake labels = %v", err)
	}
	labels, err := writer.SetIssueLabels(ctx, labelRequest)
	if err != nil || len(labels) != 2 || labels[0].Name != "bug" {
		t.Fatalf("fake labels = %+v, err=%v", labels, err)
	}
	failingWriter := &FakeGitHubWriter{Failure: errors.New("provider failed")}
	if _, err := failingWriter.CreateIssue(ctx, request); err == nil {
		t.Fatal("fake writer failure was ignored")
	}
	if _, err := failingWriter.AddIssueComment(ctx, commentRequest); err == nil {
		t.Fatal("fake comment failure was ignored")
	}
	if _, err := failingWriter.SetIssueLabels(ctx, labelRequest); err == nil {
		t.Fatal("fake labels failure was ignored")
	}

	challenges := NewMemorySafeWriteChallengeStore()
	if err := challenges.Put(SafeWriteChallenge{}); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("invalid memory challenge = %v", err)
	}
	if err := challenges.Put(challenge); err != nil {
		t.Fatalf("memory challenge Put = %v", err)
	}
	if _, err := challenges.Get(canceled, challenge.WorkspaceID, challenge.UserID, challenge.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled memory challenge Get = %v", err)
	}
	if _, err := challenges.Get(ctx, challenge.WorkspaceID, "other-user", challenge.ID); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("cross-user memory challenge Get = %v", err)
	}
	if err := challenges.Consume(ctx, challenge.Confirmation(connectorTestNow), request.Target(), connectorTestNow); err != nil {
		t.Fatalf("memory challenge Consume = %v", err)
	}
	if err := challenges.Consume(ctx, challenge.Confirmation(connectorTestNow), request.Target(), connectorTestNow); !errors.Is(err, ErrChallengeUsed) {
		t.Fatalf("replayed memory challenge = %v", err)
	}

	receipts := NewMemorySafeWriteReceiptStore()
	if _, found, err := receipts.Lookup(canceled, "workspace-1", "user-1", SafeWriteOperationCreateIssue, "key"); !errors.Is(err, context.Canceled) || found {
		t.Fatalf("canceled receipt Lookup = found:%t err:%v", found, err)
	}
	valid := SafeWriteReceipt{ID: "receipt-fixture", WorkspaceID: "workspace-1", UserID: "user-1", Operation: SafeWriteOperationCreateIssue, IdempotencyKey: "key", ActionHash: "hash", Status: SafeWriteReceiptPending, Labels: []GitHubLabel{{Name: "bug"}}}
	if _, existed, err := receipts.Reserve(ctx, valid); err != nil || existed {
		t.Fatalf("receipt Reserve = existed:%t err:%v", existed, err)
	}
	if _, existed, err := receipts.Reserve(ctx, valid); err != nil || !existed {
		t.Fatalf("receipt replay Reserve = existed:%t err:%v", existed, err)
	}
	if err := receipts.Complete(ctx, SafeWriteReceipt{ID: "wrong", WorkspaceID: valid.WorkspaceID, UserID: valid.UserID, Operation: valid.Operation, IdempotencyKey: valid.IdempotencyKey, ActionHash: valid.ActionHash, Status: SafeWriteReceiptAccepted}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("receipt identity mismatch = %v", err)
	}
	accepted := valid
	accepted.Status = SafeWriteReceiptAccepted
	accepted.Issue = issue
	if err := receipts.Complete(ctx, accepted); err != nil {
		t.Fatalf("receipt Complete = %v", err)
	}
	if err := receipts.Complete(ctx, accepted); err != nil {
		t.Fatalf("idempotent receipt Complete = %v", err)
	}
	if got, found, err := receipts.Lookup(ctx, valid.WorkspaceID, valid.UserID, valid.Operation, valid.IdempotencyKey); err != nil || !found || got.Issue.Number != issue.Number {
		t.Fatalf("receipt Lookup = %+v/%t, err=%v", got, found, err)
	}
	if err := receipts.Save(ctx, accepted); err != nil {
		t.Fatalf("receipt Save replay = %v", err)
	}
	conflicting := accepted
	conflicting.ActionHash = "other-hash"
	if err := receipts.Save(ctx, conflicting); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("receipt Save conflict = %v", err)
	}
	receipts.ReserveError = errors.New("reserve failed")
	if _, _, err := receipts.Reserve(ctx, SafeWriteReceipt{ID: "other", UserID: "user-1", IdempotencyKey: "other", ActionHash: "other", Status: SafeWriteReceiptPending}); err == nil {
		t.Fatal("configured receipt Reserve error was ignored")
	}
	receipts.ReserveError = nil
	receipts.CompleteError = errors.New("complete failed")
	if err := receipts.Complete(ctx, accepted); err == nil {
		t.Fatal("configured receipt Complete error was ignored")
	}
	receipts.CompleteError = nil
	receipts.SaveError = errors.New("save failed")
	other := accepted
	other.ID = "other-receipt"
	other.IdempotencyKey = "other-key"
	if err := receipts.Save(ctx, other); err == nil {
		t.Fatal("configured receipt Save error was ignored")
	}
	receipts.SaveError = nil
	if err := receipts.Save(canceled, accepted); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled receipt Save = %v", err)
	}
	if _, _, err := receipts.Lookup(ctx, "missing", "user-1", valid.Operation, "missing"); err != nil {
		t.Fatalf("missing receipt Lookup = %v", err)
	}
}
