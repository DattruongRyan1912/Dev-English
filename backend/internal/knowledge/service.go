package knowledge

import "context"

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

func (s *Service) CreateSourceItem(ctx context.Context, scope WorkspaceScope, item SourceItem) error {
	if err := validateScopeAndEntity(scope, item.WorkspaceID, item.Validate); err != nil {
		return err
	}
	return s.repository.CreateSourceItem(ctx, scope, item)
}

func (s *Service) AppendRevision(ctx context.Context, scope WorkspaceScope, revision SourceRevision) error {
	if err := validateScopeAndEntity(scope, revision.WorkspaceID, revision.Validate); err != nil {
		return err
	}
	return s.repository.CreateRevision(ctx, scope, revision)
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
