package main

import (
	"strings"
	"testing"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/auth"
)

func TestValidateRuntimeConfigRequiresProductionBoundary(t *testing.T) {
	t.Setenv("DEVENGLISH_BOOTSTRAP_KEY", "")
	manager := auth.New(strings.Repeat("s", 32))
	if err := validateRuntimeConfig("production", manager, "", []string{"https://app.example.com"}); err == nil {
		t.Fatal("production without DATABASE_URL must be rejected")
	}

	t.Setenv("DEVENGLISH_BOOTSTRAP_KEY", "bootstrap")
	if err := validateRuntimeConfig("production", manager, "postgres://db", nil); err == nil {
		t.Fatal("production without an origin allowlist must be rejected")
	}
	if err := validateRuntimeConfig("production", manager, "postgres://db", []string{"*"}); err == nil {
		t.Fatal("production wildcard CORS must be rejected")
	}
	if err := validateRuntimeConfig("production", manager, "postgres://db", []string{"https://app.example.com"}); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}
}

func TestValidateRuntimeConfigKeepsDevelopmentFlexible(t *testing.T) {
	t.Setenv("DEVENGLISH_BOOTSTRAP_KEY", "")
	if err := validateRuntimeConfig("development", auth.New(""), "", nil); err != nil {
		t.Fatalf("development config rejected: %v", err)
	}
}
