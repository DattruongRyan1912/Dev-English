package connectors

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type connectorNetworkError struct{}

func (connectorNetworkError) Error() string   { return "connection reset" }
func (connectorNetworkError) Timeout() bool   { return true }
func (connectorNetworkError) Temporary() bool { return true }

func TestConnectorRetryAndSyncHelperBoundaries(t *testing.T) {
	if got := ClassifyError(connectorNetworkError{}); got != RetryTransient {
		t.Fatalf("network error class = %q, want transient", got)
	}
	provider := &ProviderError{StatusCode: http.StatusTooManyRequests}
	if got := provider.RetryClass(); got != RetryRateLimited {
		t.Fatalf("provider fallback class = %q, want rate_limited", got)
	}
	if got := ClassifyError(fmt.Errorf("wrapped: %w", provider)); got != RetryRateLimited {
		t.Fatalf("wrapped provider class = %q, want rate_limited", got)
	}

	for _, testCase := range []struct {
		name string
		err  error
		want string
	}{
		{name: "invalid cursor", err: ErrInvalidCursor, want: "invalid_cursor"},
		{name: "invalid sync state", err: ErrInvalidSyncState, want: "invalid_sync_state"},
		{name: "invalid revision", err: ErrInvalidRevisionIdentity, want: "invalid_revision"},
		{name: "duplicate revision", err: ErrRevisionAlreadyExists, want: "duplicate_revision"},
		{name: "other", err: errors.New("provider unavailable"), want: "sync_failed"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := syncRunErrorCode(testCase.err); got != testCase.want {
				t.Fatalf("sync error code = %q, want %q", got, testCase.want)
			}
		})
	}

	if got := normalizedPageSize(0); got != defaultConnectorPageSize {
		t.Fatalf("default page size = %d, want %d", got, defaultConnectorPageSize)
	}
	if got := normalizedPageSize(7); got != 7 {
		t.Fatalf("explicit page size = %d, want 7", got)
	}
	if got := normalizedCursor(DriveCursor{Token: "  next  "}); got.Token != "next" {
		t.Fatalf("normalized cursor = %+v", got)
	}

	if wrapConnectorFailure("github", "list", nil) != nil {
		t.Fatal("nil connector error was wrapped")
	}
	wrapped := wrapConnectorFailure("github", "list", errors.New("api_key=secret-value"))
	if wrapped == nil || !strings.Contains(wrapped.Error(), "[REDACTED]") || strings.Contains(wrapped.Error(), "secret-value") {
		t.Fatalf("wrapped connector error leaked detail: %v", wrapped)
	}
}
