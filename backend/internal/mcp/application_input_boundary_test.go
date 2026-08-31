package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	productapp "github.com/DattruongRyan1912/Dev-English/backend/internal/application"
)

func TestFullApplicationMCPRejectsMalformedCanonicalInputs(t *testing.T) {
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
	principal := fullApplicationPrincipal("input-boundary-user", productapp.WorkspaceIDForUser("input-boundary-user"), ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead, ScopeWorkWrite)
	ctx := context.Background()

	cases := []struct {
		name string
		call func() (CallToolResult, error)
	}{
		{name: "assistant decode", call: func() (CallToolResult, error) {
			return application.handleAssistantAsk(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{`)})
		}},
		{name: "claim decode", call: func() (CallToolResult, error) {
			return application.handleKnowledgeGetClaim(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{}`)})
		}},
		{name: "chunk decode", call: func() (CallToolResult, error) {
			return application.handleKnowledgeGetChunk(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{}`)})
		}},
		{name: "project get decode", call: func() (CallToolResult, error) {
			return application.handleWorkGetProject(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{}`)})
		}},
		{name: "task get decode", call: func() (CallToolResult, error) {
			return application.handleWorkGetTask(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{}`)})
		}},
		{name: "decision get decode", call: func() (CallToolResult, error) {
			return application.handleWorkGetDecision(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{}`)})
		}},
		{name: "knowledge search decode", call: func() (CallToolResult, error) {
			return application.handleKnowledgeSearch(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{}`)})
		}},
		{name: "knowledge get decode", call: func() (CallToolResult, error) {
			return application.handleKnowledgeGet(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{}`)})
		}},
		{name: "project list bounds", call: func() (CallToolResult, error) {
			return application.handleProjectList(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"limit":51}`)})
		}},
		{name: "task list project id", call: func() (CallToolResult, error) {
			return application.handleTaskList(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"projectId":"bad id"}`)})
		}},
		{name: "project create decode", call: func() (CallToolResult, error) {
			return application.handleProjectCreate(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{`), IdempotencyKey: "input-project"})
		}},
		{name: "task create title", call: func() (CallToolResult, error) {
			return application.handleTaskUpsert(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{}`), IdempotencyKey: "input-task-create"})
		}},
		{name: "task update version", call: func() (CallToolResult, error) {
			return application.handleTaskUpsert(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"id":"task-1","title":"updated"}`), IdempotencyKey: "input-task-update"})
		}},
		{name: "decision create fields", call: func() (CallToolResult, error) {
			return application.handleDecisionRecord(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"title":"decision"}`), IdempotencyKey: "input-decision-create"})
		}},
		{name: "decision update version", call: func() (CallToolResult, error) {
			return application.handleDecisionRecord(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"id":"decision-1","outcome":"updated"}`), IdempotencyKey: "input-decision-update"})
		}},
		{name: "manual import decode", call: func() (CallToolResult, error) {
			return application.handleKnowledgeImportManual(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{`), IdempotencyKey: "input-import"})
		}},
		{name: "trash version", call: func() (CallToolResult, error) {
			return application.handleEntityTrash(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"entityType":"project","id":"project-1"}`), IdempotencyKey: "input-trash"})
		}},
		{name: "github issue decode", call: func() (CallToolResult, error) {
			return application.handleGitHubIssueCreate(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{`)})
		}},
		{name: "github comment decode", call: func() (CallToolResult, error) {
			return application.handleGitHubIssueComment(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{`)})
		}},
		{name: "github label decode", call: func() (CallToolResult, error) {
			return application.handleGitHubIssueLabel(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{`)})
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := testCase.call()
			assertApplicationRPCCode(t, err, InvalidParams)
		})
	}
}

