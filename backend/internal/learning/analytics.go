package learning

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
)

func (s *Service) TestConnections(ctx context.Context) []domain.ProviderCheck {
	checks := make([]domain.ProviderCheck, 0, 4)
	if s.DeepSeekSecrets != nil {
		_, check, err := s.DeepSeekSecrets.Test(ctx)
		if err != nil {
			check = domain.ProviderCheck{Provider: "DeepSeek", Status: "unhealthy"}
		}
		checks = append(checks, check)
	} else if s.AI != nil && s.AI.Name() == "deterministic-fallback" {
		checks = append(checks, domain.ProviderCheck{Provider: "DeepSeek", Status: "fallback"})
	} else {
		checks = append(checks, checkProvider(ctx, "DeepSeek", "text_generation", s.AI))
	}
	checks = append(checks,
		checkProvider(ctx, "Groq Whisper", "speech_to_text", s.STT),
		checkProvider(ctx, "Azure Pronunciation", "pronunciation_assessment", s.Pronunciation),
		checkProvider(ctx, "Azure Neural TTS", "text_to_speech", s.TTS),
	)
	return checks
}

func configured(provider interface{ Configured() bool }) bool {
	return provider != nil && provider.Configured()
}

type namedProvider interface {
	Name() string
	Configured() bool
}

func checkProvider(ctx context.Context, name, capability string, provider namedProvider) domain.ProviderCheck {
	if probe, ok := provider.(ai.CapabilityProbe); ok {
		check := probe.ProbeCapability(ctx, capability)
		check.Provider = name
		return check
	}
	check := domain.ProviderCheck{Provider: name}
	if provider == nil || !provider.Configured() {
		check.Status = "not_configured"
		check.Capability = capability
		return check
	}
	check.Configured = true
	check.Capability = capability
	checker, ok := provider.(ai.HealthChecker)
	if !ok {
		check.Status = "unhealthy"
		return check
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := checker.HealthCheck(probeCtx); err != nil {
		check.Status = "unhealthy"
		return check
	}
	check.Status = "healthy"
	return check
}

func (s *Service) VocabularyGraph(ctx context.Context) (domain.VocabularyGraph, error) {
	items, err := s.Store.AllVocabulary(ctx)
	if err != nil {
		return domain.VocabularyGraph{}, err
	}
	graph := domain.VocabularyGraph{Nodes: make([]domain.VocabularyGraphNode, 0, len(items)), Edges: make([]domain.VocabularyGraphEdge, 0)}
	byTerm := make(map[string]string, len(items))
	for _, item := range items {
		graph.Nodes = append(graph.Nodes, domain.VocabularyGraphNode{ID: item.ID, Label: item.Term, Mastery: item.Mastery})
		byTerm[normalizeTerm(item.Term)] = item.ID
	}
	seen := make(map[string]struct{})
	for _, item := range items {
		from := item.ID
		for _, related := range item.RelatedTerms {
			to, ok := byTerm[normalizeTerm(related)]
			if !ok || to == from {
				continue
			}
			key := from + "\x00" + to
			reverse := to + "\x00" + from
			if _, exists := seen[key]; exists {
				continue
			}
			if _, exists := seen[reverse]; exists {
				continue
			}
			seen[key] = struct{}{}
			graph.Edges = append(graph.Edges, domain.VocabularyGraphEdge{From: from, To: to, Relation: "related"})
		}
	}
	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].Label < graph.Nodes[j].Label })
	sort.Slice(graph.Edges, func(i, j int) bool {
		if graph.Edges[i].From == graph.Edges[j].From {
			return graph.Edges[i].To < graph.Edges[j].To
		}
		return graph.Edges[i].From < graph.Edges[j].From
	})
	return graph, nil
}

