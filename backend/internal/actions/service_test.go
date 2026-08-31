package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

func TestServiceConfirmReplayReturnsReceiptWithoutCallingProviderAgain(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	target := connectors.SafeWriteTarget{
		Operation:  connectors.SafeWriteOperationCreateIssue,
		Repository: "owner/repo",
		Title:      "Persist action receipt",
		Body:       "Keep retries deterministic.",
	}
	writer := &connectors.FakeGitHubWriter{NextIssueNumber: 17}
	challengeStore := connectors.NewMemorySafeWriteChallengeStore()
	receiptStore := connectors.NewMemorySafeWriteReceiptStore()
	guarded, err := connectors.NewGitHubSafeWriteService(writer, challengeStore, receiptStore)
	if err != nil {
		t.Fatal(err)
	}
	guarded.Clock = func() time.Time { return now }
	service, err := NewService(guarded, challengeStore, challengeStore)
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return now }
	service.Random = bytes.NewReader(bytes.Repeat([]byte{0x42}, 32))
	scope := Scope{WorkspaceID: "workspace-a", UserID: "user-a"}

	challenge, err := service.CreateChallenge(context.Background(), scope, CreateChallengeRequest{Target: target, IdempotencyKey: "preview-1"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Confirm(context.Background(), scope, challenge.Binding.ChallengeID, target, "confirm-1")
	if err != nil {
		t.Fatal(err)
	}
	if first.Replayed || first.Receipt.Status != connectors.SafeWriteReceiptAccepted || first.Receipt.TargetType != "github_repository" {
		t.Fatalf("first confirmation = %+v", first)
	}

	second, err := service.Confirm(context.Background(), scope, challenge.Binding.ChallengeID, target, "confirm-1")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed || second.Receipt.ID != first.Receipt.ID {
		t.Fatalf("replayed confirmation = %+v", second)
	}
	if len(writer.CreateCalls) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(writer.CreateCalls))
	}
}

func TestServiceRejectsChallengeTargetMismatchBeforeProvider(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	target := connectors.SafeWriteTarget{Operation: connectors.SafeWriteOperationCreateIssue, Repository: "owner/repo", Title: "Original"}
	writer := &connectors.FakeGitHubWriter{}
	challengeStore := connectors.NewMemorySafeWriteChallengeStore()
	guarded, err := connectors.NewGitHubSafeWriteService(writer, challengeStore, connectors.NewMemorySafeWriteReceiptStore())
	if err != nil {
		t.Fatal(err)
	}
	guarded.Clock = func() time.Time { return now }
	service, err := NewService(guarded, challengeStore, challengeStore)
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return now }
	service.Random = bytes.NewReader(bytes.Repeat([]byte{0x24}, 32))
	scope := Scope{WorkspaceID: "workspace-a", UserID: "user-a"}
	challenge, err := service.CreateChallenge(context.Background(), scope, CreateChallengeRequest{Target: target, IdempotencyKey: "preview-2"})
	if err != nil {
		t.Fatal(err)
	}
	changed := target
	changed.Title = "Tampered"
	_, err = service.Confirm(context.Background(), scope, challenge.Binding.ChallengeID, changed, "confirm-2")
	if !errors.Is(err, ErrChallengeMismatch) {
		t.Fatalf("mismatch error = %v, want ErrChallengeMismatch", err)
	}
	if len(writer.CreateCalls) != 0 {
		t.Fatal("mismatched challenge reached provider")
	}
}

type actionIssuerError struct{ err error }

func (i actionIssuerError) Put(connectors.SafeWriteChallenge) error { return i.err }

type actionReaderError struct {
	challenge connectors.SafeWriteChallenge
	err       error
}

func (r actionReaderError) Get(context.Context, string, string, string) (connectors.SafeWriteChallenge, error) {
	if r.err != nil {
		return connectors.SafeWriteChallenge{}, r.err
	}
	return r.challenge, nil
}

type actionFailingRandom struct{}

