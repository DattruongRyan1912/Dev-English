package connectors

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSafeWriteValidationAndProviderPayloadBoundaries(t *testing.T) {
	validCreate := SafeWriteTarget{Operation: SafeWriteOperationCreateIssue, Repository: "owner/repo", Title: "title"}
	validComment := SafeWriteTarget{Operation: SafeWriteOperationAddComment, Repository: "owner/repo", Issue: 4, Body: "body"}
	validLabels := SafeWriteTarget{Operation: SafeWriteOperationSetLabels, Repository: "owner/repo", Issue: 4, Labels: []string{"bug", "urgent"}}

	invalidTargets := []struct {
		name   string
		target SafeWriteTarget
		want   error
	}{
		{"missing repository", SafeWriteTarget{Operation: SafeWriteOperationCreateIssue, Title: "title"}, ErrInvalidRepository},
		{"create has issue", SafeWriteTarget{Operation: SafeWriteOperationCreateIssue, Repository: "owner/repo", Issue: 1, Title: "title"}, ErrInvalidWriteTarget},
		{"create has labels", SafeWriteTarget{Operation: SafeWriteOperationCreateIssue, Repository: "owner/repo", Title: "title", Labels: []string{"bug"}}, ErrInvalidWriteTarget},
		{"comment missing issue", SafeWriteTarget{Operation: SafeWriteOperationAddComment, Repository: "owner/repo", Body: "body"}, ErrInvalidWriteTarget},
		{"comment missing body", SafeWriteTarget{Operation: SafeWriteOperationAddComment, Repository: "owner/repo", Issue: 1}, ErrInvalidWriteTarget},
		{"comment has title", SafeWriteTarget{Operation: SafeWriteOperationAddComment, Repository: "owner/repo", Issue: 1, Body: "body", Title: "title"}, ErrInvalidWriteTarget},
		{"labels missing issue", SafeWriteTarget{Operation: SafeWriteOperationSetLabels, Repository: "owner/repo"}, ErrInvalidWriteTarget},
		{"labels has title", SafeWriteTarget{Operation: SafeWriteOperationSetLabels, Repository: "owner/repo", Issue: 1, Title: "title"}, ErrInvalidWriteTarget},
		{"labels has body", SafeWriteTarget{Operation: SafeWriteOperationSetLabels, Repository: "owner/repo", Issue: 1, Body: "body"}, ErrInvalidWriteTarget},
		{"labels has blank", SafeWriteTarget{Operation: SafeWriteOperationSetLabels, Repository: "owner/repo", Issue: 1, Labels: []string{" "}}, ErrInvalidWriteTarget},
		{"labels duplicate", SafeWriteTarget{Operation: SafeWriteOperationSetLabels, Repository: "owner/repo", Issue: 1, Labels: []string{"bug", "bug"}}, ErrInvalidWriteTarget},
		{"unsupported operation", SafeWriteTarget{Operation: "github.push", Repository: "owner/repo"}, ErrUnsupportedWrite},
	}
	for _, testCase := range invalidTargets {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.target.Validate(); !errors.Is(err, testCase.want) {
				t.Fatalf("target validation = %v, want %v", err, testCase.want)
			}
		})
	}
	for name, target := range map[string]SafeWriteTarget{"create": validCreate, "comment": validComment, "labels": validLabels} {
		t.Run("valid "+name, func(t *testing.T) {
			if err := target.Validate(); err != nil {
				t.Fatalf("valid target rejected: %v", err)
			}
			if hash, err := target.ActionHash(); err != nil || len(hash) != 64 {
				t.Fatalf("action hash = %q, err=%v", hash, err)
			}
		})
	}

	requestCases := []struct {
		name string
		err  error
	}{
		{"create missing idempotency", (CreateIssueRequest{Repository: "owner/repo", Title: "title"}).Validate()},
		{"comment missing idempotency", (CommentIssueRequest{Repository: "owner/repo", Issue: 1, Body: "body"}).Validate()},
		{"labels missing idempotency", (LabelIssueRequest{Repository: "owner/repo", Issue: 1, Labels: []string{"bug"}}).Validate()},
	}
	for _, testCase := range requestCases {
		if !errors.Is(testCase.err, ErrMissingIdempotencyKey) {
			t.Fatalf("%s error = %v, want missing idempotency", testCase.name, testCase.err)
		}
	}

	if typ, id := validCreate.TargetTypeAndID(); typ != "github_repository" || id != "owner/repo" {
		t.Fatalf("create target identity = %q/%q", typ, id)
	}
	if typ, id := validComment.TargetTypeAndID(); typ != "github_issue" || id != "owner/repo#4" {
		t.Fatalf("comment target identity = %q/%q", typ, id)
	}
	if typ, id := validLabels.TargetTypeAndID(); typ != "github_issue" || id != "owner/repo#4" {
		t.Fatalf("labels target identity = %q/%q", typ, id)
	}

	now := connectorTestNow
	valid, err := NewWorkspaceSafeWriteChallenge("workspace-1", "challenge-1", "user-1", validCreate, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := valid.Validate(now); err != nil {
		t.Fatalf("valid challenge rejected: %v", err)
	}
	challengeInvalid := []struct {
		name string
		make func() SafeWriteChallenge
	}{
		{"missing id", func() SafeWriteChallenge { c := valid; c.ID = ""; return c }},
		{"missing user", func() SafeWriteChallenge { c := valid; c.UserID = ""; return c }},
		{"control workspace", func() SafeWriteChallenge { c := valid; c.WorkspaceID = "workspace\x00"; return c }},
		{"zero created", func() SafeWriteChallenge { c := valid; c.CreatedAt = time.Time{}; return c }},
		{"expires before created", func() SafeWriteChallenge { c := valid; c.ExpiresAt = c.CreatedAt; return c }},
		{"expires too far", func() SafeWriteChallenge {
			c := valid
			c.ExpiresAt = c.CreatedAt.Add(SafeWriteChallengeTTL + time.Second)
			return c
		}},
		{"used", func() SafeWriteChallenge { c := valid; used := now; c.UsedAt = &used; return c }},
	}
	for _, testCase := range challengeInvalid {
		t.Run("challenge "+testCase.name, func(t *testing.T) {
			if err := testCase.make().Validate(now); !errors.Is(err, ErrInvalidChallenge) && !errors.Is(err, ErrChallengeUsed) {
				t.Fatalf("challenge validation = %v", err)
			}
		})
	}
	if err := valid.Validate(valid.ExpiresAt); !errors.Is(err, ErrChallengeExpired) {
		t.Fatalf("expired challenge = %v", err)
	}

	confirmation := valid.Confirmation(now)
	confirmationCases := []struct {
		name string
		mut  func(*SafeWriteMetadata)
		want error
	}{
		{"missing user", func(m *SafeWriteMetadata) { m.UserID = "" }, ErrMissingUserID},
		{"missing challenge", func(m *SafeWriteMetadata) { m.ChallengeID = "" }, ErrMissingChallenge},
		{"bad hash", func(m *SafeWriteMetadata) { m.ActionHash = "other" }, ErrInvalidChallenge},
		{"confirmed in future", func(m *SafeWriteMetadata) { m.ConfirmedAt = now.Add(time.Second) }, ErrChallengeExpired},
		{"expired", func(m *SafeWriteMetadata) { m.ExpiresAt = now }, ErrChallengeExpired},
		{"expires before confirmation", func(m *SafeWriteMetadata) { m.ExpiresAt = now.Add(-time.Second) }, ErrChallengeExpired},
		{"ttl too long", func(m *SafeWriteMetadata) { m.ExpiresAt = m.ConfirmedAt.Add(SafeWriteChallengeTTL + time.Second) }, ErrChallengeExpired},
	}
	for _, testCase := range confirmationCases {
		t.Run("confirmation "+testCase.name, func(t *testing.T) {
			candidate := confirmation
			testCase.mut(&candidate)
			if err := candidate.ValidateConfirmation(now, validCreate); !errors.Is(err, testCase.want) {
				t.Fatalf("confirmation validation = %v, want %v", err, testCase.want)
			}
		})
	}
	if err := confirmation.ValidateConfirmation(now, validCreate); err != nil {
		t.Fatalf("valid confirmation rejected: %v", err)
	}

	if err := validateWrittenIssue(GitHubIssue{Repository: "owner/repo", Number: 1}, validCreate); err != nil {
		t.Fatalf("valid issue payload rejected: %v", err)
	}
	if err := validateWrittenComment(GitHubComment{Repository: "owner/repo", Issue: 4, ID: 1}, validComment); err != nil {
		t.Fatalf("valid comment payload rejected: %v", err)
	}
	if err := validateWrittenLabels([]GitHubLabel{{Repository: "owner/repo", Issue: 4, Name: "bug"}}, validLabels); err != nil {
		t.Fatalf("valid labels payload rejected: %v", err)
	}
	for name, issue := range map[string]GitHubIssue{
		"missing repository": {Number: 1},
		"missing number":     {Repository: "owner/repo"},
		"wrong repository":   {Repository: "other/repo", Number: 1},
	} {
		if err := validateWrittenIssue(issue, validCreate); !errors.Is(err, ErrInvalidProviderPayload) {
			t.Fatalf("invalid issue %s = %v", name, err)
		}
	}
	for name, comment := range map[string]GitHubComment{
		"missing repository": {Issue: 4, ID: 1},
		"wrong repository":   {Repository: "other/repo", Issue: 4, ID: 1},
		"wrong issue":        {Repository: "owner/repo", Issue: 5, ID: 1},
		"missing id":         {Repository: "owner/repo", Issue: 4},
	} {
		if err := validateWrittenComment(comment, validComment); !errors.Is(err, ErrInvalidProviderPayload) {
			t.Fatalf("invalid comment %s = %v", name, err)
		}
	}
	for name, labels := range map[string][]GitHubLabel{
		"missing repository": {{Issue: 4, Name: "bug"}},
		"wrong repository":   {{Repository: "other/repo", Issue: 4, Name: "bug"}},
		"wrong issue":        {{Repository: "owner/repo", Issue: 5, Name: "bug"}},
		"blank name":         {{Repository: "owner/repo", Issue: 4}},
	} {
		if err := validateWrittenLabels(labels, validLabels); !errors.Is(err, ErrInvalidProviderPayload) {
			t.Fatalf("invalid labels %s = %v", name, err)
		}
	}
}

