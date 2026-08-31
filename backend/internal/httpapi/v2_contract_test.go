package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	productapp "github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func TestV2CanonicalRoutesExposeSourceGraphConversationAndLifecycle(t *testing.T) {
	handler := productTestServer(t)

	projectResponse := v2ContractRequest(t, handler, http.MethodPost, "/api/v2/projects", `{"name":"Release workflow","description":"Track the verified rollback work."}`, "contract-project-create")
	if projectResponse.Code != http.StatusCreated {
		t.Fatalf("create project returned %d: %s", projectResponse.Code, projectResponse.Body.String())
	}
	var project struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	v2DecodeContract(t, projectResponse, &project)
	if project.ID == "" || project.Version != 1 {
		t.Fatalf("created project = %+v", project)
	}
	if response := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/projects/"+project.ID, "", ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Release workflow") {
		t.Fatalf("get project returned %d: %s", response.Code, response.Body.String())
	}
	if response := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/projects?limit=1", "", ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), project.ID) {
		t.Fatalf("list projects returned %d: %s", response.Code, response.Body.String())
	}

	taskResponse := v2ContractRequest(t, handler, http.MethodPost, "/api/v2/projects/"+project.ID+"/tasks", `{"title":"Verify rollback","description":"Check the source-backed rollback path.","priority":"high"}`, "contract-task-create")
	if taskResponse.Code != http.StatusCreated {
		t.Fatalf("create task returned %d: %s", taskResponse.Code, taskResponse.Body.String())
	}
	var task struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	v2DecodeContract(t, taskResponse, &task)
	if task.ID == "" || task.Version != 1 {
		t.Fatalf("created task = %+v", task)
	}
	if response := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/tasks/"+task.ID, "", ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Verify rollback") {
		t.Fatalf("get task returned %d: %s", response.Code, response.Body.String())
	}
	if response := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/tasks?projectId="+project.ID, "", ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), task.ID) {
		t.Fatalf("list tasks returned %d: %s", response.Code, response.Body.String())
	}
	updatedTask := v2ContractRequest(t, handler, http.MethodPatch, "/api/v2/tasks/"+task.ID, `{"title":"Verify rollback evidence","expectedVersion":1}`, "contract-task-update")
	if updatedTask.Code != http.StatusOK || !strings.Contains(updatedTask.Body.String(), "Verify rollback evidence") {
		t.Fatalf("update task returned %d: %s", updatedTask.Code, updatedTask.Body.String())
	}
	trashedTask := v2ContractRequest(t, handler, http.MethodPost, "/api/v2/tasks/"+task.ID+"/trash", `{"expectedVersion":2}`, "contract-task-trash")
	if trashedTask.Code != http.StatusOK || !strings.Contains(trashedTask.Body.String(), "deletedAt") {
		t.Fatalf("trash task returned %d: %s", trashedTask.Code, trashedTask.Body.String())
	}
	restoredTask := v2ContractRequest(t, handler, http.MethodPost, "/api/v2/tasks/"+task.ID+"/restore", `{"expectedVersion":3}`, "contract-task-restore")
	if restoredTask.Code != http.StatusOK || strings.Contains(restoredTask.Body.String(), "deletedAt") {
		t.Fatalf("restore task returned %d: %s", restoredTask.Code, restoredTask.Body.String())
	}
	if history := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/tasks/"+task.ID+"/history", "", ""); history.Code != http.StatusOK || strings.Count(history.Body.String(), `"action"`) != 4 {
		t.Fatalf("task history returned %d: %s", history.Code, history.Body.String())
	}

	decisionResponse := v2ContractRequest(t, handler, http.MethodPost, "/api/v2/decisions", `{"projectId":"`+project.ID+`","title":"Keep rollback reversible","context":"A provider may fail during release.","outcome":"Retain a reversible migration.","rationale":"It reduces recovery risk."}`, "contract-decision-create")
	if decisionResponse.Code != http.StatusCreated {
		t.Fatalf("create decision returned %d: %s", decisionResponse.Code, decisionResponse.Body.String())
	}
	var decision struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	v2DecodeContract(t, decisionResponse, &decision)
	if decision.ID == "" || decision.Version != 1 {
		t.Fatalf("created decision = %+v", decision)
	}
	if response := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/decisions/"+decision.ID, "", ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Keep rollback reversible") {
		t.Fatalf("get decision returned %d: %s", response.Code, response.Body.String())
	}
	if response := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/decisions?projectId="+project.ID, "", ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), decision.ID) {
		t.Fatalf("list decisions returned %d: %s", response.Code, response.Body.String())
	}
	updatedDecision := v2ContractRequest(t, handler, http.MethodPatch, "/api/v2/decisions/"+decision.ID, `{"outcome":"Retain the reversible migration and verify it.","expectedVersion":1}`, "contract-decision-update")
	if updatedDecision.Code != http.StatusOK || !strings.Contains(updatedDecision.Body.String(), "verify it") {
		t.Fatalf("update decision returned %d: %s", updatedDecision.Code, updatedDecision.Body.String())
	}
	if response := v2ContractRequest(t, handler, http.MethodPost, "/api/v2/decisions/"+decision.ID+"/trash", `{"expectedVersion":2}`, "contract-decision-trash"); response.Code != http.StatusOK {
		t.Fatalf("trash decision returned %d: %s", response.Code, response.Body.String())
	}
	if response := v2ContractRequest(t, handler, http.MethodPost, "/api/v2/decisions/"+decision.ID+"/restore", `{"expectedVersion":3}`, "contract-decision-restore"); response.Code != http.StatusOK {
		t.Fatalf("restore decision returned %d: %s", response.Code, response.Body.String())
	}
	if history := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/decisions/"+decision.ID+"/history", "", ""); history.Code != http.StatusOK || strings.Count(history.Body.String(), `"action"`) != 4 {
		t.Fatalf("decision history returned %d: %s", history.Code, history.Body.String())
	}

	sourceResponse := v2ContractRequest(t, handler, http.MethodPost, "/api/v2/knowledge/sources", `{"name":"Rollback runbook","kind":"runbook","uri":"memory://rollback","mimeType":"text/plain","content":"The rollback procedure is documented and reversible."}`, "contract-source-import")
	if sourceResponse.Code != http.StatusCreated {
		t.Fatalf("import source returned %d: %s", sourceResponse.Code, sourceResponse.Body.String())
	}
	var imported struct {
		Source struct {
			ID string `json:"id"`
		} `json:"source"`
		RevisionID string `json:"revisionId"`
		ChunkID    string `json:"chunkId"`
	}
	v2DecodeContract(t, sourceResponse, &imported)
	if imported.Source.ID == "" || imported.RevisionID == "" || imported.ChunkID == "" {
		t.Fatalf("imported source identity = %+v", imported)
	}
	if response := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/knowledge/sources", "", ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), imported.Source.ID) {
		t.Fatalf("list sources returned %d: %s", response.Code, response.Body.String())
	}
	if response := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/knowledge/sources/"+imported.Source.ID, "", ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Rollback runbook") {
		t.Fatalf("get source returned %d: %s", response.Code, response.Body.String())
	}
	var detail struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		Revisions []struct {
			ID string `json:"id"`
		} `json:"revisions"`
		Chunks []struct {
			ID string `json:"id"`
		} `json:"chunks"`
	}
	detailResponse := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/knowledge/sources/"+imported.Source.ID+"/detail", "", "")
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("source detail returned %d: %s", detailResponse.Code, detailResponse.Body.String())
	}
	v2DecodeContract(t, detailResponse, &detail)
	if len(detail.Items) != 1 || len(detail.Revisions) != 1 || len(detail.Chunks) != 1 {
		t.Fatalf("source detail graph = %+v", detail)
	}
	for _, route := range []string{
		"/api/v2/knowledge/items/" + detail.Items[0].ID,
		"/api/v2/knowledge/revisions/" + detail.Revisions[0].ID,
		"/api/v2/knowledge/chunks/" + detail.Chunks[0].ID,
	} {
		if response := v2ContractRequest(t, handler, http.MethodGet, route, "", ""); response.Code != http.StatusOK {
			t.Fatalf("GET %s returned %d: %s", route, response.Code, response.Body.String())
		}
	}
	search := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/knowledge/search?q=reversible", "", "")
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), imported.ChunkID) {
		t.Fatalf("knowledge search returned %d: %s", search.Code, search.Body.String())
	}

	conversationResponse := v2ContractRequest(t, handler, http.MethodPost, "/api/v2/assistant/conversations", `{"message":"What does the rollback runbook say?"}`, "")
	if conversationResponse.Code != http.StatusCreated {
		t.Fatalf("start conversation returned %d: %s", conversationResponse.Code, conversationResponse.Body.String())
	}
	var conversation struct {
		ConversationID string `json:"conversationId"`
		Response       struct {
			Grounding string `json:"grounding"`
			Evidence  []any  `json:"evidence"`
		} `json:"response"`
	}
	v2DecodeContract(t, conversationResponse, &conversation)
	if conversation.ConversationID == "" || conversation.Response.Grounding != "grounded" || len(conversation.Response.Evidence) != 1 {
		t.Fatalf("assistant response = %+v", conversation)
	}
	continued := v2ContractRequest(t, handler, http.MethodPost, "/api/v2/assistant/conversations/"+conversation.ConversationID+"/messages", `{"message":"Summarize the rollback procedure."}`, "")
	if continued.Code != http.StatusOK || !strings.Contains(continued.Body.String(), conversation.ConversationID) || !strings.Contains(continued.Body.String(), `"grounding":"grounded"`) {
		t.Fatalf("continue conversation returned %d: %s", continued.Code, continued.Body.String())
	}
	conversationList := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/assistant/conversations?limit=20", "", "")
	if conversationList.Code != http.StatusOK || !strings.Contains(conversationList.Body.String(), conversation.ConversationID) || !strings.Contains(conversationList.Body.String(), `"messageCount":4`) {
		t.Fatalf("list conversations returned %d: %s", conversationList.Code, conversationList.Body.String())
	}
	conversationDetail := v2ContractRequest(t, handler, http.MethodGet, "/api/v2/assistant/conversations/"+conversation.ConversationID, "", "")
	if conversationDetail.Code != http.StatusOK || strings.Count(conversationDetail.Body.String(), `"role"`) != 4 || !strings.Contains(conversationDetail.Body.String(), "rollback") {
		t.Fatalf("get conversation returned %d: %s", conversationDetail.Code, conversationDetail.Body.String())
	}

	contextResponse := v2ContractRequest(t, handler, http.MethodPost, "/api/v2/assistant/conversations", `{"message":"What should I verify on this task?","contextType":"task","contextId":"`+task.ID+`"}`, "")
	if contextResponse.Code != http.StatusCreated || !strings.Contains(contextResponse.Body.String(), `"grounding":"grounded"`) || !strings.Contains(contextResponse.Body.String(), `work-task-`+task.ID) {
		t.Fatalf("context conversation returned %d: %s", contextResponse.Code, contextResponse.Body.String())
	}
	var contextualConversation struct {
		ConversationID string `json:"conversationId"`
	}
	v2DecodeContract(t, contextResponse, &contextualConversation)
	if contextualConversation.ConversationID == "" {
		t.Fatalf("context conversation did not return an id: %s", contextResponse.Body.String())
	}
	contextConflict := v2ContractRequest(t, handler, http.MethodPost, "/api/v2/assistant/conversations/"+contextualConversation.ConversationID+"/messages", `{"message":"Switch to the project","contextType":"project","contextId":"`+project.ID+`"}`, "")
	if contextConflict.Code != http.StatusConflict {
		t.Fatalf("context switch returned %d: %s", contextConflict.Code, contextConflict.Body.String())
	}
}

