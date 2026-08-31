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
	"sync"
	"time"
	"unicode"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
)

var ErrProviderUnavailable = errors.New("ai provider unavailable")

const (
	maxProviderJSONAttempts = 3
	providerRetryBaseDelay  = 50 * time.Millisecond
)

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
	Reply        string
	Evaluation   domain.Evaluation
	GuidanceOnly bool
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

// JSONUsage is the provider-reported accounting payload for one structured
// generation. Available is false when the upstream response did not include
// usage; callers must not silently turn an unavailable value into an estimate.
type JSONUsage struct {
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	TotalTokens  int    `json:"totalTokens"`
	Available    bool   `json:"available"`
	Model        string `json:"-"`
}

// JSONGenerator is the narrow provider boundary used by the grounded
// assistant. The application owns model selection and output validation; the
// provider only returns JSON plus the usage metadata it actually received.
type JSONGenerator interface {
	GenerateJSON(context.Context, string, string, string, any) (JSONUsage, error)
}

// These optional interfaces let legacy learning features persist provider-
// reported usage without estimating tokens from word counts.
type UsageAwareMissionProvider interface {
	GenerateMissionWithUsage(context.Context, MissionRequest) (domain.Mission, JSONUsage, error)
}

type UsageAwareWritingProvider interface {
	EvaluateWritingWithUsage(context.Context, WritingRequest) (domain.Evaluation, JSONUsage, error)
}

type UsageAwareRoleplayProvider interface {
	GenerateRoleplayWithUsage(context.Context, RoleplayRequest) (RoleplayResult, JSONUsage, error)
}

type UsageAwareCopilotProvider interface {
	GenerateCopilotWithUsage(context.Context, CopilotRequest) (domain.CopilotResult, JSONUsage, error)
}

// HealthChecker is implemented by providers that can perform a cheap,
// authenticated connectivity probe without consuming a generation quota.
type HealthChecker interface {
	HealthCheck(context.Context) error
}

// CapabilityProbe reports safe, capability-specific connectivity metadata.
// Implementations must never include provider response bodies or credentials
// in the returned error code.
type CapabilityProbe interface {
	ProbeCapability(context.Context, string) domain.ProviderCheck
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
	guidanceOnly := isAdaptiveRoleplayAnswer(answer)
	if guidanceOnly {
		return adaptiveRoleplayResult(request.Scenario), nil
	}
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
	return RoleplayResult{Reply: reply, Evaluation: evaluation, GuidanceOnly: guidanceOnly}, nil
}

func isAdaptiveRoleplayAnswer(answer string) bool {
	return containsVietnameseText(answer) || containsRoleplayHelpRequest(answer)
}

func containsVietnameseText(value string) bool {
	const vietnameseDiacritics = "àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđ"
	for _, char := range strings.ToLower(value) {
		if strings.ContainsRune(vietnameseDiacritics, char) {
			return true
		}
	}
	normalized := normalizeRoleplayText(value)
	markers := map[string]struct{}{
		"anh": {}, "ban": {}, "biet": {}, "cach": {}, "can": {},
		"chua": {}, "duoc": {}, "em": {}, "giai": {}, "giup": {},
		"goi": {}, "huong": {}, "khong": {}, "loi": {}, "minh": {},
		"muon": {}, "nao": {}, "noi": {}, "the": {}, "thich": {},
		"tieng": {}, "toi": {}, "tra": {}, "tro": {}, "xin": {}, "y": {},
	}
	strongMarkers := map[string]struct{}{
		"biet": {}, "chua": {}, "duoc": {}, "giai": {}, "giup": {},
		"huong": {}, "khong": {}, "loi": {}, "minh": {}, "muon": {},
		"thich": {}, "tieng": {}, "toi": {}, "tra": {}, "tro": {}, "xin": {},
	}
	count := 0
	strongCount := 0
	for _, word := range strings.Fields(normalized) {
		if _, ok := markers[word]; ok {
			count++
		}
		if _, ok := strongMarkers[word]; ok {
			strongCount++
		}
	}
	return count >= 3 || strongCount >= 1 && count >= 2
}