func (actionFailingRandom) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type actionShortRandom struct {
	reads int
}

func (r *actionShortRandom) Read(p []byte) (int, error) {
	if r.reads > 0 {
		return 0, io.ErrUnexpectedEOF
	}
	r.reads++
	return len(p), nil
}

func newActionServiceFixture(t *testing.T) (*Service, *connectors.FakeGitHubWriter, *connectors.MemorySafeWriteChallengeStore) {
	t.Helper()
	writer := &connectors.FakeGitHubWriter{NextIssueNumber: 31}
	challenges := connectors.NewMemorySafeWriteChallengeStore()
	guarded, err := connectors.NewGitHubSafeWriteService(writer, challenges, connectors.NewMemorySafeWriteReceiptStore())
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(guarded, challenges, challenges)
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC) }
	guarded.Clock = service.Clock
	service.Random = bytes.NewReader(bytes.Repeat([]byte{0x7a}, 64))
	return service, writer, challenges
}

func TestServiceValidatesConstructionScopeAndRequestBoundaries(t *testing.T) {
	challenges := connectors.NewMemorySafeWriteChallengeStore()
	guarded, err := connectors.NewGitHubSafeWriteService(&connectors.FakeGitHubWriter{}, challenges, connectors.NewMemorySafeWriteReceiptStore())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewService(nil, challenges, challenges); !errors.Is(err, ErrActionUnavailable) {
		t.Fatalf("nil GitHub service error = %v", err)
	}
	if _, err := NewService(guarded, nil, challenges); !errors.Is(err, ErrActionUnavailable) {
		t.Fatalf("nil issuer error = %v", err)
	}
	if _, err := NewService(guarded, challenges, nil); !errors.Is(err, ErrActionUnavailable) {
		t.Fatalf("nil reader error = %v", err)
	}

	service, _, _ := newActionServiceFixture(t)
	target := connectors.SafeWriteTarget{Operation: connectors.SafeWriteOperationCreateIssue, Repository: "owner/repo", Title: "Issue"}
	if _, err := service.CreateChallenge(context.Background(), Scope{WorkspaceID: "", UserID: "user"}, CreateChallengeRequest{Target: target}); !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("invalid scope error = %v", err)
	}
	if _, err := service.CreateChallenge(context.Background(), Scope{WorkspaceID: "workspace", UserID: " user"}, CreateChallengeRequest{Target: target}); !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("invalid user scope error = %v", err)
	}
	if _, err := service.CreateChallenge(context.Background(), Scope{WorkspaceID: "workspace", UserID: strings.Repeat("u", maxIDBytes+1)}, CreateChallengeRequest{Target: target}); !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("oversized user scope error = %v", err)
	}
	if _, err := service.CreateChallenge(context.Background(), Scope{WorkspaceID: "workspace", UserID: "user"}, CreateChallengeRequest{Target: connectors.SafeWriteTarget{}}); err == nil {
		t.Fatal("invalid target unexpectedly accepted")
	}
	unsupported := target
	unsupported.Operation = "github.push"
	if _, err := service.CreateChallenge(context.Background(), Scope{WorkspaceID: "workspace", UserID: "user"}, CreateChallengeRequest{Target: unsupported}); !errors.Is(err, connectors.ErrUnsupportedWrite) {
		t.Fatalf("unsupported operation error = %v", err)
	}
	if _, err := service.CreateChallenge(context.Background(), Scope{WorkspaceID: "workspace", UserID: "user"}, CreateChallengeRequest{Target: target, IdempotencyKey: " leading"}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("leading-space idempotency error = %v", err)
	}
	if _, err := service.CreateChallenge(context.Background(), Scope{WorkspaceID: "workspace", UserID: "user"}, CreateChallengeRequest{Target: target, IdempotencyKey: strings.Repeat("x", maxIdempotency+1)}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("long idempotency error = %v", err)
	}

	issuerFailure := fmt.Errorf("issuer failed")
	service.Issuer = actionIssuerError{err: issuerFailure}
	if _, err := service.CreateChallenge(context.Background(), Scope{WorkspaceID: "workspace", UserID: "user"}, CreateChallengeRequest{Target: target, IdempotencyKey: "preview"}); !errors.Is(err, issuerFailure) {
		t.Fatalf("issuer error = %v", err)
	}
}

