package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learning"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learningoverlay"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func testServer() http.Handler {
	memory := store.NewSeeded(time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC))
	service := learning.NewService(memory, ai.DeterministicProvider{})
	return NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
}

func productTestServer(t *testing.T) http.Handler {
	server, _ := productTestServerWithApp(t)
	return server.Handler()
}

func productTestServerWithApp(t *testing.T) (*Server, *application.App) {
	t.Helper()
	memory := store.NewSeeded(time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC))
	service := learning.NewService(memory, ai.DeterministicProvider{})
	productApp, err := application.NewMemory()
	if err != nil {
		t.Fatalf("create product application: %v", err)
	}
	server := NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.Application = productApp
	overlay, err := learningoverlay.NewService(learningoverlay.NewMemoryRepository())
	if err != nil {
		t.Fatalf("create learning overlay: %v", err)
	}
	server.LearningOverlay = overlay
	return server, productApp
}

func TestHomeEndpointReturnsLearningState(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/home", nil)
	response := httptest.NewRecorder()
	testServer().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", response.Code)
	}
	if !strings.Contains(response.Body.String(), "todayMission") || !strings.Contains(response.Body.String(), "technical_writing") {
		t.Fatalf("home response did not include expected learning state: %s", response.Body.String())
	}
}

func TestReadyEndpointReturnsReady(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	testServer().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"status":"ready"`) {
		t.Fatalf("readiness response did not contain ready status: %s", response.Body.String())
	}
}

func TestConfiguredCORSRejectsUnknownOrigins(t *testing.T) {
	memory := store.NewSeeded(time.Now().UTC())
	service := learning.NewService(memory, ai.DeterministicProvider{})
	server := NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.AllowedOrigins = []string{"https://app.example.com"}
	handler := server.Handler()

	allowed := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	allowed.Header.Set("Origin", "https://app.example.com")
	allowedResponse := httptest.NewRecorder()
	handler.ServeHTTP(allowedResponse, allowed)
	if allowedResponse.Code != http.StatusOK ||
		allowedResponse.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" ||
		allowedResponse.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("allowed origin was not accepted: %d %q", allowedResponse.Code, allowedResponse.Header().Get("Access-Control-Allow-Origin"))
	}

	denied := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	denied.Header.Set("Origin", "https://evil.example.com")
	deniedResponse := httptest.NewRecorder()
	handler.ServeHTTP(deniedResponse, denied)
	if deniedResponse.Code != http.StatusForbidden {
		t.Fatalf("unknown origin returned %d, want 403", deniedResponse.Code)
	}
}

func TestProductionCORSOnlyAdvertisesCookieHeaders(t *testing.T) {
	memory := store.NewSeeded(time.Now().UTC())
	service := learning.NewService(memory, ai.DeterministicProvider{})
	server := NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.StrictAuth = true
	server.AllowedOrigins = []string{"https://app.example.com"}
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/home", nil)
	request.Header.Set("Origin", "https://app.example.com")
	request.Header.Set("Access-Control-Request-Headers", "content-type, x-bootstrap-key")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("production preflight returned %d, want 204", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, Idempotency-Key" {
		t.Fatalf("production preflight advertised %q, want Content-Type and Idempotency-Key", got)
	}
}

func TestDevelopmentCORSSupportsCredentialedBrowserOrigin(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Origin", "http://localhost:8093")
	response := httptest.NewRecorder()
	testServer().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("development health returned %d", response.Code)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:8093" ||
		response.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("development CORS did not support credentials: %v", response.Header())
	}
}

func TestStrictAuthFailsClosedWhenNotConfigured(t *testing.T) {
	memory := store.NewSeeded(time.Now().UTC())
	service := learning.NewService(memory, ai.DeterministicProvider{})
	server := NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.StrictAuth = true
	handler := server.Handler()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/home", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("strict auth returned %d, want 503", response.Code)
	}
}

func TestWritingAttemptCreatesFeedback(t *testing.T) {
	body := `{"answer":"The API returns a 500 error for files over 10 MB. The expected behavior is a validation error. This impacts users, and the next step is to reproduce the issue and fix the upload limit."}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/missions/mission-today/attempts", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	testServer().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "evaluation") || !strings.Contains(response.Body.String(), "nextReview") {
		t.Fatalf("submission response missing structured result: %s", response.Body.String())
	}
}

