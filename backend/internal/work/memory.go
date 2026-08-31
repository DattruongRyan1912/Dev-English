package work

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

var _ Repository = (*MemoryRepository)(nil)

// MemoryRepository is a small reference implementation of the contracts. It
// is useful for unit tests and local development; a PostgreSQL adapter can
// preserve the same atomic mutation/idempotency/history semantics.
type MemoryRepository struct {
	mu          sync.RWMutex
	projects    map[memoryEntityKey]Project
	tasks       map[memoryEntityKey]Task
	decisions   map[memoryEntityKey]Decision
	idempotency map[memoryIdempotencyKey]memoryIdempotencyEntry
	history     map[memoryHistoryKey][]HistoryEvent
	historySeq  uint64
}

type memoryEntityKey struct {
	WorkspaceID string
	UserID      string
	EntityID    string
}

type memoryIdempotencyKey struct {
	WorkspaceID string
	UserID      string
	Key         string
}

type memoryIdempotencyEntry struct {
	Operation   string
	RequestHash string
	EntityType  EntityType
	EntityID    string
	Result      []byte
}

type memoryHistoryKey struct {
	WorkspaceID string
	UserID      string
	EntityType  EntityType
	EntityID    string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		projects:    make(map[memoryEntityKey]Project),
		tasks:       make(map[memoryEntityKey]Task),
		decisions:   make(map[memoryEntityKey]Decision),
		idempotency: make(map[memoryIdempotencyKey]memoryIdempotencyEntry),
		history:     make(map[memoryHistoryKey][]HistoryEvent),
	}
}

func (r *MemoryRepository) CreateProject(_ context.Context, scope Scope, project Project, mutation Mutation) (Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationProjectCreate, mutation); err != nil {
		return Project{}, err
	} else if replay != nil {
		var result Project
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return Project{}, fmt.Errorf("decode idempotent project result: %w", err)
		}
		return result, nil
	}
	if err := validateStoredProject(scope, project); err != nil {
		return Project{}, err
	}
	if _, exists := r.projects[entityKey(scope, project.ID)]; exists {
		return Project{}, ErrAlreadyExists
	}
	if project.CreatedAt.IsZero() {
		project.CreatedAt = mutationTime(mutation)
	}
	if project.UpdatedAt.IsZero() {
		project.UpdatedAt = project.CreatedAt
	}
	if err := r.appendHistoryLocked(scope, EntityProject, project.ID, HistoryCreated, 0, project.Version, mutation.IdempotencyKey, nil, project, mutationTime(mutation)); err != nil {
		return Project{}, err
	}
	r.projects[entityKey(scope, project.ID)] = cloneProject(project)
	if err := r.rememberLocked(scope, mutation, OperationProjectCreate, EntityProject, project.ID, project); err != nil {
		return Project{}, err
	}
	return cloneProject(project), nil
}

func (r *MemoryRepository) GetProject(_ context.Context, scope Scope, id string) (Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	project, ok := r.projects[entityKey(scope, id)]
	if !ok || !projectInScope(project.WorkspaceID, project.OwnerUserID, scope) || project.DeletedAt != nil {
		return Project{}, notFound(EntityProject, id)
	}
	return cloneProject(project), nil
}

func (r *MemoryRepository) ListProjects(_ context.Context, scope Scope, options ListOptions) ([]Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	options, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	items := make([]Project, 0)
	for _, project := range r.projects {
		if !projectInScope(project.WorkspaceID, project.OwnerUserID, scope) || (!options.IncludeTrashed && project.DeletedAt != nil) {
			continue
		}
		items = append(items, cloneProject(project))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	if len(items) > options.Limit {
		items = items[:options.Limit]
	}
	return items, nil
}

func (r *MemoryRepository) UpdateProject(_ context.Context, scope Scope, id string, patch ProjectPatch, expectedVersion int64, mutation Mutation) (Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationProjectUpdate, mutation); err != nil {
		return Project{}, err
	} else if replay != nil {
		var result Project
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return Project{}, fmt.Errorf("decode idempotent project result: %w", err)
		}
		return result, nil
	}
	if err := validateProjectPatch(patch); err != nil {
		return Project{}, err
	}
	project, ok := r.projects[entityKey(scope, id)]
	if !ok || !projectInScope(project.WorkspaceID, project.OwnerUserID, scope) {
		return Project{}, notFound(EntityProject, id)
	}
	if err := checkVersion(EntityProject, id, project.Version, expectedVersion); err != nil {
		return Project{}, err
	}
	if project.DeletedAt != nil {
		return Project{}, &StateError{EntityType: EntityProject, EntityID: id, State: "trashed", Cause: ErrRecordTrashed}
	}
	updated := cloneProject(project)
	if patch.Name != nil {
		updated.Name = *patch.Name
	}
	if patch.Description != nil {
		updated.Description = *patch.Description
	}
	if patch.Status != nil {
		updated.Status = *patch.Status
	}
	updated.Version++
	updated.UpdatedAt = mutationTime(mutation)
	if err := r.appendHistoryLocked(scope, EntityProject, id, HistoryUpdated, project.Version, updated.Version, mutation.IdempotencyKey, project, updated, mutationTime(mutation)); err != nil {
		return Project{}, err
	}
	r.projects[entityKey(scope, id)] = updated
	if err := r.rememberLocked(scope, mutation, OperationProjectUpdate, EntityProject, id, updated); err != nil {
		return Project{}, err
	}
	return cloneProject(updated), nil
}

