package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

type Service struct {
	repo  Repository
	clock func() time.Time
}

type Option func(*Service)

func WithClock(clock func() time.Time) Option {
	return func(service *Service) {
		if clock != nil {
			service.clock = clock
		}
	}
}

func NewService(repo Repository, options ...Option) (*Service, error) {
	if repo == nil {
		return nil, &ValidationError{Field: "repository", Message: "is required"}
	}
	service := &Service{repo: repo, clock: func() time.Time { return time.Now().UTC() }}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service, nil
}

func (s *Service) CreateProject(ctx context.Context, scope Scope, input CreateProjectInput, idempotencyKey string) (Project, error) {
	if err := scope.Validate(); err != nil {
		return Project{}, err
	}
	input, err := validateProjectCreate(input)
	if err != nil {
		return Project{}, err
	}
	if err := ValidateIdempotencyKey(idempotencyKey); err != nil {
		return Project{}, err
	}
	if input.ID == "" {
		input.ID = stableID(EntityProject, scope, idempotencyKey)
	}
	now := s.now()
	project := Project{
		ID:          input.ID,
		WorkspaceID: scope.WorkspaceID,
		OwnerUserID: scope.UserID,
		Name:        input.Name,
		Description: input.Description,
		Status:      input.Status,
		Origin:      OriginCanonical,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	mutation, err := newMutation(OperationProjectCreate, idempotencyKey, input, now)
	if err != nil {
		return Project{}, err
	}
	return s.repo.CreateProject(ctx, scope, project, mutation)
}

func (s *Service) GetProject(ctx context.Context, scope Scope, id string) (Project, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return Project{}, err
	}
	return s.repo.GetProject(ctx, scope, id)
}

func (s *Service) ListProjects(ctx context.Context, scope Scope, options ListOptions) ([]Project, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	options, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	return s.repo.ListProjects(ctx, scope, options)
}

func (s *Service) UpdateProject(ctx context.Context, scope Scope, id string, patch ProjectPatch, expectedVersion int64, idempotencyKey string) (Project, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return Project{}, err
	}
	if err := validateVersion(expectedVersion); err != nil {
		return Project{}, err
	}
	if err := validateProjectPatch(patch); err != nil {
		return Project{}, err
	}
	mutation, err := s.mutation(OperationProjectUpdate, idempotencyKey, struct {
		ID              string       `json:"id"`
		Patch           ProjectPatch `json:"patch"`
		ExpectedVersion int64        `json:"expectedVersion"`
	}{id, patch, expectedVersion})
	if err != nil {
		return Project{}, err
	}
	return s.repo.UpdateProject(ctx, scope, id, patch, expectedVersion, mutation)
}

func (s *Service) TrashProject(ctx context.Context, scope Scope, id string, expectedVersion int64, idempotencyKey string) (Project, error) {
	return s.trashProject(ctx, scope, id, expectedVersion, idempotencyKey)
}

func (s *Service) RestoreProject(ctx context.Context, scope Scope, id string, expectedVersion int64, idempotencyKey string) (Project, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return Project{}, err
	}
	if err := validateVersion(expectedVersion); err != nil {
		return Project{}, err
	}
	mutation, err := s.mutation(OperationProjectRestore, idempotencyKey, versionRequest{id, expectedVersion})
	if err != nil {
		return Project{}, err
	}
	return s.repo.RestoreProject(ctx, scope, id, expectedVersion, mutation)
}

func (s *Service) PurgeProject(ctx context.Context, scope Scope, id string, expectedVersion int64, idempotencyKey string) (PurgeResult, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return PurgeResult{}, err
	}
	if err := validateVersion(expectedVersion); err != nil {
		return PurgeResult{}, err
	}
	mutation, err := s.mutation(OperationProjectPurge, idempotencyKey, versionRequest{id, expectedVersion})
	if err != nil {
		return PurgeResult{}, err
	}
	return s.repo.PurgeProject(ctx, scope, id, expectedVersion, mutation)
}

func (s *Service) CreateTask(ctx context.Context, scope Scope, input CreateTaskInput, idempotencyKey string) (Task, error) {
	if err := scope.Validate(); err != nil {
		return Task{}, err
	}
	input, err := validateTaskCreate(input)
	if err != nil {
		return Task{}, err
	}
	if err := ValidateIdempotencyKey(idempotencyKey); err != nil {
		return Task{}, err
	}
	if input.ID == "" {
		input.ID = stableID(EntityTask, scope, idempotencyKey)
	}
	now := s.now()
	task := Task{
		ID:          input.ID,
		WorkspaceID: scope.WorkspaceID,
		OwnerUserID: scope.UserID,
		ProjectID:   input.ProjectID,
		Title:       input.Title,
		Description: input.Description,
		Status:      input.Status,
		Priority:    input.Priority,
		DueAt:       cloneTimePtr(input.DueAt),
		Origin:      OriginCanonical,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	mutation, err := newMutation(OperationTaskCreate, idempotencyKey, input, now)
	if err != nil {
		return Task{}, err
	}
	return s.repo.CreateTask(ctx, scope, task, mutation)
}

func (s *Service) GetTask(ctx context.Context, scope Scope, id string) (Task, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return Task{}, err
	}
	return s.repo.GetTask(ctx, scope, id)
}

