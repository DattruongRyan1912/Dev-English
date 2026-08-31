package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learning"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/platform"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func TestV2BoundaryRoutesFailClosedWhenDependenciesAreMissing(t *testing.T) {
	learningService := learning.NewService(store.NewSeeded(time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)), ai.DeterministicProvider{})
	server := NewServer(learningService, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if response := v2BoundaryRequest(t, server.Handler(), http.MethodGet, "/api/v2/projects", "", ""); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil application returned %d: %s", response.Code, response.Body.String())
	}

	productApp, err := application.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	server.Application = productApp
	handler := server.Handler()
	for _, route := range []string{
		"/api/v2/connectors/drive/sync",
		"/api/v2/connectors/github/sync",
		"/api/v2/learning/observations",
	} {
		method := http.MethodPost
		if route == "/api/v2/learning/observations" {
			method = http.MethodGet
		}
		response := v2BoundaryRequest(t, handler, method, route, "{}", "")
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("missing dependency %s returned %d: %s", route, response.Code, response.Body.String())
		}
	}
}

func TestV2ActionAndMCPTokenValidationFailsClosed(t *testing.T) {
	withoutActions, _ := productTestServerWithApp(t)
	for _, route := range []string{
		"/api/v2/actions/challenges",
		"/api/v2/actions/confirm",
		"/api/v2/mcp/tokens",
	} {
		response := v2BoundaryRequest(t, withoutActions.Handler(), http.MethodPost, route, `{}`, "")
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("unconfigured %s returned %d: %s", route, response.Code, response.Body.String())
		}
	}

	configured, _ := actionTestServer(t)
	invalidTarget := v2BoundaryRequest(t, configured.Handler(), http.MethodPost, "/api/v2/actions/challenges", `{"operation":"github.issue.create","repository":"owner/repo"}`, "")
	if invalidTarget.Code != http.StatusBadRequest || !strings.Contains(invalidTarget.Body.String(), "invalid action request") {
		t.Fatalf("invalid action target returned %d: %s", invalidTarget.Code, invalidTarget.Body.String())
	}
	missingConfirmKey := v2BoundaryRequest(t, configured.Handler(), http.MethodPost, "/api/v2/actions/confirm", `{"challengeId":"challenge-missing","operation":"github.issue.create","repository":"owner/repo","title":"Title"}`, "")
	if missingConfirmKey.Code != http.StatusBadRequest || !strings.Contains(missingConfirmKey.Body.String(), "invalid action request") {
		t.Fatalf("missing action idempotency key returned %d: %s", missingConfirmKey.Code, missingConfirmKey.Body.String())
	}
	unknownScope := v2BoundaryRequest(t, configured.Handler(), http.MethodPost, "/api/v2/mcp/tokens", `{"scopes":["provider:admin"]}`, "")
	if unknownScope.Code != http.StatusBadRequest || !strings.Contains(unknownScope.Body.String(), "MCP token") {
		t.Fatalf("unknown MCP scope returned %d: %s", unknownScope.Code, unknownScope.Body.String())
	}
}

func TestExplicitModuleManifestDisablesOptionalV2AndMCPSurfaces(t *testing.T) {
	server, _ := productTestServerWithApp(t)
	server.Modules = platform.NewManifest(platform.ModulePlatform, platform.ModuleWork)
	handler := server.Handler()

	for _, route := range []string{
		"/api/v2/knowledge/search?q=project",
		"/api/v2/assistant/conversations",
		"/api/v2/actions/challenges",
		"/api/v2/mcp/tokens",
		"/api/v2/projects/project-1/trash",
		"/api/v2/projects/project-1/purge",
		"/mcp",
	} {
		method := http.MethodGet
		if route == "/api/v2/assistant/conversations" || route == "/api/v2/actions/challenges" || route == "/api/v2/mcp/tokens" || strings.HasSuffix(route, "/trash") || strings.HasSuffix(route, "/purge") {
			method = http.MethodPost
		}
		response := v2BoundaryRequest(t, handler, method, route, `{}`, "")
		if response.Code != http.StatusNotFound {
			t.Fatalf("disabled module route %s returned %d: %s", route, response.Code, response.Body.String())
		}
	}

	projects := v2BoundaryRequest(t, handler, http.MethodGet, "/api/v2/projects", "", "")
	if projects.Code != http.StatusOK {
		t.Fatalf("selected work module returned %d: %s", projects.Code, projects.Body.String())
	}
}