func (r *MemoryRepository) TrashProject(_ context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationProjectTrash, mutation); err != nil {
		return Project{}, err
	} else if replay != nil {
		var result Project
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return Project{}, fmt.Errorf("decode idempotent project result: %w", err)
		}
		return result, nil
	}
	project, ok := r.projects[entityKey(scope, id)]
	if !ok || !projectInScope(project.WorkspaceID, project.OwnerUserID, scope) {
		return Project{}, notFound(EntityProject, id)
	}
	if err := checkVersion(EntityProject, id, project.Version, expectedVersion); err != nil {
		return Project{}, err
	}
	if project.DeletedAt != nil {
		return Project{}, &StateError{EntityType: EntityProject, EntityID: id, State: "trashed", Cause: ErrAlreadyTrashed}
	}
	updated := cloneProject(project)
	deletedAt := mutationTime(mutation)
	updated.DeletedAt = &deletedAt
	updated.Version++
	updated.UpdatedAt = deletedAt
	if err := r.appendHistoryLocked(scope, EntityProject, id, HistoryTrashed, project.Version, updated.Version, mutation.IdempotencyKey, project, updated, deletedAt); err != nil {
		return Project{}, err
	}
	r.projects[entityKey(scope, id)] = updated
	if err := r.rememberLocked(scope, mutation, OperationProjectTrash, EntityProject, id, updated); err != nil {
		return Project{}, err
	}
	return cloneProject(updated), nil
}

func (r *MemoryRepository) RestoreProject(_ context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationProjectRestore, mutation); err != nil {
		return Project{}, err
	} else if replay != nil {
		var result Project
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return Project{}, fmt.Errorf("decode idempotent project result: %w", err)
		}
		return result, nil
	}
	project, ok := r.projects[entityKey(scope, id)]
	if !ok || !projectInScope(project.WorkspaceID, project.OwnerUserID, scope) {
		return Project{}, notFound(EntityProject, id)
	}
	if err := checkVersion(EntityProject, id, project.Version, expectedVersion); err != nil {
		return Project{}, err
	}
	if project.DeletedAt == nil {
		return Project{}, &StateError{EntityType: EntityProject, EntityID: id, State: "active", Cause: ErrNotTrashed}
	}
	updated := cloneProject(project)
	updated.DeletedAt = nil
	updated.Version++
	updated.UpdatedAt = mutationTime(mutation)
	if err := r.appendHistoryLocked(scope, EntityProject, id, HistoryRestored, project.Version, updated.Version, mutation.IdempotencyKey, project, updated, updated.UpdatedAt); err != nil {
		return Project{}, err
	}
	r.projects[entityKey(scope, id)] = updated
	if err := r.rememberLocked(scope, mutation, OperationProjectRestore, EntityProject, id, updated); err != nil {
		return Project{}, err
	}
	return cloneProject(updated), nil
}

func (r *MemoryRepository) PurgeProject(_ context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (PurgeResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationProjectPurge, mutation); err != nil {
		return PurgeResult{}, err
	} else if replay != nil {
		var result PurgeResult
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return PurgeResult{}, fmt.Errorf("decode idempotent purge result: %w", err)
		}
		return result, nil
	}
	project, ok := r.projects[entityKey(scope, id)]
	if !ok || !projectInScope(project.WorkspaceID, project.OwnerUserID, scope) {
		return PurgeResult{}, notFound(EntityProject, id)
	}
	if err := checkVersion(EntityProject, id, project.Version, expectedVersion); err != nil {
		return PurgeResult{}, err
	}
	if project.DeletedAt == nil {
		return PurgeResult{}, &StateError{EntityType: EntityProject, EntityID: id, State: "active", Cause: ErrNotTrashed}
	}
	at := mutationTime(mutation)
	if at.Before(project.DeletedAt.Add(RetentionPeriod)) {
		return PurgeResult{}, &PurgeNotReadyError{EntityType: EntityProject, EntityID: id, EligibleAt: project.DeletedAt.Add(RetentionPeriod).UTC().Format(time.RFC3339)}
	}
	if dependents := r.projectDependentsLocked(project); len(dependents) > 0 {
		return PurgeResult{}, &DependencyError{EntityType: EntityProject, EntityID: id, Dependents: dependents}
	}
	if err := r.appendHistoryLocked(scope, EntityProject, id, HistoryPurged, project.Version, project.Version+1, mutation.IdempotencyKey, project, nil, at); err != nil {
		return PurgeResult{}, err
	}
	delete(r.projects, entityKey(scope, id))
	result := PurgeResult{EntityType: EntityProject, EntityID: id, PurgedAt: at}
	if err := r.rememberLocked(scope, mutation, OperationProjectPurge, EntityProject, id, result); err != nil {
		return PurgeResult{}, err
	}
	return result, nil
}

