package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/actions"
	productapp "github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

const (
	applicationMaxArgumentsBytes      = 64 << 10
	applicationMaxIdentifierBytes     = 256
	applicationMaxConversationIDBytes = 256
	applicationMaxMessageBytes        = 32 << 10
	applicationMaxListItems           = 50

	ResourceWorkProjectsURI  = "devenglish://work/projects"
	ResourceWorkTasksURI     = "devenglish://work/tasks"
	ResourceWorkDecisionsURI = "devenglish://work/decisions"
)

var (
	ErrInvalidApplicationServices = errors.New("invalid MCP application services")
	ErrInvalidApplicationIdentity = errors.New("invalid MCP application identity")
	ErrApplicationScopeMismatch   = errors.New("MCP application response is outside the authenticated scope")
	ErrApplicationDependency      = errors.New("MCP application dependency failed")
	ErrApplicationEncoding        = errors.New("MCP application response encoding failed")
)

// assistantAsker is the provider-neutral assistant boundary. The adapter only
// forwards the verified token scope and never creates action bindings.
type assistantAsker interface {
	Ask(context.Context, assistant.AskRequest) (assistant.AssistantResponse, error)
}

// knowledgeReader is a package-private test seam. Production construction
// uses *knowledge.Service through ApplicationServices below, so a repository
// cannot be injected into the public MCP composition boundary.
type knowledgeReader interface {
	GetClaim(context.Context, knowledge.WorkspaceScope, string) (knowledge.KnowledgeClaim, error)
	GetChunk(context.Context, knowledge.WorkspaceScope, string) (knowledge.KnowledgeChunk, error)
}

type workReader interface {
	GetProject(context.Context, work.Scope, string) (work.Project, error)
	ListProjects(context.Context, work.Scope, work.ListOptions) ([]work.Project, error)
	GetTask(context.Context, work.Scope, string) (work.Task, error)
	ListTasks(context.Context, work.Scope, string, work.ListOptions) ([]work.Task, error)
	GetDecision(context.Context, work.Scope, string) (work.Decision, error)
	ListDecisions(context.Context, work.Scope, string, work.ListOptions) ([]work.Decision, error)
}

type ApplicationServices struct {
	Assistant *assistant.Service
	Knowledge *knowledge.Service
	Work      *work.Service
	// Product is optional for the original read-only test seam. When present,
	// production MCP calls use the canonical application composition so
	// conversation persistence, hybrid retrieval and workspace derivation stay
	// identical to REST.
	Product *productapp.App
	Actions *actions.Service
}

// applicationDependencies is an unexported seam used by package tests. The
// exported constructor below only accepts canonical service types.
type applicationDependencies struct {
	Assistant assistantAsker
	Knowledge knowledgeReader
	Work      workReader
	Product   *productapp.App
	Actions   *actions.Service
	Full      bool
}

// Application wires MCP read and mutation tools/resources to application
// services. It deliberately does not mount HTTP routes; Handler owns
// transport, authentication and replay behavior.
type Application struct {
	services applicationDependencies
	full     bool
}

func NewApplication(services ApplicationServices) (*Application, error) {
	return newApplication(applicationDependencies{
		Assistant: services.Assistant,
		Knowledge: services.Knowledge,
		Work:      services.Work,
		Product:   services.Product,
		Actions:   services.Actions,
		Full:      services.Product != nil || services.Actions != nil,
	})
}

func newApplication(services applicationDependencies) (*Application, error) {
	if !usableDependency(services.Assistant) || !usableDependency(services.Knowledge) || !usableDependency(services.Work) {
		return nil, ErrInvalidApplicationServices
	}
	return &Application{services: services, full: services.Full}, nil
}

// Register installs the MCP application surface into an existing registry.
// The transport, auth, nonce and replay behavior remain owned by Handler and
// Registry.
func (application *Application) Register(registry *Registry) error {
	if application == nil || !usableDependency(application.services.Assistant) || !usableDependency(application.services.Knowledge) || !usableDependency(application.services.Work) {
		return ErrInvalidApplicationServices
	}
	if registry == nil {
		return ErrInvalidTool
	}
	tools := applicationTools(application)
	resources := applicationResources(application)
	for _, tool := range tools {
		if _, exists := registry.tool(tool.Name); exists {
			return ErrDuplicateTool
		}
	}
	for _, resource := range resources {
		if _, exists := registry.resource(resource.URI); exists {
			return ErrDuplicateResource
		}
	}
	for _, tool := range tools {
		if err := registry.RegisterTool(tool); err != nil {
			return err
		}
	}
	for _, resource := range resources {
		if err := registry.RegisterResource(resource); err != nil {
			return err
		}
	}
	return nil
}

func applicationTools(application *Application) []Tool {
	tools := []Tool{
		{
			Name: "assistant.ask", Description: "Ask the grounded assistant using verified workspace context.",
			InputSchema: applicationAskSchema, RequiredScope: ScopeAssistantUse,
			Handler: application.handleAssistantAsk,
		},
		{
			Name: "knowledge.get_claim", Description: "Read one canonical knowledge claim in the authenticated workspace.",
			InputSchema: applicationIDSchema, RequiredScope: ScopeKnowledgeRead,
			Handler: application.handleKnowledgeGetClaim,
		},
		{
			Name: "knowledge.get_chunk", Description: "Read one knowledge chunk in the authenticated workspace.",
			InputSchema: applicationIDSchema, RequiredScope: ScopeKnowledgeRead,
			Handler: application.handleKnowledgeGetChunk,
		},
		{
			Name: "work.get_project", Description: "Read one project in the authenticated workspace and user scope.",
			InputSchema: applicationIDSchema, RequiredScope: ScopeWorkRead,
			Handler: application.handleWorkGetProject,
		},
		{
			Name: "work.get_task", Description: "Read one task in the authenticated workspace and user scope.",
			InputSchema: applicationIDSchema, RequiredScope: ScopeWorkRead,
			Handler: application.handleWorkGetTask,
		},
		{
			Name: "work.get_decision", Description: "Read one decision in the authenticated workspace and user scope.",
			InputSchema: applicationIDSchema, RequiredScope: ScopeWorkRead,
			Handler: application.handleWorkGetDecision,
		},
	}
	if application == nil || !application.full {
		return tools
	}
	// Underscore names are the stable V1 contract for Codex/Claude. Dotted
	// names above remain as compatibility aliases for the first MCP slice.
	tools = append(tools,
		Tool{Name: "assistant_ask", Description: "Ask the grounded assistant using verified workspace context.", InputSchema: applicationAskSchema, RequiredScope: ScopeAssistantUse, Handler: application.handleAssistantAsk},
		Tool{Name: "knowledge_search", Description: "Search canonical knowledge and work context in the authenticated workspace.", InputSchema: knowledgeSearchSchema, RequiredScope: ScopeKnowledgeRead, Handler: application.handleKnowledgeSearch},
		Tool{Name: "knowledge_get", Description: "Read one canonical knowledge source, item, revision, claim, or chunk.", InputSchema: knowledgeGetSchema, RequiredScope: ScopeKnowledgeRead, Handler: application.handleKnowledgeGet},
		Tool{Name: "project_list", Description: "List canonical projects in the authenticated workspace and user scope.", InputSchema: listSchema, RequiredScope: ScopeWorkRead, Handler: application.handleProjectList},
		Tool{Name: "task_list", Description: "List canonical tasks in the authenticated workspace and user scope.", InputSchema: taskListSchema, RequiredScope: ScopeWorkRead, Handler: application.handleTaskList},
		Tool{Name: "decision_get", Description: "Read one canonical decision in the authenticated workspace and user scope.", InputSchema: applicationIDSchema, RequiredScope: ScopeWorkRead, Handler: application.handleWorkGetDecision},
	)
	if application.services.Product != nil {
		tools = append(tools,
			Tool{Name: "project_create", Description: "Create one canonical project. Requires an MCP idempotency key.", InputSchema: projectCreateSchema, RequiredScope: ScopeWorkWrite, Mutating: true, Handler: application.handleProjectCreate},
			Tool{Name: "task_upsert", Description: "Create or version-update one canonical task. Requires an MCP idempotency key.", InputSchema: taskUpsertSchema, RequiredScope: ScopeWorkWrite, Mutating: true, Handler: application.handleTaskUpsert},
			Tool{Name: "decision_record", Description: "Create or version-update one canonical decision. Requires an MCP idempotency key.", InputSchema: decisionRecordSchema, RequiredScope: ScopeWorkWrite, Mutating: true, Handler: application.handleDecisionRecord},
			Tool{Name: "knowledge_import_manual", Description: "Import a manually supplied knowledge source as an immutable revision. Requires an MCP idempotency key.", InputSchema: knowledgeImportSchema, RequiredScope: ScopeWorkWrite, Mutating: true, Handler: application.handleKnowledgeImportManual},
			Tool{Name: "entity_trash", Description: "Confirm a previously previewed canonical work-trash challenge.", InputSchema: entityTrashSchema, RequiredScope: ScopeWorkWrite, Mutating: true, Handler: application.handleEntityTrash},
		)
	}
	if application.services.Actions != nil {
		tools = append(tools,
			Tool{Name: "github_issue_create", Description: "Confirm a previously previewed GitHub issue creation challenge.", InputSchema: githubIssueCreateSchema, RequiredScope: ScopeGitHubWrite, Mutating: true, Handler: application.handleGitHubIssueCreate},
			Tool{Name: "github_issue_comment", Description: "Confirm a previously previewed GitHub issue comment challenge.", InputSchema: githubIssueCommentSchema, RequiredScope: ScopeGitHubWrite, Mutating: true, Handler: application.handleGitHubIssueComment},
			Tool{Name: "github_issue_label", Description: "Confirm a previously previewed GitHub issue label challenge.", InputSchema: githubIssueLabelSchema, RequiredScope: ScopeGitHubWrite, Mutating: true, Handler: application.handleGitHubIssueLabel},
		)
	}
	return tools
}