func containsRoleplayHelpRequest(value string) bool {
	normalized := normalizeRoleplayText(value)
	for _, phrase := range []string{
		"help me", "please help", "need help", "can you help", "could you help",
		"guidance", "guide me", "give me a hint", "what should i say",
		"what can i say", "how should i answer", "how do i answer", "how can i answer",
		"can you explain", "could you explain", "please explain", "i am stuck", "i m stuck",
		"not sure how", "do not know how", "dont know how", "don t know how",
	} {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	return false
}

func normalizeRoleplayText(value string) string {
	return strings.Map(func(char rune) rune {
		if unicode.IsLetter(char) || unicode.IsSpace(char) {
			return unicode.ToLower(char)
		}
		return ' '
	}, value)
}

func adaptiveRoleplayResult(scenario domain.RoleplayScenario) RoleplayResult {
	explanation := "Câu hỏi yêu cầu bạn nêu ý chính và thêm một chi tiết kỹ thuật."
	starter := "I think we should start with the main issue."
	question := "What happened first?"
	switch strings.ToLower(strings.TrimSpace(scenario.Type)) {
	case "technical-interview":
		explanation = "Câu hỏi yêu cầu bạn nêu cách tiếp cận đầu tiên khi thiết kế API và làm rõ các yêu cầu."
		starter = "I would start by clarifying the requirements."
		question = "What should the solution do?"
	case "system-design":
		explanation = "Câu hỏi yêu cầu bạn xác định ràng buộc chính trước khi chọn thiết kế."
		starter = "I would start with the main system constraint."
		question = "What must the system handle?"
	}
	return RoleplayResult{
		Reply: explanation + "\n" + starter + "\n" + question,
		Evaluation: domain.Evaluation{
			Summary:    "Bạn chưa cung cấp câu trả lời kỹ thuật bằng tiếng Anh.",
			MainIssue:  "Chưa có câu trả lời tiếng Anh để đánh giá.",
			NextAction: "Reuse the starter sentence and add one detail about the situation.",
			Provider:   "deterministic-fallback",
		},
		GuidanceOnly: true,
	}
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
	configMu   sync.RWMutex
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
func (p *DeepSeekProvider) Configured() bool { return strings.TrimSpace(p.apiKey()) != "" }

func (p *DeepSeekProvider) apiKey() string {
	p.configMu.RLock()
	defer p.configMu.RUnlock()
	return p.APIKey
}

func (p *DeepSeekProvider) models() (string, string) {
	p.configMu.RLock()
	defer p.configMu.RUnlock()
	return p.FastModel, p.SmartModel
}

// FastModelName and SmartModelName expose the configured routing names to the
// application layer without exposing the provider's mutable configuration.
func (p *DeepSeekProvider) FastModelName() string {
	fast, _ := p.models()
	return strings.TrimSpace(fast)
}

func (p *DeepSeekProvider) SmartModelName() string {
	_, smart := p.models()
	return strings.TrimSpace(smart)
}

func (p *DeepSeekProvider) SetAPIKey(value string) {
	p.configMu.Lock()
	p.APIKey = strings.TrimSpace(value)
	p.configMu.Unlock()
}

func (p *DeepSeekProvider) ClearAPIKey() {
	p.SetAPIKey("")
}

func (p *DeepSeekProvider) SetModels(fast, smart string) error {
	if err := ValidateModelName(fast); err != nil {
		return fmt.Errorf("invalid DeepSeek fast model: %w", err)
	}
	if err := ValidateModelName(smart); err != nil {
		return fmt.Errorf("invalid DeepSeek smart model: %w", err)
	}
	p.configMu.Lock()
	p.FastModel = strings.TrimSpace(fast)
	p.SmartModel = strings.TrimSpace(smart)
	p.configMu.Unlock()
	return nil
}

func ValidateModelName(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("model is required")
	}
	if len(value) > 128 || strings.ContainsAny(value, "\r\n\t ") {
		return errors.New("model contains invalid characters")
	}
	for index, char := range value {
		if !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') && char != '.' && char != '_' && char != '-' && char != ':' {
			return errors.New("model contains invalid characters")
		}
		if index == 0 && (char == '.' || char == '_' || char == '-' || char == ':') {
			return errors.New("model must start with a letter or number")
		}
	}
	return nil
}

