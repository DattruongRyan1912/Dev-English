package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"golang.org/x/oauth2"
)

type staticDriveTokenSource struct {
	token *oauth2.Token
	err   error
}

func (s staticDriveTokenSource) Token() (*oauth2.Token, error) {
	return s.token, s.err
}

func TestNewGoogleDriveFromEnvConfiguresReadOnlyRefreshSource(t *testing.T) {
	t.Setenv("GOOGLE_DRIVE_API_BASE_URL", "")
	t.Setenv("GOOGLE_DRIVE_ACCESS_TOKEN", "")
	t.Setenv("DRIVE_ACCESS_TOKEN", "")
	t.Setenv("GOOGLE_DRIVE_CLIENT_ID", "client-id")
	t.Setenv("GOOGLE_DRIVE_CLIENT_SECRET", "client-secret")
	t.Setenv("GOOGLE_DRIVE_REFRESH_TOKEN", "refresh-token")

	reader := NewGoogleDriveFromEnv()
	if reader.APIBaseURL != defaultGoogleDriveAPIBase || !reader.Configured() || reader.TokenSource == nil {
		t.Fatalf("refresh-token Drive configuration = %+v", reader)
	}
	if reader.UseChangesAPI != true {
		t.Fatal("production Drive reader must use the Changes API")
	}
	if reader.Client == nil || reader.Client.Timeout != googleDriveHTTPTimeout {
		t.Fatalf("Drive client timeout = %v, want %v", reader.Client.Timeout, googleDriveHTTPTimeout)
	}
}

