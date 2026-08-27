package connectors

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSafeWriteChallengeBindsActionAndReplayReturnsReceipt(t *testing.T) {
	request := CreateIssueRequest{Repository: "Owner/Repo", Title: "Fix sync", Body: "details", Metadata: SafeWriteMetadata{WorkspaceID: "workspace-1", IdempotencyKey: "request-1", UserID: "user-1"}}
	challenge, err := NewSafeWriteChallenge("challenge-1", "workspace-1", "user-1", request.Target(), connectorTestNow)
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
	if len(writer.CreateCalls) != 1 || receipts.ReceiptCount() != 1 {
		t.Fatalf("writer calls=%d receipts=%d", len(writer.CreateCalls), receipts.ReceiptCount())
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
	request := CreateIssueRequest{Repository: "owner/repo", Title: "Original", Metadata: SafeWriteMetadata{WorkspaceID: "workspace-1", IdempotencyKey: "request-1", UserID: "user-1"}}
	otherTarget := request.Target()
	otherTarget.Title = "Different"
	challenge, err := NewSafeWriteChallenge("challenge-1", "workspace-1", "user-1", otherTarget, connectorTestNow)
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

	labels := LabelIssueRequest{Repository: "owner/repo", Issue: 7, Labels: []string{"z", "a"}, Metadata: SafeWriteMetadata{WorkspaceID: "workspace-1", IdempotencyKey: "labels-1", UserID: "user-1"}}
	labelChallenge, err := NewSafeWriteChallenge("challenge-labels", "workspace-1", "user-1", labels.Target(), connectorTestNow)
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
	for _, target := range []SafeWriteTarget{
		{Operation: SafeWriteOperationAddComment, Repository: "owner/repo", Issue: 0, Body: "comment"},
		{Operation: SafeWriteOperationSetLabels, Repository: "owner/repo", Issue: 7, Labels: []string{"a", " a "}},
		{Operation: SafeWriteOperationSetLabels, Repository: "owner/repo", Issue: 7, Labels: []string{""}},
		{Operation: SafeWriteOperationCreateIssue, Repository: "not-a-repository", Title: "title"},
	} {
		if err := target.Validate(); err == nil {
			t.Fatalf("invalid target unexpectedly passed: %+v", target)
		}
	}
}

func TestSafeWriteProviderErrorsAreClassifiedAndRedacted(t *testing.T) {
	request, challenge := newConfirmedCreateRequest(t, "request-error", "challenge-error", "Will fail")
	challenges := NewMemorySafeWriteChallengeStore()
	if err := challenges.Put(challenge); err != nil {
		t.Fatal(err)
	}
	writer := &FakeGitHubWriter{Failure: NewProviderError("github", "create", http.StatusUnauthorized, "authorization: Bearer fake-provider-secret")}
	service, err := NewGitHubSafeWriteService(writer, challenges, NewMemorySafeWriteReceiptStore())
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }
	_, err = service.CreateIssue(context.Background(), request)
	if err == nil || strings.Contains(err.Error(), "fake-provider-secret") || ClassifyError(err) != RetryAuthentication {
		t.Fatalf("provider error = %v, class=%q", err, ClassifyError(err))
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
	changed := right
	changed.ExpectedRevision = "new-revision"
	changedHash, err := changed.ActionHash()
	if err != nil || changedHash == rightHash {
		t.Fatalf("expected revision to bind action: %q %v", changedHash, err)
	}
}

func TestSafeWriteChallengeTTLAndSingleUse(t *testing.T) {
	target := SafeWriteTarget{Operation: SafeWriteOperationCreateIssue, Repository: "owner/repo", Title: "title"}
	challenge, err := NewSafeWriteChallenge("challenge-ttl", "workspace-1", "user-1", target, connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if !challenge.ExpiresAt.Equal(connectorTestNow.Add(SafeWriteChallengeTTL)) {
		t.Fatalf("challenge expiry = %v", challenge.ExpiresAt)
	}
	if err := challenge.Validate(connectorTestNow); err != nil {
		t.Fatal(err)
	}
	if err := challenge.Validate(challenge.ExpiresAt); !errors.Is(err, ErrChallengeExpired) {
		t.Fatalf("expired challenge error = %v", err)
	}
	if err := (SafeWriteChallenge{ID: "x", WorkspaceID: "workspace-1", UserID: "u", TargetType: "t", TargetID: "i", ActionHash: "h", CreatedAt: connectorTestNow, ExpiresAt: connectorTestNow.Add(SafeWriteChallengeTTL + time.Nanosecond)}).Validate(connectorTestNow); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("long challenge error = %v", err)
	}

	store := NewMemorySafeWriteChallengeStore()
	if err := store.Put(challenge); err != nil {
		t.Fatal(err)
	}
	metadata := challenge.Confirmation(connectorTestNow.Add(time.Second))
	if err := store.Consume(context.Background(), metadata, target, connectorTestNow.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.Consume(context.Background(), metadata, target, connectorTestNow.Add(2*time.Second)); !errors.Is(err, ErrChallengeUsed) {
		t.Fatalf("second consume error = %v", err)
	}
	used, ok := store.Challenge(challenge.ID)
	if !ok || used.UsedAt == nil {
		t.Fatalf("used challenge = %+v exists=%v", used, ok)
	}
}

func TestSafeWriteConfirmationValidation(t *testing.T) {
	target := SafeWriteTarget{Operation: SafeWriteOperationAddComment, Repository: "owner/repo", Issue: 7, Body: "comment"}
	valid, err := NewSafeWriteChallenge("challenge-confirm", "workspace-1", "user-1", target, connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	metadata := valid.Confirmation(connectorTestNow.Add(time.Second))
	if err := metadata.ValidateConfirmation(connectorTestNow.Add(time.Second), target); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []struct {
		name string
		meta SafeWriteMetadata
		want error
	}{
		{name: "missing user", meta: SafeWriteMetadata{ChallengeID: "id", ActionHash: "hash"}, want: ErrMissingUserID},
		{name: "missing challenge", meta: SafeWriteMetadata{UserID: "user", ActionHash: "hash"}, want: ErrMissingChallenge},
		{name: "wrong hash", meta: SafeWriteMetadata{WorkspaceID: "workspace-1", UserID: "user", ChallengeID: "id", ActionHash: "wrong", ConfirmedAt: connectorTestNow, ExpiresAt: connectorTestNow.Add(time.Minute)}, want: ErrInvalidChallenge},
		{name: "future confirmation", meta: valid.Confirmation(connectorTestNow.Add(2 * time.Minute)), want: ErrChallengeExpired},
		{name: "expired confirmation", meta: valid.Confirmation(connectorTestNow), want: ErrChallengeExpired},
		{name: "long confirmation", meta: SafeWriteMetadata{WorkspaceID: "workspace-1", UserID: "user-1", ChallengeID: valid.ID, ActionHash: valid.ActionHash, ConfirmedAt: connectorTestNow.Add(-time.Minute), ExpiresAt: connectorTestNow.Add(6 * time.Minute)}, want: ErrChallengeExpired},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			now := connectorTestNow.Add(time.Second)
			if invalid.name == "expired confirmation" {
				now = valid.ExpiresAt
			}
			if err := invalid.meta.ValidateConfirmation(now, target); !errors.Is(err, invalid.want) {
				t.Fatalf("error = %v, want %v", err, invalid.want)
			}
		})
	}
	if err := metadata.ValidateConfirmation(connectorTestNow.Add(time.Second), SafeWriteTarget{Operation: SafeWriteOperationAddComment, Repository: "owner/repo", Issue: 8, Body: "comment"}); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("target mismatch error = %v", err)
	}
}

