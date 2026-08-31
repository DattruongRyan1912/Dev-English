package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/actions"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learning"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/mcp"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

func TestActionHTTPBoundaryHelpersCoverValidationAndErrorMapping(t *testing.T) {
	target := (actionTargetRequest{
		Operation:        " github.issue.create ",
		Repository:       " owner/repo ",
		Issue:            12,
		Title:            "Title",
		Body:             "Body",
		Labels:           []string{"bug"},
		ExpectedRevision: " revision-1 ",
	}).target()
	if target.Operation != connectors.SafeWriteOperation("github.issue.create") {
		t.Fatalf("target operation = %q", target.Operation)
	}
	response := newActionTargetResponse(target)
	if response.Repository != "owner/repo" || response.ExpectedRevision != "revision-1" || len(response.Labels) != 1 {
		t.Fatalf("target response normalization = %#v", response)
	}
	target.Labels[0] = "changed"
	if response.Labels[0] != "bug" {
		t.Fatal("target response shares mutable labels with the input")
	}

	keyCases := []struct {
		name     string
		body     string
		header   string
		required bool
		want     string
		wantErr  error
	}{
		{name: "optional empty", want: ""},
		{name: "body", body: "body-key", want: "body-key"},
		{name: "header", header: "header-key", want: "header-key"},
		{name: "matching sources", body: "same", header: "same", want: "same"},
		{name: "required empty", required: true, wantErr: errActionIdempotencyRequired},
		{name: "body whitespace", body: " body-key", wantErr: errInvalidActionIdempotencyKey},
		{name: "header whitespace", header: " header-key", wantErr: errInvalidActionIdempotencyKey},
		{name: "sources differ", body: "one", header: "two", wantErr: errInvalidActionIdempotencyKey},
		{name: "control character", body: "bad\nkey", wantErr: errInvalidActionIdempotencyKey},
		{name: "too long", body: strings.Repeat("x", 257), wantErr: errInvalidActionIdempotencyKey},
	}
	for _, item := range keyCases {
		t.Run("key/"+item.name, func(t *testing.T) {
			got, err := actionIdempotencyKey(item.body, item.header, item.required)
			if item.wantErr != nil {
				if !errors.Is(err, item.wantErr) {
					t.Fatalf("error = %v, want %v", err, item.wantErr)
				}
				return
			}
			if err != nil || got != item.want {
				t.Fatalf("result = %q, %v; want %q", got, err, item.want)
			}
		})
	}

	actionErrors := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "invalid request", err: actions.ErrInvalidRequest, wantStatus: http.StatusBadRequest},
		{name: "challenge missing", err: actions.ErrChallengeNotFound, wantStatus: http.StatusNotFound},
		{name: "challenge mismatch", err: actions.ErrChallengeMismatch, wantStatus: http.StatusConflict},
		{name: "receipt persistence", err: connectors.ErrReceiptPersistence, wantStatus: http.StatusServiceUnavailable},
		{name: "receipt uncertain", err: connectors.ErrReceiptUncertain, wantStatus: http.StatusConflict},
		{name: "provider payload", err: connectors.ErrInvalidProviderPayload, wantStatus: http.StatusBadGateway},
		{name: "missing confirmation user", err: connectors.ErrMissingUserID, wantStatus: http.StatusBadRequest},
		{name: "unknown", err: errors.New("sensitive provider detail"), wantStatus: http.StatusInternalServerError},
	}
	for _, item := range actionErrors {
		t.Run("action-error/"+item.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeActionError(response, item.err)
			if response.Code != item.wantStatus || strings.Contains(response.Body.String(), "sensitive provider detail") {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
		})
	}

	mcpErrors := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "unknown scope", err: mcp.ErrUnknownScope, wantStatus: http.StatusBadRequest},
		{name: "persistence", err: mcp.ErrTokenPersistence, wantStatus: http.StatusServiceUnavailable},
		{name: "unknown", err: errors.New("MCP internal detail"), wantStatus: http.StatusInternalServerError},
	}
	for _, item := range mcpErrors {
		t.Run("mcp-error/"+item.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeMCPTokenError(response, item.err)
			if response.Code != item.wantStatus || strings.Contains(response.Body.String(), "MCP internal detail") {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestActionHTTPHandlersFailClosedForMissingServicesAndMalformedInput(t *testing.T) {
	server, _ := actionTestServer(t)
	handler := server.Handler()

	server.Actions = nil
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v2/actions/challenges", strings.NewReader(`{}`)))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing action service returned %d: %s", response.Code, response.Body.String())
	}

	server, _ = actionTestServer(t)
	handler = server.Handler()
	for _, item := range []struct {
		name string
		path string
		body string
	}{
		{name: "challenge malformed", path: "/api/v2/actions/challenges", body: "{"},
		{name: "challenge invalid key", path: "/api/v2/actions/challenges", body: `{"idempotencyKey":" bad"}`},
		{name: "confirm missing key", path: "/api/v2/actions/confirm", body: `{"challengeId":"challenge-1"}`},
		{name: "token malformed", path: "/api/v2/mcp/tokens", body: "{"},
		{name: "token empty scopes", path: "/api/v2/mcp/tokens", body: `{}`},
	} {
		t.Run(item.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, item.path, strings.NewReader(item.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("returned %d: %s", response.Code, response.Body.String())
			}
		})
	}

	server.MCPTokenStore = nil
	response = httptest.NewRecorder()
	handler = server.Handler()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v2/mcp/tokens", strings.NewReader(`{"scopes":["work:read"]}`)))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing token service returned %d: %s", response.Code, response.Body.String())
	}
}

