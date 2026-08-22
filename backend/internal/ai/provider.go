package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
)

var ErrProviderUnavailable = errors.New("ai provider unavailable")

type MissionRequest struct {
	LearningState domain.LearningState
	WorkContext   string
}

type WritingRequest struct {
	Mission domain.Mission
	Answer  string
}

type RoleplayRequest struct {
	Scenario     domain.RoleplayScenario
	Conversation domain.Conversation
	Answer       string
}

type RoleplayResult struct {
	Reply      string
	Evaluation domain.Evaluation
}

type CopilotRequest struct {
	Vietnamese string
	Context    string
}

type LLMProvider interface {
	Name() string
	Configured() bool
	GenerateMission(context.Context, MissionRequest) (domain.Mission, error)
	EvaluateWriting(context.Context, WritingRequest) (domain.Evaluation, error)
}

// Provider is kept as a concise alias for the learning service dependency.
type Provider = LLMProvider

// HealthChecker is implemented by providers that can perform a cheap,
// authenticated connectivity probe without consuming a generation quota.
type HealthChecker interface {
	HealthCheck(context.Context) error
}

type Transcript struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

type PronunciationResult struct {
	Accuracy     float64            `json:"accuracy"`
	Fluency      float64            `json:"fluency"`
	Completeness float64            `json:"completeness"`
	Prosody      float64            `json:"prosody"`
	Words        map[string]float64 `json:"words,omitempty"`
}

// These interfaces keep speech integrations replaceable. V1 can continue with
// text-only feedback when the corresponding provider is not configured.
type STTProvider interface {
	Name() string
	Configured() bool
	Transcribe(context.Context, []byte, string) (Transcript, error)
}

type TTSProvider interface {
	Name() string
	Configured() bool
	Synthesize(context.Context, string, string) ([]byte, error)
}

type PronunciationProvider interface {
	Name() string
	Configured() bool
	Assess(context.Context, []byte, string, string) (PronunciationResult, error)
}

type RoleplayProvider interface {
	GenerateRoleplay(context.Context, RoleplayRequest) (RoleplayResult, error)
}

type CopilotProvider interface {
	GenerateCopilot(context.Context, CopilotRequest) (domain.CopilotResult, error)
}

type UnavailableSpeechProvider struct{ ProviderName string }

func (p UnavailableSpeechProvider) Name() string     { return p.ProviderName }
func (p UnavailableSpeechProvider) Configured() bool { return false }
func (p UnavailableSpeechProvider) Transcribe(context.Context, []byte, string) (Transcript, error) {
	return Transcript{}, ErrProviderUnavailable
}
func (p UnavailableSpeechProvider) Synthesize(context.Context, string, string) ([]byte, error) {
	return nil, ErrProviderUnavailable
}
func (p UnavailableSpeechProvider) Assess(context.Context, []byte, string, string) (PronunciationResult, error) {
	return PronunciationResult{}, ErrProviderUnavailable
}

type DeterministicProvider struct{}

func (DeterministicProvider) Name() string     { return "deterministic-fallback" }
func (DeterministicProvider) Configured() bool { return true }

func (DeterministicProvider) GenerateMission(_ context.Context, request MissionRequest) (domain.Mission, error) {
	contextText := strings.TrimSpace(request.WorkContext)
	if contextText == "" {
		contextText = "an API error from your current project"
	}
	tweak := request.LearningState.WeakestSkill
	if tweak == "" {
		tweak = "technical_writing"
	}
	return domain.Mission{
		ID:               "mission-generated-" + fmt.Sprintf("%d", time.Now().UTC().UnixNano()),
		Title:            "Explain a technical problem clearly",
		Mode:             domain.ModeWriting,
		Skill:            tweak,
		SkillLabel:       skillLabel(tweak),
		Level:            request.LearningState.CEFR,
		Context:          contextText,
		Prompt:           "Write a concise technical explanation. State what happened, why it matters, and what you would do next.",
		TargetVocabulary: []string{"observed behavior", "root cause", "impact", "next step"},
		ExpectedPoints:   []string{"observed behavior", "technical reason", "impact", "next step"},
		EstimatedMinutes: 10,
		Status:           "available",
		CreatedAt:        time.Now().UTC(),
	}, nil
}

func (p DeterministicProvider) EvaluateWriting(_ context.Context, request WritingRequest) (domain.Evaluation, error) {
	return EvaluateWritingDeterministically(request.Mission, request.Answer), nil
}