func TestSafeWriteReplayIgnoresExpiredConfirmation(t *testing.T) {
	request, challenge := newConfirmedCreateRequest(t, "replay-expiry", "challenge-expiry", "Replay")
	challenges := NewMemorySafeWriteChallengeStore()
	if err := challenges.Put(challenge); err != nil {
		t.Fatal(err)
	}
	writer := &FakeGitHubWriter{NextIssueNumber: 9}
	service, err := NewGitHubSafeWriteService(writer, challenges, NewMemorySafeWriteReceiptStore())
	if err != nil {
		t.Fatal(err)
	}
	now := connectorTestNow
	service.Clock = func() time.Time { return now }
	if _, err := service.CreateIssue(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	now = challenge.ExpiresAt.Add(time.Second)
	replay, err := service.CreateIssue(context.Background(), request)
	if err != nil || !replay.Replayed || len(writer.CreateCalls) != 1 {
		t.Fatalf("expired replay=%+v err=%v calls=%d", replay, err, len(writer.CreateCalls))
	}
}

func TestSafeWriteConcurrentReplayCallsProviderOnce(t *testing.T) {
	request, challenge := newConfirmedCreateRequest(t, "concurrent", "challenge-concurrent", "Concurrent")
	challenges := NewMemorySafeWriteChallengeStore()
	if err := challenges.Put(challenge); err != nil {
		t.Fatal(err)
	}
	writer := &FakeGitHubWriter{NextIssueNumber: 100}
	service, err := NewGitHubSafeWriteService(writer, challenges, NewMemorySafeWriteReceiptStore())
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }
	const calls = 12
	results := make(chan GitHubWriteOutcome, calls)
	errorsCh := make(chan error, calls)
	var wait sync.WaitGroup
	for i := 0; i < calls; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			outcome, callErr := service.CreateIssue(context.Background(), request)
			results <- outcome
			errorsCh <- callErr
		}()
	}
	wait.Wait()
	close(results)
	close(errorsCh)
	replayed := 0
	for outcome := range results {
		if outcome.Replayed {
			replayed++
		}
	}
	for callErr := range errorsCh {
		if callErr != nil {
			t.Fatalf("concurrent call error = %v", callErr)
		}
	}
	if len(writer.CreateCalls) != 1 || replayed != calls-1 {
		t.Fatalf("provider calls=%d replayed=%d", len(writer.CreateCalls), replayed)
	}
}

