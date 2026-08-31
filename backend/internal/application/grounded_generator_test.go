package application

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func TestDeepSeekAssistantGeneratorRoutesModelsSanitizesEvidenceAndRecordsUsage(t *testing.T) {
	var models []string
	var authorization string
	var prompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("provider authorization was not sent to the provider")
		}
		authorization = r.Header.Get("Authorization")
		var request struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		models = append(models, request.Model)
		for _, message := range request.Messages {
			if message.Role == "user" {
				prompt = message.Content
			}
		}
		w.Header().Set("Content-Type", "application/json")
		content := `{"answer":"The API timeout is documented here.","evidence":[{"evidenceId":"knowledge-chunk-1","quote":"The API timeout is documented here.","locator":"https://attacker.invalid"}],"unknowns":[],"suggestedActions":[{"id":"action-1","kind":"github.issue.create","label":"Create issue"}]}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": content}}},
			"usage":   map[string]int{"prompt_tokens": 12, "completion_tokens": 8, "total_tokens": 20},
		})
	}))
	defer server.Close()

	usageStore := store.NewSeeded(time.Now().UTC())
	generator := NewDeepSeekAssistantGenerator(&ai.DeepSeekProvider{
		APIKey: "test-key", BaseURL: server.URL, FastModel: "fast-model", SmartModel: "smart-model", Client: server.Client(),
	}, usageStore, false)
	generator.Clock = func() time.Time { return time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC) }
	generator.InputCost = 1
	generator.OutputCost = 2

	request := assistant.GenerationRequest{
		Scope:   assistant.Scope{WorkspaceID: "workspace-1", UserID: "user-1"},
		Message: "What is the root cause and what should we decide?",
		Context: assistant.ContextRef{Type: assistant.ContextDecision, ID: "decision-1"},
		History: []assistant.ConversationTurn{{Role: "user", Content: "Earlier context"}},
		Evidence: []assistant.Evidence{{
			Scope: assistant.Scope{WorkspaceID: "workspace-1", UserID: "user-1"}, ID: "knowledge-chunk-1", SourceID: "source-1", RevisionID: "revision-1",
			Title: "Runbook", URI: "https://canonical.example/runbook", Snippet: "The API timeout is documented here.", Freshness: assistant.FreshCurrent,
		}},
	}
	draft, err := generator.Generate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0] != "smart-model" {
		t.Fatalf("smart request used models %v", models)
	}
	if authorization != "Bearer test-key" || !strings.Contains(prompt, "knowledge-chunk-1") || !strings.Contains(prompt, "Earlier context") || !strings.Contains(prompt, `"type":"decision"`) || !strings.Contains(prompt, `"id":"decision-1"`) {
		t.Fatalf("provider request lost auth or bounded context: auth=%q prompt=%q", authorization, prompt)
	}
	if len(draft.Citations) != 1 || draft.Citations[0].Locator != "https://canonical.example/runbook" || strings.Contains(draft.Citations[0].Locator, "attacker.invalid") || !strings.Contains(draft.Answer, "The API timeout is documented here.") {
		t.Fatalf("citation was not sanitized to canonical evidence: %+v", draft)
	}
	if len(draft.SuggestedActions) != 1 || !draft.SuggestedActions[0].RequiresConfirmation {
		t.Fatalf("suggested action was not confirmation-gated: %+v", draft.SuggestedActions)
	}
	records, err := usageStore.Usage(store.WithUser(context.Background(), "user-1"), monthStart(generator.now()))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].InputTokens != 12 || records[0].OutputTokens != 8 || !records[0].UsageAvailable || records[0].EstimatedCost <= 0 {
		t.Fatalf("provider usage was not persisted faithfully: %+v", records)
	}

	request.Message = "Explain the API timeout simply."
	if _, err := generator.Generate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[1] != "fast-model" {
		t.Fatalf("fast request used models %v", models)
	}
}

func TestDeepSeekAssistantGeneratorFailsClosedWhenStrictUsageIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		content := `{"answer":"The source says this.","evidence":[],"unknowns":[],"suggestedActions":[]}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": content}}},
		})
	}))
	defer server.Close()

	generator := NewDeepSeekAssistantGenerator(&ai.DeepSeekProvider{APIKey: "test-key", BaseURL: server.URL, FastModel: "fast", SmartModel: "smart", Client: server.Client()}, nil, false)
	generator.StrictUsage = true
	_, err := generator.Generate(context.Background(), assistant.GenerationRequest{
		Scope: assistant.Scope{WorkspaceID: "workspace-1", UserID: "user-1"}, Message: "hello",
		Evidence: []assistant.Evidence{{Scope: assistant.Scope{WorkspaceID: "workspace-1", UserID: "user-1"}, ID: "evidence-1", Snippet: "The source says this.", Freshness: assistant.FreshCurrent}},
	})
	if !errors.Is(err, ErrAssistantUsageUnavailable) {
		t.Fatalf("strict usage error = %v, want ErrAssistantUsageUnavailable", err)
	}
}

