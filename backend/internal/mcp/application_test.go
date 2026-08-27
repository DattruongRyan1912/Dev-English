package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

type applicationAssistantFake struct {
	calls      int
	request    assistant.AskRequest
	response   assistant.AssistantResponse
	err        error
	scopeBound bool
}

func (fake *applicationAssistantFake) Ask(_ context.Context, request assistant.AskRequest) (assistant.AssistantResponse, error) {
	fake.calls++
	fake.request = request
	if fake.err != nil {
		return assistant.AssistantResponse{}, fake.err
	}
	response := fake.response
	if fake.scopeBound {
		response.Scope = request.Scope
	}
	return response, nil
}

type applicationKnowledgeFake struct {
	claimCalls int
	chunkCalls int
	claimScope knowledge.WorkspaceScope
	chunkScope knowledge.WorkspaceScope
	claimID    string
	chunkID    string
	claim      knowledge.KnowledgeClaim
	chunk      knowledge.KnowledgeChunk
	err        error
	scopeBound bool
}

func (fake *applicationKnowledgeFake) GetClaim(_ context.Context, scope knowledge.WorkspaceScope, id string) (knowledge.KnowledgeClaim, error) {
	fake.claimCalls++
	fake.claimScope = scope
	fake.claimID = id
	if fake.err != nil {
		return knowledge.KnowledgeClaim{}, fake.err
	}
	claim := fake.claim
	if fake.scopeBound {
		claim.WorkspaceID = scope.ID
	}
	return claim, nil
}

func (fake *applicationKnowledgeFake) GetChunk(_ context.Context, scope knowledge.WorkspaceScope, id string) (knowledge.KnowledgeChunk, error) {
	fake.chunkCalls++
	fake.chunkScope = scope
	fake.chunkID = id
	if fake.err != nil {
		return knowledge.KnowledgeChunk{}, fake.err
	}
	chunk := fake.chunk
	if fake.scopeBound {
		chunk.WorkspaceID = scope.ID
	}
	return chunk, nil
}

type applicationWorkFake struct {
	projectCalls  int
	taskCalls     int
	decisionCalls int
	listProjects  int
	listTasks     int
	listDecisions int
	projectScope  work.Scope
	taskScope     work.Scope
	decisionScope work.Scope
	projectID     string
	taskID        string
	decisionID    string
	project       work.Project
	task          work.Task
	decision      work.Decision
	projects      []work.Project
	tasks         []work.Task
	decisions     []work.Decision
	projectLimit  int
	taskLimit     int
	decisionLimit int
	err           error
	scopeBound    bool
}

func (fake *applicationWorkFake) GetProject(_ context.Context, scope work.Scope, id string) (work.Project, error) {
	fake.projectCalls++
	fake.projectScope = scope
	fake.projectID = id
	if fake.err != nil {
		return work.Project{}, fake.err
	}
	project := fake.project
	if fake.scopeBound {
		project.WorkspaceID = scope.WorkspaceID
		project.OwnerUserID = scope.UserID
	}
	return project, nil
}

func (fake *applicationWorkFake) ListProjects(_ context.Context, scope work.Scope, options work.ListOptions) ([]work.Project, error) {
	fake.listProjects++
	fake.projectScope = scope
	fake.projectLimit = options.Limit
	if fake.err != nil {
		return nil, fake.err
	}
	return bindProjectScope(fake.projects, scope, fake.scopeBound), nil
}

func (fake *applicationWorkFake) GetTask(_ context.Context, scope work.Scope, id string) (work.Task, error) {
	fake.taskCalls++
	fake.taskScope = scope
	fake.taskID = id
	if fake.err != nil {
		return work.Task{}, fake.err
	}
	task := fake.task
	if fake.scopeBound {
		task.WorkspaceID = scope.WorkspaceID
		task.OwnerUserID = scope.UserID
	}
	return task, nil
}

func (fake *applicationWorkFake) ListTasks(_ context.Context, scope work.Scope, _ string, options work.ListOptions) ([]work.Task, error) {
	fake.listTasks++
	fake.taskScope = scope
	fake.taskLimit = options.Limit
	if fake.err != nil {
		return nil, fake.err
	}
	return bindTaskScope(fake.tasks, scope, fake.scopeBound), nil
}

