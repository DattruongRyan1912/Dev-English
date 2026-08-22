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
	server := NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil)), auth.New(strings.Repeat("s", 32)))
	server.StrictAuth = true
	handler := server.Handler()

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/home", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", unauthorized.Code)
	}

	server.StrictAuth = false
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
	server.StrictAuth = true
	authorizedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/home", nil)
	authorizedRequest.Header.Set("Authorization", "Bearer "+session.Token)
	authorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(authorizedResponse, authorizedRequest)
	if authorizedResponse.Code != http.StatusOK {
		t.Fatalf("got authorized status %d: %s", authorizedResponse.Code, authorizedResponse.Body.String())
	}
}

func TestProductionUsesHttpOnlyCookieSessionAndRevokesItOnLogout(t *testing.T) {
	memory := store.NewSeeded(time.Now().UTC())
	service := learning.NewService(memory, ai.DeterministicProvider{})
	server := NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil)), auth.New(strings.Repeat("s", 32)))
	server.StrictAuth = true
	server.LoginSecret = "production-login-secret"
	handler := server.Handler()

	anonymous := httptest.NewRecorder()
	handler.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/api/v1/home", nil))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous request returned %d, want 401", anonymous.Code)
	}

	invalidLogin := httptest.NewRecorder()
	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"secret":"wrong"}`))
	invalidRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(invalidLogin, invalidRequest)
	if invalidLogin.Code != http.StatusUnauthorized {
		t.Fatalf("invalid login returned %d, want 401", invalidLogin.Code)
	}

	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"secret":"production-login-secret"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusOK {
		t.Fatalf("login returned %d: %s", login.Code, login.Body.String())
	}
	if strings.Contains(login.Body.String(), `"token"`) {
		t.Fatalf("production login must not return a bearer token: %s", login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("login did not issue a secure HttpOnly Strict cookie: %#v", cookies)
	}
	cookieHeader := cookies[0].String()

	authenticated := httptest.NewRequest(http.MethodGet, "/api/v1/home", nil)
	authenticated.Header.Set("Cookie", cookieHeader)
	authenticatedResponse := httptest.NewRecorder()
	handler.ServeHTTP(authenticatedResponse, authenticated)
	if authenticatedResponse.Code != http.StatusOK {
		t.Fatalf("cookie-authenticated request returned %d: %s", authenticatedResponse.Code, authenticatedResponse.Body.String())
	}

	reload := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	reload.Header.Set("Cookie", cookieHeader)
	reloadResponse := httptest.NewRecorder()
	handler.ServeHTTP(reloadResponse, reload)
	if reloadResponse.Code != http.StatusOK || !strings.Contains(reloadResponse.Body.String(), `"id":"user-1"`) {
		t.Fatalf("session reload returned %d: %s", reloadResponse.Code, reloadResponse.Body.String())
	}

	logout := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logout.Header.Set("Cookie", cookieHeader)
	logoutResponse := httptest.NewRecorder()
	handler.ServeHTTP(logoutResponse, logout)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logout returned %d: %s", logoutResponse.Code, logoutResponse.Body.String())
	}
	logoutCookies := logoutResponse.Result().Cookies()
	if len(logoutCookies) != 1 || logoutCookies[0].MaxAge != -1 {
		t.Fatalf("logout did not clear the session cookie: %#v", logoutCookies)
	}

	revoked := httptest.NewRequest(http.MethodGet, "/api/v1/home", nil)
	revoked.Header.Set("Cookie", cookieHeader)
	revokedResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokedResponse, revoked)
	if revokedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session returned %d, want 401", revokedResponse.Code)
	}

	bootstrap := httptest.NewRecorder()
	bootstrapRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/session", strings.NewReader(`{"userId":"user-1"}`))
	bootstrapRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(bootstrap, bootstrapRequest)
	if bootstrap.Code != http.StatusNotFound {
		t.Fatalf("production bootstrap endpoint returned %d, want 404", bootstrap.Code)
	}
}