func TestV2KnowledgeClaimRouteReturnsEvidenceAndEnforcesWorkspaceScope(t *testing.T) {
	server, productApp := productTestServerWithApp(t)
	userID := "claim-route-user"
	ctx := store.WithUser(context.Background(), userID)
	imported, err := productApp.ImportManualSource(ctx, productapp.ManualSourceInput{
		ID:       "claim-route-source",
		Name:     "Claim route notes",
		Kind:     "manual",
		URI:      "memory://claim-route",
		MIMEType: "text/plain",
		Content:  "A canonical claim must remain tied to current evidence.",
	})
	if err != nil {
		t.Fatalf("ImportManualSource() error = %v", err)
	}
	_, knowledgeScope, _, err := productApp.Scope(ctx)
	if err != nil {
		t.Fatalf("Scope() error = %v", err)
	}
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	claim := knowledge.KnowledgeClaim{
		ID: "claim-route-1", WorkspaceID: knowledgeScope.ID, Statement: "A canonical claim has current evidence.",
		Certainty: knowledge.ClaimCanonical, Freshness: knowledge.ClaimCurrent, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	evidence := knowledge.ClaimEvidence{
		ID: "claim-route-evidence-1", WorkspaceID: knowledgeScope.ID, ClaimID: claim.ID,
		SourceRevisionID: imported.RevisionID, ChunkID: imported.ChunkID, Locator: "memory://claim-route#1",
		Quote: "A canonical claim must remain tied to current evidence.", Freshness: knowledge.EvidenceCurrent, CreatedAt: now,
	}
	if err := productApp.Knowledge.CreateClaimBundle(ctx, knowledgeScope, claim, []knowledge.ClaimEvidence{evidence}); err != nil {
		t.Fatalf("CreateClaimBundle() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v2/knowledge/claims/"+claim.ID, nil).WithContext(ctx)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("claim route returned %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		ID        string `json:"id"`
		Statement string `json:"statement"`
		Evidence  []struct {
			ID               string `json:"id"`
			SourceRevisionID string `json:"sourceRevisionId"`
			ChunkID          string `json:"chunkId"`
			Freshness        string `json:"freshness"`
			Quote            string `json:"quote"`
		} `json:"evidence"`
	}
	v2DecodeContract(t, response, &payload)
	if payload.ID != claim.ID || payload.Statement != claim.Statement || len(payload.Evidence) != 1 {
		t.Fatalf("claim response = %+v", payload)
	}
	gotEvidence := payload.Evidence[0]
	if gotEvidence.ID != evidence.ID || gotEvidence.SourceRevisionID != evidence.SourceRevisionID || gotEvidence.ChunkID != evidence.ChunkID || gotEvidence.Freshness != string(knowledge.EvidenceCurrent) || gotEvidence.Quote != evidence.Quote {
		t.Fatalf("claim evidence response = %+v", gotEvidence)
	}

	otherContext := store.WithUser(context.Background(), "different-user")
	otherRequest := httptest.NewRequest(http.MethodGet, "/api/v2/knowledge/claims/"+claim.ID, nil).WithContext(otherContext)
	otherResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(otherResponse, otherRequest)
	if otherResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-workspace claim route returned %d: %s", otherResponse.Code, otherResponse.Body.String())
	}
}

func v2ContractRequest(t *testing.T, handler http.Handler, method, path, body, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func v2DecodeContract(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %d: %v: %s", response.Code, err, response.Body.String())
	}
}
