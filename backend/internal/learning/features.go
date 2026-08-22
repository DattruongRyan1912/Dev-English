package learning

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

var ErrBudgetExceeded = errors.New("monthly AI budget would be exceeded")

var diagnosticQuestionBank = []domain.DiagnosticQuestion{
	{ID: "documentation-1", Skill: "documentation", Mode: "reading", Prompt: "How comfortable are you summarizing a technical document for a teammate?", Options: diagnosticOptions()},
	{ID: "technical-writing-1", Skill: "technical_writing", Mode: "writing", Prompt: "How comfortable are you writing a bug report with observed and expected behavior?", Options: diagnosticOptions()},
	{ID: "git-communication-1", Skill: "git_communication", Mode: "writing", Prompt: "How comfortable are you explaining a pull request trade-off in English?", Options: diagnosticOptions()},
	{ID: "speaking-1", Skill: "speaking", Mode: "speaking", Prompt: "How comfortable are you explaining a technical decision out loud?", Options: diagnosticOptions()},
	{ID: "meeting-1", Skill: "meeting", Mode: "speaking", Prompt: "How comfortable are you asking for clarification in a technical meeting?", Options: diagnosticOptions()},
	{ID: "system-design-1", Skill: "system_design", Mode: "speaking", Prompt: "How comfortable are you describing an architecture and its failure modes?", Options: diagnosticOptions()},
	{ID: "documentation-2", Skill: "documentation", Mode: "writing", Prompt: "Choose the clearest sentence for a README: the endpoint returns an error when the token expires.", Options: []string{"It error token", "Endpoint error token expired", "The endpoint returns an error when the token expires", "The endpoint, it is returning an error for token things"}},
	{ID: "technical-writing-2", Skill: "technical_writing", Mode: "writing", Prompt: "Which detail makes a bug report reproducible?", Options: []string{"A strong opinion", "Steps, input and observed result", "Only the ticket title", "A screenshot without context"}},
	{ID: "git-communication-2", Skill: "git_communication", Mode: "writing", Prompt: "Which PR sentence is most useful?", Options: []string{"This changes stuff", "Please review", "This adds a retry for transient 502 responses and keeps the timeout unchanged", "I changed the code"}},
	{ID: "speaking-2", Skill: "speaking", Mode: "speaking", Prompt: "Which structure is best for a short technical explanation?", Options: []string{"Context, evidence, impact, next step", "Only the conclusion", "Several unrelated details", "Apology and silence"}},
	{ID: "meeting-2", Skill: "meeting", Mode: "speaking", Prompt: "Which question clarifies an ambiguous requirement?", Options: []string{"What do you mean?", "Could we define the expected behavior for an empty result?", "Why is this hard?", "Just build it"}},
	{ID: "system-design-2", Skill: "system_design", Mode: "speaking", Prompt: "What should you mention when proposing a queue?", Options: []string{"Only the technology name", "Latency, failure handling and operational trade-offs", "The color of the dashboard", "Nothing until production"}},
}

func diagnosticOptions() []string {
	return []string{"I need a lot of support", "I can do this with a template", "I can do this independently", "I can coach another developer"}
}

func (s *Service) DiagnosticQuestions() []domain.DiagnosticQuestion {
	questions := make([]domain.DiagnosticQuestion, len(diagnosticQuestionBank))
	copy(questions, diagnosticQuestionBank)
	return questions
}

func (s *Service) Diagnostic(ctx context.Context) (domain.DiagnosticResult, error) {
	return s.Store.Diagnostic(ctx)
}