func (r *MemoryRepository) CreateTask(_ context.Context, scope Scope, task Task, mutation Mutation) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationTaskCreate, mutation); err != nil {
		return Task{}, err
	} else if replay != nil {
		var result Task
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return Task{}, fmt.Errorf("decode idempotent task result: %w", err)
		}
		return result, nil
	}
	if err := validateStoredTask(scope, task); err != nil {
		return Task{}, err
	}
	if err := r.validateProjectLinkLocked(scope, task.ProjectID); err != nil {
		return Task{}, err
	}
	if _, exists := r.tasks[entityKey(scope, task.ID)]; exists {
		return Task{}, ErrAlreadyExists
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = mutationTime(mutation)
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = task.CreatedAt
	}
	if err := r.appendHistoryLocked(scope, EntityTask, task.ID, HistoryCreated, 0, task.Version, mutation.IdempotencyKey, nil, task, mutationTime(mutation)); err != nil {
		return Task{}, err
	}
	r.tasks[entityKey(scope, task.ID)] = cloneTask(task)
	if err := r.rememberLocked(scope, mutation, OperationTaskCreate, EntityTask, task.ID, task); err != nil {
		return Task{}, err
	}
	return cloneTask(task), nil
}

func (r *MemoryRepository) GetTask(_ context.Context, scope Scope, id string) (Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[entityKey(scope, id)]
	if !ok || !projectInScope(task.WorkspaceID, task.OwnerUserID, scope) || task.DeletedAt != nil {
		return Task{}, notFound(EntityTask, id)
	}
	return cloneTask(task), nil
}

func (r *MemoryRepository) ListTasks(_ context.Context, scope Scope, projectID string, options ListOptions) ([]Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	options, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	if projectID != "" {
		if project, ok := r.projects[entityKey(scope, projectID)]; !ok || !projectInScope(project.WorkspaceID, project.OwnerUserID, scope) {
			return nil, notFound(EntityProject, projectID)
		}
	}
	items := make([]Task, 0)
	for _, task := range r.tasks {
		if !projectInScope(task.WorkspaceID, task.OwnerUserID, scope) || (!options.IncludeTrashed && task.DeletedAt != nil) {
			continue
		}
		if projectID != "" && task.ProjectID != projectID {
			continue
		}
		items = append(items, cloneTask(task))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	if len(items) > options.Limit {
		items = items[:options.Limit]
	}
	return items, nil
}

func (r *MemoryRepository) UpdateTask(_ context.Context, scope Scope, id string, patch TaskPatch, expectedVersion int64, mutation Mutation) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationTaskUpdate, mutation); err != nil {
		return Task{}, err
	} else if replay != nil {
		var result Task
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return Task{}, fmt.Errorf("decode idempotent task result: %w", err)
		}
		return result, nil
	}
	if err := validateTaskPatch(patch); err != nil {
		return Task{}, err
	}
	task, ok := r.tasks[entityKey(scope, id)]
	if !ok || !projectInScope(task.WorkspaceID, task.OwnerUserID, scope) {
		return Task{}, notFound(EntityTask, id)
	}
	if err := checkVersion(EntityTask, id, task.Version, expectedVersion); err != nil {
		return Task{}, err
	}
	if task.DeletedAt != nil {
		return Task{}, &StateError{EntityType: EntityTask, EntityID: id, State: "trashed", Cause: ErrRecordTrashed}
	}
	updated := cloneTask(task)
	if patch.ProjectID != nil {
		updated.ProjectID = *patch.ProjectID
	}
	if err := r.validateProjectLinkLocked(scope, updated.ProjectID); err != nil {
		return Task{}, err
	}
	if patch.Title != nil {
		updated.Title = *patch.Title
	}
	if patch.Description != nil {
		updated.Description = *patch.Description
	}
	if patch.Status != nil {
		updated.Status = *patch.Status
	}
	if patch.Priority != nil {
		updated.Priority = *patch.Priority
	}
	if patch.DueAt != nil {
		updated.DueAt = cloneTimePtr(*patch.DueAt)
	}
	updated.Version++
	updated.UpdatedAt = mutationTime(mutation)
	if err := r.appendHistoryLocked(scope, EntityTask, id, HistoryUpdated, task.Version, updated.Version, mutation.IdempotencyKey, task, updated, updated.UpdatedAt); err != nil {
		return Task{}, err
	}
	r.tasks[entityKey(scope, id)] = updated
	if err := r.rememberLocked(scope, mutation, OperationTaskUpdate, EntityTask, id, updated); err != nil {
		return Task{}, err
	}
	return cloneTask(updated), nil
}

