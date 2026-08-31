package httpapi

import (
	"context"
	"errors"
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

func TestLegacyHTTPBoundaryRejectsMalformedAndMissingInputs(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantText   string
	}{
		{name: "review route shape", method: http.MethodPost, path: "/api/v1/review/mistake-1", body: `{}`, wantStatus: http.StatusNotFound, wantText: "review endpoint not found"},
		{name: "review malformed json", method: http.MethodPost, path: "/api/v1/review/mistake/mistake-1", body: `{`, wantStatus: http.StatusBadRequest, wantText: "valid JSON"},
		{name: "review unknown kind", method: http.MethodPost, path: "/api/v1/review/other/item-1", body: `{ "success": true }`, wantStatus: http.StatusBadRequest, wantText: "review kind"},
		{name: "diagnostic not submitted", method: http.MethodGet, path: "/api/v1/diagnostic", wantStatus: http.StatusNotFound, wantText: "resource not found"},
		{name: "diagnostic empty", method: http.MethodPost, path: "/api/v1/diagnostic", body: `{ "responses": [] }`, wantStatus: http.StatusBadRequest, wantText: "at least one diagnostic response"},
		{name: "diagnostic unknown question", method: http.MethodPost, path: "/api/v1/diagnostic", body: `{ "responses": [{"questionId":"unknown","answer":"anything"}] }`, wantStatus: http.StatusBadRequest, wantText: "do not match"},
		{name: "roleplay unknown scenario", method: http.MethodPost, path: "/api/v1/roleplay/conversations", body: `{ "scenarioId": "missing" }`, wantStatus: http.StatusNotFound, wantText: "resource not found"},
		{name: "roleplay turn route shape", method: http.MethodPost, path: "/api/v1/roleplay/conversations/conversation-1", body: `{}`, wantStatus: http.StatusNotFound, wantText: "roleplay turn endpoint not found"},
		{name: "roleplay turn missing answer", method: http.MethodPost, path: "/api/v1/roleplay/conversations/conversation-1/turns", body: `{}`, wantStatus: http.StatusBadRequest, wantText: "answer is required"},
		{name: "roleplay conversation not found", method: http.MethodPost, path: "/api/v1/roleplay/conversations/missing/turns", body: `{ "answer": "I will inspect the logs and share an ETA." }`, wantStatus: http.StatusNotFound, wantText: "resource not found"},
		{name: "copilot missing vietnamese text", method: http.MethodPost, path: "/api/v1/copilot", body: `{ "context": "backend" }`, wantStatus: http.StatusBadRequest, wantText: "vietnamese text is required"},
		{name: "github missing url", method: http.MethodPost, path: "/api/v1/integrations/github/import", body: `{}`, wantStatus: http.StatusBadRequest, wantText: "GitHub URL is required"},
		{name: "work context too short", method: http.MethodPost, path: "/api/v1/work-context", body: `{ "sourceType": "error", "content": "short" }`, wantStatus: http.StatusBadRequest, wantText: "at least 20 characters"},
		{name: "mission route shape", method: http.MethodPost, path: "/api/v1/missions/mission-today", body: `{}`, wantStatus: http.StatusNotFound, wantText: "mission endpoint not found"},
		{name: "mission missing answer", method: http.MethodPost, path: "/api/v1/missions/mission-today/attempts", body: `{}`, wantStatus: http.StatusBadRequest, wantText: "answer is required"},
		{name: "transcript missing text", method: http.MethodPost, path: "/api/v1/speaking/transcript", body: `{ "missionId": "mission-today" }`, wantStatus: http.StatusBadRequest, wantText: "between 1 and 5000"},
		{name: "synthesize malformed json", method: http.MethodPost, path: "/api/v1/speaking/synthesize", body: `{`, wantStatus: http.StatusBadRequest, wantText: "valid JSON"},
		{name: "settings malformed json", method: http.MethodPut, path: "/api/v1/settings", body: `{`, wantStatus: http.StatusBadRequest, wantText: "valid JSON"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			request := httptest.NewRequest(item.method, item.path, strings.NewReader(item.body))
			if item.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			testServer().ServeHTTP(response, request)
			if response.Code != item.wantStatus || !strings.Contains(response.Body.String(), item.wantText) {
				t.Fatalf("returned %d: %s; want %d containing %q", response.Code, response.Body.String(), item.wantStatus, item.wantText)
			}
		})
	}
}

