package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learning"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func testServer() http.Handler {
	memory := store.NewSeeded(time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC))
	service := learning.NewService(memory, ai.DeterministicProvider{})
	return NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
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
	if got := response.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type" {
		t.Fatalf("production preflight advertised %q, want Content-Type only", got)
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

type fakeGitHubImporter struct{}

func (fakeGitHubImporter) Import(_ context.Context, rawURL string) (domain.GitHubImport, error) {
	return domain.GitHubImport{
		URL:        rawURL,
		SourceType: "github-pull",
		Title:      "Improve retry behavior",
		Content:    "The API retry reduces latency for a transient dependency failure and documents the next step.",
	}, nil
}