func TestGoogleDriveTokenSourceUsesBoundedHTTPClient(t *testing.T) {
	var requests int
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodPost || req.URL.String() != "https://oauth.test/token" {
			t.Fatalf("unexpected token request: %s %s", req.Method, req.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"drive-access-token","token_type":"Bearer","expires_in":3600}`)),
			Request:    req,
		}, nil
	})}
	config := &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://oauth.test/token",
		},
	}
	tokenSource := newGoogleDriveTokenSource(config, "refresh-token", client)
	token, err := tokenSource.Token()
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "drive-access-token" || requests != 1 {
		t.Fatalf("token source result = %+v, requests = %d", token, requests)
	}
	if client.Timeout != 0 {
		t.Fatalf("test client should remain injectable, got timeout %v", client.Timeout)
	}
}

func TestGoogleDriveReaderPropagatesCancellationToOAuthRefresh(t *testing.T) {
	started := make(chan struct{})
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	config := &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://oauth.test/token",
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := &GoogleDriveReader{
		APIBaseURL:    "https://drive.test",
		TokenSource:   newGoogleDriveTokenSource(config, "refresh-token", client),
		UseChangesAPI: false,
	}
	result := make(chan error, 1)
	go func() {
		_, err := reader.List(ctx, connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 1})
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("OAuth refresh did not reach the injected token endpoint")
	}
	cancel()
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "access token refresh failed") {
			t.Fatalf("cancellation error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("OAuth refresh did not stop after request cancellation")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGoogleDriveReaderUsesTokenSourceWhenStaticTokenIsAbsent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer refreshed-drive-token" {
			t.Fatalf("authorization header did not use refreshed token")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"files": []any{}})
	}))
	defer server.Close()

	reader := &GoogleDriveReader{
		APIBaseURL:    server.URL,
		TokenSource:   staticDriveTokenSource{token: &oauth2.Token{AccessToken: "refreshed-drive-token"}},
		Client:        server.Client(),
		UseChangesAPI: false,
	}
	if _, err := reader.List(context.Background(), connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestGoogleDriveReaderFailsClosedWhenTokenSourceCannotRefresh(t *testing.T) {
	reader := &GoogleDriveReader{
		APIBaseURL:    "http://127.0.0.1:1",
		TokenSource:   staticDriveTokenSource{err: errors.New("refresh secret must not escape")},
		UseChangesAPI: false,
	}
	_, err := reader.List(context.Background(), connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 1})
	if err == nil || !strings.Contains(err.Error(), "access token refresh failed") || strings.Contains(err.Error(), "refresh secret") {
		t.Fatalf("refresh failure = %v", err)
	}
}

func TestGoogleDriveReaderListsAndExportsReadableFiles(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RequestURI())
		if r.Header.Get("Authorization") != "Bearer drive-test-token" {
			t.Fatalf("authorization header was not forwarded safely")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/files":
			if r.URL.Query().Get("q") != "trashed = false" || r.URL.Query().Get("pageSize") != "2" {
				t.Fatalf("unexpected Drive list query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"nextPageToken": "drive-next-token",
				"files": []map[string]any{
					{
						"id": "doc-1", "name": "Architecture", "mimeType": "application/vnd.google-apps.document",
						"webViewLink": "https://drive.google.com/file/d/doc-1", "modifiedTime": "2026-08-28T02:00:00Z", "headRevisionId": "rev-1",
					},
					{"id": "image-1", "name": "Screenshot", "mimeType": "image/png", "modifiedTime": "2026-08-28T02:01:00Z"},
				},
			})
		case "/files/doc-1/export":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, "  source-backed architecture notes  ")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	reader := &GoogleDriveReader{APIBaseURL: server.URL, AccessToken: "drive-test-token", Client: server.Client()}
	page, err := reader.List(context.Background(), connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].FileID != "doc-1" || page.Items[0].Text != "source-backed architecture notes" {
		t.Fatalf("unexpected normalized Drive page: %+v", page)
	}
	if !page.HasMore || page.NextCursor.Token != "drive-next-token" {
		t.Fatalf("Drive cursor was not preserved opaquely: %+v", page)
	}
	if len(requests) != 2 || !strings.Contains(requests[1], "/files/doc-1/export") {
		t.Fatalf("unexpected Drive request sequence: %v", requests)
	}
}

func TestGoogleDriveReaderRedactsProviderBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":"token=drive-secret-value"}`)
	}))
	defer server.Close()

	reader := &GoogleDriveReader{APIBaseURL: server.URL, AccessToken: "drive-test-token", Client: server.Client()}
	_, err := reader.List(context.Background(), connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 1})
	if err == nil || strings.Contains(err.Error(), "drive-secret-value") || strings.Contains(err.Error(), "token=") {
		t.Fatalf("provider response leaked or error was missing: %v", err)
	}
}

