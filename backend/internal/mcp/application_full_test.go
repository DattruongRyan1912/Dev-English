package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/actions"
	productapp "github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

func TestFullApplicationRegistryExposesStableV1Surface(t *testing.T) {
	product := newFullProductFixture(t)
	application, err := NewApplication(ApplicationServices{
		Assistant: product.Assistant,
		Knowledge: product.Knowledge,
		Work:      product.Work,
		Product:   product,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	principal := fullApplicationPrincipal("user-a", productapp.WorkspaceIDForUser("user-a"), ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead, ScopeWorkWrite)
	definitions := registry.ListTools(principal)
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}
	sort.Strings(names)
	want := []string{
		"assistant.ask", "assistant_ask", "knowledge.get_chunk", "knowledge.get_claim", "knowledge_get", "knowledge_search",
		"project_create", "project_list", "task_list", "task_upsert", "decision_get", "work.get_decision", "work.get_project", "work.get_task", "entity_trash", "decision_record", "knowledge_import_manual",
	}
	sort.Strings(want)
	if fmt.Sprint(names) != fmt.Sprint(want) {
		t.Fatalf("full MCP tools = %v, want %v", names, want)
	}
	resources := registry.ListResources(principal)
	if len(resources) != 3 {
		t.Fatalf("full MCP resources = %#v", resources)
	}
}

func TestFullApplicationKnowledgeImportAdvertisesReplayRequirement(t *testing.T) {
	product := newFullProductFixture(t)
	application, err := NewApplication(ApplicationServices{
		Assistant: product.Assistant,
		Knowledge: product.Knowledge,
		Work:      product.Work,
		Product:   product,
	})
	if err != nil {
		t.Fatal(err)
	}

	var found Tool
	for _, tool := range applicationTools(application) {
		if tool.Name == "knowledge_import_manual" {
			found = tool
			break
		}
	}
	if found.Name == "" {
		t.Fatal("knowledge_import_manual is not registered in the full application")
	}
	if !found.Mutating {
		t.Fatal("knowledge_import_manual must be marked mutating")
	}
	if !strings.Contains(found.Description, "idempotency key") {
		t.Fatalf("knowledge_import_manual description = %q, want idempotency requirement", found.Description)
	}
}

func TestFullApplicationMCPUsesCanonicalProjectService(t *testing.T) {
	product := newFullProductFixture(t)
	application, err := NewApplication(ApplicationServices{
		Assistant: product.Assistant,
		Knowledge: product.Knowledge,
		Work:      product.Work,
		Product:   product,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	userID := "user-a"
	principal := fullApplicationPrincipal(userID, productapp.WorkspaceIDForUser(userID), ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead, ScopeWorkWrite)
	handler := NewHandler(registry, nil)
	ctx := store.WithUser(context.Background(), userID)

	create := handler.Dispatch(ctx, principal, Request{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: json.RawMessage(`{"name":"project_create","arguments":{"name":"MCP project","description":"created through the canonical application"}}`)}, RequestMeta{IdempotencyKey: "mcp-project-1", Nonce: "mcp-nonce-1"})
	if create == nil || create.Error != nil {
		t.Fatalf("project_create response = %#v", create)
	}
	var created struct {
		StructuredContent json.RawMessage `json:"structuredContent"`
	}
	if err := json.Unmarshal(create.Result, &created); err != nil {
		t.Fatal(err)
	}
	var project work.Project
	if err := json.Unmarshal(created.StructuredContent, &project); err != nil {
		t.Fatal(err)
	}
	if project.Name != "MCP project" || project.WorkspaceID != principal.WorkspaceID || project.OwnerUserID != userID {
		t.Fatalf("created project escaped canonical scope: %+v", project)
	}

	list := handler.Dispatch(ctx, principal, Request{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/call", Params: json.RawMessage(`{"name":"project_list","arguments":{"limit":10}}`)}, RequestMeta{})
	if list == nil || list.Error != nil {
		t.Fatalf("project_list response = %#v", list)
	}
	var listed struct {
		StructuredContent json.RawMessage `json:"structuredContent"`
	}
	if err := json.Unmarshal(list.Result, &listed); err != nil {
		t.Fatal(err)
	}
	var projects []work.Project
	if err := json.Unmarshal(listed.StructuredContent, &projects); err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].ID != project.ID {
		t.Fatalf("project_list = %+v, want project %q", projects, project.ID)
	}
}

func TestFullApplicationMCPReplayAndVersionConflictStayCanonical(t *testing.T) {
	product := newFullProductFixture(t)
	application, err := NewApplication(ApplicationServices{
		Assistant: product.Assistant,
		Knowledge: product.Knowledge,
		Work:      product.Work,
		Product:   product,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	userID := "user-a"
	principal := fullApplicationPrincipal(userID, productapp.WorkspaceIDForUser(userID), ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead, ScopeWorkWrite)
	handler := NewHandler(registry, nil)
	ctx := store.WithUser(context.Background(), userID)

	createProject := testFullApplicationCall(`"project_create"`, "project_create", `{"name":"Conflict project"}`)
	first := handler.Dispatch(ctx, principal, createProject, RequestMeta{IdempotencyKey: "mcp-replay-project", Nonce: "mcp-replay-project-1"})
	if first == nil || first.Error != nil {
		t.Fatalf("project_create response = %#v", first)
	}
	replay := handler.Dispatch(ctx, principal, testFullApplicationCall(`"project_create-retry"`, "project_create", `{"name":"Conflict project"}`), RequestMeta{IdempotencyKey: "mcp-replay-project", Nonce: "mcp-replay-project-2"})
	if replay == nil || replay.Error != nil || string(first.Result) != string(replay.Result) {
		t.Fatalf("project_create replay = %#v, first = %#v", replay, first)
	}

	var project work.Project
	decodeFullApplicationStructured(t, first, &project)
	createTask := handler.Dispatch(ctx, principal, testFullApplicationCall(`"task-create"`, "task_upsert", `{"projectId":`+mustJSON(t, project.ID)+`,"title":"Versioned task"}`), RequestMeta{IdempotencyKey: "mcp-conflict-task-create", Nonce: "mcp-conflict-task-create-1"})
	if createTask == nil || createTask.Error != nil {
		t.Fatalf("task_upsert create response = %#v", createTask)
	}
	var task work.Task
	decodeFullApplicationStructured(t, createTask, &task)

	update := handler.Dispatch(ctx, principal, testFullApplicationCall(`"task-update"`, "task_upsert", `{"id":`+mustJSON(t, task.ID)+`,"title":"Current task","expectedVersion":1}`), RequestMeta{IdempotencyKey: "mcp-conflict-task-update", Nonce: "mcp-conflict-task-update-1"})
	if update == nil || update.Error != nil {
		t.Fatalf("task_upsert update response = %#v", update)
	}
	conflict := handler.Dispatch(ctx, principal, testFullApplicationCall(`"task-stale"`, "task_upsert", `{"id":`+mustJSON(t, task.ID)+`,"title":"Stale task","expectedVersion":1}`), RequestMeta{IdempotencyKey: "mcp-conflict-task-stale", Nonce: "mcp-conflict-task-stale-1"})
	if conflict == nil || conflict.Error == nil || conflict.Error.Code != ConflictError {
		t.Fatalf("stale task response = %#v, want ConflictError", conflict)
	}

	scope, _, _, err := product.Scope(ctx)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := product.Work.GetTask(ctx, scope, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Title != "Current task" || actual.Version != 2 {
		t.Fatalf("stale MCP update changed canonical task: %+v", actual)
	}

	importArguments := `{"id":"mcp-source","name":"MCP notes","kind":"manual","uri":"memory://mcp-source","mimeType":"text/plain","content":"MCP idempotency remains canonical at the application boundary."}`
	importFirst := handler.Dispatch(ctx, principal, testFullApplicationCall(`"source-import"`, "knowledge_import_manual", importArguments), RequestMeta{IdempotencyKey: "mcp-source-import", Nonce: "mcp-source-import-1"})
	if importFirst == nil || importFirst.Error != nil {
		t.Fatalf("knowledge_import_manual response = %#v", importFirst)
	}
	importReplay := handler.Dispatch(ctx, principal, testFullApplicationCall(`"source-import-retry"`, "knowledge_import_manual", importArguments), RequestMeta{IdempotencyKey: "mcp-source-import", Nonce: "mcp-source-import-2"})
	if importReplay == nil || importReplay.Error != nil || string(importFirst.Result) != string(importReplay.Result) {
		t.Fatalf("knowledge_import_manual replay = %#v, first = %#v", importReplay, importFirst)
	}
	importConflict := handler.Dispatch(ctx, principal, testFullApplicationCall(`"source-import-conflict"`, "knowledge_import_manual", `{"id":"mcp-source","name":"MCP notes","kind":"manual","uri":"memory://mcp-source","mimeType":"text/plain","content":"Changed content must not reuse the MCP import key."}`), RequestMeta{IdempotencyKey: "mcp-source-import", Nonce: "mcp-source-import-3"})
	if importConflict == nil || importConflict.Error == nil || importConflict.Error.Code != IdempotencyError {
		t.Fatalf("knowledge_import_manual key reuse response = %#v, want IdempotencyError", importConflict)
	}
}

func TestFullApplicationMCPKnowledgeSearchMatchesCanonicalApplication(t *testing.T) {
	product := newFullProductFixture(t)
	application, err := NewApplication(ApplicationServices{
		Assistant: product.Assistant,
		Knowledge: product.Knowledge,
		Work:      product.Work,
		Product:   product,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	userID := "user-search"
	principal := fullApplicationPrincipal(userID, productapp.WorkspaceIDForUser(userID), ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead, ScopeWorkWrite)
	handler := NewHandler(registry, nil)
	ctx := store.WithUser(context.Background(), userID)
	if _, err := product.ImportManualSource(ctx, productapp.ManualSourceInput{
		ID:      "search-source",
		Name:    "Retrieval notes",
		Kind:    "manual",
		URI:     "memory://retrieval-notes",
		Content: "Hybrid retrieval keeps canonical evidence close to the work context.",
	}); err != nil {
		t.Fatal(err)
	}

	want, err := product.SearchKnowledge(ctx, "canonical evidence", 20)
	if err != nil {
		t.Fatal(err)
	}
	response := handler.Dispatch(ctx, principal, testFullApplicationCall(`"knowledge-search"`, "knowledge_search", `{"query":"canonical evidence","limit":20}`), RequestMeta{})
	if response == nil || response.Error != nil {
		t.Fatalf("knowledge_search response = %#v", response)
	}
	var got []productapp.SearchHit
	decodeFullApplicationStructured(t, response, &got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MCP knowledge search = %#v, canonical application = %#v", got, want)
	}
}

func TestFullApplicationMCPExposesCanonicalGraphAndResources(t *testing.T) {
	product := newFullProductFixture(t)
	application, err := NewApplication(ApplicationServices{
		Assistant: product.Assistant,
		Knowledge: product.Knowledge,
		Work:      product.Work,
		Product:   product,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	userID := "graph-user"
	workspaceID := productapp.WorkspaceIDForUser(userID)
	principal := fullApplicationPrincipal(userID, workspaceID, ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead, ScopeWorkWrite)
	handler := NewHandler(registry, nil)
	ctx := store.WithUser(context.Background(), userID)

	projectResponse := handler.Dispatch(ctx, principal, testFullApplicationCall(`"graph-project"`, "project_create", `{"name":"Graph project","description":"Canonical project for graph coverage"}`), RequestMeta{IdempotencyKey: "graph-project-create", Nonce: "graph-project-create-1"})
	if projectResponse == nil || projectResponse.Error != nil {
		t.Fatalf("project_create response = %#v", projectResponse)
	}
	var project work.Project
	decodeFullApplicationStructured(t, projectResponse, &project)

	taskResponse := handler.Dispatch(ctx, principal, testFullApplicationCall(`"graph-task"`, "task_upsert", `{"projectId":`+mustJSON(t, project.ID)+`,"title":"Graph task","description":"Task in canonical graph"}`), RequestMeta{IdempotencyKey: "graph-task-create", Nonce: "graph-task-create-1"})
	if taskResponse == nil || taskResponse.Error != nil {
		t.Fatalf("task_upsert response = %#v", taskResponse)
	}
	var task work.Task
	decodeFullApplicationStructured(t, taskResponse, &task)

	decisionResponse := handler.Dispatch(ctx, principal, testFullApplicationCall(`"graph-decision"`, "decision_record", `{"projectId":`+mustJSON(t, project.ID)+`,"title":"Graph decision","outcome":"Keep the source-backed workflow"}`), RequestMeta{IdempotencyKey: "graph-decision-create", Nonce: "graph-decision-create-1"})
	if decisionResponse == nil || decisionResponse.Error != nil {
		t.Fatalf("decision_record response = %#v", decisionResponse)
	}
	var decision work.Decision
	decodeFullApplicationStructured(t, decisionResponse, &decision)

	importResponse := handler.Dispatch(ctx, principal, testFullApplicationCall(`"graph-source"`, "knowledge_import_manual", `{"id":"graph-source","name":"Graph notes","kind":"manual","uri":"memory://graph","mimeType":"text/plain","content":"Canonical graph evidence remains immutable and workspace scoped."}`), RequestMeta{IdempotencyKey: "graph-source-import", Nonce: "graph-source-import-1"})
	if importResponse == nil || importResponse.Error != nil {
		t.Fatalf("knowledge_import_manual response = %#v", importResponse)
	}
	var imported productapp.ImportedSource
	decodeFullApplicationStructured(t, importResponse, &imported)
	detail, err := product.GetKnowledgeSourceDetail(ctx, imported.Source.ID)
	if err != nil || len(detail.Items) != 1 {
		t.Fatalf("GetKnowledgeSourceDetail() = %+v, %v", detail, err)
	}
	itemID := detail.Items[0].ID

	_, knowledgeScope, _, err := product.Scope(ctx)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	claim := knowledge.KnowledgeClaim{
		ID: "graph-claim", WorkspaceID: knowledgeScope.ID, Statement: "Canonical graph evidence is immutable.",
		Certainty: knowledge.ClaimCanonical, Freshness: knowledge.ClaimCurrent, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := product.Knowledge.CreateClaimBundle(ctx, knowledgeScope, claim, []knowledge.ClaimEvidence{{
		ID: "graph-evidence", WorkspaceID: knowledgeScope.ID, ClaimID: claim.ID, SourceRevisionID: imported.RevisionID,
		ChunkID: imported.ChunkID, Locator: "memory://graph#1", Quote: "Canonical graph evidence remains immutable and workspace scoped.",
		Freshness: knowledge.EvidenceCurrent, CreatedAt: now,
	}}); err != nil {
		t.Fatalf("CreateClaimBundle() error = %v", err)
	}

	for _, item := range []struct {
		kind string
		id   string
	}{
		{kind: "source", id: imported.Source.ID},
		{kind: "item", id: itemID},
		{kind: "revision", id: imported.RevisionID},
		{kind: "chunk", id: imported.ChunkID},
		{kind: "claim", id: claim.ID},
	} {
		response := handler.Dispatch(ctx, principal, testFullApplicationCall(`"knowledge-`+item.kind+`"`, "knowledge_get", `{"kind":`+mustJSON(t, item.kind)+`,"id":`+mustJSON(t, item.id)+`}`), RequestMeta{})
		if response == nil || response.Error != nil {
			t.Fatalf("knowledge_get %s response = %#v", item.kind, response)
		}
		var payload map[string]any
		decodeFullApplicationStructured(t, response, &payload)
		if payload["id"] != item.id {
			t.Fatalf("knowledge_get %s payload id = %#v, want %q", item.kind, payload["id"], item.id)
		}
	}

	for _, item := range []struct {
		name string
		args string
	}{
		{name: "project_list", args: `{"limit":10}`},
		{name: "task_list", args: `{"projectId":` + mustJSON(t, project.ID) + `,"limit":10}`},
		{name: "decision_get", args: `{"id":` + mustJSON(t, decision.ID) + `}`},
		{name: "assistant_ask", args: `{"message":"What remains immutable in the graph?"}`},
	} {
		response := handler.Dispatch(ctx, principal, testFullApplicationCall(`"`+item.name+`"`, item.name, item.args), RequestMeta{})
		if response == nil || response.Error != nil {
			t.Fatalf("%s response = %#v", item.name, response)
		}
	}

	for _, uri := range []string{ResourceWorkProjectsURI, ResourceWorkTasksURI, ResourceWorkDecisionsURI} {
		response := handler.Dispatch(ctx, principal, Request{
			JSONRPC: "2.0", ID: json.RawMessage(`"resource-` + uri + `"`), Method: "resources/read",
			Params: json.RawMessage(`{"uri":` + mustJSON(t, uri) + `}`),
		}, RequestMeta{})
		if response == nil || response.Error != nil {
			t.Fatalf("resource %s response = %#v", uri, response)
		}
		var result ReadResourceResult
		if err := json.Unmarshal(response.Result, &result); err != nil {
			t.Fatalf("decode resource %s: %v", uri, err)
		}
		if len(result.Contents) != 1 || result.Contents[0].Text == nil {
			t.Fatalf("resource %s contents = %+v", uri, result.Contents)
		}
	}

	if task.ID == "" || decision.ID == "" {
		t.Fatalf("canonical graph identities missing: task=%+v decision=%+v", task, decision)
	}
}

func TestFullApplicationMCPEntityTrashUsesCanonicalVersionAndReplay(t *testing.T) {
	product := newFullProductFixture(t)
	application, err := NewApplication(ApplicationServices{
		Assistant: product.Assistant,
		Knowledge: product.Knowledge,
		Work:      product.Work,
		Product:   product,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	userID := "trash-user"
	principal := fullApplicationPrincipal(userID, productapp.WorkspaceIDForUser(userID), ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead, ScopeWorkWrite)
	handler := NewHandler(registry, nil)
	ctx := store.WithUser(context.Background(), userID)

	create := handler.Dispatch(ctx, principal, testFullApplicationCall(`"trash-project-create"`, "project_create", `{"name":"Trash candidate"}`), RequestMeta{IdempotencyKey: "trash-project-create", Nonce: "trash-project-create-1"})
	if create == nil || create.Error != nil {
		t.Fatalf("project_create response = %#v", create)
	}
	var project work.Project
	decodeFullApplicationStructured(t, create, &project)

	trashCall := testFullApplicationCall(`"trash-project"`, "entity_trash", `{"entityType":"project","id":`+mustJSON(t, project.ID)+`,"expectedVersion":1}`)
	trash := handler.Dispatch(ctx, principal, trashCall, RequestMeta{IdempotencyKey: "trash-project", Nonce: "trash-project-1"})
	if trash == nil || trash.Error != nil {
		t.Fatalf("entity_trash response = %#v", trash)
	}
	var trashed work.Project
	decodeFullApplicationStructured(t, trash, &trashed)
	if trashed.ID != project.ID || trashed.DeletedAt == nil || trashed.Version != 2 {
		t.Fatalf("entity_trash result = %+v, want trashed version 2", trashed)
	}

	replay := handler.Dispatch(ctx, principal, testFullApplicationCall(`"trash-project-replay"`, "entity_trash", `{"entityType":"project","id":`+mustJSON(t, project.ID)+`,"expectedVersion":1}`), RequestMeta{IdempotencyKey: "trash-project", Nonce: "trash-project-replay-1"})
	if replay == nil || replay.Error != nil || string(replay.Result) != string(trash.Result) {
		t.Fatalf("entity_trash replay = %#v, first = %#v", replay, trash)
	}

	stored, err := product.Work.GetProject(ctx, work.Scope{WorkspaceID: principal.WorkspaceID, UserID: userID}, project.ID)
	if !errors.Is(err, work.ErrNotFound) {
		t.Fatalf("canonical trashed project error = %v, want ErrNotFound", err)
	}
	if stored != (work.Project{}) {
		t.Fatalf("canonical trashed project leaked through GetProject: %+v", stored)
	}
}

func TestFullApplicationMCPProductionEntityTrashUsesActionReceipt(t *testing.T) {
	product := newFullProductFixture(t)
	challenges := connectors.NewMemorySafeWriteChallengeStore()
	receipts := connectors.NewMemorySafeWriteReceiptStore()
	guarded, err := connectors.NewGitHubSafeWriteService(&connectors.FakeGitHubWriter{}, challenges, receipts)
	if err != nil {
		t.Fatal(err)
	}
	actionService, err := actions.NewServiceWithWork(guarded, product.Work, challenges, challenges)
	if err != nil {
		t.Fatal(err)
	}
	application, err := NewApplication(ApplicationServices{
		Assistant: product.Assistant,
		Knowledge: product.Knowledge,
		Work:      product.Work,
		Product:   product,
		Actions:   actionService,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	userID := "mcp-work-trash-user"
	workspaceID := productapp.WorkspaceIDForUser(userID)
	scope := actions.Scope{WorkspaceID: workspaceID, UserID: userID}
	principal := fullApplicationPrincipal(userID, workspaceID, ScopeWorkRead, ScopeWorkWrite)
	ctx := store.WithUser(context.Background(), userID)
	project, err := product.Work.CreateProject(ctx, work.Scope(scope), work.CreateProjectInput{Name: "MCP production trash"}, "mcp-production-trash-create")
	if err != nil {
		t.Fatal(err)
	}
	target := connectors.SafeWriteTarget{
		Operation:       connectors.SafeWriteOperationEntityTrash,
		EntityType:      string(work.EntityProject),
		EntityID:        project.ID,
		ExpectedVersion: project.Version,
	}
	challenge, err := actionService.CreateChallenge(ctx, scope, actions.CreateChallengeRequest{Target: target, IdempotencyKey: "mcp-production-trash-preview"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(registry, nil)
	response := handler.Dispatch(ctx, principal, testFullApplicationCall(
		`"mcp-production-trash"`, "entity_trash",
		`{"challengeId":`+mustJSON(t, challenge.Binding.ChallengeID)+`,"entityType":"project","id":`+mustJSON(t, project.ID)+`,"expectedVersion":1}`,
	), RequestMeta{IdempotencyKey: "mcp-production-trash-confirm", Nonce: "mcp-production-trash-confirm-1"})
	if response == nil || response.Error != nil {
		t.Fatalf("production entity_trash response = %#v", response)
	}
	var result struct {
		Action   assistant.ActionBinding `json:"action"`
		Receipt  assistant.ActionReceipt `json:"receipt"`
		Replayed bool                    `json:"replayed"`
	}
	decodeFullApplicationStructured(t, response, &result)
	if result.Action.Provider != connectors.ProviderWork || result.Receipt.Provider != connectors.ProviderWork || result.Receipt.Status != connectors.SafeWriteReceiptAccepted || result.Replayed {
		t.Fatalf("production entity_trash result = %+v, want accepted work receipt", result)
	}
	if _, err := product.Work.GetProject(ctx, work.Scope(scope), project.ID); !errors.Is(err, work.ErrNotFound) {
		t.Fatalf("production MCP project lookup after action = %v, want ErrNotFound", err)
	}
}

func TestFullApplicationMCPWriteToolsBindPrincipalWithoutCallerContext(t *testing.T) {
	product := newFullProductFixture(t)
	application, err := NewApplication(ApplicationServices{
		Assistant: product.Assistant,
		Knowledge: product.Knowledge,
		Work:      product.Work,
		Product:   product,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	userID := "mcp-principal-isolation"
	workspaceID := productapp.WorkspaceIDForUser(userID)
	principal := fullApplicationPrincipal(userID, workspaceID, ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead, ScopeWorkWrite)
	handler := NewHandler(registry, nil)

	// A real MCP adapter receives a request context that is not allowed to
	// supply the canonical application identity. The principal must be the
	// only source of owner/workspace scope for every write tool.
	ctx := context.Background()
	projectResponse := handler.Dispatch(ctx, principal, testFullApplicationCall(`"principal-project"`, "project_create", `{"name":"Principal project"}`), RequestMeta{IdempotencyKey: "principal-project-create", Nonce: "principal-project-create-1"})
	if projectResponse == nil || projectResponse.Error != nil {
		t.Fatalf("project_create response = %#v", projectResponse)
	}
	var project work.Project
	decodeFullApplicationStructured(t, projectResponse, &project)
	if project.OwnerUserID != userID || project.WorkspaceID != workspaceID {
		t.Fatalf("project scope = %+v, want user=%q workspace=%q", project, userID, workspaceID)
	}

	taskResponse := handler.Dispatch(ctx, principal, testFullApplicationCall(`"principal-task"`, "task_upsert", `{"projectId":`+mustJSON(t, project.ID)+`,"title":"Principal task"}`), RequestMeta{IdempotencyKey: "principal-task-create", Nonce: "principal-task-create-1"})
	if taskResponse == nil || taskResponse.Error != nil {
		t.Fatalf("task_upsert response = %#v", taskResponse)
	}
	var task work.Task
	decodeFullApplicationStructured(t, taskResponse, &task)
	if task.OwnerUserID != userID || task.WorkspaceID != workspaceID {
		t.Fatalf("task scope = %+v, want user=%q workspace=%q", task, userID, workspaceID)
	}

	decisionResponse := handler.Dispatch(ctx, principal, testFullApplicationCall(`"principal-decision"`, "decision_record", `{"projectId":`+mustJSON(t, project.ID)+`,"title":"Principal decision","outcome":"Keep principal scope"}`), RequestMeta{IdempotencyKey: "principal-decision-create", Nonce: "principal-decision-create-1"})
	if decisionResponse == nil || decisionResponse.Error != nil {
		t.Fatalf("decision_record response = %#v", decisionResponse)
	}
	var decision work.Decision
	decodeFullApplicationStructured(t, decisionResponse, &decision)
	if decision.OwnerUserID != userID || decision.WorkspaceID != workspaceID {
		t.Fatalf("decision scope = %+v, want user=%q workspace=%q", decision, userID, workspaceID)
	}

	sourceResponse := handler.Dispatch(ctx, principal, testFullApplicationCall(`"principal-source"`, "knowledge_import_manual", `{"id":"principal-source","name":"Principal notes","kind":"manual","uri":"memory://principal","content":"Principal scoped source"}`), RequestMeta{IdempotencyKey: "principal-source-import", Nonce: "principal-source-import-1"})
	if sourceResponse == nil || sourceResponse.Error != nil {
		t.Fatalf("knowledge_import_manual response = %#v", sourceResponse)
	}
	var imported productapp.ImportedSource
	decodeFullApplicationStructured(t, sourceResponse, &imported)
	principalContext := store.WithUser(context.Background(), userID)
	if _, err := product.GetKnowledgeSourceDetail(principalContext, imported.Source.ID); err != nil {
		t.Fatalf("principal source lookup = %v", err)
	}
	if _, err := product.GetKnowledgeSourceDetail(context.Background(), imported.Source.ID); !errors.Is(err, knowledge.ErrNotFound) {
		t.Fatalf("implicit default scope source lookup = %v, want ErrNotFound", err)
	}
}

func TestFullApplicationMCPConfirmedGitHubToolsUseActionService(t *testing.T) {
	product := newFullProductFixture(t)
	challenges := connectors.NewMemorySafeWriteChallengeStore()
	writer := &connectors.FakeGitHubWriter{NextIssueNumber: 41}
	guarded, err := connectors.NewGitHubSafeWriteService(writer, challenges, connectors.NewMemorySafeWriteReceiptStore())
	if err != nil {
		t.Fatal(err)
	}
	actionService, err := actions.NewService(guarded, challenges, challenges)
	if err != nil {
		t.Fatal(err)
	}
	application, err := NewApplication(ApplicationServices{
		Assistant: product.Assistant,
		Knowledge: product.Knowledge,
		Work:      product.Work,
		Product:   product,
		Actions:   actionService,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	userID := "mcp-github-user"
	scope := actions.Scope{WorkspaceID: productapp.WorkspaceIDForUser(userID), UserID: userID}
	principal := fullApplicationPrincipal(userID, scope.WorkspaceID, ScopeGitHubWrite)
	handler := NewHandler(registry, nil)

	testCases := []struct {
		name       string
		operation  connectors.SafeWriteOperation
		target     connectors.SafeWriteTarget
		arguments  func(string) string
		requestKey string
	}{
		{
			name:      "create",
			operation: connectors.SafeWriteOperationCreateIssue,
			target: connectors.SafeWriteTarget{
				Operation:  connectors.SafeWriteOperationCreateIssue,
				Repository: "owner/repo", Title: "MCP issue", Body: "Created through MCP",
			},
			arguments: func(challengeID string) string {
				return fmt.Sprintf(`{"challengeId":%q,"repository":"owner/repo","title":"MCP issue","body":"Created through MCP"}`, challengeID)
			},
			requestKey: "mcp-confirm-create",
		},
		{
			name:      "comment",
			operation: connectors.SafeWriteOperationAddComment,
			target: connectors.SafeWriteTarget{
				Operation:  connectors.SafeWriteOperationAddComment,
				Repository: "owner/repo", Issue: 7, Body: "MCP comment",
			},
			arguments: func(challengeID string) string {
				return fmt.Sprintf(`{"challengeId":%q,"repository":"owner/repo","issue":7,"body":"MCP comment"}`, challengeID)
			},
			requestKey: "mcp-confirm-comment",
		},
		{
			name:      "label",
			operation: connectors.SafeWriteOperationSetLabels,
			target: connectors.SafeWriteTarget{
				Operation:  connectors.SafeWriteOperationSetLabels,
				Repository: "owner/repo", Issue: 7, Labels: []string{"bug", "backend"},
			},
			arguments: func(challengeID string) string {
				return fmt.Sprintf(`{"challengeId":%q,"repository":"owner/repo","issue":7,"labels":["bug","backend"]}`, challengeID)
			},
			requestKey: "mcp-confirm-label",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			challenge, err := actionService.CreateChallenge(context.Background(), scope, actions.CreateChallengeRequest{
				Target: testCase.target, IdempotencyKey: "mcp-preview-" + testCase.name,
			})
			if err != nil {
				t.Fatal(err)
			}
			response := handler.Dispatch(context.Background(), principal, testFullApplicationCall(
				`"github-`+testCase.name+`"`, "github_issue_"+map[string]string{"create": "create", "comment": "comment", "label": "label"}[testCase.name], testCase.arguments(challenge.Binding.ChallengeID),
			), RequestMeta{IdempotencyKey: testCase.requestKey, Nonce: "nonce-" + testCase.name})
			if response == nil || response.Error != nil {
				t.Fatalf("MCP confirmation response = %#v", response)
			}
			var result struct {
				Receipt  assistant.ActionReceipt `json:"receipt"`
				Replayed bool                    `json:"replayed"`
			}
			decodeFullApplicationStructured(t, response, &result)
			if result.Replayed || result.Receipt.ChallengeID != challenge.Binding.ChallengeID || result.Receipt.Status != connectors.SafeWriteReceiptAccepted {
				t.Fatalf("confirmation result = %+v", result)
			}
		})
	}

	if len(writer.CreateCalls) != 1 || len(writer.CommentCalls) != 1 || len(writer.LabelCalls) != 1 {
		t.Fatalf("GitHub writer calls = create %d comment %d labels %d, want one each", len(writer.CreateCalls), len(writer.CommentCalls), len(writer.LabelCalls))
	}
}

func testFullApplicationCall(requestID, toolName, arguments string) Request {
	return Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(requestID),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":` + mustJSONValue(toolName) + `,"arguments":` + arguments + `}`),
	}
}

func mustJSONValue(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func mustJSON(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func decodeFullApplicationStructured(t *testing.T, response *Response, target any) {
	t.Helper()
	var result struct {
		StructuredContent json.RawMessage `json:"structuredContent"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.StructuredContent) == 0 {
		t.Fatalf("MCP response has no structuredContent: %s", response.Result)
	}
	if err := json.Unmarshal(result.StructuredContent, target); err != nil {
		t.Fatal(err)
	}
}

func newFullProductFixture(t *testing.T) *productapp.App {
	t.Helper()
	product, err := productapp.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	return product
}

func fullApplicationPrincipal(userID, workspaceID string, scopes ...Scope) Principal {
	return Principal{TokenID: "full-application-token", UserID: userID, WorkspaceID: workspaceID, Scopes: scopes}
}