func applicationResources(application *Application) []Resource {
	return []Resource{
		{
			URI: ResourceWorkProjectsURI, Name: "Work projects", Description: "Bounded projects for the authenticated workspace and user.",
			MIMEType: "application/json", RequiredScope: ScopeWorkRead,
			Reader: application.readWorkProjects,
		},
		{
			URI: ResourceWorkTasksURI, Name: "Work tasks", Description: "Bounded tasks for the authenticated workspace and user.",
			MIMEType: "application/json", RequiredScope: ScopeWorkRead,
			Reader: application.readWorkTasks,
		},
		{
			URI: ResourceWorkDecisionsURI, Name: "Work decisions", Description: "Bounded decisions for the authenticated workspace and user.",
			MIMEType: "application/json", RequiredScope: ScopeWorkRead,
			Reader: application.readWorkDecisions,
		},
	}
}

var applicationAskSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"conversationId":{"type":"string","maxLength":256},"message":{"type":"string","minLength":1,"maxLength":32768}},"required":["message"]}`)
var applicationIDSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"id":{"type":"string","minLength":1,"maxLength":256}},"required":["id"]}`)
var listSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"includeTrashed":{"type":"boolean"},"limit":{"type":"integer","minimum":1,"maximum":50}}}`)
var taskListSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"projectId":{"type":"string","maxLength":256},"includeTrashed":{"type":"boolean"},"limit":{"type":"integer","minimum":1,"maximum":50}}}`)
var knowledgeSearchSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"query":{"type":"string","minLength":1,"maxLength":32768},"limit":{"type":"integer","minimum":1,"maximum":50}},"required":["query"]}`)
var knowledgeGetSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"kind":{"type":"string","enum":["source","item","revision","claim","chunk"]},"id":{"type":"string","minLength":1,"maxLength":256}},"required":["kind","id"]}`)
var projectCreateSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"id":{"type":"string","maxLength":256},"name":{"type":"string","minLength":1,"maxLength":200},"description":{"type":"string","maxLength":20000},"status":{"type":"string","enum":["active","on_hold","completed","archived"]}},"required":["name"]}`)
var taskUpsertSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"id":{"type":"string","maxLength":256},"projectId":{"type":"string","maxLength":256},"title":{"type":"string","maxLength":500},"description":{"type":"string","maxLength":20000},"status":{"type":"string","enum":["backlog","todo","in_progress","blocked","done","cancelled"]},"priority":{"type":"string","enum":["low","normal","high","urgent"]},"dueAt":{"type":"string","format":"date-time"},"expectedVersion":{"type":"integer","minimum":1}}}`)
var decisionRecordSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"id":{"type":"string","maxLength":256},"projectId":{"type":"string","maxLength":256},"title":{"type":"string","maxLength":20000},"context":{"type":"string","maxLength":20000},"outcome":{"type":"string","maxLength":20000},"rationale":{"type":"string","maxLength":20000},"status":{"type":"string","enum":["proposed","accepted","rejected","superseded"]},"expectedVersion":{"type":"integer","minimum":1}}}`)
var knowledgeImportSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"id":{"type":"string","maxLength":256},"name":{"type":"string","minLength":1,"maxLength":200},"kind":{"type":"string","maxLength":200},"uri":{"type":"string","maxLength":2048},"mimeType":{"type":"string","maxLength":200},"content":{"type":"string","minLength":1,"maxLength":524288}},"required":["name","content"]}`)
var entityTrashSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"challengeId":{"type":"string","minLength":1,"maxLength":256},"entityType":{"type":"string","enum":["project","task","decision"]},"id":{"type":"string","minLength":1,"maxLength":256},"expectedVersion":{"type":"integer","minimum":1}},"required":["challengeId","entityType","id","expectedVersion"]}`)
var githubIssueCreateSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"challengeId":{"type":"string","minLength":1,"maxLength":256},"repository":{"type":"string","minLength":1,"maxLength":256},"title":{"type":"string","minLength":1,"maxLength":500},"body":{"type":"string","maxLength":20000},"expectedRevision":{"type":"string","maxLength":256}},"required":["challengeId","repository","title"]}`)
var githubIssueCommentSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"challengeId":{"type":"string","minLength":1,"maxLength":256},"repository":{"type":"string","minLength":1,"maxLength":256},"issue":{"type":"integer","minimum":1},"body":{"type":"string","minLength":1,"maxLength":20000},"expectedRevision":{"type":"string","maxLength":256}},"required":["challengeId","repository","issue","body"]}`)
var githubIssueLabelSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"challengeId":{"type":"string","minLength":1,"maxLength":256},"repository":{"type":"string","minLength":1,"maxLength":256},"issue":{"type":"integer","minimum":1},"labels":{"type":"array","items":{"type":"string","maxLength":100},"minItems":1,"maxItems":50},"expectedRevision":{"type":"string","maxLength":256}},"required":["challengeId","repository","issue","labels"]}`)

type assistantAskArguments struct {
	ConversationID string `json:"conversationId"`
	Message        string `json:"message"`
}

type applicationIDArguments struct {
	ID string `json:"id"`
}

type knowledgeSearchArguments struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type knowledgeGetArguments struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type listArguments struct {
	IncludeTrashed bool `json:"includeTrashed"`
	Limit          int  `json:"limit"`
}

type taskListArguments struct {
	ProjectID      string `json:"projectId"`
	IncludeTrashed bool   `json:"includeTrashed"`
	Limit          int    `json:"limit"`
}

type projectCreateArguments struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Status      work.ProjectStatus `json:"status"`
}

type taskUpsertArguments struct {
	ID              string             `json:"id"`
	ProjectID       *string            `json:"projectId"`
	Title           *string            `json:"title"`
	Description     *string            `json:"description"`
	Status          *work.TaskStatus   `json:"status"`
	Priority        *work.TaskPriority `json:"priority"`
	DueAt           *time.Time         `json:"dueAt"`
	ExpectedVersion int64              `json:"expectedVersion"`
}

type decisionRecordArguments struct {
	ID              string               `json:"id"`
	ProjectID       *string              `json:"projectId"`
	Title           *string              `json:"title"`
	Context         *string              `json:"context"`
	Outcome         *string              `json:"outcome"`
	Rationale       *string              `json:"rationale"`
	Status          *work.DecisionStatus `json:"status"`
	ExpectedVersion int64                `json:"expectedVersion"`
}