func TestWorkContextCreatesSuggestedMission(t *testing.T) {
	body := `{"sourceType":"error","title":"Upload API","content":"The upload API returns HTTP 500 for files larger than 10 MB. We need a clearer validation error and a safe next step."}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/work-context", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	testServer().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("got status %d, want 201: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "suggestedMission") || !strings.Contains(response.Body.String(), "backend") {
		t.Fatalf("work import response missing suggestions: %s", response.Body.String())
	}
}

func TestDiagnosticRoleplayAndCopilotEndpoints(t *testing.T) {
	handler := testServer()
	questionsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/diagnostic/questions", nil)
	questionsResponse := httptest.NewRecorder()
	handler.ServeHTTP(questionsResponse, questionsRequest)
	if questionsResponse.Code != http.StatusOK || !strings.Contains(questionsResponse.Body.String(), "technical-writing-1") {
		t.Fatalf("unexpected diagnostic response: %d %s", questionsResponse.Code, questionsResponse.Body.String())
	}

	roleplayRequest := httptest.NewRequest(http.MethodPost, "/api/v1/roleplay/conversations", strings.NewReader(`{"scenarioId":"standup-blocker"}`))
	roleplayRequest.Header.Set("Content-Type", "application/json")
	roleplayResponse := httptest.NewRecorder()
	handler.ServeHTTP(roleplayResponse, roleplayRequest)
	if roleplayResponse.Code != http.StatusCreated || !strings.Contains(roleplayResponse.Body.String(), "conversation") {
		t.Fatalf("unexpected roleplay response: %d %s", roleplayResponse.Code, roleplayResponse.Body.String())
	}

	copilotRequest := httptest.NewRequest(http.MethodPost, "/api/v1/copilot", strings.NewReader(`{"vietnamese":"nhờ kiểm tra lại API","context":"backend"}`))
	copilotRequest.Header.Set("Content-Type", "application/json")
	copilotResponse := httptest.NewRecorder()
	handler.ServeHTTP(copilotResponse, copilotRequest)
	if copilotResponse.Code != http.StatusOK || !strings.Contains(copilotResponse.Body.String(), "professional") {
		t.Fatalf("unexpected copilot response: %d %s", copilotResponse.Code, copilotResponse.Body.String())
	}
}

func TestReviewEndpointUpdatesDueItems(t *testing.T) {
	handler := testServer()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/review/mistake/mistake-1", strings.NewReader(`{"success":true,"score":90}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "nextReview") {
		t.Fatalf("unexpected review response: %d %s", response.Code, response.Body.String())
	}
}

