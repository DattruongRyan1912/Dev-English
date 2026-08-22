package domain

import "time"

const (
	ModeWriting  = "writing"
	ModeSpeaking = "speaking"
	ModeReading  = "reading"
	ModeReview   = "review"
)

type User struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"displayName"`
	CEFR        string    `json:"cefr"`
	CreatedAt   time.Time `json:"createdAt"`
}

type SkillState struct {
	Skill       string  `json:"skill"`
	Label       string  `json:"label"`
	Score       float64 `json:"score"`
	Level       string  `json:"level"`
	Trend       float64 `json:"trend"`
	IsWeakest   bool    `json:"isWeakest"`
	Description string  `json:"description,omitempty"`
}

type LearningState struct {
	CEFR              string       `json:"cefr"`
	OverallScore      float64      `json:"overallScore"`
	WeakestSkill      string       `json:"weakestSkill"`
	RecentTopics      []string     `json:"recentTopics"`
	CurrentPriorities []string     `json:"currentPriorities"`
	Skills            []SkillState `json:"skills"`
}

type Mission struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	Mode             string     `json:"mode"`
	Skill            string     `json:"skill"`
	SkillLabel       string     `json:"skillLabel"`
	Level            string     `json:"level"`
	Context          string     `json:"context"`
	Prompt           string     `json:"prompt"`
	TargetVocabulary []string   `json:"targetVocabulary"`
	ExpectedPoints   []string   `json:"expectedPoints"`
	EstimatedMinutes int        `json:"estimatedMinutes"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"createdAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
}

type Mistake struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Original   string    `json:"original"`
	Corrected  string    `json:"corrected"`
	Context    string    `json:"context"`
	Severity   int       `json:"severity"`
	Frequency  int       `json:"frequency"`
	LastSeen   time.Time `json:"lastSeen"`
	NextReview time.Time `json:"nextReview"`
	Mastery    float64   `json:"mastery"`
}

type Vocabulary struct {
	ID               string    `json:"id"`
	Term             string    `json:"term"`
	Domain           string    `json:"domain"`
	Level            string    `json:"level"`
	Definition       string    `json:"definition"`
	UserContext      string    `json:"userContext"`
	TechnicalExample string    `json:"technicalExample"`
	RelatedTerms     []string  `json:"relatedTerms,omitempty"`
	CommonMistakes   []string  `json:"commonMistakes,omitempty"`
	Mastery          float64   `json:"mastery"`
	NextReview       time.Time `json:"nextReview"`
}

type MissionAttempt struct {
	ID          string    `json:"id"`
	MissionID   string    `json:"missionId"`
	Answer      string    `json:"answer"`
	Score       float64   `json:"score"`
	SubmittedAt time.Time `json:"submittedAt"`
}

type Evaluation struct {
	Score           float64      `json:"score"`
	Summary         string       `json:"summary"`
	WhatWasGood     []string     `json:"whatWasGood"`
	MainIssue       string       `json:"mainIssue"`
	NextAction      string       `json:"nextAction"`
	Corrections     []Correction `json:"corrections"`
	TechnicalPoints []string     `json:"technicalPoints"`
	Provider        string       `json:"provider"`
}

type Correction struct {
	Original  string `json:"original"`
	Corrected string `json:"corrected"`
	Why       string `json:"why"`
	Type      string `json:"type"`
	Severity  int    `json:"severity"`
}

type SubmissionResult struct {
	Attempt    MissionAttempt `json:"attempt"`
	Evaluation Evaluation     `json:"evaluation"`
	Mistakes   []Mistake      `json:"mistakes"`
	NextReview time.Time      `json:"nextReview"`
}

type WorkContext struct {
	ID         string    `json:"id"`
	SourceType string    `json:"sourceType"`
	SourceURL  string    `json:"sourceUrl,omitempty"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Domain     string    `json:"domain"`
	CreatedAt  time.Time `json:"createdAt"`
}

type GitHubImport struct {
	URL        string `json:"url"`
	SourceType string `json:"sourceType"`
	Title      string `json:"title"`
	Content    string `json:"content"`
}

type HomeSummary struct {
	User           User            `json:"user"`
	LearningState  LearningState   `json:"learningState"`
	TodayMission   Mission         `json:"todayMission"`
	ReviewDueCount int             `json:"reviewDueCount"`
	RecentProgress []ProgressPoint `json:"recentProgress"`
	Provider       ProviderStatus  `json:"provider"`
}

type ProgressPoint struct {
	Date  string  `json:"date"`
	Score float64 `json:"score"`
}

type ReviewItem struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Prompt     string    `json:"prompt"`
	Answer     string    `json:"answer"`
	Context    string    `json:"context"`
	Level      string    `json:"level"`
	NextReview time.Time `json:"nextReview"`
}

type PracticeMode struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Minutes     int    `json:"minutes"`
	Available   bool   `json:"available"`
}

type ProgressSummary struct {
	LearningState    LearningState   `json:"learningState"`
	RepeatedMistakes int             `json:"repeatedMistakes"`
	SpeakingTrend    []ProgressPoint `json:"speakingTrend"`
	TechnicalScore   float64         `json:"technicalScore"`
}

type Settings struct {
	AIProvider         string `json:"aiProvider"`
	FastModel          string `json:"fastModel"`
	SmartModel         string `json:"smartModel"`
	DeepSeekConfigured bool   `json:"deepSeekConfigured"`
	DeepSeekStatus     string `json:"deepSeekStatus"`
	SpeechConfigured   bool   `json:"speechConfigured"`
	PronunciationOn    bool   `json:"pronunciationOn"`
	MonthlyBudgetVND   int    `json:"monthlyBudgetVnd"`
}

type ProviderStatus struct {
	Name       string `json:"name"`
	Mode       string `json:"mode"`
	Configured bool   `json:"configured"`
}

type ProviderCheck struct {
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
	Reachable  bool   `json:"reachable"`
	Healthy    bool   `json:"healthy"`
	Capability string `json:"capability"`
	Model      string `json:"model"`
	LatencyMs  int64  `json:"latencyMs"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

type WorkImportResult struct {
	Context          WorkContext `json:"context"`
	SuggestedSkills  []string    `json:"suggestedSkills"`
	SuggestedTerms   []string    `json:"suggestedTerms"`
	SuggestedMission Mission     `json:"suggestedMission"`
}

type DiagnosticQuestion struct {
	ID      string   `json:"id"`
	Skill   string   `json:"skill"`
	Mode    string   `json:"mode"`
	Prompt  string   `json:"prompt"`
	Options []string `json:"options,omitempty"`
}

type DiagnosticResponse struct {
	QuestionID string `json:"questionId"`
	Answer     string `json:"answer"`
}

type DiagnosticResult struct {
	CEFR            string       `json:"cefr"`
	OverallScore    float64      `json:"overallScore"`
	SkillScores     []SkillState `json:"skillScores"`
	Strengths       []string     `json:"strengths"`
	Priorities      []string     `json:"priorities"`
	RecommendedPlan []string     `json:"recommendedPlan"`
}

type RoleplayScenario struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	PartnerRole string   `json:"partnerRole"`
	Level       string   `json:"level"`
	Context     string   `json:"context"`
	Goal        string   `json:"goal"`
	Opening     string   `json:"opening"`
	Vocabulary  []string `json:"vocabulary"`
}

