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

	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
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
}

// applicationDependencies is an unexported seam used by package tests. The
// exported constructor below only accepts canonical service types.
type applicationDependencies struct {
	Assistant assistantAsker
	Knowledge knowledgeReader
	Work      workReader
}

// Application wires read-only MCP tools/resources to application services.
// It deliberately does not mount HTTP routes or expose mutation tools.
type Application struct {
	services applicationDependencies
}

func NewApplication(services ApplicationServices) (*Application, error) {
	return newApplication(applicationDependencies{
		Assistant: services.Assistant,
		Knowledge: services.Knowledge,
		Work:      services.Work,
	})
}

func newApplication(services applicationDependencies) (*Application, error) {
	if !usableDependency(services.Assistant) || !usableDependency(services.Knowledge) || !usableDependency(services.Work) {
		return nil, ErrInvalidApplicationServices
	}
	return &Application{services: services}, nil
}

// Register installs the Wave 3 read-only surface into an existing MCP
// registry. The transport, auth, nonce and replay behavior remain owned by
// Handler and Registry.
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
	return []Tool{
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

type assistantAskArguments struct {
	ConversationID string `json:"conversationId"`
	Message        string `json:"message"`
}

type applicationIDArguments struct {
	ID string `json:"id"`
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

func (application *Application) readWorkProjects(ctx context.Context, request ResourceRequest) ([]ResourceContent, error) {
	_, _, scope, err := scopesForPrincipal(request.Principal)
	if err != nil {
		return nil, err
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
	items, err := application.services.Work.ListDecisions(ctx, scope, "", work.ListOptions{Limit: applicationMaxListItems})
	if err != nil {
		return nil, safeApplicationDependency("work.decisions", err)
	}
	return applicationJSONResource(ResourceWorkDecisionsURI, filterDecisions(scope, items))
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