func TestLegacyLearningEndpointsRemainCompatible(t *testing.T) {
	handler := testServer()
	readPaths := []string{
		"/api/v1/auth/me",
		"/api/v1/practice",
		"/api/v1/review/due",
		"/api/v1/review",
		"/api/v1/progress",
		"/api/v1/roleplay/scenarios",
		"/api/v1/vocabulary",
		"/api/v1/usage",
		"/api/v1/privacy/export",
		"/api/v1/settings",
	}
	for _, path := range readPaths {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s returned %d: %s", path, response.Code, response.Body.String())
		}
	}

	diagnosticRequest := httptest.NewRequest(http.MethodPost, "/api/v1/diagnostic", strings.NewReader(`{"responses":[{"questionId":"technical-writing-1","answer":"I can do this independently"}]}`))
	diagnosticRequest.Header.Set("Content-Type", "application/json")
	diagnosticResponse := httptest.NewRecorder()
	handler.ServeHTTP(diagnosticResponse, diagnosticRequest)
	if diagnosticResponse.Code != http.StatusOK || !strings.Contains(diagnosticResponse.Body.String(), "skillScores") {
		t.Fatalf("diagnostic submission returned %d: %s", diagnosticResponse.Code, diagnosticResponse.Body.String())
	}
	storedDiagnostic := httptest.NewRecorder()
	handler.ServeHTTP(storedDiagnostic, httptest.NewRequest(http.MethodGet, "/api/v1/diagnostic", nil))
	if storedDiagnostic.Code != http.StatusOK || !strings.Contains(storedDiagnostic.Body.String(), "recommendedPlan") {
		t.Fatalf("diagnostic read returned %d: %s", storedDiagnostic.Code, storedDiagnostic.Body.String())
	}

	startRequest := httptest.NewRequest(http.MethodPost, "/api/v1/roleplay/conversations", strings.NewReader(`{"scenarioId":"standup-blocker"}`))
	startRequest.Header.Set("Content-Type", "application/json")
	startResponse := httptest.NewRecorder()
	handler.ServeHTTP(startResponse, startRequest)
	if startResponse.Code != http.StatusCreated {
		t.Fatalf("roleplay start returned %d: %s", startResponse.Code, startResponse.Body.String())
	}
	var conversation domain.Conversation
	if err := json.Unmarshal(startResponse.Body.Bytes(), &conversation); err != nil {
		t.Fatalf("decode roleplay conversation: %v", err)
	}
	turnRequest := httptest.NewRequest(http.MethodPost, "/api/v1/roleplay/conversations/"+conversation.ID+"/turns", strings.NewReader(`{"answer":"The provider is returning unstable responses, so the API request is blocked. I will inspect the logs and share an ETA."}`))
	turnRequest.Header.Set("Content-Type", "application/json")
	turnResponse := httptest.NewRecorder()
	handler.ServeHTTP(turnResponse, turnRequest)
	if turnResponse.Code != http.StatusOK || !strings.Contains(turnResponse.Body.String(), "feedback") {
		t.Fatalf("roleplay turn returned %d: %s", turnResponse.Code, turnResponse.Body.String())
	}

	transcriptRequest := httptest.NewRequest(http.MethodPost, "/api/v1/speaking/transcript", strings.NewReader(`{"missionId":"mission-today","transcript":"The deployment is stable."}`))
	transcriptRequest.Header.Set("Content-Type", "application/json")
	transcriptResponse := httptest.NewRecorder()
	handler.ServeHTTP(transcriptResponse, transcriptRequest)
	if transcriptResponse.Code != http.StatusCreated || !strings.Contains(transcriptResponse.Body.String(), "The deployment is stable") {
		t.Fatalf("transcript save returned %d: %s", transcriptResponse.Code, transcriptResponse.Body.String())
	}

	dailyResponse := httptest.NewRecorder()
	handler.ServeHTTP(dailyResponse, httptest.NewRequest(http.MethodPost, "/api/v1/missions/daily", nil))
	if dailyResponse.Code != http.StatusCreated || !strings.Contains(dailyResponse.Body.String(), "mission") {
		t.Fatalf("daily mission returned %d: %s", dailyResponse.Code, dailyResponse.Body.String())
	}

	settingsRequest := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(`{"aiProvider":"deepseek","fastModel":"deepseek-v4-flash","smartModel":"deepseek-v4-pro","monthlyBudgetVND":300000}`))
	settingsRequest.Header.Set("Content-Type", "application/json")
	settingsResponse := httptest.NewRecorder()
	handler.ServeHTTP(settingsResponse, settingsRequest)
	if settingsResponse.Code != http.StatusOK || !strings.Contains(settingsResponse.Body.String(), "deepseek-v4-pro") {
		t.Fatalf("settings update returned %d: %s", settingsResponse.Code, settingsResponse.Body.String())
	}

	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, httptest.NewRequest(http.MethodDelete, "/api/v1/privacy/data", nil))
	if deleteResponse.Code != http.StatusForbidden {
		t.Fatalf("development data deletion returned %d, want 403: %s", deleteResponse.Code, deleteResponse.Body.String())
	}
}

func TestSpeechHTTPEndpointsPreserveEditableTranscriptFlow(t *testing.T) {
	memory := store.NewSeeded(time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC))
	service := learning.NewService(memory, ai.DeterministicProvider{}, learning.SpeechDependencies{
		STT:           httpTestSTTProvider{},
		TTS:           httpTestTTSProvider{},
		Pronunciation: httpTestPronunciationProvider{},
	})
	handler := NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()

	transcribeResponse := httptest.NewRecorder()
	handler.ServeHTTP(transcribeResponse, newAudioRequest(t, "/api/v1/speaking/transcribe", map[string]string{"missionId": "mission-today"}, true))
	if transcribeResponse.Code != http.StatusCreated {
		t.Fatalf("transcribe returned %d: %s", transcribeResponse.Code, transcribeResponse.Body.String())
	}
	var session domain.SpeakingSession
	if err := json.Unmarshal(transcribeResponse.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode speaking session: %v", err)
	}
	if session.ID == "" || session.Transcript != "The deployment is stable." {
		t.Fatalf("unexpected transcript session: %+v", session)
	}

	assessResponse := httptest.NewRecorder()
	handler.ServeHTTP(assessResponse, newAudioRequest(t, "/api/v1/speaking/assess", map[string]string{"sessionId": session.ID}, true))
	if assessResponse.Code != http.StatusOK || !strings.Contains(assessResponse.Body.String(), "evaluated") {
		t.Fatalf("assess returned %d: %s", assessResponse.Code, assessResponse.Body.String())
	}

	synthesizeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/speaking/synthesize", strings.NewReader(`{"text":"Hello developer","voice":"en-US-Test"}`))
	synthesizeRequest.Header.Set("Content-Type", "application/json")
	synthesizeResponse := httptest.NewRecorder()
	handler.ServeHTTP(synthesizeResponse, synthesizeRequest)
	if synthesizeResponse.Code != http.StatusOK || synthesizeResponse.Header().Get("Content-Type") != "audio/mpeg" || synthesizeResponse.Body.String() != "audio" {
		t.Fatalf("synthesize returned %d content-type=%q body=%q", synthesizeResponse.Code, synthesizeResponse.Header().Get("Content-Type"), synthesizeResponse.Body.String())
	}

	missingAudio := httptest.NewRecorder()
	handler.ServeHTTP(missingAudio, httptest.NewRequest(http.MethodPost, "/api/v1/speaking/transcribe", strings.NewReader(`{}`)))
	if missingAudio.Code != http.StatusBadRequest {
		t.Fatalf("missing audio returned %d, want 400: %s", missingAudio.Code, missingAudio.Body.String())
	}
	missingSession := httptest.NewRecorder()
	handler.ServeHTTP(missingSession, newAudioRequest(t, "/api/v1/speaking/assess", nil, true))
	if missingSession.Code != http.StatusBadRequest || !strings.Contains(missingSession.Body.String(), "sessionId") {
		t.Fatalf("missing session returned %d: %s", missingSession.Code, missingSession.Body.String())
	}
}

