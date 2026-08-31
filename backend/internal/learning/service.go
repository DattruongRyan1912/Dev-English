package learning

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/usageguard"
)

type Service struct {
	Store           store.Repository
	AI              ai.Provider
	DeepSeekSecrets *DeepSeekSecretManager
	STT             ai.STTProvider
	TTS             ai.TTSProvider
	Pronunciation   ai.PronunciationProvider
	GitHub          GitHubImporter
	Now             func() time.Time
	BudgetGuard     *usageguard.Guard
}

type GitHubImporter interface {
	Import(context.Context, string) (domain.GitHubImport, error)
}

type SpeechDependencies struct {
	STT           ai.STTProvider
	TTS           ai.TTSProvider
	Pronunciation ai.PronunciationProvider
	GitHub        GitHubImporter
}

func (s *Service) generateMissionWithUsage(ctx context.Context, request ai.MissionRequest) (domain.Mission, ai.JSONUsage, error) {
	if provider, ok := s.AI.(ai.UsageAwareMissionProvider); ok {
		return provider.GenerateMissionWithUsage(ctx, request)
	}
	result, err := s.AI.GenerateMission(ctx, request)
	return result, ai.JSONUsage{}, err
}

func (s *Service) evaluateWritingWithUsage(ctx context.Context, request ai.WritingRequest) (domain.Evaluation, ai.JSONUsage, error) {
	if provider, ok := s.AI.(ai.UsageAwareWritingProvider); ok {
		return provider.EvaluateWritingWithUsage(ctx, request)
	}
	result, err := s.AI.EvaluateWriting(ctx, request)
	return result, ai.JSONUsage{}, err
}

func (s *Service) generateRoleplayWithUsage(ctx context.Context, request ai.RoleplayRequest) (ai.RoleplayResult, ai.JSONUsage, error) {
	if provider, ok := s.AI.(ai.UsageAwareRoleplayProvider); ok {
		return provider.GenerateRoleplayWithUsage(ctx, request)
	}
	provider, ok := s.AI.(ai.RoleplayProvider)
	if !ok {
		return ai.RoleplayResult{}, ai.JSONUsage{}, errors.New("roleplay provider is unavailable")
	}
	result, err := provider.GenerateRoleplay(ctx, request)
	return result, ai.JSONUsage{}, err
}

func (s *Service) generateCopilotWithUsage(ctx context.Context, request ai.CopilotRequest) (domain.CopilotResult, ai.JSONUsage, error) {
	if provider, ok := s.AI.(ai.UsageAwareCopilotProvider); ok {
		return provider.GenerateCopilotWithUsage(ctx, request)
	}
	provider, ok := s.AI.(ai.CopilotProvider)
	if !ok {
		return domain.CopilotResult{}, ai.JSONUsage{}, errors.New("copilot provider is unavailable")
	}
	result, err := provider.GenerateCopilot(ctx, request)
	return result, ai.JSONUsage{}, err
}

func NewService(repository store.Repository, provider ai.Provider, speech ...SpeechDependencies) *Service {
	service := &Service{
		Store:       repository,
		AI:          provider,
		Now:         func() time.Time { return time.Now().UTC() },
		BudgetGuard: usageguard.New(),
	}
	if len(speech) > 0 {
		service.STT = speech[0].STT
		service.TTS = speech[0].TTS
		service.Pronunciation = speech[0].Pronunciation
		service.GitHub = speech[0].GitHub
	}
	return service
}

func (s *Service) Home(ctx context.Context) domain.HomeSummary {
	now := s.Now()
	mission, err := s.Store.TodayMission(ctx)
	if err != nil {
		mission, _ = s.CreateDailyMission(ctx, "")
	}
	state, _ := s.Store.LearningState(ctx)
	state = normalizeWeakest(state)
	user, _ := s.Store.User(ctx)
	recentProgress, _ := s.Store.Progress(ctx)
	reviewDue, _ := s.reviewItems(ctx, now)
	return domain.HomeSummary{User: user, LearningState: state, TodayMission: mission, ReviewDueCount: len(reviewDue), RecentProgress: recentProgress, Provider: s.providerStatus()}
}