func (s *Service) ListTasks(ctx context.Context, scope Scope, projectID string, options ListOptions) ([]Task, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if projectID != "" {
		if err := validateEntityID("projectId", projectID); err != nil {
			return nil, err
		}
	}
	options, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	return s.repo.ListTasks(ctx, scope, projectID, options)
}

func (s *Service) UpdateTask(ctx context.Context, scope Scope, id string, patch TaskPatch, expectedVersion int64, idempotencyKey string) (Task, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return Task{}, err
	}
	if err := validateVersion(expectedVersion); err != nil {
		return Task{}, err
	}
	if err := validateTaskPatch(patch); err != nil {
		return Task{}, err
	}
	mutation, err := s.mutation(OperationTaskUpdate, idempotencyKey, struct {
		ID              string    `json:"id"`
		Patch           TaskPatch `json:"patch"`
		ExpectedVersion int64     `json:"expectedVersion"`
	}{id, patch, expectedVersion})
	if err != nil {
		return Task{}, err
	}
	return s.repo.UpdateTask(ctx, scope, id, patch, expectedVersion, mutation)
}

func (s *Service) TrashTask(ctx context.Context, scope Scope, id string, expectedVersion int64, idempotencyKey string) (Task, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return Task{}, err
	}
	if err := validateVersion(expectedVersion); err != nil {
		return Task{}, err
	}
	mutation, err := s.mutation(OperationTaskTrash, idempotencyKey, versionRequest{id, expectedVersion})
	if err != nil {
		return Task{}, err
	}
	return s.repo.TrashTask(ctx, scope, id, expectedVersion, mutation)
}

func (s *Service) RestoreTask(ctx context.Context, scope Scope, id string, expectedVersion int64, idempotencyKey string) (Task, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return Task{}, err
	}
	if err := validateVersion(expectedVersion); err != nil {
		return Task{}, err
	}
	mutation, err := s.mutation(OperationTaskRestore, idempotencyKey, versionRequest{id, expectedVersion})
	if err != nil {
		return Task{}, err
	}
	return s.repo.RestoreTask(ctx, scope, id, expectedVersion, mutation)
}

func (s *Service) PurgeTask(ctx context.Context, scope Scope, id string, expectedVersion int64, idempotencyKey string) (PurgeResult, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return PurgeResult{}, err
	}
	if err := validateVersion(expectedVersion); err != nil {
		return PurgeResult{}, err
	}
	mutation, err := s.mutation(OperationTaskPurge, idempotencyKey, versionRequest{id, expectedVersion})
	if err != nil {
		return PurgeResult{}, err
	}
	return s.repo.PurgeTask(ctx, scope, id, expectedVersion, mutation)
}

func (s *Service) CreateDecision(ctx context.Context, scope Scope, input CreateDecisionInput, idempotencyKey string) (Decision, error) {
	if err := scope.Validate(); err != nil {
		return Decision{}, err
	}
	input, err := validateDecisionCreate(input)
	if err != nil {
		return Decision{}, err
	}
	if err := ValidateIdempotencyKey(idempotencyKey); err != nil {
		return Decision{}, err
	}
	if input.ID == "" {
		input.ID = stableID(EntityDecision, scope, idempotencyKey)
	}
	now := s.now()
	decision := Decision{
		ID:          input.ID,
		WorkspaceID: scope.WorkspaceID,
		OwnerUserID: scope.UserID,
		ProjectID:   input.ProjectID,
		Title:       input.Title,
		Context:     input.Context,
		Outcome:     input.Outcome,
		Rationale:   input.Rationale,
		Status:      input.Status,
		Origin:      OriginCanonical,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	mutation, err := newMutation(OperationDecisionCreate, idempotencyKey, input, now)
	if err != nil {
		return Decision{}, err
	}
	return s.repo.CreateDecision(ctx, scope, decision, mutation)
}

func (s *Service) GetDecision(ctx context.Context, scope Scope, id string) (Decision, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return Decision{}, err
	}
	return s.repo.GetDecision(ctx, scope, id)
}

func (s *Service) ListDecisions(ctx context.Context, scope Scope, projectID string, options ListOptions) ([]Decision, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if projectID != "" {
		if err := validateEntityID("projectId", projectID); err != nil {
			return nil, err
		}
	}
	options, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	return s.repo.ListDecisions(ctx, scope, projectID, options)
}