func (fake *applicationWorkFake) GetDecision(_ context.Context, scope work.Scope, id string) (work.Decision, error) {
	fake.decisionCalls++
	fake.decisionScope = scope
	fake.decisionID = id
	if fake.err != nil {
		return work.Decision{}, fake.err
	}
	decision := fake.decision
	if fake.scopeBound {
		decision.WorkspaceID = scope.WorkspaceID
		decision.OwnerUserID = scope.UserID
	}
	return decision, nil
}

func (fake *applicationWorkFake) ListDecisions(_ context.Context, scope work.Scope, _ string, options work.ListOptions) ([]work.Decision, error) {
	fake.listDecisions++
	fake.decisionScope = scope
	fake.decisionLimit = options.Limit
	if fake.err != nil {
		return nil, fake.err
	}
	return bindDecisionScope(fake.decisions, scope, fake.scopeBound), nil
}

func bindProjectScope(items []work.Project, scope work.Scope, enabled bool) []work.Project {
	result := append([]work.Project(nil), items...)
	if !enabled {
		return result
	}
	for index := range result {
		result[index].WorkspaceID = scope.WorkspaceID
		result[index].OwnerUserID = scope.UserID
	}
	return result
}

func bindTaskScope(items []work.Task, scope work.Scope, enabled bool) []work.Task {
	result := append([]work.Task(nil), items...)
	if !enabled {
		return result
	}
	for index := range result {
		result[index].WorkspaceID = scope.WorkspaceID
		result[index].OwnerUserID = scope.UserID
	}
	return result
}

func bindDecisionScope(items []work.Decision, scope work.Scope, enabled bool) []work.Decision {
	result := append([]work.Decision(nil), items...)
	if !enabled {
		return result
	}
	for index := range result {
		result[index].WorkspaceID = scope.WorkspaceID
		result[index].OwnerUserID = scope.UserID
	}
	return result
}

func newApplicationFixture(t *testing.T) (*Application, *applicationAssistantFake, *applicationKnowledgeFake, *applicationWorkFake) {
	t.Helper()
	assistantFake := &applicationAssistantFake{response: assistant.AssistantResponse{Scope: assistant.Scope{WorkspaceID: "workspace-a", UserID: "user-a"}, Answer: "grounded answer"}}
	knowledgeFake := &applicationKnowledgeFake{
		claim: knowledge.KnowledgeClaim{ID: "claim-1", WorkspaceID: "workspace-a", Statement: "canonical statement", Certainty: knowledge.ClaimCanonical, Freshness: knowledge.ClaimCurrent},
		chunk: knowledge.KnowledgeChunk{ID: "chunk-1", WorkspaceID: "workspace-a", RevisionID: "revision-1", Text: "source chunk", TokenCount: 2},
	}
	workFake := &applicationWorkFake{
		project:  work.Project{ID: "project-1", WorkspaceID: "workspace-a", OwnerUserID: "user-a", Name: "Project", Status: work.ProjectActive, Origin: work.OriginCanonical, Version: 1},
		task:     work.Task{ID: "task-1", WorkspaceID: "workspace-a", OwnerUserID: "user-a", Title: "Task", Status: work.TaskTodo, Priority: work.PriorityNormal, Origin: work.OriginCanonical, Version: 1},
		decision: work.Decision{ID: "decision-1", WorkspaceID: "workspace-a", OwnerUserID: "user-a", Title: "Decision", Status: work.DecisionProposed, Origin: work.OriginCanonical, Version: 1},
	}
	application, err := newApplication(applicationDependencies{Assistant: assistantFake, Knowledge: knowledgeFake, Work: workFake})
	if err != nil {
		t.Fatalf("NewApplication() error = %v", err)
	}
	return application, assistantFake, knowledgeFake, workFake
}

func applicationPrincipal() Principal {
	return Principal{
		TokenID:     "token-a",
		Scopes:      []Scope{ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead},
		WorkspaceID: "workspace-a",
		UserID:      "user-a",
	}
}

