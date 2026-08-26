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