func (p *DeepSeekProvider) HealthCheck(ctx context.Context) error {
	apiKey := p.apiKey()
	if strings.TrimSpace(apiKey) == "" {
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
	req.Header.Set("Authorization", "Bearer "+apiKey)
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

func (p *DeepSeekProvider) ProbeCapability(ctx context.Context, capability string) domain.ProviderCheck {
	fastModel, smartModel := p.models()
	check := startProbe("DeepSeek", capability, "fast="+fastModel+",smart="+smartModel, p.Configured())
	if !check.Configured {
		return check
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(p.BaseURL, "/")+"/models", nil)
	if err != nil {
		return finishProbeError(check, err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey())
	check, resp := doProbeRequest(check, client, req)
	if resp == nil || !check.Healthy {
		return check
	}
	defer resp.Body.Close()
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return finishProbeError(check, err)
	}
	available := make(map[string]struct{}, len(payload.Data))
	for _, item := range payload.Data {
		available[item.ID] = struct{}{}
	}
	if _, ok := available[fastModel]; !ok {
		check.Healthy = false
		check.Status = "unhealthy"
		check.Error = "fast_model_unavailable"
		return check
	}
	if _, ok := available[smartModel]; !ok {
		check.Healthy = false
		check.Status = "unhealthy"
		check.Error = "smart_model_unavailable"
	}
	return check
}

func (p *DeepSeekProvider) GenerateMission(ctx context.Context, request MissionRequest) (domain.Mission, error) {
	result, _, err := p.GenerateMissionWithUsage(ctx, request)
	return result, err
}

func (p *DeepSeekProvider) GenerateMissionWithUsage(ctx context.Context, request MissionRequest) (domain.Mission, JSONUsage, error) {
	if !p.Configured() {
		return domain.Mission{}, JSONUsage{}, ErrProviderUnavailable
	}
	fastModel, _ := p.models()
	system := "You are DevEnglish's mission generator. Return only valid JSON with title, mode, skill, skillLabel, level, context, prompt, targetVocabulary, expectedPoints, estimatedMinutes. Keep the mission practical for a developer."
	user := fmt.Sprintf("Learning state: %+v\nWork context: %s", request.LearningState, request.WorkContext)
	var result domain.Mission
	var usage JSONUsage
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		result = domain.Mission{}
		usage, err = p.GenerateJSON(ctx, fastModel, system, user, &result)
		if err == nil {
			err = validateMission(result)
		}
		if err == nil {
			break
		}
	}
	if err != nil {
		return domain.Mission{}, JSONUsage{}, fmt.Errorf("validate generated mission: %w", err)
	}
	result.ID = "mission-ai-" + fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	result.Status = "available"
	result.CreatedAt = time.Now().UTC()
	return result, usage, nil
}

func (p *DeepSeekProvider) EvaluateWriting(ctx context.Context, request WritingRequest) (domain.Evaluation, error) {
	result, _, err := p.EvaluateWritingWithUsage(ctx, request)
	return result, err
}

func (p *DeepSeekProvider) EvaluateWritingWithUsage(ctx context.Context, request WritingRequest) (domain.Evaluation, JSONUsage, error) {
	if !p.Configured() {
		return domain.Evaluation{}, JSONUsage{}, ErrProviderUnavailable
	}
	_, smartModel := p.models()
	system := "You are a technical English evaluator. Return only valid JSON with score, summary, whatWasGood, mainIssue, nextAction, corrections, technicalPoints. Corrections must contain original, corrected, why, type, severity. Score is an observation only; do not invent missing evidence."
	user := fmt.Sprintf("Mission: %+v\nLearner answer: %s", request.Mission, request.Answer)
	var result domain.Evaluation
	var usage JSONUsage
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		result = domain.Evaluation{}
		usage, err = p.GenerateJSON(ctx, smartModel, system, user, &result)
		if err == nil {
			err = validateEvaluation(result)
		}
		if err == nil {
			break
		}
	}
	if err != nil {
		return domain.Evaluation{}, JSONUsage{}, fmt.Errorf("validate generated evaluation: %w", err)
	}
	result.Provider = p.Name()
	return result, usage, nil
}

func (p *DeepSeekProvider) GenerateRoleplay(ctx context.Context, request RoleplayRequest) (RoleplayResult, error) {
	result, _, err := p.GenerateRoleplayWithUsage(ctx, request)
	return result, err
}

func (p *DeepSeekProvider) GenerateRoleplayWithUsage(ctx context.Context, request RoleplayRequest) (RoleplayResult, JSONUsage, error) {
	if !p.Configured() {
		return RoleplayResult{}, JSONUsage{}, ErrProviderUnavailable
	}
	guidanceOnly := isAdaptiveRoleplayAnswer(request.Answer)
	_, smartModel := p.models()
	system := "You are an adaptive technical English roleplay partner. Return only valid JSON with reply and evaluation. First inspect the learner answer. If it is Vietnamese or asks for help or guidance, reply with a brief Vietnamese explanation, exactly one simple English starter sentence that is immediately usable, and one short, easy follow-up question. In that adaptive case, do not pretend that the learner supplied a technical answer: the evaluation must reflect that no technical answer was supplied, and nextAction must tell the learner to reuse the starter sentence and add one detail. Otherwise, continue the roleplay normally in English and evaluate only evidence the learner actually supplied. Adapt the vocabulary, sentence complexity, starter sentence and follow-up difficulty to the scenario level. The reply must ask one focused follow-up question. The evaluation must contain score, summary, whatWasGood, mainIssue, nextAction, corrections, technicalPoints."
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
		var usage JSONUsage
		usage, lastErr = p.GenerateJSON(ctx, smartModel, system, user, &output)
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
			return RoleplayResult{Reply: output.Reply, Evaluation: output.Evaluation, GuidanceOnly: guidanceOnly}, usage, nil
		}
	}
	return RoleplayResult{}, JSONUsage{}, fmt.Errorf("validate generated roleplay: %w", lastErr)
}