func TestSafeWriteEntityTrashTargetValidation(t *testing.T) {
	validTargets := []SafeWriteTarget{
		{Operation: SafeWriteOperationEntityTrash, EntityType: "project", EntityID: "project-1", ExpectedVersion: 1},
		{Operation: SafeWriteOperationEntityTrash, EntityType: "task", EntityID: "task-1", ExpectedVersion: 2},
		{Operation: SafeWriteOperationEntityTrash, EntityType: "decision", EntityID: "decision-1", ExpectedVersion: 3},
	}
	for _, target := range validTargets {
		if err := target.Validate(); err != nil {
			t.Fatalf("valid entity trash target rejected: %+v: %v", target, err)
		}
		wantType := "work_" + target.EntityType
		if targetType, targetID := target.TargetTypeAndID(); targetType != wantType || targetID != target.EntityID {
			t.Fatalf("entity trash target identity = %q/%q, want %q/%q", targetType, targetID, wantType, target.EntityID)
		}
		if provider := ProviderForSafeWriteOperation(target.Operation); provider != ProviderWork {
			t.Fatalf("entity trash provider = %q, want %q", provider, ProviderWork)
		}
	}

	longIdentifier := strings.Repeat("x", 33)
	longEntityID := strings.Repeat("x", 257)
	invalidTargets := []struct {
		name   string
		target SafeWriteTarget
	}{
		{name: "blank entity type", target: SafeWriteTarget{Operation: SafeWriteOperationEntityTrash, EntityID: "project-1", ExpectedVersion: 1}},
		{name: "trimmed entity type", target: SafeWriteTarget{Operation: SafeWriteOperationEntityTrash, EntityType: " project", EntityID: "project-1", ExpectedVersion: 1}},
		{name: "spaced entity type", target: SafeWriteTarget{Operation: SafeWriteOperationEntityTrash, EntityType: "pro ject", EntityID: "project-1", ExpectedVersion: 1}},
		{name: "control entity type", target: SafeWriteTarget{Operation: SafeWriteOperationEntityTrash, EntityType: "project\x00", EntityID: "project-1", ExpectedVersion: 1}},
		{name: "long entity type", target: SafeWriteTarget{Operation: SafeWriteOperationEntityTrash, EntityType: longIdentifier, EntityID: "project-1", ExpectedVersion: 1}},
		{name: "blank entity id", target: SafeWriteTarget{Operation: SafeWriteOperationEntityTrash, EntityType: "project", ExpectedVersion: 1}},
		{name: "trimmed entity id", target: SafeWriteTarget{Operation: SafeWriteOperationEntityTrash, EntityType: "project", EntityID: " project-1", ExpectedVersion: 1}},
		{name: "control entity id", target: SafeWriteTarget{Operation: SafeWriteOperationEntityTrash, EntityType: "project", EntityID: "project\x00-1", ExpectedVersion: 1}},
		{name: "long entity id", target: SafeWriteTarget{Operation: SafeWriteOperationEntityTrash, EntityType: "project", EntityID: longEntityID, ExpectedVersion: 1}},
		{name: "missing expected version", target: SafeWriteTarget{Operation: SafeWriteOperationEntityTrash, EntityType: "project", EntityID: "project-1"}},
		{name: "unexpected github fields", target: SafeWriteTarget{Operation: SafeWriteOperationEntityTrash, EntityType: "project", EntityID: "project-1", ExpectedVersion: 1, Repository: "owner/repo"}},
		{name: "mixed entity fields", target: SafeWriteTarget{Operation: SafeWriteOperationCreateIssue, EntityType: "project", Repository: "owner/repo", Title: "title"}},
	}
	for _, testCase := range invalidTargets {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.target.Validate(); !errors.Is(err, ErrInvalidWriteTarget) {
				t.Fatalf("target validation = %v, want %v", err, ErrInvalidWriteTarget)
			}
		})
	}
}