func TestSafeWriteReservationPreventsDuplicateAcrossServicesWhenFinalizeFails(t *testing.T) {
	requestOne, challengeOne := newConfirmedCreateRequest(t, "shared-idempotency", "challenge-one", "Reserved")
	requestTwo, challengeTwo := newConfirmedCreateRequest(t, "shared-idempotency", "challenge-two", "Reserved")
	challengesOne := NewMemorySafeWriteChallengeStore()
	challengesTwo := NewMemorySafeWriteChallengeStore()
	if err := challengesOne.Put(challengeOne); err != nil {
		t.Fatal(err)
	}
	if err := challengesTwo.Put(challengeTwo); err != nil {
		t.Fatal(err)
	}
	receipts := newFinalizeFailureReceiptStore()
	writerOne := &FakeGitHubWriter{NextIssueNumber: 1}
	writerTwo := &FakeGitHubWriter{NextIssueNumber: 2}
	serviceOne, err := NewGitHubSafeWriteService(writerOne, challengesOne, receipts)
	if err != nil {
		t.Fatal(err)
	}
	serviceTwo, err := NewGitHubSafeWriteService(writerTwo, challengesTwo, receipts)
	if err != nil {
		t.Fatal(err)
	}
	serviceOne.Clock = func() time.Time { return connectorTestNow }
	serviceTwo.Clock = func() time.Time { return connectorTestNow }

	first, err := serviceOne.CreateIssue(context.Background(), requestOne)
	if err == nil || !errors.Is(err, ErrReceiptPersistence) || first.Receipt.Status != SafeWriteReceiptPending || len(writerOne.CreateCalls) != 1 {
		t.Fatalf("first write outcome=%+v err=%v calls=%d", first, err, len(writerOne.CreateCalls))
	}
	stored, found, err := receipts.inner.Lookup(context.Background(), "workspace-1", "user-1", SafeWriteOperationCreateIssue, "shared-idempotency")
	if err != nil || !found || stored.Status != SafeWriteReceiptPending {
		t.Fatalf("durable reservation=%+v found=%v err=%v", stored, found, err)
	}

	second, err := serviceTwo.CreateIssue(context.Background(), requestTwo)
	if !errors.Is(err, ErrWritePending) || second.Receipt.Status != SafeWriteReceiptPending || len(writerTwo.CreateCalls) != 0 {
		t.Fatalf("duplicate write outcome=%+v err=%v calls=%d", second, err, len(writerTwo.CreateCalls))
	}
}

