package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learning"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

// Keep every V2 route fail-closed when the product application has not been
// wired. This protects the boundary during partial startup and keeps route
// registration from silently turning into a nil-pointer panic.
func TestV2RoutesFailClosedWhenProductApplicationIsMissing(t *testing.T) {
	service := learning.NewService(
		store.NewSeeded(time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)),
		ai.DeterministicProvider{},
	)
	server := NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := server.Handler()

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "bootstrap", method: http.MethodGet, path: "/api/v2/bootstrap"},
		{name: "project list", method: http.MethodGet, path: "/api/v2/projects"},
		{name: "project create", method: http.MethodPost, path: "/api/v2/projects"},
		{name: "project detail", method: http.MethodGet, path: "/api/v2/projects/project-1"},
		{name: "project update", method: http.MethodPatch, path: "/api/v2/projects/project-1"},
		{name: "project trash", method: http.MethodPost, path: "/api/v2/projects/project-1/trash"},
		{name: "project restore", method: http.MethodPost, path: "/api/v2/projects/project-1/restore"},
		{name: "project purge", method: http.MethodPost, path: "/api/v2/projects/project-1/purge"},
		{name: "project history", method: http.MethodGet, path: "/api/v2/projects/project-1/history"},
		{name: "task list", method: http.MethodGet, path: "/api/v2/tasks"},
		{name: "task create", method: http.MethodPost, path: "/api/v2/projects/project-1/tasks"},
		{name: "task detail", method: http.MethodGet, path: "/api/v2/tasks/task-1"},
		{name: "task update", method: http.MethodPatch, path: "/api/v2/tasks/task-1"},
		{name: "task trash", method: http.MethodPost, path: "/api/v2/tasks/task-1/trash"},
		{name: "task restore", method: http.MethodPost, path: "/api/v2/tasks/task-1/restore"},
		{name: "task purge", method: http.MethodPost, path: "/api/v2/tasks/task-1/purge"},
		{name: "task history", method: http.MethodGet, path: "/api/v2/tasks/task-1/history"},
		{name: "decision list", method: http.MethodGet, path: "/api/v2/decisions"},
		{name: "decision create", method: http.MethodPost, path: "/api/v2/decisions"},
		{name: "decision detail", method: http.MethodGet, path: "/api/v2/decisions/decision-1"},
		{name: "decision update", method: http.MethodPatch, path: "/api/v2/decisions/decision-1"},
		{name: "decision trash", method: http.MethodPost, path: "/api/v2/decisions/decision-1/trash"},
		{name: "decision restore", method: http.MethodPost, path: "/api/v2/decisions/decision-1/restore"},
		{name: "decision purge", method: http.MethodPost, path: "/api/v2/decisions/decision-1/purge"},
		{name: "decision history", method: http.MethodGet, path: "/api/v2/decisions/decision-1/history"},
		{name: "source import", method: http.MethodPost, path: "/api/v2/knowledge/sources"},
		{name: "source list", method: http.MethodGet, path: "/api/v2/knowledge/sources"},
		{name: "source detail", method: http.MethodGet, path: "/api/v2/knowledge/sources/source-1"},
		{name: "source detail view", method: http.MethodGet, path: "/api/v2/knowledge/sources/source-1/detail"},
		{name: "knowledge item", method: http.MethodGet, path: "/api/v2/knowledge/items/item-1"},
		{name: "knowledge revision", method: http.MethodGet, path: "/api/v2/knowledge/revisions/revision-1"},
		{name: "knowledge chunk", method: http.MethodGet, path: "/api/v2/knowledge/chunks/chunk-1"},
		{name: "knowledge claim", method: http.MethodGet, path: "/api/v2/knowledge/claims/claim-1"},
		{name: "knowledge search", method: http.MethodGet, path: "/api/v2/knowledge/search?q=release"},
		{name: "drive sync", method: http.MethodPost, path: "/api/v2/connectors/drive/sync"},
		{name: "github sync", method: http.MethodPost, path: "/api/v2/connectors/github/sync"},
		{name: "learning record", method: http.MethodPost, path: "/api/v2/learning/observations"},
		{name: "learning list", method: http.MethodGet, path: "/api/v2/learning/observations"},
		{name: "conversation start", method: http.MethodPost, path: "/api/v2/assistant/conversations"},
		{name: "conversation list", method: http.MethodGet, path: "/api/v2/assistant/conversations"},
		{name: "conversation detail", method: http.MethodGet, path: "/api/v2/assistant/conversations/conversation-1"},
		{name: "conversation message", method: http.MethodPost, path: "/api/v2/assistant/conversations/conversation-1/messages"},
		{name: "action challenge", method: http.MethodPost, path: "/api/v2/actions/challenges"},
		{name: "action confirm", method: http.MethodPost, path: "/api/v2/actions/confirm"},
		{name: "MCP token issue", method: http.MethodPost, path: "/api/v2/mcp/tokens"},
		{name: "MCP token revoke", method: http.MethodDelete, path: "/api/v2/mcp/tokens/token-1"},
	}

	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			request := httptest.NewRequest(item.method, item.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("%s %s returned %d: %s; want 503", item.method, item.path, response.Code, response.Body.String())
			}
		})
	}
}