func (p *DeepSeekProvider) GenerateCopilot(ctx context.Context, request CopilotRequest) (domain.CopilotResult, error) {
	result, _, err := p.GenerateCopilotWithUsage(ctx, request)
	return result, err
}

func (p *DeepSeekProvider) GenerateCopilotWithUsage(ctx context.Context, request CopilotRequest) (domain.CopilotResult, JSONUsage, error) {
	if !p.Configured() {
		return domain.CopilotResult{}, JSONUsage{}, ErrProviderUnavailable
	}
	fastModel, _ := p.models()
	system := "You are a technical English copilot for developers. Return only JSON with simple, natural, professional and explanation. Preserve the requested meaning and do not invent technical facts."
	user := fmt.Sprintf("Vietnamese request: %s\nWork context: %s", request.Vietnamese, request.Context)
	var output domain.CopilotResult
	var usage JSONUsage
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		output = domain.CopilotResult{}
		usage, lastErr = p.GenerateJSON(ctx, fastModel, system, user, &output)
		if lastErr == nil && (strings.TrimSpace(output.Simple) == "" || strings.TrimSpace(output.Natural) == "" || strings.TrimSpace(output.Professional) == "") {
			lastErr = errors.New("copilot provider returned incomplete output")
		}
		if lastErr == nil {
			return output, usage, nil
		}
	}
	return domain.CopilotResult{}, JSONUsage{}, fmt.Errorf("validate generated copilot: %w", lastErr)
}

func (p *DeepSeekProvider) GenerateJSON(ctx context.Context, model, system, user string, output any) (JSONUsage, error) {
	if !p.Configured() {
		return JSONUsage{}, ErrProviderUnavailable
	}
	if err := ValidateModelName(model); err != nil {
		return JSONUsage{}, err
	}
	body := map[string]any{"model": model, "temperature": 0.2, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}}}
	encoded, err := json.Marshal(body)
	if err != nil {
		return JSONUsage{}, err
	}
	var lastErr error
	for attempt := 1; attempt <= maxProviderJSONAttempts; attempt++ {
		usage, requestErr := p.generateJSONOnce(ctx, encoded, output)
		if requestErr == nil {
			usage.Model = model
			return usage, nil
		}
		lastErr = requestErr
		if attempt == maxProviderJSONAttempts || !connectors.IsRetryable(requestErr) {
			break
		}
		if waitErr := waitForProviderRetry(ctx, attempt); waitErr != nil {
			return JSONUsage{}, waitErr
		}
	}
	return JSONUsage{}, lastErr
}

func (p *DeepSeekProvider) generateJSONOnce(ctx context.Context, encoded []byte, output any) (JSONUsage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return JSONUsage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey())
	req.Header.Set("Content-Type", "application/json")
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return JSONUsage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return JSONUsage{}, providerHTTPError("deepseek generation", resp)
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return JSONUsage{}, err
	}
	if len(envelope.Choices) == 0 || strings.TrimSpace(envelope.Choices[0].Message.Content) == "" {
		return JSONUsage{}, errors.New("deepseek returned empty content")
	}
	content := strings.TrimSpace(envelope.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), output); err != nil {
		return JSONUsage{}, err
	}
	usage := JSONUsage{InputTokens: envelope.Usage.PromptTokens, OutputTokens: envelope.Usage.CompletionTokens, TotalTokens: envelope.Usage.TotalTokens}
	usage.Available = usage.InputTokens > 0 || usage.OutputTokens > 0 || usage.TotalTokens > 0
	return usage, nil
}

