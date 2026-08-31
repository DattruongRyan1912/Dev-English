package platform

import (
	"fmt"
	"sort"
	"strings"
)

// Registry contains descriptors for modules linked into the application.
// Registration is explicit and compile-time oriented: this package does not
// inspect files, load shared objects, or resolve dynamic plugins.
type Registry struct {
	modules map[string]ModuleDescriptor
}

// NewRegistry returns an empty registry ready for built-in registration.
func NewRegistry() *Registry {
	return &Registry{modules: make(map[string]ModuleDescriptor)}
}

// RegisterBuiltin registers one module descriptor. The descriptor is copied,
// and a duplicate ID is rejected without changing the registry.
func (r *Registry) RegisterBuiltin(descriptor ModuleDescriptor) error {
	return r.RegisterBuiltins(descriptor)
}

// RegisterBuiltins atomically registers a group of compiled-in descriptors.
// This allows modules to declare dependencies before those dependencies are
// registered; the complete graph is checked when a manifest is validated.
func (r *Registry) RegisterBuiltins(descriptors ...ModuleDescriptor) error {
	if r == nil {
		return ErrNilRegistry
	}

	updated := r.cloneModules()
	for _, descriptor := range descriptors {
		normalized, err := descriptor.normalized()
		if err != nil {
			return err
		}
		if _, exists := updated[normalized.ID]; exists {
			return fmt.Errorf("%w: module %q is already registered", ErrDuplicateModule, normalized.ID)
		}
		updated[normalized.ID] = normalized.clone()
	}
	r.modules = updated
	return nil
}

// RegisterBuiltin is the package-level convenience form for callers that
// assemble a registry through registration functions.
func RegisterBuiltin(registry *Registry, descriptor ModuleDescriptor) error {
	if registry == nil {
		return ErrNilRegistry
	}
	return registry.RegisterBuiltin(descriptor)
}

// RegisterBuiltins is the package-level convenience form for registering a
// static set of compiled-in descriptors.
func RegisterBuiltins(registry *Registry, descriptors ...ModuleDescriptor) error {
	if registry == nil {
		return ErrNilRegistry
	}
	return registry.RegisterBuiltins(descriptors...)
}

// Lookup returns a defensive copy of a registered descriptor.
func (r *Registry) Lookup(id string) (ModuleDescriptor, bool) {
	if r == nil {
		return ModuleDescriptor{}, false
	}
	descriptor, ok := r.modules[strings.TrimSpace(id)]
	if !ok {
		return ModuleDescriptor{}, false
	}
	return descriptor.clone(), true
}

// Descriptors returns all registered descriptors in lexicographic ID order.
func (r *Registry) Descriptors() []ModuleDescriptor {
	if r == nil {
		return nil
	}
	ids := make([]string, 0, len(r.modules))
	for id := range r.modules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]ModuleDescriptor, 0, len(ids))
	for _, id := range ids {
		result = append(result, r.modules[id].clone())
	}
	return result
}

// Manifest returns a manifest containing every registered module in stable
// order. It is useful for composing a default all-built-in application.
func (r *Registry) Manifest() Manifest {
	descriptors := r.Descriptors()
	ids := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	return Manifest{Modules: ids}
}

// ValidateManifest verifies that every selected module is registered, every
// selected dependency is present, and the selected graph is acyclic.
func (r *Registry) ValidateManifest(manifest Manifest) error {
	if r == nil {
		return ErrNilRegistry
	}
	normalized, err := manifest.normalized()
	if err != nil {
		return err
	}
	selected := selectedIDs(normalized)
	ids := sortedIDs(selected)

	for _, id := range ids {
		if _, exists := r.modules[id]; !exists {
			return fmt.Errorf("%w: module %q is not registered", ErrUnknownModule, id)
		}
	}
	for _, id := range ids {
		descriptor := r.modules[id]
		for _, dependency := range sortedStrings(descriptor.Dependencies) {
			if _, exists := r.modules[dependency]; !exists {
				return fmt.Errorf("%w: module %q declares unknown dependency %q", ErrUnknownModule, id, dependency)
			}
			if _, selected := selected[dependency]; !selected {
				return fmt.Errorf("%w: module %q requires %q", ErrMissingDependency, id, dependency)
			}
		}
	}
	_, err = r.orderValidated(selected)
	return err
}

// ValidateManifest is the package-level convenience form of Registry's
// manifest validation method.
func ValidateManifest(registry *Registry, manifest Manifest) error {
	if registry == nil {
		return ErrNilRegistry
	}
	return registry.ValidateManifest(manifest)
}

// Order returns selected descriptors in dependency-first topological order.
// Ties are broken lexicographically by module ID, making output independent
// of registration and manifest input order.
func (r *Registry) Order(manifest Manifest) ([]ModuleDescriptor, error) {
	if r == nil {
		return nil, ErrNilRegistry
	}
	if err := r.ValidateManifest(manifest); err != nil {
		return nil, err
	}
	normalized, _ := manifest.normalized()
	ordered, err := r.orderValidated(selectedIDs(normalized))
	if err != nil {
		return nil, err
	}
	return ordered, nil
}

// DependencyOrder is an explicit alias for Order for callers who prefer the
// graph operation's name.
func (r *Registry) DependencyOrder(manifest Manifest) ([]ModuleDescriptor, error) {
	return r.Order(manifest)
}

// OrderIDs returns the dependency-first order as module IDs.
func (r *Registry) OrderIDs(manifest Manifest) ([]string, error) {
	ordered, err := r.Order(manifest)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(ordered))
	for _, descriptor := range ordered {
		ids = append(ids, descriptor.ID)
	}
	return ids, nil
}

// DependencyOrder returns IDs in dependency-first topological order.
func DependencyOrder(registry *Registry, manifest Manifest) ([]string, error) {
	if registry == nil {
		return nil, ErrNilRegistry
	}
	return registry.OrderIDs(manifest)
}

func (r *Registry) orderValidated(selected map[string]struct{}) ([]ModuleDescriptor, error) {
	indegree := make(map[string]int, len(selected))
	dependents := make(map[string][]string)
	for id := range selected {
		indegree[id] = 0
	}
	for id := range selected {
		descriptor := r.modules[id]
		for _, dependency := range descriptor.Dependencies {
			if _, included := selected[dependency]; !included {
				continue
			}
			indegree[id]++
			dependents[dependency] = append(dependents[dependency], id)
		}
	}
	for dependency := range dependents {
		sort.Strings(dependents[dependency])
	}

	ready := make([]string, 0)
	for id, degree := range indegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)

	ordered := make([]ModuleDescriptor, 0, len(selected))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, r.modules[id].clone())

		for _, dependent := range dependents[id] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
		sort.Strings(ready)
	}

	if len(ordered) != len(selected) {
		remaining := make([]string, 0, len(selected)-len(ordered))
		for id, degree := range indegree {
			if degree > 0 {
				remaining = append(remaining, id)
			}
		}
		sort.Strings(remaining)
		return nil, fmt.Errorf("%w: %s", ErrDependencyCycle, strings.Join(remaining, ", "))
	}
	return ordered, nil
}

func (r *Registry) cloneModules() map[string]ModuleDescriptor {
	updated := make(map[string]ModuleDescriptor, len(r.modules)+1)
	for id, descriptor := range r.modules {
		updated[id] = descriptor.clone()
	}
	return updated
}

func selectedIDs(manifest Manifest) map[string]struct{} {
	selected := make(map[string]struct{}, len(manifest.Modules))
	for _, id := range manifest.Modules {
		selected[id] = struct{}{}
	}
	return selected
}

func sortedIDs(values map[string]struct{}) []string {
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