func TestLegacyHTTPBoundaryMapsProviderAndConfigurationFailures(t *testing.T) {
	noOptional := httpNoOptionalProvider{providerName: "primary-test", configured: true}
	noOptionalHandler := NewServer(learning.NewService(store.NewSeeded(time.Now().UTC()), noOptional), slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()

	copilot := postJSONRequest(noOptionalHandler, "/api/v1/copilot", `{ "vietnamese": "giải thích lỗi" }`)
	if copilot.Code != http.StatusBadGateway || !strings.Contains(copilot.Body.String(), "copilot provider is unavailable") {
		t.Fatalf("missing Copilot capability returned %d: %s", copilot.Code, copilot.Body.String())
	}

	startRoleplay := postJSONRequest(noOptionalHandler, "/api/v1/roleplay/conversations", `{ "scenarioId": "standup-blocker" }`)
	if startRoleplay.Code != http.StatusCreated {
		t.Fatalf("roleplay start with provider missing returned %d: %s", startRoleplay.Code, startRoleplay.Body.String())
	}
	turn := postJSONRequest(noOptionalHandler, "/api/v1/roleplay/conversations/missing/turns", `{ "answer": "I will inspect the logs and share an ETA." }`)
	if turn.Code != http.StatusNotFound {
		t.Fatalf("missing roleplay conversation returned %d: %s", turn.Code, turn.Body.String())
	}

	githubHandler := NewServer(learning.NewService(store.NewSeeded(time.Now().UTC()), ai.DeterministicProvider{}, learning.SpeechDependencies{
		GitHub: httpErrorGitHubImporter{err: errors.New("upstream GitHub timeout")},
	}), slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
	github := postJSONRequest(githubHandler, "/api/v1/integrations/github/import", `{ "url": "https://github.com/acme/api/issues/1" }`)
	if github.Code != http.StatusBadGateway || !strings.Contains(github.Body.String(), "upstream GitHub timeout") {
		t.Fatalf("GitHub upstream failure returned %d: %s", github.Code, github.Body.String())
	}

	missingGitHub := postJSONRequest(testServer(), "/api/v1/integrations/github/import", `{ "url": "https://github.com/acme/api/issues/1" }`)
	if missingGitHub.Code != http.StatusServiceUnavailable || !strings.Contains(missingGitHub.Body.String(), "not configured") {
		t.Fatalf("unconfigured GitHub returned %d: %s", missingGitHub.Code, missingGitHub.Body.String())
	}

	deepSeekHandler := NewServer(learning.NewService(store.NewSeeded(time.Now().UTC()), ai.DeterministicProvider{}), slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodPut, "/api/v1/settings/deepseek", strings.NewReader(`{"apiKey":"sk-boundary-test-key-123456"}`)),
		httptest.NewRequest(http.MethodDelete, "/api/v1/settings/deepseek", nil),
		httptest.NewRequest(http.MethodPost, "/api/v1/settings/deepseek/test", nil),
	} {
		if request.Method == http.MethodPut {
			request.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		deepSeekHandler.ServeHTTP(response, request)
		if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "not configured") {
			t.Fatalf("DeepSeek %s without manager returned %d: %s", request.Method, response.Code, response.Body.String())
		}
	}
}

func TestLegacyHTTPBoundaryMapsRepositoryFailures(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		method     string
		body       string
		setError   func(*httpBoundaryRepository)
		wantStatus int
	}{
		{name: "ready", method: http.MethodGet, path: "/readyz", setError: func(repository *httpBoundaryRepository) { repository.readyErr = errors.New("database down") }, wantStatus: http.StatusServiceUnavailable},
		{name: "auth me", method: http.MethodGet, path: "/api/v1/auth/me", setError: func(repository *httpBoundaryRepository) { repository.userErr = errors.New("user lookup failed") }, wantStatus: http.StatusNotFound},
		{name: "review library", method: http.MethodGet, path: "/api/v1/review", setError: func(repository *httpBoundaryRepository) {
			repository.allMistakesErr = errors.New("mistakes unavailable")
		}, wantStatus: http.StatusInternalServerError},
		{name: "roleplay scenarios", method: http.MethodGet, path: "/api/v1/roleplay/scenarios", setError: func(repository *httpBoundaryRepository) {
			repository.scenariosErr = errors.New("scenario store unavailable")
		}, wantStatus: http.StatusInternalServerError},
		{name: "vocabulary", method: http.MethodGet, path: "/api/v1/vocabulary", setError: func(repository *httpBoundaryRepository) {
			repository.allVocabularyErr = errors.New("vocabulary unavailable")
		}, wantStatus: http.StatusInternalServerError},
		{name: "weekly speaking", method: http.MethodGet, path: "/api/v1/speaking/weekly", setError: func(repository *httpBoundaryRepository) {
			repository.allSpeakingErr = errors.New("speaking unavailable")
		}, wantStatus: http.StatusInternalServerError},
		{name: "usage", method: http.MethodGet, path: "/api/v1/usage", setError: func(repository *httpBoundaryRepository) { repository.usageErr = errors.New("usage unavailable") }, wantStatus: http.StatusInternalServerError},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			repository := &httpBoundaryRepository{Repository: store.NewSeeded(time.Now().UTC())}
			item.setError(repository)
			service := learning.NewService(repository, ai.DeterministicProvider{})
			handler := NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
			request := httptest.NewRequest(item.method, item.path, strings.NewReader(item.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != item.wantStatus {
				t.Fatalf("returned %d: %s; want %d", response.Code, response.Body.String(), item.wantStatus)
			}
		})
	}
}

