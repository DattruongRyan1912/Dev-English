package mcp

import (
	"sort"
	"strings"
)

// Scope is an allow-listed permission for the MCP boundary.
type Scope string

const (
	ScopeKnowledgeRead Scope = "knowledge:read"
	ScopeWorkRead      Scope = "work:read"
	ScopeWorkWrite     Scope = "work:write"
	ScopeAssistantUse  Scope = "assistant:use"
	ScopeGitHubWrite   Scope = "github:write"
)

var knownScopes = map[Scope]struct{}{
	ScopeKnowledgeRead: {},
	ScopeWorkRead:      {},
	ScopeWorkWrite:     {},
	ScopeAssistantUse:  {},
	ScopeGitHubWrite:   {},
}

var knownWriteScopes = map[Scope]struct{}{
	ScopeWorkWrite:   {},
	ScopeGitHubWrite: {},
}

// KnownScopes returns a sorted copy of the scopes supported by this adapter.
func KnownScopes() []Scope {
	result := make([]Scope, 0, len(knownScopes))
	for scope := range knownScopes {
		result = append(result, scope)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

// ParseScopes parses a space- or comma-separated OAuth-style scope string.
// Unknown scopes are rejected so a typo never silently grants less or more
// access than the caller intended.
func ParseScopes(value string) ([]Scope, error) {
	value = strings.ReplaceAll(value, ",", " ")
	parts := strings.Fields(value)
	result := make([]Scope, 0, len(parts))
	seen := make(map[Scope]struct{}, len(parts))
	for _, part := range parts {
		scope := Scope(strings.TrimSpace(part))
		if _, ok := knownScopes[scope]; !ok {
			return nil, ErrUnknownScope
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}
	if len(result) == 0 && strings.TrimSpace(value) != "" {
		return nil, ErrUnknownScope
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

// ParseScopeList validates and normalizes a list commonly received from an
// issue token endpoint or configuration file.
func ParseScopeList(values []string) ([]Scope, error) {
	return ParseScopes(strings.Join(values, " "))
}

// ScopeSet is an immutable-by-convention set used by Principal and registry
// authorization checks. Constructors copy their input.
type ScopeSet map[Scope]struct{}

func NewScopeSet(scopes ...Scope) (ScopeSet, error) {
	set := make(ScopeSet, len(scopes))
	for _, scope := range scopes {
		if _, ok := knownScopes[scope]; !ok {
			return nil, ErrUnknownScope
		}
		set[scope] = struct{}{}
	}
	return set, nil
}

func (s ScopeSet) Has(scope Scope) bool {
	_, ok := s[scope]
	return ok
}

func (s ScopeSet) Require(scope Scope) error {
	if scope == "" {
		return nil
	}
	if s.Has(scope) {
		return nil
	}
	return ErrMissingScope
}

func (s ScopeSet) List() []Scope {
	result := make([]Scope, 0, len(s))
	for scope := range s {
		result = append(result, scope)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func isWriteScope(scope Scope) bool {
	_, ok := knownWriteScopes[scope]
	return ok
}