func TestApplicationRegistersReadOnlySurface(t *testing.T) {
	application, _, _, _ := newApplicationFixture(t)
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	tools := registry.ListTools(applicationPrincipal())
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
		registered, ok := registry.tool(tool.Name)
		if !ok || registered.Mutating {
			t.Fatalf("tool %q is missing or mutating", tool.Name)
		}
	}
	sort.Strings(names)
	wantTools := []string{"assistant.ask", "knowledge.get_chunk", "knowledge.get_claim", "work.get_decision", "work.get_project", "work.get_task"}
	if fmt.Sprint(names) != fmt.Sprint(wantTools) {
		t.Fatalf("registered tools = %v, want %v", names, wantTools)
	}
	resources := registry.ListResources(applicationPrincipal())
	if len(resources) != 3 || resources[0].URI != ResourceWorkDecisionsURI || resources[1].URI != ResourceWorkProjectsURI || resources[2].URI != ResourceWorkTasksURI {
		t.Fatalf("registered resources = %#v", resources)
	}
	if err := application.Register(registry); !errors.Is(err, ErrDuplicateTool) {
		t.Fatalf("duplicate Register() error = %v, want ErrDuplicateTool", err)
	}
}

func TestTokenIdentityIsVerifiedWithoutBearerSecret(t *testing.T) {
	store := NewTokenStore()
	identity := TokenIdentity{WorkspaceID: "workspace-a", UserID: "user-a"}
	issued, err := store.IssueForIdentity([]Scope{ScopeAssistantUse}, identity)
	if err != nil {
		t.Fatalf("IssueForIdentity() error = %v", err)
	}
	token := mustReveal(t, &issued)
	principal, err := store.Verify(token, issued.ExpiresAt.Add(-1))
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if principal.WorkspaceID != identity.WorkspaceID || principal.UserID != identity.UserID {
		t.Fatalf("verified identity = %#v", principal)
	}
	if strings.Contains(fmt.Sprintf("%+v", principal), token) {
		t.Fatal("verified Principal contains the bearer secret")
	}
	invalid := []TokenIdentity{
		{},
		{WorkspaceID: "workspace-a"},
		{UserID: "user-a"},
		{WorkspaceID: " workspace-a", UserID: "user-a"},
		{WorkspaceID: "workspace a", UserID: "user-a"},
		{WorkspaceID: "workspace\u0085a", UserID: "user-a"},
		{WorkspaceID: "workspace-a", UserID: "user\n-a"},
		{WorkspaceID: strings.Repeat("w", maxTokenIdentityLength+1), UserID: "user-a"},
	}
	for _, value := range invalid {
		if _, err := store.IssueForIdentity(nil, value); !errors.Is(err, ErrInvalidTokenIdentity) {
			t.Errorf("IssueForIdentity(%#v) error = %v, want ErrInvalidTokenIdentity", value, err)
		}
	}
}