func waitForProviderRetry(ctx context.Context, attempt int) error {
	delay := providerRetryBaseDelay * time.Duration(1<<(attempt-1))
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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

func (p FallbackProvider) GenerateMissionWithUsage(ctx context.Context, request MissionRequest) (domain.Mission, JSONUsage, error) {
	if p.Primary != nil && p.Primary.Configured() {
		if provider, ok := p.Primary.(UsageAwareMissionProvider); ok {
			if result, usage, err := provider.GenerateMissionWithUsage(ctx, request); err == nil {
				return result, usage, nil
			} else if !p.fallbackEnabled() {
				return domain.Mission{}, JSONUsage{}, fmt.Errorf("%w: primary provider request failed", ErrProviderUnavailable)
			}
		} else if result, err := p.Primary.GenerateMission(ctx, request); err == nil {
			return result, JSONUsage{}, nil
		} else if !p.fallbackEnabled() {
			return domain.Mission{}, JSONUsage{}, fmt.Errorf("%w: primary provider request failed", ErrProviderUnavailable)
		}
	}
	if !p.fallbackEnabled() {
		return domain.Mission{}, JSONUsage{}, ErrProviderUnavailable
	}
	result, err := p.Fallback.GenerateMission(ctx, request)
	return result, JSONUsage{}, err
}

func (p FallbackProvider) EvaluateWritingWithUsage(ctx context.Context, request WritingRequest) (domain.Evaluation, JSONUsage, error) {
	if p.Primary != nil && p.Primary.Configured() {
		if provider, ok := p.Primary.(UsageAwareWritingProvider); ok {
			if result, usage, err := provider.EvaluateWritingWithUsage(ctx, request); err == nil {
				return result, usage, nil
			} else if !p.fallbackEnabled() {
				return domain.Evaluation{}, JSONUsage{}, fmt.Errorf("%w: primary provider request failed", ErrProviderUnavailable)
			}
		} else if result, err := p.Primary.EvaluateWriting(ctx, request); err == nil {
			return result, JSONUsage{}, nil
		} else if !p.fallbackEnabled() {
			return domain.Evaluation{}, JSONUsage{}, fmt.Errorf("%w: primary provider request failed", ErrProviderUnavailable)
		}
	}
	if !p.fallbackEnabled() {
		return domain.Evaluation{}, JSONUsage{}, ErrProviderUnavailable
	}
	result, err := p.Fallback.EvaluateWriting(ctx, request)
	return result, JSONUsage{}, err
}

func (p FallbackProvider) GenerateRoleplayWithUsage(ctx context.Context, request RoleplayRequest) (RoleplayResult, JSONUsage, error) {
	if provider, ok := p.Primary.(UsageAwareRoleplayProvider); ok && p.Primary.Configured() {
		if result, usage, err := provider.GenerateRoleplayWithUsage(ctx, request); err == nil {
			return result, usage, nil
		} else if !p.fallbackEnabled() {
			return RoleplayResult{}, JSONUsage{}, fmt.Errorf("%w: primary provider request failed", ErrProviderUnavailable)
		}
	} else if provider, ok := p.Primary.(RoleplayProvider); ok && p.Primary != nil && p.Primary.Configured() {
		if result, err := provider.GenerateRoleplay(ctx, request); err == nil {
			return result, JSONUsage{}, nil
		} else if !p.fallbackEnabled() {
			return RoleplayResult{}, JSONUsage{}, fmt.Errorf("%w: primary provider request failed", ErrProviderUnavailable)
		}
	}
	if !p.fallbackEnabled() {
		return RoleplayResult{}, JSONUsage{}, ErrProviderUnavailable
	}
	result, err := p.Fallback.GenerateRoleplay(ctx, request)
	return result, JSONUsage{}, err
}

func (p FallbackProvider) GenerateCopilotWithUsage(ctx context.Context, request CopilotRequest) (domain.CopilotResult, JSONUsage, error) {
	if provider, ok := p.Primary.(UsageAwareCopilotProvider); ok && p.Primary.Configured() {
		if result, usage, err := provider.GenerateCopilotWithUsage(ctx, request); err == nil {
			return result, usage, nil
		} else if !p.fallbackEnabled() {
			return domain.CopilotResult{}, JSONUsage{}, fmt.Errorf("%w: primary provider request failed", ErrProviderUnavailable)
		}
	} else if provider, ok := p.Primary.(CopilotProvider); ok && p.Primary != nil && p.Primary.Configured() {
		if result, err := provider.GenerateCopilot(ctx, request); err == nil {
			return result, JSONUsage{}, nil
		} else if !p.fallbackEnabled() {
			return domain.CopilotResult{}, JSONUsage{}, fmt.Errorf("%w: primary provider request failed", ErrProviderUnavailable)
		}
	}
	if !p.fallbackEnabled() {
		return domain.CopilotResult{}, JSONUsage{}, ErrProviderUnavailable
	}
	result, err := p.Fallback.GenerateCopilot(ctx, request)
	return result, JSONUsage{}, err
}
