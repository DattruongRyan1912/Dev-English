package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
)

func TestDeepSeekMissionRetriesInvalidStructuredOutput(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing provider authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		content := `{"title":"bad"}`
		if requests == 2 {
			content = `{"title":"Explain an API timeout","mode":"writing","skill":"technical_writing","skillLabel":"Technical Writing","level":"B1","context":"checkout API","prompt":"Describe the observed behavior, impact and next step.","targetVocabulary":["timeout"],"expectedPoints":["observed behavior","impact"],"estimatedMinutes":10}`
		}
		payload := map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}}
		if requests == 2 {
			payload["usage"] = map[string]int{"prompt_tokens": 11, "completion_tokens": 7, "total_tokens": 18}
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	provider := &DeepSeekProvider{APIKey: "test-key", BaseURL: server.URL, FastModel: "fast", Client: server.Client()}
	mission, usage, err := provider.GenerateMissionWithUsage(context.Background(), MissionRequest{LearningState: domain.LearningState{CEFR: "B1"}})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || mission.ID == "" || mission.Status != "available" || !usage.Available || usage.InputTokens != 11 || usage.OutputTokens != 7 || usage.TotalTokens != 18 || usage.Model != "fast" {
		t.Fatalf("expected one validation retry, finalized mission and provider usage, requests=%d mission=%+v usage=%+v", requests, mission, usage)
	}
}

func TestDeepSeekGenerateJSONRetriesTransientProviderFailure(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			http.Error(w, "temporary upstream failure", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": `{"ok":true}`}}},
			"usage":   map[string]int{"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5},
		})
	}))
	defer server.Close()

	provider := &DeepSeekProvider{APIKey: "test-key", BaseURL: server.URL, FastModel: "fast", Client: server.Client()}
	var output struct {
		OK bool `json:"ok"`
	}
	usage, err := provider.GenerateJSON(context.Background(), "fast", "system", "user", &output)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || !output.OK || !usage.Available || usage.TotalTokens != 5 {
		t.Fatalf("expected bounded transient retry and reported usage, requests=%d output=%+v usage=%+v", requests, output, usage)
	}
}