func TestSafeWriteReservationsAreWorkspaceScoped(t *testing.T) {
	requestA, challengeA := newConfirmedCreateRequestForScope(t, "workspace-1", "user-1", "same-key", "challenge-a", "Workspace A")
	requestB, challengeB := newConfirmedCreateRequestForScope(t, "workspace-2", "user-2", "same-key", "challenge-b", "Workspace B")
	challengesA := NewMemorySafeWriteChallengeStore()
	challengesB := NewMemorySafeWriteChallengeStore()
	if err := challengesA.Put(challengeA); err != nil {
		t.Fatal(err)
	}
	if err := challengesB.Put(challengeB); err != nil {
		t.Fatal(err)
	}
	receipts := NewMemorySafeWriteReceiptStore()
	writerA := &FakeGitHubWriter{NextIssueNumber: 1}
	writerB := &FakeGitHubWriter{NextIssueNumber: 2}
	serviceA, err := NewGitHubSafeWriteService(writerA, challengesA, receipts)
	if err != nil {
		t.Fatal(err)
	}
	serviceB, err := NewGitHubSafeWriteService(writerB, challengesB, receipts)
	if err != nil {
		t.Fatal(err)
	}
	serviceA.Clock = func() time.Time { return connectorTestNow }
	serviceB.Clock = func() time.Time { return connectorTestNow }

	first, err := serviceA.CreateIssue(context.Background(), requestA)
	if err != nil || first.Receipt.Status != SafeWriteReceiptAccepted {
		t.Fatalf("workspace A outcome=%+v err=%v", first, err)
	}
	second, err := serviceB.CreateIssue(context.Background(), requestB)
	if err != nil || second.Receipt.Status != SafeWriteReceiptAccepted {
		t.Fatalf("workspace B outcome=%+v err=%v", second, err)
	}
	if first.Receipt.ID == second.Receipt.ID || receipts.ReceiptCount() != 2 || len(writerA.CreateCalls) != 1 || len(writerB.CreateCalls) != 1 {
		t.Fatalf("workspace-scoped reservations: first=%+v second=%+v receipts=%d callsA=%d callsB=%d", first.Receipt, second.Receipt, receipts.ReceiptCount(), len(writerA.CreateCalls), len(writerB.CreateCalls))
	}
}