func (p DeterministicProvider) GenerateRoleplay(_ context.Context, request RoleplayRequest) (RoleplayResult, error) {
	answer := strings.TrimSpace(request.Answer)
	mission := domain.Mission{Skill: "meeting", SkillLabel: "Technical Discussion", Level: request.Scenario.Level, Context: request.Scenario.Context, Prompt: request.Scenario.Goal, ExpectedPoints: []string{"reason", "impact", "next step"}}
	evaluation := EvaluateWritingDeterministically(mission, answer)
	reply := "That makes sense. Can you explain the evidence and the next step you would take?"
	lower := strings.ToLower(answer)
	switch {
	case strings.Contains(lower, "blocked") || strings.Contains(lower, "error"):
		reply = "What is the user impact, and do you have a safe workaround while we investigate?"
	case strings.Contains(lower, "because") || strings.Contains(lower, "root cause"):
		reply = "How would you validate that root cause before changing the system?"
	case strings.Contains(lower, "next") || strings.Contains(lower, "fix"):
		reply = "Good. How will you communicate the result and prevent the same issue from recurring?"
	}
	return RoleplayResult{Reply: reply, Evaluation: evaluation}, nil
}

func (p DeterministicProvider) GenerateCopilot(_ context.Context, request CopilotRequest) (domain.CopilotResult, error) {
	text := strings.TrimSpace(request.Vietnamese)
	if text == "" {
		return domain.CopilotResult{}, errors.New("vietnamese text is required")
	}
	action := translateIntent(text)
	return domain.CopilotResult{
		Simple:       "Please " + action + ".",
		Natural:      "Could you please " + action + "?",
		Professional: "Could you please " + professionalIntent(action) + "?",
		Explanation:  "The three options move from direct wording to a more natural and professional developer-facing request while preserving the technical intent.",
	}, nil
}

func translateIntent(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	replacements := []struct{ from, to string }{
		{"nhờ kiểm tra lại api", "check the API again"},
		{"kiểm tra lại api", "check the API again"},
		{"nhờ kiểm tra lại", "check it again"},
		{"giúp tôi kiểm tra", "help me check"},
		{"cập nhật trạng thái", "update the status"},
		{"giải thích nguyên nhân", "explain the root cause"},
		{"xác nhận lỗi", "confirm the bug"},
	}
	for _, replacement := range replacements {
		if strings.Contains(lower, replacement.from) {
			return replacement.to
		}
	}
	return value
}

func professionalIntent(action string) string {
	if strings.Contains(strings.ToLower(action), "check the api") {
		return "review the API again and confirm the expected behavior"
	}
	return action + " and confirm the expected outcome"
}

func skillLabel(skill string) string {
	labels := map[string]string{
		"documentation":     "Documentation",
		"technical_writing": "Technical Writing",
		"git_communication": "Git Communication",
		"speaking":          "Speaking",
		"meeting":           "Meeting",
		"system_design":     "System Design",
		"interview":         "Technical Interview",
	}
	if label, ok := labels[skill]; ok {
		return label
	}
	return "Technical English"
}

// EvaluateWritingDeterministically keeps the V1 feedback path useful without an API key.
// It intentionally returns observations, while the learning service owns score updates.
func EvaluateWritingDeterministically(mission domain.Mission, answer string) domain.Evaluation {
	answer = strings.TrimSpace(answer)
	lower := strings.ToLower(answer)
	evaluation := domain.Evaluation{Provider: "deterministic-fallback"}
	if answer == "" {
		evaluation.Summary = "Add a short technical answer before checking it."
		evaluation.MainIssue = "There is no answer to evaluate yet."
		evaluation.NextAction = "Write two or three sentences covering the observed behavior and next step."
		return evaluation
	}
	wordCount := len(strings.Fields(answer))
	score := 35.0
	if wordCount >= 20 {
		score += 15
	}
	if wordCount >= 45 {
		score += 10
	}
	good := make([]string, 0, 3)
	for _, point := range mission.ExpectedPoints {
		if containsConcept(lower, point) {
			score += 8
			good = append(good, "You covered the "+point+".")
		}
	}
	if len(good) == 0 {
		good = append(good, "You started with a concrete technical situation.")
	}
	corrections := detectCorrections(answer)
	for _, correction := range corrections {
		score -= float64(correction.Severity * 4)
	}
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	evaluation.Score = score
	evaluation.WhatWasGood = good
	evaluation.Corrections = corrections
	evaluation.TechnicalPoints = expectedPointsPresent(mission, lower)
	switch {
	case score >= 80:
		evaluation.Summary = "Good technical communication. Your explanation is useful to a teammate."
		evaluation.MainIssue = "Make the next step even more specific if the team needs to act quickly."
	case score >= 60:
		evaluation.Summary = "The core idea is understandable, but the explanation can be more complete."
		evaluation.MainIssue = "Add the missing technical evidence and connect it to the impact."
	default:
		evaluation.Summary = "You have a starting point; now make the technical message easier to act on."
		evaluation.MainIssue = "The answer needs a clearer structure: observed behavior, reason, impact, next step."
	}
	if len(corrections) > 0 {
		evaluation.NextAction = "Retry the answer and apply the first correction before adding more detail."
	} else {
		evaluation.NextAction = "Retry once with one specific technical detail and a clear next action."
	}
	return evaluation
}

