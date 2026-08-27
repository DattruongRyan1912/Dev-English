package mcp

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

const defaultInputSchema = `{"type":"object","additionalProperties":true}`

// Invocation is the application-neutral input to a registered MCP tool.
// Handlers receive the authenticated principal but never the bearer secret.
type Invocation struct {
	Principal      Principal
	RequestID      json.RawMessage
	Arguments      json.RawMessage
	IdempotencyKey string
	Nonce          string
}

type ToolHandler func(context.Context, Invocation) (CallToolResult, error)

type Tool struct {
	Name          string
	Description   string
	InputSchema   json.RawMessage
	RequiredScope Scope
	Mutating      bool
	Handler       ToolHandler
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type ResourceReader func(context.Context, ResourceRequest) ([]ResourceContent, error)

type Resource struct {
	URI           string
	Name          string
	Description   string
	MIMEType      string
	RequiredScope Scope
	Reader        ResourceReader
}

type ResourceDefinition struct {
	URI         string `json:"uri"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

type ResourceRequest struct {
	Principal Principal
	URI       string
}

type Registry struct {
	mu        sync.RWMutex
	tools     map[string]Tool
	resources map[string]Resource
}

func NewRegistry() *Registry {
	return &Registry{
		tools:     make(map[string]Tool),
		resources: make(map[string]Resource),
	}
}

func (registry *Registry) RegisterTool(tool Tool) error {
	if registry == nil {
		return ErrInvalidTool
	}
	name := strings.TrimSpace(tool.Name)
	if err := validateName(name); err != nil {
		return err
	}
	if tool.Handler == nil {
		return ErrInvalidTool
	}
	if err := validateRequiredScope(tool.RequiredScope); err != nil {
		return err
	}
	if tool.Mutating && !isWriteScope(tool.RequiredScope) {
		return ErrInvalidTool
	}
	schema, err := normalizeSchema(tool.InputSchema)
	if err != nil {
		return err
	}
	tool.Name = name
	tool.InputSchema = schema

	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.tools == nil {
		registry.tools = make(map[string]Tool)
	}
	if _, exists := registry.tools[name]; exists {
		return ErrDuplicateTool
	}
	registry.tools[name] = cloneTool(tool)
	return nil
}

func (registry *Registry) RegisterResource(resource Resource) error {
	if registry == nil {
		return ErrInvalidResource
	}
	uri := strings.TrimSpace(resource.URI)
	if uri == "" || len(uri) > 2048 || strings.IndexFunc(uri, isControl) >= 0 {
		return ErrInvalidResource
	}
	if resource.Reader == nil {
		return ErrInvalidResource
	}
	if err := validateRequiredScope(resource.RequiredScope); err != nil {
		return err
	}
	resource.URI = uri

	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.resources == nil {
		registry.resources = make(map[string]Resource)
	}
	if _, exists := registry.resources[uri]; exists {
		return ErrDuplicateResource
	}
	registry.resources[uri] = resource
	return nil
}

func (registry *Registry) ListTools(principal Principal) []ToolDefinition {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	result := make([]ToolDefinition, 0, len(registry.tools))
	for _, tool := range registry.tools {
		if !principal.HasScope(tool.RequiredScope) {
			continue
		}
		result = append(result, ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: append(json.RawMessage(nil), tool.InputSchema...),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (registry *Registry) ListResources(principal Principal) []ResourceDefinition {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	result := make([]ResourceDefinition, 0, len(registry.resources))
	for _, resource := range registry.resources {
		if !principal.HasScope(resource.RequiredScope) {
			continue
		}
		result = append(result, ResourceDefinition{
			URI:         resource.URI,
			Name:        resource.Name,
			Description: resource.Description,
			MIMEType:    resource.MIMEType,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].URI < result[j].URI })
	return result
}

func (registry *Registry) tool(name string) (Tool, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	tool, ok := registry.tools[name]
	if !ok {
		return Tool{}, false
	}
	return cloneTool(tool), true
}

func (registry *Registry) resource(uri string) (Resource, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	resource, ok := registry.resources[uri]
	return resource, ok
}

func cloneTool(tool Tool) Tool {
	tool.InputSchema = append(json.RawMessage(nil), tool.InputSchema...)
	return tool
}

func normalizeSchema(schema json.RawMessage) (json.RawMessage, error) {
	if len(strings.TrimSpace(string(schema))) == 0 {
		return json.RawMessage(defaultInputSchema), nil
	}
	var value map[string]any
	if err := json.Unmarshal(schema, &value); err != nil || value == nil {
		return nil, ErrInvalidTool
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return nil, ErrInvalidTool
	}
	return normalized, nil
}

func validateName(name string) error {
	if name == "" || len(name) > 128 || strings.IndexFunc(name, isControl) >= 0 {
		return ErrInvalidTool
	}
	return nil
}

func validateRequiredScope(scope Scope) error {
	if scope == "" {
		return nil
	}
	if _, ok := knownScopes[scope]; !ok {
		return ErrUnknownScope
	}
	return nil
}

func isControl(r rune) bool {
	return r < 0x20 || r == 0x7f
}