func TestApplicationHTTPBindsIdentityAndReturnsStructuredAssistantResponse(t *testing.T) {
	application, assistantFake, _, _ := newApplicationFixture(t)
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	store := NewTokenStore()
	issued, err := store.IssueForIdentity([]Scope{ScopeAssistantUse}, TokenIdentity{WorkspaceID: "workspace-a", UserID: "user-a"})
	if err != nil {
		t.Fatal(err)
	}
	token := mustReveal(t, &issued)
	handler := NewHandler(registry, store)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"assistant.ask","arguments":{"conversationId":"conversation-1","message":"help me"}}}`
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	request.Header.Set(AuthorizationHeader, "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error != nil {
		t.Fatalf("unexpected MCP error: %#v", response.Error)
	}
	result := responseResult[CallToolResult](t, &response)
	if !strings.Contains(result.Content[0].Text, "grounded answer") || result.StructuredContent == nil {
		t.Fatalf("assistant result = %#v", result)
	}
	if assistantFake.calls != 1 || assistantFake.request.Scope != (assistant.Scope{WorkspaceID: "workspace-a", UserID: "user-a"}) {
		t.Fatalf("assistant request = %#v, calls = %d", assistantFake.request, assistantFake.calls)
	}
	if strings.Contains(recorder.Body.String(), token) {
		t.Fatal("MCP response leaked bearer token")
	}
}

func TestApplicationRejectsInvalidArgumentsAndMissingIdentityBeforeServices(t *testing.T) {
	application, assistantFake, _, _ := newApplicationFixture(t)
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(registry, nil)
	invalidArguments := []string{
		`{"message":"hello","workspaceId":"other"}`,
		`{"message":"hello","scope":"assistant:use"}`,
		`{"message":"hello\u000aagain"}`,
		`{"conversationId":"conversation id","message":"hello"}`,
		`{"conversationId":"conversation\u0085id","message":"hello"}`,
		`{"message":"hello\u0085again"}`,
	}
	for _, arguments := range invalidArguments {
		response := handler.Dispatch(context.Background(), applicationPrincipal(), testRequest("1", "tools/call", `{"name":"assistant.ask","arguments":`+arguments+"}"), RequestMeta{})
		if response == nil || response.Error == nil {
			t.Fatalf("arguments %s did not return a JSON-RPC error: %#v", arguments, response)
		}
		if response.Error.Code != InvalidParams {
			t.Fatalf("arguments %s did not return InvalidParams", arguments)
		}
	}
	largeMessage, err := json.Marshal(map[string]any{"name": "assistant.ask", "arguments": map[string]string{"message": strings.Repeat("x", applicationMaxMessageBytes+1)}})
	if err != nil {
		t.Fatal(err)
	}
	largeRequest := Request{JSONRPC: "2.0", ID: json.RawMessage("2"), Method: "tools/call", Params: largeMessage}
	if responseError(t, handler.Dispatch(context.Background(), applicationPrincipal(), largeRequest, RequestMeta{})).Code != InvalidParams {
		t.Fatal("oversized application argument did not return InvalidParams")
	}
	trailingStore := NewTokenStore()
	trailingIssued, err := trailingStore.IssueForIdentity([]Scope{ScopeAssistantUse}, TokenIdentity{WorkspaceID: "workspace-a", UserID: "user-a"})
	if err != nil {
		t.Fatal(err)
	}
	trailingToken := mustReveal(t, &trailingIssued)
	trailing := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"} {}`))
	trailing.Header.Set(AuthorizationHeader, "Bearer "+trailingToken)
	trailingRecorder := httptest.NewRecorder()
	NewHandler(registry, trailingStore).ServeHTTP(trailingRecorder, trailing)
	if trailingRecorder.Code != http.StatusBadRequest {
		t.Fatalf("trailing request status = %d", trailingRecorder.Code)
	}
	missingIdentity := handler.Dispatch(context.Background(), Principal{TokenID: "unbound", Scopes: []Scope{ScopeAssistantUse}}, testRequest("3", "tools/call", `{"name":"assistant.ask","arguments":{"message":"hello"}}`), RequestMeta{})
	if responseError(t, missingIdentity).Code != ForbiddenError {
		t.Fatalf("missing identity code = %d", responseError(t, missingIdentity).Code)
	}
	if assistantFake.calls != 0 {
		t.Fatalf("assistant service was called for invalid requests: %d", assistantFake.calls)
	}
}

func TestApplicationRejectsMalformedUTF8BeforeDependencies(t *testing.T) {
	application, assistantFake, knowledgeFake, _ := newApplicationFixture(t)
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	assistantTool, ok := registry.tool("assistant.ask")
	if !ok {
		t.Fatal("assistant.ask was not registered")
	}
	claimTool, ok := registry.tool("knowledge.get_claim")
	if !ok {
		t.Fatal("knowledge.get_claim was not registered")
	}

	malformedMessage := malformedUTF8StringArgument("message")
	if _, err := assistantTool.Handler(context.Background(), Invocation{Principal: applicationPrincipal(), Arguments: malformedMessage}); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("malformed message error = %v, want ErrInvalidParams", err)
	}
	malformedConversation := append([]byte(`{"conversationId":"conversation`), 0xff)
	malformedConversation = append(malformedConversation, []byte(`","message":"hello"}`)...)
	if _, err := assistantTool.Handler(context.Background(), Invocation{Principal: applicationPrincipal(), Arguments: malformedConversation}); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("malformed conversation ID error = %v, want ErrInvalidParams", err)
	}
	malformedID := malformedUTF8StringArgument("id")
	if _, err := claimTool.Handler(context.Background(), Invocation{Principal: applicationPrincipal(), Arguments: malformedID}); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("malformed entity ID error = %v, want ErrInvalidParams", err)
	}
	if assistantFake.calls != 0 || knowledgeFake.claimCalls != 0 {
		t.Fatalf("malformed UTF-8 reached dependencies: assistant=%d claim=%d", assistantFake.calls, knowledgeFake.claimCalls)
	}
}

