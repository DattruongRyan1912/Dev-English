package store

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
)

var ErrNotFound = errors.New("resource not found")

type MemoryStore struct {
	mu             sync.RWMutex
	user           domain.User
	learningState  domain.LearningState
	missions       map[string]domain.Mission
	attempts       []domain.MissionAttempt
	mistakes       map[string]domain.Mistake
	vocabulary     map[string]domain.Vocabulary
	workContexts   map[string]domain.WorkContext
	settings       domain.Settings
	deepSeekSecret *EncryptedSecret
	progress       []domain.ProgressPoint
	diagnostic     *domain.DiagnosticResult
	scenarios      []domain.RoleplayScenario
	conversations  map[string]domain.Conversation
	speaking       map[string]domain.SpeakingSession
	usage          []domain.UsageRecord
	reservations   map[string]memoryUsageReservation
}

type memoryUsageReservation struct {
	request UsageReservationRequest
	status  string
}

func NewSeeded(now time.Time) *MemoryStore {
	state := domain.LearningState{
		CEFR:              "B1",
		OverallScore:      58,
		WeakestSkill:      "technical_writing",
		RecentTopics:      []string{"API error handling", "Flutter state", "PostgreSQL"},
		CurrentPriorities: []string{"write clearer bug reports", "explain root cause", "review technical vocabulary"},
		Skills: []domain.SkillState{
			{Skill: "documentation", Label: "Documentation", Score: 66, Level: "B1", Trend: 4, Description: "Read and summarize technical docs."},
			{Skill: "technical_writing", Label: "Technical Writing", Score: 48, Level: "B1", Trend: 2, IsWeakest: true, Description: "Write clear developer-facing messages."},
			{Skill: "git_communication", Label: "Git Communication", Score: 54, Level: "B1", Trend: 3, Description: "Write commits, issues and PRs."},
			{Skill: "speaking", Label: "Speaking", Score: 57, Level: "B1", Trend: 5, Description: "Explain code and technical decisions."},
			{Skill: "meeting", Label: "Meeting", Score: 61, Level: "B1", Trend: 1, Description: "Clarify requirements and discuss trade-offs."},
			{Skill: "system_design", Label: "System Design", Score: 45, Level: "A2", Trend: 0, Description: "Describe architecture and failure handling."},
		},
	}
	mission := domain.Mission{
		ID:               "mission-today",
		Title:            "Write a useful bug report",
		Mode:             domain.ModeWriting,
		Skill:            "technical_writing",
		SkillLabel:       "Technical Writing",
		Level:            "B1",
		Context:          "Your API returns HTTP 500 when a user uploads a file larger than 10 MB.",
		Prompt:           "Write a concise bug report for the backend team. Include the observed behavior, expected behavior, likely impact, and one useful next step.",
		TargetVocabulary: []string{"reproduce", "expected behavior", "root cause", "impact"},
		ExpectedPoints:   []string{"observed behavior", "expected behavior", "impact", "next step"},
		EstimatedMinutes: 10,
		Status:           "available",
		CreatedAt:        now,
	}
	return &MemoryStore{
		user:          domain.User{ID: "user-1", DisplayName: "Developer", CEFR: "B1", CreatedAt: now.Add(-14 * 24 * time.Hour)},
		learningState: state,
		missions:      map[string]domain.Mission{mission.ID: mission},
		attempts:      []domain.MissionAttempt{},
		mistakes: map[string]domain.Mistake{
			"mistake-1": {ID: "mistake-1", Type: "article", Original: "I created bug report", Corrected: "I created a bug report", Context: "technical writing", Severity: 2, Frequency: 3, LastSeen: now.Add(-24 * time.Hour), NextReview: now.Add(-2 * time.Hour), Mastery: 0.35},
			"mistake-2": {ID: "mistake-2", Type: "collocation", Original: "discuss about the issue", Corrected: "discuss the issue", Context: "meeting", Severity: 2, Frequency: 2, LastSeen: now.Add(-3 * 24 * time.Hour), NextReview: now.Add(4 * 24 * time.Hour), Mastery: 0.5},
		},
		vocabulary: map[string]domain.Vocabulary{
			"vocab-1": {ID: "vocab-1", Term: "reproduce", Domain: "backend", Level: "B1", Definition: "to make a problem happen again", UserContext: "reproduce an API error", TechnicalExample: "We can reproduce the 500 error with a 12 MB file.", RelatedTerms: []string{"root cause", "observed behavior"}, Mastery: 0.45, NextReview: now.Add(-1 * time.Hour)},
			"vocab-2": {ID: "vocab-2", Term: "root cause", Domain: "debugging", Level: "B1", Definition: "the fundamental reason a problem occurs", UserContext: "identify the root cause", TechnicalExample: "The root cause is an unchecked upload limit.", RelatedTerms: []string{"reproduce", "next step"}, Mastery: 0.6, NextReview: now.Add(24 * time.Hour)},
		},
		workContexts: map[string]domain.WorkContext{},
		settings:     domain.Settings{AIProvider: "deepseek", FastModel: "deepseek-v4-flash", SmartModel: "deepseek-v4-pro", DeepSeekConfigured: false, DeepSeekStatus: "not_configured", SpeechConfigured: false, PronunciationOn: false, MonthlyBudgetVND: 150000},
		progress: []domain.ProgressPoint{
			{Date: now.AddDate(0, 0, -6).Format("2006-01-02"), Score: 49},
			{Date: now.AddDate(0, 0, -5).Format("2006-01-02"), Score: 51},
			{Date: now.AddDate(0, 0, -4).Format("2006-01-02"), Score: 52},
			{Date: now.AddDate(0, 0, -3).Format("2006-01-02"), Score: 54},
			{Date: now.AddDate(0, 0, -2).Format("2006-01-02"), Score: 56},
			{Date: now.AddDate(0, 0, -1).Format("2006-01-02"), Score: 57},
			{Date: now.Format("2006-01-02"), Score: 58},
		},
		scenarios:     defaultScenarios(),
		conversations: map[string]domain.Conversation{},
		speaking:      map[string]domain.SpeakingSession{},
		usage:         []domain.UsageRecord{},
		reservations:  map[string]memoryUsageReservation{},
	}
}