func TestSpeechUnavailableAndHTTPBoundaryHelpers(t *testing.T) {
	noProvider := httptest.NewRecorder()
	handler := testServer()
	handler.ServeHTTP(noProvider, newAudioRequest(t, "/api/v1/speaking/transcribe", nil, true))
	if noProvider.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured speech returned %d, want 503: %s", noProvider.Code, noProvider.Body.String())
	}

	if got := featureErrorStatus(learning.ErrBudgetExceeded, http.StatusBadGateway); got != http.StatusTooManyRequests {
		t.Fatalf("budget error status = %d", got)
	}
	for _, test := range []struct {
		err  error
		want int
	}{
		{errors.New("GitHub integration is not configured"), http.StatusServiceUnavailable},
		{errors.New("only github.com URLs are supported"), http.StatusBadRequest},
		{errors.New("upstream failed"), http.StatusBadGateway},
	} {
		if got := githubErrorStatus(test.err); got != test.want {
			t.Errorf("githubErrorStatus(%q) = %d, want %d", test.err, got, test.want)
		}
	}

	if got := providerMode(nil); got != "unavailable" {
		t.Errorf("providerMode(nil) = %q", got)
	}
	if got := providerMode(ai.DeterministicProvider{}); got != "deterministic-fallback" {
		t.Errorf("providerMode(deterministic) = %q", got)
	}
	ctx, cancel := WithTimeout(context.Background())
	defer cancel()
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 31*time.Second {
		t.Fatalf("WithTimeout deadline is not bounded: %v, %v", deadline, ok)
	}
}

func newAudioRequest(t *testing.T, path string, fields map[string]string, includeAudio bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write multipart field %s: %v", key, err)
		}
	}
	if includeAudio {
		part, err := writer.CreateFormFile("audio", "recording.webm")
		if err != nil {
			t.Fatalf("create audio part: %v", err)
		}
		if _, err := part.Write([]byte("audio")); err != nil {
			t.Fatalf("write audio part: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart body: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

type httpTestSTTProvider struct{}

func (httpTestSTTProvider) Name() string     { return "groq-http-test" }
func (httpTestSTTProvider) Configured() bool { return true }
func (httpTestSTTProvider) Transcribe(context.Context, []byte, string) (ai.Transcript, error) {
	return ai.Transcript{Text: "The deployment is stable."}, nil
}

type httpTestTTSProvider struct{}

func (httpTestTTSProvider) Name() string     { return "azure-tts-http-test" }
func (httpTestTTSProvider) Configured() bool { return true }
func (httpTestTTSProvider) Synthesize(context.Context, string, string) ([]byte, error) {
	return []byte("audio"), nil
}

type httpTestPronunciationProvider struct{}

func (httpTestPronunciationProvider) Name() string     { return "azure-pronunciation-http-test" }
func (httpTestPronunciationProvider) Configured() bool { return true }
func (httpTestPronunciationProvider) Assess(context.Context, []byte, string, string) (ai.PronunciationResult, error) {
	return ai.PronunciationResult{Accuracy: 80, Fluency: 75, Completeness: 90, Prosody: 70}, nil
}

func TestV2ReadEndpointsReturnAnalyticsGraphAndProviderChecks(t *testing.T) {
	handler := testServer()
	paths := []string{
		"/api/v1/vocabulary/graph",
		"/api/v1/analytics",
		"/api/v1/speaking/weekly",
		"/api/v1/settings/test",
	}
	for _, path := range paths {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s returned %d: %s", path, response.Code, response.Body.String())
		}
	}
}

func TestGitHubImportValidatesInputBeforeIntegration(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/github/import", strings.NewReader(`{"url":""}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	testServer().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "GitHub URL is required") {
		t.Fatalf("unexpected GitHub validation response: %d %s", response.Code, response.Body.String())
	}
}

func TestGitHubImportCreatesWorkContextFromConnector(t *testing.T) {
	memory := store.NewSeeded(time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC))
	service := learning.NewService(memory, ai.DeterministicProvider{}, learning.SpeechDependencies{GitHub: fakeGitHubImporter{}})
	handler := NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/github/import", strings.NewReader(`{"url":"https://github.com/acme/api/pull/42"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), "github-pull") || !strings.Contains(response.Body.String(), "sourceUrl") {
		t.Fatalf("unexpected GitHub import response: %d %s", response.Code, response.Body.String())
	}
}