func TestActionHTTPFlowRequiresChallengeAndReturnsStableReplayReceipt(t *testing.T) {
	server, writer := actionTestServer(t)
	handler := server.Handler()

	challengeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/actions/challenges", strings.NewReader(`{"operation":"github.issue.create","repository":"owner/repo","title":"Add action audit","body":"Keep the write bounded."}`))
	challengeRequest.Header.Set("Content-Type", "application/json")
	challengeRequest.Header.Set("Idempotency-Key", "preview-http-1")
	challengeResponse := httptest.NewRecorder()
	handler.ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusCreated {
		t.Fatalf("challenge returned %d: %s", challengeResponse.Code, challengeResponse.Body.String())
	}
	var challenge struct {
		Action struct {
			ChallengeID string `json:"challengeId"`
		} `json:"action"`
	}
	if err := json.Unmarshal(challengeResponse.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	if challenge.Action.ChallengeID == "" || strings.Contains(challengeResponse.Body.String(), "token") {
		t.Fatalf("challenge response is unsafe or incomplete: %s", challengeResponse.Body.String())
	}

	confirmBody := `{"challengeId":"` + challenge.Action.ChallengeID + `","operation":"github.issue.create","repository":"owner/repo","title":"Add action audit","body":"Keep the write bounded."}`
	confirmRequest := httptest.NewRequest(http.MethodPost, "/api/v2/actions/confirm", strings.NewReader(confirmBody))
	confirmRequest.Header.Set("Content-Type", "application/json")
	confirmRequest.Header.Set("Idempotency-Key", "confirm-http-1")
	confirmResponse := httptest.NewRecorder()
	handler.ServeHTTP(confirmResponse, confirmRequest)
	if confirmResponse.Code != http.StatusOK || !strings.Contains(confirmResponse.Body.String(), `"status":"accepted"`) || strings.Contains(confirmResponse.Body.String(), "Keep the write bounded.") {
		t.Fatalf("confirm returned %d: %s", confirmResponse.Code, confirmResponse.Body.String())
	}

	replayRequest := httptest.NewRequest(http.MethodPost, "/api/v2/actions/confirm", strings.NewReader(confirmBody))
	replayRequest.Header.Set("Content-Type", "application/json")
	replayRequest.Header.Set("Idempotency-Key", "confirm-http-1")
	replayResponse := httptest.NewRecorder()
	handler.ServeHTTP(replayResponse, replayRequest)
	if replayResponse.Code != http.StatusOK || !strings.Contains(replayResponse.Body.String(), `"replayed":true`) {
		t.Fatalf("replay returned %d: %s", replayResponse.Code, replayResponse.Body.String())
	}
	if len(writer.CreateCalls) != 1 {
		t.Fatalf("GitHub provider calls = %d, want 1", len(writer.CreateCalls))
	}
}

func TestActionHTTPFlowConfirmsCanonicalWorkTrash(t *testing.T) {
	server, _ := actionTestServer(t)
	scope := work.Scope{WorkspaceID: application.WorkspaceIDForUser("user-1"), UserID: "user-1"}
	project, err := server.Application.Work.CreateProject(context.Background(), scope, work.CreateProjectInput{Name: "HTTP action target"}, "http-work-create")
	if err != nil {
		t.Fatal(err)
	}
	challengeBody := `{"operation":"work.entity.trash","entityType":"project","entityId":"` + project.ID + `","expectedVersion":1}`
	challengeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/actions/challenges", strings.NewReader(challengeBody))
	challengeRequest.Header.Set("Content-Type", "application/json")
	challengeRequest.Header.Set("Idempotency-Key", "http-work-preview")
	challengeResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusCreated {
		t.Fatalf("work challenge returned %d: %s", challengeResponse.Code, challengeResponse.Body.String())
	}
	var challenge struct {
		Action struct {
			ChallengeID string `json:"challengeId"`
		} `json:"action"`
	}
	if err := json.Unmarshal(challengeResponse.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	if challenge.Action.ChallengeID == "" {
		t.Fatalf("work challenge omitted challenge id: %s", challengeResponse.Body.String())
	}
	confirmBody := `{"challengeId":"` + challenge.Action.ChallengeID + `","operation":"work.entity.trash","entityType":"project","entityId":"` + project.ID + `","expectedVersion":1}`
	confirmRequest := httptest.NewRequest(http.MethodPost, "/api/v2/actions/confirm", strings.NewReader(confirmBody))
	confirmRequest.Header.Set("Content-Type", "application/json")
	confirmRequest.Header.Set("Idempotency-Key", "http-work-trash")
	confirmResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(confirmResponse, confirmRequest)
	if confirmResponse.Code != http.StatusOK || !strings.Contains(confirmResponse.Body.String(), `"provider":"work"`) || !strings.Contains(confirmResponse.Body.String(), `"status":"accepted"`) {
		t.Fatalf("work confirm returned %d: %s", confirmResponse.Code, confirmResponse.Body.String())
	}
	replayRequest := httptest.NewRequest(http.MethodPost, "/api/v2/actions/confirm", strings.NewReader(confirmBody))
	replayRequest.Header.Set("Content-Type", "application/json")
	replayRequest.Header.Set("Idempotency-Key", "http-work-trash")
	replayResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(replayResponse, replayRequest)
	if replayResponse.Code != http.StatusOK || !strings.Contains(replayResponse.Body.String(), `"replayed":true`) {
		t.Fatalf("work replay returned %d: %s", replayResponse.Code, replayResponse.Body.String())
	}
	if _, err := server.Application.Work.GetProject(context.Background(), scope, project.ID); !errors.Is(err, work.ErrNotFound) {
		t.Fatalf("work project lookup after confirmation = %v, want ErrNotFound", err)
	}
}

func TestConfiguredPurgeRouteRequiresChallengeAndReplaysReceipt(t *testing.T) {
	server, _ := actionTestServer(t)
	clock := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	workService, err := work.NewService(work.NewMemoryRepository(), work.WithClock(func() time.Time { return clock }))
	if err != nil {
		t.Fatal(err)
	}
	server.Application.Work = workService
	server.Actions.Work = workService
	server.Actions.Clock = func() time.Time { return clock }
	handler := server.Handler()
	scope := work.Scope{WorkspaceID: application.WorkspaceIDForUser("user-1"), UserID: "user-1"}
	project, err := workService.CreateProject(context.Background(), scope, work.CreateProjectInput{Name: "Protected purge target"}, "protected-purge-create")
	if err != nil {
		t.Fatal(err)
	}

	missingChallenge := serveBoundaryRequest(handler, http.MethodPost, "/api/v2/projects/"+project.ID+"/purge", `{"expectedVersion":1}`, map[string]string{
		"Content-Type":    "application/json",
		"Idempotency-Key": "protected-purge-missing-challenge",
	})
	if missingChallenge.Code != http.StatusBadRequest || !strings.Contains(missingChallenge.Body.String(), "invalid action request") {
		t.Fatalf("purge without challenge returned %d: %s", missingChallenge.Code, missingChallenge.Body.String())
	}

	trashed, err := workService.TrashProject(context.Background(), scope, project.ID, project.Version, "protected-purge-trash")
	if err != nil {
		t.Fatal(err)
	}
	clock = trashed.DeletedAt.Add(work.RetentionPeriod + time.Hour)
	challenge := serveBoundaryRequest(handler, http.MethodPost, "/api/v2/actions/challenges", `{"operation":"work.entity.purge","entityType":"project","entityId":"`+project.ID+`","expectedVersion":2}`, map[string]string{
		"Content-Type":    "application/json",
		"Idempotency-Key": "protected-purge-preview",
	})
	if challenge.Code != http.StatusCreated {
		t.Fatalf("purge challenge returned %d: %s", challenge.Code, challenge.Body.String())
	}
	var issued struct {
		Action struct {
			ChallengeID string `json:"challengeId"`
		} `json:"action"`
	}
	if err := json.Unmarshal(challenge.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}
	if issued.Action.ChallengeID == "" {
		t.Fatalf("purge challenge omitted id: %s", challenge.Body.String())
	}

	mismatchedVersion := serveBoundaryRequest(handler, http.MethodPost, "/api/v2/projects/"+project.ID+"/purge", `{"challengeId":"`+issued.Action.ChallengeID+`","expectedVersion":1}`, map[string]string{
		"Content-Type":    "application/json",
		"Idempotency-Key": "protected-purge-version-mismatch",
	})
	if mismatchedVersion.Code != http.StatusConflict || !strings.Contains(mismatchedVersion.Body.String(), "challenge") {
		t.Fatalf("purge with mismatched challenge version returned %d: %s", mismatchedVersion.Code, mismatchedVersion.Body.String())
	}

	purgeBody := `{"challengeId":"` + issued.Action.ChallengeID + `","expectedVersion":2}`
	purged := serveBoundaryRequest(handler, http.MethodPost, "/api/v2/projects/"+project.ID+"/purge", purgeBody, map[string]string{
		"Content-Type":    "application/json",
		"Idempotency-Key": "protected-purge-confirm",
	})
	if purged.Code != http.StatusOK || !strings.Contains(purged.Body.String(), `"operation":"work.entity.purge"`) || !strings.Contains(purged.Body.String(), `"status":"accepted"`) {
		t.Fatalf("purge confirmation returned %d: %s", purged.Code, purged.Body.String())
	}
	replay := serveBoundaryRequest(handler, http.MethodPost, "/api/v2/projects/"+project.ID+"/purge", purgeBody, map[string]string{
		"Content-Type":    "application/json",
		"Idempotency-Key": "protected-purge-confirm",
	})
	if replay.Code != http.StatusOK || !strings.Contains(replay.Body.String(), `"replayed":true`) {
		t.Fatalf("purge replay returned %d: %s", replay.Code, replay.Body.String())
	}
	if _, err := workService.GetProject(context.Background(), scope, project.ID); !errors.Is(err, work.ErrNotFound) {
		t.Fatalf("purged project lookup = %v, want ErrNotFound", err)
	}
}

func TestConfiguredLegacyWorkTrashUsesChallengeForEveryEntityType(t *testing.T) {
	server, _ := actionTestServer(t)
	handler := server.Handler()
	scope := work.Scope{WorkspaceID: application.WorkspaceIDForUser("user-1"), UserID: "user-1"}
	project, err := server.Application.Work.CreateProject(context.Background(), scope, work.CreateProjectInput{Name: "Legacy project target"}, "legacy-project-create")
	if err != nil {
		t.Fatal(err)
	}
	task, err := server.Application.Work.CreateTask(context.Background(), scope, work.CreateTaskInput{ProjectID: project.ID, Title: "Legacy task target"}, "legacy-task-create")
	if err != nil {
		t.Fatal(err)
	}
	decision, err := server.Application.Work.CreateDecision(context.Background(), scope, work.CreateDecisionInput{ProjectID: project.ID, Title: "Legacy decision target", Outcome: "Keep the action boundary"}, "legacy-decision-create")
	if err != nil {
		t.Fatal(err)
	}

	missingChallenge := serveBoundaryRequest(handler, http.MethodPost, "/api/v2/projects/"+project.ID+"/trash", `{"expectedVersion":1}`, map[string]string{
		"Content-Type":    "application/json",
		"Idempotency-Key": "legacy-project-missing-challenge",
	})
	if missingChallenge.Code != http.StatusBadRequest || !strings.Contains(missingChallenge.Body.String(), "invalid action request") {
		t.Fatalf("legacy trash without challenge returned %d: %s", missingChallenge.Code, missingChallenge.Body.String())
	}

	testCases := []struct {
		name       string
		entityType string
		id         string
		path       string
		key        string
	}{
		{name: "project", entityType: "project", id: project.ID, path: "/api/v2/projects/" + project.ID + "/trash", key: "legacy-project-trash"},
		{name: "task", entityType: "task", id: task.ID, path: "/api/v2/tasks/" + task.ID + "/trash", key: "legacy-task-trash"},
		{name: "decision", entityType: "decision", id: decision.ID, path: "/api/v2/decisions/" + decision.ID + "/trash", key: "legacy-decision-trash"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			challengeBody := `{"operation":"work.entity.trash","entityType":"` + testCase.entityType + `","entityId":"` + testCase.id + `","expectedVersion":1}`
			challenge := serveBoundaryRequest(handler, http.MethodPost, "/api/v2/actions/challenges", challengeBody, map[string]string{
				"Content-Type":    "application/json",
				"Idempotency-Key": "preview-" + testCase.key,
			})
			if challenge.Code != http.StatusCreated {
				t.Fatalf("challenge returned %d: %s", challenge.Code, challenge.Body.String())
			}
			var issued struct {
				Action struct {
					ChallengeID string `json:"challengeId"`
				} `json:"action"`
			}
			if err := json.Unmarshal(challenge.Body.Bytes(), &issued); err != nil {
				t.Fatal(err)
			}
			if issued.Action.ChallengeID == "" {
				t.Fatalf("challenge omitted id: %s", challenge.Body.String())
			}

			trashBody := `{"challengeId":"` + issued.Action.ChallengeID + `","expectedVersion":1}`
			response := serveBoundaryRequest(handler, http.MethodPost, testCase.path, trashBody, map[string]string{
				"Content-Type":    "application/json",
				"Idempotency-Key": testCase.key,
			})
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"provider":"work"`) || !strings.Contains(response.Body.String(), `"status":"accepted"`) {
				t.Fatalf("legacy trash returned %d: %s", response.Code, response.Body.String())
			}
		})
	}

	if _, err := server.Application.Work.GetProject(context.Background(), scope, project.ID); !errors.Is(err, work.ErrNotFound) {
		t.Fatalf("project after legacy action = %v, want ErrNotFound", err)
	}
	if _, err := server.Application.Work.GetTask(context.Background(), scope, task.ID); !errors.Is(err, work.ErrNotFound) {
		t.Fatalf("task after legacy action = %v, want ErrNotFound", err)
	}
	if _, err := server.Application.Work.GetDecision(context.Background(), scope, decision.ID); !errors.Is(err, work.ErrNotFound) {
		t.Fatalf("decision after legacy action = %v, want ErrNotFound", err)
	}
}

func TestMCPTokenHTTPFlowBindsIdentityAndSupportsRevoke(t *testing.T) {
	server, _ := actionTestServer(t)
	handler := server.Handler()

	issueRequest := httptest.NewRequest(http.MethodPost, "/api/v2/mcp/tokens", strings.NewReader(`{"scopes":["assistant:use","work:read"]}`))
	issueRequest.Header.Set("Content-Type", "application/json")
	issueResponse := httptest.NewRecorder()
	handler.ServeHTTP(issueResponse, issueRequest)
	if issueResponse.Code != http.StatusCreated || issueResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("issue token returned %d/%q: %s", issueResponse.Code, issueResponse.Header().Get("Cache-Control"), issueResponse.Body.String())
	}
	var tokenResponse struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(issueResponse.Body.Bytes(), &tokenResponse); err != nil {
		t.Fatal(err)
	}
	if tokenResponse.ID == "" || tokenResponse.Token == "" {
		t.Fatalf("token response omitted one-time credentials: %s", issueResponse.Body.String())
	}

	initialize := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`))
	initialize.Header.Set("Authorization", "Bearer "+tokenResponse.Token)
	initialize.Header.Set(mcp.AcceptHeader, "application/json, text/event-stream")
	initializeResponse := httptest.NewRecorder()
	handler.ServeHTTP(initializeResponse, initialize)
	if initializeResponse.Code != http.StatusOK || strings.Contains(initializeResponse.Body.String(), tokenResponse.Token) {
		t.Fatalf("MCP initialize returned %d: %s", initializeResponse.Code, initializeResponse.Body.String())
	}

	revoke := httptest.NewRequest(http.MethodDelete, "/api/v2/mcp/tokens/"+tokenResponse.ID, nil)
	revokeResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokeResponse, revoke)
	if revokeResponse.Code != http.StatusNoContent {
		t.Fatalf("revoke returned %d: %s", revokeResponse.Code, revokeResponse.Body.String())
	}

	afterRevoke := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"ping"}`))
	afterRevoke.Header.Set("Authorization", "Bearer "+tokenResponse.Token)
	afterRevoke.Header.Set(mcp.AcceptHeader, "application/json, text/event-stream")
	afterRevokeResponse := httptest.NewRecorder()
	handler.ServeHTTP(afterRevokeResponse, afterRevoke)
	if afterRevokeResponse.Code != http.StatusUnauthorized {
		t.Fatalf("revoked MCP token returned %d: %s", afterRevokeResponse.Code, afterRevokeResponse.Body.String())
	}
}

func actionTestServer(t *testing.T) (*Server, *connectors.FakeGitHubWriter) {
	t.Helper()
	memory := store.NewSeeded(time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC))
	learningService := learning.NewService(memory, ai.DeterministicProvider{})
	productApp, err := application.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	challenges := connectors.NewMemorySafeWriteChallengeStore()
	receipts := connectors.NewMemorySafeWriteReceiptStore()
	writer := &connectors.FakeGitHubWriter{NextIssueNumber: 1}
	guarded, err := connectors.NewGitHubSafeWriteService(writer, challenges, receipts)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	guarded.Clock = func() time.Time { return clock }
	actionService, err := actions.NewServiceWithWork(guarded, productApp.Work, challenges, challenges)
	if err != nil {
		t.Fatal(err)
	}
	actionService.Clock = func() time.Time { return clock }
	mcpApplication, err := mcp.NewApplication(mcp.ApplicationServices{
		Assistant: productApp.Assistant,
		Knowledge: productApp.Knowledge,
		Work:      productApp.Work,
		Product:   productApp,
		Actions:   actionService,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := mcp.NewRegistry()
	if err := mcpApplication.Register(registry); err != nil {
		t.Fatal(err)
	}
	tokenStore := mcp.NewTokenStore()
	server := NewServer(learningService, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.Application = productApp
	server.Actions = actionService
	server.MCPTokenStore = tokenStore
	server.MCP = mcp.NewHandler(registry, tokenStore)
	return server, writer
}