func TestDeepSeekEvaluationRetriesInvalidStructuredOutput(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		content := `{"score":101}`
		if requests == 2 {
			content = `{"score":80,"summary":"Clear explanation","whatWasGood":["Evidence"],"mainIssue":"Add the impact","nextAction":"State the next step","corrections":[],"technicalPoints":["timeout"]}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer server.Close()

	provider := &DeepSeekProvider{APIKey: "test-key", BaseURL: server.URL, SmartModel: "smart", Client: server.Client()}
	evaluation, err := provider.EvaluateWriting(context.Background(), WritingRequest{Mission: domain.Mission{Prompt: "Explain the issue"}, Answer: "The API timed out."})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || evaluation.Provider != "deepseek" || evaluation.Score != 80 {
		t.Fatalf("expected one validation retry and provider metadata, requests=%d evaluation=%+v", requests, evaluation)
	}
}

func TestDeepSeekRoleplayPromptRequiresAdaptiveBilingualHelp(t *testing.T) {
	var systemPrompt string
	var userPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected DeepSeek request: %s %s", r.Method, r.URL.Path)
		}
		var request struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		for _, message := range request.Messages {
			switch message.Role {
			case "system":
				systemPrompt = message.Content
			case "user":
				userPrompt = message.Content
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": `{"reply":"Mình chưa đánh giá nội dung kỹ thuật vì bạn đang cần gợi ý.\nI think we should start with the main issue.\nWhat happened first?","evaluation":{"score":0,"summary":"No technical answer was supplied","whatWasGood":[],"mainIssue":"The learner needs a starter","nextAction":"Reuse the starter sentence and add one detail","corrections":[],"technicalPoints":[]}}`}}}})
	}))
	defer server.Close()

	provider := &DeepSeekProvider{APIKey: "test-key", BaseURL: server.URL, SmartModel: "smart", Client: server.Client()}
	result, err := provider.GenerateRoleplay(context.Background(), RoleplayRequest{Scenario: domain.RoleplayScenario{Type: "technical-interview", Level: "A2", Goal: "Explain a blocked deployment"}, Answer: "Mình chưa biết nói thế nào, giúp mình với."})
	if err != nil {
		t.Fatal(err)
	}
	for _, clause := range []string{
		"If it is Vietnamese or asks for help or guidance",
		"brief Vietnamese explanation",
		"exactly one simple English starter sentence",
		"do not pretend that the learner supplied a technical answer",
		"nextAction must tell the learner to reuse the starter sentence and add one detail",
		"continue the roleplay normally in English",
		"scenario level",
	} {
		if !strings.Contains(systemPrompt, clause) {
			t.Fatalf("adaptive prompt contract missing %q in %q", clause, systemPrompt)
		}
	}
	if !strings.Contains(userPrompt, "Learner answer: Mình chưa biết nói thế nào") {
		t.Fatalf("DeepSeek request did not include learner answer: %q", userPrompt)
	}
	if !result.GuidanceOnly || result.Evaluation.Provider != "deepseek" || result.Evaluation.Score != 0 {
		t.Fatalf("unexpected adaptive DeepSeek result: %+v", result)
	}
}

func TestProviderHealthChecksProbeAuthenticatedEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/models" {
			if r.Header.Get("Authorization") != "Bearer provider-key" {
				t.Fatalf("missing provider authorization header")
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":[]}`)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/cognitiveservices/voices/list" {
			if r.Header.Get("Ocp-Apim-Subscription-Key") != "provider-key" {
				t.Fatalf("missing Azure subscription header")
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `[]`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	deepSeek := &DeepSeekProvider{APIKey: "provider-key", BaseURL: server.URL, Client: server.Client()}
	groq := &GroqSTTProvider{APIKey: "provider-key", BaseURL: server.URL, Client: server.Client()}
	azure := &AzureSpeechProvider{APIKey: "provider-key", STTURL: server.URL, TTSURL: server.URL, Client: server.Client()}
	for _, provider := range []HealthChecker{deepSeek, groq, azure} {
		if err := provider.HealthCheck(context.Background()); err != nil {
			t.Fatalf("health check failed: %v", err)
		}
	}
}

func TestCapabilityProbesCheckTheConfiguredModelAndSeparateAzureCapabilities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/models":
			if r.Header.Get("Authorization") != "Bearer groq-key" {
				t.Fatalf("missing Groq authorization header")
			}
			_, _ = io.WriteString(w, `{"data":[{"id":"whisper-test"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/cognitiveservices/voices/list":
			if r.Header.Get("Ocp-Apim-Subscription-Key") != "azure-key" {
				t.Fatalf("missing Azure TTS key")
			}
			_, _ = io.WriteString(w, `[{"ShortName":"en-US-Test"}]`)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/speech/recognition/"):
			if r.Header.Get("Ocp-Apim-Subscription-Key") != "azure-key" || r.Header.Get("Pronunciation-Assessment") == "" {
				t.Fatalf("missing Azure pronunciation probe headers")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"RecognitionStatus":"Success"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	groq := &GroqSTTProvider{APIKey: "groq-key", BaseURL: server.URL, Model: "whisper-test", Client: server.Client()}
	groqCheck := groq.ProbeCapability(context.Background(), "speech_to_text")
	if !groqCheck.Configured || !groqCheck.Reachable || !groqCheck.Healthy || groqCheck.Capability != "speech_to_text" || groqCheck.Model != "whisper-test" {
		t.Fatalf("unexpected Groq capability probe: %+v", groqCheck)
	}

	azure := &AzureSpeechProvider{APIKey: "azure-key", STTURL: server.URL, TTSURL: server.URL, Voice: "en-US-Test", Client: server.Client()}
	pronunciation := azure.ProbeCapability(context.Background(), "pronunciation_assessment")
	if !pronunciation.Healthy || pronunciation.Capability != "pronunciation_assessment" || pronunciation.Model != "azure-pronunciation-assessment" {
		t.Fatalf("unexpected Azure pronunciation probe: %+v", pronunciation)
	}
	tts := azure.ProbeCapability(context.Background(), "text_to_speech")
	if !tts.Healthy || tts.Capability != "text_to_speech" || tts.Model != "en-US-Test" {
		t.Fatalf("unexpected Azure TTS probe: %+v", tts)
	}
}

func TestCapabilityProbeReturnsSafeErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"secret provider details must not escape"}`)
	}))
	defer server.Close()

	provider := &GroqSTTProvider{APIKey: "groq-key", BaseURL: server.URL, Model: "whisper-test", Client: server.Client()}
	check := provider.ProbeCapability(context.Background(), "speech_to_text")
	if check.Healthy || !check.Reachable || check.Error != "authentication_failed" || strings.Contains(check.Error, "secret") {
		t.Fatalf("probe leaked or misclassified provider error: %+v", check)
	}
}

func TestFallbackProviderDoesNotHidePrimaryFailureWhenDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "provider unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	provider := FallbackProvider{
		Primary:       &DeepSeekProvider{APIKey: "provider-key", BaseURL: server.URL, Client: server.Client()},
		Fallback:      DeterministicProvider{},
		AllowFallback: false,
	}
	_, err := provider.GenerateMission(context.Background(), MissionRequest{LearningState: domain.LearningState{CEFR: "B1"}})
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected provider-unavailable error, got %v", err)
	}
}