func TestV2WalkingSkeletonPersistsProjectTaskKnowledgeAndConversation(t *testing.T) {
	handler := productTestServer(t)

	bootstrap := httptest.NewRequest(http.MethodGet, "/api/v2/bootstrap", nil)
	bootstrapResponse := httptest.NewRecorder()
	handler.ServeHTTP(bootstrapResponse, bootstrap)
	if bootstrapResponse.Code != http.StatusOK {
		t.Fatalf("bootstrap returned %d: %s", bootstrapResponse.Code, bootstrapResponse.Body.String())
	}

	projectRequest := httptest.NewRequest(http.MethodPost, "/api/v2/projects", strings.NewReader(`{"name":"Release hardening","description":"Track the verified release work."}`))
	projectRequest.Header.Set("Content-Type", "application/json")
	projectRequest.Header.Set("Idempotency-Key", "project-release-hardening")
	projectResponse := httptest.NewRecorder()
	handler.ServeHTTP(projectResponse, projectRequest)
	if projectResponse.Code != http.StatusCreated {
		t.Fatalf("project create returned %d: %s", projectResponse.Code, projectResponse.Body.String())
	}
	var project map[string]any
	if err := json.Unmarshal(projectResponse.Body.Bytes(), &project); err != nil {
		t.Fatalf("decode project response: %v", err)
	}
	projectID, ok := project["id"].(string)
	if !ok || projectID == "" {
		t.Fatalf("project response did not contain an id: %s", projectResponse.Body.String())
	}

	replayRequest := httptest.NewRequest(http.MethodPost, "/api/v2/projects", strings.NewReader(`{"name":"Release hardening","description":"Track the verified release work."}`))
	replayRequest.Header.Set("Content-Type", "application/json")
	replayRequest.Header.Set("Idempotency-Key", "project-release-hardening")
	replayResponse := httptest.NewRecorder()
	handler.ServeHTTP(replayResponse, replayRequest)
	if replayResponse.Code != http.StatusCreated || !strings.Contains(replayResponse.Body.String(), projectID) {
		t.Fatalf("idempotent project replay returned %d: %s", replayResponse.Code, replayResponse.Body.String())
	}

	taskRequest := httptest.NewRequest(http.MethodPost, "/api/v2/projects/"+projectID+"/tasks", strings.NewReader(`{"title":"Verify citation flow","description":"Confirm source revision and excerpt survive restart.","priority":"high"}`))
	taskRequest.Header.Set("Content-Type", "application/json")
	taskRequest.Header.Set("Idempotency-Key", "task-citation-flow")
	taskResponse := httptest.NewRecorder()
	handler.ServeHTTP(taskResponse, taskRequest)
	if taskResponse.Code != http.StatusCreated || !strings.Contains(taskResponse.Body.String(), "Verify citation flow") {
		t.Fatalf("task create returned %d: %s", taskResponse.Code, taskResponse.Body.String())
	}

	decisionRequest := httptest.NewRequest(http.MethodPost, "/api/v2/decisions", strings.NewReader(`{"title":"Use source-backed answers","context":"The assistant must not invent project facts.","outcome":"Require evidence before presenting a fact.","rationale":"Protect trust in the workspace."}`))
	decisionRequest.Header.Set("Content-Type", "application/json")
	decisionRequest.Header.Set("Idempotency-Key", "decision-source-backed")
	decisionResponse := httptest.NewRecorder()
	handler.ServeHTTP(decisionResponse, decisionRequest)
	if decisionResponse.Code != http.StatusCreated || !strings.Contains(decisionResponse.Body.String(), "Use source-backed answers") {
		t.Fatalf("decision create returned %d: %s", decisionResponse.Code, decisionResponse.Body.String())
	}

	sourceRequest := httptest.NewRequest(http.MethodPost, "/api/v2/knowledge/sources", strings.NewReader(`{"name":"Release runbook","kind":"manual","uri":"manual://release-runbook","mimeType":"text/plain","content":"The release gate requires citation-backed answers and a verified rollback plan."}`))
	sourceRequest.Header.Set("Content-Type", "application/json")
	sourceRequest.Header.Set("Idempotency-Key", "source-release-runbook")
	sourceResponse := httptest.NewRecorder()
	handler.ServeHTTP(sourceResponse, sourceRequest)
	if sourceResponse.Code != http.StatusCreated {
		t.Fatalf("source import returned %d: %s", sourceResponse.Code, sourceResponse.Body.String())
	}

	sourceReplayRequest := httptest.NewRequest(http.MethodPost, "/api/v2/knowledge/sources", strings.NewReader(`{"name":"Release runbook","kind":"manual","uri":"manual://release-runbook","mimeType":"text/plain","content":"The release gate requires citation-backed answers and a verified rollback plan."}`))
	sourceReplayRequest.Header.Set("Content-Type", "application/json")
	sourceReplayRequest.Header.Set("Idempotency-Key", "source-release-runbook")
	sourceReplayResponse := httptest.NewRecorder()
	handler.ServeHTTP(sourceReplayResponse, sourceReplayRequest)
	if sourceReplayResponse.Code != http.StatusCreated || sourceReplayResponse.Body.String() != sourceResponse.Body.String() {
		t.Fatalf("source import replay returned %d/%s, want the original response %d/%s", sourceReplayResponse.Code, sourceReplayResponse.Body.String(), sourceResponse.Code, sourceResponse.Body.String())
	}

	sourceConflictRequest := httptest.NewRequest(http.MethodPost, "/api/v2/knowledge/sources", strings.NewReader(`{"name":"Release runbook","kind":"manual","uri":"manual://release-runbook","mimeType":"text/plain","content":"A changed payload must not reuse the existing import key."}`))
	sourceConflictRequest.Header.Set("Content-Type", "application/json")
	sourceConflictRequest.Header.Set("Idempotency-Key", "source-release-runbook")
	sourceConflictResponse := httptest.NewRecorder()
	handler.ServeHTTP(sourceConflictResponse, sourceConflictRequest)
	if sourceConflictResponse.Code != http.StatusConflict {
		t.Fatalf("source import key reuse returned %d: %s, want 409", sourceConflictResponse.Code, sourceConflictResponse.Body.String())
	}

	searchRequest := httptest.NewRequest(http.MethodGet, "/api/v2/knowledge/search?q=citation-backed", nil)
	searchResponse := httptest.NewRecorder()
	handler.ServeHTTP(searchResponse, searchRequest)
	if searchResponse.Code != http.StatusOK || !strings.Contains(searchResponse.Body.String(), "citation-backed") {
		t.Fatalf("knowledge search returned %d: %s", searchResponse.Code, searchResponse.Body.String())
	}

	decisionSearchRequest := httptest.NewRequest(http.MethodGet, "/api/v2/knowledge/search?q=evidence", nil)
	decisionSearchResponse := httptest.NewRecorder()
	handler.ServeHTTP(decisionSearchResponse, decisionSearchRequest)
	if decisionSearchResponse.Code != http.StatusOK || !strings.Contains(decisionSearchResponse.Body.String(), "Use source-backed answers") {
		t.Fatalf("work decision search returned %d: %s", decisionSearchResponse.Code, decisionSearchResponse.Body.String())
	}

	conversationRequest := httptest.NewRequest(http.MethodPost, "/api/v2/assistant/conversations", strings.NewReader(`{"message":"What does the release gate require?"}`))
	conversationRequest.Header.Set("Content-Type", "application/json")
	conversationResponse := httptest.NewRecorder()
	handler.ServeHTTP(conversationResponse, conversationRequest)
	if conversationResponse.Code != http.StatusCreated ||
		!strings.Contains(conversationResponse.Body.String(), `"grounding":"grounded"`) ||
		!strings.Contains(conversationResponse.Body.String(), "citation-backed") {
		t.Fatalf("assistant ask returned %d: %s", conversationResponse.Code, conversationResponse.Body.String())
	}

	finalBootstrap := httptest.NewRequest(http.MethodGet, "/api/v2/bootstrap", nil)
	finalBootstrapResponse := httptest.NewRecorder()
	handler.ServeHTTP(finalBootstrapResponse, finalBootstrap)
	if finalBootstrapResponse.Code != http.StatusOK ||
		!strings.Contains(finalBootstrapResponse.Body.String(), "Release hardening") ||
		!strings.Contains(finalBootstrapResponse.Body.String(), "Release runbook") ||
		!strings.Contains(finalBootstrapResponse.Body.String(), "What does the release gate require?") {
		t.Fatalf("final bootstrap did not contain durable walking-skeleton data: %d %s", finalBootstrapResponse.Code, finalBootstrapResponse.Body.String())
	}
}

