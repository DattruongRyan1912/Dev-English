package platform

import (
	"fmt"
	"strings"
)

// Built-in module IDs are stable configuration values. They describe code
// already linked into the binary; they are not plugin paths.
const (
	ModulePlatform   = "platform"
	ModuleWork       = "work"
	ModuleKnowledge  = "knowledge"
	ModuleConnectors = "connectors"
	ModuleAssistant  = "assistant"
	ModuleActions    = "actions"
	ModuleMCP        = "mcp"
	ModuleLearning   = "learning"
)

const builtinModuleVersion = "1.0.0"

// BuiltinRegistry returns the registry for every module compiled into the
// DevEnglish binary. Keeping this list in code prevents a runtime manifest
// from loading arbitrary code or silently referring to an unlinked module.
func BuiltinRegistry() (*Registry, error) {
	registry := NewRegistry()
	if err := registry.RegisterBuiltins(
		ModuleDescriptor{ID: ModulePlatform, Version: builtinModuleVersion, Capabilities: []string{"workspace", "health", "migrations"}},
		ModuleDescriptor{ID: ModuleWork, Version: builtinModuleVersion, Dependencies: []string{ModulePlatform}, Capabilities: []string{"projects", "tasks", "decisions", "history", "trash"}},
		ModuleDescriptor{ID: ModuleKnowledge, Version: builtinModuleVersion, Dependencies: []string{ModulePlatform}, Capabilities: []string{"sources", "revisions", "evidence", "retrieval"}},
		ModuleDescriptor{ID: ModuleConnectors, Version: builtinModuleVersion, Dependencies: []string{ModulePlatform, ModuleKnowledge}, Capabilities: []string{"drive.read", "github.read", "sync"}},
		ModuleDescriptor{ID: ModuleAssistant, Version: builtinModuleVersion, Dependencies: []string{ModulePlatform, ModuleWork, ModuleKnowledge}, Capabilities: []string{"conversations", "grounding", "provider-routing"}},
		ModuleDescriptor{ID: ModuleActions, Version: builtinModuleVersion, Dependencies: []string{ModulePlatform, ModuleWork, ModuleConnectors}, Capabilities: []string{"challenge", "confirmation", "receipt"}},
		ModuleDescriptor{ID: ModuleMCP, Version: builtinModuleVersion, Dependencies: []string{ModulePlatform, ModuleWork, ModuleKnowledge, ModuleConnectors, ModuleAssistant, ModuleActions}, Capabilities: []string{"streamable-http", "tools", "resources"}},
		ModuleDescriptor{ID: ModuleLearning, Version: builtinModuleVersion, Dependencies: []string{ModulePlatform}, Capabilities: []string{"overlay", "observations", "legacy-learning"}},
	); err != nil {
		return nil, fmt.Errorf("register built-in modules: %w", err)
	}
	return registry, nil
}

// DefaultManifest selects every compiled-in module. An empty environment
// variable is intentionally equivalent to this default for local development
// and existing deployments.
func DefaultManifest() Manifest {
	return NewManifest(ModulePlatform, ModuleWork, ModuleKnowledge, ModuleConnectors, ModuleAssistant, ModuleActions, ModuleMCP, ModuleLearning)
}

// ParseManifest parses DEVENGLISH_MODULES as a comma-separated allowlist. The
// special values "all" and "*" select the default compiled-in manifest.
// Every selected module must include its declared dependencies.
func ParseManifest(raw string) (Manifest, error) {
	registry, err := BuiltinRegistry()
	if err != nil {
		return Manifest{}, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" || strings.EqualFold(raw, "all") {
		manifest := DefaultManifest()
		if err := registry.ValidateManifest(manifest); err != nil {
			return Manifest{}, err
		}
		return manifest, nil
	}

	parts := strings.Split(raw, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.EqualFold(part, "all") || part == "*" {
			return Manifest{}, fmt.Errorf("%w: all or * cannot be combined with module IDs", ErrInvalidManifest)
		}
	}
	manifest := NewManifest(parts...)
	if err := registry.ValidateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// Enabled reports whether a module is selected. A zero manifest is treated as
// "all" so embedders that construct Server directly retain the historical
// default until they opt into explicit selection.
func (m Manifest) Enabled(id string) bool {
	if len(m.Modules) == 0 {
		return true
	}
	id = strings.TrimSpace(id)
	for _, selected := range m.Modules {
		if strings.TrimSpace(selected) == id {
			return true
		}
	}
	return false
}