func (s *Service) RunDiagnostic(ctx context.Context, responses []domain.DiagnosticResponse) (domain.DiagnosticResult, error) {
	if len(responses) == 0 {
		return domain.DiagnosticResult{}, errors.New("at least one diagnostic response is required")
	}
	answers := make(map[string]string, len(responses))
	for _, response := range responses {
		if strings.TrimSpace(response.QuestionID) == "" || strings.TrimSpace(response.Answer) == "" {
			return domain.DiagnosticResult{}, errors.New("diagnostic question and answer are required")
		}
		answers[response.QuestionID] = response.Answer
	}
	bySkill := make(map[string][]float64)
	for _, question := range diagnosticQuestionBank {
		answer, ok := answers[question.ID]
		if !ok {
			continue
		}
		score := diagnosticAnswerScore(question, answer)
		bySkill[question.Skill] = append(bySkill[question.Skill], score)
	}
	if len(bySkill) == 0 {
		return domain.DiagnosticResult{}, errors.New("responses do not match the diagnostic question bank")
	}
	state, err := s.Store.LearningState(ctx)
	if err != nil {
		return domain.DiagnosticResult{}, err
	}
	for index := range state.Skills {
		if scores := bySkill[state.Skills[index].Skill]; len(scores) > 0 {
			total := 0.0
			for _, score := range scores {
				total += score
			}
			state.Skills[index].Score = total / float64(len(scores))
			state.Skills[index].Level = cefrForScore(state.Skills[index].Score)
			state.Skills[index].Trend = 0
		}
	}
	state = recomputeState(state)
	result := domain.DiagnosticResult{CEFR: state.CEFR, OverallScore: state.OverallScore, SkillScores: append([]domain.SkillState(nil), state.Skills...), Strengths: topSkills(state.Skills, true, 2), Priorities: topSkills(state.Skills, false, 3), RecommendedPlan: []string{"Start with a 10-minute technical writing mission", "Review two mistakes after each mission", "Practice one speaking explanation before the weekly checkpoint"}}
	if err := s.Store.SaveLearningState(ctx, state); err != nil {
		return domain.DiagnosticResult{}, err
	}
	if err := s.Store.SaveDiagnostic(ctx, result); err != nil {
		return domain.DiagnosticResult{}, err
	}
	return result, nil
}

func diagnosticAnswerScore(question domain.DiagnosticQuestion, answer string) float64 {
	answer = strings.TrimSpace(answer)
	for index, option := range question.Options {
		if strings.EqualFold(answer, option) {
			if len(question.Options) == 4 && (strings.Contains(question.ID, "-2") || strings.HasSuffix(question.ID, "-1") && !strings.Contains(answer, "support")) {
				return []float64{20, 45, 72, 90}[index]
			}
			return []float64{25, 50, 75, 100}[index]
		}
	}
	return 35
}

func recomputeState(state domain.LearningState) domain.LearningState {
	if len(state.Skills) == 0 {
		return state
	}
	weakest, total := 0, 0.0
	for index := range state.Skills {
		state.Skills[index].IsWeakest = false
		state.Skills[index].Level = cefrForScore(state.Skills[index].Score)
		total += state.Skills[index].Score
		if state.Skills[index].Score < state.Skills[weakest].Score {
			weakest = index
		}
	}
	state.Skills[weakest].IsWeakest = true
	state.WeakestSkill = state.Skills[weakest].Skill
	state.OverallScore = total / float64(len(state.Skills))
	state.CEFR = cefrForScore(state.OverallScore)
	return state
}

func cefrForScore(score float64) string {
	switch {
	case score >= 85:
		return "C1"
	case score >= 70:
		return "B2"
	case score >= 50:
		return "B1"
	case score >= 30:
		return "A2"
	default:
		return "A1"
	}
}

func topSkills(skills []domain.SkillState, strongest bool, limit int) []string {
	copySkills := append([]domain.SkillState(nil), skills...)
	sort.Slice(copySkills, func(i, j int) bool {
		if strongest {
			return copySkills[i].Score > copySkills[j].Score
		}
		return copySkills[i].Score < copySkills[j].Score
	})
	if len(copySkills) > limit {
		copySkills = copySkills[:limit]
	}
	result := make([]string, 0, len(copySkills))
	for _, skill := range copySkills {
		result = append(result, skill.Skill)
	}
	return result
}

