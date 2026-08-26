package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

type testNetError struct{}

func (testNetError) Error() string   { return "network token=fake-network-secret" }
func (testNetError) Timeout() bool   { return true }
func (testNetError) Temporary() bool { return true }

func TestErrorBoundaryAndCompatibilityEdges(t *testing.T) {
	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooEarly, 599, 600, 0} {
		class := ClassifyHTTPStatus(status)
		if status == http.StatusRequestTimeout || status == http.StatusTooEarly || status == 599 {
			if class != RetryTransient {
				t.Fatalf("status %d class=%q", status, class)
			}
		} else if class != RetryNone {
			t.Fatalf("status %d class=%q", status, class)
		}
	}
	if got := ClassifyRetry(testNetError{}); got != RetryTransient {
		t.Fatalf("network class=%q", got)
	}
	if got := ClassifyError(context.DeadlineExceeded); got != RetryCanceled {
		t.Fatalf("deadline class=%q", got)
	}

	withoutCause := WrapProviderError("", "", http.StatusOK, nil)
	if withoutCause.Error() != "provider returned HTTP 200" || withoutCause.Unwrap() != nil {
		t.Fatalf("provider without cause=%q unwrap=%v", withoutCause, withoutCause.Unwrap())
	}
	provider := &ProviderError{StatusCode: 0, detail: "token=fake-private-detail"}
	if strings.Contains(provider.Error(), "fake-private-detail") || !strings.HasPrefix(provider.Error(), "provider") {
		t.Fatalf("empty provider output=%q", provider.Error())
	}
	for _, format := range []string{"%q", "%x", "%X", "%v", "%+v"} {
		if output := fmt.Sprintf(format, provider); strings.Contains(output, "fake-private-detail") {
			t.Fatalf("format %s leaked detail: %q", format, output)
		}
	}
	var nilProvider *ProviderError
	encoded, err := json.Marshal(nilProvider)
	if err != nil || string(encoded) != "null" {
		t.Fatalf("nil provider JSON=%s err=%v", encoded, err)
	}
	if got := RedactErrorMessage("password=fake-password-secret"); strings.Contains(got, "fake-password-secret") {
		t.Fatalf("alias leaked secret=%q", got)
	}

	var nilRedacted *redactedError
	if nilRedacted.Error() != "" || nilRedacted.Unwrap() != nil {
		t.Fatal("nil redacted error mismatch")
	}
	if output := fmt.Sprintf("%+v", nilRedacted); output != "" {
		t.Fatalf("nil redacted format=%q", output)
	}
	encoded, err = json.Marshal(nilRedacted)
	if err != nil || string(encoded) != "null" {
		t.Fatalf("nil redacted JSON=%s err=%v", encoded, err)
	}
	redacted := &redactedError{message: "safe"}
	if output := fmt.Sprintf("%q", redacted); output != `"safe"` {
		t.Fatalf("quoted redacted=%q", output)
	}
	if output := fmt.Sprintf("%x", redacted); output != "safe" {
		t.Fatalf("hex redacted=%q", output)
	}
	if encoded, err := json.Marshal(redacted); err != nil || !strings.Contains(string(encoded), `"error":"safe"`) {
		t.Fatalf("redacted JSON=%s err=%v", encoded, err)
	}
}

