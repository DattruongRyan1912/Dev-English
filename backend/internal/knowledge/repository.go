package knowledge

import "context"

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