func (s *MemoryStore) Ready(context.Context) error { return nil }

func (s *MemoryStore) EnsureUser(_ context.Context, user domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if user.ID == "" || user.ID == s.user.ID {
		return nil
	}
	return ErrNotFound
}

func (s *MemoryStore) User(context.Context) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.user, nil
}

func (s *MemoryStore) LearningState(context.Context) (domain.LearningState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state := s.learningState
	state.Skills = append([]domain.SkillState(nil), state.Skills...)
	state.RecentTopics = append([]string(nil), state.RecentTopics...)
	state.CurrentPriorities = append([]string(nil), state.CurrentPriorities...)
	return state, nil
}

func (s *MemoryStore) SaveLearningState(_ context.Context, state domain.LearningState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.learningState = state
	return nil
}

func (s *MemoryStore) Mission(_ context.Context, id string) (domain.Mission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mission, ok := s.missions[id]
	if !ok {
		return domain.Mission{}, ErrNotFound
	}
	return mission, nil
}

func (s *MemoryStore) AllMissions(context.Context) ([]domain.Mission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Mission, 0, len(s.missions))
	for _, mission := range s.missions {
		items = append(items, mission)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (s *MemoryStore) SaveMission(_ context.Context, mission domain.Mission) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.missions[mission.ID] = mission
	return nil
}

func (s *MemoryStore) TodayMission(ctx context.Context) (domain.Mission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, mission := range s.missions {
		if mission.Status != "completed" {
			return mission, nil
		}
	}
	return domain.Mission{}, ErrNotFound
}

func (s *MemoryStore) SaveAttempt(_ context.Context, attempt domain.MissionAttempt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts = append(s.attempts, attempt)
	return nil
}

func (s *MemoryStore) SaveWritingEvaluation(_ context.Context, _ domain.MissionAttempt, _ domain.Evaluation) error {
	return nil
}

func (s *MemoryStore) SaveWritingOutcome(_ context.Context, outcome WritingOutcome) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mission, ok := s.missions[outcome.MissionID]
	if !ok {
		return ErrNotFound
	}
	s.attempts = append(s.attempts, outcome.Attempt)
	mission.Status = "completed"
	mission.CompletedAt = &outcome.CompletedAt
	s.missions[outcome.MissionID] = mission
	for _, item := range outcome.Mistakes {
		if current, exists := s.mistakes[item.ID]; exists {
			item.Frequency = current.Frequency + 1
		}
		s.mistakes[item.ID] = item
	}
	for index := range s.learningState.Skills {
		if s.learningState.Skills[index].Skill == outcome.Skill {
			s.learningState.Skills[index].Score = clampScore(s.learningState.Skills[index].Score + outcome.SkillDelta)
			if outcome.SkillDelta > 0 {
				s.learningState.Skills[index].Trend += outcome.SkillDelta
			}
		}
	}
	s.learningState = recomputeLearningState(s.learningState)
	return nil
}

