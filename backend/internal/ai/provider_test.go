package ai

import (
	"context"
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
	provider := FallbackProvider{Primary: &DeepSeekProvider{}, Fallback: DeterministicProvider{}}
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