func (s *Service) RoleplayScenarios(ctx context.Context) ([]domain.RoleplayScenario, error) {
	return s.Store.Scenarios(ctx)
}

func (s *Service) StartRoleplay(ctx context.Context, scenarioID string) (domain.Conversation, error) {
	var scenario domain.RoleplayScenario
	scenarios, err := s.Store.Scenarios(ctx)
	if err != nil {
		return domain.Conversation{}, err
	}
	for _, item := range scenarios {
		if item.ID == scenarioID {
			scenario = item
			break
		}
	}
	if scenario.ID == "" {
		return domain.Conversation{}, store.ErrNotFound
	}
	now := s.Now()
	conversation := domain.Conversation{ID: fmt.Sprintf("conversation-%d", now.UnixNano()), RoleplayType: scenario.ID, Context: scenario.Context, Status: "active", CreatedAt: now, Messages: []domain.Message{{ID: fmt.Sprintf("message-%d", now.UnixNano()), Role: "assistant", Content: scenario.Opening, CreatedAt: now}}}
	if err := s.Store.SaveConversation(ctx, conversation); err != nil {
		return domain.Conversation{}, err
	}
	return conversation, nil
}

func (s *Service) RoleplayTurn(ctx context.Context, request domain.RoleplayTurnRequest) (domain.RoleplayTurnResult, error) {
	if strings.TrimSpace(request.Answer) == "" {
		return domain.RoleplayTurnResult{}, errors.New("answer is required")
	}
	conversation, err := s.Store.Conversation(ctx, request.ConversationID)
	if err != nil {
		return domain.RoleplayTurnResult{}, err
	}
	scenario, err := s.scenario(ctx, conversation.RoleplayType)
	if err != nil {
		return domain.RoleplayTurnResult{}, err
	}
	provider, ok := s.AI.(ai.RoleplayProvider)
	if !ok {
		return domain.RoleplayTurnResult{}, errors.New("roleplay provider is unavailable")
	}
	if err := s.ensureBudget(ctx, estimateCost(s.AI.Name(), "deepseek-v4-pro", 800, 500)); err != nil {
		return domain.RoleplayTurnResult{}, err
	}
	result, err := provider.GenerateRoleplay(ctx, ai.RoleplayRequest{Scenario: scenario, Conversation: conversation, Answer: request.Answer})
	if err != nil {
		return domain.RoleplayTurnResult{}, err
	}
	result.Evaluation.Score = ai.FinalWritingScore(domain.Mission{ExpectedPoints: []string{"reason", "impact", "next step"}}, request.Answer, result.Evaluation)
	now := s.Now()
	conversation.Messages = append(conversation.Messages, domain.Message{ID: fmt.Sprintf("message-%d-user", now.UnixNano()), Role: "user", Content: strings.TrimSpace(request.Answer), CreatedAt: now}, domain.Message{ID: fmt.Sprintf("message-%d-assistant", now.UnixNano()), Role: "assistant", Content: result.Reply, CreatedAt: now.Add(time.Nanosecond)})
	if err := s.Store.SaveConversation(ctx, conversation); err != nil {
		return domain.RoleplayTurnResult{}, err
	}
	_ = s.recordUsage(ctx, domain.UsageRecord{Provider: s.AI.Name(), Model: "deepseek-v4-pro", Feature: "roleplay", InputTokens: len(strings.Fields(request.Answer)) * 2, OutputTokens: len(strings.Fields(result.Reply)) * 2, EstimatedCost: estimateCost(s.AI.Name(), "deepseek-v4-pro", len(strings.Fields(request.Answer))*2, len(strings.Fields(result.Reply))*2), CreatedAt: now})
	return domain.RoleplayTurnResult{Conversation: conversation, Reply: conversation.Messages[len(conversation.Messages)-1], Feedback: result.Evaluation}, nil
}