type entityTrashArguments struct {
	ChallengeID     string          `json:"challengeId"`
	EntityType      work.EntityType `json:"entityType"`
	ID              string          `json:"id"`
	ExpectedVersion int64           `json:"expectedVersion"`
}

type githubIssueCreateArguments struct {
	ChallengeID      string `json:"challengeId"`
	Repository       string `json:"repository"`
	Title            string `json:"title"`
	Body             string `json:"body"`
	ExpectedRevision string `json:"expectedRevision"`
}

type githubIssueCommentArguments struct {
	ChallengeID      string `json:"challengeId"`
	Repository       string `json:"repository"`
	Issue            int64  `json:"issue"`
	Body             string `json:"body"`
	ExpectedRevision string `json:"expectedRevision"`
}

type githubIssueLabelArguments struct {
	ChallengeID      string   `json:"challengeId"`
	Repository       string   `json:"repository"`
	Issue            int64    `json:"issue"`
	Labels           []string `json:"labels"`
	ExpectedRevision string   `json:"expectedRevision"`
}

func (application *Application) handleAssistantAsk(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	principalScope, _, _, err := scopesForPrincipal(invocation.Principal)
	if err != nil {
		return CallToolResult{}, applicationIdentityRPCError()
	}
	var arguments assistantAskArguments
	if err := decodeApplicationArguments(invocation.Arguments, &arguments); err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	if err := validateApplicationText(arguments.Message, applicationMaxMessageBytes, true); err != nil ||
		!validMCPIdentifier(arguments.ConversationID, applicationMaxConversationIDBytes, false) {
		return CallToolResult{}, applicationParamsRPCError()
	}
	if application.services.Product != nil {
		boundContext, _, _, err := application.productScopes(ctx, invocation.Principal)
		if err != nil {
			return CallToolResult{}, err
		}
		result, err := application.services.Product.Ask(boundContext, arguments.ConversationID, arguments.Message)
		if err != nil {
			return CallToolResult{}, safeApplicationDependency("assistant.ask", err)
		}
		if result.Response.Scope != principalScope {
			return CallToolResult{}, safeApplicationDependency("assistant.ask.scope", ErrApplicationScopeMismatch)
		}
		return applicationJSONToolResult(result)
	}
	response, err := application.services.Assistant.Ask(ctx, assistant.AskRequest{
		Scope:          principalScope,
		ConversationID: arguments.ConversationID,
		Message:        arguments.Message,
	})
	if err != nil {
		return CallToolResult{}, safeApplicationDependency("assistant.ask", err)
	}
	if response.Scope != principalScope {
		return CallToolResult{}, safeApplicationDependency("assistant.ask.scope", ErrApplicationScopeMismatch)
	}
	return applicationJSONToolResult(response)
}

func (application *Application) handleKnowledgeGetClaim(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	_, knowledgeScope, _, err := scopesForPrincipal(invocation.Principal)
	if err != nil {
		return CallToolResult{}, applicationIdentityRPCError()
	}
	if application.services.Product != nil {
		boundContext, _, productKnowledgeScope, err := application.productScopes(ctx, invocation.Principal)
		if err != nil {
			return CallToolResult{}, err
		}
		arguments, err := decodeIDArguments(invocation.Arguments)
		if err != nil {
			return CallToolResult{}, applicationParamsRPCError()
		}
		claim, err := application.services.Product.Knowledge.GetClaim(boundContext, productKnowledgeScope, arguments.ID)
		if err != nil {
			return CallToolResult{}, safeApplicationDependency("knowledge.get_claim", err)
		}
		evidence, err := application.services.Product.Knowledge.ListClaimEvidence(boundContext, productKnowledgeScope, claim.ID)
		if err != nil {
			return CallToolResult{}, safeApplicationDependency("knowledge.get_claim.evidence", err)
		}
		if claim.WorkspaceID != productKnowledgeScope.ID {
			return CallToolResult{}, safeApplicationDependency("knowledge.get_claim.scope", ErrApplicationScopeMismatch)
		}
		return applicationJSONToolResult(toKnowledgeClaimDetailPayload(claim, evidence))
	}
	arguments, err := decodeIDArguments(invocation.Arguments)
	if err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	claim, err := application.services.Knowledge.GetClaim(ctx, knowledgeScope, arguments.ID)
	if err != nil {
		return CallToolResult{}, safeApplicationDependency("knowledge.get_claim", err)
	}
	if claim.WorkspaceID != knowledgeScope.ID {
		return CallToolResult{}, safeApplicationDependency("knowledge.get_claim.scope", ErrApplicationScopeMismatch)
	}
	return applicationJSONToolResult(toKnowledgeClaimPayload(claim))
}

func (application *Application) handleKnowledgeGetChunk(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	_, knowledgeScope, _, err := scopesForPrincipal(invocation.Principal)
	if err != nil {
		return CallToolResult{}, applicationIdentityRPCError()
	}
	if application.services.Product != nil {
		boundContext, _, productKnowledgeScope, err := application.productScopes(ctx, invocation.Principal)
		if err != nil {
			return CallToolResult{}, err
		}
		arguments, err := decodeIDArguments(invocation.Arguments)
		if err != nil {
			return CallToolResult{}, applicationParamsRPCError()
		}
		chunk, err := application.services.Product.Knowledge.GetChunk(boundContext, productKnowledgeScope, arguments.ID)
		if err != nil {
			return CallToolResult{}, safeApplicationDependency("knowledge.get_chunk", err)
		}
		if chunk.WorkspaceID != productKnowledgeScope.ID {
			return CallToolResult{}, safeApplicationDependency("knowledge.get_chunk.scope", ErrApplicationScopeMismatch)
		}
		return applicationJSONToolResult(toKnowledgeChunkPayload(chunk))
	}
	arguments, err := decodeIDArguments(invocation.Arguments)
	if err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	chunk, err := application.services.Knowledge.GetChunk(ctx, knowledgeScope, arguments.ID)
	if err != nil {
		return CallToolResult{}, safeApplicationDependency("knowledge.get_chunk", err)
	}
	if chunk.WorkspaceID != knowledgeScope.ID {
		return CallToolResult{}, safeApplicationDependency("knowledge.get_chunk.scope", ErrApplicationScopeMismatch)
	}
	return applicationJSONToolResult(toKnowledgeChunkPayload(chunk))
}

func (application *Application) handleWorkGetProject(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	_, _, scope, err := scopesForPrincipal(invocation.Principal)
	if err != nil {
		return CallToolResult{}, applicationIdentityRPCError()
	}
	if application.services.Product != nil {
		boundContext, productScope, _, err := application.productScopes(ctx, invocation.Principal)
		if err != nil {
			return CallToolResult{}, err
		}
		scope = productScope
		ctx = boundContext
	}
	arguments, err := decodeIDArguments(invocation.Arguments)
	if err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	project, err := application.services.Work.GetProject(ctx, scope, arguments.ID)
	if err != nil {
		return CallToolResult{}, safeApplicationDependency("work.get_project", err)
	}
	if !projectInApplicationScope(project, scope) {
		return CallToolResult{}, safeApplicationDependency("work.get_project.scope", ErrApplicationScopeMismatch)
	}
	return applicationJSONToolResult(project)
}

func (application *Application) handleWorkGetTask(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	_, _, scope, err := scopesForPrincipal(invocation.Principal)
	if err != nil {
		return CallToolResult{}, applicationIdentityRPCError()
	}
	if application.services.Product != nil {
		boundContext, productScope, _, err := application.productScopes(ctx, invocation.Principal)
		if err != nil {
			return CallToolResult{}, err
		}
		scope = productScope
		ctx = boundContext
	}
	arguments, err := decodeIDArguments(invocation.Arguments)
	if err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	task, err := application.services.Work.GetTask(ctx, scope, arguments.ID)
	if err != nil {
		return CallToolResult{}, safeApplicationDependency("work.get_task", err)
	}
	if !taskInApplicationScope(task, scope) {
		return CallToolResult{}, safeApplicationDependency("work.get_task.scope", ErrApplicationScopeMismatch)
	}
	return applicationJSONToolResult(task)
}