func (r *MemoryRepository) TrashTask(_ context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationTaskTrash, mutation); err != nil {
		return Task{}, err
	} else if replay != nil {
		var result Task
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return Task{}, fmt.Errorf("decode idempotent task result: %w", err)
		}
		return result, nil
	}
	task, ok := r.tasks[entityKey(scope, id)]
	if !ok || !projectInScope(task.WorkspaceID, task.OwnerUserID, scope) {
		return Task{}, notFound(EntityTask, id)
	}
	if err := checkVersion(EntityTask, id, task.Version, expectedVersion); err != nil {
		return Task{}, err
	}
	if task.DeletedAt != nil {
		return Task{}, &StateError{EntityType: EntityTask, EntityID: id, State: "trashed", Cause: ErrAlreadyTrashed}
	}
	updated := cloneTask(task)
	at := mutationTime(mutation)
	updated.DeletedAt = &at
	updated.Version++
	updated.UpdatedAt = at
	if err := r.appendHistoryLocked(scope, EntityTask, id, HistoryTrashed, task.Version, updated.Version, mutation.IdempotencyKey, task, updated, at); err != nil {
		return Task{}, err
	}
	r.tasks[entityKey(scope, id)] = updated
	if err := r.rememberLocked(scope, mutation, OperationTaskTrash, EntityTask, id, updated); err != nil {
		return Task{}, err
	}
	return cloneTask(updated), nil
}

func (r *MemoryRepository) RestoreTask(_ context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationTaskRestore, mutation); err != nil {
		return Task{}, err
	} else if replay != nil {
		var result Task
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return Task{}, fmt.Errorf("decode idempotent task result: %w", err)
		}
		return result, nil
	}
	task, ok := r.tasks[entityKey(scope, id)]
	if !ok || !projectInScope(task.WorkspaceID, task.OwnerUserID, scope) {
		return Task{}, notFound(EntityTask, id)
	}
	if err := checkVersion(EntityTask, id, task.Version, expectedVersion); err != nil {
		return Task{}, err
	}
	if task.DeletedAt == nil {
		return Task{}, &StateError{EntityType: EntityTask, EntityID: id, State: "active", Cause: ErrNotTrashed}
	}
	updated := cloneTask(task)
	updated.DeletedAt = nil
	updated.Version++
	updated.UpdatedAt = mutationTime(mutation)
	if err := r.appendHistoryLocked(scope, EntityTask, id, HistoryRestored, task.Version, updated.Version, mutation.IdempotencyKey, task, updated, updated.UpdatedAt); err != nil {
		return Task{}, err
	}
	r.tasks[entityKey(scope, id)] = updated
	if err := r.rememberLocked(scope, mutation, OperationTaskRestore, EntityType(EntityTask), id, updated); err != nil {
		return Task{}, err
	}
	return cloneTask(updated), nil
}

func (r *MemoryRepository) PurgeTask(_ context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (PurgeResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationTaskPurge, mutation); err != nil {
		return PurgeResult{}, err
	} else if replay != nil {
		var result PurgeResult
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return PurgeResult{}, fmt.Errorf("decode idempotent purge result: %w", err)
		}
		return result, nil
	}
	task, ok := r.tasks[entityKey(scope, id)]
	if !ok || !projectInScope(task.WorkspaceID, task.OwnerUserID, scope) {
		return PurgeResult{}, notFound(EntityTask, id)
	}
	if err := checkVersion(EntityTask, id, task.Version, expectedVersion); err != nil {
		return PurgeResult{}, err
	}
	if task.DeletedAt == nil {
		return PurgeResult{}, &StateError{EntityType: EntityTask, EntityID: id, State: "active", Cause: ErrNotTrashed}
	}
	at := mutationTime(mutation)
	if at.Before(task.DeletedAt.Add(RetentionPeriod)) {
		return PurgeResult{}, &PurgeNotReadyError{EntityType: EntityTask, EntityID: id, EligibleAt: task.DeletedAt.Add(RetentionPeriod).UTC().Format(time.RFC3339)}
	}
	if err := r.appendHistoryLocked(scope, EntityTask, id, HistoryPurged, task.Version, task.Version+1, mutation.IdempotencyKey, task, nil, at); err != nil {
		return PurgeResult{}, err
	}
	delete(r.tasks, entityKey(scope, id))
	result := PurgeResult{EntityType: EntityTask, EntityID: id, PurgedAt: at}
	if err := r.rememberLocked(scope, mutation, OperationTaskPurge, EntityTask, id, result); err != nil {
		return PurgeResult{}, err
	}
	return result, nil
}

