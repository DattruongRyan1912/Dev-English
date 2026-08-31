package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

func TestHTTPActionScopeAndServiceFailureBoundaries(t *testing.T) {
	var nilServer *Server
	if _, err := nilServer.actionScope(httptest.NewRequest(http.MethodGet, "/", nil)); err == nil {
		t.Fatal("nil server action scope unexpectedly succeeded")
	}

	wantErr := errors.New("action workspace is unavailable")
	productApp, err := application.New(
		work.NewMemoryRepository(),
		knowledge.NewMemoryRepository(),
		application.NewMemoryConversationRepository(),
		failingWorkspaceEnsurer{err: wantErr},
	)
	if err != nil {
		t.Fatalf("create failing product application: %v", err)
	}

	server, _ := actionTestServer(t)
	server.Application = productApp
	handler := server.Handler()
	cases := []struct {
		name    string
		method  string
		path    string
		body    string
		headers map[string]string
	}{
		{name: "challenge scope", method: http.MethodPost, path: "/api/v2/actions/challenges", body: `{"operation":"github.issue.create","repository":"owner/repo"}`},
		{name: "confirm scope", method: http.MethodPost, path: "/api/v2/actions/confirm", body: `{"challengeId":"challenge-1","operation":"github.issue.create","repository":"owner/repo"}`, headers: map[string]string{"Idempotency-Key": "confirm-scope"}},
		{name: "token scope", method: http.MethodPost, path: "/api/v2/mcp/tokens", body: `{"scopes":["work:read"]}`},
		{name: "revoke scope", method: http.MethodDelete, path: "/api/v2/mcp/tokens/token-1"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			request := httptest.NewRequest(item.method, item.path, strings.NewReader(item.body))
			if item.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			for key, value := range item.headers {
				request.Header.Set(key, value)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), wantErr.Error()) {
				t.Fatalf("returned %d: %s; want 500 containing workspace error", response.Code, response.Body.String())
			}
		})
	}

	server, _ = actionTestServer(t)
	handler = server.Handler()
	missingChallenge := serveBoundaryRequest(handler, http.MethodPost, "/api/v2/actions/confirm", `{"challengeId":"missing-challenge","operation":"github.issue.create","repository":"owner/repo","title":"Missing challenge"}`, map[string]string{
		"Content-Type":    "application/json",
		"Idempotency-Key": "missing-challenge-confirm",
	})
	if missingChallenge.Code != http.StatusNotFound || !strings.Contains(missingChallenge.Body.String(), "challenge") {
		t.Fatalf("missing action challenge returned %d: %s", missingChallenge.Code, missingChallenge.Body.String())
	}

	server.MCPTokenStore = nil
	missingTokenStore := serveBoundaryRequest(server.Handler(), http.MethodDelete, "/api/v2/mcp/tokens/token-1", "", nil)
	if missingTokenStore.Code != http.StatusServiceUnavailable || !strings.Contains(missingTokenStore.Body.String(), "MCP token service") {
		t.Fatalf("missing MCP token store returned %d: %s", missingTokenStore.Code, missingTokenStore.Body.String())
	}
}