func (s *Service) scenario(ctx context.Context, id string) (domain.RoleplayScenario, error) {
	scenarios, err := s.Store.Scenarios(ctx)
	if err != nil {
		return domain.RoleplayScenario{}, err
	}
	for _, scenario := range scenarios {
		if scenario.ID == id {
			return scenario, nil
		}
	}
	return domain.RoleplayScenario{}, store.ErrNotFound
}

func (s *Service) Copilot(ctx context.Context, request domain.CopilotRequest) (domain.CopilotResult, error) {
	if strings.TrimSpace(request.Vietnamese) == "" {
		return domain.CopilotResult{}, errors.New("vietnamese text is required")
	}
	provider, ok := s.AI.(ai.CopilotProvider)
	if !ok {
		return domain.CopilotResult{}, errors.New("copilot provider is unavailable")
	}
	if err := s.ensureBudget(ctx, estimateCost(s.AI.Name(), "deepseek-v4-flash", 500, 300)); err != nil {
		return domain.CopilotResult{}, err
	}
	result, err := provider.GenerateCopilot(ctx, ai.CopilotRequest{Vietnamese: request.Vietnamese, Context: request.Context})
	if err != nil {
		return domain.CopilotResult{}, err
	}
	now := s.Now()
	_ = s.recordUsage(ctx, domain.UsageRecord{Provider: s.AI.Name(), Model: "deepseek-v4-flash", Feature: "copilot", InputTokens: len(strings.Fields(request.Vietnamese)) * 2, OutputTokens: len(strings.Fields(result.Professional)) * 2, EstimatedCost: estimateCost(s.AI.Name(), "deepseek-v4-flash", len(strings.Fields(request.Vietnamese))*2, len(strings.Fields(result.Professional))*2), CreatedAt: now})
	return result, nil
}

func (s *Service) ReviewLibrary(ctx context.Context) ([]domain.ReviewItem, error) {
	mistakes, err := s.Store.AllMistakes(ctx)
	if err != nil {
		return nil, err
	}
	vocabulary, err := s.Store.AllVocabulary(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.ReviewItem, 0, len(mistakes)+len(vocabulary))
	for _, item := range mistakes {
		items = append(items, domain.ReviewItem{ID: item.ID, Kind: "mistake", Prompt: fmt.Sprintf("Correct this sentence: %s", item.Original), Answer: item.Corrected, Context: item.Context, Level: "B1", NextReview: item.NextReview})
	}
	for _, item := range vocabulary {
		items = append(items, domain.ReviewItem{ID: item.ID, Kind: "vocabulary", Prompt: fmt.Sprintf("Use “%s” in a technical sentence.", item.Term), Answer: item.TechnicalExample, Context: item.Definition, Level: item.Level, NextReview: item.NextReview})
	}
	return items, nil
}

func (s *Service) SubmitReview(ctx context.Context, kind, id string, success bool, score float64) (any, error) {
	now := s.Now()
	score = clamp(score, 0, 100)
	switch kind {
	case "mistake":
		return s.Store.ReviewMistake(ctx, id, success, score, now)
	case "vocabulary":
		return s.Store.ReviewVocabulary(ctx, id, success, score, now)
	default:
		return nil, errors.New("review kind must be mistake or vocabulary")
	}
}