func TestDeepSeekAssistantGeneratorDoesNotEstimateUnavailableUsage(t *testing.T) {
	usageStore := store.NewSeeded(time.Now().UTC())
	generator := NewDeepSeekAssistantGenerator(nil, usageStore, false)
	generator.Clock = func() time.Time { return time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC) }

	if err := generator.recordUsage(context.Background(), "user-1", "deepseek-v4-flash", ai.JSONUsage{
		InputTokens:  123,
		OutputTokens: 45,
		TotalTokens:  168,
		Available:    false,
	}); err != nil {
		t.Fatal(err)
	}
	records, err := usageStore.Usage(store.WithUser(context.Background(), "user-1"), monthStart(generator.now()))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("usage record count = %d, want 1", len(records))
	}
	record := records[0]
	if record.UsageAvailable || record.InputTokens != 0 || record.OutputTokens != 0 || record.EstimatedCost != 0 {
		t.Fatalf("unavailable provider usage was estimated: %+v", record)
	}
}

func TestDeepSeekAssistantGeneratorHonorsMonthlyTokenCap(t *testing.T) {
	usageStore := store.NewSeeded(time.Now().UTC())
	if err := usageStore.SaveUsage(store.WithUser(context.Background(), "user-1"), domain.UsageRecord{Feature: "assistant", InputTokens: 9_000, OutputTokens: 2_000, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	generator := NewDeepSeekAssistantGenerator(nil, usageStore, false)
	generator.MonthlyCap = 10_000
	_, err := generator.Generate(context.Background(), assistant.GenerationRequest{
		Scope: assistant.Scope{WorkspaceID: "workspace-1", UserID: "user-1"}, Message: "hello",
	})
	if !errors.Is(err, ErrAssistantUsageLimit) {
		t.Fatalf("cap error = %v, want ErrAssistantUsageLimit", err)
	}
}

func TestImportManualSourceScopesIdenticalContentIDsPerSource(t *testing.T) {
	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	ctx := store.WithUser(context.Background(), "user-1")
	content := "The canonical rollback procedure keeps the database migration reversible."
	first, err := app.ImportManualSource(ctx, ManualSourceInput{Name: "Runbook A", Kind: "manual", URI: "memory://runbook-a", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.ImportManualSource(ctx, ManualSourceInput{Name: "Runbook B", Kind: "manual", URI: "memory://runbook-b", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	if first.Source.ID == second.Source.ID || first.RevisionID == second.RevisionID || first.ChunkID == second.ChunkID {
		t.Fatalf("identical content collided across sources: first=%+v second=%+v", first, second)
	}
	firstDetail, err := app.GetKnowledgeSourceDetail(ctx, first.Source.ID)
	if err != nil {
		t.Fatal(err)
	}
	secondDetail, err := app.GetKnowledgeSourceDetail(ctx, second.Source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstDetail.Revisions) != 1 || firstDetail.Revisions[0].ID != first.RevisionID || len(secondDetail.Revisions) != 1 || secondDetail.Revisions[0].ID != second.RevisionID {
		t.Fatalf("source revisions were not isolated: first=%+v second=%+v", firstDetail, secondDetail)
	}
}

func TestDeepSeekGeneratorFailsClosedOnUnverifiedDraftParts(t *testing.T) {
	evidence := []assistant.Evidence{{
		Scope: assistant.Scope{WorkspaceID: "workspace-1", UserID: "user-1"},
		ID:    "evidence-1", URI: "memory://canonical", Snippet: "The canonical fact is verified.", Freshness: assistant.FreshCurrent,
	}}
	draft, err := sanitizeAssistantDraft(providerAssistantDraft{
		Answer: "The answer does not contain the source quote.",
		Evidence: []assistant.Citation{
			{EvidenceID: "missing", Quote: "forged quote", Locator: "https://attacker.invalid"},
			{EvidenceID: "evidence-1", Quote: "The canonical fact is verified."},
			{EvidenceID: "evidence-1", Quote: "The canonical fact is verified."},
		},
		Unknowns: []string{"already unknown"},
		SuggestedActions: []assistant.SuggestedAction{
			{Kind: "github.issue.create", Label: "Create issue", Target: "issue-1"},
			{ID: "duplicate", Kind: "github.issue.comment", Label: "Comment", Target: "issue-1"},
			{ID: "duplicate", Kind: "github.issue.comment", Label: "Duplicate", Target: "issue-1"},
			{ID: "incomplete", Kind: "", Label: ""},
		},
	}, evidence)
	if err != nil {
		t.Fatalf("sanitizeAssistantDraft() error = %v", err)
	}
	if len(draft.Citations) != 1 || draft.Citations[0].EvidenceID != "evidence-1" || draft.Citations[0].Locator != evidence[0].URI {
		t.Fatalf("unverified or duplicate citations survived: %+v", draft.Citations)
	}
	if !strings.Contains(draft.Answer, "Source excerpt: The canonical fact is verified.") {
		t.Fatalf("verified quote was not added to answer: %q", draft.Answer)
	}
	if len(draft.SuggestedActions) != 2 || !draft.SuggestedActions[0].RequiresConfirmation || draft.SuggestedActions[0].ID != "suggested-action-1" {
		t.Fatalf("actions were not normalized and confirmation-gated: %+v", draft.SuggestedActions)
	}
	if len(draft.Unknowns) != 3 {
		t.Fatalf("invalid citation/action findings = %#v, want one each plus original", draft.Unknowns)
	}

	fromCitations, err := sanitizeAssistantDraft(providerAssistantDraft{
		Answer:    "The canonical fact is verified.",
		Citations: []assistant.Citation{{EvidenceID: "evidence-1", Quote: "The canonical fact is verified."}},
	}, evidence)
	if err != nil || len(fromCitations.Citations) != 1 {
		t.Fatalf("citations compatibility field was not accepted: draft=%+v err=%v", fromCitations, err)
	}
	if _, err := sanitizeAssistantDraft(providerAssistantDraft{}, evidence); err == nil {
		t.Fatal("empty provider answer unexpectedly succeeded")
	}
}

func TestDeepSeekGeneratorProviderAndFallbackBoundaries(t *testing.T) {
	request := assistant.GenerationRequest{
		Scope: assistant.Scope{WorkspaceID: "workspace-1", UserID: "user-1"}, Message: "hello",
		Evidence: []assistant.Evidence{{Scope: assistant.Scope{WorkspaceID: "workspace-1", UserID: "user-1"}, ID: "evidence-1", Snippet: "canonical fact", URI: "memory://fact", Freshness: assistant.FreshCurrent}},
	}
	var nilGenerator *DeepSeekAssistantGenerator
	if _, err := nilGenerator.Generate(context.Background(), request); !errors.Is(err, ErrAssistantProviderMissing) {
		t.Fatalf("nil generator error = %v, want ErrAssistantProviderMissing", err)
	}

	provider := &ai.DeepSeekProvider{APIKey: "test-key"}
	withFallback := NewDeepSeekAssistantGenerator(provider, nil, true)
	draft, err := withFallback.Generate(context.Background(), request)
	if err != nil || !strings.Contains(draft.Answer, "canonical fact") {
		t.Fatalf("configured provider without a model should use deterministic fallback: draft=%+v err=%v", draft, err)
	}
	withoutFallback := NewDeepSeekAssistantGenerator(provider, nil, false)
	if _, err := withoutFallback.Generate(context.Background(), request); !errors.Is(err, ErrAssistantProviderMissing) {
		t.Fatalf("missing model without fallback error = %v, want ErrAssistantProviderMissing", err)
	}

	fallback := &DeepSeekAssistantGenerator{AllowFallback: true, Fallback: deterministicGenerator{}}
	if _, err := fallback.fallback(context.Background(), request, ErrAssistantUsageLimit); !errors.Is(err, ErrAssistantUsageLimit) {
		t.Fatalf("usage limit fallback error = %v, want ErrAssistantUsageLimit", err)
	}
	if _, err := fallback.fallback(context.Background(), request, ErrAssistantUsageUnavailable); !errors.Is(err, ErrAssistantUsageUnavailable) {
		t.Fatalf("usage unavailable fallback error = %v, want ErrAssistantUsageUnavailable", err)
	}
	noFallback := &DeepSeekAssistantGenerator{}
	if _, err := noFallback.fallback(context.Background(), request, nil); !errors.Is(err, ErrAssistantProviderMissing) {
		t.Fatalf("nil fallback cause error = %v, want ErrAssistantProviderMissing", err)
	}
}

func TestDeepSeekGeneratorUsageBoundaries(t *testing.T) {
	ctx := context.Background()
	noStore := &DeepSeekAssistantGenerator{}
	if err := noStore.recordUsage(ctx, "user-1", "model", ai.JSONUsage{Available: true, TotalTokens: 1}); err != nil {
		t.Fatalf("nil usage store error = %v, want nil", err)
	}

	usageStore := store.NewSeeded(time.Now().UTC())
	generator := &DeepSeekAssistantGenerator{
		UsageStore: usageStore,
		MonthlyCap: 10_000,
		Clock:      nil,
	}
	if err := generator.recordUsage(ctx, "user-1", "model", ai.JSONUsage{Available: true}); !errors.Is(err, ErrAssistantUsageUnavailable) {
		t.Fatalf("zero available usage error = %v, want ErrAssistantUsageUnavailable", err)
	}
	if reservation, err := generator.reserveBudget(ctx, "user-1", "short prompt"); err != nil || reservation == nil {
		t.Fatalf("usage reservation = %v, error = %v; want reservation", reservation, err)
	}

	noCap := &DeepSeekAssistantGenerator{UsageStore: usageStore, MonthlyCap: 0}
	if reservation, err := noCap.reserveBudget(ctx, "user-1", "short prompt"); err != nil || reservation != nil {
		t.Fatalf("disabled cap reservation = %v, error = %v; want no reservation", reservation, err)
	}
}