func TestServiceGeneratesIdempotencyAndHandlesRandomFailures(t *testing.T) {
	service, _, _ := newActionServiceFixture(t)
	scope := Scope{WorkspaceID: "workspace", UserID: "user"}
	target := connectors.SafeWriteTarget{Operation: connectors.SafeWriteOperationCreateIssue, Repository: "owner/repo", Title: "Generated key"}
	challenge, err := service.CreateChallenge(context.Background(), scope, CreateChallengeRequest{Target: target})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(challenge.Binding.IdempotencyKey, "action-") {
		t.Fatalf("generated idempotency key = %q", challenge.Binding.IdempotencyKey)
	}

	service.Random = actionFailingRandom{}
	if _, err := service.CreateChallenge(context.Background(), scope, CreateChallengeRequest{Target: target, IdempotencyKey: "preview"}); !strings.Contains(err.Error(), "allocate action identity") {
		t.Fatalf("random failure error = %v", err)
	}
	service.Random = &actionShortRandom{}
	if _, err := service.CreateChallenge(context.Background(), scope, CreateChallengeRequest{Target: target}); !strings.Contains(err.Error(), "allocate action identity") {
		t.Fatalf("generated-key random failure error = %v", err)
	}
}

func TestServiceConfirmRejectsInvalidReaderStateAndRoutesAllSupportedWrites(t *testing.T) {
	service, writer, challenges := newActionServiceFixture(t)
	scope := Scope{WorkspaceID: "workspace", UserID: "user"}
	createTarget := connectors.SafeWriteTarget{Operation: connectors.SafeWriteOperationCreateIssue, Repository: "owner/repo", Title: "Create"}
	if _, err := service.Confirm(context.Background(), scope, "missing", createTarget, "confirm"); !errors.Is(err, ErrChallengeNotFound) {
		t.Fatalf("missing challenge error = %v", err)
	}
	if _, err := service.Confirm(context.Background(), scope, "", createTarget, "confirm"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("empty challenge ID error = %v", err)
	}
	if _, err := service.Confirm(context.Background(), scope, "challenge", createTarget, ""); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("empty idempotency key error = %v", err)
	}
	service.Reader = actionReaderError{err: context.DeadlineExceeded}
	if _, err := service.Confirm(context.Background(), scope, "challenge", createTarget, "confirm"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("reader failure = %v", err)
	}

	hash, err := createTarget.ActionHash()
	if err != nil {
		t.Fatal(err)
	}
	malformed := connectors.SafeWriteChallenge{ID: "challenge", WorkspaceID: scope.WorkspaceID, UserID: scope.UserID, TargetType: "github_repository", TargetID: "owner/repo", ActionHash: hash, CreatedAt: time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)}
	service.Reader = actionReaderError{challenge: malformed}
	if _, err := service.Confirm(context.Background(), scope, "challenge", createTarget, "confirm"); !errors.Is(err, ErrInvalidActionState) {
		t.Fatalf("malformed challenge error = %v", err)
	}

	validChallenge, err := connectors.NewWorkspaceSafeWriteChallenge(scope.WorkspaceID, "challenge-mismatch", scope.UserID, createTarget, service.Clock())
	if err != nil {
		t.Fatal(err)
	}
	changedScopeChallenge := validChallenge
	changedScopeChallenge.WorkspaceID = "other-workspace"
	service.Reader = actionReaderError{challenge: changedScopeChallenge}
	if _, err := service.Confirm(context.Background(), scope, "challenge-mismatch", createTarget, "confirm"); !errors.Is(err, ErrChallengeMismatch) {
		t.Fatalf("workspace mismatch error = %v", err)
	}
	service.Reader = challenges

	testCases := []struct {
		name   string
		target connectors.SafeWriteTarget
		check  func(t *testing.T)
	}{
		{name: "comment", target: connectors.SafeWriteTarget{Operation: connectors.SafeWriteOperationAddComment, Repository: "owner/repo", Issue: 9, Body: "Comment"}, check: func(t *testing.T) {
			if len(writer.CommentCalls) != 1 {
				t.Fatalf("comment calls = %d", len(writer.CommentCalls))
			}
		}},
		{name: "labels", target: connectors.SafeWriteTarget{Operation: connectors.SafeWriteOperationSetLabels, Repository: "owner/repo", Issue: 9, Labels: []string{"bug", "backend"}}, check: func(t *testing.T) {
			if len(writer.LabelCalls) != 1 {
				t.Fatalf("label calls = %d", len(writer.LabelCalls))
			}
		}},
	}
	for index, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			challengeID := fmt.Sprintf("challenge-route-%d", index)
			challenge, err := connectors.NewWorkspaceSafeWriteChallenge(scope.WorkspaceID, challengeID, scope.UserID, test.target, service.Clock())
			if err != nil {
				t.Fatal(err)
			}
			if err := challenges.Put(challenge); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Confirm(context.Background(), scope, challengeID, test.target, fmt.Sprintf("confirm-route-%d", index)); err != nil {
				t.Fatal(err)
			}
			test.check(t)
		})
	}
	if len(writer.CreateCalls) != 0 {
		t.Fatalf("unexpected create calls = %d", len(writer.CreateCalls))
	}
}

