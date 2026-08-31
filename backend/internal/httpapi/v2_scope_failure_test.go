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
	"github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learning"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learningoverlay"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/mcp"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

func TestV2RoutesFailClosedWhenWorkspaceInitializationFails(t *testing.T) {
	wantErr := errors.New("workspace initialization failed")
	productApp, err := application.New(
		work.NewMemoryRepository(),
		knowledge.NewMemoryRepository(),
		application.NewMemoryConversationRepository(),
		failingWorkspaceEnsurer{err: wantErr},
	)
	if err != nil {
		t.Fatalf("create product application: %v", err)
	}
	overlay, err := learningoverlay.NewService(learningoverlay.NewMemoryRepository())
	if err != nil {
		t.Fatalf("create learning overlay: %v", err)
	}
	server := NewServer(
		learning.NewService(store.NewSeeded(time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)), ai.DeterministicProvider{}),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	server.Application = productApp
	server.LearningOverlay = overlay
	server.MCPTokenStore = mcp.NewTokenStore()
	handler := server.Handler()

	cases := []struct {
		name           string
		method         string
		path           string
		body           string
		idempotencyKey string
	}{
		{name: "bootstrap", method: http.MethodGet, path: "/api/v2/bootstrap"},
		{name: "project list", method: http.MethodGet, path: "/api/v2/projects"},
		{name: "project detail", method: http.MethodGet, path: "/api/v2/projects/project-1"},
		{name: "project create", method: http.MethodPost, path: "/api/v2/projects", body: `{"name":"Project"}`, idempotencyKey: "scope-project-create"},
		{name: "project update", method: http.MethodPatch, path: "/api/v2/projects/project-1", body: `{"name":"Updated","expectedVersion":1}`, idempotencyKey: "scope-project-update"},
		{name: "project trash", method: http.MethodPost, path: "/api/v2/projects/project-1/trash", body: `{"expectedVersion":1}`, idempotencyKey: "scope-project-trash"},
		{name: "project restore", method: http.MethodPost, path: "/api/v2/projects/project-1/restore", body: `{"expectedVersion":1}`, idempotencyKey: "scope-project-restore"},
		{name: "project purge", method: http.MethodPost, path: "/api/v2/projects/project-1/purge", body: `{"expectedVersion":1}`, idempotencyKey: "scope-project-purge"},
		{name: "task list", method: http.MethodGet, path: "/api/v2/tasks"},
		{name: "task detail", method: http.MethodGet, path: "/api/v2/tasks/task-1"},
		{name: "task create", method: http.MethodPost, path: "/api/v2/projects/project-1/tasks", body: `{"title":"Task"}`, idempotencyKey: "scope-task-create"},
		{name: "task update", method: http.MethodPatch, path: "/api/v2/tasks/task-1", body: `{"title":"Updated","expectedVersion":1}`, idempotencyKey: "scope-task-update"},
		{name: "task trash", method: http.MethodPost, path: "/api/v2/tasks/task-1/trash", body: `{"expectedVersion":1}`, idempotencyKey: "scope-task-trash"},
		{name: "task restore", method: http.MethodPost, path: "/api/v2/tasks/task-1/restore", body: `{"expectedVersion":1}`, idempotencyKey: "scope-task-restore"},
		{name: "task purge", method: http.MethodPost, path: "/api/v2/tasks/task-1/purge", body: `{"expectedVersion":1}`, idempotencyKey: "scope-task-purge"},
		{name: "decision list", method: http.MethodGet, path: "/api/v2/decisions"},
		{name: "decision detail", method: http.MethodGet, path: "/api/v2/decisions/decision-1"},
		{name: "decision create", method: http.MethodPost, path: "/api/v2/decisions", body: `{"title":"Decision","outcome":"Keep it reversible"}`, idempotencyKey: "scope-decision-create"},
		{name: "decision update", method: http.MethodPatch, path: "/api/v2/decisions/decision-1", body: `{"title":"Updated","expectedVersion":1}`, idempotencyKey: "scope-decision-update"},
		{name: "decision trash", method: http.MethodPost, path: "/api/v2/decisions/decision-1/trash", body: `{"expectedVersion":1}`, idempotencyKey: "scope-decision-trash"},
		{name: "decision restore", method: http.MethodPost, path: "/api/v2/decisions/decision-1/restore", body: `{"expectedVersion":1}`, idempotencyKey: "scope-decision-restore"},
		{name: "decision purge", method: http.MethodPost, path: "/api/v2/decisions/decision-1/purge", body: `{"expectedVersion":1}`, idempotencyKey: "scope-decision-purge"},
		{name: "task history", method: http.MethodGet, path: "/api/v2/tasks/task-1/history"},
		{name: "project history", method: http.MethodGet, path: "/api/v2/projects/project-1/history"},
		{name: "decision history", method: http.MethodGet, path: "/api/v2/decisions/decision-1/history"},
		{name: "source import", method: http.MethodPost, path: "/api/v2/knowledge/sources", body: `{"name":"Notes","content":"Source-backed notes."}`, idempotencyKey: "scope-source-import"},
		{name: "knowledge sources", method: http.MethodGet, path: "/api/v2/knowledge/sources"},
		{name: "knowledge source", method: http.MethodGet, path: "/api/v2/knowledge/sources/source-1"},
		{name: "knowledge detail", method: http.MethodGet, path: "/api/v2/knowledge/sources/source-1/detail"},
		{name: "knowledge item", method: http.MethodGet, path: "/api/v2/knowledge/items/item-1"},
		{name: "knowledge revision", method: http.MethodGet, path: "/api/v2/knowledge/revisions/revision-1"},
		{name: "knowledge chunk", method: http.MethodGet, path: "/api/v2/knowledge/chunks/chunk-1"},
		{name: "knowledge claim", method: http.MethodGet, path: "/api/v2/knowledge/claims/claim-1"},
		{name: "knowledge search", method: http.MethodGet, path: "/api/v2/knowledge/search?q=release"},
		{name: "learning record", method: http.MethodPost, path: "/api/v2/learning/observations", body: `{"sourceType":"task","sourceId":"task-1","skill":"technical_writing","prompt":"Explain the next step.","response":"I will inspect the logs."}`},
		{name: "learning list", method: http.MethodGet, path: "/api/v2/learning/observations"},
		{name: "conversation start", method: http.MethodPost, path: "/api/v2/assistant/conversations", body: `{"message":"What is next?"}`},
		{name: "conversation list", method: http.MethodGet, path: "/api/v2/assistant/conversations"},
		{name: "conversation detail", method: http.MethodGet, path: "/api/v2/assistant/conversations/conversation-1"},
		{name: "conversation message", method: http.MethodPost, path: "/api/v2/assistant/conversations/conversation-1/messages", body: `{"message":"Continue."}`},
		{name: "mcp token revoke", method: http.MethodDelete, path: "/api/v2/mcp/tokens/token-1"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			response := v2BoundaryRequest(t, handler, item.method, item.path, item.body, item.idempotencyKey)
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("returned %d: %s; want 500", response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), wantErr.Error()) {
				t.Fatalf("response omitted workspace error: %s", response.Body.String())
			}
		})
	}
}

