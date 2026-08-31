package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func TestV2WorkspaceIsolationAndQueryFlags(t *testing.T) {
	server, _ := productTestServerWithApp(t)
	handler := server.Handler()
	userA := store.WithUser(context.Background(), "http-isolation-a")
	userB := store.WithUser(context.Background(), "http-isolation-b")

	created := v2ContextRequest(t, handler, userA, http.MethodPost, "/api/v2/projects", `{"name":"Private project"}`, "http-isolation-create")
	if created.Code != http.StatusCreated {
		t.Fatalf("create project returned %d: %s", created.Code, created.Body.String())
	}
	var project struct {
		ID          string `json:"id"`
		WorkspaceID string `json:"workspaceId"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	if project.ID == "" || project.WorkspaceID == "" {
		t.Fatalf("created project identity = %+v", project)
	}

	trashed := v2ContextRequest(t, handler, userA, http.MethodPost, "/api/v2/projects/"+project.ID+"/trash", `{"expectedVersion":1}`, "http-isolation-trash")
	if trashed.Code != http.StatusOK {
		t.Fatalf("trash project returned %d: %s", trashed.Code, trashed.Body.String())
	}
	withoutTrashed := v2ContextRequest(t, handler, userA, http.MethodGet, "/api/v2/projects?limit=0&includeTrashed=false", "", "")
	if withoutTrashed.Code != http.StatusOK || strings.Contains(withoutTrashed.Body.String(), project.ID) {
		t.Fatalf("default project list returned %d and body %s; trashed project leaked", withoutTrashed.Code, withoutTrashed.Body.String())
	}
	withTrashed := v2ContextRequest(t, handler, userA, http.MethodGet, "/api/v2/projects?limit=999&includeTrashed=YES", "", "")
	if withTrashed.Code != http.StatusOK || !strings.Contains(withTrashed.Body.String(), project.ID) {
		t.Fatalf("includeTrashed project list returned %d: %s", withTrashed.Code, withTrashed.Body.String())
	}

	otherList := v2ContextRequest(t, handler, userB, http.MethodGet, "/api/v2/projects?includeTrashed=on", "", "")
	if otherList.Code != http.StatusOK || strings.Contains(otherList.Body.String(), project.ID) {
		t.Fatalf("cross-workspace project list returned %d: %s", otherList.Code, otherList.Body.String())
	}
	otherGet := v2ContextRequest(t, handler, userB, http.MethodGet, "/api/v2/projects/"+project.ID, "", "")
	if otherGet.Code != http.StatusNotFound {
		t.Fatalf("cross-workspace project get returned %d: %s", otherGet.Code, otherGet.Body.String())
	}

	source := v2ContextRequest(t, handler, userA, http.MethodPost, "/api/v2/knowledge/sources", `{"name":"Private notes","kind":"manual","uri":"memory://private","mimeType":"text/plain","content":"Private source evidence"}`, "http-isolation-source")
	if source.Code != http.StatusCreated {
		t.Fatalf("create source returned %d: %s", source.Code, source.Body.String())
	}
	otherSearch := v2ContextRequest(t, handler, userB, http.MethodGet, "/api/v2/knowledge/search?q=Private+source+evidence", "", "")
	if otherSearch.Code != http.StatusOK || strings.Contains(otherSearch.Body.String(), "Private source evidence") {
		t.Fatalf("cross-workspace knowledge search returned %d: %s", otherSearch.Code, otherSearch.Body.String())
	}

	conversation := v2ContextRequest(t, handler, userA, http.MethodPost, "/api/v2/assistant/conversations", `{"message":"Keep this conversation private."}`, "")
	if conversation.Code != http.StatusCreated {
		t.Fatalf("create conversation returned %d: %s", conversation.Code, conversation.Body.String())
	}
	var conversationPayload struct {
		ID string `json:"conversationId"`
	}
	if err := json.Unmarshal(conversation.Body.Bytes(), &conversationPayload); err != nil {
		t.Fatal(err)
	}
	if conversationPayload.ID == "" {
		t.Fatalf("conversation identity missing: %s", conversation.Body.String())
	}
	otherConversation := v2ContextRequest(t, handler, userB, http.MethodGet, "/api/v2/assistant/conversations/"+conversationPayload.ID, "", "")
	if otherConversation.Code != http.StatusNotFound {
		t.Fatalf("cross-workspace conversation get returned %d: %s", otherConversation.Code, otherConversation.Body.String())
	}
}

func TestV2ConnectorRoutesRejectInvalidInputBeforeProviderIO(t *testing.T) {
	server := connectorTestServer(t)
	driveReader := &connectors.FakeDriveReader{}
	driveStore := connectors.NewMemoryDriveRevisionStore()
	server.DriveSync = func(_ string) (*connectors.DriveSyncService, error) {
		return connectors.NewDriveSyncService(driveReader, driveStore)
	}

	invalidDrivePageSize := postConnectorJSON(t, server.Handler(), "/api/v2/connectors/drive/sync", `{"pageSize":1001}`)
	if invalidDrivePageSize.Code != http.StatusBadRequest || !strings.Contains(invalidDrivePageSize.Body.String(), "page size") {
		t.Fatalf("invalid Drive page size returned %d: %s", invalidDrivePageSize.Code, invalidDrivePageSize.Body.String())
	}
	if requests := driveReader.RequestsSnapshot(); len(requests) != 0 {
		t.Fatalf("invalid Drive request reached provider: %+v", requests)
	}

	driveReader.Errors = map[int]error{0: connectors.NewProviderError("drive", "list", http.StatusBadGateway, "authorization: Bearer super-secret")}
	providerFailure := postConnectorJSON(t, server.Handler(), "/api/v2/connectors/drive/sync", `{}`)
	if providerFailure.Code != http.StatusBadGateway || strings.Contains(providerFailure.Body.String(), "super-secret") {
		t.Fatalf("Drive provider failure returned %d and leaked data: %s", providerFailure.Code, providerFailure.Body.String())
	}

	githubReader := &connectors.FakeGitHubReadClient{}
	githubStore := connectors.NewMemoryGitHubRevisionStore()
	server.GitHubSync = func(_ string) (*connectors.GitHubImportService, error) {
		return connectors.NewGitHubImportService(githubReader, githubStore)
	}
	invalidRepository := postConnectorJSON(t, server.Handler(), "/api/v2/connectors/github/sync", `{"repository":"not-a-repository"}`)
	if invalidRepository.Code != http.StatusBadRequest || !strings.Contains(invalidRepository.Body.String(), "repository") {
		t.Fatalf("invalid GitHub repository returned %d: %s", invalidRepository.Code, invalidRepository.Body.String())
	}
	if requests := githubReader.ListRequestsSnapshot(); len(requests) != 0 {
		t.Fatalf("invalid GitHub request reached provider: %+v", requests)
	}
}

func TestV2QueryHelpersNormalizeBounds(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want int
	}{
		{raw: "", want: 50},
		{raw: "  ", want: 50},
		{raw: "not-a-number", want: 50},
		{raw: "0", want: 50},
		{raw: "-1", want: 50},
		{raw: "25", want: 25},
		{raw: "999", want: 200},
	} {
		if got := parseLimit(test.raw); got != test.want {
			t.Fatalf("parseLimit(%q) = %d, want %d", test.raw, got, test.want)
		}
	}
	for _, test := range []struct {
		raw  string
		want bool
	}{
		{raw: "1", want: true},
		{raw: " TRUE ", want: true},
		{raw: "yes", want: true},
		{raw: "on", want: true},
		{raw: "false", want: false},
		{raw: "2", want: false},
		{raw: "", want: false},
	} {
		if got := parseBoolQuery(test.raw); got != test.want {
			t.Fatalf("parseBoolQuery(%q) = %t, want %t", test.raw, got, test.want)
		}
	}
}

func v2ContextRequest(t *testing.T, handler http.Handler, ctx context.Context, method, path, body, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
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