type noReaderChallengeStore struct{}

func (noReaderChallengeStore) Consume(context.Context, SafeWriteMetadata, SafeWriteTarget, time.Time) error {
	return nil
}

func TestSafeWriteServiceConstructorAndNoReaderPath(t *testing.T) {
	writer := &FakeGitHubWriter{NextIssueNumber: 1}
	if _, err := NewGitHubSafeWriteService(nil, noReaderChallengeStore{}, NewMemorySafeWriteReceiptStore()); err == nil {
		t.Fatal("nil writer was accepted")
	}
	if _, err := NewGitHubSafeWriteService(writer, nil, NewMemorySafeWriteReceiptStore()); err == nil {
		t.Fatal("nil challenge store was accepted")
	}
	if _, err := NewGitHubSafeWriteService(writer, noReaderChallengeStore{}, nil); err == nil {
		t.Fatal("nil receipt store was accepted")
	}

	receipts := NewMemorySafeWriteReceiptStore()
	service, err := NewGitHubSafeWriteService(writer, noReaderChallengeStore{}, receipts)
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return connectorTestNow }
	request := CreateIssueRequest{Repository: "owner/repo", Title: "no-reader", Metadata: SafeWriteMetadata{UserID: "user-1", IdempotencyKey: "no-reader-key"}}
	challenge, err := NewSafeWriteChallenge("not-persisted", "user-1", request.Target(), connectorTestNow)
	if err != nil {
		t.Fatal(err)
	}
	request.Metadata = challenge.Confirmation(connectorTestNow)
	request.Metadata.IdempotencyKey = "no-reader-key"
	outcome, err := service.CreateIssue(context.Background(), request)
	if err != nil || outcome.Receipt.Issue.Number != 1 {
		t.Fatalf("no-reader write = %+v, err=%v", outcome, err)
	}

	lookupFailure := &errorReceiptStore{err: errors.New("receipt lookup failed")}
	service.Receipts = lookupFailure
	if _, err := service.CreateIssue(context.Background(), request); err == nil || !strings.Contains(err.Error(), "lookup write receipt") {
		t.Fatalf("receipt lookup failure = %v", err)
	}

	if _, err := service.CreateIssue(context.Background(), CreateIssueRequest{Repository: "owner/repo", Title: "title", Metadata: SafeWriteMetadata{UserID: "user-1"}}); !errors.Is(err, ErrMissingIdempotencyKey) {
		t.Fatalf("metadata validation = %v", err)
	}
	if _, err := service.CreateIssue(context.Background(), CreateIssueRequest{Repository: "owner/repo", Title: "title", Metadata: SafeWriteMetadata{IdempotencyKey: "missing-user"}}); !errors.Is(err, ErrMissingUserID) {
		t.Fatalf("missing user validation = %v", err)
	}
}

