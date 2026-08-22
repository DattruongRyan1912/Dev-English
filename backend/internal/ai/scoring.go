package ai

import (
	"math"
	"strings"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
)

// FinalWritingScore is intentionally deterministic. The model may provide
// observations and corrections, but mastery and progress never depend on an
// unbounded model-generated number.
func FinalWritingScore(mission domain.Mission, answer string, observation domain.Evaluation) float64 {
	lower := strings.ToLower(strings.TrimSpace(answer))
	words := len(strings.Fields(answer))
	grammar := 100.0 - float64(len(observation.Corrections))*14
	if grammar < 0 {
		grammar = 0
	}
	clarity := 35.0
	if words >= 12 {
		clarity += 25
	}
	if words >= 30 {
		clarity += 20
	}
	if strings.Contains(lower, ".") || strings.Contains(lower, ";") {
		clarity += 10
	}
	naturalness := 70.0
	if len(observation.Corrections) == 0 {
		naturalness += 20
	}
	technical := 25.0
	for _, point := range mission.ExpectedPoints {
		if containsConcept(lower, point) {
			technical += 75 / float64(maxInt(1, len(mission.ExpectedPoints)))
		}
	}
	vocabulary := 40.0
	for _, term := range mission.TargetVocabulary {
		if strings.Contains(lower, strings.ToLower(term)) {
			vocabulary += 60 / float64(maxInt(1, len(mission.TargetVocabulary)))
		}
	}
	score := grammar*0.20 + clarity*0.25 + naturalness*0.20 + technical*0.25 + vocabulary*0.10
	return math.Round(clampScore(score)*10) / 10
}

func FinalSpeakingScore(result PronunciationResult, transcript string) float64 {
	words := len(strings.Fields(transcript))
	communication := 40.0
	if words >= 12 {
		communication += 30
	}
	if words >= 30 {
		communication += 20
	}
	score := result.Fluency*0.20 + result.Accuracy*0.20 + result.Completeness*0.15 + result.Prosody*0.15 + communication*0.30
	return math.Round(clampScore(score)*10) / 10
}

func clampScore(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
