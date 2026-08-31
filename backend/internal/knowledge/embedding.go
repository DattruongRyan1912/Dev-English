package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

var ErrEmbeddingUnavailable = errors.New("embedding sidecar is unavailable")

const embeddingRequestLimit = 32 << 10

// HTTPEmbeddingProvider talks to a local embedding sidecar. The sidecar is
// optional in development; when it is unavailable the application falls back
// to PostgreSQL full-text search and marks retrieval degraded.
type HTTPEmbeddingProvider struct {
	BaseURL string
	Client  *http.Client
}

func NewHTTPEmbeddingProviderFromEnv() *HTTPEmbeddingProvider {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("EMBEDDING_SIDECAR_URL")), "/")
	if baseURL == "" {
		return nil
	}
	return &HTTPEmbeddingProvider{BaseURL: baseURL, Client: &http.Client{Timeout: 5 * time.Second}}
}

func (p *HTTPEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return p.embed(ctx, text, "query")
}

// EmbedDocument creates a passage vector for source content. It is kept
// separate from Embed so the provider can apply the model's retrieval
// instruction without making callers know about sidecar-specific details.
func (p *HTTPEmbeddingProvider) EmbedDocument(ctx context.Context, text string) ([]float32, error) {
	return p.embed(ctx, text, "passage")
}

func (p *HTTPEmbeddingProvider) embed(ctx context.Context, text, inputType string) ([]float32, error) {
	if p == nil || strings.TrimSpace(p.BaseURL) == "" {
		return nil, ErrEmbeddingUnavailable
	}
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("embedding text is required")
	}
	payload, err := json.Marshal(struct {
		Text      string `json:"text"`
		InputType string `json:"input_type"`
	}{Text: text, InputType: inputType})
	if err != nil {
		return nil, err
	}
	if len(payload) > embeddingRequestLimit {
		return nil, errors.New("embedding text is too large")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/embed", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEmbeddingUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, ErrEmbeddingUnavailable
	}
	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 16<<10)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(result.Embedding) != EmbeddingDimensions {
		return nil, fmt.Errorf("embedding has %d dimensions, want %d", len(result.Embedding), EmbeddingDimensions)
	}
	for _, value := range result.Embedding {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, errors.New("embedding contains a non-finite value")
		}
	}
	return result.Embedding, nil
}
