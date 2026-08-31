package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearerRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (transport bearerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	base := transport.base
	if base == nil {
		base = http.DefaultTransport
	}
	clone := request.Clone(request.Context())
	clone.Header.Set(AuthorizationHeader, "Bearer "+transport.token)
	return base.RoundTrip(clone)
}

func TestStreamableHTTPOfficialGoSDKConformance(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterTool(Tool{
		Name:          "knowledge_search",
		Description:   "Search verified knowledge",
		RequiredScope: ScopeKnowledgeRead,
		Handler: func(context.Context, Invocation) (CallToolResult, error) {
			return CallToolResult{Content: []Content{{Type: "text", Text: "verified result"}}}, nil
		},
	}); err != nil {
		t.Fatalf("RegisterTool() error = %v", err)
	}
	resourceText := "verified resource"
	if err := registry.RegisterResource(Resource{
		URI:           "urn:devenglish:knowledge",
		Name:          "Verified knowledge",
		MIMEType:      "text/plain",
		RequiredScope: ScopeKnowledgeRead,
		Reader: func(context.Context, ResourceRequest) ([]ResourceContent, error) {
			return []ResourceContent{{
				URI:      "urn:devenglish:knowledge",
				MIMEType: "text/plain",
				Text:     &resourceText,
			}}, nil
		},
	}); err != nil {
		t.Fatalf("RegisterResource() error = %v", err)
	}

	tokens := NewTokenStore()
	issued, err := tokens.Issue([]Scope{ScopeKnowledgeRead})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	token, err := issued.Reveal()
	if err != nil {
		t.Fatalf("Reveal() error = %v", err)
	}

	server := httptest.NewServer(NewHandler(registry, tokens))
	defer server.Close()

	transport := &officialmcp.StreamableClientTransport{
		Endpoint: server.URL,
		HTTPClient: &http.Client{Transport: bearerRoundTripper{
			base:  http.DefaultTransport,
			token: token,
		}},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}
	client := officialmcp.NewClient(&officialmcp.Implementation{
		Name:    "devenglish-conformance-test",
		Version: "1.0.0",
	}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("official SDK Connect() error = %v", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("official SDK Close() error = %v", err)
		}
	}()

	if got := session.InitializeResult().ProtocolVersion; got != ModernProtocolVersion {
		t.Fatalf("negotiated protocol version = %q, want %q", got, ModernProtocolVersion)
	}
	if got := session.InitializeResult().ServerInfo.Name; got != DefaultServerName {
		t.Fatalf("server name = %q, want %q", got, DefaultServerName)
	}

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("official SDK ListTools() error = %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "knowledge_search" {
		t.Fatalf("official SDK tools = %#v", tools.Tools)
	}

	resources, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("official SDK ListResources() error = %v", err)
	}
	if len(resources.Resources) != 1 || resources.Resources[0].URI != "urn:devenglish:knowledge" {
		t.Fatalf("official SDK resources = %#v", resources.Resources)
	}

	contents, err := session.ReadResource(ctx, &officialmcp.ReadResourceParams{URI: "urn:devenglish:knowledge"})
	if err != nil {
		t.Fatalf("official SDK ReadResource() error = %v", err)
	}
	if len(contents.Contents) != 1 || contents.Contents[0].Text != resourceText {
		t.Fatalf("official SDK resource contents = %#v", contents.Contents)
	}

	toolResult, err := session.CallTool(ctx, &officialmcp.CallToolParams{
		Name:      "knowledge_search",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("official SDK CallTool() error = %v", err)
	}
	if len(toolResult.Content) != 1 {
		t.Fatalf("official SDK tool result = %#v", toolResult.Content)
	}
	content, ok := toolResult.Content[0].(*officialmcp.TextContent)
	if !ok || strings.TrimSpace(content.Text) != "verified result" {
		t.Fatalf("official SDK tool content = %#v", toolResult.Content[0])
	}
}