func (s *Service) Practice(context.Context) []domain.PracticeMode {
	return []domain.PracticeMode{
		{ID: "writing", Title: "Writing", Description: "Write a bug report, PR, commit or technical explanation.", Minutes: 10, Available: true},
		{ID: "speaking", Title: "Speaking", Description: "Explain code and technical decisions out loud.", Minutes: 15, Available: true},
		{ID: "roleplay", Title: "Roleplay", Description: "Practice a focused developer conversation with AI.", Minutes: 15, Available: true},
		{ID: "system-design", Title: "System Design", Description: "Explain architecture, constraints and failure handling.", Minutes: 20, Available: true},
		{ID: "technical-interview", Title: "Technical Interview", Description: "Clarify requirements and defend technical trade-offs.", Minutes: 20, Available: true},
		{ID: "reading", Title: "Reading & Listening", Description: "Work through technical content in context.", Minutes: 10, Available: true},
		{ID: "work-import", Title: "Learn From My Work", Description: "Paste code, an error, issue or documentation to create a mission.", Minutes: 10, Available: true},
	}
}

func (s *Service) CreateDailyMission(ctx context.Context, workContext string) (domain.Mission, error) {
	stateValue, err := s.Store.LearningState(ctx)
	if err != nil {
		return domain.Mission{}, err
	}
	state := normalizeWeakest(stateValue)
	if strings.TrimSpace(workContext) == "" {
		workContext = s.recentWorkContext(ctx)
	}
	reservation, err := s.reserveBudget(ctx, estimateCost(s.AI.Name(), "deepseek-v4-flash", 700, 350), "mission")
	if err != nil {
		return domain.Mission{}, err
	}
	defer func() {
		if reservation != nil {
			_ = reservation.Release(ctx)
		}
	}()
	mission, usage, err := s.generateMissionWithUsage(ctx, ai.MissionRequest{LearningState: state, WorkContext: workContext})
	if err != nil {
		return domain.Mission{}, err
	}
	if mission.ID == "" {
		return domain.Mission{}, errors.New("provider returned an invalid mission")
	}
	if err := s.Store.SaveMission(ctx, mission); err != nil {
		return domain.Mission{}, err
	}
	if err := s.recordUsage(ctx, providerUsageRecord(s.AI.Name(), "deepseek-v4-flash", "mission", usage, s.Now())); err != nil {
		return domain.Mission{}, err
	}
	if reservation != nil {
		if err := reservation.Commit(ctx); err != nil {
			return domain.Mission{}, err
		}
	}
	return mission, nil
}

func (s *Service) Mission(ctx context.Context, id string) (domain.Mission, error) {
	return s.Store.Mission(ctx, id)
}