func containsConcept(answer, concept string) bool {
	concept = strings.ToLower(strings.TrimSpace(concept))
	switch concept {
	case "observed behavior":
		return strings.Contains(answer, "observed") || strings.Contains(answer, "happened") || strings.Contains(answer, "returns") || strings.Contains(answer, "error")
	case "expected behavior":
		return strings.Contains(answer, "expected") || strings.Contains(answer, "should")
	case "technical reason", "root cause":
		return strings.Contains(answer, "because") || strings.Contains(answer, "root cause") || strings.Contains(answer, "caused")
	case "impact":
		return strings.Contains(answer, "impact") || strings.Contains(answer, "affect") || strings.Contains(answer, "user")
	case "next step":
		return strings.Contains(answer, "next") || strings.Contains(answer, "fix") || strings.Contains(answer, "should") || strings.Contains(answer, "investigate")
	default:
		return strings.Contains(answer, concept)
	}
}

func expectedPointsPresent(mission domain.Mission, answer string) []string {
	result := make([]string, 0, len(mission.ExpectedPoints))
	for _, point := range mission.ExpectedPoints {
		if containsConcept(answer, point) {
			result = append(result, point)
		}
	}
	return result
}

func detectCorrections(answer string) []domain.Correction {
	type rule struct {
		bad, good, kind string
		severity        int
		why             string
	}
	rules := []rule{
		{"i has", "I have", "grammar", 2, "Use have with I."},
		{"he go", "he goes", "grammar", 2, "Use the third-person singular form in the present simple."},
		{"discuss about", "discuss", "collocation", 2, "Discuss is transitive; do not add about."},
		{"informations", "information", "noun form", 1, "Information is uncountable in this context."},
		{"didn't went", "didn't go", "grammar", 2, "After did not, use the base verb."},
		{"explain about", "explain", "collocation", 1, "Explain takes a direct object in this context."},
	}
	result := make([]domain.Correction, 0, 3)
	lower := strings.ToLower(answer)
	for _, item := range rules {
		if strings.Contains(lower, item.bad) {
			result = append(result, domain.Correction{Original: item.bad, Corrected: item.good, Why: item.why, Type: item.kind, Severity: item.severity})
		}
	}
	if len(result) > 3 {
		result = result[:3]
	}
	return result
}

type DeepSeekProvider struct {
	APIKey     string
	BaseURL    string
	FastModel  string
	SmartModel string
	Client     *http.Client
}