func (application *Application) handleWorkGetDecision(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	_, _, scope, err := scopesForPrincipal(invocation.Principal)
	if err != nil {
		return CallToolResult{}, applicationIdentityRPCError()
	}
	if application.services.Product != nil {
		boundContext, productScope, _, err := application.productScopes(ctx, invocation.Principal)
		if err != nil {
			return CallToolResult{}, err
		}
		scope = productScope
		ctx = boundContext
	}
	arguments, err := decodeIDArguments(invocation.Arguments)
	if err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	decision, err := application.services.Work.GetDecision(ctx, scope, arguments.ID)
	if err != nil {
		return CallToolResult{}, safeApplicationDependency("work.get_decision", err)
	}
	if !decisionInApplicationScope(decision, scope) {
		return CallToolResult{}, safeApplicationDependency("work.get_decision.scope", ErrApplicationScopeMismatch)
	}
	return applicationJSONToolResult(decision)
}

func (application *Application) handleKnowledgeSearch(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	if application == nil || application.services.Product == nil && application.services.Knowledge == nil {
		return CallToolResult{}, safeApplicationDependency("knowledge.search", ErrInvalidApplicationServices)
	}
	var arguments knowledgeSearchArguments
	if err := decodeApplicationArguments(invocation.Arguments, &arguments); err != nil || validateApplicationText(arguments.Query, applicationMaxMessageBytes, true) != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	limit := normalizeApplicationLimit(arguments.Limit)
	if application.services.Product != nil {
		boundContext, _, _, err := application.productScopes(ctx, invocation.Principal)
		if err != nil {
			return CallToolResult{}, err
		}
		hits, err := application.services.Product.SearchKnowledge(boundContext, arguments.Query, limit)
		if err != nil {
			return CallToolResult{}, safeApplicationDependency("knowledge.search", err)
		}
		return applicationJSONToolResult(hits)
	}
	_, knowledgeScope, _, err := scopesForPrincipal(invocation.Principal)
	if err != nil {
		return CallToolResult{}, applicationIdentityRPCError()
	}
	searcher, ok := application.services.Knowledge.(interface {
		Search(context.Context, knowledge.WorkspaceScope, string, int) ([]knowledge.SearchResult, error)
	})
	if !ok {
		return CallToolResult{}, safeApplicationDependency("knowledge.search", knowledge.ErrUnsupportedRead)
	}
	items, err := searcher.Search(ctx, knowledgeScope, arguments.Query, limit)
	if err != nil {
		return CallToolResult{}, safeApplicationDependency("knowledge.search", err)
	}
	return applicationJSONToolResult(toKnowledgeSearchPayloads(items))
}

func (application *Application) handleKnowledgeGet(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	var arguments knowledgeGetArguments
	if err := decodeApplicationArguments(invocation.Arguments, &arguments); err != nil || !validMCPIdentifier(arguments.ID, applicationMaxIdentifierBytes, true) {
		return CallToolResult{}, applicationParamsRPCError()
	}
	if arguments.Kind != "source" && arguments.Kind != "item" && arguments.Kind != "revision" && arguments.Kind != "claim" && arguments.Kind != "chunk" {
		return CallToolResult{}, applicationParamsRPCError()
	}
	if application.services.Product == nil {
		if arguments.Kind == "claim" {
			return application.handleKnowledgeGetClaim(ctx, Invocation{Principal: invocation.Principal, Arguments: applicationIDArgumentsJSON(arguments.ID)})
		}
		if arguments.Kind == "chunk" {
			return application.handleKnowledgeGetChunk(ctx, Invocation{Principal: invocation.Principal, Arguments: applicationIDArgumentsJSON(arguments.ID)})
		}
		return CallToolResult{}, safeApplicationDependency("knowledge.get", knowledge.ErrUnsupportedRead)
	}
	boundContext, _, knowledgeScope, err := application.productScopes(ctx, invocation.Principal)
	if err != nil {
		return CallToolResult{}, err
	}
	switch arguments.Kind {
	case "source":
		item, err := application.services.Product.Knowledge.GetSource(boundContext, knowledgeScope, arguments.ID)
		if err != nil {
			return CallToolResult{}, safeApplicationDependency("knowledge.get.source", err)
		}
		return applicationJSONToolResult(toKnowledgeSourcePayload(item))
	case "item":
		item, err := application.services.Product.Knowledge.GetSourceItem(boundContext, knowledgeScope, arguments.ID)
		if err != nil {
			return CallToolResult{}, safeApplicationDependency("knowledge.get.item", err)
		}
		return applicationJSONToolResult(toKnowledgeItemPayload(item))
	case "revision":
		item, err := application.services.Product.Knowledge.GetRevision(boundContext, knowledgeScope, arguments.ID)
		if err != nil {
			return CallToolResult{}, safeApplicationDependency("knowledge.get.revision", err)
		}
		return applicationJSONToolResult(toKnowledgeRevisionPayload(item))
	case "claim":
		item, err := application.services.Product.Knowledge.GetClaim(boundContext, knowledgeScope, arguments.ID)
		if err != nil {
			return CallToolResult{}, safeApplicationDependency("knowledge.get.claim", err)
		}
		evidence, err := application.services.Product.Knowledge.ListClaimEvidence(boundContext, knowledgeScope, arguments.ID)
		if err != nil {
			return CallToolResult{}, safeApplicationDependency("knowledge.get.claim.evidence", err)
		}
		return applicationJSONToolResult(toKnowledgeClaimDetailPayload(item, evidence))
	case "chunk":
		item, err := application.services.Product.Knowledge.GetChunk(boundContext, knowledgeScope, arguments.ID)
		if err != nil {
			return CallToolResult{}, safeApplicationDependency("knowledge.get.chunk", err)
		}
		return applicationJSONToolResult(toKnowledgeChunkPayload(item))
	default:
		return CallToolResult{}, applicationParamsRPCError()
	}
}

func (application *Application) handleProjectList(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	arguments, err := decodeListArguments(invocation.Arguments)
	if err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	_, _, scope, err := scopesForPrincipal(invocation.Principal)
	if err != nil {
		return CallToolResult{}, applicationIdentityRPCError()
	}
	if application.services.Product != nil {
		boundContext, productScope, _, err := application.productScopes(ctx, invocation.Principal)
		if err != nil {
			return CallToolResult{}, err
		}
		ctx, scope = boundContext, productScope
	}
	items, err := application.services.Work.ListProjects(ctx, scope, work.ListOptions{IncludeTrashed: arguments.IncludeTrashed, Limit: normalizeApplicationLimit(arguments.Limit)})
	if err != nil {
		return CallToolResult{}, safeApplicationDependency("work.projects", err)
	}
	return applicationJSONToolResult(filterProjects(scope, items))
}

func (application *Application) handleTaskList(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	arguments, err := decodeTaskListArguments(invocation.Arguments)
	if err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	_, _, scope, err := scopesForPrincipal(invocation.Principal)
	if err != nil {
		return CallToolResult{}, applicationIdentityRPCError()
	}
	if application.services.Product != nil {
		boundContext, productScope, _, err := application.productScopes(ctx, invocation.Principal)
		if err != nil {
			return CallToolResult{}, err
		}
		ctx, scope = boundContext, productScope
	}
	items, err := application.services.Work.ListTasks(ctx, scope, arguments.ProjectID, work.ListOptions{IncludeTrashed: arguments.IncludeTrashed, Limit: normalizeApplicationLimit(arguments.Limit)})
	if err != nil {
		return CallToolResult{}, safeApplicationDependency("work.tasks", err)
	}
	return applicationJSONToolResult(filterTasks(scope, items))
}

func (application *Application) handleProjectCreate(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	if application.services.Product == nil {
		return CallToolResult{}, safeApplicationDependency("project.create", ErrInvalidApplicationServices)
	}
	key, err := invocationIdempotencyKey(invocation)
	if err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	var arguments projectCreateArguments
	if err := decodeApplicationArguments(invocation.Arguments, &arguments); err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	boundContext, _, _, err := application.productScopes(ctx, invocation.Principal)
	if err != nil {
		return CallToolResult{}, err
	}
	project, err := application.services.Product.CreateProject(boundContext, work.CreateProjectInput{ID: arguments.ID, Name: arguments.Name, Description: arguments.Description, Status: arguments.Status}, key)
	if err != nil {
		return CallToolResult{}, applicationMutationError("project.create", err)
	}
	return applicationJSONToolResult(project)
}