func TestSafeWriteProviderFailureMarksUncertainAndBlocksRetry(t *testing.T) {
	requestOne, challengeOne := newConfirmedCreateRequest(t, "uncertain-idempotency", "uncertain-one", "Provider failure")
	requestTwo, challengeTwo := newConfirmedCreateRequest(t, "uncertain-idempotency", "uncertain-two", "Provider failure")
	challengesOne := NewMemorySafeWriteChallengeStore()
	challengesTwo := NewMemorySafeWriteChallengeStore()
	if err := challengesOne.Put(challengeOne); err != nil {
		t.Fatal(err)
	}
	if err := challengesTwo.Put(challengeTwo); err != nil {
		t.Fatal(err)
	}
	receipts := NewMemorySafeWriteReceiptStore()
	writerOne := &FakeGitHubWriter{Failure: NewProviderError("github", "create", http.StatusBadGateway, "temporary provider failure")}
	writerTwo := &FakeGitHubWriter{NextIssueNumber: 7}
	serviceOne, err := NewGitHubSafeWriteService(writerOne, challengesOne, receipts)
	if err != nil {
		t.Fatal(err)
	}
	serviceTwo, err := NewGitHubSafeWriteService(writerTwo, challengesTwo, receipts)
	if err != nil {
		t.Fatal(err)
	}
	serviceOne.Clock = func() time.Time { return connectorTestNow }
	serviceTwo.Clock = func() time.Time { return connectorTestNow }

	first, err := serviceOne.CreateIssue(context.Background(), requestOne)
	if err == nil || !errors.Is(err, ErrWriteUncertain) || !first.Uncertain || first.Receipt.Status != SafeWriteReceiptUncertain || ClassifyError(err) != RetryTransient || len(writerOne.CreateCalls) != 1 {
		t.Fatalf("provider failure outcome=%+v err=%v class=%q calls=%d", first, err, ClassifyError(err), len(writerOne.CreateCalls))
	}
	second, err := serviceTwo.CreateIssue(context.Background(), requestTwo)
	if !errors.Is(err, ErrWriteUncertain) || !second.Uncertain || second.Receipt.Status != SafeWriteReceiptUncertain || len(writerTwo.CreateCalls) != 0 {
		t.Fatalf("uncertain replay outcome=%+v err=%v calls=%d", second, err, len(writerTwo.CreateCalls))
	}
}

func TestSafeWriteRejectsInvalidProviderPayloadAndReceiptFailure(t *testing.T) {
	request, challenge := newConfirmedCreateRequest(t, "payload", "challenge-payload", "Payload")
	challenges := NewMemorySafeWriteChallengeStore()
	if err := challenges.Put(challenge); err != nil {
		t.Fatal(err)
	}
	invalidWriter := &scriptedGitHubWriter{issue: GitHubIssue{Repository: "other/repo", Number: 1}}
	service, err := NewGitHubSafeWriteService(invalidWriter, challenges, NewMemorySafeWriteReceiptStore())
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }
	if _, err := service.CreateIssue(context.Background(), request); !errors.Is(err, ErrInvalidProviderPayload) {
		t.Fatalf("invalid issue payload error = %v", err)
	}

	commentRequest, commentChallenge := newConfirmedCommentRequest(t)
	commentChallenges := NewMemorySafeWriteChallengeStore()
	if err := commentChallenges.Put(commentChallenge); err != nil {
		t.Fatal(err)
	}
	invalidWriter = &scriptedGitHubWriter{comment: GitHubComment{Repository: "owner/repo", Issue: commentRequest.Issue}}
	service, err = NewGitHubSafeWriteService(invalidWriter, commentChallenges, NewMemorySafeWriteReceiptStore())
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }
	if _, err := service.AddIssueComment(context.Background(), commentRequest); !errors.Is(err, ErrInvalidProviderPayload) {
		t.Fatalf("invalid comment payload error = %v", err)
	}

	labelRequest := LabelIssueRequest{Repository: "owner/repo", Issue: 7, Labels: []string{"bug"}, Metadata: SafeWriteMetadata{WorkspaceID: "workspace-1", IdempotencyKey: "payload-label", UserID: "user-1"}}
	labelChallenge, err := NewSafeWriteChallenge("challenge-payload-label", "workspace-1", "user-1", labelRequest.Target(), connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	labelRequest.Metadata = labelChallenge.Confirmation(connectorTestNow)
	labelRequest.Metadata.IdempotencyKey = "payload-label"
	labelChallenges := NewMemorySafeWriteChallengeStore()
	if err := labelChallenges.Put(labelChallenge); err != nil {
		t.Fatal(err)
	}
	invalidWriter = &scriptedGitHubWriter{labels: []GitHubLabel{{Repository: "other/repo", Issue: 7, Name: "bug"}}}
	service, err = NewGitHubSafeWriteService(invalidWriter, labelChallenges, NewMemorySafeWriteReceiptStore())
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }
	if _, err := service.SetIssueLabels(context.Background(), labelRequest); !errors.Is(err, ErrInvalidProviderPayload) {
		t.Fatalf("invalid labels payload error = %v", err)
	}

	receiptChallenges := NewMemorySafeWriteChallengeStore()
	receiptRequest, receiptChallenge := newConfirmedCreateRequest(t, "receipt-failure", "challenge-receipt-failure", "Receipt")
	if err := receiptChallenges.Put(receiptChallenge); err != nil {
		t.Fatal(err)
	}
	service, err = NewGitHubSafeWriteService(&FakeGitHubWriter{}, receiptChallenges, failingReceiptStore{err: errors.New("database token=fake-receipt-secret")})
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }
	if _, err := service.CreateIssue(context.Background(), receiptRequest); err == nil || !errors.Is(err, ErrReceiptPersistence) || strings.Contains(err.Error(), "fake-receipt-secret") {
		t.Fatalf("receipt failure error = %v", err)
	}
}

