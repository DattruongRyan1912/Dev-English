package ai

import (
	"errors"
	"fmt"
	"strings"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
)

func validateMission(mission domain.Mission) error {
	if strings.TrimSpace(mission.Title) == "" || strings.TrimSpace(mission.Prompt) == "" {
		return errors.New("title and prompt are required")
	}
	if mission.Mode != domain.ModeWriting && mission.Mode != domain.ModeSpeaking && mission.Mode != domain.ModeReading {
		return fmt.Errorf("unsupported mode %q", mission.Mode)
	}
	if strings.TrimSpace(mission.Skill) == "" || strings.TrimSpace(mission.Level) == "" {
		return errors.New("skill and level are required")
	}
	if mission.EstimatedMinutes < 1 || mission.EstimatedMinutes > 60 {
		return errors.New("estimated minutes must be between 1 and 60")
	}
	if len(mission.TargetVocabulary) > 12 || len(mission.ExpectedPoints) > 12 {
		return errors.New("mission arrays are too large")
	}
	return nil
}

func validateEvaluation(evaluation domain.Evaluation) error {
	if evaluation.Score < 0 || evaluation.Score > 100 {
		return errors.New("score must be between 0 and 100")
	}
	if strings.TrimSpace(evaluation.Summary) == "" || strings.TrimSpace(evaluation.MainIssue) == "" || strings.TrimSpace(evaluation.NextAction) == "" {
		return errors.New("summary, main issue and next action are required")
	}
	if len(evaluation.Corrections) > 8 {
		return errors.New("too many corrections")
	}
	for _, correction := range evaluation.Corrections {
		if strings.TrimSpace(correction.Original) == "" || strings.TrimSpace(correction.Corrected) == "" {
			return errors.New("correction original and corrected text are required")
		}
		if correction.Severity < 1 || correction.Severity > 3 {
			return errors.New("correction severity must be between 1 and 3")
		}
	}
	return nil
}
