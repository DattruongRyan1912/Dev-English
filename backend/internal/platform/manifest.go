package platform

import (
	"fmt"
	"strings"
)

// Manifest selects the compiled-in modules that make up one application
// composition. It deliberately contains IDs only; module metadata comes from
// the built-in registry, so a manifest cannot load executable code.
type Manifest struct {
	Modules []string `json:"modules"`
}

// NewManifest creates a manifest from module IDs. IDs are copied so callers
// can safely reuse or modify their input slice after construction.
func NewManifest(moduleIDs ...string) Manifest {
	return Manifest{Modules: append([]string(nil), moduleIDs...)}
}

// Validate checks manifest-local invariants such as blank and duplicate IDs.
// Registry-dependent checks are performed by Registry.ValidateManifest.
func (m Manifest) Validate() error {
	_, err := m.normalized()
	return err
}

func (m Manifest) normalized() (Manifest, error) {
	result := Manifest{Modules: make([]string, 0, len(m.Modules))}
	seen := make(map[string]struct{}, len(m.Modules))
	for index, raw := range m.Modules {
		id := strings.TrimSpace(raw)
		if id == "" {
			return Manifest{}, fmt.Errorf("%w: module ID at index %d is required", ErrInvalidManifest, index)
		}
		if _, exists := seen[id]; exists {
			return Manifest{}, fmt.Errorf("%w: module %q appears more than once", ErrDuplicateModule, id)
		}
		seen[id] = struct{}{}
		result.Modules = append(result.Modules, id)
	}
	return result, nil
}
