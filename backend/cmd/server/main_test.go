package main

import (
	"strings"
	"testing"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/auth"
)

func TestValidateRuntimeConfigRequiresProductionBoundary(t *testing.T) {
	t.Setenv("DEVENGLISH_LOGIN_SECRET", "")
	manager := auth.New(strings.Repeat("s", 32))
	if err := validateRuntimeConfig("production", manager, "", []string{"https://app.example.com"}, ""); err == nil {
		t.Fatal("production without DATABASE_URL must be rejected")
	}

	t.Setenv("DEVENGLISH_LOGIN_SECRET", "production-login-secret")
	t.Setenv("DEVENGLISH_SECRET_ENCRYPTION_KEY", "production-secret-encryption-key-32")
	if err := validateRuntimeConfig("production", manager, "postgres://db", []string{"https://app.example.com"}, "short"); err == nil {
		t.Fatal("production with a short login secret must be rejected")
	}
	if err := validateRuntimeConfig("production", manager, "postgres://db", nil, "production-login-secret"); err == nil {
		t.Fatal("production without an origin allowlist must be rejected")
	}
	if err := validateRuntimeConfig("production", manager, "postgres://db", []string{"*"}, "production-login-secret"); err == nil {
		t.Fatal("production wildcard CORS must be rejected")
	}
	if err := validateRuntimeConfig("production", manager, "postgres://db", []string{"https://app.example.com"}, "production-login-secret"); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}
	t.Setenv("DEVENGLISH_SECRET_ENCRYPTION_KEY", "short")
	if err := validateRuntimeConfig("production", manager, "postgres://db", []string{"https://app.example.com"}, "production-login-secret"); err == nil {
		t.Fatal("production with a short secret-encryption key must be rejected")
	}
}

func TestValidateRuntimeConfigKeepsDevelopmentFlexible(t *testing.T) {
	t.Setenv("DEVENGLISH_LOGIN_SECRET", "")
	if err := validateRuntimeConfig("development", auth.New(""), "", nil, ""); err != nil {
		t.Fatalf("development config rejected: %v", err)
	}
}
