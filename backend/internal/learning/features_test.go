package learning

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func TestDiagnosticUpdatesLearningState(t *testing.T) {
	memory := store.NewSeeded(time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC))
	service := NewService(memory, ai.FallbackProvider{Fallback: ai.DeterministicProvider{}, AllowFallback: true})
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
	service := NewService(memory, ai.FallbackProvider{Fallback: ai.DeterministicProvider{}, AllowFallback: true})
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

func TestRoleplayGuidanceOnlyDoesNotAwardTechnicalScore(t *testing.T) {
	memory := store.NewSeeded(time.Now().UTC())
	service := NewService(memory, ai.FallbackProvider{Fallback: ai.DeterministicProvider{}, AllowFallback: true})
	conversation, err := service.StartRoleplay(context.Background(), "technical-interview-api")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RoleplayTurn(context.Background(), domain.RoleplayTurnRequest{ConversationID: conversation.ID, Answer: "Em cần bạn gợi ý cách nói"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Feedback.Score != 0 || len(result.Feedback.WhatWasGood) != 0 || len(result.Feedback.TechnicalPoints) != 0 {
		t.Fatalf("guidance-only turn received fake technical credit: %+v", result.Feedback)
	}
	if !strings.Contains(result.Feedback.NextAction, "Reuse the starter sentence") || !strings.Contains(result.Feedback.NextAction, "one detail") {
		t.Fatalf("unexpected guidance next action: %q", result.Feedback.NextAction)
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

func TestSubmitWritingPersistsTheLearningOutcomeAsOneTransition(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	memory := store.NewSeeded(now)
	service := NewService(memory, ai.DeterministicProvider{})
	service.Now = func() time.Time { return now }
	answer := "Observed behavior: the API returns HTTP 500. Expected behavior: it should return HTTP 200. The root cause is an unchecked upload limit. The impact is that users cannot load the required data. Next step: reproduce the request locally and fix the validation."
	result, err := service.SubmitWriting(context.Background(), "mission-today", answer)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := memory.Mission(context.Background(), "mission-today")
	if err != nil || mission.Status != "completed" || result.Attempt.ID == "" {
		t.Fatalf("writing outcome was not persisted atomically: mission=%+v result=%+v err=%v", mission, result, err)
	}
	state, err := memory.LearningState(context.Background())
	if err != nil || state.Skills[1].Score <= 48 {
		t.Fatalf("learning state did not advance with the writing outcome: %+v err=%v", state, err)
	}
}

func TestV2GraphAnalyticsAndWeeklySpeaking(t *testing.T) {
	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	memory := store.NewSeeded(now)
	service := NewService(memory, ai.FallbackProvider{Fallback: ai.DeterministicProvider{}, AllowFallback: true})
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

func TestProviderUsageRecordDoesNotEstimateUnavailableUsage(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	record := providerUsageRecord("groq", "whisper-large-v3", "stt", ai.JSONUsage{
		InputTokens:  100,
		OutputTokens: 20,
		TotalTokens:  120,
		Available:    false,
	}, now)
	if record.UsageAvailable || record.InputTokens != 0 || record.OutputTokens != 0 || record.EstimatedCost != 0 {
		t.Fatalf("unavailable usage was estimated: %+v", record)
	}

	available := providerUsageRecord("deepseek", "deepseek-v4-flash", "copilot", ai.JSONUsage{
		InputTokens:  100,
		OutputTokens: 20,
		TotalTokens:  120,
		Available:    true,
		Model:        "deepseek-v4-flash",
	}, now)
	if !available.UsageAvailable || available.InputTokens != 100 || available.OutputTokens != 20 || available.EstimatedCost <= 0 {
		t.Fatalf("provider usage was not preserved: %+v", available)
	}
}

func TestUsageSummaryReportsUnavailableProviderUsage(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	memory := store.NewSeeded(now)
	service := NewService(memory, ai.DeterministicProvider{})
	service.Now = func() time.Time { return now }

	if err := memory.SaveUsage(context.Background(), domain.UsageRecord{
		Provider: "deepseek", Model: "deepseek-v4-flash", Feature: "assistant",
		UsageAvailable: true, InputTokens: 100, OutputTokens: 20, EstimatedCost: 12,
		CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := memory.SaveUsage(context.Background(), domain.UsageRecord{
		Provider: "azure", Model: "azure-neural-tts", Feature: "tts",
		UsageAvailable: false, TTSCharacters: 42, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	summary, err := service.UsageSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.EstimatedCost != 12 || summary.UnavailableRecords != 1 || len(summary.Records) != 2 {
		t.Fatalf("usage summary = %+v, want cost=12 unavailable=1 records=2", summary)
	}
}

type testSTTProvider struct{}

func (testSTTProvider) Name() string     { return "groq-test" }
func (testSTTProvider) Configured() bool { return true }
func (testSTTProvider) Transcribe(context.Context, []byte, string) (ai.Transcript, error) {
	return ai.Transcript{Text: "The deployment is stable."}, nil
}

type testTTSProvider struct{}

func (testTTSProvider) Name() string     { return "azure-tts-test" }
func (testTTSProvider) Configured() bool { return true }
func (testTTSProvider) Synthesize(context.Context, string, string) ([]byte, error) {
	return []byte("audio"), nil
}

type testPronunciationProvider struct{}

func (testPronunciationProvider) Name() string     { return "azure-pronunciation-test" }
func (testPronunciationProvider) Configured() bool { return true }
func (testPronunciationProvider) Assess(context.Context, []byte, string, string) (ai.PronunciationResult, error) {
	return ai.PronunciationResult{Accuracy: 80, Fluency: 75, Completeness: 90, Prosody: 70}, nil
}

func TestSpeechUsageRecordsStayUnavailableWithoutProviderUsage(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	memory := store.NewSeeded(now)
	service := NewService(memory, ai.DeterministicProvider{}, SpeechDependencies{
		STT:           testSTTProvider{},
		TTS:           testTTSProvider{},
		Pronunciation: testPronunciationProvider{},
	})
	service.Now = func() time.Time { return now }
	ctx := context.Background()

	session, err := service.TranscribeAudio(ctx, "mission-today", "audio/webm", []byte("audio"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AssessSpeaking(ctx, session.ID, "audio/webm", session.Transcript, []byte("audio")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Synthesize(ctx, "Hello", "en-US-Test"); err != nil {
		t.Fatal(err)
	}

	records, err := memory.Usage(ctx, now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	byFeature := make(map[string]domain.UsageRecord, len(records))
	for _, record := range records {
		byFeature[record.Feature] = record
	}
	for _, feature := range []string{"stt", "pronunciation", "tts"} {
		record, ok := byFeature[feature]
		if !ok {
			t.Fatalf("missing %s usage record: %+v", feature, records)
		}
		if record.UsageAvailable || record.InputTokens != 0 || record.OutputTokens != 0 || record.EstimatedCost != 0 {
			t.Fatalf("%s recorded guessed provider usage: %+v", feature, record)
		}
	}
	if byFeature["tts"].TTSCharacters != len([]rune("Hello")) {
		t.Fatalf("tts request metadata was lost: %+v", byFeature["tts"])
	}
}