func TestV2AssistantReturnsUnknownWithoutVerifiedEvidence(t *testing.T) {
	handler := productTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/assistant/conversations", strings.NewReader(`{"message":"Tell me a fact that is not in the workspace."}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated ||
		!strings.Contains(response.Body.String(), `"grounding":"unknown"`) ||
		!strings.Contains(response.Body.String(), "enough verified evidence") {
		t.Fatalf("unknown assistant response returned %d: %s", response.Code, response.Body.String())
	}
}

func TestV2LearningOverlayRecordsWithoutMutatingWork(t *testing.T) {
	handler := productTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/learning/observations", strings.NewReader(`{"sourceType":"task","sourceId":"task-1","skill":"technical_writing","prompt":"Explain the next step.","response":"I will inspect the backend logs."}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), "learning-observation-") {
		t.Fatalf("record observation returned %d: %s", response.Code, response.Body.String())
	}

	list := httptest.NewRequest(http.MethodGet, "/api/v2/learning/observations", nil)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, list)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "inspect the backend logs") {
		t.Fatalf("list observations returned %d: %s", listResponse.Code, listResponse.Body.String())
	}

	workList := httptest.NewRequest(http.MethodGet, "/api/v2/projects", nil)
	workResponse := httptest.NewRecorder()
	handler.ServeHTTP(workResponse, workList)
	if workResponse.Code != http.StatusOK {
		t.Fatalf("work list returned %d: %s", workResponse.Code, workResponse.Body.String())
	}
	if strings.Contains(workResponse.Body.String(), "inspect the backend logs") {
		t.Fatal("learning observation leaked into canonical work data")
	}
}