func (s *MemoryStore) Mistakes(_ context.Context, now time.Time) ([]domain.Mistake, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Mistake, 0, len(s.mistakes))
	for _, item := range s.mistakes {
		if !item.NextReview.After(now) {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].NextReview.Before(items[j].NextReview) })
	return items, nil
}

func (s *MemoryStore) AllMistakes(context.Context) ([]domain.Mistake, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Mistake, 0, len(s.mistakes))
	for _, item := range s.mistakes {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Mastery < items[j].Mastery })
	return items, nil
}

func (s *MemoryStore) UpsertMistake(_ context.Context, item domain.Mistake) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.mistakes[item.ID]; ok {
		item.Frequency = current.Frequency + 1
	}
	s.mistakes[item.ID] = item
	return nil
}

func (s *MemoryStore) ReviewMistake(_ context.Context, id string, success bool, score float64, now time.Time) (domain.Mistake, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.mistakes[id]
	if !ok {
		return domain.Mistake{}, ErrNotFound
	}
	item.Mastery = updateMastery(item.Mastery, success, score)
	item.LastSeen = now
	item.NextReview = reviewAt(now, item.Mastery, success)
	if !success {
		item.Frequency++
	}
	s.mistakes[id] = item
	return item, nil
}

func (s *MemoryStore) Vocabulary(_ context.Context, now time.Time) ([]domain.Vocabulary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Vocabulary, 0, len(s.vocabulary))
	for _, item := range s.vocabulary {
		if !item.NextReview.After(now) {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].NextReview.Before(items[j].NextReview) })
	return items, nil
}

func (s *MemoryStore) AllVocabulary(context.Context) ([]domain.Vocabulary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Vocabulary, 0, len(s.vocabulary))
	for _, item := range s.vocabulary {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Mastery < items[j].Mastery })
	return items, nil
}

func (s *MemoryStore) SaveVocabulary(_ context.Context, item domain.Vocabulary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vocabulary[item.ID] = item
	return nil
}

func (s *MemoryStore) ReviewVocabulary(_ context.Context, id string, success bool, score float64, now time.Time) (domain.Vocabulary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.vocabulary[id]
	if !ok {
		return domain.Vocabulary{}, ErrNotFound
	}
	item.Mastery = updateMastery(item.Mastery, success, score)
	item.NextReview = reviewAt(now, item.Mastery, success)
	s.vocabulary[id] = item
	return item, nil
}

func (s *MemoryStore) SaveWorkContext(_ context.Context, item domain.WorkContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workContexts[item.ID] = item
	return nil
}

func (s *MemoryStore) AllWorkContexts(context.Context) ([]domain.WorkContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.WorkContext, 0, len(s.workContexts))
	for _, item := range s.workContexts {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (s *MemoryStore) Settings(context.Context) (domain.Settings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings, nil
}

func (s *MemoryStore) SaveSettings(_ context.Context, settings domain.Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = settings
	return nil
}

func (s *MemoryStore) DeepSeekSecret(context.Context) (EncryptedSecret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.deepSeekSecret == nil {
		return EncryptedSecret{}, ErrNotFound
	}
	return EncryptedSecret{
		Ciphertext: append([]byte(nil), s.deepSeekSecret.Ciphertext...),
		Nonce:      append([]byte(nil), s.deepSeekSecret.Nonce...),
	}, nil
}

func (s *MemoryStore) SaveDeepSeekSecret(_ context.Context, secret EncryptedSecret) error {
	if len(secret.Ciphertext) == 0 || len(secret.Nonce) == 0 {
		return errors.New("encrypted DeepSeek secret is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deepSeekSecret = &EncryptedSecret{
		Ciphertext: append([]byte(nil), secret.Ciphertext...),
		Nonce:      append([]byte(nil), secret.Nonce...),
	}
	return nil
}

func (s *MemoryStore) DeleteDeepSeekSecret(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deepSeekSecret = nil
	return nil
}

func (s *MemoryStore) Progress(context.Context) ([]domain.ProgressPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.ProgressPoint(nil), s.progress...), nil
}

func (s *MemoryStore) CompleteMission(_ context.Context, id string, completedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mission, ok := s.missions[id]
	if !ok {
		return ErrNotFound
	}
	mission.Status = "completed"
	mission.CompletedAt = &completedAt
	s.missions[id] = mission
	return nil
}

func (s *MemoryStore) UpdateSkill(_ context.Context, skill string, delta float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.learningState.Skills {
		if s.learningState.Skills[i].Skill == skill {
			s.learningState.Skills[i].Score = clamp(s.learningState.Skills[i].Score+delta, 0, 100)
			if delta > 0 {
				s.learningState.Skills[i].Trend += delta
			}
		}
	}
	sort.SliceStable(s.learningState.Skills, func(i, j int) bool { return s.learningState.Skills[i].Score < s.learningState.Skills[j].Score })
	for i := range s.learningState.Skills {
		s.learningState.Skills[i].IsWeakest = i == 0
	}
	if len(s.learningState.Skills) > 0 {
		s.learningState.WeakestSkill = s.learningState.Skills[0].Skill
	}
	total := 0.0
	for _, skillState := range s.learningState.Skills {
		total += skillState.Score
	}
	if len(s.learningState.Skills) > 0 {
		s.learningState.OverallScore = total / float64(len(s.learningState.Skills))
	}
	return nil
}

func (s *MemoryStore) SaveDiagnostic(_ context.Context, result domain.DiagnosticResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := result
	copy.SkillScores = append([]domain.SkillState(nil), result.SkillScores...)
	s.diagnostic = &copy
	return nil
}

func (s *MemoryStore) Diagnostic(context.Context) (domain.DiagnosticResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.diagnostic == nil {
		return domain.DiagnosticResult{}, ErrNotFound
	}
	copy := *s.diagnostic
	copy.SkillScores = append([]domain.SkillState(nil), s.diagnostic.SkillScores...)
	return copy, nil
}

func (s *MemoryStore) Scenarios(context.Context) ([]domain.RoleplayScenario, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.RoleplayScenario(nil), s.scenarios...), nil
}

