package connectors

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type connectorTestEmbedder struct {
	vector []float32
	err    error
}

func (e *connectorTestEmbedder) Embed(context.Context, string) ([]float32, error) {
	return append([]float32(nil), e.vector...), e.err
}

func (e *connectorTestEmbedder) EmbedDocument(context.Context, string) ([]float32, error) {
	return append([]float32(nil), e.vector...), e.err
}

type connectorQueryOnlyEmbedder struct{}

func (connectorQueryOnlyEmbedder) Embed(context.Context, string) ([]float32, error) {
	return make([]float32, knowledge.EmbeddingDimensions), nil
}

func TestPostgresConnectorConstructorsAndPureHelpers(t *testing.T) {
	pool := &pgxpool.Pool{}
	embedder := &connectorTestEmbedder{vector: make([]float32, knowledge.EmbeddingDimensions)}

	drive, err := NewPostgresDriveRevisionStoreWithEmbedder(pool, "workspace-a", embedder)
	if err != nil {
		t.Fatalf("drive constructor error = %v", err)
	}
	if drive.Workspace != "workspace-a" || drive.Provider != ProviderGoogleDrive || drive.Embedder != embedder {
		t.Fatalf("drive store = %+v, want normalized workspace/provider/embedder", drive)
	}

	github, err := NewPostgresGitHubRevisionStoreWithEmbedder(pool, "workspace-a", embedder)
	if err != nil {
		t.Fatalf("GitHub constructor error = %v", err)
	}
	if github.PostgresRevisionStore.Provider != ProviderGitHub || github.Embedder != embedder {
		t.Fatalf("GitHub store = %+v, want provider/embedder", github)
	}

	if _, err := NewPostgresDriveRevisionStore(nil, "workspace-a"); err == nil {
		t.Fatal("nil Drive pool was accepted")
	}
	if _, err := NewPostgresGitHubRevisionStore(pool, " "); !errors.Is(err, knowledge.ErrInvalidInput) {
		t.Fatalf("invalid GitHub workspace error = %v, want knowledge validation", err)
	}
	if _, err := NewPostgresDriveRevisionStoreWithEmbedder(pool, "workspace-a", nil); err != nil {
		t.Fatalf("nil optional embedder rejected: %v", err)
	}

	if got := drive.sourceURI(); got != "drive://workspace" {
		t.Fatalf("Drive source URI = %q", got)
	}
	github.PostgresRevisionStore.Provider = ProviderGitHub
	if got := github.sourceURI(); got != "github://workspace" {
		t.Fatalf("GitHub source URI = %q", got)
	}
	if got := drive.scopedConnectorID("item", "file-1"); got != drive.scopedConnectorID("item", "file-1") || got == "" {
		t.Fatalf("scoped connector ID is not stable: %q", got)
	}
	if err := drive.validate(); err != nil {
		t.Fatalf("valid store rejected: %v", err)
	}
	if err := (&PostgresRevisionStore{Pool: pool}).validate(); !errors.Is(err, knowledge.ErrInvalidInput) {
		t.Fatalf("invalid store workspace error = %v", err)
	}
	var nilStore *PostgresRevisionStore
	if err := nilStore.validate(); err == nil {
		t.Fatal("nil store validated successfully")
	}

	if got := drive.embedDocument(context.Background(), "source"); len(got) != knowledge.EmbeddingDimensions {
		t.Fatalf("document embedding length = %d, want %d", len(got), knowledge.EmbeddingDimensions)
	}
	embedder.vector = make([]float32, knowledge.EmbeddingDimensions-1)
	if got := drive.embedDocument(context.Background(), "source"); got != nil {
		t.Fatalf("wrong-sized document embedding = %d values, want nil", len(got))
	}
	drive.Embedder = connectorQueryOnlyEmbedder{}
	if got := drive.embedDocument(context.Background(), "source"); got != nil {
		t.Fatalf("query-only embedder produced document vector: %d values", len(got))
	}
	drive.Embedder = &connectorTestEmbedder{vector: make([]float32, knowledge.EmbeddingDimensions), err: errors.New("sidecar unavailable")}
	if got := drive.embedDocument(context.Background(), "source"); got != nil {
		t.Fatalf("failed document embedding = %d values, want nil", len(got))
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if got := drive.embedDocument(canceled, "source"); got != nil {
		t.Fatalf("canceled document embedding = %d values, want nil", len(got))
	}

	if got := connectorVectorValue(nil); got != nil {
		t.Fatalf("nil vector = %v, want nil", got)
	}
	if got := connectorVectorValue([]float32{1, 0.5, -2}); got != "[1,0.5,-2]" {
		t.Fatalf("vector literal = %v", got)
	}
}

func TestConnectorDatabaseErrorsAndProviderClassificationCoverAllBoundaries(t *testing.T) {
	if mapConnectorDBError(nil) != nil {
		t.Fatal("nil connector database error was not preserved")
	}
	if !errors.Is(mapConnectorDBError(pgx.ErrNoRows), ErrNotFound) {
		t.Fatal("missing connector row was not mapped to ErrNotFound")
	}
	for _, testCase := range []struct {
		code string
		want error
	}{
		{code: "23505", want: ErrRevisionAlreadyExists},
		{code: "23503", want: ErrNotFound},
		{code: "23514", want: ErrInvalidRevisionIdentity},
	} {
		if !errors.Is(mapConnectorDBError(&pgconn.PgError{Code: testCase.code}), testCase.want) {
			t.Fatalf("PostgreSQL code %s was not mapped to %v", testCase.code, testCase.want)
		}
	}
	if got := mapConnectorDBError(errors.New("driver leaked details")); got == nil || got.Error() != "connector persistence failed" {
		t.Fatalf("unknown connector error = %v, want safe persistence error", got)
	}

	statuses := map[int]RetryClass{
		http.StatusRequestTimeout:  RetryTransient,
		http.StatusTooEarly:        RetryTransient,
		http.StatusTooManyRequests: RetryRateLimited,
		http.StatusUnauthorized:    RetryAuthentication,
		http.StatusForbidden:       RetryAuthorization,
		http.StatusNotFound:        RetryNotFound,
		http.StatusConflict:        RetryConflict,
		http.StatusBadGateway:      RetryTransient,
		http.StatusBadRequest:      RetryInvalidRequest,
		0:                          RetryNone,
	}
	for status, want := range statuses {
		if got := ClassifyHTTPStatus(status); got != want {
			t.Fatalf("HTTP status %d = %q, want %q", status, got, want)
		}
	}
	if RetryNone.Retryable() || !RetryTransient.Retryable() || !RetryRateLimited.Retryable() {
		t.Fatal("retryability classification is incorrect")
	}
	if got := ClassifyError(errors.New("ordinary error")); got != RetryNone {
		t.Fatalf("ordinary error class = %q", got)
	}
	if got := ClassifyError(context.DeadlineExceeded); got != RetryCanceled {
		t.Fatalf("deadline class = %q", got)
	}
	provider := NewProviderError("", "", 0, "token=secret")
	if provider.Error() != "provider: token=[REDACTED]" || provider.RetryClass() != RetryNone || provider.Unwrap() != nil {
		t.Fatalf("provider boundary = %q/%q/%v", provider.Error(), provider.RetryClass(), provider.Unwrap())
	}
	var nilProvider *ProviderError
	if nilProvider.Error() != "provider error" || nilProvider.Unwrap() != nil || nilProvider.RetryClass() != RetryNone {
		t.Fatal("nil provider error boundary is unstable")
	}
	if RedactSecrets("authorization: Bearer abc access_token=xyz github_pat_123") == "" {
		t.Fatal("secret redactor returned empty output")
	}
}

func TestPostgresSafeWriteConstructorsRejectUnconfiguredStores(t *testing.T) {
	if _, err := NewPostgresSafeWriteChallengeStore(nil); err == nil {
		t.Fatal("nil challenge pool was accepted")
	}
	if _, err := NewPostgresSafeWriteReceiptStore(nil); err == nil {
		t.Fatal("nil receipt pool was accepted")
	}
	challengeStore := &PostgresSafeWriteChallengeStore{}
	if err := challengeStore.Put(SafeWriteChallenge{}); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("unconfigured challenge store Put = %v", err)
	}
	if _, err := challengeStore.Get(context.Background(), "", "", ""); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("invalid challenge store Get = %v", err)
	}
	receiptStore := &PostgresSafeWriteReceiptStore{}
	if _, _, err := receiptStore.Lookup(context.Background(), "", "", "", ""); !errors.Is(err, ErrReceiptPersistence) {
		t.Fatalf("unconfigured receipt Lookup = %v", err)
	}
	if err := receiptStore.Save(context.Background(), SafeWriteReceipt{}); !errors.Is(err, ErrReceiptPersistence) {
		t.Fatalf("unconfigured receipt Save = %v", err)
	}
	if _, _, err := receiptStore.Reserve(context.Background(), SafeWriteReceipt{}); !errors.Is(err, ErrReceiptPersistence) {
		t.Fatalf("unconfigured receipt Reserve = %v", err)
	}
	if err := receiptStore.Complete(context.Background(), SafeWriteReceipt{}); !errors.Is(err, ErrReceiptPersistence) {
		t.Fatalf("unconfigured receipt Complete = %v", err)
	}
}