func TestFakeConnectorFailureBranches(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (&FakeDriveReader{}).List(ctx, DriveListRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled drive reader=%v", err)
	}
	driveReader := &FakeDriveReader{Errors: map[int]error{0: errors.New("drive fixture error")}}
	if _, err := driveReader.List(context.Background(), DriveListRequest{}); err == nil {
		t.Fatal("drive fixture error was ignored")
	}
	if _, err := (&FakeDriveReader{}).List(context.Background(), DriveListRequest{}); err != nil {
		t.Fatal(err)
	}

	driveStore := NewMemoryDriveRevisionStore()
	item := DriveSourceItem{FileID: "file", RevisionID: "revision"}
	driveStore.HasRevisionError = errors.New("has error")
	if _, err := driveStore.HasRevision(context.Background(), "workspace", item.RevisionKey()); err == nil {
		t.Fatal("drive has error was ignored")
	}
	driveStore.HasRevisionError = nil
	if err := driveStore.PutRevision(context.Background(), "workspace", DriveSourceItem{}); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("invalid drive put=%v", err)
	}
	driveStore.PutRevisionError = errors.New("put error")
	if err := driveStore.PutRevision(context.Background(), "workspace", item); err == nil {
		t.Fatal("drive put error was ignored")
	}
	driveStore.PutRevisionError = nil
	if err := driveStore.PutRevision(context.Background(), "workspace", item); err != nil {
		t.Fatal(err)
	}
	if err := driveStore.PutRevision(context.Background(), "workspace", item); !errors.Is(err, ErrRevisionAlreadyExists) {
		t.Fatalf("duplicate drive put=%v", err)
	}
	if err := driveStore.SaveCursor(context.Background(), DriveSyncState{WorkspaceID: "", HasMore: true}); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("invalid drive cursor state=%v", err)
	}
	if err := driveStore.SaveCursor(context.Background(), DriveSyncState{WorkspaceID: "workspace", HasMore: true}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("missing drive cursor state=%v", err)
	}
	driveStore.SaveCursorError = errors.New("save error")
	if err := driveStore.SaveCursor(context.Background(), DriveSyncState{WorkspaceID: "workspace"}); err == nil {
		t.Fatal("drive save error was ignored")
	}
	if err := driveStore.SaveCursor(ctx, DriveSyncState{WorkspaceID: "workspace"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled drive store=%v", err)
	}

	githubReader := &FakeGitHubReadClient{ListErrors: map[int]error{0: errors.New("list fixture error")}}
	if _, err := githubReader.ListIssues(context.Background(), GitHubIssueListRequest{}); err == nil {
		t.Fatal("github list error was ignored")
	}
	if _, err := (&FakeGitHubReadClient{}).ListIssues(context.Background(), GitHubIssueListRequest{}); err != nil {
		t.Fatal(err)
	}
	githubReader.GetErrors = map[string]error{"owner/repo#1": errors.New("get fixture error")}
	if _, err := githubReader.GetIssue(context.Background(), "owner/repo", 1); err == nil {
		t.Fatal("github get error was ignored")
	}
	if _, err := githubReader.GetIssue(context.Background(), "owner/repo", 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("github missing issue=%v", err)
	}
	if _, err := githubReader.GetIssue(ctx, "owner/repo", 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled github reader=%v", err)
	}

	githubStore := NewMemoryGitHubRevisionStore()
	githubItem := GitHubImportItem{Repository: "owner/repo", Kind: "issue", Number: 1, Revision: "revision"}
	githubStore.HasRevisionError = errors.New("has github error")
	if _, err := githubStore.HasRevision(context.Background(), "workspace", githubItem.RevisionKey()); err == nil {
		t.Fatal("github has error was ignored")
	}
	githubStore.HasRevisionError = nil
	if err := githubStore.PutRevision(context.Background(), "workspace", GitHubImportItem{}); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("invalid github put=%v", err)
	}
	githubStore.PutRevisionError = errors.New("put github error")
	if err := githubStore.PutRevision(context.Background(), "workspace", githubItem); err == nil {
		t.Fatal("github put error was ignored")
	}
	githubStore.PutRevisionError = nil
	if err := githubStore.PutRevision(context.Background(), "workspace", githubItem); err != nil {
		t.Fatal(err)
	}
	if err := githubStore.PutRevision(context.Background(), "workspace", githubItem); !errors.Is(err, ErrRevisionAlreadyExists) {
		t.Fatalf("duplicate github put=%v", err)
	}
	if err := githubStore.SaveCursor(context.Background(), GitHubSyncState{}); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("invalid github workspace=%v", err)
	}
	if err := githubStore.SaveCursor(context.Background(), GitHubSyncState{WorkspaceID: "workspace"}); !errors.Is(err, ErrInvalidRepository) {
		t.Fatalf("invalid github repository=%v", err)
	}
	if err := githubStore.SaveCursor(context.Background(), GitHubSyncState{WorkspaceID: "workspace", Repository: "owner/repo", HasMore: true}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("missing github cursor=%v", err)
	}
	githubStore.SaveCursorError = errors.New("save github error")
	if err := githubStore.SaveCursor(context.Background(), GitHubSyncState{WorkspaceID: "workspace", Repository: "owner/repo"}); err == nil {
		t.Fatal("github save error was ignored")
	}

	writer := &FakeGitHubWriter{Failure: errors.New("writer error")}
	if _, err := writer.CreateIssue(context.Background(), CreateIssueRequest{}); err == nil {
		t.Fatal("writer create error was ignored")
	}
	if _, err := writer.AddIssueComment(context.Background(), CommentIssueRequest{}); err == nil {
		t.Fatal("writer comment error was ignored")
	}
	if _, err := writer.SetIssueLabels(context.Background(), LabelIssueRequest{}); err == nil {
		t.Fatal("writer labels error was ignored")
	}
	writer.Failure = nil
	if _, err := writer.CreateIssue(context.Background(), CreateIssueRequest{Repository: "owner/repo", Title: "title"}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.AddIssueComment(context.Background(), CommentIssueRequest{Repository: "owner/repo", Issue: 1, Body: "body"}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.SetIssueLabels(context.Background(), LabelIssueRequest{Repository: "owner/repo", Issue: 1, Labels: []string{"label"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.CreateIssue(ctx, CreateIssueRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled writer=%v", err)
	}
}

func TestSafeWriteValidationAndStoreEdges(t *testing.T) {
	for _, target := range []SafeWriteTarget{
		{Operation: SafeWriteOperationCreateIssue, Repository: "owner/repo", Issue: 1, Title: "title"},
		{Operation: SafeWriteOperationCreateIssue, Repository: "owner/repo", Title: "title", Labels: []string{"label"}},
		{Operation: SafeWriteOperationAddComment, Repository: "owner/repo", Issue: 1, Body: "body", Title: "title"},
		{Operation: SafeWriteOperationAddComment, Repository: "owner/repo", Issue: 1, Body: "body", Labels: []string{"label"}},
		{Operation: SafeWriteOperationSetLabels, Repository: "owner/repo", Issue: 0, Labels: []string{"label"}},
		{Operation: SafeWriteOperationSetLabels, Repository: "owner/repo", Issue: 1, Body: "body", Labels: []string{"label"}},
		{Operation: SafeWriteOperationSetLabels, Repository: "owner/repo", Issue: 1, Title: "title", Labels: []string{"label"}},
	} {
		if err := target.Validate(); err == nil {
			t.Fatalf("invalid target passed: %+v", target)
		}
	}
	createTarget := SafeWriteTarget{Operation: SafeWriteOperationCreateIssue, Repository: "owner/repo", Title: "title"}
	if typ, id := createTarget.TargetTypeAndID(); typ != "github_repository" || id != "owner/repo" {
		t.Fatalf("create target=%s/%s", typ, id)
	}
	commentTarget := SafeWriteTarget{Operation: SafeWriteOperationAddComment, Repository: "owner/repo", Issue: 3, Body: "body"}
	if typ, id := commentTarget.TargetTypeAndID(); typ != "github_issue" || id != "owner/repo#3" {
		t.Fatalf("comment target=%s/%s", typ, id)
	}
	for _, request := range []interface{ Validate() error }{
		CreateIssueRequest{Repository: "owner/repo", Title: "title"},
		CommentIssueRequest{Repository: "owner/repo", Issue: 1, Body: "body"},
		LabelIssueRequest{Repository: "owner/repo", Issue: 1},
	} {
		if err := request.Validate(); !errors.Is(err, ErrMissingIdempotencyKey) {
			t.Fatalf("missing metadata error=%v", err)
		}
	}
	if _, err := NewGitHubSafeWriteService(nil, nil, nil); err == nil {
		t.Fatal("nil safe write dependencies accepted")
	}
	if _, err := (&GitHubSafeWriteService{}).CreateIssue(context.Background(), CreateIssueRequest{Repository: "owner/repo", Title: "title", Metadata: SafeWriteMetadata{IdempotencyKey: "key", UserID: "user"}}); err == nil {
		t.Fatal("zero safe write service accepted")
	}

	valid, err := NewSafeWriteChallenge("edge-challenge", "workspace-1", "user", createTarget, connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	challengeStore := NewMemorySafeWriteChallengeStore()
	if err := challengeStore.Put(valid); err != nil {
		t.Fatal(err)
	}
	wrongUser := valid.Confirmation(connectorTestNow)
	wrongUser.UserID = "other"
	if err := challengeStore.Consume(context.Background(), wrongUser, createTarget, connectorTestNow); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("wrong user consume=%v", err)
	}
	wrongWorkspace := valid.Confirmation(connectorTestNow)
	wrongWorkspace.WorkspaceID = "workspace-2"
	if err := challengeStore.Consume(context.Background(), wrongWorkspace, createTarget, connectorTestNow); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("wrong workspace consume=%v", err)
	}
	wrongExpiry := valid.Confirmation(connectorTestNow)
	wrongExpiry.ExpiresAt = wrongExpiry.ExpiresAt.Add(-time.Second)
	if err := challengeStore.Consume(context.Background(), wrongExpiry, createTarget, connectorTestNow); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("wrong expiry consume=%v", err)
	}
	oldConfirmation := valid.Confirmation(connectorTestNow.Add(-time.Second))
	if err := challengeStore.Consume(context.Background(), oldConfirmation, createTarget, connectorTestNow); !errors.Is(err, ErrChallengeExpired) {
		t.Fatalf("old confirmation consume=%v", err)
	}
	if _, err := NewSafeWriteChallenge("", "workspace-1", "user", createTarget, connectorTestNow); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("empty challenge id=%v", err)
	}
	if _, err := NewSafeWriteChallenge("id", "workspace-1", "", createTarget, connectorTestNow); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("empty challenge user=%v", err)
	}

	receipts := NewMemorySafeWriteReceiptStore()
	receipt := SafeWriteReceipt{ID: "id", WorkspaceID: "workspace-1", UserID: "user", Operation: SafeWriteOperationCreateIssue, IdempotencyKey: "key", ActionHash: "hash", TargetType: "github_repository", TargetID: "owner/repo", Status: SafeWriteReceiptAccepted, CreatedAt: connectorTestNow}
	if err := receipts.Save(context.Background(), receipt); err != nil {
		t.Fatal(err)
	}
	if err := receipts.Save(context.Background(), receipt); err != nil {
		t.Fatalf("same receipt replay=%v", err)
	}
	if _, _, err := receipts.Lookup(context.Background(), "workspace-1", "user", SafeWriteOperationCreateIssue, "missing"); err != nil {
		t.Fatal(err)
	}
	if err := receipts.Save(context.Background(), receipt); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := receipts.Lookup(ctx, "workspace-1", "user", SafeWriteOperationCreateIssue, "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled receipt lookup=%v", err)
	}
	if err := receipts.Save(ctx, receipt); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled receipt save=%v", err)
	}
}