func (s *Service) UpdateDecision(ctx context.Context, scope Scope, id string, patch DecisionPatch, expectedVersion int64, idempotencyKey string) (Decision, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return Decision{}, err
	}
	if err := validateVersion(expectedVersion); err != nil {
		return Decision{}, err
	}
	if err := validateDecisionPatch(patch); err != nil {
		return Decision{}, err
	}
	mutation, err := s.mutation(OperationDecisionUpdate, idempotencyKey, struct {
		ID              string        `json:"id"`
		Patch           DecisionPatch `json:"patch"`
		ExpectedVersion int64         `json:"expectedVersion"`
	}{id, patch, expectedVersion})
	if err != nil {
		return Decision{}, err
	}
	return s.repo.UpdateDecision(ctx, scope, id, patch, expectedVersion, mutation)
}

func (s *Service) TrashDecision(ctx context.Context, scope Scope, id string, expectedVersion int64, idempotencyKey string) (Decision, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return Decision{}, err
	}
	if err := validateVersion(expectedVersion); err != nil {
		return Decision{}, err
	}
	mutation, err := s.mutation(OperationDecisionTrash, idempotencyKey, versionRequest{id, expectedVersion})
	if err != nil {
		return Decision{}, err
	}
	return s.repo.TrashDecision(ctx, scope, id, expectedVersion, mutation)
}

func (s *Service) RestoreDecision(ctx context.Context, scope Scope, id string, expectedVersion int64, idempotencyKey string) (Decision, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return Decision{}, err
	}
	if err := validateVersion(expectedVersion); err != nil {
		return Decision{}, err
	}
	mutation, err := s.mutation(OperationDecisionRestore, idempotencyKey, versionRequest{id, expectedVersion})
	if err != nil {
		return Decision{}, err
	}
	return s.repo.RestoreDecision(ctx, scope, id, expectedVersion, mutation)
}

func (s *Service) PurgeDecision(ctx context.Context, scope Scope, id string, expectedVersion int64, idempotencyKey string) (PurgeResult, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return PurgeResult{}, err
	}
	if err := validateVersion(expectedVersion); err != nil {
		return PurgeResult{}, err
	}
	mutation, err := s.mutation(OperationDecisionPurge, idempotencyKey, versionRequest{id, expectedVersion})
	if err != nil {
		return PurgeResult{}, err
	}
	return s.repo.PurgeDecision(ctx, scope, id, expectedVersion, mutation)
}

func (s *Service) History(ctx context.Context, scope Scope, entityType EntityType, id string) ([]HistoryEvent, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if err := validateEntityType(entityType); err != nil {
		return nil, err
	}
	if err := validateEntityID("entityId", id); err != nil {
		return nil, err
	}
	return s.repo.History(ctx, scope, entityType, id)
}

func (s *Service) trashProject(ctx context.Context, scope Scope, id string, expectedVersion int64, idempotencyKey string) (Project, error) {
	if err := validateScopeAndID(scope, id); err != nil {
		return Project{}, err
	}
	if err := validateVersion(expectedVersion); err != nil {
		return Project{}, err
	}
	mutation, err := s.mutation(OperationProjectTrash, idempotencyKey, versionRequest{id, expectedVersion})
	if err != nil {
		return Project{}, err
	}
	return s.repo.TrashProject(ctx, scope, id, expectedVersion, mutation)
}

type versionRequest struct {
	ID              string `json:"id"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

func (s *Service) mutation(operation, idempotencyKey string, payload any) (Mutation, error) {
	return newMutation(operation, idempotencyKey, payload, s.now())
}

func newMutation(operation, idempotencyKey string, payload any, at time.Time) (Mutation, error) {
	if err := ValidateIdempotencyKey(idempotencyKey); err != nil {
		return Mutation{}, err
	}
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		Payload   any    `json:"payload"`
	}{operation, payload})
	if err != nil {
		return Mutation{}, errors.New("hash mutation request: " + err.Error())
	}
	sum := sha256.Sum256(raw)
	return Mutation{IdempotencyKey: idempotencyKey, RequestHash: hex.EncodeToString(sum[:]), At: at.UTC()}, nil
}

func stableID(entityType EntityType, scope Scope, key string) string {
	sum := sha256.Sum256([]byte(string(entityType) + ":" + scope.WorkspaceID + ":" + scope.UserID + ":" + key))
	return string(entityType) + "-" + hex.EncodeToString(sum[:12])
}

func validateScopeAndID(scope Scope, id string) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	return validateEntityID("id", id)
}

func (s *Service) now() time.Time {
	if s == nil || s.clock == nil {
		return time.Now().UTC()
	}
	return s.clock().UTC()
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}
