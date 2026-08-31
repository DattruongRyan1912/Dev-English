package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestV2MissingMutationTargetsRemainNotFound(t *testing.T) {
	server, _ := productTestServerWithApp(t)
	handler := server.Handler()

	cases := []struct {
		name           string
		method         string
		path           string
		body           string
		idempotencyKey string
	}{
		{name: "project update", method: http.MethodPatch, path: "/api/v2/projects/missing-project", body: `{"name":"Updated","expectedVersion":1}`, idempotencyKey: "missing-project-update"},
		{name: "project trash", method: http.MethodPost, path: "/api/v2/projects/missing-project/trash", body: `{"expectedVersion":1}`, idempotencyKey: "missing-project-trash"},
		{name: "project restore", method: http.MethodPost, path: "/api/v2/projects/missing-project/restore", body: `{"expectedVersion":1}`, idempotencyKey: "missing-project-restore"},
		{name: "project purge", method: http.MethodPost, path: "/api/v2/projects/missing-project/purge", body: `{"expectedVersion":1}`, idempotencyKey: "missing-project-purge"},
		{name: "task update", method: http.MethodPatch, path: "/api/v2/tasks/missing-task", body: `{"title":"Updated","expectedVersion":1}`, idempotencyKey: "missing-task-update"},
		{name: "task trash", method: http.MethodPost, path: "/api/v2/tasks/missing-task/trash", body: `{"expectedVersion":1}`, idempotencyKey: "missing-task-trash"},
		{name: "task purge", method: http.MethodPost, path: "/api/v2/tasks/missing-task/purge", body: `{"expectedVersion":1}`, idempotencyKey: "missing-task-purge"},
		{name: "decision update", method: http.MethodPatch, path: "/api/v2/decisions/missing-decision", body: `{"title":"Updated","expectedVersion":1}`, idempotencyKey: "missing-decision-update"},
		{name: "decision trash", method: http.MethodPost, path: "/api/v2/decisions/missing-decision/trash", body: `{"expectedVersion":1}`, idempotencyKey: "missing-decision-trash"},
		{name: "decision purge", method: http.MethodPost, path: "/api/v2/decisions/missing-decision/purge", body: `{"expectedVersion":1}`, idempotencyKey: "missing-decision-purge"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			response := v2BoundaryRequest(t, handler, item.method, item.path, item.body, item.idempotencyKey)
			if strings.Contains(item.name, "purge") {
				if response.Code != http.StatusServiceUnavailable {
					t.Fatalf("returned %d: %s; want 503", response.Code, response.Body.String())
				}
				if !strings.Contains(response.Body.String(), "action service is not configured") {
					t.Fatalf("response omitted fail-closed action boundary: %s", response.Body.String())
				}
				return
			}
			if response.Code != http.StatusNotFound {
				t.Fatalf("returned %d: %s; want 404", response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), "not found") {
				t.Fatalf("response omitted not-found boundary: %s", response.Body.String())
			}
		})
	}
}
