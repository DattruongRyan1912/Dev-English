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
	"github.com/DattruongRyan1912/Dev-English/backend/internal/auth"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learning"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func TestHTTPServerAuthAndCORSFailureBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// The zero-value server is a useful lifecycle boundary: session helpers
	// must initialize their map and remove expired entries without panicking.
	zero := &Server{}
	zero.rememberWebSession("expired-session", now)
	if zero.webSessionActive("expired-session", now.Add(time.Second)) {
		t.Fatal("expired web session was accepted")
	}

	// An enabled auth manager in development still permits the existing
	// bootstrap/browser flow when no bearer token is present.
	development := NewServer(learning.NewService(store.NewSeeded(now), ai.DeterministicProvider{}), logger, auth.New(strings.Repeat("s", 32)))
	development.StrictAuth = false
	if response := serveBoundaryRequest(development.Handler(), http.MethodGet, "/api/v1/home", "", nil); response.Code != http.StatusOK {
		t.Fatalf("development request without token returned %d: %s", response.Code, response.Body.String())
	}

	invalidUser := serveBoundaryRequest(development.Handler(), http.MethodPost, "/api/v1/auth/session", `{"userId":"bad\nid"}`, map[string]string{"Content-Type": "application/json"})
	if invalidUser.Code != http.StatusBadRequest || !strings.Contains(invalidUser.Body.String(), "invalid user id") {
		t.Fatalf("invalid bootstrap user returned %d: %s", invalidUser.Code, invalidUser.Body.String())
	}

	bootstrapManager := auth.New(strings.Repeat("b", 32))
	bootstrapManager.BootstrapKey = "expected-bootstrap-key"
	bootstrap := NewServer(learning.NewService(store.NewSeeded(now), ai.DeterministicProvider{}), logger, bootstrapManager)
	bootstrapResponse := serveBoundaryRequest(bootstrap.Handler(), http.MethodPost, "/api/v1/auth/session", `{"userId":"user-1"}`, map[string]string{
		"Content-Type":    "application/json",
		"X-Bootstrap-Key": "wrong-bootstrap-key",
	})
	if bootstrapResponse.Code != http.StatusForbidden || !strings.Contains(bootstrapResponse.Body.String(), "bootstrap authorization failed") {
		t.Fatalf("bootstrap authorization returned %d: %s", bootstrapResponse.Code, bootstrapResponse.Body.String())
	}

	ensureError := errors.New("user persistence unavailable")
	failingRepository := &httpEnsureUserErrorRepository{Repository: store.NewSeeded(now), err: ensureError}
	failingSession := NewServer(learning.NewService(failingRepository, ai.DeterministicProvider{}), logger)
	failingResponse := serveBoundaryRequest(failingSession.Handler(), http.MethodPost, "/api/v1/auth/session", `{"userId":"user-1"}`, map[string]string{"Content-Type": "application/json"})
	if failingResponse.Code != http.StatusBadRequest || !strings.Contains(failingResponse.Body.String(), ensureError.Error()) {
		t.Fatalf("bootstrap persistence error returned %d: %s", failingResponse.Code, failingResponse.Body.String())
	}

	// Production login has an explicit mode boundary and configuration check.
	loginDevelopment := NewServer(learning.NewService(store.NewSeeded(now), ai.DeterministicProvider{}), logger, auth.New(strings.Repeat("l", 32)))
	loginDevelopment.StrictAuth = false
	if response := serveBoundaryRequest(loginDevelopment.Handler(), http.MethodPost, "/api/v1/auth/login", `{}`, map[string]string{"Content-Type": "application/json"}); response.Code != http.StatusNotFound {
		t.Fatalf("development login returned %d: %s", response.Code, response.Body.String())
	}

	loginMissing := NewServer(learning.NewService(store.NewSeeded(now), ai.DeterministicProvider{}), logger)
	loginMissing.StrictAuth = true
	if response := serveBoundaryRequest(loginMissing.Handler(), http.MethodPost, "/api/v1/auth/login", `{}`, map[string]string{"Content-Type": "application/json"}); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured production login returned %d: %s", response.Code, response.Body.String())
	}

	loginMalformed := NewServer(learning.NewService(store.NewSeeded(now), ai.DeterministicProvider{}), logger, auth.New(strings.Repeat("m", 32)))
	loginMalformed.StrictAuth = true
	loginMalformed.LoginSecret = "login-secret"
	if response := serveBoundaryRequest(loginMalformed.Handler(), http.MethodPost, "/api/v1/auth/login", `{`, map[string]string{"Content-Type": "application/json"}); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed production login returned %d: %s", response.Code, response.Body.String())
	}

	strictCORS := NewServer(learning.NewService(store.NewSeeded(now), ai.DeterministicProvider{}), logger)
	strictCORS.StrictAuth = true
	if response := serveBoundaryRequest(strictCORS.Handler(), http.MethodGet, "/healthz", "", map[string]string{"Origin": "https://app.example.com"}); response.Code != http.StatusForbidden {
		t.Fatalf("strict CORS without allowlist returned %d: %s", response.Code, response.Body.String())
	}

	request := httptest.NewRequest(http.MethodPost, "/boundary", strings.NewReader(`{"value":1} {"value":2}`))
	response := httptest.NewRecorder()
	var target struct {
		Value int `json:"value"`
	}
	if err := decodeJSON(response, request, &target); err == nil || response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "one JSON value") {
		t.Fatalf("trailing JSON was not rejected: err=%v status=%d body=%s", err, response.Code, response.Body.String())
	}
}

type httpEnsureUserErrorRepository struct {
	store.Repository
	err error
}

func (r *httpEnsureUserErrorRepository) EnsureUser(context.Context, domain.User) error {
	return r.err
}

func serveBoundaryRequest(handler http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