func TestMemoryStoresValidateAndClone(t *testing.T) {
	if err := NewMemorySafeWriteChallengeStore().Put(SafeWriteChallenge{}); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("invalid challenge put = %v", err)
	}
	validTarget := SafeWriteTarget{Operation: SafeWriteOperationCreateIssue, Repository: "owner/repo", Title: "title"}
	validChallenge, err := NewSafeWriteChallenge("duplicate", "workspace-1", "user", validTarget, connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	challengeStore := NewMemorySafeWriteChallengeStore()
	if err := challengeStore.Put(validChallenge); err != nil {
		t.Fatal(err)
	}
	if err := challengeStore.Put(validChallenge); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("duplicate challenge put = %v", err)
	}
	if err := challengeStore.Consume(context.Background(), validChallenge.Confirmation(connectorTestNow), validTarget, connectorTestNow); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := challengeStore.Consume(canceled, validChallenge.Confirmation(connectorTestNow), validTarget, connectorTestNow); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled challenge consume = %v", err)
	}

	receipts := NewMemorySafeWriteReceiptStore()
	if err := receipts.Save(context.Background(), SafeWriteReceipt{}); !errors.Is(err, ErrReceiptPersistence) {
		t.Fatalf("invalid receipt save = %v", err)
	}
	receipt := SafeWriteReceipt{ID: "id", WorkspaceID: "workspace-1", UserID: "user", Operation: SafeWriteOperationCreateIssue, IdempotencyKey: "key", ActionHash: "hash", TargetType: "github_repository", TargetID: "owner/repo", Status: SafeWriteReceiptAccepted, CreatedAt: connectorTestNow, Labels: []GitHubLabel{{Name: "one"}}}
	if err := receipts.Save(context.Background(), receipt); err != nil {
		t.Fatal(err)
	}
	got, ok, err := receipts.Lookup(context.Background(), "workspace-1", "user", SafeWriteOperationCreateIssue, "key")
	if err != nil || !ok || len(got.Labels) != 1 {
		t.Fatalf("receipt lookup=%+v ok=%v err=%v", got, ok, err)
	}
	receipt.Labels[0].Name = "changed"
	gotAgain, _, _ := receipts.Lookup(context.Background(), "workspace-1", "user", SafeWriteOperationCreateIssue, "key")
	if gotAgain.Labels[0].Name != "one" {
		t.Fatal("receipt lookup was not cloned")
	}
	conflict := receipt
	conflict.ActionHash = "different"
	if err := receipts.Save(context.Background(), conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("receipt conflict = %v", err)
	}
	pending := receipt
	pending.Status = SafeWriteReceiptPending
	reserved, replayed, err := receipts.Reserve(context.Background(), pending)
	if err != nil || !replayed || reserved.Status != SafeWriteReceiptAccepted {
		t.Fatalf("existing accepted reservation=%+v replayed=%v err=%v", reserved, replayed, err)
	}
	if err := receipts.Save(context.Background(), pending); err != nil {
		t.Fatalf("accepted receipt downgrade save = %v", err)
	}
	final, _, err := receipts.Lookup(context.Background(), "workspace-1", "user", SafeWriteOperationCreateIssue, "key")
	if err != nil || final.Status != SafeWriteReceiptAccepted {
		t.Fatalf("accepted receipt was downgraded: %+v err=%v", final, err)
	}
}