func TestFullApplicationMCPBindsEveryCanonicalPathToPrincipalScope(t *testing.T) {
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
	userID := "scope-boundary-user"
	principal := fullApplicationPrincipal(userID, "workspace-forged", ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead, ScopeWorkWrite)
	ctx := context.Background()

	cases := []struct {
		name string
		call func() error
	}{
		{name: "assistant", call: func() error {
			_, err := application.handleAssistantAsk(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"message":"hello"}`)})
			return err
		}},
		{name: "claim", call: func() error {
			_, err := application.handleKnowledgeGetClaim(ctx, Invocation{Principal: principal, Arguments: applicationIDArgumentsJSON("claim")})
			return err
		}},
		{name: "chunk", call: func() error {
			_, err := application.handleKnowledgeGetChunk(ctx, Invocation{Principal: principal, Arguments: applicationIDArgumentsJSON("chunk")})
			return err
		}},
		{name: "project", call: func() error {
			_, err := application.handleWorkGetProject(ctx, Invocation{Principal: principal, Arguments: applicationIDArgumentsJSON("project")})
			return err
		}},
		{name: "task", call: func() error {
			_, err := application.handleWorkGetTask(ctx, Invocation{Principal: principal, Arguments: applicationIDArgumentsJSON("task")})
			return err
		}},
		{name: "decision", call: func() error {
			_, err := application.handleWorkGetDecision(ctx, Invocation{Principal: principal, Arguments: applicationIDArgumentsJSON("decision")})
			return err
		}},
		{name: "search", call: func() error {
			_, err := application.handleKnowledgeSearch(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"query":"canonical"}`)})
			return err
		}},
		{name: "project list", call: func() error {
			_, err := application.handleProjectList(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{}`)})
			return err
		}},
		{name: "task list", call: func() error {
			_, err := application.handleTaskList(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{}`)})
			return err
		}},
		{name: "project create", call: func() error {
			_, err := application.handleProjectCreate(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"name":"project"}`), IdempotencyKey: "scope-project"})
			return err
		}},
		{name: "task create", call: func() error {
			_, err := application.handleTaskUpsert(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"title":"task"}`), IdempotencyKey: "scope-task"})
			return err
		}},
		{name: "decision create", call: func() error {
			_, err := application.handleDecisionRecord(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"title":"decision","outcome":"outcome"}`), IdempotencyKey: "scope-decision"})
			return err
		}},
		{name: "manual import", call: func() error {
			_, err := application.handleKnowledgeImportManual(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"name":"notes","content":"content"}`), IdempotencyKey: "scope-import"})
			return err
		}},
		{name: "trash", call: func() error {
			_, err := application.handleEntityTrash(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"entityType":"project","id":"project","expectedVersion":1}`), IdempotencyKey: "scope-trash"})
			return err
		}},
		{name: "projects resource", call: func() error {
			_, err := application.readWorkProjects(ctx, ResourceRequest{Principal: principal, URI: ResourceWorkProjectsURI})
			return err
		}},
		{name: "tasks resource", call: func() error {
			_, err := application.readWorkTasks(ctx, ResourceRequest{Principal: principal, URI: ResourceWorkTasksURI})
			return err
		}},
		{name: "decisions resource", call: func() error {
			_, err := application.readWorkDecisions(ctx, ResourceRequest{Principal: principal, URI: ResourceWorkDecisionsURI})
			return err
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.call(); !errors.Is(err, ErrApplicationScopeMismatch) {
				t.Fatalf("error = %v, want ErrApplicationScopeMismatch", err)
			}
		})
	}

	if _, _, _, err := application.productScopes(nil, fullApplicationPrincipal(userID, productapp.WorkspaceIDForUser(userID))); err != nil {
		t.Fatalf("nil context product scope = %v", err)
	}
}

func TestApplicationCompositionRejectsInvalidRegistrationInputs(t *testing.T) {
	if _, err := newApplication(applicationDependencies{}); !errors.Is(err, ErrInvalidApplicationServices) {
		t.Fatalf("invalid dependencies error = %v", err)
	}
	var nilApplication *Application
	if err := nilApplication.Register(NewRegistry()); !errors.Is(err, ErrInvalidApplicationServices) {
		t.Fatalf("nil application Register() error = %v", err)
	}
	application, _, _, _ := newApplicationFixture(t)
	if err := application.Register(nil); !errors.Is(err, ErrInvalidTool) {
		t.Fatalf("nil registry Register() error = %v", err)
	}
	registry := NewRegistry()
	if err := registry.RegisterResource(Resource{URI: ResourceWorkProjectsURI, Reader: noopResource}); err != nil {
		t.Fatal(err)
	}
	if err := application.Register(registry); !errors.Is(err, ErrDuplicateResource) {
		t.Fatalf("duplicate resource Register() error = %v", err)
	}
}
