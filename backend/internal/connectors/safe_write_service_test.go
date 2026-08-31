package connectors

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSafeWriteChallengeBindsActionAndReplayReturnsReceipt(t *testing.T) {
	request := CreateIssueRequest{Repository: "Owner/Repo", Title: "Fix sync", Body: "details", Metadata: SafeWriteMetadata{IdempotencyKey: "request-1", UserID: "user-1"}}
	challenge, err := NewSafeWriteChallenge("challenge-1", "user-1", request.Target(), connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	request.Metadata = challenge.Confirmation(connectorTestNow.Add(time.Second))
	request.Metadata.IdempotencyKey = "request-1"

	writer := &FakeGitHubWriter{NextIssueNumber: 42}
	challenges := NewMemorySafeWriteChallengeStore()
	if err := challenges.Put(challenge); err != nil {
		t.Fatal(err)
	}
	receipts := NewMemorySafeWriteReceiptStore()
	service, err := NewGitHubSafeWriteService(writer, challenges, receipts)
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow.Add(time.Second) }

	first, err := service.CreateIssue(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Replayed || first.Receipt.Status != SafeWriteReceiptAccepted || first.Receipt.TargetType != "github_repository" || first.Receipt.TargetID != "owner/repo" || first.Receipt.Issue.Number != 42 {
		t.Fatalf("first write = %+v", first)
	}
	if len(writer.CreateCalls) != 1 {
		t.Fatalf("writer calls = %d, want 1", len(writer.CreateCalls))
	}

	second, err := service.CreateIssue(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed || second.Receipt.ID != first.Receipt.ID || second.Receipt.Issue.Number != 42 {
		t.Fatalf("replayed write = %+v", second)
	}
	if len(writer.CreateCalls) != 1 {
		t.Fatalf("replay called provider %d times", len(writer.CreateCalls))
	}
}

func TestSafeWriteRejectsChallengeMismatchAndIdempotencyConflict(t *testing.T) {
	request := CreateIssueRequest{Repository: "owner/repo", Title: "Original", Metadata: SafeWriteMetadata{IdempotencyKey: "request-1", UserID: "user-1"}}
	otherTarget := request.Target()
	otherTarget.Title = "Different"
	challenge, err := NewSafeWriteChallenge("challenge-1", "user-1", otherTarget, connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	request.Metadata = challenge.Confirmation(connectorTestNow)
	request.Metadata.IdempotencyKey = "request-1"
	challenges := NewMemorySafeWriteChallengeStore()
	if err := challenges.Put(challenge); err != nil {
		t.Fatal(err)
	}
	writer := &FakeGitHubWriter{}
	service, err := NewGitHubSafeWriteService(writer, challenges, NewMemorySafeWriteReceiptStore())
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }
	if _, err := service.CreateIssue(context.Background(), request); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("mismatched challenge error = %v", err)
	}
	if len(writer.CreateCalls) != 0 {
		t.Fatal("mismatched challenge reached provider")
	}

	validRequest, validChallenge := newConfirmedCreateRequest(t, "request-1", "valid-challenge", "Original")
	validChallengeStore := NewMemorySafeWriteChallengeStore()
	if err := validChallengeStore.Put(validChallenge); err != nil {
		t.Fatal(err)
	}
	service.Challenges = validChallengeStore
	if _, err := service.CreateIssue(context.Background(), validRequest); err != nil {
		t.Fatal(err)
	}
	conflicting := validRequest
	conflicting.Title = "Different body"
	if _, err := service.CreateIssue(context.Background(), conflicting); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict error = %v", err)
	}
}

