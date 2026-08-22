package learning

import (
	"context"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func TestDiagnosticUpdatesLearningState(t *testing.T) {
	memory := store.NewSeeded(time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC))
	service := NewService(memory, ai.FallbackProvider{Fallback: ai.DeterministicProvider{}})
	questions := service.DiagnosticQuestions()
	responses := make([]domain.DiagnosticResponse, 0, len(questions))
	for _, question := range questions {
		responses = append(responses, domain.DiagnosticResponse{QuestionID: question.ID, Answer: question.Options[len(question.Options)-1]})
	}
	result, err := service.RunDiagnostic(context.Background(), responses)
	if err != nil {
		t.Fatal(err)
	}
	if result.CEFR != "C1" || result.OverallScore < 80 {
		t.Fatalf("unexpected diagnostic result: %+v", result)
	}
	state, err := memory.LearningState(context.Background())
	if err != nil || state.CEFR != result.CEFR {
		t.Fatalf("learning state was not persisted: %+v, err=%v", state, err)
	}
}

func TestRoleplayPersistsConversationAndFeedback(t *testing.T) {
	memory := store.NewSeeded(time.Now().UTC())
	service := NewService(memory, ai.FallbackProvider{Fallback: ai.DeterministicProvider{}})
	conversation, err := service.StartRoleplay(context.Background(), "pr-review")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RoleplayTurn(context.Background(), domain.RoleplayTurnRequest{ConversationID: conversation.ID, Answer: "I chose a cache because it reduces latency, and I will measure the trade-off."})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Conversation.Messages) != 3 || result.Reply.Role != "assistant" {
		t.Fatalf("unexpected conversation: %+v", result.Conversation)
	}
}

func TestReviewMovesVocabularyToNextInterval(t *testing.T) {
	memory := store.NewSeeded(time.Now().UTC())
	service := NewService(memory, ai.DeterministicProvider{})
	result, err := service.SubmitReview(context.Background(), "vocabulary", "vocab-1", true, 90)
	if err != nil {
		t.Fatal(err)
	}
	item, ok := result.(domain.Vocabulary)
	if !ok || !item.NextReview.After(time.Now()) {
		t.Fatalf("unexpected review result: %#v", result)
	}
}

func TestV2GraphAnalyticsAndWeeklySpeaking(t *testing.T) {
	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	memory := store.NewSeeded(now)
	service := NewService(memory, ai.FallbackProvider{Fallback: ai.DeterministicProvider{}})
	service.Now = func() time.Time { return now }

	graph, err := service.VocabularyGraph(context.Background())
	if err != nil || len(graph.Nodes) != 2 || len(graph.Edges) == 0 {
		t.Fatalf("unexpected vocabulary graph: %+v, err=%v", graph, err)
	}
	analytics, err := service.Analytics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if analytics.WindowDays != 7 || analytics.MissionsCreated == 0 || analytics.RepeatedMistakes != 2 {
		t.Fatalf("unexpected analytics: %+v", analytics)
	}
	weekly, err := service.WeeklySpeaking(context.Background())
	if err != nil || weekly.Sessions != 0 || weekly.Evaluated != 0 || weekly.Recommendation == "" {
		t.Fatalf("unexpected weekly speaking assessment: %+v, err=%v", weekly, err)
	}
	checks := service.TestConnections(context.Background())
	if len(checks) != 4 || checks[0].Status != "fallback" {
		t.Fatalf("unexpected provider checks: %+v", checks)
	}
}

func TestWorkImportFeedsNextMissionAndVocabularyRelations(t *testing.T) {
	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	memory := store.NewSeeded(now)
	service := NewService(memory, ai.DeterministicProvider{})
	service.Now = func() time.Time { return now }
	content := "The API latency is high because the dependency times out. We need a safe rollback and a clear next step."
	if _, err := service.WorkImport(context.Background(), "incident", "Provider timeout", content); err != nil {
		t.Fatal(err)
	}
	mission, err := service.CreateDailyMission(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if mission.Context == "" || mission.Context == "an API error from your current project" {
		t.Fatalf("recent work context was not used: %+v", mission)
	}
	items, err := memory.AllVocabulary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	foundRelated := false
	for _, item := range items {
		if item.Term == "latency" && len(item.RelatedTerms) > 0 {
			foundRelated = true
		}
	}
	if !foundRelated {
		t.Fatalf("imported vocabulary did not retain graph relations: %+v", items)
	}
}