func (r *MemoryRepository) CreateDecision(_ context.Context, scope Scope, decision Decision, mutation Mutation) (Decision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationDecisionCreate, mutation); err != nil {
		return Decision{}, err
	} else if replay != nil {
		var result Decision
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return Decision{}, fmt.Errorf("decode idempotent decision result: %w", err)
		}
		return result, nil
	}
	if err := validateStoredDecision(scope, decision); err != nil {
		return Decision{}, err
	}
	if err := r.validateProjectLinkLocked(scope, decision.ProjectID); err != nil {
		return Decision{}, err
	}
	if _, exists := r.decisions[entityKey(scope, decision.ID)]; exists {
		return Decision{}, ErrAlreadyExists
	}
	if decision.CreatedAt.IsZero() {
		decision.CreatedAt = mutationTime(mutation)
	}
	if decision.UpdatedAt.IsZero() {
		decision.UpdatedAt = decision.CreatedAt
	}
	if err := r.appendHistoryLocked(scope, EntityDecision, decision.ID, HistoryCreated, 0, decision.Version, mutation.IdempotencyKey, nil, decision, mutationTime(mutation)); err != nil {
		return Decision{}, err
	}
	r.decisions[entityKey(scope, decision.ID)] = cloneDecision(decision)
	if err := r.rememberLocked(scope, mutation, OperationDecisionCreate, EntityDecision, decision.ID, decision); err != nil {
		return Decision{}, err
	}
	return cloneDecision(decision), nil
}

func (r *MemoryRepository) GetDecision(_ context.Context, scope Scope, id string) (Decision, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	decision, ok := r.decisions[entityKey(scope, id)]
	if !ok || !projectInScope(decision.WorkspaceID, decision.OwnerUserID, scope) || decision.DeletedAt != nil {
		return Decision{}, notFound(EntityDecision, id)
	}
	return cloneDecision(decision), nil
}

func (r *MemoryRepository) ListDecisions(_ context.Context, scope Scope, projectID string, options ListOptions) ([]Decision, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	options, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	if projectID != "" {
		if project, ok := r.projects[entityKey(scope, projectID)]; !ok || !projectInScope(project.WorkspaceID, project.OwnerUserID, scope) {
			return nil, notFound(EntityProject, projectID)
		}
	}
	items := make([]Decision, 0)
	for _, decision := range r.decisions {
		if !projectInScope(decision.WorkspaceID, decision.OwnerUserID, scope) || (!options.IncludeTrashed && decision.DeletedAt != nil) {
			continue
		}
		if projectID != "" && decision.ProjectID != projectID {
			continue
		}
		items = append(items, cloneDecision(decision))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	if len(items) > options.Limit {
		items = items[:options.Limit]
	}
	return items, nil
}

func (r *MemoryRepository) UpdateDecision(_ context.Context, scope Scope, id string, patch DecisionPatch, expectedVersion int64, mutation Mutation) (Decision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationDecisionUpdate, mutation); err != nil {
		return Decision{}, err
	} else if replay != nil {
		var result Decision
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return Decision{}, fmt.Errorf("decode idempotent decision result: %w", err)
		}
		return result, nil
	}
	if err := validateDecisionPatch(patch); err != nil {
		return Decision{}, err
	}
	decision, ok := r.decisions[entityKey(scope, id)]
	if !ok || !projectInScope(decision.WorkspaceID, decision.OwnerUserID, scope) {
		return Decision{}, notFound(EntityDecision, id)
	}
	if err := checkVersion(EntityDecision, id, decision.Version, expectedVersion); err != nil {
		return Decision{}, err
	}
	if decision.DeletedAt != nil {
		return Decision{}, &StateError{EntityType: EntityDecision, EntityID: id, State: "trashed", Cause: ErrRecordTrashed}
	}
	updated := cloneDecision(decision)
	if patch.ProjectID != nil {
		updated.ProjectID = *patch.ProjectID
	}
	if err := r.validateProjectLinkLocked(scope, updated.ProjectID); err != nil {
		return Decision{}, err
	}
	if patch.Title != nil {
		updated.Title = *patch.Title
	}
	if patch.Context != nil {
		updated.Context = *patch.Context
	}
	if patch.Outcome != nil {
		updated.Outcome = *patch.Outcome
	}
	if patch.Rationale != nil {
		updated.Rationale = *patch.Rationale
	}
	if patch.Status != nil {
		updated.Status = *patch.Status
	}
	updated.Version++
	updated.UpdatedAt = mutationTime(mutation)
	if err := r.appendHistoryLocked(scope, EntityDecision, id, HistoryUpdated, decision.Version, updated.Version, mutation.IdempotencyKey, decision, updated, updated.UpdatedAt); err != nil {
		return Decision{}, err
	}
	r.decisions[entityKey(scope, id)] = updated
	if err := r.rememberLocked(scope, mutation, OperationDecisionUpdate, EntityDecision, id, updated); err != nil {
		return Decision{}, err
	}
	return cloneDecision(updated), nil
}

