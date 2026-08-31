package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPEmbeddingProviderSeparatesQueryAndPassageInputs(t *testing.T) {
	seen := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Text      string `json:"text"`
			InputType string `json:"input_type"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		seen <- payload.InputType + ":" + payload.Text
		_ = json.NewEncoder(writer).Encode(struct {
			Embedding []float32 `json:"embedding"`
		}{Embedding: make([]float32, EmbeddingDimensions)})
	}))
	defer server.Close()

	provider := &HTTPEmbeddingProvider{BaseURL: server.URL, Client: server.Client()}
	if _, err := provider.Embed(context.Background(), "find retry policy"); err != nil {
		t.Fatalf("query embedding: %v", err)
	}
	if _, err := provider.EmbedDocument(context.Background(), "The service retries twice."); err != nil {
		t.Fatalf("passage embedding: %v", err)
	}
	if got := <-seen; got != "query:find retry policy" {
		t.Fatalf("query payload = %q", got)
	}
	if got := <-seen; got != "passage:The service retries twice." {
		t.Fatalf("passage payload = %q", got)
	}
}

func TestHTTPEmbeddingProviderRejectsInvalidResponses(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		body       any
		wantErr    error
	}{
		{name: "wrong dimensions", statusCode: http.StatusOK, body: struct {
			Embedding []float32 `json:"embedding"`
		}{Embedding: make([]float32, EmbeddingDimensions-1)}},
		{name: "invalid json", statusCode: http.StatusOK, body: "not-json"},
		{name: "provider error", statusCode: http.StatusTooManyRequests, body: map[string]string{"error": "quota"}, wantErr: ErrEmbeddingUnavailable},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(testCase.statusCode)
				switch body := testCase.body.(type) {
				case string:
					_, _ = writer.Write([]byte(body))
				default:
					_ = json.NewEncoder(writer).Encode(body)
				}
			}))
			defer server.Close()

			provider := &HTTPEmbeddingProvider{BaseURL: server.URL, Client: server.Client()}
			_, err := provider.Embed(context.Background(), "query")
			if err == nil {
				t.Fatal("expected embedding response error")
			}
			if testCase.wantErr != nil && !errors.Is(err, testCase.wantErr) {
				t.Fatalf("error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestHTTPEmbeddingProviderRejectsOversizedInput(t *testing.T) {
	provider := &HTTPEmbeddingProvider{BaseURL: "http://127.0.0.1:1"}
	_, err := provider.Embed(context.Background(), strings.Repeat("x", embeddingRequestLimit))
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("oversized embedding input error = %v", err)
	}
}

func TestNewHTTPEmbeddingProviderFromEnvTrimsURLAndDisablesWhenUnset(t *testing.T) {
	t.Setenv("EMBEDDING_SIDECAR_URL", "  http://embedding.local/  ")
	provider := NewHTTPEmbeddingProviderFromEnv()
	if provider == nil || provider.BaseURL != "http://embedding.local" || provider.Client == nil {
		t.Fatalf("provider from environment = %+v, want trimmed URL and client", provider)
	}

	t.Setenv("EMBEDDING_SIDECAR_URL", "  ")
	if provider := NewHTTPEmbeddingProviderFromEnv(); provider != nil {
		t.Fatalf("blank embedding URL returned provider %+v, want nil", provider)
	}
}
