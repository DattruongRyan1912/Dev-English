package platform

import (
	"errors"
	"reflect"
	"testing"
)

func TestRegistryValidatesValidManifestAndOrdersDependencies(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterBuiltins(
		ModuleDescriptor{ID: "app", Version: "1.0.0", Dependencies: []string{"content"}},
		ModuleDescriptor{ID: "content", Version: "1.0.0", Dependencies: []string{"core"}, Capabilities: []string{"catalog"}},
		ModuleDescriptor{ID: "core", Version: "1.0.0", Capabilities: []string{"runtime"}},
		ModuleDescriptor{ID: "metrics", Version: "1.0.0"},
	); err != nil {
		t.Fatal(err)
	}

	manifest := NewManifest("app", "metrics", "content", "core")
	if err := registry.ValidateManifest(manifest); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	ordered, err := registry.OrderIDs(manifest)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"core", "content", "app", "metrics"}
	if !reflect.DeepEqual(ordered, want) {
		t.Fatalf("order = %v, want %v", ordered, want)
	}
}

func TestRegistryRejectsDuplicateIDs(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterBuiltin(ModuleDescriptor{ID: "core", Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterBuiltin(ModuleDescriptor{ID: "core", Version: "2.0.0"}); !errors.Is(err, ErrDuplicateModule) {
		t.Fatalf("duplicate registration error = %v, want ErrDuplicateModule", err)
	}

	if err := registry.ValidateManifest(NewManifest("core", "core")); !errors.Is(err, ErrDuplicateModule) {
		t.Fatalf("duplicate manifest error = %v, want ErrDuplicateModule", err)
	}
}

func TestRegistryRejectsUnknownModule(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterBuiltin(ModuleDescriptor{ID: "core", Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if err := registry.ValidateManifest(NewManifest("missing")); !errors.Is(err, ErrUnknownModule) {
		t.Fatalf("unknown module error = %v, want ErrUnknownModule", err)
	}
}

func TestRegistryRejectsMissingDependency(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterBuiltins(
		ModuleDescriptor{ID: "app", Version: "1.0.0", Dependencies: []string{"core"}},
		ModuleDescriptor{ID: "core", Version: "1.0.0"},
	); err != nil {
		t.Fatal(err)
	}
	if err := registry.ValidateManifest(NewManifest("app")); !errors.Is(err, ErrMissingDependency) {
		t.Fatalf("missing dependency error = %v, want ErrMissingDependency", err)
	}
}

func TestRegistryRejectsDependencyCycle(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterBuiltins(
		ModuleDescriptor{ID: "alpha", Version: "1.0.0", Dependencies: []string{"beta"}},
		ModuleDescriptor{ID: "beta", Version: "1.0.0", Dependencies: []string{"alpha"}},
	); err != nil {
		t.Fatal(err)
	}
	if err := registry.ValidateManifest(NewManifest("alpha", "beta")); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("cycle error = %v, want ErrDependencyCycle", err)
	}
}

func TestRegistryDependencyOrderIsDeterministic(t *testing.T) {
	modules := []ModuleDescriptor{
		{ID: "web", Version: "1.0.0", Dependencies: []string{"service"}},
		{ID: "worker", Version: "1.0.0", Dependencies: []string{"service"}},
		{ID: "service", Version: "1.0.0", Dependencies: []string{"core"}},
		{ID: "core", Version: "1.0.0"},
		{ID: "audit", Version: "1.0.0", Dependencies: []string{"core"}},
	}

	first := NewRegistry()
	if err := first.RegisterBuiltins(modules...); err != nil {
		t.Fatal(err)
	}
	second := NewRegistry()
	reversed := append([]ModuleDescriptor(nil), modules...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	if err := second.RegisterBuiltins(reversed...); err != nil {
		t.Fatal(err)
	}

	firstOrder, err := first.OrderIDs(NewManifest("worker", "audit", "web", "service", "core"))
	if err != nil {
		t.Fatal(err)
	}
	secondOrder, err := second.OrderIDs(NewManifest("core", "service", "web", "audit", "worker"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(firstOrder, secondOrder) {
		t.Fatalf("orders differ: first=%v second=%v", firstOrder, secondOrder)
	}
	want := []string{"core", "audit", "service", "web", "worker"}
	if !reflect.DeepEqual(firstOrder, want) {
		t.Fatalf("order = %v, want %v", firstOrder, want)
	}
}

func TestBuiltinRegistrationCopiesDescriptor(t *testing.T) {
	descriptor := ModuleDescriptor{ID: "core", Version: "1.0.0", Dependencies: []string{"base"}, Capabilities: []string{"runtime"}}
	registry := NewRegistry()
	if err := RegisterBuiltin(registry, descriptor); err != nil {
		t.Fatal(err)
	}
	descriptor.Dependencies[0] = "mutated"
	descriptor.Capabilities[0] = "mutated"

	got, ok := registry.Lookup("core")
	if !ok {
		t.Fatal("registered descriptor not found")
	}
	want := ModuleDescriptor{ID: "core", Version: "1.0.0", Dependencies: []string{"base"}, Capabilities: []string{"runtime"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("descriptor = %+v, want %+v", got, want)
	}
}

func TestManifestAndDescriptorValidationCoversLocalBoundaries(t *testing.T) {
	tests := []struct {
		name string
		got  error
		want error
	}{
		{"missing descriptor id", (ModuleDescriptor{Version: "1.0.0"}).Validate(), ErrInvalidDescriptor},
		{"missing descriptor version", (ModuleDescriptor{ID: "core"}).Validate(), ErrInvalidDescriptor},
		{"empty dependency", (ModuleDescriptor{ID: "core", Version: "1.0.0", Dependencies: []string{" "}}).Validate(), ErrInvalidDescriptor},
		{"duplicate dependency", (ModuleDescriptor{ID: "core", Version: "1.0.0", Dependencies: []string{"base", "base"}}).Validate(), ErrInvalidDescriptor},
		{"empty capability", (ModuleDescriptor{ID: "core", Version: "1.0.0", Capabilities: []string{" "}}).Validate(), ErrInvalidDescriptor},
		{"duplicate capability", (ModuleDescriptor{ID: "core", Version: "1.0.0", Capabilities: []string{"runtime", "runtime"}}).Validate(), ErrInvalidDescriptor},
		{"blank manifest id", NewManifest("core", " ").Validate(), ErrInvalidManifest},
		{"duplicate manifest id", NewManifest("core", "core").Validate(), ErrDuplicateModule},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !errors.Is(test.got, test.want) {
				t.Fatalf("error = %v, want %v", test.got, test.want)
			}
		})
	}

	trimmed := ModuleDescriptor{ID: " core ", Version: " 1.0.0 ", Dependencies: []string{" base "}, Capabilities: []string{" runtime "}}
	if err := trimmed.Validate(); err != nil {
		t.Fatalf("trimmed descriptor rejected: %v", err)
	}
	manifest := NewManifest(" core ")
	if err := manifest.Validate(); err != nil {
		t.Fatalf("trimmed manifest rejected: %v", err)
	}
}

func TestRegistryNilAndPackageConvenienceBoundaries(t *testing.T) {
	var registry *Registry
	if _, ok := registry.Lookup("core"); ok {
		t.Fatal("nil registry unexpectedly returned a module")
	}
	if registry.Descriptors() != nil || len(registry.Manifest().Modules) != 0 {
		t.Fatal("nil registry returned descriptors or manifest modules")
	}
	if !errors.Is(registry.ValidateManifest(NewManifest("core")), ErrNilRegistry) {
		t.Fatal("nil registry validation did not return ErrNilRegistry")
	}
	if _, err := registry.Order(NewManifest("core")); !errors.Is(err, ErrNilRegistry) {
		t.Fatalf("nil registry order error = %v", err)
	}
	if _, err := registry.DependencyOrder(NewManifest("core")); !errors.Is(err, ErrNilRegistry) {
		t.Fatalf("nil registry dependency order error = %v", err)
	}
	if _, err := registry.OrderIDs(NewManifest("core")); !errors.Is(err, ErrNilRegistry) {
		t.Fatalf("nil registry order IDs error = %v", err)
	}
	if err := RegisterBuiltin(nil, ModuleDescriptor{ID: "core", Version: "1.0.0"}); !errors.Is(err, ErrNilRegistry) {
		t.Fatalf("package RegisterBuiltin error = %v", err)
	}
	if err := RegisterBuiltins(nil, ModuleDescriptor{ID: "core", Version: "1.0.0"}); !errors.Is(err, ErrNilRegistry) {
		t.Fatalf("package RegisterBuiltins error = %v", err)
	}
	if err := ValidateManifest(nil, NewManifest("core")); !errors.Is(err, ErrNilRegistry) {
		t.Fatalf("package ValidateManifest error = %v", err)
	}
	if _, err := DependencyOrder(nil, NewManifest("core")); !errors.Is(err, ErrNilRegistry) {
		t.Fatalf("package DependencyOrder error = %v", err)
	}
}

func TestRegistryRejectsUnknownDependencyAndPreservesAtomicRegistration(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterBuiltin(ModuleDescriptor{ID: "stable", Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterBuiltins(
		ModuleDescriptor{ID: "invalid", Version: "1.0.0", Dependencies: []string{"missing"}},
		ModuleDescriptor{ID: "", Version: "1.0.0"},
	); !errors.Is(err, ErrInvalidDescriptor) {
		t.Fatalf("invalid batch error = %v, want ErrInvalidDescriptor", err)
	}
	if _, ok := registry.Lookup("invalid"); ok {
		t.Fatal("failed batch partially registered a module")
	}
	if err := registry.RegisterBuiltin(ModuleDescriptor{ID: "invalid", Version: "1.0.0", Dependencies: []string{"missing"}}); err != nil {
		t.Fatal(err)
	}
	if err := registry.ValidateManifest(NewManifest("invalid")); !errors.Is(err, ErrUnknownModule) {
		t.Fatalf("unknown dependency error = %v, want ErrUnknownModule", err)
	}

	registry.RegisterBuiltin(ModuleDescriptor{ID: "second", Version: "1.0.0"})
	manifest := registry.Manifest()
	if !reflect.DeepEqual(manifest.Modules, []string{"invalid", "second", "stable"}) {
		t.Fatalf("manifest modules = %v", manifest.Modules)
	}
	descriptors := registry.Descriptors()
	descriptors[0].ID = "mutated"
	if got, ok := registry.Lookup("invalid"); !ok || got.ID != "invalid" {
		t.Fatalf("descriptor slice was not defensive: %+v, found=%t", got, ok)
	}
}

func TestBuiltinManifestSelectsCompiledModulesAndValidatesDependencies(t *testing.T) {
	registry, err := BuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	manifest := DefaultManifest()
	if err := registry.ValidateManifest(manifest); err != nil {
		t.Fatalf("default manifest rejected: %v", err)
	}
	if !manifest.Enabled(ModuleMCP) || manifest.Enabled("missing") {
		t.Fatalf("unexpected module selection: %+v", manifest.Modules)
	}
	ordered, err := registry.OrderIDs(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(ordered) != len(manifest.Modules) || ordered[0] != ModulePlatform {
		t.Fatalf("unexpected default module order: %v", ordered)
	}
}

func TestParseManifestRejectsPartialGraphAndMixedAllSelection(t *testing.T) {
	if manifest, err := ParseManifest("all"); err != nil || len(manifest.Modules) != 8 {
		t.Fatalf("all manifest = %+v, err = %v", manifest, err)
	}
	if _, err := ParseManifest("assistant"); !errors.Is(err, ErrMissingDependency) {
		t.Fatalf("partial manifest error = %v, want ErrMissingDependency", err)
	}
	if _, err := ParseManifest("all,work"); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("mixed all manifest error = %v, want ErrInvalidManifest", err)
	}
	if _, err := ParseManifest("unknown"); !errors.Is(err, ErrUnknownModule) {
		t.Fatalf("unknown manifest error = %v, want ErrUnknownModule", err)
	}
}