func (application *Application) handleTaskUpsert(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	if application.services.Product == nil {
		return CallToolResult{}, safeApplicationDependency("task.upsert", ErrInvalidApplicationServices)
	}
	key, err := invocationIdempotencyKey(invocation)
	if err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	var arguments taskUpsertArguments
	if err := decodeApplicationArguments(invocation.Arguments, &arguments); err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	if arguments.ID == "" {
		if arguments.Title == nil {
			return CallToolResult{}, applicationParamsRPCError()
		}
		input := work.CreateTaskInput{Title: *arguments.Title, DueAt: arguments.DueAt}
		if arguments.ProjectID != nil {
			input.ProjectID = *arguments.ProjectID
		}
		if arguments.Description != nil {
			input.Description = *arguments.Description
		}
		if arguments.Status != nil {
			input.Status = *arguments.Status
		}
		if arguments.Priority != nil {
			input.Priority = *arguments.Priority
		}
		boundContext, _, _, err := application.productScopes(ctx, invocation.Principal)
		if err != nil {
			return CallToolResult{}, err
		}
		task, err := application.services.Product.CreateTask(boundContext, input, key)
		if err != nil {
			return CallToolResult{}, applicationMutationError("task.create", err)
		}
		return applicationJSONToolResult(task)
	}
	if arguments.ExpectedVersion < 1 {
		return CallToolResult{}, applicationParamsRPCError()
	}
	patch := work.TaskPatch{ProjectID: arguments.ProjectID, Title: arguments.Title, Description: arguments.Description, Status: arguments.Status, Priority: arguments.Priority}
	if arguments.DueAt != nil {
		patch.DueAt = &arguments.DueAt
	}
	boundContext, scope, _, err := application.productScopes(ctx, invocation.Principal)
	if err != nil {
		return CallToolResult{}, err
	}
	task, err := application.services.Product.Work.UpdateTask(boundContext, scope, arguments.ID, patch, arguments.ExpectedVersion, key)
	if err != nil {
		return CallToolResult{}, applicationMutationError("task.update", err)
	}
	return applicationJSONToolResult(task)
}

func (application *Application) handleDecisionRecord(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	if application.services.Product == nil {
		return CallToolResult{}, safeApplicationDependency("decision.record", ErrInvalidApplicationServices)
	}
	key, err := invocationIdempotencyKey(invocation)
	if err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	var arguments decisionRecordArguments
	if err := decodeApplicationArguments(invocation.Arguments, &arguments); err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	if arguments.ID == "" {
		if arguments.Title == nil || arguments.Outcome == nil {
			return CallToolResult{}, applicationParamsRPCError()
		}
		input := work.CreateDecisionInput{Title: *arguments.Title, Outcome: *arguments.Outcome}
		if arguments.ProjectID != nil {
			input.ProjectID = *arguments.ProjectID
		}
		if arguments.Context != nil {
			input.Context = *arguments.Context
		}
		if arguments.Rationale != nil {
			input.Rationale = *arguments.Rationale
		}
		if arguments.Status != nil {
			input.Status = *arguments.Status
		}
		boundContext, _, _, err := application.productScopes(ctx, invocation.Principal)
		if err != nil {
			return CallToolResult{}, err
		}
		decision, err := application.services.Product.CreateDecision(boundContext, input, key)
		if err != nil {
			return CallToolResult{}, applicationMutationError("decision.create", err)
		}
		return applicationJSONToolResult(decision)
	}
	if arguments.ExpectedVersion < 1 {
		return CallToolResult{}, applicationParamsRPCError()
	}
	patch := work.DecisionPatch{ProjectID: arguments.ProjectID, Title: arguments.Title, Context: arguments.Context, Outcome: arguments.Outcome, Rationale: arguments.Rationale, Status: arguments.Status}
	boundContext, scope, _, err := application.productScopes(ctx, invocation.Principal)
	if err != nil {
		return CallToolResult{}, err
	}
	decision, err := application.services.Product.Work.UpdateDecision(boundContext, scope, arguments.ID, patch, arguments.ExpectedVersion, key)
	if err != nil {
		return CallToolResult{}, applicationMutationError("decision.update", err)
	}
	return applicationJSONToolResult(decision)
}

func (application *Application) handleKnowledgeImportManual(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	if application.services.Product == nil {
		return CallToolResult{}, safeApplicationDependency("knowledge.import_manual", ErrInvalidApplicationServices)
	}
	key, err := invocationIdempotencyKey(invocation)
	if err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	var arguments struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Kind     string `json:"kind"`
		URI      string `json:"uri"`
		MIMEType string `json:"mimeType"`
		Content  string `json:"content"`
	}
	if err := decodeApplicationArguments(invocation.Arguments, &arguments); err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	boundContext, _, _, err := application.productScopes(ctx, invocation.Principal)
	if err != nil {
		return CallToolResult{}, err
	}
	result, err := application.services.Product.ImportManualSource(boundContext, productapp.ManualSourceInput{ID: arguments.ID, Name: arguments.Name, Kind: arguments.Kind, URI: arguments.URI, MIMEType: arguments.MIMEType, Content: arguments.Content}, key)
	if err != nil {
		return CallToolResult{}, applicationMutationError("knowledge.import_manual", err)
	}
	return applicationJSONToolResult(result)
}

func (application *Application) handleEntityTrash(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	if application.services.Product == nil {
		return CallToolResult{}, safeApplicationDependency("entity.trash", ErrInvalidApplicationServices)
	}
	key, err := invocationIdempotencyKey(invocation)
	if err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	var arguments entityTrashArguments
	if err := decodeApplicationArguments(invocation.Arguments, &arguments); err != nil || arguments.ExpectedVersion < 1 || !validMCPIdentifier(arguments.ID, applicationMaxIdentifierBytes, true) {
		return CallToolResult{}, applicationParamsRPCError()
	}
	boundContext, scope, _, err := application.productScopes(ctx, invocation.Principal)
	if err != nil {
		return CallToolResult{}, err
	}
	if application.services.Actions != nil {
		if application.services.Actions.Work == nil || !validMCPIdentifier(arguments.ChallengeID, applicationMaxIdentifierBytes, true) {
			return CallToolResult{}, applicationParamsRPCError()
		}
		target := connectors.SafeWriteTarget{
			Operation:  connectors.SafeWriteOperationEntityTrash,
			EntityType: string(arguments.EntityType), EntityID: arguments.ID,
			ExpectedVersion: arguments.ExpectedVersion,
		}
		confirmation, err := application.services.Actions.Confirm(boundContext, actions.Scope(scope), arguments.ChallengeID, target, key)
		if err != nil {
			return CallToolResult{}, applicationActionError("entity.trash.confirm", err)
		}
		return applicationJSONToolResult(struct {
			Action   assistant.ActionBinding `json:"action"`
			Receipt  assistant.ActionReceipt `json:"receipt"`
			Replayed bool                    `json:"replayed"`
		}{Action: confirmation.Binding, Receipt: confirmation.Receipt, Replayed: confirmation.Replayed})
	}
	var result any
	switch arguments.EntityType {
	case work.EntityProject:
		result, err = application.services.Product.Work.TrashProject(boundContext, scope, arguments.ID, arguments.ExpectedVersion, key)
	case work.EntityTask:
		result, err = application.services.Product.Work.TrashTask(boundContext, scope, arguments.ID, arguments.ExpectedVersion, key)
	case work.EntityDecision:
		result, err = application.services.Product.Work.TrashDecision(boundContext, scope, arguments.ID, arguments.ExpectedVersion, key)
	default:
		return CallToolResult{}, applicationParamsRPCError()
	}
	if err != nil {
		return CallToolResult{}, applicationMutationError("entity.trash", err)
	}
	return applicationJSONToolResult(result)
}

func (application *Application) handleGitHubIssueCreate(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	var arguments githubIssueCreateArguments
	if err := decodeApplicationArguments(invocation.Arguments, &arguments); err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	return application.confirmGitHub(ctx, invocation, arguments.ChallengeID, connectors.SafeWriteTarget{Operation: connectors.SafeWriteOperationCreateIssue, Repository: arguments.Repository, Title: arguments.Title, Body: arguments.Body, ExpectedRevision: arguments.ExpectedRevision})
}

