package testkit

import (
	"errors"
	"testing"
	"time"
)

func TestConfigFromEnvIsOptInAndTrimmed(t *testing.T) {
	t.Setenv(DatabaseURLEnv, "  postgres://example.test/devenglish  ")

	config := ConfigFromEnv()
	if config.DatabaseURL != "postgres://example.test/devenglish" {
		t.Fatalf("database URL = %q, want trimmed opt-in URL", config.DatabaseURL)
	}
	if config.PingTimeout != 30*time.Second {
		t.Fatalf("ping timeout = %s, want 30s", config.PingTimeout)
	}
	if !config.Enabled() {
		t.Fatal("configured URL should enable integration tests")
	}
}

func TestOpenPostgresDoesNotProceedWithoutConfiguration(t *testing.T) {
	_, err := OpenPostgres(nil, Config{})
	if !errors.Is(err, ErrDatabaseURLNotConfigured) {
		t.Fatalf("error = %v, want ErrDatabaseURLNotConfigured", err)
	}
}

func TestOpenPostgresRejectsMalformedURLWithoutDialling(t *testing.T) {
	_, err := OpenPostgres(nil, Config{DatabaseURL: "not-a-database-url"})
	if !errors.Is(err, ErrInvalidDatabaseURL) {
		t.Fatalf("error = %v, want ErrInvalidDatabaseURL", err)
	}
}