func TestMemorySafeWriteReceiptSaveIsValidatedScopedAndIdempotent(t *testing.T) {
	store := NewMemorySafeWriteReceiptStore()
	receipt := SafeWriteReceipt{
		ID:             "receipt-1",
		WorkspaceID:    "workspace-a",
		UserID:         "user-a",
		Operation:      SafeWriteOperationCreateIssue,
		IdempotencyKey: "request-1",
		ActionHash:     "hash-1",
		Labels:         []GitHubLabel{{Name: "bug"}},
	}
	if err := store.Save(context.Background(), receipt); err != nil {
		t.Fatalf("valid receipt Save = %v", err)
	}
	if err := store.Save(context.Background(), receipt); err != nil {
		t.Fatalf("idempotent receipt Save = %v", err)
	}
	found, ok, err := store.Lookup(context.Background(), receipt.WorkspaceID, receipt.UserID, receipt.Operation, receipt.IdempotencyKey)
	if err != nil || !ok || found.ActionHash != receipt.ActionHash || len(found.Labels) != 1 {
		t.Fatalf("saved receipt lookup = %+v, found=%v, err=%v", found, ok, err)
	}
	found.Labels[0].Name = "mutated"
	cloned, _, err := store.Lookup(context.Background(), receipt.WorkspaceID, receipt.UserID, receipt.Operation, receipt.IdempotencyKey)
	if err != nil || cloned.Labels[0].Name != "bug" {
		t.Fatalf("receipt lookup exposed mutable labels: %+v, err=%v", cloned, err)
	}

	conflict := receipt
	conflict.ActionHash = "hash-2"
	if err := store.Save(context.Background(), conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting receipt Save = %v", err)
	}
	for _, invalid := range []SafeWriteReceipt{
		{},
		{UserID: "user-a", IdempotencyKey: "request-1", ActionHash: "hash-1"},
		{ID: "receipt-1", IdempotencyKey: "request-1", ActionHash: "hash-1"},
		{ID: "receipt-1", UserID: "user-a", IdempotencyKey: "request-1"},
	} {
		if err := store.Save(context.Background(), invalid); !errors.Is(err, ErrReceiptPersistence) {
			t.Fatalf("invalid receipt Save = %v, want ErrReceiptPersistence", err)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Save(canceled, receipt); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled receipt Save = %v, want context.Canceled", err)
	}
	store.SaveError = errors.New("storage unavailable")
	newReceipt := receipt
	newReceipt.ID = "receipt-2"
	newReceipt.IdempotencyKey = "request-2"
	if err := store.Save(context.Background(), newReceipt); err == nil || err.Error() != "storage unavailable" {
		t.Fatalf("configured receipt Save error = %v", err)
	}
}