func (application *Application) handleGitHubIssueComment(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	var arguments githubIssueCommentArguments
	if err := decodeApplicationArguments(invocation.Arguments, &arguments); err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	return application.confirmGitHub(ctx, invocation, arguments.ChallengeID, connectors.SafeWriteTarget{Operation: connectors.SafeWriteOperationAddComment, Repository: arguments.Repository, Issue: arguments.Issue, Body: arguments.Body, ExpectedRevision: arguments.ExpectedRevision})
}

func (application *Application) handleGitHubIssueLabel(ctx context.Context, invocation Invocation) (CallToolResult, error) {
	var arguments githubIssueLabelArguments
	if err := decodeApplicationArguments(invocation.Arguments, &arguments); err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	return application.confirmGitHub(ctx, invocation, arguments.ChallengeID, connectors.SafeWriteTarget{Operation: connectors.SafeWriteOperationSetLabels, Repository: arguments.Repository, Issue: arguments.Issue, Labels: append([]string(nil), arguments.Labels...), ExpectedRevision: arguments.ExpectedRevision})
}

func (application *Application) confirmGitHub(ctx context.Context, invocation Invocation, challengeID string, target connectors.SafeWriteTarget) (CallToolResult, error) {
	if application.services.Actions == nil {
		return CallToolResult{}, safeApplicationDependency("github.confirm", ErrInvalidApplicationServices)
	}
	key, err := invocationIdempotencyKey(invocation)
	if err != nil {
		return CallToolResult{}, applicationParamsRPCError()
	}
	principalScope, _, _, err := scopesForPrincipal(invocation.Principal)
	if err != nil {
		return CallToolResult{}, applicationIdentityRPCError()
	}
	confirmation, err := application.services.Actions.Confirm(ctx, actions.Scope(principalScope), challengeID, target, key)
	if err != nil {
		return CallToolResult{}, applicationActionError("github.confirm", err)
	}
	return applicationJSONToolResult(struct {
		Action   assistant.ActionBinding `json:"action"`
		Receipt  assistant.ActionReceipt `json:"receipt"`
		Replayed bool                    `json:"replayed"`
	}{Action: confirmation.Binding, Receipt: confirmation.Receipt, Replayed: confirmation.Replayed})
}

func (application *Application) readWorkProjects(ctx context.Context, request ResourceRequest) ([]ResourceContent, error) {
	_, _, scope, err := scopesForPrincipal(request.Principal)
	if err != nil {
		return nil, err
	}
	if application.services.Product != nil {
		boundContext, productScope, _, err := application.productScopes(ctx, request.Principal)
		if err != nil {
			return nil, err
		}
		ctx, scope = boundContext, productScope
	}
	items, err := application.services.Work.ListProjects(ctx, scope, work.ListOptions{Limit: applicationMaxListItems})
	if err != nil {
		return nil, safeApplicationDependency("work.projects", err)
	}
	return applicationJSONResource(ResourceWorkProjectsURI, filterProjects(scope, items))
}

func (application *Application) readWorkTasks(ctx context.Context, request ResourceRequest) ([]ResourceContent, error) {
	_, _, scope, err := scopesForPrincipal(request.Principal)
	if err != nil {
		return nil, err
	}
	if application.services.Product != nil {
		boundContext, productScope, _, err := application.productScopes(ctx, request.Principal)
		if err != nil {
			return nil, err
		}
		ctx, scope = boundContext, productScope
	}
	items, err := application.services.Work.ListTasks(ctx, scope, "", work.ListOptions{Limit: applicationMaxListItems})
	if err != nil {
		return nil, safeApplicationDependency("work.tasks", err)
	}
	return applicationJSONResource(ResourceWorkTasksURI, filterTasks(scope, items))
}

func (application *Application) readWorkDecisions(ctx context.Context, request ResourceRequest) ([]ResourceContent, error) {
	_, _, scope, err := scopesForPrincipal(request.Principal)
	if err != nil {
		return nil, err
	}
	if application.services.Product != nil {
		boundContext, productScope, _, err := application.productScopes(ctx, request.Principal)
		if err != nil {
			return nil, err
		}
		ctx, scope = boundContext, productScope
	}
	items, err := application.services.Work.ListDecisions(ctx, scope, "", work.ListOptions{Limit: applicationMaxListItems})
	if err != nil {
		return nil, safeApplicationDependency("work.decisions", err)
	}
	return applicationJSONResource(ResourceWorkDecisionsURI, filterDecisions(scope, items))
}