func TestV2WorkMutationRoutesExposeVersionConflictTrashRestoreAndDecisionCRUD(t *testing.T) {
	handler := productTestServer(t)
	createProject := httptest.NewRequest(http.MethodPost, "/api/v2/projects", strings.NewReader(`{"name":"API project"}`))
	createProject.Header.Set("Content-Type", "application/json")
	createProject.Header.Set("Idempotency-Key", "http-project-create")
	createdProject := httptest.NewRecorder()
	handler.ServeHTTP(createdProject, createProject)
	if createdProject.Code != http.StatusCreated {
		t.Fatalf("create project returned %d: %s", createdProject.Code, createdProject.Body.String())
	}
	var project struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	if err := json.Unmarshal(createdProject.Body.Bytes(), &project); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	if project.ID == "" || project.Version != 1 {
		t.Fatalf("created project = %+v", project)
	}

	update := httptest.NewRequest(http.MethodPatch, "/api/v2/projects/"+project.ID, strings.NewReader(`{"name":"Renamed project","expectedVersion":1}`))
	update.Header.Set("Content-Type", "application/json")
	update.Header.Set("Idempotency-Key", "http-project-update")
	updated := httptest.NewRecorder()
	handler.ServeHTTP(updated, update)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), "Renamed project") || !strings.Contains(updated.Body.String(), `"version":2`) {
		t.Fatalf("update project returned %d: %s", updated.Code, updated.Body.String())
	}

	stale := httptest.NewRequest(http.MethodPatch, "/api/v2/projects/"+project.ID, strings.NewReader(`{"description":"stale","expectedVersion":1}`))
	stale.Header.Set("Content-Type", "application/json")
	stale.Header.Set("Idempotency-Key", "http-project-stale")
	staleResponse := httptest.NewRecorder()
	handler.ServeHTTP(staleResponse, stale)
	if staleResponse.Code != http.StatusConflict || !strings.Contains(staleResponse.Body.String(), "version") {
		t.Fatalf("stale project update returned %d: %s", staleResponse.Code, staleResponse.Body.String())
	}

	decisionRequest := httptest.NewRequest(http.MethodPost, "/api/v2/decisions", strings.NewReader(`{"projectId":"`+project.ID+`","title":"Choose source of truth","outcome":"Use revisions","rationale":"Keep canonical data separate from inference"}`))
	decisionRequest.Header.Set("Content-Type", "application/json")
	decisionRequest.Header.Set("Idempotency-Key", "http-decision-create")
	decisionResponse := httptest.NewRecorder()
	handler.ServeHTTP(decisionResponse, decisionRequest)
	if decisionResponse.Code != http.StatusCreated || !strings.Contains(decisionResponse.Body.String(), "Use revisions") {
		t.Fatalf("create decision returned %d: %s", decisionResponse.Code, decisionResponse.Body.String())
	}

	listDecisions := httptest.NewRequest(http.MethodGet, "/api/v2/decisions?projectId="+project.ID, nil)
	listDecisionsResponse := httptest.NewRecorder()
	handler.ServeHTTP(listDecisionsResponse, listDecisions)
	if listDecisionsResponse.Code != http.StatusOK || !strings.Contains(listDecisionsResponse.Body.String(), "Choose source of truth") {
		t.Fatalf("list decisions returned %d: %s", listDecisionsResponse.Code, listDecisionsResponse.Body.String())
	}

	trash := httptest.NewRequest(http.MethodPost, "/api/v2/projects/"+project.ID+"/trash", strings.NewReader(`{"expectedVersion":2}`))
	trash.Header.Set("Content-Type", "application/json")
	trash.Header.Set("Idempotency-Key", "http-project-trash")
	trashResponse := httptest.NewRecorder()
	handler.ServeHTTP(trashResponse, trash)
	if trashResponse.Code != http.StatusOK || !strings.Contains(trashResponse.Body.String(), "deletedAt") || !strings.Contains(trashResponse.Body.String(), `"version":3`) {
		t.Fatalf("trash project returned %d: %s", trashResponse.Code, trashResponse.Body.String())
	}

	activeProjects := httptest.NewRequest(http.MethodGet, "/api/v2/projects", nil)
	activeProjectsResponse := httptest.NewRecorder()
	handler.ServeHTTP(activeProjectsResponse, activeProjects)
	if activeProjectsResponse.Code != http.StatusOK || strings.Contains(activeProjectsResponse.Body.String(), "Renamed project") {
		t.Fatalf("default project list leaked trashed project: %d %s", activeProjectsResponse.Code, activeProjectsResponse.Body.String())
	}

	allProjects := httptest.NewRequest(http.MethodGet, "/api/v2/projects?includeTrashed=true", nil)
	allProjectsResponse := httptest.NewRecorder()
	handler.ServeHTTP(allProjectsResponse, allProjects)
	if allProjectsResponse.Code != http.StatusOK || !strings.Contains(allProjectsResponse.Body.String(), "Renamed project") {
		t.Fatalf("includeTrashed project list omitted trashed project: %d %s", allProjectsResponse.Code, allProjectsResponse.Body.String())
	}

	restore := httptest.NewRequest(http.MethodPost, "/api/v2/projects/"+project.ID+"/restore", strings.NewReader(`{"expectedVersion":3}`))
	restore.Header.Set("Content-Type", "application/json")
	restore.Header.Set("Idempotency-Key", "http-project-restore")
	restoreResponse := httptest.NewRecorder()
	handler.ServeHTTP(restoreResponse, restore)
	if restoreResponse.Code != http.StatusOK || strings.Contains(restoreResponse.Body.String(), "deletedAt") || !strings.Contains(restoreResponse.Body.String(), `"version":4`) {
		t.Fatalf("restore project returned %d: %s", restoreResponse.Code, restoreResponse.Body.String())
	}

	history := httptest.NewRequest(http.MethodGet, "/api/v2/projects/"+project.ID+"/history", nil)
	historyResponse := httptest.NewRecorder()
	handler.ServeHTTP(historyResponse, history)
	if historyResponse.Code != http.StatusOK || strings.Count(historyResponse.Body.String(), `"action"`) != 4 {
		t.Fatalf("project history returned %d: %s", historyResponse.Code, historyResponse.Body.String())
	}
}

type fakeGitHubImporter struct{}

func (fakeGitHubImporter) Import(_ context.Context, rawURL string) (domain.GitHubImport, error) {
	return domain.GitHubImport{
		URL:        rawURL,
		SourceType: "github-pull",
		Title:      "Improve retry behavior",
		Content:    "The API retry reduces latency for a transient dependency failure and documents the next step.",
	}, nil
}