func (s *Service) SubmitWriting(ctx context.Context, missionID, answer string) (domain.SubmissionResult, error) {
	mission, err := s.Store.Mission(ctx, missionID)
	if err != nil {
		return domain.SubmissionResult{}, err
	}
	if strings.TrimSpace(answer) == "" {
		return domain.SubmissionResult{}, errors.New("answer is required")
	}
	reservation, err := s.reserveBudget(ctx, estimateCost(s.AI.Name(), "deepseek-v4-pro", 900, 500), "writing-evaluation")
	if err != nil {
		return domain.SubmissionResult{}, err
	}
	defer func() {
		if reservation != nil {
			_ = reservation.Release(ctx)
		}
	}()
	evaluation, usage, err := s.evaluateWritingWithUsage(ctx, ai.WritingRequest{Mission: mission, Answer: answer})
	if err != nil {
		return domain.SubmissionResult{}, err
	}
	evaluation.Score = ai.FinalWritingScore(mission, answer, evaluation)
	now := s.Now()
	attempt := domain.MissionAttempt{ID: fmt.Sprintf("attempt-%d", now.UnixNano()), MissionID: missionID, Answer: answer, Score: evaluation.Score, SubmittedAt: now}
	mistakes := make([]domain.Mistake, 0, len(evaluation.Corrections))
	for index, correction := range evaluation.Corrections {
		id := fmt.Sprintf("mistake-%s-%s-%d", stableSlug(store.UserID(ctx)), stableSlug(correction.Original), index)
		mastery := 0.2
		mistakes = append(mistakes, domain.Mistake{ID: id, Type: correction.Type, Original: correction.Original, Corrected: correction.Corrected, Context: mission.Skill, Severity: correction.Severity, Frequency: 1, LastSeen: now, NextReview: NextReview(now, mastery, false), Mastery: mastery})
	}
	success := evaluation.Score >= 70 && len(evaluation.Corrections) <= 1
	next := NextReview(now, averageMastery(mistakes), success)
	outcome := store.WritingOutcome{Attempt: attempt, Evaluation: evaluation, Mistakes: mistakes, MissionID: missionID, CompletedAt: now, Skill: mission.Skill, SkillDelta: (evaluation.Score - 60) / 10}
	if transactional, ok := s.Store.(store.TransactionalWritingRepository); ok {
		if err := transactional.SaveWritingOutcome(ctx, outcome); err != nil {
			return domain.SubmissionResult{}, err
		}
	} else {
		if err := s.Store.SaveAttempt(ctx, attempt); err != nil {
			return domain.SubmissionResult{}, err
		}
		if err := s.Store.SaveWritingEvaluation(ctx, attempt, evaluation); err != nil {
			return domain.SubmissionResult{}, err
		}
		if err := s.Store.CompleteMission(ctx, missionID, now); err != nil {
			return domain.SubmissionResult{}, err
		}
		for _, mistake := range mistakes {
			if err := s.Store.UpsertMistake(ctx, mistake); err != nil {
				return domain.SubmissionResult{}, err
			}
		}
		if err := s.Store.UpdateSkill(ctx, mission.Skill, outcome.SkillDelta); err != nil {
			return domain.SubmissionResult{}, err
		}
	}
	if err := s.recordUsage(ctx, providerUsageRecord(s.AI.Name(), "deepseek-v4-pro", "writing-evaluation", usage, now)); err != nil {
		return domain.SubmissionResult{}, err
	}
	if reservation != nil {
		if err := reservation.Commit(ctx); err != nil {
			return domain.SubmissionResult{}, err
		}
	}
	return domain.SubmissionResult{Attempt: attempt, Evaluation: evaluation, Mistakes: mistakes, NextReview: next}, nil
}

func (s *Service) ReviewDue(ctx context.Context) []domain.ReviewItem {
	items, _ := s.reviewItems(ctx, s.Now())
	return items
}

func (s *Service) reviewItems(ctx context.Context, now time.Time) ([]domain.ReviewItem, error) {
	items := make([]domain.ReviewItem, 0)
	mistakes, err := s.Store.Mistakes(ctx, now)
	if err != nil {
		return nil, err
	}
	for _, item := range mistakes {
		items = append(items, domain.ReviewItem{ID: item.ID, Kind: "mistake", Prompt: fmt.Sprintf("Correct this sentence: %s", item.Original), Answer: item.Corrected, Context: item.Context, Level: "B1", NextReview: item.NextReview})
	}
	vocabulary, err := s.Store.Vocabulary(ctx, now)
	if err != nil {
		return nil, err
	}
	for _, item := range vocabulary {
		items = append(items, domain.ReviewItem{ID: item.ID, Kind: "vocabulary", Prompt: fmt.Sprintf("Use “%s” in a technical sentence.", item.Term), Answer: item.TechnicalExample, Context: item.Definition, Level: item.Level, NextReview: item.NextReview})
	}
	return items, nil
}

func (s *Service) Progress(ctx context.Context) domain.ProgressSummary {
	stateValue, _ := s.Store.LearningState(ctx)
	state := normalizeWeakest(stateValue)
	mistakes, _ := s.Store.AllMistakes(ctx)
	trend, _ := s.Store.Progress(ctx)
	repeated := 0
	for _, item := range mistakes {
		if item.Frequency >= 2 {
			repeated++
		}
	}
	return domain.ProgressSummary{LearningState: state, RepeatedMistakes: repeated, SpeakingTrend: trend, TechnicalScore: skillScore(state, "technical_writing")}
}

func (s *Service) WorkImport(ctx context.Context, sourceType, title, content string) (domain.WorkImportResult, error) {
	return s.WorkImportWithURL(ctx, sourceType, title, content, "")
}

