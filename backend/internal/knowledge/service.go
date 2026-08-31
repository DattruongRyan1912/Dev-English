package knowledge

import (
	"context"
	"strings"
)

// Service applies deterministic domain validation before delegating persistence
// to a workspace-scoped Repository. It contains no provider or transport code.
type Service struct {
	repository Repository
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, ErrNilRepository
	}
	return &Service{repository: repository}, nil
}

func (s *Service) CreateSource(ctx context.Context, scope WorkspaceScope, source KnowledgeSource) error {
	if err := validateScopeAndEntity(scope, source.WorkspaceID, source.Validate); err != nil {
		return err
	}
	return s.repository.CreateSource(ctx, scope, source)
}

// GetSource exposes a validated, workspace-scoped source read. Keeping these
// reads on the service prevents REST, MCP and connector code from reaching
// into a repository and accidentally skipping the workspace boundary.
func (s *Service) GetSource(ctx context.Context, scope WorkspaceScope, id string) (KnowledgeSource, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return KnowledgeSource{}, err
	}
	return s.repository.GetSource(ctx, scope, id)
}

func (s *Service) ListSources(ctx context.Context, scope WorkspaceScope, limit int) ([]KnowledgeSource, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	lister, ok := s.repository.(SourceLister)
	if !ok {
		return nil, ErrUnsupportedRead
	}
	return lister.ListSources(ctx, scope, limit)
}

// GetSourceDetail composes the canonical source timeline without exposing the
// repository to transport adapters. Every child lookup is constrained by the
// same workspace and source relationship in the concrete adapter.
func (s *Service) GetSourceDetail(ctx context.Context, scope WorkspaceScope, id string) (SourceDetail, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return SourceDetail{}, err
	}
	reader, ok := s.repository.(SourceDetailReader)
	if !ok {
		return SourceDetail{}, ErrUnsupportedRead
	}
	source, err := s.repository.GetSource(ctx, scope, id)
	if err != nil {
		return SourceDetail{}, err
	}
	items, err := reader.ListSourceItems(ctx, scope, id, 100)
	if err != nil {
		return SourceDetail{}, err
	}
	detail := SourceDetail{Source: source, Items: items, Revisions: make([]SourceRevision, 0), Chunks: make([]KnowledgeChunk, 0), Evidence: make([]ClaimEvidence, 0)}
	for _, item := range items {
		revisions, err := reader.ListRevisions(ctx, scope, item.ID, 50)
		if err != nil {
			return SourceDetail{}, err
		}
		detail.Revisions = append(detail.Revisions, revisions...)
		for _, revision := range revisions {
			chunks, err := reader.ListChunks(ctx, scope, revision.ID, 100)
			if err != nil {
				return SourceDetail{}, err
			}
			detail.Chunks = append(detail.Chunks, chunks...)
		}
	}
	detail.Evidence, err = reader.ListEvidenceForSource(ctx, scope, id, 200)
	if err != nil {
		return SourceDetail{}, err
	}
	return detail, nil
}

// Search keeps retrieval behind the Knowledge service so REST, assistant and
// MCP use the same workspace validation and degraded-mode behavior.
func (s *Service) Search(ctx context.Context, scope WorkspaceScope, query string, limit int) ([]SearchResult, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	searcher, ok := s.repository.(Searcher)
	if !ok {
		return nil, ErrUnsupportedRead
	}
	if strings.TrimSpace(query) == "" {
		return []SearchResult{}, nil
	}
	return searcher.Search(ctx, scope, query, limit)
}

func (s *Service) CreateSourceItem(ctx context.Context, scope WorkspaceScope, item SourceItem) error {
	if err := validateScopeAndEntity(scope, item.WorkspaceID, item.Validate); err != nil {
		return err
	}
	return s.repository.CreateSourceItem(ctx, scope, item)
}

func (s *Service) GetSourceItem(ctx context.Context, scope WorkspaceScope, id string) (SourceItem, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return SourceItem{}, err
	}
	return s.repository.GetSourceItem(ctx, scope, id)
}

func (s *Service) AppendRevision(ctx context.Context, scope WorkspaceScope, revision SourceRevision) error {
	if err := validateScopeAndEntity(scope, revision.WorkspaceID, revision.Validate); err != nil {
		return err
	}
	return s.repository.CreateRevision(ctx, scope, revision)
}

func (s *Service) GetRevision(ctx context.Context, scope WorkspaceScope, id string) (SourceRevision, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return SourceRevision{}, err
	}
	return s.repository.GetRevision(ctx, scope, id)
}

func (s *Service) CreateChunk(ctx context.Context, scope WorkspaceScope, chunk KnowledgeChunk) error {
	if err := validateScopeAndEntity(scope, chunk.WorkspaceID, chunk.Validate); err != nil {
		return err
	}
	return s.repository.CreateChunk(ctx, scope, chunk)
}

// GetChunk exposes a validated, workspace-scoped read for transport adapters.
// The repository remains private to the service so callers cannot bypass the
// workspace boundary.
func (s *Service) GetChunk(ctx context.Context, scope WorkspaceScope, id string) (KnowledgeChunk, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return KnowledgeChunk{}, err
	}
	return s.repository.GetChunk(ctx, scope, id)
}

func (s *Service) CreateTopic(ctx context.Context, scope WorkspaceScope, topic Topic) error {
	if err := validateScopeAndEntity(scope, topic.WorkspaceID, topic.Validate); err != nil {
		return err
	}
	return s.repository.CreateTopic(ctx, scope, topic)
}

func (s *Service) CreateClaimBundle(ctx context.Context, scope WorkspaceScope, claim KnowledgeClaim, evidence []ClaimEvidence) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if claim.WorkspaceID != scope.ID {
		return ErrWorkspaceMismatch
	}
	if err := ValidateClaimBundle(claim, evidence); err != nil {
		return err
	}
	return s.repository.CreateClaimBundle(ctx, scope, claim, evidence)
}

// GetClaim exposes a validated, workspace-scoped read for transport adapters.
func (s *Service) GetClaim(ctx context.Context, scope WorkspaceScope, id string) (KnowledgeClaim, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return KnowledgeClaim{}, err
	}
	return s.repository.GetClaim(ctx, scope, id)
}

func (s *Service) ListClaimEvidence(ctx context.Context, scope WorkspaceScope, claimID string) ([]ClaimEvidence, error) {
	if err := validateScopeAndID(scope, claimID); err != nil {
		return nil, err
	}
	return s.repository.ListClaimEvidence(ctx, scope, claimID)
}

type validateFunc func() error

func validateScopeAndEntity(scope WorkspaceScope, workspaceID string, validate validateFunc) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if workspaceID != scope.ID {
		return ErrWorkspaceMismatch
	}
	return validate()
}

func validateScopeAndID(scope WorkspaceScope, id string) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	return requireIdentifier("id", id)
}