func TestSafeWriteSupportsOnlyValidatedCommentAndLabels(t *testing.T) {
	writer := &FakeGitHubWriter{}
	challenges := NewMemorySafeWriteChallengeStore()
	service, err := NewGitHubSafeWriteService(writer, challenges, NewMemorySafeWriteReceiptStore())
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }

	comment, challenge := newConfirmedCommentRequest(t)
	if err := challenges.Put(challenge); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddIssueComment(context.Background(), comment); err != nil {
		t.Fatal(err)
	}

	labels := LabelIssueRequest{Repository: "owner/repo", Issue: 7, Labels: []string{"z", "a"}, Metadata: SafeWriteMetadata{IdempotencyKey: "labels-1", UserID: "user-1"}}
	labelChallenge, err := NewSafeWriteChallenge("challenge-labels", "user-1", labels.Target(), connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := challenges.Put(labelChallenge); err != nil {
		t.Fatal(err)
	}
	labels.Metadata = labelChallenge.Confirmation(connectorTestNow)
	labels.Metadata.IdempotencyKey = "labels-1"
	if _, err := service.SetIssueLabels(context.Background(), labels); err != nil {
		t.Fatal(err)
	}
	if len(writer.CommentCalls) != 1 || len(writer.LabelCalls) != 1 {
		t.Fatalf("comment calls=%d label calls=%d", len(writer.CommentCalls), len(writer.LabelCalls))
	}

	if err := (CreateIssueRequest{Repository: "owner/repo", Title: "", Metadata: SafeWriteMetadata{IdempotencyKey: "x"}}).Validate(); !errors.Is(err, ErrInvalidWriteTarget) {
		t.Fatalf("invalid create request error = %v", err)
	}
	if _, err := (SafeWriteTarget{Operation: "github.push", Repository: "owner/repo"}).ActionHash(); !errors.Is(err, ErrUnsupportedWrite) {
		t.Fatalf("unsupported operation error = %v", err)
	}
}

func TestSafeWriteProviderErrorsAreClassifiedAndRedacted(t *testing.T) {
	request, challenge := newConfirmedCreateRequest(t, "request-error", "challenge-error", "Will fail")
	challenges := NewMemorySafeWriteChallengeStore()
	if err := challenges.Put(challenge); err != nil {
		t.Fatal(err)
	}
	writer := &FakeGitHubWriter{Failure: NewProviderError("github", "create", http.StatusUnauthorized, "authorization: Bearer super-secret")}
	service, err := NewGitHubSafeWriteService(writer, challenges, NewMemorySafeWriteReceiptStore())
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }
	_, err = service.CreateIssue(context.Background(), request)
	if err == nil || strings.Contains(err.Error(), "super-secret") || ClassifyError(err) != RetryAuthentication {
		t.Fatalf("provider error = %v, class=%q", err, ClassifyError(err))
	}
}

func TestSafeWriteCompleteFailureLeavesPendingAndPreventsProviderReplay(t *testing.T) {
	request, challenge := newConfirmedCreateRequest(t, "request-uncertain", "challenge-uncertain", "Uncertain write")
	challenges := NewMemorySafeWriteChallengeStore()
	if err := challenges.Put(challenge); err != nil {
		t.Fatal(err)
	}
	receipts := NewMemorySafeWriteReceiptStore()
	receipts.CompleteError = errors.New("database connection lost")
	writer := &FakeGitHubWriter{NextIssueNumber: 9}
	service, err := NewGitHubSafeWriteService(writer, challenges, receipts)
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }

	if _, err := service.CreateIssue(context.Background(), request); !errors.Is(err, ErrReceiptUncertain) {
		t.Fatalf("first write error = %v, want ErrReceiptUncertain", err)
	}
	if len(writer.CreateCalls) != 1 {
		t.Fatalf("provider calls after uncertain completion = %d, want 1", len(writer.CreateCalls))
	}
	pending, found, err := receipts.Lookup(context.Background(), request.Metadata.WorkspaceID, request.Metadata.UserID, SafeWriteOperationCreateIssue, request.Metadata.IdempotencyKey)
	if err != nil || !found || pending.Status != SafeWriteReceiptPending {
		t.Fatalf("pending receipt = %+v, found=%v, err=%v", pending, found, err)
	}

	if _, err := service.CreateIssue(context.Background(), request); !errors.Is(err, ErrReceiptUncertain) {
		t.Fatalf("retry error = %v, want ErrReceiptUncertain", err)
	}
	if len(writer.CreateCalls) != 1 {
		t.Fatalf("provider replayed after pending receipt: %d calls", len(writer.CreateCalls))
	}
}