func (r *MemoryRepository) TrashDecision(_ context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (Decision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationDecisionTrash, mutation); err != nil {
		return Decision{}, err
	} else if replay != nil {
		var result Decision
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return Decision{}, fmt.Errorf("decode idempotent decision result: %w", err)
		}
		return result, nil
	}
	decision, ok := r.decisions[entityKey(scope, id)]
	if !ok || !projectInScope(decision.WorkspaceID, decision.OwnerUserID, scope) {
		return Decision{}, notFound(EntityDecision, id)
	}
	if err := checkVersion(EntityDecision, id, decision.Version, expectedVersion); err != nil {
		return Decision{}, err
	}
	if decision.DeletedAt != nil {
		return Decision{}, &StateError{EntityType: EntityDecision, EntityID: id, State: "trashed", Cause: ErrAlreadyTrashed}
	}
	updated := cloneDecision(decision)
	at := mutationTime(mutation)
	updated.DeletedAt = &at
	updated.Version++
	updated.UpdatedAt = at
	if err := r.appendHistoryLocked(scope, EntityDecision, id, HistoryTrashed, decision.Version, updated.Version, mutation.IdempotencyKey, decision, updated, at); err != nil {
		return Decision{}, err
	}
	r.decisions[entityKey(scope, id)] = updated
	if err := r.rememberLocked(scope, mutation, OperationDecisionTrash, EntityDecision, id, updated); err != nil {
		return Decision{}, err
	}
	return cloneDecision(updated), nil
}

func (r *MemoryRepository) RestoreDecision(_ context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (Decision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationDecisionRestore, mutation); err != nil {
		return Decision{}, err
	} else if replay != nil {
		var result Decision
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return Decision{}, fmt.Errorf("decode idempotent decision result: %w", err)
		}
		return result, nil
	}
	decision, ok := r.decisions[entityKey(scope, id)]
	if !ok || !projectInScope(decision.WorkspaceID, decision.OwnerUserID, scope) {
		return Decision{}, notFound(EntityDecision, id)
	}
	if err := checkVersion(EntityDecision, id, decision.Version, expectedVersion); err != nil {
		return Decision{}, err
	}
	if decision.DeletedAt == nil {
		return Decision{}, &StateError{EntityType: EntityDecision, EntityID: id, State: "active", Cause: ErrNotTrashed}
	}
	updated := cloneDecision(decision)
	updated.DeletedAt = nil
	updated.Version++
	updated.UpdatedAt = mutationTime(mutation)
	if err := r.appendHistoryLocked(scope, EntityDecision, id, HistoryRestored, decision.Version, updated.Version, mutation.IdempotencyKey, decision, updated, updated.UpdatedAt); err != nil {
		return Decision{}, err
	}
	r.decisions[entityKey(scope, id)] = updated
	if err := r.rememberLocked(scope, mutation, OperationDecisionRestore, EntityDecision, id, updated); err != nil {
		return Decision{}, err
	}
	return cloneDecision(updated), nil
}

func (r *MemoryRepository) PurgeDecision(_ context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (PurgeResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replay, err := r.beginMutationLocked(scope, OperationDecisionPurge, mutation); err != nil {
		return PurgeResult{}, err
	} else if replay != nil {
		var result PurgeResult
		if err := json.Unmarshal(replay.Result, &result); err != nil {
			return PurgeResult{}, fmt.Errorf("decode idempotent purge result: %w", err)
		}
		return result, nil
	}
	decision, ok := r.decisions[entityKey(scope, id)]
	if !ok || !projectInScope(decision.WorkspaceID, decision.OwnerUserID, scope) {
		return PurgeResult{}, notFound(EntityDecision, id)
	}
	if err := checkVersion(EntityDecision, id, decision.Version, expectedVersion); err != nil {
		return PurgeResult{}, err
	}
	if decision.DeletedAt == nil {
		return PurgeResult{}, &StateError{EntityType: EntityDecision, EntityID: id, State: "active", Cause: ErrNotTrashed}
	}
	at := mutationTime(mutation)
	if at.Before(decision.DeletedAt.Add(RetentionPeriod)) {
		return PurgeResult{}, &PurgeNotReadyError{EntityType: EntityDecision, EntityID: id, EligibleAt: decision.DeletedAt.Add(RetentionPeriod).UTC().Format(time.RFC3339)}
	}
	if err := r.appendHistoryLocked(scope, EntityDecision, id, HistoryPurged, decision.Version, decision.Version+1, mutation.IdempotencyKey, decision, nil, at); err != nil {
		return PurgeResult{}, err
	}
	delete(r.decisions, entityKey(scope, id))
	result := PurgeResult{EntityType: EntityDecision, EntityID: id, PurgedAt: at}
	if err := r.rememberLocked(scope, mutation, OperationDecisionPurge, EntityDecision, id, result); err != nil {
		return PurgeResult{}, err
	}
	return result, nil
}

func (r *MemoryRepository) History(_ context.Context, scope Scope, entityType EntityType, id string) ([]HistoryEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if err := validateEntityType(entityType); err != nil {
		return nil, err
	}
	if err := validateEntityID("entityId", id); err != nil {
		return nil, err
	}
	events := r.history[memoryHistoryKey{WorkspaceID: scope.WorkspaceID, UserID: scope.UserID, EntityType: entityType, EntityID: id}]
	if len(events) == 0 {
		return nil, notFound(entityType, id)
	}
	result := make([]HistoryEvent, len(events))
	for index, event := range events {
		if event.WorkspaceID != scope.WorkspaceID || event.ActorUserID != scope.UserID || event.EntityType != entityType || event.EntityID != id {
			return nil, notFound(entityType, id)
		}
		result[index] = cloneHistory(event)
	}
	return result, nil
}

func (r *MemoryRepository) beginMutationLocked(scope Scope, operation string, mutation Mutation) (*memoryIdempotencyEntry, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if err := mutation.Validate(operation); err != nil {
		return nil, err
	}
	key := memoryIdempotencyKey{WorkspaceID: scope.WorkspaceID, UserID: scope.UserID, Key: mutation.IdempotencyKey}
	entry, ok := r.idempotency[key]
	if !ok {
		return nil, nil
	}
	if entry.Operation != operation || entry.RequestHash != mutation.RequestHash {
		return nil, &IdempotencyConflictError{Key: mutation.IdempotencyKey, ExistingOperation: entry.Operation, RequestedOperation: operation}
	}
	copy := entry
	copy.Result = append([]byte(nil), entry.Result...)
	return &copy, nil
}

func (r *MemoryRepository) rememberLocked(scope Scope, mutation Mutation, operation string, entityType EntityType, entityID string, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent result: %w", err)
	}
	key := memoryIdempotencyKey{WorkspaceID: scope.WorkspaceID, UserID: scope.UserID, Key: mutation.IdempotencyKey}
	r.idempotency[key] = memoryIdempotencyEntry{
		Operation:   operation,
		RequestHash: mutation.RequestHash,
		EntityType:  entityType,
		EntityID:    entityID,
		Result:      append([]byte(nil), raw...),
	}
	return nil
}