func TestGoogleDriveReaderUsesChangesFeedAndPersistsEndToken(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path+"?"+r.URL.RawQuery)
		if r.Header.Get("Authorization") != "Bearer drive-test-token" {
			t.Fatalf("authorization header was not forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/changes/startPageToken":
			_ = json.NewEncoder(w).Encode(map[string]string{"startPageToken": "start-1"})
		case "/files":
			if r.URL.Query().Get("pageToken") != "" {
				t.Fatalf("bootstrap must start without a files page token: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"files": []any{}})
		case "/changes":
			if r.URL.Query().Get("pageToken") == "start-1" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"newStartPageToken": "start-2",
					"changes": []map[string]any{{
						"fileId": "doc-1",
						"file": map[string]any{
							"id": "doc-1", "name": "Architecture", "mimeType": "application/vnd.google-apps.document",
							"modifiedTime": "2026-08-28T02:00:00Z", "headRevisionId": "rev-1",
						},
					}},
				})
				return
			}
			if r.URL.Query().Get("pageToken") == "start-2" {
				_ = json.NewEncoder(w).Encode(map[string]any{"newStartPageToken": "start-3", "changes": []any{}})
				return
			}
			t.Fatalf("unexpected Drive changes page token: %q", r.URL.Query().Get("pageToken"))
		case "/files/doc-1/export":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, "source-backed change")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	reader := &GoogleDriveReader{APIBaseURL: server.URL, AccessToken: "drive-test-token", Client: server.Client(), UseChangesAPI: true}
	first, err := reader.List(context.Background(), connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 0 || first.HasMore {
		t.Fatalf("unexpected bootstrap page: %+v", first)
	}
	if first.CheckpointCursor.Token != "start-1" {
		t.Fatalf("bootstrap checkpoint token = %+v, want start-1", first.CheckpointCursor)
	}
	second, err := reader.List(context.Background(), connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 2, Cursor: first.CheckpointCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].FileID != "doc-1" || second.Items[0].Text != "source-backed change" || second.HasMore {
		t.Fatalf("unexpected changes page: %+v", second)
	}
	if second.CheckpointCursor.Token != "start-2" {
		t.Fatalf("changes checkpoint token = %+v, want start-2", second.CheckpointCursor)
	}
	third, err := reader.List(context.Background(), connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 2, Cursor: second.CheckpointCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(third.Items) != 0 || third.HasMore || third.CheckpointCursor.Token != "start-3" {
		t.Fatalf("unexpected empty changes page: %+v", third)
	}
	if len(paths) != 5 || !strings.HasPrefix(paths[0], "/changes/startPageToken?") || !strings.HasPrefix(paths[1], "/files?") || !strings.Contains(paths[2], "pageToken=start-1") || !strings.Contains(paths[3], "/files/doc-1/export") || !strings.Contains(paths[4], "pageToken=start-2") {
		t.Fatalf("unexpected Changes API request sequence: %v", paths)
	}
}

func TestGoogleDriveReaderBootstrapsPagedFilesBeforeChanges(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/changes/startPageToken":
			_ = json.NewEncoder(w).Encode(map[string]string{"startPageToken": "start-existing"})
		case "/files":
			token := r.URL.Query().Get("pageToken")
			if token == "" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"nextPageToken": "files-page-2",
					"files": []map[string]any{{
						"id": "doc-existing", "name": "Existing notes", "mimeType": "text/plain",
						"modifiedTime": "2026-08-28T02:00:00Z", "headRevisionId": "rev-existing",
					}},
				})
				return
			}
			if token == "files-page-2" {
				_ = json.NewEncoder(w).Encode(map[string]any{"files": []any{}})
				return
			}
			t.Fatalf("unexpected files page token: %q", token)
		case "/files/doc-existing":
			_, _ = io.WriteString(w, "existing bootstrap content")
		case "/changes":
			if r.URL.Query().Get("pageToken") != "start-existing" {
				t.Fatalf("changes started from %q", r.URL.Query().Get("pageToken"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"newStartPageToken": "start-after-bootstrap", "changes": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	reader := &GoogleDriveReader{APIBaseURL: server.URL, AccessToken: "drive-test-token", Client: server.Client(), UseChangesAPI: true}
	first, err := reader.List(context.Background(), connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || first.Items[0].FileID != "doc-existing" || !first.HasMore || first.NextCursor.Token == "files-page-2" {
		t.Fatalf("bootstrap first page = %+v", first)
	}
	second, err := reader.List(context.Background(), connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 1, Cursor: first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 0 || second.HasMore || second.CheckpointCursor.Token != "start-existing" {
		t.Fatalf("bootstrap completion = %+v", second)
	}
	third, err := reader.List(context.Background(), connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 1, Cursor: second.CheckpointCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(third.Items) != 0 || third.HasMore || third.CheckpointCursor.Token != "start-after-bootstrap" {
		t.Fatalf("post-bootstrap changes = %+v", third)
	}
	if len(paths) != 5 || !strings.HasPrefix(paths[0], "/changes/startPageToken?") || !strings.HasPrefix(paths[1], "/files?") || !strings.Contains(paths[2], "/files/doc-existing") || !strings.Contains(paths[3], "pageToken=files-page-2") || !strings.Contains(paths[4], "pageToken=start-existing") {
		t.Fatalf("bootstrap request sequence = %v", paths)
	}
}

func TestGoogleDriveReaderEmitsRemovalTombstone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/changes" || r.URL.Query().Get("pageToken") != "start-removed" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"newStartPageToken": "start-after-remove",
			"changes":           []map[string]any{{"fileId": "doc-removed", "removed": true}},
		})
	}))
	defer server.Close()

	reader := &GoogleDriveReader{APIBaseURL: server.URL, AccessToken: "drive-test-token", Client: server.Client(), UseChangesAPI: true}
	page, err := reader.List(context.Background(), connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 1, Cursor: connectors.DriveCursor{Token: "start-removed"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || !page.Items[0].Removed || page.Items[0].FileID != "doc-removed" || page.Items[0].Text != "" {
		t.Fatalf("removal page = %+v", page)
	}
	if page.CheckpointCursor.Token != "start-after-remove" || page.HasMore {
		t.Fatalf("removal checkpoint = %+v", page)
	}
}

func TestGoogleDriveReaderTombstonesTrashedFilesAndAcceptsRestore(t *testing.T) {
	var fields string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/changes":
			fields = r.URL.Query().Get("fields")
			switch r.URL.Query().Get("pageToken") {
			case "start-trash":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"newStartPageToken": "start-after-trash",
					"changes": []map[string]any{{"fileId": "doc-trash", "file": map[string]any{
						"id": "doc-trash", "name": "Archived notes", "mimeType": "text/plain",
						"modifiedTime": "2026-08-28T02:00:00Z", "trashed": true,
					}}},
				})
			case "start-restore":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"newStartPageToken": "start-after-restore",
					"changes": []map[string]any{{"fileId": "doc-trash", "file": map[string]any{
						"id": "doc-trash", "name": "Restored notes", "mimeType": "text/plain",
						"modifiedTime": "2026-08-28T02:01:00Z", "headRevisionId": "rev-restored", "trashed": false,
					}}},
				})
			default:
				http.NotFound(w, r)
			}
		case "/files/doc-trash":
			_, _ = io.WriteString(w, "restored source content")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	reader := &GoogleDriveReader{APIBaseURL: server.URL, AccessToken: "drive-test-token", Client: server.Client(), UseChangesAPI: true}
	trashed, err := reader.List(context.Background(), connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 1, Cursor: connectors.DriveCursor{Token: "start-trash"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(trashed.Items) != 1 || !trashed.Items[0].Removed || trashed.Items[0].FileID != "doc-trash" || trashed.Items[0].Text != "" {
		t.Fatalf("trashed file page = %+v", trashed)
	}

	restored, err := reader.List(context.Background(), connectors.DriveListRequest{WorkspaceID: "workspace-test", PageSize: 1, Cursor: connectors.DriveCursor{Token: "start-restore"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Items) != 1 || restored.Items[0].Removed || restored.Items[0].FileID != "doc-trash" || restored.Items[0].Text != "restored source content" {
		t.Fatalf("restored file page = %+v", restored)
	}
	if !strings.Contains(fields, "trashed") {
		t.Fatalf("changes fields did not request Drive trashed state: %q", fields)
	}
}

func TestGitHubClientListsIssuesExcludesPullRequestsAndUsesStableModel(t *testing.T) {
	var sawAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization") == "Bearer github-test-token"
		if r.URL.Path == "/repos/acme/api/issues" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Link", `<`+r.URL.Scheme+"://"+r.Host+`/repos/acme/api/issues?page=2>; rel="next"`)
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"node_id": "issue-node-1", "number": 7, "title": "Retry API", "body": "Investigate timeout", "state": "open", "updated_at": "2026-08-28T02:00:00Z"},
				{"node_id": "pr-node-1", "number": 8, "title": "A pull request", "body": "not an issue", "state": "open", "updated_at": "2026-08-28T02:00:00Z", "pull_request": map[string]any{"url": "https://api.github.com/pulls/8"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := &GitHubClient{APIBaseURL: server.URL, Token: "github-test-token", Client: server.Client()}
	page, err := client.ListIssues(context.Background(), connectors.GitHubIssueListRequest{Repository: "acme/api", PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !sawAuth || len(page.Issues) != 1 || page.Issues[0].Number != 7 || page.Issues[0].Revision == "" {
		t.Fatalf("unexpected GitHub page: auth=%v page=%+v", sawAuth, page)
	}
	if !page.HasMore || page.NextCursor != "2" {
		t.Fatalf("GitHub next cursor was not normalized: %+v", page)
	}
}

func TestGitHubClientGetsIssueAndRejectsPullRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/repos/acme/api/issues/7" {
			_ = json.NewEncoder(w).Encode(map[string]any{"node_id": "issue-node-1", "number": 7, "title": "Retry API", "body": "Investigate timeout", "state": "open", "updated_at": "2026-08-28T02:00:00Z"})
			return
		}
		if r.URL.Path == "/repos/acme/api/issues/8" {
			_ = json.NewEncoder(w).Encode(map[string]any{"node_id": "pr-node-1", "number": 8, "title": "PR", "pull_request": map[string]any{"url": "https://api.github.com/pulls/8"}})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := &GitHubClient{APIBaseURL: server.URL, Client: server.Client()}
	issue, err := client.GetIssue(context.Background(), "acme/api", 7)
	if err != nil || issue.Repository != "acme/api" || issue.Number != 7 {
		t.Fatalf("unexpected normalized issue: %+v err=%v", issue, err)
	}
	if _, err := client.GetIssue(context.Background(), "acme/api", 8); err == nil {
		t.Fatal("pull request must not cross the issue read boundary")
	}
}

func TestGitHubClientWritesOnlySupportedIssueOperations(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer github-write-token" {
			t.Fatalf("write authorization was not forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/acme/api/issues":
			_ = json.NewEncoder(w).Encode(map[string]any{"node_id": "issue-node-9", "number": 9, "title": "Created", "body": "details", "state": "open", "updated_at": "2026-08-28T02:00:00Z"})
		case "/repos/acme/api/issues/9/comments":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 101, "body": "comment", "updated_at": "2026-08-28T02:01:00Z"})
		case "/repos/acme/api/issues/9/labels":
			_ = json.NewEncoder(w).Encode([]map[string]string{{"name": "bug"}, {"name": "verified"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &GitHubClient{APIBaseURL: server.URL, Token: "github-write-token", Client: server.Client()}
	metadata := connectors.SafeWriteMetadata{IdempotencyKey: "write-test"}
	issue, err := client.CreateIssue(context.Background(), connectors.CreateIssueRequest{Repository: "acme/api", Title: "Created", Body: "details", Metadata: metadata})
	if err != nil || issue.Number != 9 {
		t.Fatalf("CreateIssue() = %+v, %v", issue, err)
	}
	comment, err := client.AddIssueComment(context.Background(), connectors.CommentIssueRequest{Repository: "acme/api", Issue: 9, Body: "comment", Metadata: connectors.SafeWriteMetadata{IdempotencyKey: "comment-test"}})
	if err != nil || comment.ID != 101 {
		t.Fatalf("AddIssueComment() = %+v, %v", comment, err)
	}
	labels, err := client.SetIssueLabels(context.Background(), connectors.LabelIssueRequest{Repository: "acme/api", Issue: 9, Labels: []string{"bug", "verified"}, Metadata: connectors.SafeWriteMetadata{IdempotencyKey: "labels-test"}})
	if err != nil || len(labels) != 2 || labels[0].Name != "bug" {
		t.Fatalf("SetIssueLabels() = %+v, %v", labels, err)
	}
	if strings.Join(paths, ",") != "POST /repos/acme/api/issues,POST /repos/acme/api/issues/9/comments,POST /repos/acme/api/issues/9/labels" {
		t.Fatalf("write paths = %v", paths)
	}
}