// productScopes binds the MCP principal to the canonical application scope.
// The workspace is derived by App.Scope from the user identity; a token with
// a forged workspace claim therefore fails before any repository call.
func (application *Application) productScopes(ctx context.Context, principal Principal) (context.Context, work.Scope, knowledge.WorkspaceScope, error) {
	if application == nil || application.services.Product == nil {
		return nil, work.Scope{}, knowledge.WorkspaceScope{}, safeApplicationDependency("application.scope", ErrInvalidApplicationServices)
	}
	if err := validateApplicationIdentity(principal); err != nil {
		return nil, work.Scope{}, knowledge.WorkspaceScope{}, applicationIdentityRPCError()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	boundContext := store.WithUser(ctx, principal.UserID)
	workScope, knowledgeScope, _, err := application.services.Product.Scope(boundContext)
	if err != nil {
		return nil, work.Scope{}, knowledge.WorkspaceScope{}, safeApplicationDependency("application.scope", err)
	}
	if workScope.WorkspaceID != principal.WorkspaceID || workScope.UserID != principal.UserID || knowledgeScope.ID != principal.WorkspaceID {
		return nil, work.Scope{}, knowledge.WorkspaceScope{}, safeApplicationDependency("application.scope", ErrApplicationScopeMismatch)
	}
	return boundContext, workScope, knowledgeScope, nil
}

func decodeListArguments(raw json.RawMessage) (listArguments, error) {
	var arguments listArguments
	if err := decodeApplicationArguments(raw, &arguments); err != nil || arguments.Limit < 0 || arguments.Limit > applicationMaxListItems {
		return listArguments{}, ErrInvalidParams
	}
	return arguments, nil
}

func decodeTaskListArguments(raw json.RawMessage) (taskListArguments, error) {
	var arguments taskListArguments
	if err := decodeApplicationArguments(raw, &arguments); err != nil || arguments.Limit < 0 || arguments.Limit > applicationMaxListItems || !validMCPIdentifier(arguments.ProjectID, applicationMaxIdentifierBytes, false) {
		return taskListArguments{}, ErrInvalidParams
	}
	return arguments, nil
}

func normalizeApplicationLimit(limit int) int {
	if limit <= 0 {
		return applicationMaxListItems
	}
	if limit > applicationMaxListItems {
		return applicationMaxListItems
	}
	return limit
}

func invocationIdempotencyKey(invocation Invocation) (string, error) {
	key := strings.TrimSpace(invocation.IdempotencyKey)
	if key == "" || key != invocation.IdempotencyKey || len(key) > 128 || hasMCPDisallowedControl(key, false) || strings.IndexFunc(key, func(r rune) bool { return r <= 0x20 || r > 0x7e }) >= 0 {
		return "", ErrMissingIdempotencyKey
	}
	return key, nil
}

func applicationIDArgumentsJSON(id string) json.RawMessage {
	encoded, _ := json.Marshal(applicationIDArguments{ID: id})
	return json.RawMessage(encoded)
}

func applicationMutationError(operation string, cause error) error {
	if cause == nil {
		return safeApplicationDependency(operation, ErrApplicationDependency)
	}
	var workValidation *work.ValidationError
	var knowledgeValidation *knowledge.ValidationError
	switch {
	case errors.As(cause, &workValidation), errors.As(cause, &knowledgeValidation), errors.Is(cause, knowledge.ErrInvalidInput):
		return newRPCError(InvalidParams, "invalid application arguments", cause, nil)
	case errors.Is(cause, work.ErrIdempotencyConflict):
		return newRPCError(IdempotencyError, "idempotency key cannot be reused for this request", cause, nil)
	case errors.Is(cause, work.ErrVersionConflict), errors.Is(cause, work.ErrDependenciesExist), errors.Is(cause, knowledge.ErrConflict):
		return newRPCError(ConflictError, "application mutation conflicts with current state", cause, nil)
	case errors.Is(cause, work.ErrNotFound), errors.Is(cause, knowledge.ErrNotFound):
		return newRPCError(NotFoundError, "application target was not found", cause, nil)
	default:
		return safeApplicationDependency(operation, cause)
	}
}

func applicationActionError(operation string, cause error) error {
	if cause == nil {
		return safeApplicationDependency(operation, ErrApplicationDependency)
	}
	var workValidation *work.ValidationError
	switch {
	case errors.Is(cause, actions.ErrInvalidScope), errors.Is(cause, actions.ErrInvalidRequest), errors.Is(cause, actions.ErrUnsupportedAction), errors.Is(cause, connectors.ErrInvalidWriteTarget), errors.Is(cause, connectors.ErrInvalidRepository):
		return newRPCError(InvalidParams, "invalid action arguments", cause, nil)
	case errors.As(cause, &workValidation):
		return newRPCError(InvalidParams, "invalid action arguments", cause, nil)
	case errors.Is(cause, actions.ErrChallengeNotFound), errors.Is(cause, connectors.ErrInvalidChallenge):
		return newRPCError(NotFoundError, "action challenge was not found", cause, nil)
	case errors.Is(cause, actions.ErrChallengeMismatch), errors.Is(cause, actions.ErrInvalidActionState), errors.Is(cause, connectors.ErrChallengeExpired), errors.Is(cause, connectors.ErrChallengeUsed), errors.Is(cause, connectors.ErrIdempotencyConflict):
		return newRPCError(ForbiddenError, "action challenge is not valid for this request", cause, nil)
	case errors.Is(cause, connectors.ErrReceiptUncertain):
		return newRPCError(ConflictError, "action outcome is uncertain; retry with the same idempotency key", cause, nil)
	case errors.Is(cause, work.ErrIdempotencyConflict):
		return newRPCError(IdempotencyError, "idempotency key cannot be reused for this action", cause, nil)
	case errors.Is(cause, work.ErrVersionConflict), errors.Is(cause, work.ErrDependenciesExist):
		return newRPCError(ConflictError, "action target changed or has dependent records", cause, nil)
	case errors.Is(cause, work.ErrNotFound):
		return newRPCError(NotFoundError, "action target was not found", cause, nil)
	default:
		return safeApplicationDependency(operation, cause)
	}
}

func decodeIDArguments(raw json.RawMessage) (applicationIDArguments, error) {
	var arguments applicationIDArguments
	if err := decodeApplicationArguments(raw, &arguments); err != nil {
		return applicationIDArguments{}, err
	}
	if !validMCPIdentifier(arguments.ID, applicationMaxIdentifierBytes, true) {
		return applicationIDArguments{}, ErrInvalidParams
	}
	return arguments, nil
}

func decodeApplicationArguments(raw json.RawMessage, target any) error {
	if len(raw) == 0 || len(raw) > applicationMaxArgumentsBytes || !utf8.Valid(raw) || requireJSONObject(raw) != nil {
		return ErrInvalidParams
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrInvalidParams
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return ErrInvalidParams
	}
	return nil
}

func validateApplicationText(value string, maxBytes int, required bool) error {
	if required && strings.TrimSpace(value) == "" {
		return ErrInvalidParams
	}
	if !required && value == "" {
		return nil
	}
	if strings.TrimSpace(value) != value || len(value) > maxBytes || !utf8.ValidString(value) || hasMCPDisallowedControl(value, false) {
		return ErrInvalidParams
	}
	return nil
}

func scopesForPrincipal(principal Principal) (assistant.Scope, knowledge.WorkspaceScope, work.Scope, error) {
	if validateApplicationIdentity(principal) != nil {
		return assistant.Scope{}, knowledge.WorkspaceScope{}, work.Scope{}, ErrInvalidApplicationIdentity
	}
	return assistant.Scope{WorkspaceID: principal.WorkspaceID, UserID: principal.UserID},
		knowledge.WorkspaceScope{ID: principal.WorkspaceID},
		work.Scope{WorkspaceID: principal.WorkspaceID, UserID: principal.UserID}, nil
}

func validateApplicationIdentity(principal Principal) error {
	if !validMCPIdentifier(principal.WorkspaceID, applicationMaxIdentifierBytes, true) {
		return ErrInvalidApplicationIdentity
	}
	if !validMCPIdentifier(principal.UserID, applicationMaxIdentifierBytes, true) {
		return ErrInvalidApplicationIdentity
	}
	return nil
}

func applicationParamsRPCError() error {
	return newRPCError(InvalidParams, "invalid application arguments", ErrInvalidParams, nil)
}

func applicationIdentityRPCError() error {
	return newRPCError(ForbiddenError, "authenticated MCP identity is incomplete", ErrInvalidApplicationIdentity, nil)
}

type applicationDependencyError struct {
	operation string
	cause     error
}

func (err applicationDependencyError) Error() string {
	return "MCP application dependency failed: " + err.operation
}

func (err applicationDependencyError) Unwrap() []error {
	if err.cause == nil {
		return []error{ErrApplicationDependency}
	}
	return []error{ErrApplicationDependency, err.cause}
}

func safeApplicationDependency(operation string, cause error) error {
	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		return cause
	}
	return applicationDependencyError{operation: operation, cause: cause}
}

func applicationJSONToolResult(value any) (CallToolResult, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return CallToolResult{}, ErrApplicationEncoding
	}
	return CallToolResult{
		Content:           []Content{{Type: "text", MIMEType: "application/json", Text: string(encoded)}},
		StructuredContent: json.RawMessage(encoded),
	}, nil
}

func applicationJSONResource(uri string, value any) ([]ResourceContent, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, ErrApplicationEncoding
	}
	text := string(encoded)
	return []ResourceContent{{URI: uri, MIMEType: "application/json", Text: &text}}, nil
}

type knowledgeClaimPayload struct {
	ID          string                   `json:"id"`
	WorkspaceID string                   `json:"workspaceId"`
	TopicID     string                   `json:"topicId,omitempty"`
	Statement   string                   `json:"statement"`
	Certainty   knowledge.ClaimCertainty `json:"certainty"`
	Freshness   knowledge.ClaimFreshness `json:"freshness"`
	Version     int64                    `json:"version"`
	CreatedAt   time.Time                `json:"createdAt"`
	UpdatedAt   time.Time                `json:"updatedAt"`
	DeletedAt   *time.Time               `json:"deletedAt,omitempty"`
}

type knowledgeChunkPayload struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	RevisionID  string    `json:"revisionId"`
	Ordinal     int       `json:"ordinal"`
	Text        string    `json:"text"`
	TokenCount  int       `json:"tokenCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

type knowledgeSourcePayload struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspaceId"`
	Kind        string         `json:"kind"`
	Name        string         `json:"name"`
	URI         string         `json:"uri"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Version     int64          `json:"version"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   *time.Time     `json:"deletedAt,omitempty"`
}

type knowledgeItemPayload struct {
	ID                string     `json:"id"`
	WorkspaceID       string     `json:"workspaceId"`
	SourceID          string     `json:"sourceId"`
	ExternalID        string     `json:"externalId"`
	Title             string     `json:"title"`
	URI               string     `json:"uri"`
	MIMEType          string     `json:"mimeType"`
	CurrentRevisionID string     `json:"currentRevisionId,omitempty"`
	Version           int64      `json:"version"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	DeletedAt         *time.Time `json:"deletedAt,omitempty"`
}

type knowledgeRevisionPayload struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspaceId"`
	SourceItemID string    `json:"sourceItemId"`
	RevisionKey  string    `json:"revisionKey"`
	ContentHash  string    `json:"contentHash"`
	ContentType  string    `json:"contentType,omitempty"`
	SourceURI    string    `json:"sourceUri,omitempty"`
	Content      string    `json:"content"`
	ModifiedAt   time.Time `json:"modifiedAt,omitempty"`
	IngestedAt   time.Time `json:"ingestedAt"`
}