func (r *MemoryRepository) appendHistoryLocked(scope Scope, entityType EntityType, entityID string, action HistoryAction, fromVersion, toVersion int64, idempotencyKey string, before, after any, at time.Time) error {
	beforeRaw, err := marshalSnapshot(before)
	if err != nil {
		return err
	}
	afterRaw, err := marshalSnapshot(after)
	if err != nil {
		return err
	}
	r.historySeq++
	event := HistoryEvent{
		ID:             fmt.Sprintf("work-history-%d", r.historySeq),
		WorkspaceID:    scope.WorkspaceID,
		ActorUserID:    scope.UserID,
		EntityType:     entityType,
		EntityID:       entityID,
		Action:         action,
		FromVersion:    fromVersion,
		ToVersion:      toVersion,
		IdempotencyKey: idempotencyKey,
		Before:         beforeRaw,
		After:          afterRaw,
		CreatedAt:      at.UTC(),
	}
	key := memoryHistoryKey{WorkspaceID: scope.WorkspaceID, UserID: scope.UserID, EntityType: entityType, EntityID: entityID}
	r.history[key] = append(r.history[key], event)
	return nil
}

func (r *MemoryRepository) validateProjectLinkLocked(scope Scope, projectID string) error {
	if projectID == "" {
		return nil
	}
	project, ok := r.projects[entityKey(scope, projectID)]
	if !ok || !projectInScope(project.WorkspaceID, project.OwnerUserID, scope) {
		return notFound(EntityProject, projectID)
	}
	if project.DeletedAt != nil {
		return &StateError{EntityType: EntityProject, EntityID: projectID, State: "trashed", Cause: ErrRecordTrashed}
	}
	return nil
}

func (r *MemoryRepository) projectDependentsLocked(project Project) []EntityType {
	dependents := make([]EntityType, 0, 2)
	for _, task := range r.tasks {
		if task.ProjectID == project.ID && projectInScope(task.WorkspaceID, task.OwnerUserID, Scope{WorkspaceID: project.WorkspaceID, UserID: project.OwnerUserID}) {
			dependents = append(dependents, EntityTask)
			break
		}
	}
	for _, decision := range r.decisions {
		if decision.ProjectID == project.ID && projectInScope(decision.WorkspaceID, decision.OwnerUserID, Scope{WorkspaceID: project.WorkspaceID, UserID: project.OwnerUserID}) {
			dependents = append(dependents, EntityDecision)
			break
		}
	}
	return dependents
}

