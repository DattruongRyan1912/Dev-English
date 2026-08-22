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
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer server.Close()

	provider := &DeepSeekProvider{APIKey: "test-key", BaseURL: server.URL, FastModel: "fast", Client: server.Client()}
	mission, err := provider.GenerateMission(context.Background(), MissionRequest{LearningState: domain.LearningState{CEFR: "B1"}})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || mission.ID == "" || mission.Status != "available" {
		t.Fatalf("expected one validation retry and finalized mission, requests=%d mission=%+v", requests, mission)
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