func TestV2BootstrapReturnsApplicationError(t *testing.T) {
	wantErr := errors.New("bootstrap workspace initialization failed")
	productApp, err := application.New(
		work.NewMemoryRepository(),
		knowledge.NewMemoryRepository(),
		application.NewMemoryConversationRepository(),
		failingWorkspaceEnsurer{err: wantErr},
	)
	if err != nil {
		t.Fatalf("create product application: %v", err)
	}
	server := NewServer(
		learning.NewService(store.NewSeeded(time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)), ai.DeterministicProvider{}),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	server.Application = productApp

	request := httptest.NewRequest(http.MethodGet, "/api/v2/bootstrap", nil)
	response := httptest.NewRecorder()
	server.v2Bootstrap(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("bootstrap returned %d: %s; want 500", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), wantErr.Error()) {
		t.Fatalf("bootstrap omitted application error: %s", response.Body.String())
	}
}

func TestV2ProjectsReturnsRepositoryError(t *testing.T) {
	wantErr := errors.New("project list failed")
	workRepository := &failingProjectListRepository{MemoryRepository: work.NewMemoryRepository(), err: wantErr}
	productApp, err := application.New(
		workRepository,
		knowledge.NewMemoryRepository(),
		application.NewMemoryConversationRepository(),
		nil,
	)
	if err != nil {
		t.Fatalf("create product application: %v", err)
	}
	server := NewServer(
		learning.NewService(store.NewSeeded(time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)), ai.DeterministicProvider{}),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	server.Application = productApp

	request := httptest.NewRequest(http.MethodGet, "/api/v2/projects", nil)
	response := httptest.NewRecorder()
	server.v2Projects(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("project list returned %d: %s; want 500", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), wantErr.Error()) {
		t.Fatalf("project list omitted repository error: %s", response.Body.String())
	}
}

type failingProjectListRepository struct {
	*work.MemoryRepository
	err error
}

func (r *failingProjectListRepository) ListProjects(context.Context, work.Scope, work.ListOptions) ([]work.Project, error) {
	return nil, r.err
}

type failingWorkspaceEnsurer struct {
	err error
}

func (e failingWorkspaceEnsurer) EnsureWorkspace(_ context.Context, _, _ string) error {
	return e.err
}