func (s *Service) TranscribeAudio(ctx context.Context, missionID, mimeType string, audio []byte) (domain.SpeakingSession, error) {
	if s.STT == nil || !s.STT.Configured() {
		return domain.SpeakingSession{}, ai.ErrProviderUnavailable
	}
	if err := s.ensureBudget(ctx, estimateSpeechCost(s.STT.Name(), "stt", 1)); err != nil {
		return domain.SpeakingSession{}, err
	}
	transcript, err := s.STT.Transcribe(ctx, audio, mimeType)
	if err != nil {
		return domain.SpeakingSession{}, err
	}
	now := s.Now()
	expires := now.Add(24 * time.Hour)
	session := domain.SpeakingSession{ID: fmt.Sprintf("speaking-%d", now.UnixNano()), MissionID: missionID, Status: "transcribed", Transcript: transcript.Text, CreatedAt: now, RawAudioExpiresAt: &expires}
	if err := s.Store.SaveSpeakingSession(ctx, session); err != nil {
		return domain.SpeakingSession{}, err
	}
	_ = s.recordUsage(ctx, domain.UsageRecord{Provider: s.STT.Name(), Model: "whisper-large-v3", Feature: "stt", AudioSeconds: 0, EstimatedCost: estimateSpeechCost(s.STT.Name(), "stt", 1), CreatedAt: now})
	return session, nil
}

func (s *Service) SaveTranscript(ctx context.Context, missionID, transcript string) (domain.SpeakingSession, error) {
	transcript = strings.TrimSpace(transcript)
	if transcript == "" || len([]rune(transcript)) > 5000 {
		return domain.SpeakingSession{}, errors.New("transcript must contain between 1 and 5000 characters")
	}
	now := s.Now()
	expires := now.Add(24 * time.Hour)
	session := domain.SpeakingSession{ID: fmt.Sprintf("speaking-text-%d", now.UnixNano()), MissionID: missionID, Status: "transcribed", Transcript: transcript, CreatedAt: now, RawAudioExpiresAt: &expires}
	if err := s.Store.SaveSpeakingSession(ctx, session); err != nil {
		return domain.SpeakingSession{}, err
	}
	return session, nil
}

func (s *Service) AssessSpeaking(ctx context.Context, sessionID, mimeType, reference string, audio []byte) (domain.SpeakingSession, error) {
	if s.Pronunciation == nil || !s.Pronunciation.Configured() {
		return domain.SpeakingSession{}, ai.ErrProviderUnavailable
	}
	if err := s.ensureBudget(ctx, estimateSpeechCost(s.Pronunciation.Name(), "pronunciation", 1)); err != nil {
		return domain.SpeakingSession{}, err
	}
	session, err := s.Store.SpeakingSession(ctx, sessionID)
	if err != nil {
		return domain.SpeakingSession{}, err
	}
	if reference == "" {
		reference = session.Transcript
	}
	result, err := s.Pronunciation.Assess(ctx, audio, mimeType, reference)
	if err != nil {
		return domain.SpeakingSession{}, err
	}
	score := domain.PronunciationScore{Accuracy: result.Accuracy, Fluency: result.Fluency, Completeness: result.Completeness, Prosody: result.Prosody, Words: result.Words}
	score.Score = ai.FinalSpeakingScore(result, session.Transcript)
	session.Pronunciation = &score
	session.Status = "evaluated"
	if err := s.Store.SaveSpeakingSession(ctx, session); err != nil {
		return domain.SpeakingSession{}, err
	}
	now := s.Now()
	_ = s.recordUsage(ctx, domain.UsageRecord{Provider: s.Pronunciation.Name(), Model: "azure-pronunciation-prosody", Feature: "pronunciation", EstimatedCost: estimateSpeechCost(s.Pronunciation.Name(), "pronunciation", 1), CreatedAt: now})
	return session, nil
}

func (s *Service) Synthesize(ctx context.Context, text, voice string) ([]byte, error) {
	if s.TTS == nil || !s.TTS.Configured() {
		return nil, ai.ErrProviderUnavailable
	}
	text = strings.TrimSpace(text)
	if text == "" || len([]rune(text)) > 5000 {
		return nil, errors.New("text must contain between 1 and 5000 characters")
	}
	if err := s.ensureBudget(ctx, estimateSpeechCost(s.TTS.Name(), "tts", len([]rune(text)))); err != nil {
		return nil, err
	}
	audio, err := s.TTS.Synthesize(ctx, text, voice)
	if err != nil {
		return nil, err
	}
	now := s.Now()
	_ = s.recordUsage(ctx, domain.UsageRecord{Provider: s.TTS.Name(), Model: "azure-neural-tts", Feature: "tts", TTSCharacters: len([]rune(text)), EstimatedCost: estimateSpeechCost(s.TTS.Name(), "tts", len([]rune(text))), CreatedAt: now})
	return audio, nil
}

