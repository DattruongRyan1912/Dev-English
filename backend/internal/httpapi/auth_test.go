package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/auth"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learning"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func TestAuthMiddlewareScopesRequestsToBearerIdentity(t *testing.T) {
	memory := store.NewSeeded(time.Now().UTC())
	service := learning.NewService(memory, ai.DeterministicProvider{})
	handler := NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil)), auth.New(strings.Repeat("s", 32))).Handler()

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/home", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", unauthorized.Code)
	}

	sessionBody := strings.NewReader(`{"userId":"user-1","displayName":"Developer"}`)
	sessionRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/session", sessionBody)
	sessionRequest.Header.Set("Content-Type", "application/json")
	sessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(sessionResponse, sessionRequest)
	if sessionResponse.Code != http.StatusOK || !strings.Contains(sessionResponse.Body.String(), "token") {
		t.Fatalf("unexpected session response: %d %s", sessionResponse.Code, sessionResponse.Body.String())
	}
	var session struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(sessionResponse.Body.Bytes(), &session); err != nil || session.Token == "" {
		t.Fatalf("invalid session token response: %v", err)
	}
	authorizedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/home", nil)
	authorizedRequest.Header.Set("Authorization", "Bearer "+session.Token)
	authorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(authorizedResponse, authorizedRequest)
	if authorizedResponse.Code != http.StatusOK {
		t.Fatalf("got authorized status %d: %s", authorizedResponse.Code, authorizedResponse.Body.String())
	}
}