func TestSyncNilClockAndFailureBoundaryEdges(t *testing.T) {
	item := DriveSourceItem{FileID: "file", RevisionID: "revision"}
	driveService, err := NewDriveSyncService(&FakeDriveReader{Pages: []DrivePage{{Items: []DriveSourceItem{item}}}}, NewMemoryDriveRevisionStore())
	if err != nil {
		t.Fatal(err)
	}
	result, err := driveService.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace", PageSize: 7})
	if err != nil || result.Upserted != 1 {
		t.Fatalf("drive nil clock result=%+v err=%v", result, err)
	}
	var nilDrive *DriveSyncService
	if _, err := nilDrive.Sync(context.Background(), DriveSyncRequest{WorkspaceID: "workspace"}); err == nil {
		t.Fatal("nil drive service accepted")
	}

	issue := GitHubIssue{Repository: "owner/repo", Number: 1, Revision: "revision"}
	githubService, err := NewGitHubImportService(&FakeGitHubReadClient{IssuePages: []GitHubIssuePage{{Issues: []GitHubIssue{issue}}}}, NewMemoryGitHubRevisionStore())
	if err != nil {
		t.Fatal(err)
	}
	result2, err := githubService.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace", Repository: "owner/repo", PageSize: 7})
	if err != nil || result2.Upserted != 1 {
		t.Fatalf("github nil clock result=%+v err=%v", result2, err)
	}
	var nilGitHub *GitHubImportService
	if _, err := nilGitHub.SyncIssues(context.Background(), GitHubImportRequest{WorkspaceID: "workspace", Repository: "owner/repo"}); err == nil {
		t.Fatal("nil github service accepted")
	}
	if _, err := nilGitHub.GetIssue(context.Background(), "owner/repo", 1); err == nil {
		t.Fatal("nil github detail service accepted")
	}
	if got := normalizedPageSize(7); got != 7 {
		t.Fatalf("page size=%d", got)
	}
	if err := wrapConnectorFailure("provider", "operation", nil); err != nil {
		t.Fatal(err)
	}
}