func TestServiceConfirmReturnsProviderFailure(t *testing.T) {
	service, writer, challenges := newActionServiceFixture(t)
	writer.Failure = errors.New("provider unavailable")
	scope := Scope{WorkspaceID: "workspace", UserID: "user"}
	target := connectors.SafeWriteTarget{Operation: connectors.SafeWriteOperationCreateIssue, Repository: "owner/repo", Title: "Provider failure"}
	challenge, err := service.CreateChallenge(context.Background(), scope, CreateChallengeRequest{Target: target, IdempotencyKey: "preview"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Confirm(context.Background(), scope, challenge.Binding.ChallengeID, target, "confirm"); !errors.Is(err, writer.Failure) {
		t.Fatalf("provider error = %v", err)
	}
	if _, err := challenges.Get(context.Background(), scope.WorkspaceID, scope.UserID, challenge.Binding.ChallengeID); err != nil {
		t.Fatalf("challenge lookup after provider failure = %v", err)
	}
}

func TestServiceClockFallbackIsSafe(t *testing.T) {
	service, _, _ := newActionServiceFixture(t)
	service.Clock = nil
	if service.now().IsZero() {
		t.Fatal("nil clock returned zero time")
	}
	var nilService *Service
	if nilService.now().IsZero() {
		t.Fatal("nil service clock returned zero time")
	}
}

func TestServiceUsesDefaultRandomSourceWhenUnset(t *testing.T) {
	service, _, _ := newActionServiceFixture(t)
	service.Random = nil
	_, err := service.CreateChallenge(context.Background(), Scope{WorkspaceID: "workspace", UserID: "user"}, CreateChallengeRequest{
		Target:         connectors.SafeWriteTarget{Operation: connectors.SafeWriteOperationCreateIssue, Repository: "owner/repo", Title: "Default random"},
		IdempotencyKey: "preview-default-random",
	})
	if err != nil {
		t.Fatalf("default random source error = %v", err)
	}
}

func TestServiceConfirmWorkTrashRequiresChallengeAndReplaysReceipt(t *testing.T) {
	service, workService, challenges := newWorkActionServiceFixture(t)
	scope := Scope{WorkspaceID: "workspace-work", UserID: "user-work"}
	project, err := workService.CreateProject(context.Background(), work.Scope(scope), work.CreateProjectInput{Name: "Action target"}, "project-create")
	if err != nil {
		t.Fatal(err)
	}
	target := connectors.SafeWriteTarget{
		Operation:  connectors.SafeWriteOperationEntityTrash,
		EntityType: string(work.EntityProject), EntityID: project.ID, ExpectedVersion: project.Version,
	}
	challenge, err := service.CreateChallenge(context.Background(), scope, CreateChallengeRequest{Target: target, IdempotencyKey: "work-preview"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Confirm(context.Background(), scope, challenge.Binding.ChallengeID, target, "work-trash")
	if err != nil {
		t.Fatal(err)
	}
	if first.Replayed || first.Receipt.Provider != connectors.ProviderWork || first.Receipt.Status != connectors.SafeWriteReceiptAccepted || first.Receipt.TargetType != "work_project" {
		t.Fatalf("first work confirmation = %+v", first)
	}
	second, err := service.Confirm(context.Background(), scope, challenge.Binding.ChallengeID, target, "work-trash")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed || second.Receipt.ID != first.Receipt.ID {
		t.Fatalf("work replay = %+v, first = %+v", second, first)
	}
	if _, err := workService.GetProject(context.Background(), work.Scope(scope), project.ID); !errors.Is(err, work.ErrNotFound) {
		t.Fatalf("trashed project lookup error = %v, want ErrNotFound", err)
	}
	stored, err := challenges.Get(context.Background(), scope.WorkspaceID, scope.UserID, challenge.Binding.ChallengeID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.UsedAt == nil {
		t.Fatal("work challenge was not consumed")
	}
}

func TestServiceConfirmWorkTrashChecksVersionBeforeConsumingChallenge(t *testing.T) {
	service, workService, challenges := newWorkActionServiceFixture(t)
	scope := Scope{WorkspaceID: "workspace-stale", UserID: "user-stale"}
	project, err := workService.CreateProject(context.Background(), work.Scope(scope), work.CreateProjectInput{Name: "Stale target"}, "stale-create")
	if err != nil {
		t.Fatal(err)
	}
	name := "Updated before confirmation"
	if _, err := workService.UpdateProject(context.Background(), work.Scope(scope), project.ID, work.ProjectPatch{Name: &name}, project.Version, "stale-update"); err != nil {
		t.Fatal(err)
	}
	target := connectors.SafeWriteTarget{
		Operation:  connectors.SafeWriteOperationEntityTrash,
		EntityType: string(work.EntityProject), EntityID: project.ID, ExpectedVersion: project.Version,
	}
	challenge, err := service.CreateChallenge(context.Background(), scope, CreateChallengeRequest{Target: target, IdempotencyKey: "stale-preview"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Confirm(context.Background(), scope, challenge.Binding.ChallengeID, target, "stale-trash"); !errors.Is(err, work.ErrVersionConflict) {
		t.Fatalf("stale confirmation error = %v, want version conflict", err)
	}
	stored, err := challenges.Get(context.Background(), scope.WorkspaceID, scope.UserID, challenge.Binding.ChallengeID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.UsedAt != nil {
		t.Fatal("stale work challenge was consumed before version validation")
	}
	if _, err := workService.GetProject(context.Background(), work.Scope(scope), project.ID); err != nil {
		t.Fatalf("stale project was mutated: %v", err)
	}
}

func newWorkActionServiceFixture(t *testing.T) (*Service, *work.Service, *connectors.MemorySafeWriteChallengeStore) {
	t.Helper()
	challenges := connectors.NewMemorySafeWriteChallengeStore()
	receipts := connectors.NewMemorySafeWriteReceiptStore()
	guarded, err := connectors.NewGitHubSafeWriteService(&connectors.FakeGitHubWriter{}, challenges, receipts)
	if err != nil {
		t.Fatal(err)
	}
	workService, err := work.NewService(work.NewMemoryRepository())
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewServiceWithWork(guarded, workService, challenges, challenges)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	service.Clock = func() time.Time { return now }
	service.Random = bytes.NewReader(bytes.Repeat([]byte{0x55}, 64))
	return service, workService, challenges
}