func TestLegacyHTTPBoundaryPreservesProviderModeClassification(t *testing.T) {
	for _, item := range []struct {
		provider ai.Provider
		want     string
	}{
		{provider: httpNoOptionalProvider{providerName: "primary-test", configured: false}, want: "unavailable"},
		{provider: httpNoOptionalProvider{providerName: "primary-test", configured: true}, want: "primary"},
	} {
		if got := providerMode(item.provider); got != item.want {
			t.Fatalf("providerMode(%T) = %q, want %q", item.provider, got, item.want)
		}
	}
	if got := maxAge(time.Unix(1, 0), time.Unix(2, 0)); got != 1 {
		t.Fatalf("maxAge expired = %d, want 1", got)
	}
	if got := maxAge(time.Unix(20, 0), time.Unix(2, 0)); got != 18 {
		t.Fatalf("maxAge positive = %d, want 18", got)
	}
}

func postJSONRequest(handler http.Handler, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

type httpBoundaryRepository struct {
	store.Repository
	readyErr         error
	userErr          error
	allMistakesErr   error
	allVocabularyErr error
	scenariosErr     error
	allSpeakingErr   error
	usageErr         error
}

func (r *httpBoundaryRepository) Ready(context.Context) error {
	if r.readyErr != nil {
		return r.readyErr
	}
	return r.Repository.Ready(context.Background())
}

func (r *httpBoundaryRepository) User(context.Context) (domain.User, error) {
	if r.userErr != nil {
		return domain.User{}, r.userErr
	}
	return r.Repository.User(context.Background())
}

func (r *httpBoundaryRepository) AllMistakes(context.Context) ([]domain.Mistake, error) {
	if r.allMistakesErr != nil {
		return nil, r.allMistakesErr
	}
	return r.Repository.AllMistakes(context.Background())
}

func (r *httpBoundaryRepository) AllVocabulary(context.Context) ([]domain.Vocabulary, error) {
	if r.allVocabularyErr != nil {
		return nil, r.allVocabularyErr
	}
	return r.Repository.AllVocabulary(context.Background())
}

func (r *httpBoundaryRepository) Scenarios(context.Context) ([]domain.RoleplayScenario, error) {
	if r.scenariosErr != nil {
		return nil, r.scenariosErr
	}
	return r.Repository.Scenarios(context.Background())
}

func (r *httpBoundaryRepository) AllSpeakingSessions(context.Context) ([]domain.SpeakingSession, error) {
	if r.allSpeakingErr != nil {
		return nil, r.allSpeakingErr
	}
	return r.Repository.AllSpeakingSessions(context.Background())
}

func (r *httpBoundaryRepository) Usage(context.Context, time.Time) ([]domain.UsageRecord, error) {
	if r.usageErr != nil {
		return nil, r.usageErr
	}
	return r.Repository.Usage(context.Background(), time.Time{})
}

type httpErrorGitHubImporter struct {
	err error
}

func (importer httpErrorGitHubImporter) Import(context.Context, string) (domain.GitHubImport, error) {
	return domain.GitHubImport{}, importer.err
}

type httpNoOptionalProvider struct {
	providerName string
	configured   bool
}

func (provider httpNoOptionalProvider) Name() string     { return provider.providerName }
func (provider httpNoOptionalProvider) Configured() bool { return provider.configured }
func (httpNoOptionalProvider) GenerateMission(context.Context, ai.MissionRequest) (domain.Mission, error) {
	return domain.Mission{ID: "boundary-mission"}, nil
}
func (httpNoOptionalProvider) EvaluateWriting(context.Context, ai.WritingRequest) (domain.Evaluation, error) {
	return domain.Evaluation{}, nil
}
