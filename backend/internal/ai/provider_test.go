package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
)

func TestDeterministicEvaluatorReturnsStructuredFeedback(t *testing.T) {
	mission := domain.Mission{ExpectedPoints: []string{"observed behavior", "expected behavior", "impact", "next step"}}
	result := EvaluateWritingDeterministically(mission, "The API returns a 500 error. The expected behavior is a clear validation error. This impacts users, so the next step is to reproduce and fix the upload limit.")
	if result.Score < 70 {
		t.Fatalf("expected a complete technical answer to score at least 70, got %.2f", result.Score)
	}
	if result.Summary == "" || result.NextAction == "" {
		t.Fatal("expected summary and next action")
	}
}

func TestFallbackProviderUsesDeterministicModeWithoutKey(t *testing.T) {
	provider := FallbackProvider{Primary: &DeepSeekProvider{}, Fallback: DeterministicProvider{}, AllowFallback: true}
	if provider.Configured() {
		t.Fatal("empty DeepSeek provider must not be configured")
	}
	mission, err := provider.GenerateMission(context.Background(), MissionRequest{LearningState: domain.LearningState{CEFR: "B1", WeakestSkill: "technical_writing"}})
	if err != nil {
		t.Fatal(err)
	}
	if mission.ID == "" || provider.Name() != "deterministic-fallback" {
		t.Fatalf("unexpected fallback result: %+v", mission)
	}
}

func TestDeterministicRoleplayAdaptsToVietnameseHelp(t *testing.T) {
	provider := DeterministicProvider{}
	result, err := provider.GenerateRoleplay(context.Background(), RoleplayRequest{Scenario: domain.RoleplayScenario{Type: "code-review"}, Answer: "Em cần bạn gợi ý cách nói"})
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(result.Reply), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected Vietnamese guidance, one starter and one question, got %q", result.Reply)
	}
	if lines[0] != "Câu hỏi yêu cầu bạn nêu ý chính và thêm một chi tiết kỹ thuật." {
		t.Fatalf("unexpected general Vietnamese explanation: %q", lines[0])
	}
	if lines[1] != "I think we should start with the main issue." {
		t.Fatalf("unexpected starter sentence: %q", lines[1])
	}
	if lines[2] != "What happened first?" {
		t.Fatalf("unexpected follow-up question: %q", lines[2])
	}
	if !result.GuidanceOnly || result.Evaluation.Score != 0 || len(result.Evaluation.WhatWasGood) != 0 || len(result.Evaluation.TechnicalPoints) != 0 {
		t.Fatalf("adaptive guidance must not evaluate a technical answer: %+v", result.Evaluation)
	}
	if !strings.Contains(result.Evaluation.NextAction, "Reuse the starter sentence") || !strings.Contains(result.Evaluation.NextAction, "one detail") {
		t.Fatalf("next action must reuse the starter and add one detail: %q", result.Evaluation.NextAction)
	}
}

func TestDeterministicRoleplayAdaptsStarterToScenarioType(t *testing.T) {
	provider := DeterministicProvider{}
	tests := []struct {
		name        string
		scenario    domain.RoleplayScenario
		explanation string
		starter     string
		question    string
	}{
		{
			name:        "technical interview API",
			scenario:    domain.RoleplayScenario{Type: "technical-interview", Context: "Design an API for idempotent requests."},
			explanation: "Câu hỏi yêu cầu bạn nêu cách tiếp cận đầu tiên khi thiết kế API và làm rõ các yêu cầu.",
			starter:     "I would start by clarifying the requirements.",
			question:    "What should the solution do?",
		},
		{
			name:        "system design",
			scenario:    domain.RoleplayScenario{Type: "system-design", Context: "Design a queue for bursty traffic."},
			explanation: "Câu hỏi yêu cầu bạn xác định ràng buộc chính trước khi chọn thiết kế.",
			starter:     "I would start with the main system constraint.",
			question:    "What must the system handle?",
		},
		{
			name:        "general roleplay",
			scenario:    domain.RoleplayScenario{Type: "code-review", Context: "Explain a cache choice."},
			explanation: "Câu hỏi yêu cầu bạn nêu ý chính và thêm một chi tiết kỹ thuật.",
			starter:     "I think we should start with the main issue.",
			question:    "What happened first?",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := provider.GenerateRoleplay(context.Background(), RoleplayRequest{Scenario: test.scenario, Answer: "Can you help me answer?"})
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(result.Reply), "\n")
			if !result.GuidanceOnly || len(lines) != 3 || lines[0] != test.explanation || lines[1] != test.starter || lines[2] != test.question {
				t.Fatalf("unexpected scenario guidance: %+v lines=%q", result, lines)
			}
		})
	}
}

func TestDeterministicRoleplayKeepsEnglishResponseBehavior(t *testing.T) {
	provider := DeterministicProvider{}
	result, err := provider.GenerateRoleplay(context.Background(), RoleplayRequest{Answer: "This helper function returns an error because validation is missing."})
	if err != nil {
		t.Fatal(err)
	}
	if result.Reply != "What is the user impact, and do you have a safe workaround while we investigate?" {
		t.Fatalf("unexpected English roleplay regression: %q", result.Reply)
	}
	if result.GuidanceOnly || result.Evaluation.Score == 0 || result.Evaluation.Provider != "deterministic-fallback" {
		t.Fatalf("expected normal English evaluation, got %+v", result.Evaluation)
	}
}