func TestSafeWriteReceiptIdentityIncludesWorkspace(t *testing.T) {
	writer := &FakeGitHubWriter{NextIssueNumber: 10}
	challenges := NewMemorySafeWriteChallengeStore()
	receipts := NewMemorySafeWriteReceiptStore()
	service, err := NewGitHubSafeWriteService(writer, challenges, receipts)
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }

	first := CreateIssueRequest{Repository: "owner/alpha", Title: "Same key in alpha", Metadata: SafeWriteMetadata{WorkspaceID: "workspace-alpha", UserID: "user-1", IdempotencyKey: "same-key"}}
	firstChallenge, err := NewWorkspaceSafeWriteChallenge(first.Metadata.WorkspaceID, "challenge-alpha", first.Metadata.UserID, first.Target(), connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := challenges.Put(firstChallenge); err != nil {
		t.Fatal(err)
	}
	first.Metadata = firstChallenge.Confirmation(connectorTestNow)
	first.Metadata.IdempotencyKey = "same-key"

	second := CreateIssueRequest{Repository: "owner/beta", Title: "Same key in beta", Metadata: SafeWriteMetadata{WorkspaceID: "workspace-beta", UserID: "user-1", IdempotencyKey: "same-key"}}
	secondChallenge, err := NewWorkspaceSafeWriteChallenge(second.Metadata.WorkspaceID, "challenge-beta", second.Metadata.UserID, second.Target(), connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := challenges.Put(secondChallenge); err != nil {
		t.Fatal(err)
	}
	second.Metadata = secondChallenge.Confirmation(connectorTestNow)
	second.Metadata.IdempotencyKey = "same-key"

	left, err := service.CreateIssue(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	right, err := service.CreateIssue(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	if left.Replayed || right.Replayed || left.Receipt.ID == right.Receipt.ID {
		t.Fatalf("workspace-scoped writes were incorrectly replayed or shared: left=%+v right=%+v", left, right)
	}
	if left.Receipt.WorkspaceID != first.Metadata.WorkspaceID || right.Receipt.WorkspaceID != second.Metadata.WorkspaceID {
		t.Fatalf("receipt workspace identity was not preserved: left=%+v right=%+v", left.Receipt, right.Receipt)
	}
	if len(writer.CreateCalls) != 2 {
		t.Fatalf("provider calls = %d, want 2", len(writer.CreateCalls))
	}

	replayedLeft, err := service.CreateIssue(context.Background(), first)
	if err != nil || !replayedLeft.Replayed || replayedLeft.Receipt.ID != left.Receipt.ID {
		t.Fatalf("alpha replay = %+v, err=%v", replayedLeft, err)
	}
	replayedRight, err := service.CreateIssue(context.Background(), second)
	if err != nil || !replayedRight.Replayed || replayedRight.Receipt.ID != right.Receipt.ID {
		t.Fatalf("beta replay = %+v, err=%v", replayedRight, err)
	}
}

func TestActionHashNormalizesLabelOrder(t *testing.T) {
	left := SafeWriteTarget{Operation: SafeWriteOperationSetLabels, Repository: "Owner/Repo", Issue: 7, Labels: []string{"z", "a"}}
	right := SafeWriteTarget{Operation: SafeWriteOperationSetLabels, Repository: "owner/repo", Issue: 7, Labels: []string{"a", "z"}}
	leftHash, err := left.ActionHash()
	if err != nil {
		t.Fatal(err)
	}
	rightHash, err := right.ActionHash()
	if err != nil {
		t.Fatal(err)
	}
	if leftHash != rightHash {
		t.Fatalf("hashes differ: %q != %q", leftHash, rightHash)
	}
}

func newConfirmedCreateRequest(t *testing.T, idempotency, challengeID, title string) (CreateIssueRequest, SafeWriteChallenge) {
	t.Helper()
	request := CreateIssueRequest{Repository: "owner/repo", Title: title, Metadata: SafeWriteMetadata{IdempotencyKey: idempotency, UserID: "user-1"}}
	challenge, err := NewSafeWriteChallenge(challengeID, "user-1", request.Target(), connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	request.Metadata = challenge.Confirmation(connectorTestNow)
	request.Metadata.IdempotencyKey = idempotency
	return request, challenge
}

func newConfirmedCommentRequest(t *testing.T) (CommentIssueRequest, SafeWriteChallenge) {
	t.Helper()
	request := CommentIssueRequest{Repository: "owner/repo", Issue: 7, Body: "comment", Metadata: SafeWriteMetadata{IdempotencyKey: "comment-1", UserID: "user-1"}}
	challenge, err := NewSafeWriteChallenge("challenge-comment", "user-1", request.Target(), connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	request.Metadata = challenge.Confirmation(connectorTestNow)
	request.Metadata.IdempotencyKey = "comment-1"
	return request, challenge
}
