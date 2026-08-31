package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func currentScope() Scope {
	return Scope{WorkspaceID: "workspace-1", UserID: "user-1"}
}

func TestContextRefNormalizeValidatesAndCanonicalizes(t *testing.T) {
	tests := []struct {
		name    string
		input   ContextRef
		want    ContextRef
		wantErr error
	}{
		{name: "empty", input: ContextRef{}, want: ContextRef{}},
		{
			name:  "canonicalizes type and id whitespace",
			input: ContextRef{Type: " TASK ", ID: " task-1 "},
			want:  ContextRef{Type: ContextTask, ID: "task-1"},
		},
		{name: "partial type", input: ContextRef{Type: ContextProject}, wantErr: ErrInvalidInput},
		{name: "partial id", input: ContextRef{ID: "project-1"}, wantErr: ErrInvalidInput},
		{name: "unsupported type", input: ContextRef{Type: "conversation", ID: "conversation-1"}, wantErr: ErrInvalidInput},
		{name: "control character id", input: ContextRef{Type: ContextSource, ID: "source\n1"}, wantErr: ErrInvalidInput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.input.Normalize()
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("Normalize() error = %v, want %v", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("Normalize() = %+v, want %+v", got, test.want)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func currentEvidence() []Evidence {
	return []Evidence{{
		Scope:      currentScope(),
		ID:         "evidence-1",
		SourceID:   "source-1",
		RevisionID: "revision-1",
		Title:      "Runbook",
		URI:        "https://example.test/runbook",
		Snippet:    "The API uses PostgreSQL for durable work data.",
		Freshness:  FreshCurrent,
	}}
}

func currentAction() SuggestedAction {
	return SuggestedAction{
		ID:    "action-1",
		Kind:  "work.task.create",
		Label: "Create a follow-up task",
	}
}

func currentBinding() ActionBinding {
	return ActionBinding{
		ActionID:       "action-1",
		Scope:          currentScope(),
		Provider:       "github",
		Operation:      "github.issue.create",
		ChallengeID:    "challenge-1",
		IdempotencyKey: "idempotency-1",
		ActionHash:     "hash-1",
		TargetType:     "github_repository",
		TargetID:       "owner/repo",
	}
}

func currentReceipt() ActionReceipt {
	return ActionReceipt{
		ID:             "receipt-1",
		ActionID:       "action-1",
		Scope:          currentScope(),
		Provider:       "github",
		Operation:      "github.issue.create",
		ChallengeID:    "challenge-1",
		IdempotencyKey: "idempotency-1",
		ActionHash:     "hash-1",
		TargetType:     "github_repository",
		TargetID:       "owner/repo",
		Status:         ReceiptAccepted,
	}
}

func currentAttachment() ActionReceiptAttachment {
	attachment, err := NewActionReceiptAttachment(currentBinding(), currentReceipt())
	if err != nil {
		panic(err)
	}
	return attachment
}

func TestNormalizeResponseReturnsGroundedContractForCurrentEvidence(t *testing.T) {
	response, err := NormalizeResponse(currentScope(), Draft{
		Answer:           "The API uses PostgreSQL for durable work data.",
		Citations:        []Citation{{EvidenceID: "evidence-1", Quote: "uses PostgreSQL"}},
		Unknowns:         []string{"  ", "The deployment time is unknown."},
		SuggestedActions: []SuggestedAction{currentAction()},
	}, currentEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if response.Grounding != Grounded || response.Answer == UnknownAnswer {
		t.Fatalf("expected grounded answer, got %+v", response)
	}
	if len(response.Evidence) != 1 || response.Evidence[0].EvidenceID != "evidence-1" {
		t.Fatalf("unexpected evidence: %+v", response.Evidence)
	}
	if !response.SuggestedActions[0].RequiresConfirmation {
		t.Fatal("suggested mutations must be confirmation-gated")
	}
	if response.StaleSources == nil || response.ActionReceipts == nil {
		t.Fatal("stable response arrays must not be null")
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"answer", "evidence", "unknowns", "staleSources", "suggestedActions", "actionReceipts"} {
		if _, ok := decoded[field]; !ok {
			t.Fatalf("missing stable response field %q: %s", field, encoded)
		}
	}
}

func TestNormalizeResponseFailsClosedWithoutCitations(t *testing.T) {
	response, err := NormalizeResponse(currentScope(), Draft{
		Answer:   "The system definitely stores everything in Redis.",
		Unknowns: []string{"  provider confidence is not evidence  "},
	}, currentEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if response.Grounding != Unknown || response.Answer != UnknownAnswer {
		t.Fatalf("ungrounded answer must fail closed: %+v", response)
	}
	if len(response.Evidence) != 0 || len(response.StaleSources) != 0 {
		t.Fatalf("unexpected fabricated evidence: %+v", response)
	}
	if len(response.Unknowns) != 2 {
		t.Fatalf("expected preserved and generated unknowns, got %+v", response.Unknowns)
	}
}

func TestNormalizeResponseLabelsStaleEvidenceAsInferred(t *testing.T) {
	evidence := currentEvidence()
	evidence[0].Freshness = FreshStale
	response, err := NormalizeResponse(currentScope(), Draft{
		Answer:    "The API uses PostgreSQL.",
		Citations: []Citation{{EvidenceID: "evidence-1", Quote: "uses PostgreSQL"}},
	}, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if response.Grounding != Inferred || !reflect.DeepEqual(response.StaleSources, []string{"source-1"}) {
		t.Fatalf("stale evidence must be visible: %+v", response)
	}
	if len(response.Unknowns) != 1 {
		t.Fatalf("expected stale freshness unknown, got %+v", response.Unknowns)
	}
}

func TestNormalizeResponseRejectsFabricatedOrUnsupportedCitations(t *testing.T) {
	_, err := NormalizeResponse(currentScope(), Draft{Answer: "answer", Citations: []Citation{{EvidenceID: "missing", Quote: "answer"}}}, currentEvidence())
	if !errors.Is(err, ErrEvidenceNotFound) {
		t.Fatalf("expected unknown evidence error, got %v", err)
	}
	_, err = NormalizeResponse(currentScope(), Draft{Answer: "answer", Citations: []Citation{{EvidenceID: "evidence-1", Quote: "not in snippet"}}}, currentEvidence())
	if !errors.Is(err, ErrQuoteNotSupported) {
		t.Fatalf("expected unsupported quote error, got %v", err)
	}
	_, err = NormalizeResponse(currentScope(), Draft{Answer: "The API uses PostgreSQL.", Citations: []Citation{{EvidenceID: "evidence-1", Quote: "uses PostgreSQL"}, {EvidenceID: "evidence-1", Quote: "uses PostgreSQL"}}}, currentEvidence())
	if !errors.Is(err, ErrDuplicateCitation) {
		t.Fatalf("expected duplicate citation error, got %v", err)
	}
}

func TestNormalizeResponseRequiresQuoteSupportedByEvidenceAndAnswer(t *testing.T) {
	_, err := NormalizeResponse(currentScope(), Draft{
		Answer:    "The API uses PostgreSQL.",
		Citations: []Citation{{EvidenceID: "evidence-1"}},
	}, currentEvidence())
	if !errors.Is(err, ErrQuoteRequired) {
		t.Fatalf("citation without quote must be rejected, got %v", err)
	}
	_, err = NormalizeResponse(currentScope(), Draft{
		Answer:    "The API uses SQLite.",
		Citations: []Citation{{EvidenceID: "evidence-1", Quote: "uses PostgreSQL"}},
	}, currentEvidence())
	if !errors.Is(err, ErrQuoteNotInAnswer) {
		t.Fatalf("unrelated cited answer must be rejected, got %v", err)
	}
}

func TestNormalizeResponseRejectsCrossScopeEvidence(t *testing.T) {
	for name, scope := range map[string]Scope{
		"workspace": {WorkspaceID: "workspace-2", UserID: "user-1"},
		"user":      {WorkspaceID: "workspace-1", UserID: "user-2"},
	} {
		t.Run(name, func(t *testing.T) {
			evidence := currentEvidence()
			evidence[0].Scope = scope
			_, err := NormalizeResponse(currentScope(), Draft{Answer: "answer"}, evidence)
			if !errors.Is(err, ErrScopeMismatch) {
				t.Fatalf("cross-scope evidence must be rejected, got %v", err)
			}
		})
	}
}

func TestNormalizeResponseValidatesEvidenceFreshnessAndIds(t *testing.T) {
	invalid := currentEvidence()
	invalid[0].Freshness = ""
	_, err := NormalizeResponse(currentScope(), Draft{Answer: "answer"}, invalid)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid freshness error, got %v", err)
	}
	invalid = currentEvidence()
	invalid = append(invalid, invalid[0])
	_, err = NormalizeResponse(currentScope(), Draft{Answer: "answer"}, invalid)
	if !errors.Is(err, ErrDuplicateEvidence) {
		t.Fatalf("expected duplicate evidence error, got %v", err)
	}
}

func TestAttachActionReceiptsRequiresSuggestedActionAndDoesNotExecuteIt(t *testing.T) {
	response, err := NormalizeResponse(currentScope(), Draft{
		Answer:           "The API uses PostgreSQL.",
		Citations:        []Citation{{EvidenceID: "evidence-1", Quote: "uses PostgreSQL"}},
		SuggestedActions: []SuggestedAction{currentAction()},
	}, currentEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if len(response.ActionReceipts) != 0 {
		t.Fatalf("model response must not contain receipts: %+v", response.ActionReceipts)
	}
	withReceipt, err := AttachActionReceipts(response, []ActionReceiptAttachment{currentAttachment()})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(withReceipt.ActionReceipts, []ActionReceipt{currentReceipt()}) {
		t.Fatalf("receipt was changed or fabricated: %+v", withReceipt.ActionReceipts)
	}
	invalidBinding := currentBinding()
	invalidBinding.ActionID = "not-suggested"
	invalidReceipt := currentReceipt()
	invalidReceipt.ID = "receipt-2"
	invalidReceipt.ActionID = "not-suggested"
	invalidAttachment, err := NewActionReceiptAttachment(invalidBinding, invalidReceipt)
	if err != nil {
		t.Fatal(err)
	}
	_, err = AttachActionReceipts(response, []ActionReceiptAttachment{invalidAttachment})
	if !errors.Is(err, ErrReceiptActionMissing) {
		t.Fatalf("receipt must refer to a suggested action, got %v", err)
	}
	if strings.Contains(err.Error(), "not-suggested") {
		t.Fatalf("missing action error must not echo the supplied identifier: %v", err)
	}
	_, err = AttachActionReceipts(response, []ActionReceiptAttachment{{Binding: currentBinding(), Receipt: currentReceipt()}})
	if !errors.Is(err, ErrUntrustedReceipt) {
		t.Fatalf("raw receipt attachments must be rejected: %v", err)
	}
}

func TestAttachActionReceiptsRequiresCanonicalScopeAndBinding(t *testing.T) {
	response, err := NormalizeResponse(currentScope(), Draft{
		Answer:           "The API uses PostgreSQL.",
		Citations:        []Citation{{EvidenceID: "evidence-1", Quote: "uses PostgreSQL"}},
		SuggestedActions: []SuggestedAction{currentAction()},
	}, currentEvidence())
	if err != nil {
		t.Fatal(err)
	}
	crossScopeBinding := currentBinding()
	crossScopeBinding.Scope.UserID = "user-2"
	crossScope := currentReceipt()
	crossScope.Scope.UserID = "user-2"
	crossScopeAttachment, err := NewActionReceiptAttachment(crossScopeBinding, crossScope)
	if err != nil {
		t.Fatal(err)
	}
	_, err = AttachActionReceipts(response, []ActionReceiptAttachment{crossScopeAttachment})
	if !errors.Is(err, ErrScopeMismatch) {
		t.Fatalf("cross-scope receipt must be rejected, got %v", err)
	}
	wrongBinding := currentAttachment()
	wrongBinding.Receipt.ActionHash = "different-hash"
	_, err = AttachActionReceipts(response, []ActionReceiptAttachment{wrongBinding})
	if !errors.Is(err, ErrReceiptBindingMismatch) {
		t.Fatalf("receipt with mismatched action hash must be rejected, got %v", err)
	}
	for field, mutate := range map[string]func(*ActionReceipt){
		"challenge":   func(receipt *ActionReceipt) { receipt.ChallengeID = "other-challenge" },
		"idempotency": func(receipt *ActionReceipt) { receipt.IdempotencyKey = "other-idempotency" },
	} {
		t.Run(field, func(t *testing.T) {
			mismatched := currentAttachment()
			mutate(&mismatched.Receipt)
			_, err := AttachActionReceipts(response, []ActionReceiptAttachment{mismatched})
			if !errors.Is(err, ErrReceiptBindingMismatch) {
				t.Fatalf("receipt with mismatched %s must be rejected, got %v", field, err)
			}
		})
	}
	uncertain := currentAttachment()
	uncertain.Receipt.Status = ReceiptUncertain
	withUncertain, err := AttachActionReceipts(response, []ActionReceiptAttachment{uncertain})
	if err != nil || withUncertain.ActionReceipts[0].Status != ReceiptUncertain {
		t.Fatalf("canonical uncertain receipt should be preserved: response=%+v err=%v", withUncertain, err)
	}
	replayedUncertain := currentAttachment()
	replayedUncertain.Receipt.Status = ReceiptUncertain
	replayedUncertain.Receipt.Replayed = true
	_, err = AttachActionReceipts(response, []ActionReceiptAttachment{replayedUncertain})
	if !errors.Is(err, ErrInvalidReceipt) {
		t.Fatalf("uncertain receipt cannot be marked replayed: %v", err)
	}
}

func TestNormalizeResponseRejectsInvalidActionAndReceiptData(t *testing.T) {
	_, err := NormalizeResponse(currentScope(), Draft{
		Answer:           "answer",
		SuggestedActions: []SuggestedAction{{ID: "action-1", Kind: "", Label: "Missing kind"}},
	}, currentEvidence())
	if !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("expected invalid action error, got %v", err)
	}
	response, err := NormalizeResponse(currentScope(), Draft{
		Answer:           "The API uses PostgreSQL.",
		Citations:        []Citation{{EvidenceID: "evidence-1", Quote: "uses PostgreSQL"}},
		SuggestedActions: []SuggestedAction{{ID: "action-1", Kind: "work.task.create", Label: "Create task"}},
	}, currentEvidence())
	if err != nil {
		t.Fatal(err)
	}
	invalidReceipt := currentAttachment()
	invalidReceipt.Receipt.Status = "unknown"
	_, err = AttachActionReceipts(response, []ActionReceiptAttachment{invalidReceipt})
	if !errors.Is(err, ErrInvalidReceipt) {
		t.Fatalf("expected invalid receipt error, got %v", err)
	}
}

type fakeRetriever struct {
	request RetrievalRequest
	items   []Evidence
	err     error
	called  bool
}

func (f *fakeRetriever) Search(_ context.Context, request RetrievalRequest) ([]Evidence, error) {
	f.called = true
	f.request = request
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}

type fakeGenerator struct {
	request GenerationRequest
	draft   Draft
	err     error
	called  bool
}

func (f *fakeGenerator) Generate(_ context.Context, request GenerationRequest) (Draft, error) {
	f.called = true
	f.request = request
	if f.err != nil {
		return Draft{}, f.err
	}
	return f.draft, nil
}

func TestServiceBindsWorkspaceBeforeGenerationAndGroundsOutput(t *testing.T) {
	retriever := &fakeRetriever{items: currentEvidence()}
	generator := &fakeGenerator{draft: Draft{
		Answer:    "The API uses PostgreSQL.",
		Citations: []Citation{{EvidenceID: "evidence-1", Quote: "uses PostgreSQL"}},
	}}
	service, err := NewService(retriever, generator)
	if err != nil {
		t.Fatal(err)
	}
	response, err := service.Ask(context.Background(), AskRequest{
		Scope:          currentScope(),
		ConversationID: "conversation-1",
		Message:        "What database does the API use?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !retriever.called || !generator.called || response.Grounding != Grounded {
		t.Fatalf("unexpected service flow: retriever=%v generator=%v response=%+v", retriever.called, generator.called, response)
	}
	if retriever.request.Scope != currentScope() || generator.request.Scope != currentScope() {
		t.Fatalf("workspace scope was not propagated: %+v %+v", retriever.request, generator.request)
	}
	if generator.request.Evidence[0].ID != "evidence-1" {
		t.Fatalf("evidence was not passed to generator: %+v", generator.request.Evidence)
	}
}

func TestServicePropagatesNormalizedCanonicalContext(t *testing.T) {
	retriever := &fakeRetriever{items: currentEvidence()}
	generator := &fakeGenerator{draft: Draft{
		Answer:    "The task is verified; the system uses PostgreSQL.",
		Citations: []Citation{{EvidenceID: "evidence-1", Quote: "uses PostgreSQL"}},
	}}
	service, err := NewService(retriever, generator)
	if err != nil {
		t.Fatal(err)
	}
	want := ContextRef{Type: ContextTask, ID: "task-1"}
	response, err := service.Ask(context.Background(), AskRequest{
		Scope:   currentScope(),
		Message: "What is the task status?",
		Context: ContextRef{Type: " TASK ", ID: "task-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Grounding != Grounded || retriever.request.Context != want || generator.request.Context != want {
		t.Fatalf("canonical context was not normalized and propagated: response=%+v retrieval=%+v generation=%+v", response, retriever.request.Context, generator.request.Context)
	}
}

func TestServiceDoesNotGenerateWithoutEvidence(t *testing.T) {
	retriever := &fakeRetriever{}
	generator := &fakeGenerator{draft: Draft{
		Answer:           "The model wants to invent a task.",
		SuggestedActions: []SuggestedAction{currentAction()},
	}}
	service, err := NewService(retriever, generator)
	if err != nil {
		t.Fatal(err)
	}
	response, err := service.Ask(context.Background(), AskRequest{Scope: currentScope(), Message: "What should I do?"})
	if err != nil {
		t.Fatal(err)
	}
	if generator.called {
		t.Fatal("generator must not run when retrieval returns no evidence")
	}
	if response.Grounding != Unknown || len(response.SuggestedActions) != 0 {
		t.Fatalf("empty retrieval must fail closed without actions: %+v", response)
	}
}

func TestServiceRedactsDependencyErrors(t *testing.T) {
	retriever := &fakeRetriever{err: fmt.Errorf("provider token=secret-value")}
	service, err := NewService(retriever, &fakeGenerator{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Ask(context.Background(), AskRequest{Scope: currentScope(), Message: "question"})
	if !errors.Is(err, ErrDependencyFailure) || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("dependency error must be classified and redacted: %v", err)
	}
}

func TestServiceRedactsGeneratorErrors(t *testing.T) {
	retriever := &fakeRetriever{items: currentEvidence()}
	generator := &fakeGenerator{err: fmt.Errorf("provider token=secret-value")}
	service, err := NewService(retriever, generator)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Ask(context.Background(), AskRequest{Scope: currentScope(), Message: "question"})
	if !errors.Is(err, ErrDependencyFailure) || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("generator error must be classified and redacted: %v", err)
	}
}

func TestServicePreservesCancellationAndDeadlineErrors(t *testing.T) {
	for name, dependencyErr := range map[string]error{
		"canceled": context.Canceled,
		"deadline": context.DeadlineExceeded,
	} {
		t.Run(name, func(t *testing.T) {
			service, err := NewService(&fakeRetriever{err: dependencyErr}, &fakeGenerator{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.Ask(context.Background(), AskRequest{Scope: currentScope(), Message: "question"})
			if !errors.Is(err, dependencyErr) {
				t.Fatalf("dependency %s should be preserved, got %v", name, err)
			}
		})
	}
}

func TestServiceRejectsCrossScopeRetrieverOutput(t *testing.T) {
	evidence := currentEvidence()
	evidence[0].Scope.UserID = "user-2"
	service, err := NewService(&fakeRetriever{items: evidence}, &fakeGenerator{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Ask(context.Background(), AskRequest{Scope: currentScope(), Message: "question"})
	if !errors.Is(err, ErrScopeMismatch) {
		t.Fatalf("retriever output must remain bound to authenticated scope, got %v", err)
	}
}

func TestServiceRejectsUnboundedOrControlInput(t *testing.T) {
	service, err := NewService(&fakeRetriever{}, &fakeGenerator{})
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range []string{"question\x00", strings.Repeat("x", maxTextLength+1)} {
		_, err := service.Ask(context.Background(), AskRequest{Scope: currentScope(), Message: message})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("message should be bounded and control-safe: %v", err)
		}
	}
	for _, conversationID := range []string{"conversation\x00id", strings.Repeat("x", maxIdentifierLength+1)} {
		_, err := service.Ask(context.Background(), AskRequest{Scope: currentScope(), ConversationID: conversationID, Message: "question"})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("conversation ID should be bounded and control-safe: %v", err)
		}
	}
}

func TestServiceRejectsInvalidAskBeforeCallingDependencies(t *testing.T) {
	retriever := &fakeRetriever{}
	generator := &fakeGenerator{}
	service, err := NewService(retriever, generator)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Ask(context.Background(), AskRequest{Scope: Scope{UserID: "user-1"}, Message: "question"})
	if !errors.Is(err, ErrInvalidInput) || retriever.called || generator.called {
		t.Fatalf("invalid workspace should stop before dependencies: err=%v retriever=%v generator=%v", err, retriever.called, generator.called)
	}
	_, err = service.Ask(context.Background(), AskRequest{Scope: currentScope(), Message: " "})
	if !errors.Is(err, ErrInvalidInput) || retriever.called || generator.called {
		t.Fatalf("blank message should stop before dependencies: err=%v retriever=%v generator=%v", err, retriever.called, generator.called)
	}
}

func TestNewServiceRequiresBothDependencies(t *testing.T) {
	if _, err := NewService(nil, &fakeGenerator{}); !errors.Is(err, ErrNilRetriever) {
		t.Fatalf("expected nil retriever error, got %v", err)
	}
	if _, err := NewService(&fakeRetriever{}, nil); !errors.Is(err, ErrNilGenerator) {
		t.Fatalf("expected nil generator error, got %v", err)
	}
}

func TestServiceRejectsNilReceiverAndContext(t *testing.T) {
	var service *Service
	if _, err := service.Ask(context.Background(), AskRequest{Scope: currentScope(), Message: "question"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil service should be rejected, got %v", err)
	}

	service, err := NewService(&fakeRetriever{}, &fakeGenerator{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Ask(nil, AskRequest{Scope: currentScope(), Message: "question"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil context should be rejected, got %v", err)
	}
}

func TestServiceCopiesHistoryAndNormalizesRequestIdentifiers(t *testing.T) {
	retriever := &fakeRetriever{items: currentEvidence()}
	generator := &fakeGenerator{draft: Draft{
		Answer:    "The API uses PostgreSQL.",
		Citations: []Citation{{EvidenceID: "evidence-1", Quote: "uses PostgreSQL"}},
	}}
	service, err := NewService(retriever, generator)
	if err != nil {
		t.Fatal(err)
	}
	history := []ConversationTurn{{Role: "user", Content: "Earlier question"}}
	_, err = service.Ask(context.Background(), AskRequest{
		Scope:          currentScope(),
		ConversationID: " conversation-1 ",
		Message:        " question ",
		History:        history,
	})
	if err != nil {
		t.Fatal(err)
	}
	if retriever.request.ConversationID != "conversation-1" || retriever.request.Query != "question" {
		t.Fatalf("retrieval request was not normalized: %+v", retriever.request)
	}
	if generator.request.ConversationID != "conversation-1" || generator.request.Message != "question" {
		t.Fatalf("generation request was not normalized: %+v", generator.request)
	}
	if !reflect.DeepEqual(generator.request.History, history) {
		t.Fatalf("history was not propagated: got=%+v want=%+v", generator.request.History, history)
	}
	generator.request.History[0].Content = "mutated copy"
	if history[0].Content != "Earlier question" {
		t.Fatal("service must copy non-empty history before passing it to the generator")
	}
}

func TestNewActionReceiptAttachmentValidatesCanonicalIdentity(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ActionBinding, *ActionReceipt)
		want   error
	}{
		{name: "missing binding workspace", mutate: func(binding *ActionBinding, _ *ActionReceipt) { binding.Scope.WorkspaceID = "" }, want: ErrInvalidInput},
		{name: "missing receipt scope", mutate: func(_ *ActionBinding, receipt *ActionReceipt) { receipt.Scope = Scope{} }, want: ErrInvalidReceipt},
		{name: "binding receipt mismatch", mutate: func(binding *ActionBinding, _ *ActionReceipt) { binding.Operation = "github.issue.comment" }, want: ErrReceiptBindingMismatch},
		{name: "invalid status", mutate: func(_ *ActionBinding, receipt *ActionReceipt) { receipt.Status = "completed" }, want: ErrInvalidReceipt},
		{name: "replayed pending", mutate: func(_ *ActionBinding, receipt *ActionReceipt) {
			receipt.Status = ReceiptPending
			receipt.Replayed = true
		}, want: ErrInvalidReceipt},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binding, receipt := currentBinding(), currentReceipt()
			test.mutate(&binding, &receipt)
			if _, err := NewActionReceiptAttachment(binding, receipt); !errors.Is(err, test.want) {
				t.Fatalf("constructor error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestNormalizeResponseValidatesActionAndEvidenceBoundaries(t *testing.T) {
	validDraft := func(actions []SuggestedAction) Draft {
		return Draft{
			Answer:           "The API uses PostgreSQL.",
			Citations:        []Citation{{EvidenceID: "evidence-1", Quote: "uses PostgreSQL"}},
			SuggestedActions: actions,
		}
	}
	for _, test := range []struct {
		name    string
		actions []SuggestedAction
	}{
		{name: "missing action id", actions: []SuggestedAction{{Kind: "work.task.create", Label: "Create task"}}},
		{name: "duplicate action id", actions: []SuggestedAction{{ID: "action-1", Kind: "work.task.create", Label: "Create one"}, {ID: "action-1", Kind: "work.task.create", Label: "Create two"}}},
		{name: "missing action label", actions: []SuggestedAction{{ID: "action-1", Kind: "work.task.create"}}},
		{name: "unbounded target", actions: []SuggestedAction{{ID: "action-1", Kind: "work.task.create", Label: "Create task", Target: strings.Repeat("x", maxTextLength+1)}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NormalizeResponse(currentScope(), validDraft(test.actions), currentEvidence()); !errors.Is(err, ErrInvalidAction) {
				t.Fatalf("action validation error = %v, want %v", err, ErrInvalidAction)
			}
		})
	}

	invalidEvidence := currentEvidence()
	invalidEvidence[0].SourceID = "source\x00id"
	if _, err := NormalizeResponse(currentScope(), validDraft(nil), invalidEvidence); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid source identifier should be rejected, got %v", err)
	}
	invalidEvidence = currentEvidence()
	invalidEvidence[0].Scope.UserID = ""
	if _, err := NormalizeResponse(currentScope(), validDraft(nil), invalidEvidence); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("incomplete evidence scope should be rejected, got %v", err)
	}
}

func TestNormalizeResponseUsesEvidenceIDWhenStaleSourceIsMissing(t *testing.T) {
	evidence := currentEvidence()
	evidence[0].SourceID = ""
	evidence[0].Freshness = FreshUnknown
	response, err := NormalizeResponse(currentScope(), Draft{
		Answer:    "The API uses PostgreSQL.",
		Citations: []Citation{{EvidenceID: "evidence-1", Quote: "uses PostgreSQL"}},
	}, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if response.Grounding != Inferred || !reflect.DeepEqual(response.StaleSources, []string{"evidence-1"}) {
		t.Fatalf("stale evidence fallback key was not exposed: %+v", response)
	}
}

func TestValidationHelpersRemainBoundedAndControlSafe(t *testing.T) {
	for _, value := range []string{"", " leading", "trailing ", strings.Repeat("x", maxIdentifierLength+1), "bad\x00id"} {
		if err := validateIdentifier("id", value); err == nil {
			t.Fatalf("identifier %q should be rejected", value)
		}
	}
	if err := validateIdentifier("id", "stable-id"); err != nil {
		t.Fatalf("valid identifier was rejected: %v", err)
	}
	if hasDisallowedControl("line\nwith\ttabs", true) || !hasDisallowedControl("line\x00", true) {
		t.Fatal("text-control policy is incorrect")
	}
	if got := appendUnique([]string{"one"}, "one"); !reflect.DeepEqual(got, []string{"one"}) {
		t.Fatalf("appendUnique duplicated an existing value: %+v", got)
	}
}

func TestValidationErrorFormatsNilAndEmptyFields(t *testing.T) {
	var nilError *ValidationError
	if nilError.Error() != ErrInvalidInput.Error() {
		t.Fatalf("nil validation error = %q", nilError.Error())
	}
	if got := (&ValidationError{Reason: "bad value"}).Error(); got != "assistant input is invalid: bad value" {
		t.Fatalf("empty-field validation error = %q", got)
	}
	if !errors.Is(&ValidationError{Field: "message", Reason: "bad"}, ErrInvalidInput) {
		t.Fatal("validation errors must unwrap to ErrInvalidInput")
	}
}