func malformedUTF8StringArgument(field string) json.RawMessage {
	raw := []byte(`{"` + field + `":"valid`)
	raw = append(raw, 0xff)
	raw = append(raw, []byte(`"}`)...)
	return json.RawMessage(raw)
}

func TestApplicationReadToolsAndResourcesUseCanonicalScopesAndCapLists(t *testing.T) {
	application, _, knowledgeFake, workFake := newApplicationFixture(t)
	for index := 0; index < applicationMaxListItems+5; index++ {
		itemID := applicationMaxListItems + 4 - index
		workFake.projects = append(workFake.projects, work.Project{ID: fmt.Sprintf("project-%03d", itemID), WorkspaceID: "workspace-a", OwnerUserID: "user-a", Name: "Project", Status: work.ProjectActive, Origin: work.OriginCanonical})
		workFake.tasks = append(workFake.tasks, work.Task{ID: fmt.Sprintf("task-%03d", itemID), WorkspaceID: "workspace-a", OwnerUserID: "user-a", Title: "Task", Status: work.TaskTodo, Priority: work.PriorityNormal, Origin: work.OriginCanonical})
		workFake.decisions = append(workFake.decisions, work.Decision{ID: fmt.Sprintf("decision-%03d", itemID), WorkspaceID: "workspace-a", OwnerUserID: "user-a", Title: "Decision", Status: work.DecisionProposed, Origin: work.OriginCanonical})
	}
	workFake.projects = append(workFake.projects, work.Project{ID: "project-cross", WorkspaceID: "workspace-b", OwnerUserID: "user-a"})
	workFake.tasks = append(workFake.tasks, work.Task{ID: "task-cross", WorkspaceID: "workspace-a", OwnerUserID: "user-b"})
	workFake.decisions = append(workFake.decisions, work.Decision{ID: "decision-cross", WorkspaceID: "workspace-b", OwnerUserID: "user-b"})
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	principal := applicationPrincipal()
	claimResponse := handlerForApplication(registry).Dispatch(context.Background(), principal, testRequest("1", "tools/call", `{"name":"knowledge.get_claim","arguments":{"id":"claim-1"}}`), RequestMeta{})
	claimResult := responseResult[CallToolResult](t, claimResponse)
	if !strings.Contains(claimResult.Content[0].Text, `"statement":"canonical statement"`) || knowledgeFake.claimScope.ID != "workspace-a" || knowledgeFake.claimID != "claim-1" {
		t.Fatalf("claim read result/scope = %#v, %#v", claimResult, knowledgeFake)
	}
	chunkResponse := handlerForApplication(registry).Dispatch(context.Background(), principal, testRequest("2", "tools/call", `{"name":"knowledge.get_chunk","arguments":{"id":"chunk-1"}}`), RequestMeta{})
	chunkResult := responseResult[CallToolResult](t, chunkResponse)
	if !strings.Contains(chunkResult.Content[0].Text, `"text":"source chunk"`) || knowledgeFake.chunkScope.ID != "workspace-a" || knowledgeFake.chunkID != "chunk-1" {
		t.Fatalf("chunk read result/scope = %#v, %#v", chunkResult, knowledgeFake)
	}
	workCalls := []struct {
		name string
		id   string
		seen *int
	}{
		{name: "work.get_project", id: "project-1", seen: &workFake.projectCalls},
		{name: "work.get_task", id: "task-1", seen: &workFake.taskCalls},
		{name: "work.get_decision", id: "decision-1", seen: &workFake.decisionCalls},
	}
	for index, call := range workCalls {
		request := testRequest(fmt.Sprint(index+3), "tools/call", `{"name":"`+call.name+`","arguments":{"id":"`+call.id+`"}}`)
		if responseResult[CallToolResult](t, handlerForApplication(registry).Dispatch(context.Background(), principal, request, RequestMeta{})).Content[0].Text == "" {
			t.Fatalf("empty result for %s", call.name)
		}
		if *call.seen != 1 {
			t.Fatalf("%s call count = %d", call.name, *call.seen)
		}
	}
	if workFake.projectScope != (work.Scope{WorkspaceID: "workspace-a", UserID: "user-a"}) || workFake.taskScope != workFake.projectScope || workFake.decisionScope != workFake.projectScope {
		t.Fatalf("work scopes were not identity-bound: project=%#v task=%#v decision=%#v", workFake.projectScope, workFake.taskScope, workFake.decisionScope)
	}
	resourceCases := []struct {
		uri   string
		items func([]byte) int
		ids   func([]byte) []string
		calls *int
		limit *int
	}{
		{uri: ResourceWorkProjectsURI, items: func(raw []byte) int { var value []work.Project; _ = json.Unmarshal(raw, &value); return len(value) }, ids: func(raw []byte) []string {
			var value []work.Project
			_ = json.Unmarshal(raw, &value)
			result := make([]string, 0, len(value))
			for _, item := range value {
				result = append(result, item.ID)
			}
			return result
		}, calls: &workFake.listProjects, limit: &workFake.projectLimit},
		{uri: ResourceWorkTasksURI, items: func(raw []byte) int { var value []work.Task; _ = json.Unmarshal(raw, &value); return len(value) }, ids: func(raw []byte) []string {
			var value []work.Task
			_ = json.Unmarshal(raw, &value)
			result := make([]string, 0, len(value))
			for _, item := range value {
				result = append(result, item.ID)
			}
			return result
		}, calls: &workFake.listTasks, limit: &workFake.taskLimit},
		{uri: ResourceWorkDecisionsURI, items: func(raw []byte) int { var value []work.Decision; _ = json.Unmarshal(raw, &value); return len(value) }, ids: func(raw []byte) []string {
			var value []work.Decision
			_ = json.Unmarshal(raw, &value)
			result := make([]string, 0, len(value))
			for _, item := range value {
				result = append(result, item.ID)
			}
			return result
		}, calls: &workFake.listDecisions, limit: &workFake.decisionLimit},
	}
	for index, resourceCase := range resourceCases {
		response := handlerForApplication(registry).Dispatch(context.Background(), principal, testRequest(fmt.Sprint(index+10), "resources/read", `{"uri":"`+resourceCase.uri+`"}`), RequestMeta{})
		resourceResult := responseResult[ReadResourceResult](t, response)
		if len(resourceResult.Contents) != 1 || resourceResult.Contents[0].URI != resourceCase.uri || resourceResult.Contents[0].Text == nil {
			t.Fatalf("resource %s = %#v", resourceCase.uri, resourceResult)
		}
		raw := []byte(*resourceResult.Contents[0].Text)
		if got := resourceCase.items(raw); got != applicationMaxListItems {
			t.Fatalf("resource %s item count = %d, want %d", resourceCase.uri, got, applicationMaxListItems)
		}
		if ids := resourceCase.ids(raw); !sort.StringsAreSorted(ids) {
			t.Fatalf("resource %s ids are not deterministic and sorted: %v", resourceCase.uri, ids)
		}
		if *resourceCase.calls != 1 || *resourceCase.limit > applicationMaxListItems {
			t.Fatalf("resource %s calls=%d limit=%d", resourceCase.uri, *resourceCase.calls, *resourceCase.limit)
		}
	}
}