func TestBootstrapFiltersDisabledModuleProjections(t *testing.T) {
	server, app := productTestServerWithApp(t)
	ctx := store.WithUser(context.Background(), "bootstrap-module-user")
	if _, err := app.ImportManualSource(ctx, application.ManualSourceInput{
		Name:    "Disabled knowledge source",
		Kind:    "manual",
		URI:     "memory://disabled-knowledge",
		Content: "disabled-source-secret should never appear in a Work-only bootstrap",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Ask(ctx, "", "Keep this disabled assistant conversation out of the bootstrap."); err != nil {
		t.Fatal(err)
	}

	server.Modules = platform.NewManifest(platform.ModulePlatform, platform.ModuleWork)
	response := v2ContextRequest(t, server.Handler(), ctx, http.MethodGet, "/api/v2/bootstrap", "", "")
	if response.Code != http.StatusOK {
		t.Fatalf("module-filtered bootstrap returned %d: %s", response.Code, response.Body.String())
	}
	var bootstrap application.Bootstrap
	if err := json.Unmarshal(response.Body.Bytes(), &bootstrap); err != nil {
		t.Fatal(err)
	}
	if len(bootstrap.Sources) != 0 || bootstrap.Conversation != nil || len(bootstrap.Conversations) != 0 {
		t.Fatalf("disabled bootstrap projections leaked: %+v", bootstrap)
	}
	for module, want := range map[string]bool{
		platform.ModulePlatform:   true,
		platform.ModuleWork:       true,
		platform.ModuleKnowledge:  false,
		platform.ModuleConnectors: false,
		platform.ModuleAssistant:  false,
		platform.ModuleActions:    false,
		platform.ModuleMCP:        false,
		platform.ModuleLearning:   false,
	} {
		if got := bootstrap.Capabilities[module]; got != want {
			t.Fatalf("bootstrap capability %q = %v, want %v; all=%#v", module, got, want, bootstrap.Capabilities)
		}
	}
	if strings.Contains(response.Body.String(), "disabled-source-secret") || strings.Contains(response.Body.String(), "Disabled knowledge source") {
		t.Fatalf("disabled knowledge data leaked into bootstrap: %s", response.Body.String())
	}
}

func TestV2PurgeRoutesExposeSafeLifecycleErrors(t *testing.T) {
	handler := productTestServer(t)
	created := v2BoundaryRequest(t, handler, http.MethodPost, "/api/v2/projects", `{"name":"Purge lifecycle"}`, "purge-project-create")
	if created.Code != http.StatusCreated {
		t.Fatalf("create project returned %d: %s", created.Code, created.Body.String())
	}
	var project struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	v2DecodeContract(t, created, &project)

	activePurge := v2BoundaryRequest(t, handler, http.MethodPost, "/api/v2/projects/"+project.ID+"/purge", `{"expectedVersion":1}`, "purge-active")
	if activePurge.Code != http.StatusServiceUnavailable || !strings.Contains(activePurge.Body.String(), "action service is not configured") {
		t.Fatalf("active purge returned %d: %s", activePurge.Code, activePurge.Body.String())
	}
}

func TestV2PurgeChildRoutesExposeSafeLifecycleErrors(t *testing.T) {
	handler := productTestServer(t)
	projectResponse := v2BoundaryRequest(t, handler, http.MethodPost, "/api/v2/projects", `{"name":"Child purge lifecycle"}`, "child-purge-project")
	if projectResponse.Code != http.StatusCreated {
		t.Fatalf("create project returned %d: %s", projectResponse.Code, projectResponse.Body.String())
	}
	var project struct {
		ID string `json:"id"`
	}
	v2DecodeContract(t, projectResponse, &project)

	taskResponse := v2BoundaryRequest(t, handler, http.MethodPost, "/api/v2/projects/"+project.ID+"/tasks", `{"title":"Child purge task"}`, "child-purge-task-create")
	if taskResponse.Code != http.StatusCreated {
		t.Fatalf("create task returned %d: %s", taskResponse.Code, taskResponse.Body.String())
	}
	var task struct {
		ID string `json:"id"`
	}
	v2DecodeContract(t, taskResponse, &task)
	activeTaskPurge := v2BoundaryRequest(t, handler, http.MethodPost, "/api/v2/tasks/"+task.ID+"/purge", `{"expectedVersion":1}`, "child-purge-task-active")
	if activeTaskPurge.Code != http.StatusServiceUnavailable || !strings.Contains(activeTaskPurge.Body.String(), "action service is not configured") {
		t.Fatalf("active task purge returned %d: %s", activeTaskPurge.Code, activeTaskPurge.Body.String())
	}
}

func TestV2BoundaryRoutesRejectMalformedAndIncompleteRequests(t *testing.T) {
	handler := productTestServer(t)
	cases := []struct {
		name        string
		method      string
		path        string
		body        string
		idempotency string
		wantStatus  int
		wantMessage string
	}{
		{name: "malformed json", method: http.MethodPost, path: "/api/v2/projects", body: `{"name":`, idempotency: "bad-json", wantStatus: http.StatusBadRequest, wantMessage: "valid JSON"},
		{name: "multiple json values", method: http.MethodPost, path: "/api/v2/projects", body: `{"name":"one"}{"name":"two"}`, idempotency: "two-values", wantStatus: http.StatusBadRequest, wantMessage: "one JSON value"},
		{name: "unknown field", method: http.MethodPost, path: "/api/v2/projects", body: `{"name":"project","unexpected":true}`, idempotency: "unknown-field", wantStatus: http.StatusBadRequest, wantMessage: "valid JSON"},
		{name: "missing idempotency key", method: http.MethodPost, path: "/api/v2/projects", body: `{"name":"project"}`, wantStatus: http.StatusBadRequest, wantMessage: "Idempotency-Key"},
		{name: "knowledge import missing idempotency key", method: http.MethodPost, path: "/api/v2/knowledge/sources", body: `{"name":"source","content":"content"}`, wantStatus: http.StatusBadRequest, wantMessage: "Idempotency-Key"},
		{name: "missing search query", method: http.MethodGet, path: "/api/v2/knowledge/search", wantStatus: http.StatusBadRequest, wantMessage: "q query parameter"},
		{name: "blank assistant message", method: http.MethodPost, path: "/api/v2/assistant/conversations", body: `{"message":"  "}`, wantStatus: http.StatusBadRequest, wantMessage: "message is required"},
		{name: "partial assistant context", method: http.MethodPost, path: "/api/v2/assistant/conversations", body: `{"message":"continue","contextType":"task"}`, wantStatus: http.StatusBadRequest, wantMessage: "context"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			response := v2BoundaryRequest(t, handler, item.method, item.path, item.body, item.idempotency)
			if response.Code != item.wantStatus || !strings.Contains(response.Body.String(), item.wantMessage) {
				t.Fatalf("returned %d: %s; want %d containing %q", response.Code, response.Body.String(), item.wantStatus, item.wantMessage)
			}
		})
	}
}

func TestV2MissingResourcesReturnNotFound(t *testing.T) {
	handler := productTestServer(t)
	for _, path := range []string{
		"/api/v2/projects/missing-project",
		"/api/v2/tasks/missing-task",
		"/api/v2/decisions/missing-decision",
		"/api/v2/knowledge/sources/missing-source",
		"/api/v2/knowledge/sources/missing-source/detail",
		"/api/v2/knowledge/items/missing-item",
		"/api/v2/knowledge/revisions/missing-revision",
		"/api/v2/knowledge/chunks/missing-chunk",
		"/api/v2/knowledge/claims/missing-claim",
		"/api/v2/assistant/conversations/missing-conversation",
		"/api/v2/assistant/conversations/missing-conversation/messages",
	} {
		response := v2BoundaryRequest(t, handler, http.MethodGet, path, "", "")
		if path == "/api/v2/assistant/conversations/missing-conversation/messages" {
			response = v2BoundaryRequest(t, handler, http.MethodPost, path, `{"message":"continue"}`, "")
		}
		if response.Code != http.StatusNotFound {
			t.Fatalf("GET/POST %s returned %d: %s", path, response.Code, response.Body.String())
		}
	}
}

func v2BoundaryRequest(t *testing.T, handler http.Handler, method, path, body, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
