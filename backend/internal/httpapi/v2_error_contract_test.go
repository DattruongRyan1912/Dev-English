package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

func TestV2MethodAndErrorBoundariesRemainStable(t *testing.T) {
	server, _ := productTestServerWithApp(t)
	handler := server.Handler()

	methodCases := []struct {
		name      string
		path      string
		method    string
		wantAllow string
	}{
		{name: "project collection delete", path: "/api/v2/projects", method: http.MethodDelete, wantAllow: "GET, HEAD, POST"},
		{name: "project detail post", path: "/api/v2/projects/project-1", method: http.MethodPost, wantAllow: "GET, HEAD, PATCH"},
		{name: "knowledge search post", path: "/api/v2/knowledge/search", method: http.MethodPost, wantAllow: "GET, HEAD"},
		{name: "mcp token get", path: "/api/v2/mcp/tokens/token-1", method: http.MethodGet, wantAllow: "DELETE"},
	}
	for _, test := range methodCases {
		t.Run(test.name, func(t *testing.T) {
			response := v2BoundaryRequest(t, handler, test.method, test.path, "", "")
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s returned %d, want 405: %s", test.method, test.path, response.Code, response.Body.String())
			}
			if got := response.Header().Get("Allow"); got != test.wantAllow {
				t.Fatalf("%s %s Allow = %q, want %q", test.method, test.path, got, test.wantAllow)
			}
		})
	}
}

func TestV2ErrorMappingAndCredentialRedaction(t *testing.T) {
	productErrors := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "validation", err: &work.ValidationError{Field: "name", Message: "is required"}, wantStatus: http.StatusBadRequest},
		{name: "version conflict", err: work.ErrVersionConflict, wantStatus: http.StatusConflict},
		{name: "not found", err: knowledge.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "purge precondition", err: work.ErrPurgeNotReady, wantStatus: http.StatusUnprocessableEntity},
		{name: "workspace mismatch", err: knowledge.ErrWorkspaceMismatch, wantStatus: http.StatusForbidden},
		{name: "workspace mapping required", err: application.ErrWorkspaceMappingRequired, wantStatus: http.StatusServiceUnavailable},
		{name: "workspace unavailable", err: application.ErrWorkspaceUnavailable, wantStatus: http.StatusServiceUnavailable},
	}
	for _, test := range productErrors {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeProductError(response, test.err)
			if response.Code != test.wantStatus {
				t.Fatalf("error %v returned %d, want %d", test.err, response.Code, test.wantStatus)
			}
		})
	}

	connectorErrors := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "invalid cursor", err: connectors.ErrInvalidCursor, wantStatus: http.StatusBadRequest},
		{name: "not found", err: connectors.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "revision conflict", err: connectors.ErrRevisionAlreadyExists, wantStatus: http.StatusConflict},
		{name: "provider failure", err: connectors.NewProviderError("drive", "list", http.StatusBadGateway, "temporary"), wantStatus: http.StatusBadGateway},
	}
	for _, test := range connectorErrors {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeConnectorError(response, test.err)
			if response.Code != test.wantStatus {
				t.Fatalf("error %v returned %d, want %d", test.err, response.Code, test.wantStatus)
			}
		})
	}

	secret := "authorization: Bearer super-secret api_key=abc123"
	server, _ := connectorTestServerWithDriveFactoryError(t, errors.New(secret))
	response := v2BoundaryRequest(t, server.Handler(), http.MethodPost, "/api/v2/connectors/drive/sync", `{}`, "")
	if response.Code != http.StatusBadGateway || strings.Contains(response.Body.String(), "super-secret") || strings.Contains(response.Body.String(), "abc123") {
		t.Fatalf("connector error response leaked credentials: %d %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	writeError(response, http.StatusInternalServerError, errors.New(secret))
	if strings.Contains(response.Body.String(), "super-secret") || strings.Contains(response.Body.String(), "abc123") {
		t.Fatalf("generic error response leaked credentials: %s", response.Body.String())
	}
}

func connectorTestServerWithDriveFactoryError(t *testing.T, factoryErr error) (*Server, *connectors.FakeDriveReader) {
	t.Helper()
	server := connectorTestServer(t)
	reader := &connectors.FakeDriveReader{}
	server.DriveSync = func(_ string) (*connectors.DriveSyncService, error) {
		return nil, factoryErr
	}
	return server, reader
}