func (s *MemoryStore) Conversation(_ context.Context, id string) (domain.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conversation, ok := s.conversations[id]
	if !ok {
		return domain.Conversation{}, ErrNotFound
	}
	conversation.Messages = append([]domain.Message(nil), conversation.Messages...)
	return conversation, nil
}

func (s *MemoryStore) SaveConversation(_ context.Context, conversation domain.Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	conversation.Messages = append([]domain.Message(nil), conversation.Messages...)
	s.conversations[conversation.ID] = conversation
	return nil
}

func (s *MemoryStore) SpeakingSession(_ context.Context, id string) (domain.SpeakingSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.speaking[id]
	if !ok {
		return domain.SpeakingSession{}, ErrNotFound
	}
	return session, nil
}

func (s *MemoryStore) AllSpeakingSessions(context.Context) ([]domain.SpeakingSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.SpeakingSession, 0, len(s.speaking))
	for _, item := range s.speaking {
		copy := item
		if item.Pronunciation != nil {
			score := *item.Pronunciation
			copy.Pronunciation = &score
		}
		items = append(items, copy)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (s *MemoryStore) SaveSpeakingSession(_ context.Context, session domain.SpeakingSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.speaking[session.ID] = session
	return nil
}

func (s *MemoryStore) Usage(_ context.Context, from time.Time) ([]domain.UsageRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.UsageRecord, 0)
	for _, record := range s.usage {
		if !record.CreatedAt.Before(from) {
			items = append(items, record)
		}
	}
	return items, nil
}

func (s *MemoryStore) SaveUsage(_ context.Context, record domain.UsageRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usage = append(s.usage, record)
	return nil
}

