package platform

import (
	"fmt"
	"strings"
)

// ModuleDescriptor describes a module that is compiled into the application.
// Dependencies are module IDs and capabilities are opaque, module-defined
// strings. The registry stores a defensive copy of every descriptor.
type ModuleDescriptor struct {
	ID           string   `json:"id"`
	Version      string   `json:"version"`
	Dependencies []string `json:"dependencies,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// Validate checks the shape of a descriptor without consulting a registry.
func (d ModuleDescriptor) Validate() error {
	_, err := d.normalized()
	return err
}

func (d ModuleDescriptor) normalized() (ModuleDescriptor, error) {
	d.ID = strings.TrimSpace(d.ID)
	d.Version = strings.TrimSpace(d.Version)
	if d.ID == "" {
		return ModuleDescriptor{}, fmt.Errorf("%w: id is required", ErrInvalidDescriptor)
	}
	if d.Version == "" {
		return ModuleDescriptor{}, fmt.Errorf("%w: version is required for %q", ErrInvalidDescriptor, d.ID)
	}

	var err error
	d.Dependencies, err = normalizeUniqueValues("dependencies", d.Dependencies, d.ID)
	if err != nil {
		return ModuleDescriptor{}, err
	}
	d.Capabilities, err = normalizeUniqueValues("capabilities", d.Capabilities, d.ID)
	if err != nil {
		return ModuleDescriptor{}, err
	}
	return d, nil
}

func normalizeUniqueValues(kind string, values []string, moduleID string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, fmt.Errorf("%w: %s for %q cannot contain an empty value", ErrInvalidDescriptor, kind, moduleID)
		}
		if _, exists := seen[value]; exists {
			return nil, fmt.Errorf("%w: %s for %q contains %q more than once", ErrInvalidDescriptor, kind, moduleID, value)
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func (d ModuleDescriptor) clone() ModuleDescriptor {
	d.Dependencies = append([]string(nil), d.Dependencies...)
	d.Capabilities = append([]string(nil), d.Capabilities...)
	return d
}