func (s *Service) WorkImportWithURL(ctx context.Context, sourceType, title, content, sourceURL string) (domain.WorkImportResult, error) {
	content = strings.TrimSpace(content)
	if len(content) < 20 {
		return domain.WorkImportResult{}, errors.New("work context must contain at least 20 characters")
	}
	if containsSecret(content) {
		return domain.WorkImportResult{}, errors.New("remove secrets, tokens or credentials before importing work context")
	}
	now := s.Now()
	item := domain.WorkContext{ID: fmt.Sprintf("work-%d", now.UnixNano()), SourceType: sourceType, SourceURL: strings.TrimSpace(sourceURL), Title: strings.TrimSpace(title), Content: content, Domain: detectDomain(content), CreatedAt: now}
	if err := s.Store.SaveWorkContext(ctx, item); err != nil {
		return domain.WorkImportResult{}, err
	}
	state, err := s.Store.LearningState(ctx)
	if err != nil {
		return domain.WorkImportResult{}, err
	}
	reservation, err := s.reserveBudget(ctx, estimateCost(s.AI.Name(), "deepseek-v4-flash", 700, 350), "work-context")
	if err != nil {
		return domain.WorkImportResult{}, err
	}
	defer func() {
		if reservation != nil {
			_ = reservation.Release(ctx)
		}
	}()
	mission, usage, err := s.generateMissionWithUsage(ctx, ai.MissionRequest{LearningState: state, WorkContext: content})
	if err != nil {
		return domain.WorkImportResult{}, err
	}
	if err := s.Store.SaveMission(ctx, mission); err != nil {
		return domain.WorkImportResult{}, err
	}
	if err := s.recordUsage(ctx, providerUsageRecord(s.AI.Name(), "deepseek-v4-flash", "work-context", usage, now)); err != nil {
		return domain.WorkImportResult{}, err
	}
	if reservation != nil {
		if err := reservation.Commit(ctx); err != nil {
			return domain.WorkImportResult{}, err
		}
	}
	terms := extractTerms(content)
	for _, term := range terms {
		vocabulary := domain.Vocabulary{ID: "vocab-" + stableSlug(store.UserID(ctx)) + "-" + stableSlug(term), Term: term, Domain: item.Domain, Level: state.CEFR, Definition: "A useful technical term extracted from your work context.", UserContext: term + " in this work context", TechnicalExample: "Use " + term + " in a sentence about this system.", RelatedTerms: relatedTerms(term, terms), Mastery: 0.1, NextReview: now.Add(24 * time.Hour)}
		if err := s.Store.SaveVocabulary(ctx, vocabulary); err != nil {
			return domain.WorkImportResult{}, err
		}
	}
	return domain.WorkImportResult{Context: item, SuggestedSkills: []string{state.WeakestSkill, "documentation"}, SuggestedTerms: terms, SuggestedMission: mission}, nil
}

func (s *Service) ImportGitHub(ctx context.Context, rawURL string) (domain.WorkImportResult, error) {
	if s.GitHub == nil {
		return domain.WorkImportResult{}, errors.New("GitHub integration is not configured")
	}
	imported, err := s.GitHub.Import(ctx, strings.TrimSpace(rawURL))
	if err != nil {
		return domain.WorkImportResult{}, err
	}
	return s.WorkImportWithURL(ctx, imported.SourceType, imported.Title, imported.Content, imported.URL)
}

func (s *Service) recentWorkContext(ctx context.Context) string {
	items, err := s.Store.AllWorkContexts(ctx)
	if err != nil || len(items) == 0 {
		return ""
	}
	start := 0
	if len(items) > 3 {
		start = len(items) - 3
	}
	var builder strings.Builder
	for _, item := range items[start:] {
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(item.Title)
		builder.WriteString(": ")
		builder.WriteString(item.Content)
		if builder.Len() >= 6000 {
			break
		}
	}
	contextText := builder.String()
	if len(contextText) > 6000 {
		return contextText[:6000]
	}
	return contextText
}

func (s *Service) Settings(ctx context.Context) domain.Settings {
	settings, _ := s.Store.Settings(ctx)
	if settings.DeepSeekStatus == "" {
		if settings.DeepSeekConfigured {
			settings.DeepSeekStatus = DeepSeekStatusConnected
		} else {
			settings.DeepSeekStatus = DeepSeekStatusNotConfigured
		}
	}
	return settings
}

func (s *Service) UpdateSettings(ctx context.Context, settings domain.Settings) domain.Settings {
	updated, err := s.ApplySettings(ctx, settings)
	if err != nil {
		return s.Settings(ctx)
	}
	return updated
}

