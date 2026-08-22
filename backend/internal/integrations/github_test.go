package integrations

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubClientImportsPublicPullRequestAndChangedFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/acme/api/pulls/42":
			_ = json.NewEncoder(w).Encode(map[string]any{"title": "Add retry", "body": "The provider occasionally returns 502.", "state": "open", "html_url": "https://github.com/acme/api/pull/42", "user": map[string]string{"login": "dev"}})
		case "/repos/acme/api/pulls/42/files":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"filename": "retry.go", "status": "modified", "additions": 10, "deletions": 2}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &GitHubClient{APIBaseURL: server.URL, Client: server.Client()}
	result, err := client.Import(context.Background(), "https://github.com/acme/api/pull/42")
	if err != nil {
		t.Fatal(err)
	}
	if result.SourceType != "github-pull" || result.Title != "Add retry" || len(result.Content) == 0 {
		t.Fatalf("unexpected import: %+v", result)
	}
}

func TestGitHubClientDecodesREADMEContentAndRejectsUnsafeURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/api/readme" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "# API\nUse the bounded retry.")
	}))
	defer server.Close()

	client := &GitHubClient{APIBaseURL: server.URL, Client: server.Client()}
	result, err := client.Import(context.Background(), "https://github.com/acme/api")
	if err != nil || result.Content == "" {
		t.Fatalf("unexpected README import: %+v err=%v", result, err)
	}
	if _, err := client.Import(context.Background(), "https://example.com/acme/api"); err == nil {
		t.Fatal("unsafe host must be rejected")
	}
}
