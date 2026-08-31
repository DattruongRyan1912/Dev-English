package application

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

func TestApplicationBoundaryCoverageForInvalidLifecycleInputs(t *testing.T) {
	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.BootstrapWithModules(nil, false, BootstrapModules{}); err == nil {
		t.Fatal("nil bootstrap context unexpectedly succeeded")
	}
	if _, err := app.ImportManualSource(context.Background(), ManualSourceInput{}, "one", "two"); err == nil {
		t.Fatal("multiple idempotency keys unexpectedly succeeded")
	}
	if _, err := app.ImportManualSource(context.Background(), ManualSourceInput{}, " "); err == nil {
		t.Fatal("blank idempotency key unexpectedly succeeded")
	}
}

func TestApplicationReplayAndUnavailableGeneratorFailClosed(t *testing.T) {
	if _, err := replayManualImport(knowledge.ManualImportIdempotencyRecord{
		Operation: manualImportOperation, RequestHash: "request-hash", ResponseJSON: []byte("{"),
	}, "request-hash"); err == nil {
		t.Fatal("malformed manual import replay unexpectedly succeeded")
	}
	if _, err := (unavailableAssistantGenerator{}).Generate(context.Background(), assistant.GenerationRequest{}); err == nil {
		t.Fatal("unavailable assistant generator unexpectedly succeeded")
	}
}

func TestApplicationHelpersAndConversationBoundaries(t *testing.T) {
	if estimateTokens("") != 0 || estimateTokens("x") != 1 || estimateTokens("12345678") != 2 {
		t.Fatal("estimateTokens did not preserve empty, short and bounded estimates")
	}
	if got := appendUniqueString([]string{"already"}, "already"); len(got) != 1 {
		t.Fatalf("appendUniqueString duplicated an existing value: %#v", got)
	}
	t.Setenv("DEVENGLISH_COVERAGE_INT", "7")
	if envPositiveInt("DEVENGLISH_COVERAGE_INT", 1) != 7 {
		t.Fatal("envPositiveInt did not parse a valid value")
	}
	t.Setenv("DEVENGLISH_COVERAGE_FLOAT", "1.5")
	if envFloat("DEVENGLISH_COVERAGE_FLOAT", 1) != 1.5 {
		t.Fatal("envFloat did not parse a valid value")
	}
	if _, err := assistantPrompt(assistant.GenerationRequest{
		Message:  "question",
		History:  make([]assistant.ConversationTurn, 13),
		Evidence: []assistant.Evidence{{ID: "evidence", Snippet: strings.Repeat("x", 8_001)}},
	}); err != nil {
		t.Fatalf("assistantPrompt() error = %v", err)
	}

	repository := NewMemoryConversationRepository()
	if _, err := repository.Create(context.Background(), work.Scope{}, "invalid"); err == nil {
		t.Fatal("conversation create with invalid scope unexpectedly succeeded")
	}
	validScope := work.Scope{WorkspaceID: "conversation-coverage-workspace", UserID: "user-1"}
	if _, err := repository.Create(context.Background(), validScope, "multiple", assistant.ContextRef{}, assistant.ContextRef{}); err == nil {
		t.Fatal("conversation create with multiple contexts unexpectedly succeeded")
	}
	if _, err := repository.Get(context.Background(), validScope, "latest"); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("empty latest conversation = %v", err)
	}
	conversation, err := repository.Create(context.Background(), validScope, "valid")
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Append(context.Background(), validScope, "missing", "message", assistant.AssistantResponse{}); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("missing conversation append = %v", err)
	}
	if err := repository.Append(context.Background(), validScope, conversation.ID, "message", assistant.AssistantResponse{Answer: "answer"}); err != nil {
		t.Fatalf("conversation append = %v", err)
	}

	at := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	summaries := []ConversationSummary{
		{ID: "older", UpdatedAt: at},
		{ID: "newer", UpdatedAt: at.Add(time.Minute)},
		{ID: "same-a", UpdatedAt: at},
		{ID: "same-b", UpdatedAt: at},
	}
	sortConversationSummaries(summaries)
	if !sort.SliceIsSorted(summaries, func(i, j int) bool {
		if summaries[i].UpdatedAt.Equal(summaries[j].UpdatedAt) {
			return summaries[i].ID > summaries[j].ID
		}
		return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt)
	}) {
		t.Fatalf("conversation summaries are not stably ordered: %#v", summaries)
	}
	conversations := []Conversation{{ID: "new", UpdatedAt: at.Add(time.Minute)}, {ID: "old", UpdatedAt: at}}
	sortConversations(conversations)
	if conversations[0].ID != "old" {
		t.Fatalf("sortConversations() = %#v", conversations)
	}
	if _, err := encodeAssistantResponse(assistant.AssistantResponse{Answer: "answer"}); err != nil {
		t.Fatalf("encodeAssistantResponse() error = %v", err)
	}

	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.AskWithContext(context.Background(), "", " ", assistant.ContextRef{}); err == nil {
		t.Fatal("blank assistant message unexpectedly succeeded")
	}
	if _, err := app.Search(nil, assistant.RetrievalRequest{}); err == nil {
		t.Fatal("nil assistant search context unexpectedly succeeded")
	}
	userContext := store.WithUser(context.Background(), "user-1")
	invalidContextRequest := assistant.RetrievalRequest{
		Scope:   assistant.Scope{UserID: "user-1", WorkspaceID: WorkspaceIDForUser("user-1")},
		Context: assistant.ContextRef{Type: "unsupported", ID: "entity-1"},
	}
	if _, err := app.Search(userContext, invalidContextRequest); err == nil {
		t.Fatal("invalid assistant context unexpectedly succeeded")
	}

}