func NewDeepSeekFromEnv() *DeepSeekProvider {
	baseURL := strings.TrimRight(os.Getenv("DEEPSEEK_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	fast := os.Getenv("DEEPSEEK_FAST_MODEL")
	if fast == "" {
		fast = "deepseek-v4-flash"
	}
	smart := os.Getenv("DEEPSEEK_SMART_MODEL")
	if smart == "" {
		smart = "deepseek-v4-pro"
	}
	return &DeepSeekProvider{APIKey: os.Getenv("DEEPSEEK_API_KEY"), BaseURL: baseURL, FastModel: fast, SmartModel: smart, Client: &http.Client{Timeout: 30 * time.Second}}
}

func (p *DeepSeekProvider) Name() string     { return "deepseek" }
func (p *DeepSeekProvider) Configured() bool { return strings.TrimSpace(p.APIKey) != "" }

func (p *DeepSeekProvider) HealthCheck(ctx context.Context) error {
	if !p.Configured() {
		return ErrProviderUnavailable
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(p.BaseURL, "/")+"/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providerHTTPError("deepseek health check", resp)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return nil
}

func (p *DeepSeekProvider) GenerateMission(ctx context.Context, request MissionRequest) (domain.Mission, error) {
	if !p.Configured() {
		return domain.Mission{}, ErrProviderUnavailable
	}
	system := "You are DevEnglish's mission generator. Return only valid JSON with title, mode, skill, skillLabel, level, context, prompt, targetVocabulary, expectedPoints, estimatedMinutes. Keep the mission practical for a developer."
	user := fmt.Sprintf("Learning state: %+v\nWork context: %s", request.LearningState, request.WorkContext)
	var result domain.Mission
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		result = domain.Mission{}
		err = p.chatJSON(ctx, p.FastModel, system, user, &result)
		if err == nil {
			err = validateMission(result)
		}
		if err == nil {
			break
		}
	}
	if err != nil {
		return domain.Mission{}, fmt.Errorf("validate generated mission: %w", err)
	}
	result.ID = "mission-ai-" + fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	result.Status = "available"
	result.CreatedAt = time.Now().UTC()
	return result, nil
}

func (p *DeepSeekProvider) EvaluateWriting(ctx context.Context, request WritingRequest) (domain.Evaluation, error) {
	if !p.Configured() {
		return domain.Evaluation{}, ErrProviderUnavailable
	}
	system := "You are a technical English evaluator. Return only valid JSON with score, summary, whatWasGood, mainIssue, nextAction, corrections, technicalPoints. Corrections must contain original, corrected, why, type, severity. Score is an observation only; do not invent missing evidence."
	user := fmt.Sprintf("Mission: %+v\nLearner answer: %s", request.Mission, request.Answer)
	var result domain.Evaluation
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		result = domain.Evaluation{}
		err = p.chatJSON(ctx, p.SmartModel, system, user, &result)
		if err == nil {
			err = validateEvaluation(result)
		}
		if err == nil {
			break
		}
	}
	if err != nil {
		return domain.Evaluation{}, fmt.Errorf("validate generated evaluation: %w", err)
	}
	result.Provider = p.Name()
	return result, nil
}

func (p *DeepSeekProvider) GenerateRoleplay(ctx context.Context, request RoleplayRequest) (RoleplayResult, error) {
	if !p.Configured() {
		return RoleplayResult{}, ErrProviderUnavailable
	}
	system := "You are a technical English roleplay partner. Return only JSON with reply and evaluation. The reply must ask one focused follow-up question. The evaluation must contain score, summary, whatWasGood, mainIssue, nextAction, corrections, technicalPoints."
	user := fmt.Sprintf("Scenario: %+v\nConversation: %+v\nLearner answer: %s", request.Scenario, request.Conversation, request.Answer)
	var output struct {
		Reply      string            `json:"reply"`
		Evaluation domain.Evaluation `json:"evaluation"`
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		output = struct {
			Reply      string            `json:"reply"`
			Evaluation domain.Evaluation `json:"evaluation"`
		}{}
		lastErr = p.chatJSON(ctx, p.SmartModel, system, user, &output)
		if lastErr == nil && strings.TrimSpace(output.Reply) == "" {
			lastErr = errors.New("roleplay provider returned an empty reply")
		}
		if lastErr == nil && !strings.Contains(output.Reply, "?") {
			lastErr = errors.New("roleplay provider must ask a focused follow-up question")
		}
		if lastErr == nil {
			lastErr = validateEvaluation(output.Evaluation)
		}
		if lastErr == nil {
			output.Evaluation.Provider = p.Name()
			return RoleplayResult{Reply: output.Reply, Evaluation: output.Evaluation}, nil
		}
	}
	return RoleplayResult{}, fmt.Errorf("validate generated roleplay: %w", lastErr)
}

func (p *DeepSeekProvider) GenerateCopilot(ctx context.Context, request CopilotRequest) (domain.CopilotResult, error) {
	if !p.Configured() {
		return domain.CopilotResult{}, ErrProviderUnavailable
	}
	system := "You are a technical English copilot for developers. Return only JSON with simple, natural, professional and explanation. Preserve the requested meaning and do not invent technical facts."
	user := fmt.Sprintf("Vietnamese request: %s\nWork context: %s", request.Vietnamese, request.Context)
	var output domain.CopilotResult
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		output = domain.CopilotResult{}
		lastErr = p.chatJSON(ctx, p.FastModel, system, user, &output)
		if lastErr == nil && (strings.TrimSpace(output.Simple) == "" || strings.TrimSpace(output.Natural) == "" || strings.TrimSpace(output.Professional) == "") {
			lastErr = errors.New("copilot provider returned incomplete output")
		}
		if lastErr == nil {
			return output, nil
		}
	}
	return domain.CopilotResult{}, fmt.Errorf("validate generated copilot: %w", lastErr)
}