type knowledgeEvidencePayload struct {
	ID               string                      `json:"id"`
	WorkspaceID      string                      `json:"workspaceId"`
	ClaimID          string                      `json:"claimId"`
	SourceRevisionID string                      `json:"sourceRevisionId"`
	ChunkID          string                      `json:"chunkId,omitempty"`
	Locator          string                      `json:"locator,omitempty"`
	Quote            string                      `json:"quote"`
	Freshness        knowledge.EvidenceFreshness `json:"freshness"`
	CreatedAt        time.Time                   `json:"createdAt"`
}

type knowledgeClaimDetailPayload struct {
	ID          string                     `json:"id"`
	WorkspaceID string                     `json:"workspaceId"`
	TopicID     string                     `json:"topicId,omitempty"`
	Statement   string                     `json:"statement"`
	Certainty   knowledge.ClaimCertainty   `json:"certainty"`
	Freshness   knowledge.ClaimFreshness   `json:"freshness"`
	Version     int64                      `json:"version"`
	CreatedAt   time.Time                  `json:"createdAt"`
	UpdatedAt   time.Time                  `json:"updatedAt"`
	DeletedAt   *time.Time                 `json:"deletedAt,omitempty"`
	Evidence    []knowledgeEvidencePayload `json:"evidence"`
}

type knowledgeSearchPayload struct {
	ID            string `json:"id"`
	SourceID      string `json:"sourceId"`
	SourceName    string `json:"sourceName"`
	RevisionID    string `json:"revisionId"`
	URI           string `json:"uri"`
	Excerpt       string `json:"excerpt"`
	Freshness     string `json:"freshness"`
	Origin        string `json:"origin"`
	RetrievalMode string `json:"retrievalMode,omitempty"`
}

func toKnowledgeClaimPayload(claim knowledge.KnowledgeClaim) knowledgeClaimPayload {
	return knowledgeClaimPayload{
		ID: claim.ID, WorkspaceID: claim.WorkspaceID, TopicID: claim.TopicID,
		Statement: claim.Statement, Certainty: claim.Certainty, Freshness: claim.Freshness,
		Version: claim.Version, CreatedAt: claim.CreatedAt, UpdatedAt: claim.UpdatedAt, DeletedAt: claim.DeletedAt,
	}
}

func toKnowledgeChunkPayload(chunk knowledge.KnowledgeChunk) knowledgeChunkPayload {
	return knowledgeChunkPayload{
		ID: chunk.ID, WorkspaceID: chunk.WorkspaceID, RevisionID: chunk.RevisionID,
		Ordinal: chunk.Ordinal, Text: chunk.Text, TokenCount: chunk.TokenCount, CreatedAt: chunk.CreatedAt,
	}
}

func toKnowledgeSourcePayload(source knowledge.KnowledgeSource) knowledgeSourcePayload {
	return knowledgeSourcePayload{
		ID: source.ID, WorkspaceID: source.WorkspaceID, Kind: source.Kind, Name: source.Name, URI: source.URI,
		Metadata: source.Metadata, Version: source.Version, CreatedAt: source.CreatedAt, UpdatedAt: source.UpdatedAt, DeletedAt: source.DeletedAt,
	}
}

func toKnowledgeItemPayload(item knowledge.SourceItem) knowledgeItemPayload {
	return knowledgeItemPayload{
		ID: item.ID, WorkspaceID: item.WorkspaceID, SourceID: item.SourceID, ExternalID: item.ExternalID, Title: item.Title,
		URI: item.URI, MIMEType: item.MIMEType, CurrentRevisionID: item.CurrentRevisionID, Version: item.Version,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, DeletedAt: item.DeletedAt,
	}
}

func toKnowledgeRevisionPayload(revision knowledge.SourceRevision) knowledgeRevisionPayload {
	return knowledgeRevisionPayload{
		ID: revision.ID, WorkspaceID: revision.WorkspaceID, SourceItemID: revision.SourceItemID, RevisionKey: revision.RevisionKey,
		ContentHash: revision.ContentHash, ContentType: revision.ContentType, SourceURI: revision.SourceURI, Content: revision.Content,
		ModifiedAt: revision.ModifiedAt, IngestedAt: revision.IngestedAt,
	}
}

func toKnowledgeEvidencePayload(evidence knowledge.ClaimEvidence) knowledgeEvidencePayload {
	return knowledgeEvidencePayload{
		ID: evidence.ID, WorkspaceID: evidence.WorkspaceID, ClaimID: evidence.ClaimID, SourceRevisionID: evidence.SourceRevisionID,
		ChunkID: evidence.ChunkID, Locator: evidence.Locator, Quote: evidence.Quote, Freshness: evidence.Freshness, CreatedAt: evidence.CreatedAt,
	}
}

func toKnowledgeClaimDetailPayload(claim knowledge.KnowledgeClaim, evidence []knowledge.ClaimEvidence) knowledgeClaimDetailPayload {
	payload := knowledgeClaimDetailPayload{
		ID: claim.ID, WorkspaceID: claim.WorkspaceID, TopicID: claim.TopicID, Statement: claim.Statement, Certainty: claim.Certainty,
		Freshness: claim.Freshness, Version: claim.Version, CreatedAt: claim.CreatedAt, UpdatedAt: claim.UpdatedAt, DeletedAt: claim.DeletedAt,
		Evidence: make([]knowledgeEvidencePayload, 0, len(evidence)),
	}
	for _, item := range evidence {
		payload.Evidence = append(payload.Evidence, toKnowledgeEvidencePayload(item))
	}
	return payload
}

func toKnowledgeSearchPayloads(items []knowledge.SearchResult) []knowledgeSearchPayload {
	result := make([]knowledgeSearchPayload, 0, len(items))
	for _, item := range items {
		uri := firstNonEmptyApplication(item.Revision.SourceURI, item.Source.URI, item.Item.URI)
		result = append(result, knowledgeSearchPayload{
			ID: "knowledge-" + item.Chunk.ID, SourceID: item.Source.ID, SourceName: item.Source.Name, RevisionID: item.Revision.ID,
			URI: uri, Excerpt: item.Chunk.Text, Freshness: "current", Origin: "canonical", RetrievalMode: "lexical",
		})
	}
	return result
}

func firstNonEmptyApplication(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func filterProjects(scope work.Scope, items []work.Project) []work.Project {
	result := make([]work.Project, 0, minApplicationListLen(len(items)))
	for _, item := range items {
		if item.WorkspaceID != scope.WorkspaceID || item.OwnerUserID != scope.UserID {
			continue
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return limitApplicationList(result)
}

func projectInApplicationScope(project work.Project, scope work.Scope) bool {
	return project.WorkspaceID == scope.WorkspaceID && project.OwnerUserID == scope.UserID
}

func filterTasks(scope work.Scope, items []work.Task) []work.Task {
	result := make([]work.Task, 0, minApplicationListLen(len(items)))
	for _, item := range items {
		if item.WorkspaceID != scope.WorkspaceID || item.OwnerUserID != scope.UserID {
			continue
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return limitApplicationList(result)
}

func taskInApplicationScope(task work.Task, scope work.Scope) bool {
	return task.WorkspaceID == scope.WorkspaceID && task.OwnerUserID == scope.UserID
}

func filterDecisions(scope work.Scope, items []work.Decision) []work.Decision {
	result := make([]work.Decision, 0, minApplicationListLen(len(items)))
	for _, item := range items {
		if item.WorkspaceID != scope.WorkspaceID || item.OwnerUserID != scope.UserID {
			continue
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return limitApplicationList(result)
}

func decisionInApplicationScope(decision work.Decision, scope work.Scope) bool {
	return decision.WorkspaceID == scope.WorkspaceID && decision.OwnerUserID == scope.UserID
}

func minApplicationListLen(length int) int {
	if length < applicationMaxListItems {
		return length
	}
	return applicationMaxListItems
}

func limitApplicationList[T any](items []T) []T {
	if len(items) > applicationMaxListItems {
		return items[:applicationMaxListItems]
	}
	return items
}

func usableDependency(value any) bool {
	if value == nil {
		return false
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return !reflected.IsNil()
	default:
		return true
	}
}