func validateStoredProject(scope Scope, project Project) error {
	if err := validateEntityID("id", project.ID); err != nil {
		return err
	}
	if !projectInScope(project.WorkspaceID, project.OwnerUserID, scope) {
		return &ValidationError{Field: "scope", Message: "entity workspace and owner must match request scope"}
	}
	if err := validateText("name", project.Name, true, maxNameBytes); err != nil {
		return err
	}
	if err := validateText("description", project.Description, false, maxDescriptionBytes); err != nil {
		return err
	}
	if !validProjectStatus(project.Status) {
		return &ValidationError{Field: "status", Message: "has an unsupported project status"}
	}
	if project.Origin == "" {
		project.Origin = OriginCanonical
	}
	if project.Origin != OriginCanonical {
		return &ValidationError{Field: "origin", Message: "canonical work records must have origin=canonical"}
	}
	if project.Version != 1 {
		return &ValidationError{Field: "version", Message: "new projects must start at version 1"}
	}
	return nil
}

func validateStoredTask(scope Scope, task Task) error {
	if err := validateEntityID("id", task.ID); err != nil {
		return err
	}
	if !projectInScope(task.WorkspaceID, task.OwnerUserID, scope) {
		return &ValidationError{Field: "scope", Message: "entity workspace and owner must match request scope"}
	}
	if err := validateText("title", task.Title, true, maxTitleBytes); err != nil {
		return err
	}
	if err := validateText("description", task.Description, false, maxDescriptionBytes); err != nil {
		return err
	}
	if !validTaskStatus(task.Status) {
		return &ValidationError{Field: "status", Message: "has an unsupported task status"}
	}
	if !validTaskPriority(task.Priority) {
		return &ValidationError{Field: "priority", Message: "has an unsupported task priority"}
	}
	if task.Origin == "" {
		task.Origin = OriginCanonical
	}
	if task.Origin != OriginCanonical {
		return &ValidationError{Field: "origin", Message: "canonical work records must have origin=canonical"}
	}
	if task.Version != 1 {
		return &ValidationError{Field: "version", Message: "new tasks must start at version 1"}
	}
	return nil
}

func validateStoredDecision(scope Scope, decision Decision) error {
	if err := validateEntityID("id", decision.ID); err != nil {
		return err
	}
	if !projectInScope(decision.WorkspaceID, decision.OwnerUserID, scope) {
		return &ValidationError{Field: "scope", Message: "entity workspace and owner must match request scope"}
	}
	if err := validateText("title", decision.Title, true, maxTitleBytes); err != nil {
		return err
	}
	if err := validateText("context", decision.Context, false, maxDecisionBytes); err != nil {
		return err
	}
	if err := validateText("outcome", decision.Outcome, true, maxDecisionBytes); err != nil {
		return err
	}
	if err := validateText("rationale", decision.Rationale, false, maxDecisionBytes); err != nil {
		return err
	}
	if !validDecisionStatus(decision.Status) {
		return &ValidationError{Field: "status", Message: "has an unsupported decision status"}
	}
	if decision.Origin == "" {
		decision.Origin = OriginCanonical
	}
	if decision.Origin != OriginCanonical {
		return &ValidationError{Field: "origin", Message: "canonical work records must have origin=canonical"}
	}
	if decision.Version != 1 {
		return &ValidationError{Field: "version", Message: "new decisions must start at version 1"}
	}
	return nil
}

func checkVersion(entityType EntityType, id string, actual, expected int64) error {
	if actual != expected {
		return &VersionConflictError{EntityType: entityType, EntityID: id, Expected: expected, Actual: actual}
	}
	return nil
}

func projectInScope(workspaceID, ownerUserID string, scope Scope) bool {
	return workspaceID == scope.WorkspaceID && ownerUserID == scope.UserID
}

func entityKey(scope Scope, id string) memoryEntityKey {
	return memoryEntityKey{WorkspaceID: scope.WorkspaceID, UserID: scope.UserID, EntityID: id}
}

func notFound(entityType EntityType, id string) error {
	return &NotFoundError{EntityType: entityType, EntityID: id}
}

func mutationTime(mutation Mutation) time.Time {
	if mutation.At.IsZero() {
		return time.Now().UTC()
	}
	return mutation.At.UTC()
}

func marshalSnapshot(value any) (json.RawMessage, error) {
	if value == nil {
		return json.RawMessage(`{}`), nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode history snapshot: %w", err)
	}
	return append(json.RawMessage(nil), raw...), nil
}

func cloneProject(value Project) Project {
	if value.DeletedAt != nil {
		deletedAt := *value.DeletedAt
		value.DeletedAt = &deletedAt
	}
	return value
}

func cloneTask(value Task) Task {
	value.DueAt = cloneTimePtr(value.DueAt)
	if value.DeletedAt != nil {
		deletedAt := *value.DeletedAt
		value.DeletedAt = &deletedAt
	}
	return value
}

func cloneDecision(value Decision) Decision {
	if value.DeletedAt != nil {
		deletedAt := *value.DeletedAt
		value.DeletedAt = &deletedAt
	}
	return value
}

func cloneHistory(value HistoryEvent) HistoryEvent {
	value.Before = append(json.RawMessage(nil), value.Before...)
	value.After = append(json.RawMessage(nil), value.After...)
	return value
}
