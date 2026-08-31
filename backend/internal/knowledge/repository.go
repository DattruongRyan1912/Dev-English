package knowledge

import (
	"context"
	"time"
)

const manualImportOperationName = "knowledge.import_manual"

// SourceRepository persists mutable source metadata within one workspace.
type SourceRepository interface {
	CreateSource(context.Context, WorkspaceScope, KnowledgeSource) error
	GetSource(context.Context, WorkspaceScope, string) (KnowledgeSource, error)
}

type SourceItemRepository interface {
	CreateSourceItem(context.Context, WorkspaceScope, SourceItem) error
	GetSourceItem(context.Context, WorkspaceScope, string) (SourceItem, error)
}

// RevisionRepository is append-only by contract. Adapters must reject any
// attempt to update or delete a SourceRevision and should use RevisionKey for
// deterministic deduplication.
type RevisionRepository interface {
	CreateRevision(context.Context, WorkspaceScope, SourceRevision) error
	GetRevision(context.Context, WorkspaceScope, string) (SourceRevision, error)
}

type ChunkRepository interface {
	CreateChunk(context.Context, WorkspaceScope, KnowledgeChunk) error
	GetChunk(context.Context, WorkspaceScope, string) (KnowledgeChunk, error)
}

type TopicRepository interface {
	CreateTopic(context.Context, WorkspaceScope, Topic) error
	GetTopic(context.Context, WorkspaceScope, string) (Topic, error)
}

type ClaimRepository interface {
	CreateClaimBundle(context.Context, WorkspaceScope, KnowledgeClaim, []ClaimEvidence) error
	GetClaim(context.Context, WorkspaceScope, string) (KnowledgeClaim, error)
	ListClaimEvidence(context.Context, WorkspaceScope, string) ([]ClaimEvidence, error)
}

// Repository is the provider-neutral persistence boundary for the Knowledge
// bounded context. Implementations may be PostgreSQL, memory, or a test fake;
// no method can be called without an explicit workspace scope.
type Repository interface {
	SourceRepository
	SourceItemRepository
	RevisionRepository
	ChunkRepository
	TopicRepository
	ClaimRepository
}

// SearchResult is the resolved, current source context returned to the
// assistant retrieval boundary. It intentionally carries the revision and
// chunk together so a citation can never point at a source without the exact
// immutable content that was searched.
type SearchResult struct {
	Source   KnowledgeSource
	Item     SourceItem
	Revision SourceRevision
	Chunk    KnowledgeChunk
}

// Searcher is an optional read capability kept separate from Repository so
// existing narrow fakes and mutation contracts do not grow accidentally.
type Searcher interface {
	Search(context.Context, WorkspaceScope, string, int) ([]SearchResult, error)
}

// EmbeddingProvider is the narrow boundary to the local multilingual-e5-small
// sidecar. A provider must return exactly EmbeddingDimensions values; callers
// never persist a partial or provider-specific vector.
type EmbeddingProvider interface {
	Embed(context.Context, string) ([]float32, error)
}

// DocumentEmbeddingProvider is the optional passage-side capability used
// during ingestion. multilingual-e5-small expects query and passage prefixes
// to be different; keeping this as a separate capability lets older fakes and
// providers remain query-compatible while the local sidecar can produce
// vectors that are comparable during hybrid retrieval.
type DocumentEmbeddingProvider interface {
	EmbeddingProvider
	EmbedDocument(context.Context, string) ([]float32, error)
}

// HybridSearcher is optional so memory repositories and small test fixtures
// can keep lexical search while PostgreSQL enables the vector/RRF path.
type HybridSearcher interface {
	SearchHybrid(context.Context, WorkspaceScope, string, []float32, int) ([]SearchResult, error)
}

// CurrentRevisionSetter is used by the manual-ingestion application service
// after an append-only revision is created. It is deliberately not part of
// RevisionRepository: revisions themselves remain immutable.
type CurrentRevisionSetter interface {
	SetCurrentRevision(context.Context, WorkspaceScope, string, string) error
}

// SourceLister is the bounded read capability used by the S1 bootstrap. It
// stays optional so the original narrow mutation/read contracts remain stable.
type SourceLister interface {
	ListSources(context.Context, WorkspaceScope, int) ([]KnowledgeSource, error)
}

// SourceDetailReader is optional so narrow test repositories do not have to
// implement an inspection surface they do not use. Production adapters expose
// one bounded, workspace-scoped source timeline through this interface.
type SourceDetailReader interface {
	ListSourceItems(context.Context, WorkspaceScope, string, int) ([]SourceItem, error)
	ListRevisions(context.Context, WorkspaceScope, string, int) ([]SourceRevision, error)
	ListChunks(context.Context, WorkspaceScope, string, int) ([]KnowledgeChunk, error)
	ListEvidenceForSource(context.Context, WorkspaceScope, string, int) ([]ClaimEvidence, error)
}

// ManualImportIdempotencyRecord is the durable application-boundary replay
// record for manual knowledge imports. ResponseJSON contains only the
// canonical import projection, never provider credentials or source content.
type ManualImportIdempotencyRecord struct {
	WorkspaceID    string
	UserID         string
	Operation      string
	IdempotencyKey string
	RequestHash    string
	ResponseJSON   []byte
}