func (p *DeepSeekProvider) chatJSON(ctx context.Context, model, system, user string, output any) error {
	body := map[string]any{"model": model, "temperature": 0.2, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}}}
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("deepseek returned %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if len(envelope.Choices) == 0 || strings.TrimSpace(envelope.Choices[0].Message.Content) == "" {
		return errors.New("deepseek returned empty content")
	}
	content := strings.TrimSpace(envelope.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return json.Unmarshal([]byte(strings.TrimSpace(content)), output)
}

type FallbackProvider struct {
	Primary       Provider
	Fallback      DeterministicProvider
	AllowFallback bool
}

func (p FallbackProvider) Name() string {
	if p.Primary != nil && (p.Primary.Configured() || !p.AllowFallback) {
		return p.Primary.Name()
	}
	return p.Fallback.Name()
}
func (p FallbackProvider) Configured() bool { return p.Primary != nil && p.Primary.Configured() }
func (p FallbackProvider) HealthCheck(ctx context.Context) error {
	if p.Primary == nil || !p.Primary.Configured() {
		return ErrProviderUnavailable
	}
	checker, ok := p.Primary.(HealthChecker)
	if !ok {
		return errors.New("primary provider does not support health checks")
	}
	return checker.HealthCheck(ctx)
}

func (p FallbackProvider) fallbackEnabled() bool { return p.AllowFallback }

func (p FallbackProvider) GenerateMission(ctx context.Context, request MissionRequest) (domain.Mission, error) {
	if p.Primary != nil && p.Primary.Configured() {
		if mission, err := p.Primary.GenerateMission(ctx, request); err == nil {
			return mission, nil
		} else if !p.fallbackEnabled() {
			return domain.Mission{}, fmt.Errorf("%w: primary provider request failed", ErrProviderUnavailable)
		}
	}
	if !p.fallbackEnabled() {
		return domain.Mission{}, ErrProviderUnavailable
	}
	return p.Fallback.GenerateMission(ctx, request)
}
func (p FallbackProvider) EvaluateWriting(ctx context.Context, request WritingRequest) (domain.Evaluation, error) {
	if p.Primary != nil && p.Primary.Configured() {
		if result, err := p.Primary.EvaluateWriting(ctx, request); err == nil {
			return result, nil
		} else if !p.fallbackEnabled() {
			return domain.Evaluation{}, fmt.Errorf("%w: primary provider request failed", ErrProviderUnavailable)
		}
	}
	if !p.fallbackEnabled() {
		return domain.Evaluation{}, ErrProviderUnavailable
	}
	return p.Fallback.EvaluateWriting(ctx, request)
}

func (p FallbackProvider) GenerateRoleplay(ctx context.Context, request RoleplayRequest) (RoleplayResult, error) {
	if provider, ok := p.Primary.(RoleplayProvider); ok && p.Primary.Configured() {
		if result, err := provider.GenerateRoleplay(ctx, request); err == nil {
			return result, nil
		} else if !p.fallbackEnabled() {
			return RoleplayResult{}, fmt.Errorf("%w: primary provider request failed", ErrProviderUnavailable)
		}
	}
	if !p.fallbackEnabled() {
		return RoleplayResult{}, ErrProviderUnavailable
	}
	return p.Fallback.GenerateRoleplay(ctx, request)
}

func (p FallbackProvider) GenerateCopilot(ctx context.Context, request CopilotRequest) (domain.CopilotResult, error) {
	if provider, ok := p.Primary.(CopilotProvider); ok && p.Primary.Configured() {
		if result, err := provider.GenerateCopilot(ctx, request); err == nil {
			return result, nil
		} else if !p.fallbackEnabled() {
			return domain.CopilotResult{}, fmt.Errorf("%w: primary provider request failed", ErrProviderUnavailable)
		}
	}
	if !p.fallbackEnabled() {
		return domain.CopilotResult{}, ErrProviderUnavailable
	}
	return p.Fallback.GenerateCopilot(ctx, request)
}