func TestApplicationIdentityTokensIsolateSameEntityIDsAcrossToolsAndResources(t *testing.T) {
	application, assistantFake, knowledgeFake, workFake := newApplicationFixture(t)
	assistantFake.scopeBound = true
	knowledgeFake.scopeBound = true
	workFake.scopeBound = true
	workFake.projects = []work.Project{{ID: "same-project", Name: "Project", Status: work.ProjectActive, Origin: work.OriginCanonical}}
	workFake.tasks = []work.Task{{ID: "same-task", Title: "Task", Status: work.TaskTodo, Priority: work.PriorityNormal, Origin: work.OriginCanonical}}
	workFake.decisions = []work.Decision{{ID: "same-decision", Title: "Decision", Status: work.DecisionProposed, Origin: work.OriginCanonical}}
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	store := NewTokenStore()
	type identityToken struct {
		identity TokenIdentity
		token    string
	}
	identities := []identityToken{
		{identity: TokenIdentity{WorkspaceID: "workspace-a", UserID: "user-a"}},
		{identity: TokenIdentity{WorkspaceID: "workspace-b", UserID: "user-b"}},
	}
	for index := range identities {
		issued, err := store.IssueForIdentity([]Scope{ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead}, identities[index].identity)
		if err != nil {
			t.Fatal(err)
		}
		identities[index].token = mustReveal(t, &issued)
	}
	handler := NewHandler(registry, store)
	requestID := 1
	for _, identityToken := range identities {
		identity := identityToken.identity
		toolCases := []struct {
			name string
			id   string
		}{
			{name: "knowledge.get_claim", id: "claim-1"},
			{name: "knowledge.get_chunk", id: "chunk-1"},
			{name: "work.get_project", id: "project-1"},
			{name: "work.get_task", id: "task-1"},
			{name: "work.get_decision", id: "decision-1"},
		}
		assistantResponse := applicationHTTPCall(t, handler, identityToken.token, requestID, "tools/call", `{"name":"assistant.ask","arguments":{"message":"hello"}}`)
		requestID++
		assistantResult := responseResult[CallToolResult](t, assistantResponse)
		if !strings.Contains(assistantResult.Content[0].Text, identity.WorkspaceID) || !strings.Contains(assistantResult.Content[0].Text, identity.UserID) {
			t.Fatalf("assistant response for %#v lost token identity: %s", identity, assistantResult.Content[0].Text)
		}
		for _, toolCase := range toolCases {
			response := applicationHTTPCall(t, handler, identityToken.token, requestID, "tools/call", fmt.Sprintf(`{"name":%q,"arguments":{"id":%q}}`, toolCase.name, toolCase.id))
			requestID++
			result := responseResult[CallToolResult](t, response)
			if !strings.Contains(result.Content[0].Text, identity.WorkspaceID) {
				t.Fatalf("%s response for %#v lost workspace isolation: %s", toolCase.name, identity, result.Content[0].Text)
			}
			if strings.HasPrefix(toolCase.name, "work.") && !strings.Contains(result.Content[0].Text, identity.UserID) {
				t.Fatalf("%s response for %#v lost user isolation: %s", toolCase.name, identity, result.Content[0].Text)
			}
		}
		for _, uri := range []string{ResourceWorkProjectsURI, ResourceWorkTasksURI, ResourceWorkDecisionsURI} {
			response := applicationHTTPCall(t, handler, identityToken.token, requestID, "resources/read", fmt.Sprintf(`{"uri":%q}`, uri))
			requestID++
			result := responseResult[ReadResourceResult](t, response)
			if len(result.Contents) != 1 || result.Contents[0].Text == nil {
				t.Fatalf("resource %s for %#v = %#v", uri, identity, result)
			}
			text := *result.Contents[0].Text
			if !strings.Contains(text, identity.WorkspaceID) || !strings.Contains(text, identity.UserID) {
				t.Fatalf("resource %s crossed identity boundary for %#v: %s", uri, identity, text)
			}
		}
	}
}

