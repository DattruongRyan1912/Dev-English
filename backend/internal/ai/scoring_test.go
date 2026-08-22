package ai

import (
	"testing"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
)

func TestFinalWritingScoreIsBackendOwned(t *testing.T) {
	mission := domain.Mission{ExpectedPoints: []string{"observed behavior", "expected behavior", "impact", "next step"}, TargetVocabulary: []string{"reproduce", "impact"}}
	observation := domain.Evaluation{Score: 1, Corrections: []domain.Correction{}}
	score := FinalWritingScore(mission, "The API returns an error. The expected behavior is a validation response. This impacts users, and the next step is to reproduce the issue.", observation)
	if score < 70 {
		t.Fatalf("complete answer should score at least 70, got %.1f", score)
	}
	if score == observation.Score {
		t.Fatal("final score must not trust the provider score")
	}
}
