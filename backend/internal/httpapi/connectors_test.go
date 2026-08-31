package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learning"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func TestDriveSyncHTTPResumesFromDurableCheckpoint(t *testing.T) {
	reader := &connectors.FakeDriveReader{Pages: []connectors.DrivePage{
		{Items: []connectors.DriveSourceItem{{FileID: "drive-file-1", RevisionID: "rev-1", Text: "source"}}, NextCursor: connectors.DriveCursor{Token: "drive-page-2"}, HasMore: true},
		{Items: []connectors.DriveSourceItem{{FileID: "drive-file-1", RevisionID: "rev-1", Text: "source"}}},
	}}
	revisionStore := connectors.NewMemoryDriveRevisionStore()
	server := connectorTestServer(t)
	server.DriveSync = func(_ string) (*connectors.DriveSyncService, error) {
		return connectors.NewDriveSyncService(reader, revisionStore)
	}
	handler := server.Handler()

	first := postConnectorJSON(t, handler, "/api/v2/connectors/drive/sync", `{}`)
	if first.Code != http.StatusOK {
		t.Fatalf("first Drive sync returned %d: %s", first.Code, first.Body.String())
	}
	var firstResult connectors.DriveSyncResult
	if err := json.Unmarshal(first.Body.Bytes(), &firstResult); err != nil {
		t.Fatal(err)
	}
	if !firstResult.HasMore || firstResult.NextCursor.Token != "drive-page-2" || firstResult.Upserted != 1 {
		t.Fatalf("first Drive result = %+v", firstResult)
	}

	second := postConnectorJSON(t, handler, "/api/v2/connectors/drive/sync", `{}`)
	if second.Code != http.StatusOK {
		t.Fatalf("resumed Drive sync returned %d: %s", second.Code, second.Body.String())
	}
	var secondResult connectors.DriveSyncResult
	if err := json.Unmarshal(second.Body.Bytes(), &secondResult); err != nil {
		t.Fatal(err)
	}
	if secondResult.HasMore || secondResult.Upserted != 0 || secondResult.Skipped != 1 {
		t.Fatalf("resumed Drive result = %+v", secondResult)
	}
	requests := reader.RequestsSnapshot()
	if len(requests) != 2 || requests[1].Cursor.Token != "drive-page-2" {
		t.Fatalf("Drive requests = %+v", requests)
	}
}

func TestGitHubSyncHTTPResumesFromDurableCheckpoint(t *testing.T) {
	now := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	issue := connectors.GitHubIssue{Repository: "acme/api", Number: 7, Title: "Retry", Body: "details", State: "open", Revision: "etag-7", UpdatedAt: now}
	reader := &connectors.FakeGitHubReadClient{IssuePages: []connectors.GitHubIssuePage{
		{Issues: []connectors.GitHubIssue{issue}, NextCursor: "github-page-2", HasMore: true},
		{Issues: []connectors.GitHubIssue{issue}},
	}}
	revisionStore := connectors.NewMemoryGitHubRevisionStore()
	server := connectorTestServer(t)
	server.GitHubSync = func(_ string) (*connectors.GitHubImportService, error) {
		return connectors.NewGitHubImportService(reader, revisionStore)
	}
	handler := server.Handler()

	first := postConnectorJSON(t, handler, "/api/v2/connectors/github/sync", `{"repository":"acme/api"}`)
	if first.Code != http.StatusOK {
		t.Fatalf("first GitHub sync returned %d: %s", first.Code, first.Body.String())
	}
	var firstResult connectors.GitHubImportResult
	if err := json.Unmarshal(first.Body.Bytes(), &firstResult); err != nil {
		t.Fatal(err)
	}
	if !firstResult.HasMore || firstResult.NextCursor != "github-page-2" || firstResult.Upserted != 1 {
		t.Fatalf("first GitHub result = %+v", firstResult)
	}

	second := postConnectorJSON(t, handler, "/api/v2/connectors/github/sync", `{"repository":"acme/api"}`)
	if second.Code != http.StatusOK {
		t.Fatalf("resumed GitHub sync returned %d: %s", second.Code, second.Body.String())
	}
	var secondResult connectors.GitHubImportResult
	if err := json.Unmarshal(second.Body.Bytes(), &secondResult); err != nil {
		t.Fatal(err)
	}
	if secondResult.HasMore || secondResult.Upserted != 0 || secondResult.Skipped != 1 {
		t.Fatalf("resumed GitHub result = %+v", secondResult)
	}
	requests := reader.ListRequestsSnapshot()
	if len(requests) != 2 || requests[1].Cursor != "github-page-2" {
		t.Fatalf("GitHub requests = %+v", requests)
	}
}

func connectorTestServer(t *testing.T) *Server {
	t.Helper()
	memory := store.NewSeeded(time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC))
	learningService := learning.NewService(memory, ai.DeterministicProvider{})
	productApp, err := application.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(learningService, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.Application = productApp
	return server
}

func postConnectorJSON(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