func TestApplicationRejectsIndividualReadScopeMismatches(t *testing.T) {
	application, assistantFake, knowledgeFake, workFake := newApplicationFixture(t)
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	principal := Principal{TokenID: "token-b", Scopes: []Scope{ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead}, WorkspaceID: "workspace-b", UserID: "user-b"}
	cases := []struct {
		name string
		args string
	}{
		{name: "assistant.ask", args: `{"message":"hello"}`},
		{name: "knowledge.get_claim", args: `{"id":"claim-1"}`},
		{name: "knowledge.get_chunk", args: `{"id":"chunk-1"}`},
		{name: "work.get_project", args: `{"id":"project-1"}`},
		{name: "work.get_task", args: `{"id":"task-1"}`},
		{name: "work.get_decision", args: `{"id":"decision-1"}`},
	}
	for index, testCase := range cases {
		response := handlerForApplication(registry).Dispatch(context.Background(), principal, testRequest(fmt.Sprint(index+1), "tools/call", fmt.Sprintf(`{"name":%q,"arguments":%s}`, testCase.name, testCase.args)), RequestMeta{})
		if response.Error == nil {
			t.Fatalf("%s accepted an out-of-scope response", testCase.name)
		}
		if strings.Contains(string(response.Result), "canonical statement") || strings.Contains(string(response.Result), "source chunk") || strings.Contains(string(response.Result), "Project") || strings.Contains(string(response.Result), "Task") || strings.Contains(string(response.Result), "Decision") {
			t.Fatalf("%s disclosed an out-of-scope entity: %s", testCase.name, response.Result)
		}
	}
	if assistantFake.calls != 1 || knowledgeFake.claimCalls != 1 || knowledgeFake.chunkCalls != 1 || workFake.projectCalls != 1 || workFake.taskCalls != 1 || workFake.decisionCalls != 1 {
		t.Fatalf("scope mismatch calls = assistant %d claim %d chunk %d project %d task %d decision %d", assistantFake.calls, knowledgeFake.claimCalls, knowledgeFake.chunkCalls, workFake.projectCalls, workFake.taskCalls, workFake.decisionCalls)
	}
}