func TestGroqTranscribeSendsMultipartAudioWithoutPersistingIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer groq-key" {
			t.Fatalf("missing STT authorization header")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("model") != "whisper-test" || r.FormValue("language") != "en" {
			t.Fatalf("unexpected STT fields: model=%q language=%q", r.FormValue("model"), r.FormValue("language"))
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if !strings.HasSuffix(header.Filename, ".webm") {
			t.Fatalf("expected a provider-compatible audio filename, got %q", header.Filename)
		}
		body, err := io.ReadAll(file)
		if err != nil || string(body) != "audio-bytes" {
			t.Fatalf("unexpected audio payload: %q err=%v", body, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"text":"The API is stable."}`)
	}))
	defer server.Close()

	provider := &GroqSTTProvider{APIKey: "groq-key", BaseURL: server.URL, Model: "whisper-test", Client: server.Client()}
	transcript, err := provider.Transcribe(context.Background(), []byte("audio-bytes"), "audio/webm")
	if err != nil {
		t.Fatal(err)
	}
	if transcript.Text != "The API is stable." {
		t.Fatalf("unexpected transcript: %+v", transcript)
	}
}

func TestAzureTTSUsesEscapedSSMLAndReturnsAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		payload := string(body)
		if !strings.Contains(payload, "&lt;tag&gt;") || strings.Contains(payload, "<tag>") {
			t.Fatalf("text was not escaped in SSML: %s", payload)
		}
		if r.Header.Get("Ocp-Apim-Subscription-Key") != "azure-key" {
			t.Fatalf("missing Azure key")
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("mp3"))
	}))
	defer server.Close()

	provider := &AzureSpeechProvider{APIKey: "azure-key", STTURL: server.URL, TTSURL: server.URL, Voice: "en-US-Test", Client: server.Client()}
	audio, err := provider.Synthesize(context.Background(), "Use <tag> safely", "")
	if err != nil {
		t.Fatal(err)
	}
	if string(audio) != "mp3" {
		t.Fatalf("unexpected TTS audio: %q", audio)
	}
}

func TestDeepSeekUsageAwareCopilotAndRoleplayKeepModelAccounting(t *testing.T) {
	var models []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		models = append(models, request.Model)
		userContent := ""
		for _, message := range request.Messages {
			if message.Role == "user" {
				userContent = message.Content
			}
		}
		content := `{"simple":"Please check the API.","natural":"Could you check the API?","professional":"Could you please check the API and confirm the result?","explanation":"These options keep the technical meaning."}`
		if strings.Contains(userContent, "Scenario:") {
			content = `{"reply":"That makes sense. What evidence supports this choice?","evaluation":{"score":80,"summary":"The answer identifies the technical choice.","whatWasGood":["It names the choice."],"mainIssue":"Add one concrete evidence detail.","nextAction":"State the evidence and the next step.","corrections":[],"technicalPoints":["technical choice"]}}`
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": content}}},
			"usage":   map[string]int{"prompt_tokens": 17, "completion_tokens": 9, "total_tokens": 26},
		})
	}))
	defer server.Close()

	provider := &DeepSeekProvider{
		APIKey: "test-key", BaseURL: server.URL, FastModel: "fast-model", SmartModel: "smart-model", Client: server.Client(),
	}
	copilot, copilotUsage, err := provider.GenerateCopilotWithUsage(context.Background(), CopilotRequest{Vietnamese: "Hãy kiểm tra API", Context: "release"})
	if err != nil {
		t.Fatal(err)
	}
	if copilot.Simple == "" || copilot.Natural == "" || copilot.Professional == "" || !copilotUsage.Available || copilotUsage.TotalTokens != 26 || copilotUsage.Model != "fast-model" {
		t.Fatalf("copilot result/usage = %+v / %+v", copilot, copilotUsage)
	}
	roleplay, roleplayUsage, err := provider.GenerateRoleplayWithUsage(context.Background(), RoleplayRequest{
		Scenario: domain.RoleplayScenario{Type: "technical-interview", Level: "A2", Goal: "Explain an API timeout"},
		Answer:   "The API timed out because the database was slow.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if roleplay.Reply == "" || roleplay.Evaluation.Provider != "deepseek" || !roleplayUsage.Available || roleplayUsage.TotalTokens != 26 || roleplayUsage.Model != "smart-model" {
		t.Fatalf("roleplay result/usage = %+v / %+v", roleplay, roleplayUsage)
	}
	if len(models) != 2 || models[0] != "fast-model" || models[1] != "smart-model" {
		t.Fatalf("DeepSeek routing models = %v, want fast then smart", models)
	}
}