func (s *Service) ApplySettings(ctx context.Context, settings domain.Settings) (domain.Settings, error) {
	current, _ := s.Store.Settings(ctx)
	if strings.TrimSpace(settings.AIProvider) == "" {
		settings.AIProvider = current.AIProvider
	}
	if strings.TrimSpace(settings.FastModel) == "" {
		settings.FastModel = current.FastModel
	}
	if strings.TrimSpace(settings.SmartModel) == "" {
		settings.SmartModel = current.SmartModel
	}
	if settings.MonthlyBudgetVND <= 0 {
		settings.MonthlyBudgetVND = current.MonthlyBudgetVND
	}
	if settings.MonthlyBudgetVND > 300000 {
		settings.MonthlyBudgetVND = 300000
	}
	if err := ai.ValidateModelName(settings.FastModel); err != nil {
		return domain.Settings{}, err
	}
	if err := ai.ValidateModelName(settings.SmartModel); err != nil {
		return domain.Settings{}, err
	}
	settings.DeepSeekConfigured = current.DeepSeekConfigured
	settings.DeepSeekStatus = current.DeepSeekStatus
	if settings.DeepSeekStatus == "" {
		settings.DeepSeekStatus = DeepSeekStatusNotConfigured
	}
	settings.SpeechConfigured = current.SpeechConfigured
	if err := s.Store.SaveSettings(ctx, settings); err != nil {
		return domain.Settings{}, err
	}
	if s.DeepSeekSecrets != nil && s.DeepSeekSecrets.Provider != nil {
		if err := s.DeepSeekSecrets.Provider.SetModels(settings.FastModel, settings.SmartModel); err != nil {
			return domain.Settings{}, err
		}
	}
	return settings, nil
}

func (s *Service) providerStatus() domain.ProviderStatus {
	status := domain.ProviderStatus{Mode: "unavailable"}
	if s.AI == nil {
		return status
	}
	status.Name = s.AI.Name()
	status.Configured = s.AI.Configured()
	if status.Name == "deterministic-fallback" {
		status.Mode = "deterministic-fallback"
	} else if status.Configured {
		status.Mode = "primary"
	}
	return status
}

func normalizeWeakest(state domain.LearningState) domain.LearningState {
	if len(state.Skills) == 0 {
		return state
	}
	weakest := 0
	for i := range state.Skills {
		if state.Skills[i].Score < state.Skills[weakest].Score {
			weakest = i
		}
	}
	for i := range state.Skills {
		state.Skills[i].IsWeakest = i == weakest
	}
	state.WeakestSkill = state.Skills[weakest].Skill
	return state
}

func skillScore(state domain.LearningState, skill string) float64 {
	for _, item := range state.Skills {
		if item.Skill == skill {
			return item.Score
		}
	}
	return 0
}
func averageMastery(items []domain.Mistake) float64 {
	if len(items) == 0 {
		return 0.2
	}
	total := 0.0
	for _, item := range items {
		total += item.Mastery
	}
	return total / float64(len(items))
}
func stableSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(" ", "-", ".", "", ",", "").Replace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func detectDomain(content string) string {
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "flutter"):
		return "flutter"
	case strings.Contains(lower, "postgres") || strings.Contains(lower, "sql"):
		return "database"
	case strings.Contains(lower, "api") || strings.Contains(lower, "http"):
		return "backend"
	case strings.Contains(lower, "git") || strings.Contains(lower, "pull request"):
		return "git"
	default:
		return "software-engineering"
	}
}

func extractTerms(content string) []string {
	known := []string{"root cause", "reproduce", "expected behavior", "trade-off", "rollback", "latency", "dependency", "deployment", "schema", "endpoint"}
	lower := strings.ToLower(content)
	terms := make([]string, 0, 4)
	for _, term := range known {
		if strings.Contains(lower, term) {
			terms = append(terms, term)
		}
	}
	if len(terms) == 0 {
		terms = append(terms, "observed behavior", "next step")
	}
	if len(terms) > 5 {
		terms = terms[:5]
	}
	return terms
}

func containsSecret(content string) bool {
	lower := strings.ToLower(content)
	patterns := []string{"authorization: bearer ", "api_key=", "api-key:", "secret_key=", "password=", "-----begin private key-----", "ghp_", "sk-"}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