type errorReceiptStore struct{ err error }

func (s *errorReceiptStore) Lookup(context.Context, string, string, SafeWriteOperation, string) (SafeWriteReceipt, bool, error) {
	return SafeWriteReceipt{}, false, s.err
}

func (s *errorReceiptStore) Save(context.Context, SafeWriteReceipt) error { return nil }

func TestSafeWriteProviderAndRetryBoundaries(t *testing.T) {
	if got := ClassifyError(NewProviderError("github", "request", http.StatusInternalServerError, "temporary")); got != RetryTransient {
		t.Fatalf("provider 500 class = %q", got)
	}
	if got := ClassifyError(NewProviderError("github", "request", http.StatusBadRequest, "invalid")); got != RetryInvalidRequest {
		t.Fatalf("provider 400 class = %q", got)
	}
	if got := ClassifyError(NewProviderError("github", "request", http.StatusForbidden, "forbidden")); got != RetryAuthorization {
		t.Fatalf("provider 403 class = %q", got)
	}
	if got := ClassifyError(NewProviderError("github", "request", http.StatusNotFound, "missing")); got != RetryNotFound {
		t.Fatalf("provider 404 class = %q", got)
	}
	if got := ClassifyError(NewProviderError("github", "request", http.StatusConflict, "conflict")); got != RetryConflict {
		t.Fatalf("provider 409 class = %q", got)
	}
	if got := ClassifyError(NewProviderError("github", "request", http.StatusRequestTimeout, "timeout")); got != RetryTransient {
		t.Fatalf("provider 408 class = %q", got)
	}
	if got := ClassifyError(NewProviderError("github", "request", http.StatusTooEarly, "early")); got != RetryTransient {
		t.Fatalf("provider 425 class = %q", got)
	}
}
