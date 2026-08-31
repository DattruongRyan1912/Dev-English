package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	productapp "github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

func TestFullApplicationMCPCanonicalReadBoundaries(t *testing.T) {
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
	userID := "canonical-read-user"
	principal := fullApplicationPrincipal(userID, productapp.WorkspaceIDForUser(userID), ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead, ScopeWorkWrite)
	ctx := context.Background()

	readCases := []struct {
		name string
		call func() (CallToolResult, error)
		want error
	}{
		{name: "claim", call: func() (CallToolResult, error) {
			return application.handleKnowledgeGetClaim(ctx, Invocation{Principal: principal, Arguments: applicationIDArgumentsJSON("missing-claim")})
		}, want: knowledge.ErrNotFound},
		{name: "chunk", call: func() (CallToolResult, error) {
			return application.handleKnowledgeGetChunk(ctx, Invocation{Principal: principal, Arguments: applicationIDArgumentsJSON("missing-chunk")})
		}, want: knowledge.ErrNotFound},
		{name: "project", call: func() (CallToolResult, error) {
			return application.handleWorkGetProject(ctx, Invocation{Principal: principal, Arguments: applicationIDArgumentsJSON("missing-project")})
		}, want: work.ErrNotFound},
		{name: "task", call: func() (CallToolResult, error) {
			return application.handleWorkGetTask(ctx, Invocation{Principal: principal, Arguments: applicationIDArgumentsJSON("missing-task")})
		}, want: work.ErrNotFound},
		{name: "decision", call: func() (CallToolResult, error) {
			return application.handleWorkGetDecision(ctx, Invocation{Principal: principal, Arguments: applicationIDArgumentsJSON("missing-decision")})
		}, want: work.ErrNotFound},
	}
	for _, testCase := range readCases {
		t.Run(testCase.name+" not found", func(t *testing.T) {
			_, err := testCase.call()
			if !errors.Is(err, ErrApplicationDependency) || !errors.Is(err, testCase.want) {
				t.Fatalf("error = %v, want redacted dependency wrapping %v", err, testCase.want)
			}
		})
	}

	userContext := store.WithUser(context.Background(), userID)
	project, err := product.CreateProject(userContext, work.CreateProjectInput{Name: "Canonical MCP project"}, "canonical-read-project")
	if err != nil {
		t.Fatal(err)
	}
	task, err := product.CreateTask(userContext, work.CreateTaskInput{ProjectID: project.ID, Title: "Canonical MCP task"}, "canonical-read-task")
	if err != nil {
		t.Fatal(err)
	}
	decision, err := product.CreateDecision(userContext, work.CreateDecisionInput{ProjectID: project.ID, Title: "Canonical MCP decision", Outcome: "Keep the source-backed path"}, "canonical-read-decision")
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range []struct {
		name string
		id   string
		call func(json.RawMessage) (CallToolResult, error)
	}{
		{name: "project", id: project.ID, call: func(arguments json.RawMessage) (CallToolResult, error) {
			return application.handleWorkGetProject(ctx, Invocation{Principal: principal, Arguments: arguments})
		}},
		{name: "task", id: task.ID, call: func(arguments json.RawMessage) (CallToolResult, error) {
			return application.handleWorkGetTask(ctx, Invocation{Principal: principal, Arguments: arguments})
		}},
		{name: "decision", id: decision.ID, call: func(arguments json.RawMessage) (CallToolResult, error) {
			return application.handleWorkGetDecision(ctx, Invocation{Principal: principal, Arguments: arguments})
		}},
	} {
		t.Run(testCase.name+" success", func(t *testing.T) {
			result, err := testCase.call(applicationIDArgumentsJSON(testCase.id))
			if err != nil || result.StructuredContent == nil {
				t.Fatalf("result = %#v, error = %v", result, err)
			}
		})
	}

	imported, err := product.ImportManualSource(userContext, productapp.ManualSourceInput{
		ID: "canonical-read-source", Name: "Canonical notes", Kind: "manual", URI: "memory://canonical-read",
		Content: "Canonical evidence remains immutable and workspace scoped.",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, knowledgeScope, _, err := product.Scope(userContext)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	claim := knowledge.KnowledgeClaim{
		ID: "canonical-read-claim", WorkspaceID: knowledgeScope.ID,
		Statement: "Canonical evidence remains immutable.", Certainty: knowledge.ClaimCanonical,
		Freshness: knowledge.ClaimCurrent, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := product.Knowledge.CreateClaimBundle(userContext, knowledgeScope, claim, []knowledge.ClaimEvidence{{
		ID: "canonical-read-evidence", WorkspaceID: knowledgeScope.ID, ClaimID: claim.ID,
		SourceRevisionID: imported.RevisionID, ChunkID: imported.ChunkID,
		Locator: "memory://canonical-read#1", Quote: "Canonical evidence remains immutable and workspace scoped.",
		Freshness: knowledge.EvidenceCurrent, CreatedAt: now,
	}}); err != nil {
		t.Fatal(err)
	}
	claimResult, err := application.handleKnowledgeGetClaim(ctx, Invocation{Principal: principal, Arguments: applicationIDArgumentsJSON(claim.ID)})
	if err != nil {
		t.Fatalf("canonical claim read error = %v", err)
	}
	var claimPayload knowledgeClaimDetailPayload
	claimJSON, err := json.Marshal(claimResult.StructuredContent)
	if err != nil || json.Unmarshal(claimJSON, &claimPayload) != nil || len(claimPayload.Evidence) != 1 {
		t.Fatalf("canonical claim payload = %+v, error = %v", claimPayload, err)
	}
	chunkResult, err := application.handleKnowledgeGetChunk(ctx, Invocation{Principal: principal, Arguments: applicationIDArgumentsJSON(imported.ChunkID)})
	if err != nil {
		t.Fatalf("canonical chunk read error = %v", err)
	}
	var chunkPayload knowledgeChunkPayload
	chunkJSON, err := json.Marshal(chunkResult.StructuredContent)
	if err != nil || json.Unmarshal(chunkJSON, &chunkPayload) != nil || chunkPayload.ID != imported.ChunkID {
		t.Fatalf("canonical chunk payload = %+v, error = %v", chunkPayload, err)
	}

	for _, testCase := range []struct {
		name string
		kind string
	}{
		{name: "source", kind: "source"},
		{name: "item", kind: "item"},
		{name: "revision", kind: "revision"},
		{name: "claim", kind: "claim"},
		{name: "chunk", kind: "chunk"},
	} {
		t.Run("knowledge_get "+testCase.name+" missing", func(t *testing.T) {
			_, err := application.handleKnowledgeGet(ctx, Invocation{
				Principal: principal,
				Arguments: json.RawMessage(`{"kind":` + mustJSONValue(testCase.kind) + `,"id":"missing-knowledge-record"}`),
			})
			if !errors.Is(err, ErrApplicationDependency) || !errors.Is(err, knowledge.ErrNotFound) {
				t.Fatalf("error = %v, want redacted dependency wrapping knowledge.ErrNotFound", err)
			}
		})
	}

	for _, testCase := range []struct {
		name string
		call func() (CallToolResult, error)
	}{
		{name: "project create", call: func() (CallToolResult, error) {
			return application.handleProjectCreate(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"name":""}`), IdempotencyKey: "invalid-project"})
		}},
		{name: "task create missing project", call: func() (CallToolResult, error) {
			return application.handleTaskUpsert(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"projectId":"missing-project","title":"Task"}`), IdempotencyKey: "invalid-task"})
		}},
		{name: "task update missing task", call: func() (CallToolResult, error) {
			return application.handleTaskUpsert(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"id":"missing-task","title":"Task","expectedVersion":1}`), IdempotencyKey: "missing-task-update"})
		}},
		{name: "decision create missing project", call: func() (CallToolResult, error) {
			return application.handleDecisionRecord(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"projectId":"missing-project","title":"Decision","outcome":"Outcome"}`), IdempotencyKey: "invalid-decision"})
		}},
		{name: "decision update missing decision", call: func() (CallToolResult, error) {
			return application.handleDecisionRecord(ctx, Invocation{Principal: principal, Arguments: json.RawMessage(`{"id":"missing-decision","outcome":"Outcome","expectedVersion":1}`), IdempotencyKey: "missing-decision-update"})
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := testCase.call()
			if err == nil {
				t.Fatal("call unexpectedly succeeded")
			}
			if testCase.name == "project create" {
				assertApplicationRPCCode(t, err, InvalidParams)
				return
			}
			if !errors.Is(err, work.ErrNotFound) {
				t.Fatalf("error = %v, want work.ErrNotFound", err)
			}
			assertApplicationRPCCode(t, err, NotFoundError)
		})
	}

	if _, err := application.handleEntityTrash(ctx, Invocation{
		Principal: principal, Arguments: json.RawMessage(`{"entityType":"unknown","id":"entity","expectedVersion":1}`), IdempotencyKey: "invalid-trash",
	}); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("unknown trash entity error = %v, want ErrInvalidParams", err)
	}
	forged := principal
	forged.WorkspaceID = "forged-workspace"
	if _, err := application.handleProjectList(ctx, Invocation{Principal: forged, Arguments: json.RawMessage(`{}`)}); !errors.Is(err, ErrApplicationDependency) || !errors.Is(err, ErrApplicationScopeMismatch) {
		t.Fatalf("forged project-list scope error = %v, want redacted scope mismatch", err)
	}
}