func TestApplicationForwardsTrustedAssistantActionDataWithoutManufacturing(t *testing.T) {
	application, assistantFake, _, _ := newApplicationFixture(t)
	assistantFake.response.SuggestedActions = []assistant.SuggestedAction{{ID: "action-1", Kind: "github.issue.create", Label: "Create issue", Target: "repo/1", RequiresConfirmation: true}}
	assistantFake.response.ActionReceipts = []assistant.ActionReceipt{{ID: "receipt-1", ActionID: "action-1", Scope: assistant.Scope{WorkspaceID: "workspace-a", UserID: "user-a"}, Provider: "github", Operation: "issue.create", ChallengeID: "challenge-1", IdempotencyKey: "idem-1", ActionHash: "hash-1", TargetType: "issue", TargetID: "repo/1", Status: assistant.ReceiptPending}}
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	response := handlerForApplication(registry).Dispatch(context.Background(), applicationPrincipal(), testRequest("1", "tools/call", `{"name":"assistant.ask","arguments":{"message":"hello"}}`), RequestMeta{})
	result := responseResult[CallToolResult](t, response)
	var got assistant.AssistantResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &got); err != nil {
		t.Fatalf("decode assistant response: %v", err)
	}
	if len(got.SuggestedActions) != 1 || got.SuggestedActions[0].ID != "action-1" || len(got.ActionReceipts) != 1 || got.ActionReceipts[0].ChallengeID != "challenge-1" || got.ActionReceipts[0].IdempotencyKey != "idem-1" {
		t.Fatalf("trusted action data was not forwarded exactly: %#v", got)
	}
}

func TestApplicationRedactsDependencyErrorsAndDoesNotManufactureActions(t *testing.T) {
	application, assistantFake, _, _ := newApplicationFixture(t)
	secret := "provider-key-that-must-not-leak"
	assistantFake.err = errors.New(secret)
	registry := NewRegistry()
	if err := application.Register(registry); err != nil {
		t.Fatal(err)
	}
	tool, ok := registry.tool("assistant.ask")
	if !ok {
		t.Fatal("assistant.ask was not registered")
	}
	_, err := tool.Handler(context.Background(), Invocation{Principal: applicationPrincipal(), Arguments: json.RawMessage(`{"message":"hello"}`)})
	if err == nil || !errors.Is(err, ErrApplicationDependency) || !errors.Is(err, assistantFake.err) || strings.Contains(err.Error(), secret) {
		t.Fatalf("dependency error = %v", err)
	}
	response := handlerForApplication(registry).Dispatch(context.Background(), applicationPrincipal(), testRequest("1", "tools/call", `{"name":"assistant.ask","arguments":{"message":"hello"}}`), RequestMeta{})
	rpcError := responseError(t, response)
	if strings.Contains(rpcError.Message, secret) || strings.Contains(string(response.Result), secret) {
		t.Fatalf("dependency secret leaked in response: %#v", response)
	}
}

func handlerForApplication(registry *Registry) *Handler {
	return NewHandler(registry, nil)
}

func applicationHTTPCall(t *testing.T, handler *Handler, token string, requestID int, method, params string) *Response {
	t.Helper()
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%q,"params":%s}`, requestID, method, params)
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	request.Header.Set(AuthorizationHeader, "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("application HTTP status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode application HTTP response: %v", err)
	}
	return &response
}
