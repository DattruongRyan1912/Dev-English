package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type contractDriveReader struct {
	pages []DrivePage
	index int
}

func (f *contractDriveReader) List(ctx context.Context, request DriveListRequest) (DrivePage, error) {
	if err := ctx.Err(); err != nil {
		return DrivePage{}, err
	}
	if request.Cursor.Valid() && request.Cursor.Token != "page-2" {
		return DrivePage{}, ErrInvalidCursor
	}
	if f.index >= len(f.pages) {
		return DrivePage{}, nil
	}
	page := f.pages[f.index]
	f.index++
	return page, nil
}

func TestDriveRevisionIdentityAndCursorAreStable(t *testing.T) {
	item := DriveSourceItem{FileID: "file-1", RevisionID: "rev-1"}
	if err := item.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := item.RevisionKey(); got != "google_drive:file-1:rev-1" {
		t.Fatalf("revision key = %q", got)
	}
	if err := (DriveSourceItem{FileID: "file-1"}).Validate(); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("invalid identity error = %v", err)
	}
	unique, skipped, err := DeduplicateDriveItems([]DriveSourceItem{
		item,
		{FileID: " file-1 ", RevisionID: " rev-1 "},
	})
	if err != nil || len(unique) != 1 || skipped != 1 {
		t.Fatalf("dedupe = items:%d skipped:%d err:%v", len(unique), skipped, err)
	}
	if _, _, err := DeduplicateDriveItems([]DriveSourceItem{{FileID: "file-1"}}); !errors.Is(err, ErrInvalidRevisionIdentity) {
		t.Fatalf("invalid dedupe error = %v", err)
	}

	reader := &contractDriveReader{pages: []DrivePage{{Items: []DriveSourceItem{item}, NextCursor: DriveCursor{Token: "page-2"}, HasMore: true}, {HasMore: false}}}
	first, err := reader.List(context.Background(), DriveListRequest{})
	if err != nil || len(first.Items) != 1 || !first.HasMore {
		t.Fatalf("first page = %+v, err=%v", first, err)
	}
	second, err := reader.List(context.Background(), DriveListRequest{Cursor: first.NextCursor})
	if err != nil || second.HasMore {
		t.Fatalf("second page = %+v, err=%v", second, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.List(canceled, DriveListRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled list error = %v", err)
	}
	if (DriveCursor{}).Valid() || !(DriveCursor{Token: " token "}).Valid() {
		t.Fatal("cursor validity mismatch")
	}
}

func TestSafeGitHubWritesRequireIdempotency(t *testing.T) {
	request := CreateIssueRequest{Repository: "owner/repo", Title: "Issue"}
	if err := request.Metadata.Validate(); !errors.Is(err, ErrMissingIdempotencyKey) {
		t.Fatalf("metadata error = %v", err)
	}
	request.Metadata.IdempotencyKey = " request-1 "
	request.Metadata.WorkspaceID = "workspace-1"
	if err := request.Metadata.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (SafeWriteMetadata{IdempotencyKey: "request-1"}).Validate(); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("missing workspace error = %v", err)
	}
	if err := (SafeWriteMetadata{IdempotencyKey: "  "}).Validate(); !errors.Is(err, ErrMissingIdempotencyKey) {
		t.Fatalf("blank metadata error = %v", err)
	}
}

func TestRetryClassificationAndSecretRedaction(t *testing.T) {
	for status, want := range map[int]RetryClass{
		http.StatusTooManyRequests: RetryRateLimited,
		http.StatusBadGateway:      RetryTransient,
		http.StatusUnauthorized:    RetryAuthentication,
		http.StatusConflict:        RetryConflict,
		http.StatusBadRequest:      RetryInvalidRequest,
		http.StatusNotFound:        RetryNotFound,
		http.StatusForbidden:       RetryAuthorization,
		http.StatusOK:              RetryNone,
	} {
		if got := ClassifyHTTPStatus(status); got != want {
			t.Fatalf("status %d = %q, want %q", status, got, want)
		}
		if got := ClassifyHTTPRetry(status); got != want {
			t.Fatalf("legacy status %d = %q, want %q", status, got, want)
		}
	}
	if !RetryTransient.IsRetryable() || !RetryRateLimited.Retryable() || RetryConflict.IsRetryable() {
		t.Fatal("retry class helpers mismatch")
	}
	if !IsRetryable(NewProviderError("github", "list", http.StatusBadGateway, "temporary")) {
		t.Fatal("502 should be retryable")
	}
	if got := ClassifyError(context.Canceled); got != RetryCanceled || ClassifyRetry(nil) != RetryNone {
		t.Fatalf("cancellation class = %q", got)
	}
	message := NewProviderError("github", "request", http.StatusUnauthorized, "authorization: Bearer fake-header-secret api_key=fake-api-secret")
	formatted := message.Error()
	if strings.Contains(formatted, "fake-header-secret") || strings.Contains(formatted, "fake-api-secret") {
		t.Fatalf("provider error leaked secret: %q", formatted)
	}
}

