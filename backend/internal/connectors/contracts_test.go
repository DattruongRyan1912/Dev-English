package connectors

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

type fakeDriveReader struct {
	pages []DrivePage
	index int
}

func (f *fakeDriveReader) List(ctx context.Context, request DriveListRequest) (DrivePage, error) {
	select {
	case <-ctx.Done():
		return DrivePage{}, ctx.Err()
	default:
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

	reader := &fakeDriveReader{pages: []DrivePage{{Items: []DriveSourceItem{item}, NextCursor: DriveCursor{Token: "page-2"}, HasMore: true}, {HasMore: false}}}
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
}

func TestSafeGitHubWritesRequireIdempotency(t *testing.T) {
	request := CreateIssueRequest{Repository: "owner/repo", Title: "Issue"}
	if err := request.Metadata.Validate(); !errors.Is(err, ErrMissingIdempotencyKey) {
		t.Fatalf("metadata error = %v", err)
	}
	request.Metadata.IdempotencyKey = "request-1"
	if err := request.Metadata.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRetryClassificationAndSecretRedaction(t *testing.T) {
	for status, want := range map[int]RetryClass{
		http.StatusTooManyRequests: RetryRateLimited,
		http.StatusBadGateway:      RetryTransient,
		http.StatusUnauthorized:    RetryAuthentication,
		http.StatusConflict:        RetryConflict,
	} {
		if got := ClassifyHTTPStatus(status); got != want {
			t.Fatalf("status %d = %q, want %q", status, got, want)
		}
	}
	if !IsRetryable(NewProviderError("github", "list", http.StatusBadGateway, "temporary")) {
		t.Fatal("502 should be retryable")
	}
	if got := ClassifyError(context.Canceled); got != RetryCanceled {
		t.Fatalf("canceled class = %q", got)
	}
	message := NewProviderError("github", "request", http.StatusUnauthorized, "authorization: Bearer super-secret api_key=abc123")
	formatted := message.Error()
	if strings.Contains(formatted, "super-secret") || strings.Contains(formatted, "abc123") {
		t.Fatalf("provider error leaked secret: %q", formatted)
	}
}