func (s *Service) WeeklySpeaking(ctx context.Context) (domain.WeeklySpeakingAssessment, error) {
	now := s.Now()
	cutoff := now.Add(-7 * 24 * time.Hour)
	sessions, err := s.Store.AllSpeakingSessions(ctx)
	if err != nil {
		return domain.WeeklySpeakingAssessment{}, err
	}
	assessment := domain.WeeklySpeakingAssessment{Week: fmt.Sprintf("%s to %s", cutoff.Format("2006-01-02"), now.Format("2006-01-02"))}
	for _, session := range sessions {
		if session.CreatedAt.Before(cutoff) {
			continue
		}
		assessment.Sessions++
		if session.Pronunciation == nil {
			continue
		}
		assessment.Evaluated++
		assessment.AverageScore += session.Pronunciation.Score
		assessment.AverageFluency += session.Pronunciation.Fluency
		assessment.AverageProsody += session.Pronunciation.Prosody
	}
	if assessment.Evaluated == 0 {
		assessment.Recommendation = "Complete one speaking assessment this week to unlock pronunciation and prosody feedback."
		return assessment, nil
	}
	count := float64(assessment.Evaluated)
	assessment.AverageScore /= count
	assessment.AverageFluency /= count
	assessment.AverageProsody /= count
	switch {
	case assessment.AverageProsody < 60:
		assessment.Recommendation = "Slow down at key points and stress the words that carry the technical decision."
	case assessment.AverageFluency < 60:
		assessment.Recommendation = "Use a short context-evidence-impact structure to reduce pauses while explaining the issue."
	case assessment.AverageScore < 70:
		assessment.Recommendation = "Repeat one familiar technical explanation and apply the previous pronunciation feedback."
	default:
		assessment.Recommendation = "Keep one assessed speaking session in the weekly routine and increase technical specificity gradually."
	}
	return assessment, nil
}

func (s *Service) Analytics(ctx context.Context) (domain.AnalyticsSummary, error) {
	now := s.Now()
	cutoff := now.Add(-7 * 24 * time.Hour)
	missions, err := s.Store.AllMissions(ctx)
	if err != nil {
		return domain.AnalyticsSummary{}, err
	}
	mistakes, err := s.Store.AllMistakes(ctx)
	if err != nil {
		return domain.AnalyticsSummary{}, err
	}
	vocabulary, err := s.Store.AllVocabulary(ctx)
	if err != nil {
		return domain.AnalyticsSummary{}, err
	}
	sessions, err := s.Store.AllSpeakingSessions(ctx)
	if err != nil {
		return domain.AnalyticsSummary{}, err
	}
	progress, err := s.Store.Progress(ctx)
	if err != nil {
		return domain.AnalyticsSummary{}, err
	}
	state, err := s.Store.LearningState(ctx)
	if err != nil {
		return domain.AnalyticsSummary{}, err
	}
	result := domain.AnalyticsSummary{WindowDays: 7, SpeakingTrend: progress}
	for _, mission := range missions {
		if !mission.CreatedAt.Before(cutoff) {
			result.MissionsCreated++
		}
		if mission.CompletedAt != nil && !mission.CompletedAt.Before(cutoff) {
			result.MissionsCompleted++
			result.MinutesLearned += mission.EstimatedMinutes
		}
	}
	for _, session := range sessions {
		if !session.CreatedAt.Before(cutoff) {
			result.MinutesLearned += 5
		}
	}
	for _, mistake := range mistakes {
		if mistake.Frequency >= 2 {
			result.RepeatedMistakes++
		}
	}
	for _, item := range vocabulary {
		result.VocabularyMastery += item.Mastery
	}
	if len(vocabulary) > 0 {
		result.VocabularyMastery /= float64(len(vocabulary))
	}
	result.TechnicalCommunicationScore = technicalCommunicationScore(state)
	weekly, err := s.WeeklySpeaking(ctx)
	if err != nil {
		return domain.AnalyticsSummary{}, err
	}
	result.WeeklySpeaking = weekly
	return result, nil
}

func technicalCommunicationScore(state domain.LearningState) float64 {
	values := make([]float64, 0, 2)
	for _, skill := range state.Skills {
		if skill.Skill == "technical_writing" || skill.Skill == "git_communication" {
			values = append(values, skill.Score)
		}
	}
	if len(values) == 0 {
		return skillScore(state, "technical_writing")
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func relatedTerms(term string, terms []string) []string {
	result := make([]string, 0, len(terms)-1)
	for _, candidate := range terms {
		if !strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(term)) {
			result = append(result, candidate)
		}
	}
	return result
}

func normalizeTerm(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