type scriptedGitHubWriter struct {
	issue   GitHubIssue
	comment GitHubComment
	labels  []GitHubLabel
}

func (w *scriptedGitHubWriter) CreateIssue(context.Context, CreateIssueRequest) (GitHubIssue, error) {
	return w.issue, nil
}

func (w *scriptedGitHubWriter) AddIssueComment(context.Context, CommentIssueRequest) (GitHubComment, error) {
	return w.comment, nil
}

func (w *scriptedGitHubWriter) SetIssueLabels(context.Context, LabelIssueRequest) ([]GitHubLabel, error) {
	return w.labels, nil
}

type failingReceiptStore struct{ err error }

func (s failingReceiptStore) Lookup(context.Context, string, string, SafeWriteOperation, string) (SafeWriteReceipt, bool, error) {
	return SafeWriteReceipt{}, false, nil
}

func (s failingReceiptStore) Reserve(_ context.Context, receipt SafeWriteReceipt) (SafeWriteReceipt, bool, error) {
	return receipt, false, nil
}

func (s failingReceiptStore) Save(context.Context, SafeWriteReceipt) error { return s.err }

type finalizeFailureReceiptStore struct {
	inner *MemorySafeWriteReceiptStore
	err   error
}

func newFinalizeFailureReceiptStore() *finalizeFailureReceiptStore {
	return &finalizeFailureReceiptStore{inner: NewMemorySafeWriteReceiptStore(), err: errors.New("database temporarily unavailable")}
}

func (s *finalizeFailureReceiptStore) Lookup(ctx context.Context, workspaceID, userID string, operation SafeWriteOperation, idempotencyKey string) (SafeWriteReceipt, bool, error) {
	return s.inner.Lookup(ctx, workspaceID, userID, operation, idempotencyKey)
}

func (s *finalizeFailureReceiptStore) Reserve(ctx context.Context, receipt SafeWriteReceipt) (SafeWriteReceipt, bool, error) {
	return s.inner.Reserve(ctx, receipt)
}

func (s *finalizeFailureReceiptStore) Save(context.Context, SafeWriteReceipt) error { return s.err }

func newConfirmedCreateRequest(t *testing.T, idempotency, challengeID, title string) (CreateIssueRequest, SafeWriteChallenge) {
	return newConfirmedCreateRequestForScope(t, "workspace-1", "user-1", idempotency, challengeID, title)
}

func newConfirmedCreateRequestForScope(t *testing.T, workspaceID, userID, idempotency, challengeID, title string) (CreateIssueRequest, SafeWriteChallenge) {
	t.Helper()
	request := CreateIssueRequest{Repository: "owner/repo", Title: title, Metadata: SafeWriteMetadata{WorkspaceID: workspaceID, IdempotencyKey: idempotency, UserID: userID}}
	challenge, err := NewSafeWriteChallenge(challengeID, workspaceID, userID, request.Target(), connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	request.Metadata = challenge.Confirmation(connectorTestNow)
	request.Metadata.IdempotencyKey = idempotency
	return request, challenge
}

func newConfirmedCommentRequest(t *testing.T) (CommentIssueRequest, SafeWriteChallenge) {
	t.Helper()
	request := CommentIssueRequest{Repository: "owner/repo", Issue: 7, Body: "comment", Metadata: SafeWriteMetadata{WorkspaceID: "workspace-1", IdempotencyKey: "comment-1", UserID: "user-1"}}
	challenge, err := NewSafeWriteChallenge("challenge-comment", "workspace-1", "user-1", request.Target(), connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	request.Metadata = challenge.Confirmation(connectorTestNow)
	request.Metadata.IdempotencyKey = "comment-1"
	return request, challenge
}