func TestProviderErrorRedactsStructuredAndWrappedOutput(t *testing.T) {
	const (
		jsonSecret   = "fake-json-secret"
		headerSecret = "fake-header-secret"
		querySecret  = "fake-query-secret"
		keySecret    = "fake-key-secret"
		pemSecret    = "fake-pem-secret"
	)
	detail := strings.Join([]string{
		`{"access_token": "` + jsonSecret + `", "refreshToken": "refresh-secret"}`,
		"Authorization: Bearer " + headerSecret,
		"?api_key=" + querySecret,
		"private-key=" + keySecret,
		"-----BEGIN PRIVATE KEY-----\n" + pemSecret + "\n-----END PRIVATE KEY-----",
	}, " ")
	err := NewProviderError("github", "request", http.StatusBadGateway, detail)
	for _, output := range []string{err.Error(), fmt.Sprintf("%+v", err)} {
		for _, secret := range []string{jsonSecret, "refresh-secret", headerSecret, querySecret, keySecret, pemSecret} {
			if strings.Contains(output, secret) {
				t.Fatalf("formatted provider error leaked %q: %q", secret, output)
			}
		}
	}
	encoded, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, secret := range []string{jsonSecret, headerSecret, querySecret, keySecret, pemSecret} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("JSON provider error leaked %q: %s", secret, encoded)
		}
	}
	if !strings.Contains(string(encoded), `"provider":"github"`) {
		t.Fatalf("safe JSON envelope missing provider: %s", encoded)
	}
	if got := RedactSecrets(`"api_key":"fake-json-secret" token=fake-token`); strings.Contains(got, "fake-json-secret") || strings.Contains(got, "fake-token") {
		t.Fatalf("direct redaction leaked secret: %q", got)
	}

	sentinel := errors.New("provider body access_token=fake-wrapped-secret")
	wrapped := WrapProviderError("github", "read", http.StatusBadGateway, sentinel)
	if !errors.Is(wrapped, sentinel) || ClassifyRetry(wrapped) != RetryTransient {
		t.Fatalf("wrapped provider error lost matching/class: %v", wrapped)
	}
	redacted := RedactProviderError(fmt.Errorf("outer: %w", wrapped))
	if !errors.Is(redacted, sentinel) || strings.Contains(redacted.Error(), "fake-wrapped-secret") {
		t.Fatalf("redacted wrapped error = %v", redacted)
	}
	plain := RedactProviderError(errors.New("plain token=fake-plain-secret"))
	var redactedPlain *redactedError
	if strings.Contains(plain.Error(), "fake-plain-secret") || !errors.As(plain, &redactedPlain) {
		t.Fatal("plain redacted error mismatch")
	}
	if RedactProviderError(nil) != nil {
		t.Fatal("nil redacted error should be nil")
	}
}

func TestProviderErrorStructLiteralAndNilAreSafe(t *testing.T) {
	var nilProvider *ProviderError
	if nilProvider.Error() != "provider error" || nilProvider.RetryClass() != RetryNone || nilProvider.Unwrap() != nil {
		t.Fatal("nil ProviderError behavior mismatch")
	}
	provider := &ProviderError{Provider: " provider ", Operation: " operation ", StatusCode: http.StatusServiceUnavailable, RequestID: "request-id", detail: "body token=fake-literal-secret"}
	if provider.RetryClass() != RetryTransient || !provider.Retryable() {
		t.Fatalf("struct literal class = %q", provider.RetryClass())
	}
	if strings.Contains(fmt.Sprintf("%+v", provider), "fake-literal-secret") {
		t.Fatal("struct formatting exposed private detail")
	}
	alias := NewHTTPProviderError("github", "read", http.StatusOK, "ok")
	if alias.Error() != "github read returned HTTP 200: ok" {
		t.Fatalf("HTTP alias = %q", alias)
	}
}