// ManualImportIdempotencyRepository is optional so narrow knowledge fakes can
// keep their original contract. Durable production adapters implement it;
// application code uses it only when a transport supplies an idempotency key.
type ManualImportIdempotencyRepository interface {
	LookupManualImportIdempotency(context.Context, WorkspaceScope, string, string) (ManualImportIdempotencyRecord, bool, error)
	SaveManualImportIdempotency(context.Context, WorkspaceScope, ManualImportIdempotencyRecord) error
}

// ManualImportBundle is the complete immutable source graph created by a
// manual import. A production adapter must persist the graph and its
// idempotency record in one atomic boundary when a key is supplied.
type ManualImportBundle struct {
	Source   KnowledgeSource
	Item     SourceItem
	Revision SourceRevision
	Chunk    KnowledgeChunk
}

// ManualImportCommitRequest is the application-owned input to the atomic
// manual-import boundary. RequestHash is compared on replay; it is never
// derived from mutable database state.
type ManualImportCommitRequest struct {
	UserID         string
	IdempotencyKey string
	RequestHash    string
	Bundle         ManualImportBundle
}

// ManualImportSourceSummary is the stable response projection stored in the
// idempotency record. Its JSON shape intentionally matches the application
// ImportedSource projection without exposing source content.
type ManualImportSourceSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Kind      string    `json:"kind"`
	URI       string    `json:"uri"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ManualImportProjection struct {
	Source     ManualImportSourceSummary `json:"source"`
	RevisionID string                    `json:"revisionId"`
	ChunkID    string                    `json:"chunkId"`
}

// ManualImportCommit is returned by the atomic boundary. Replayed commits
// carry the durable record and must not execute the source graph again.
type ManualImportCommit struct {
	Projection ManualImportProjection
	Record     ManualImportIdempotencyRecord
	Replayed   bool
}

// ManualImportRepository is implemented by durable adapters that can reserve
// the idempotency key and commit the complete manual-import graph atomically.
// It is optional so narrow repositories used by compatibility tests retain
// their smaller contract.
type ManualImportRepository interface {
	CommitManualImport(context.Context, WorkspaceScope, ManualImportCommitRequest) (ManualImportCommit, error)
}

func validateManualImportCommitRequest(scope WorkspaceScope, request ManualImportCommitRequest) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	for field, value := range map[string]string{
		"userId": request.UserID, "idempotencyKey": request.IdempotencyKey,
		"requestHash": request.RequestHash,
	} {
		if err := requireIdentifier(field, value); err != nil {
			return err
		}
	}
	bundle := request.Bundle
	if err := validateScopeAndEntity(scope, bundle.Source.WorkspaceID, bundle.Source.Validate); err != nil {
		return err
	}
	if err := validateScopeAndEntity(scope, bundle.Item.WorkspaceID, bundle.Item.Validate); err != nil {
		return err
	}
	if bundle.Item.SourceID != bundle.Source.ID {
		return invalidField("sourceId", "must match the imported source")
	}
	if err := validateScopeAndEntity(scope, bundle.Revision.WorkspaceID, bundle.Revision.Validate); err != nil {
		return err
	}
	if bundle.Revision.SourceItemID != bundle.Item.ID {
		return invalidField("sourceItemId", "must match the imported source item")
	}
	if err := validateScopeAndEntity(scope, bundle.Chunk.WorkspaceID, bundle.Chunk.Validate); err != nil {
		return err
	}
	if bundle.Chunk.RevisionID != bundle.Revision.ID {
		return invalidField("revisionId", "must match the imported revision")
	}
	return nil
}

func manualImportProjection(source KnowledgeSource, revisionID, chunkID string) ManualImportProjection {
	return ManualImportProjection{
		Source: ManualImportSourceSummary{
			ID: source.ID, Title: source.Name, Kind: source.Kind,
			URI: source.URI, UpdatedAt: source.UpdatedAt.UTC(),
		},
		RevisionID: revisionID,
		ChunkID:    chunkID,
	}
}

func sameManualImportRevision(existing, expected SourceRevision) bool {
	return existing.ID == expected.ID &&
		existing.WorkspaceID == expected.WorkspaceID &&
		existing.SourceItemID == expected.SourceItemID &&
		existing.RevisionKey == expected.RevisionKey &&
		existing.ContentHash == expected.ContentHash &&
		existing.ContentType == expected.ContentType &&
		existing.SourceURI == expected.SourceURI &&
		existing.Content == expected.Content
}

func sameManualImportChunk(existing, expected KnowledgeChunk) bool {
	if existing.ID != expected.ID || existing.WorkspaceID != expected.WorkspaceID ||
		existing.RevisionID != expected.RevisionID || existing.Ordinal != expected.Ordinal ||
		existing.Text != expected.Text || existing.TokenCount != expected.TokenCount ||
		len(existing.Embedding) != len(expected.Embedding) {
		return false
	}
	for index := range existing.Embedding {
		if existing.Embedding[index] != expected.Embedding[index] {
			return false
		}
	}
	return true
}