func (s *Service) ensureBudget(ctx context.Context, estimate float64) error {
	if estimate <= 0 {
		return nil
	}
	settings, err := s.Store.Settings(ctx)
	if err != nil {
		return err
	}
	budget := settings.MonthlyBudgetVND
	if budget <= 0 || budget > 300000 {
		budget = 300000
	}
	now := s.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	records, err := s.Store.Usage(ctx, from)
	if err != nil {
		return err
	}
	used := 0.0
	for _, record := range records {
		used += record.EstimatedCost
	}
	if used+estimate > float64(budget) {
		return ErrBudgetExceeded
	}
	return nil
}

func estimateSpeechCost(provider, feature string, units int) float64 {
	if units <= 0 || provider == "" || provider == "deterministic-fallback" {
		return 0
	}
	// Keep the cap conservative without coupling the service to a provider price sheet.
	// TTS is character-based; STT and pronunciation use one bounded request unit.
	if feature == "tts" {
		return float64(units) * 0.05
	}
	return 250
}

func (s *Service) UsageSummary(ctx context.Context) (domain.UsageSummary, error) {
	now := s.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	records, err := s.Store.Usage(ctx, from)
	if err != nil {
		return domain.UsageSummary{}, err
	}
	settings, err := s.Store.Settings(ctx)
	if err != nil {
		return domain.UsageSummary{}, err
	}
	total := 0.0
	for _, record := range records {
		total += record.EstimatedCost
	}
	used := 0.0
	if settings.MonthlyBudgetVND > 0 {
		used = total / float64(settings.MonthlyBudgetVND) * 100
	}
	return domain.UsageSummary{Month: now.Format("2006-01"), EstimatedCost: total, BudgetVND: settings.MonthlyBudgetVND, BudgetUsedPercent: used, Records: records}, nil
}

func (s *Service) ExportData(ctx context.Context) (map[string]any, error) {
	user, err := s.Store.User(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.Store.LearningState(ctx)
	if err != nil {
		return nil, err
	}
	missions, err := s.Store.AllMissions(ctx)
	if err != nil {
		return nil, err
	}
	workContexts, err := s.Store.AllWorkContexts(ctx)
	if err != nil {
		return nil, err
	}
	mistakes, err := s.Store.AllMistakes(ctx)
	if err != nil {
		return nil, err
	}
	vocabulary, err := s.Store.AllVocabulary(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := s.Store.Settings(ctx)
	if err != nil {
		return nil, err
	}
	progress, err := s.Store.Progress(ctx)
	if err != nil {
		return nil, err
	}
	diagnostic, _ := s.Store.Diagnostic(ctx)
	usage, err := s.UsageSummary(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"exportedAt": s.Now(), "user": user, "learningState": state, "missions": missions, "workContexts": workContexts, "mistakes": mistakes, "vocabulary": vocabulary, "settings": settings, "progress": progress, "diagnostic": diagnostic, "usage": usage}, nil
}

func (s *Service) recordUsage(ctx context.Context, record domain.UsageRecord) error {
	if record.CreatedAt.IsZero() {
		record.CreatedAt = s.Now()
	}
	return s.Store.SaveUsage(ctx, record)
}

func estimateCost(provider, model string, input, output int) float64 {
	if provider == "deterministic-fallback" {
		return 0
	}
	perMillion := 0.20
	if strings.Contains(model, "pro") {
		perMillion = 1.00
	}
	return float64(input+output) / 1_000_000 * perMillion * 25_000
}

func clamp(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