type Conversation struct {
	ID           string    `json:"id"`
	RoleplayType string    `json:"roleplayType"`
	Context      string    `json:"context"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	Messages     []Message `json:"messages"`
}

type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Feedback  string    `json:"feedback,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type RoleplayTurnRequest struct {
	ConversationID string `json:"conversationId"`
	Answer         string `json:"answer"`
}

type RoleplayTurnResult struct {
	Conversation Conversation `json:"conversation"`
	Reply        Message      `json:"reply"`
	Feedback     Evaluation   `json:"feedback"`
}

type CopilotRequest struct {
	Vietnamese string `json:"vietnamese"`
	Context    string `json:"context"`
}

type CopilotResult struct {
	Simple       string `json:"simple"`
	Natural      string `json:"natural"`
	Professional string `json:"professional"`
	Explanation  string `json:"explanation"`
}

type SpeakingSession struct {
	ID                string              `json:"id"`
	MissionID         string              `json:"missionId,omitempty"`
	Status            string              `json:"status"`
	Transcript        string              `json:"transcript"`
	Pronunciation     *PronunciationScore `json:"pronunciation,omitempty"`
	CreatedAt         time.Time           `json:"createdAt"`
	RawAudioExpiresAt *time.Time          `json:"rawAudioExpiresAt,omitempty"`
}

type PronunciationScore struct {
	Score        float64            `json:"score"`
	Accuracy     float64            `json:"accuracy"`
	Fluency      float64            `json:"fluency"`
	Completeness float64            `json:"completeness"`
	Prosody      float64            `json:"prosody"`
	Words        map[string]float64 `json:"words,omitempty"`
}

type UsageRecord struct {
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	Feature       string    `json:"feature"`
	InputTokens   int       `json:"inputTokens"`
	OutputTokens  int       `json:"outputTokens"`
	AudioSeconds  float64   `json:"audioSeconds"`
	TTSCharacters int       `json:"ttsCharacters"`
	EstimatedCost float64   `json:"estimatedCost"`
	CreatedAt     time.Time `json:"createdAt"`
}

type UsageSummary struct {
	Month             string        `json:"month"`
	EstimatedCost     float64       `json:"estimatedCost"`
	BudgetVND         int           `json:"budgetVnd"`
	BudgetUsedPercent float64       `json:"budgetUsedPercent"`
	Records           []UsageRecord `json:"records"`
}

type VocabularyGraph struct {
	Nodes []VocabularyGraphNode `json:"nodes"`
	Edges []VocabularyGraphEdge `json:"edges"`
}

type VocabularyGraphNode struct {
	ID      string  `json:"id"`
	Label   string  `json:"label"`
	Mastery float64 `json:"mastery"`
}

type VocabularyGraphEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"`
}

type WeeklySpeakingAssessment struct {
	Week           string  `json:"week"`
	Sessions       int     `json:"sessions"`
	Evaluated      int     `json:"evaluated"`
	AverageScore   float64 `json:"averageScore"`
	AverageFluency float64 `json:"averageFluency"`
	AverageProsody float64 `json:"averageProsody"`
	Recommendation string  `json:"recommendation"`
}

type AnalyticsSummary struct {
	WindowDays                  int                      `json:"windowDays"`
	MinutesLearned              int                      `json:"minutesLearned"`
	MissionsCompleted           int                      `json:"missionsCompleted"`
	MissionsCreated             int                      `json:"missionsCreated"`
	RepeatedMistakes            int                      `json:"repeatedMistakes"`
	TechnicalCommunicationScore float64                  `json:"technicalCommunicationScore"`
	VocabularyMastery           float64                  `json:"vocabularyMastery"`
	SpeakingTrend               []ProgressPoint          `json:"speakingTrend"`
	WeeklySpeaking              WeeklySpeakingAssessment `json:"weeklySpeaking"`
}