func (s *MemoryStore) ReserveUsage(_ context.Context, request UsageReservationRequest) (bool, error) {
	if request.ID == "" || request.UserID == "" || request.MonthStart.IsZero() ||
		request.Amount <= 0 || request.Limit <= 0 {
		return false, errors.New("invalid usage reservation")
	}
	if request.Metric != UsageMetricTokens && request.Metric != UsageMetricCost {
		return false, errors.New("invalid usage reservation metric")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.reservations[request.ID]; ok {
		return existing.status == "active" || existing.status == "committed", nil
	}
	now := time.Now().UTC()
	monthEnd := request.MonthStart.UTC().AddDate(0, 1, 0)
	used := 0.0
	for _, record := range s.usage {
		if record.CreatedAt.Before(request.MonthStart) || !record.CreatedAt.Before(monthEnd) {
			continue
		}
		if request.Feature != "" && record.Feature != request.Feature {
			continue
		}
		if request.Metric == UsageMetricTokens {
			used += float64(record.InputTokens + record.OutputTokens)
		} else {
			used += record.EstimatedCost
		}
	}
	reserved := 0.0
	for _, existing := range s.reservations {
		if existing.status != "active" ||
			!existing.request.ExpiresAt.IsZero() && !existing.request.ExpiresAt.After(now) {
			continue
		}
		if existing.request.UserID == request.UserID &&
			existing.request.MonthStart.Equal(request.MonthStart) &&
			existing.request.Feature == request.Feature &&
			existing.request.Metric == request.Metric {
			reserved += existing.request.Amount
		}
	}
	if used+reserved+request.Amount > request.Limit {
		return false, nil
	}
	s.reservations[request.ID] = memoryUsageReservation{request: request, status: "active"}
	return true, nil
}

func (s *MemoryStore) CompleteUsageReservation(_ context.Context, id string, committed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	reservation, ok := s.reservations[id]
	if !ok || reservation.status != "active" {
		return nil
	}
	if committed {
		reservation.status = "committed"
	} else {
		reservation.status = "released"
	}
	s.reservations[id] = reservation
	return nil
}

func (s *MemoryStore) DeleteUserData(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.user = domain.User{}
	s.learningState = domain.LearningState{}
	s.reservations = map[string]memoryUsageReservation{}
	s.missions = map[string]domain.Mission{}
	s.attempts = nil
	s.mistakes = map[string]domain.Mistake{}
	s.vocabulary = map[string]domain.Vocabulary{}
	s.workContexts = map[string]domain.WorkContext{}
	s.settings = domain.Settings{}
	s.progress = nil
	s.diagnostic = nil
	s.conversations = map[string]domain.Conversation{}
	s.speaking = map[string]domain.SpeakingSession{}
	s.usage = nil
	return nil
}

func updateMastery(mastery float64, success bool, score float64) float64 {
	if success {
		mastery += 0.12 + clamp(score, 0, 100)/1000
	} else {
		mastery -= 0.08
	}
	return clamp(mastery, 0, 1)
}

func reviewAt(now time.Time, mastery float64, success bool) time.Time {
	intervals := [...]time.Duration{24 * time.Hour, 3 * 24 * time.Hour, 7 * 24 * time.Hour, 14 * 24 * time.Hour, 30 * 24 * time.Hour}
	index := int(clamp(mastery, 0, 1) * float64(len(intervals)))
	if index >= len(intervals) {
		index = len(intervals) - 1
	}
	if !success && index > 0 {
		index--
	}
	return now.Add(intervals[index])
}

func defaultScenarios() []domain.RoleplayScenario {
	return []domain.RoleplayScenario{
		{ID: "standup-blocker", Title: "Explain a blocker in stand-up", Type: "standup", PartnerRole: "Engineering Manager", Level: "B1", Context: "Your API integration is blocked by an unstable provider response.", Goal: "State the impact, current evidence, and next step clearly.", Opening: "What is blocking you today?", Vocabulary: []string{"blocked by", "investigate", "ETA", "workaround"}},
		{ID: "pr-review", Title: "Respond to a PR review", Type: "code-review", PartnerRole: "Senior Engineer", Level: "B1", Context: "A reviewer asks why you chose a cache instead of a database query.", Goal: "Explain the trade-off and acknowledge a valid concern.", Opening: "Why did you choose this approach?", Vocabulary: []string{"trade-off", "latency", "maintainable", "concern"}},
		{ID: "clarify-requirement", Title: "Clarify an ambiguous requirement", Type: "requirements", PartnerRole: "Product Manager", Level: "B1", Context: "The requirement says the system should be fast but gives no target.", Goal: "Ask precise questions and propose an acceptance criterion.", Opening: "Can you implement this by Friday?", Vocabulary: []string{"acceptance criterion", "scope", "edge case", "clarify"}},
		{ID: "incident-response", Title: "Give an incident update", Type: "incident", PartnerRole: "Incident Commander", Level: "B2", Context: "A deployment caused elevated error rates for a subset of users.", Goal: "Give a calm update with facts, impact, mitigation, and follow-up.", Opening: "What do we know about the incident?", Vocabulary: []string{"rollback", "mitigation", "error rate", "root cause"}},
		{ID: "system-design-tradeoff", Title: "Lead a system design discussion", Type: "system-design", PartnerRole: "Staff Engineer", Level: "B2", Context: "You need to design a queue for bursty notification traffic.", Goal: "Explain constraints, failure handling, trade-offs and the next design decision.", Opening: "How would you design this system?", Vocabulary: []string{"throughput", "failure mode", "trade-off", "observability"}},
		{ID: "technical-interview-api", Title: "Technical interview: design an API", Type: "technical-interview", PartnerRole: "Interviewer", Level: "B2", Context: "Design an API that supports idempotent payment requests.", Goal: "Clarify requirements, propose a design and defend one trade-off.", Opening: "How would you approach this API design?", Vocabulary: []string{"idempotency", "constraint", "scalability", "trade-off"}},
	}
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
