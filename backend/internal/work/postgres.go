package work

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ Repository = (*PostgresRepository)(nil)

// PostgresRepository is the durable adapter for canonical work records. Every
// mutation is applied in the same transaction as its history and idempotency
// receipt; callers therefore never fall back to process-local state when a
// database is configured.
type PostgresRepository struct {
	Pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, &ValidationError{Field: "pool", Message: "is required"}
	}
	return &PostgresRepository{Pool: pool}, nil
}

func (r *PostgresRepository) CreateProject(ctx context.Context, scope Scope, project Project, mutation Mutation) (Project, error) {
	if err := mutation.Validate(OperationProjectCreate); err != nil {
		return Project{}, err
	}
	tx, replay, err := r.beginMutation(ctx, scope, OperationProjectCreate, mutation)
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if replay != nil {
		var result Project
		if err := json.Unmarshal(replay, &result); err != nil {
			return Project{}, fmt.Errorf("decode idempotent project result: %w", err)
		}
		return result, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO projects
			(id, workspace_id, owner_user_id, name, description, status, origin, version, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,'canonical',$7,$8,$9)
	`, project.ID, scope.WorkspaceID, scope.UserID, project.Name, project.Description, string(project.Status), positiveVersion(project.Version), project.CreatedAt, project.UpdatedAt); err != nil {
		return Project{}, mapWorkInsertError(err, EntityProject, project.ID)
	}
	project.Version = positiveVersion(project.Version)
	if err := appendHistory(ctx, tx, scope, EntityProject, project.ID, HistoryCreated, 0, project.Version, mutation.IdempotencyKey, nil, project, mutation.At); err != nil {
		return Project{}, err
	}
	if err := rememberMutation(ctx, tx, scope, mutation, OperationProjectCreate, EntityProject, project.ID, project); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Project{}, mapWorkDBError(err)
	}
	return project, nil
}

func (r *PostgresRepository) GetProject(ctx context.Context, scope Scope, id string) (Project, error) {
	var project Project
	err := r.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, owner_user_id, name, description, status, origin,
		       version, created_at, updated_at, deleted_at
		FROM projects
		WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3 AND deleted_at IS NULL
	`, scope.WorkspaceID, scope.UserID, id).Scan(&project.ID, &project.WorkspaceID, &project.OwnerUserID, &project.Name, &project.Description, &project.Status, &project.Origin, &project.Version, &project.CreatedAt, &project.UpdatedAt, &project.DeletedAt)
	if err != nil {
		return Project{}, mapWorkReadError(err, EntityProject, id)
	}
	return project, nil
}

func (r *PostgresRepository) ListProjects(ctx context.Context, scope Scope, options ListOptions) ([]Project, error) {
	options, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	query := `
		SELECT id, workspace_id, owner_user_id, name, description, status, origin,
		       version, created_at, updated_at, deleted_at
		FROM projects
		WHERE workspace_id=$1 AND owner_user_id=$2
	`
	args := []any{scope.WorkspaceID, scope.UserID}
	if !options.IncludeTrashed {
		query += " AND deleted_at IS NULL"
	}
	query += " ORDER BY created_at, id LIMIT $3"
	args = append(args, options.Limit)
	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapWorkDBError(err)
	}
	defer rows.Close()
	result := make([]Project, 0)
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.WorkspaceID, &project.OwnerUserID, &project.Name, &project.Description, &project.Status, &project.Origin, &project.Version, &project.CreatedAt, &project.UpdatedAt, &project.DeletedAt); err != nil {
			return nil, mapWorkDBError(err)
		}
		result = append(result, project)
	}
	if err := rows.Err(); err != nil {
		return nil, mapWorkDBError(err)
	}
	return result, nil
}

func (r *PostgresRepository) CreateTask(ctx context.Context, scope Scope, task Task, mutation Mutation) (Task, error) {
	if err := mutation.Validate(OperationTaskCreate); err != nil {
		return Task{}, err
	}
	tx, replay, err := r.beginMutation(ctx, scope, OperationTaskCreate, mutation)
	if err != nil {
		return Task{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if replay != nil {
		var result Task
		if err := json.Unmarshal(replay, &result); err != nil {
			return Task{}, fmt.Errorf("decode idempotent task result: %w", err)
		}
		return result, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO tasks
			(id, workspace_id, owner_user_id, project_id, title, description, status, priority, due_at, origin, version, created_at, updated_at)
		VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9,'canonical',$10,$11,$12)
	`, task.ID, scope.WorkspaceID, scope.UserID, task.ProjectID, task.Title, task.Description, string(task.Status), string(task.Priority), task.DueAt, positiveVersion(task.Version), task.CreatedAt, task.UpdatedAt); err != nil {
		return Task{}, mapWorkInsertError(err, EntityTask, task.ID)
	}
	task.Version = positiveVersion(task.Version)
	if err := appendHistory(ctx, tx, scope, EntityTask, task.ID, HistoryCreated, 0, task.Version, mutation.IdempotencyKey, nil, task, mutation.At); err != nil {
		return Task{}, err
	}
	if err := rememberMutation(ctx, tx, scope, mutation, OperationTaskCreate, EntityTask, task.ID, task); err != nil {
		return Task{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, mapWorkDBError(err)
	}
	return task, nil
}

func (r *PostgresRepository) GetTask(ctx context.Context, scope Scope, id string) (Task, error) {
	var task Task
	err := r.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, owner_user_id, COALESCE(project_id,''), title, description,
		       status, priority, due_at, origin, version, created_at, updated_at, deleted_at
		FROM tasks
		WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3 AND deleted_at IS NULL
	`, scope.WorkspaceID, scope.UserID, id).Scan(&task.ID, &task.WorkspaceID, &task.OwnerUserID, &task.ProjectID, &task.Title, &task.Description, &task.Status, &task.Priority, &task.DueAt, &task.Origin, &task.Version, &task.CreatedAt, &task.UpdatedAt, &task.DeletedAt)
	if err != nil {
		return Task{}, mapWorkReadError(err, EntityTask, id)
	}
	return task, nil
}

func (r *PostgresRepository) ListTasks(ctx context.Context, scope Scope, projectID string, options ListOptions) ([]Task, error) {
	options, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	query := `
		SELECT id, workspace_id, owner_user_id, COALESCE(project_id,''), title, description,
		       status, priority, due_at, origin, version, created_at, updated_at, deleted_at
		FROM tasks
		WHERE workspace_id=$1 AND owner_user_id=$2
	`
	args := []any{scope.WorkspaceID, scope.UserID}
	if projectID != "" {
		query += " AND project_id=$3"
		args = append(args, projectID)
	}
	if !options.IncludeTrashed {
		query += fmt.Sprintf(" AND deleted_at IS NULL")
	}
	query += fmt.Sprintf(" ORDER BY created_at, id LIMIT $%d", len(args)+1)
	args = append(args, options.Limit)
	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapWorkDBError(err)
	}
	defer rows.Close()
	result := make([]Task, 0)
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.WorkspaceID, &task.OwnerUserID, &task.ProjectID, &task.Title, &task.Description, &task.Status, &task.Priority, &task.DueAt, &task.Origin, &task.Version, &task.CreatedAt, &task.UpdatedAt, &task.DeletedAt); err != nil {
			return nil, mapWorkDBError(err)
		}
		result = append(result, task)
	}
	if err := rows.Err(); err != nil {
		return nil, mapWorkDBError(err)
	}
	return result, nil
}

func (r *PostgresRepository) CreateDecision(ctx context.Context, scope Scope, decision Decision, mutation Mutation) (Decision, error) {
	if err := mutation.Validate(OperationDecisionCreate); err != nil {
		return Decision{}, err
	}
	tx, replay, err := r.beginMutation(ctx, scope, OperationDecisionCreate, mutation)
	if err != nil {
		return Decision{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if replay != nil {
		var result Decision
		if err := json.Unmarshal(replay, &result); err != nil {
			return Decision{}, fmt.Errorf("decode idempotent decision result: %w", err)
		}
		return result, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO decisions
			(id, workspace_id, owner_user_id, project_id, title, context, outcome, rationale, status, origin, version, created_at, updated_at)
		VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9,'canonical',$10,$11,$12)
	`, decision.ID, scope.WorkspaceID, scope.UserID, decision.ProjectID, decision.Title, decision.Context, decision.Outcome, decision.Rationale, string(decision.Status), positiveVersion(decision.Version), decision.CreatedAt, decision.UpdatedAt); err != nil {
		return Decision{}, mapWorkInsertError(err, EntityDecision, decision.ID)
	}
	decision.Version = positiveVersion(decision.Version)
	if err := appendHistory(ctx, tx, scope, EntityDecision, decision.ID, HistoryCreated, 0, decision.Version, mutation.IdempotencyKey, nil, decision, mutation.At); err != nil {
		return Decision{}, err
	}
	if err := rememberMutation(ctx, tx, scope, mutation, OperationDecisionCreate, EntityDecision, decision.ID, decision); err != nil {
		return Decision{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Decision{}, mapWorkDBError(err)
	}
	return decision, nil
}

func (r *PostgresRepository) GetDecision(ctx context.Context, scope Scope, id string) (Decision, error) {
	var decision Decision
	err := r.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, owner_user_id, COALESCE(project_id,''), title, context,
		       outcome, rationale, status, origin, version, created_at, updated_at, deleted_at
		FROM decisions
		WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3 AND deleted_at IS NULL
	`, scope.WorkspaceID, scope.UserID, id).Scan(&decision.ID, &decision.WorkspaceID, &decision.OwnerUserID, &decision.ProjectID, &decision.Title, &decision.Context, &decision.Outcome, &decision.Rationale, &decision.Status, &decision.Origin, &decision.Version, &decision.CreatedAt, &decision.UpdatedAt, &decision.DeletedAt)
	if err != nil {
		return Decision{}, mapWorkReadError(err, EntityDecision, id)
	}
	return decision, nil
}

func (r *PostgresRepository) ListDecisions(ctx context.Context, scope Scope, projectID string, options ListOptions) ([]Decision, error) {
	options, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	query := `
		SELECT id, workspace_id, owner_user_id, COALESCE(project_id,''), title, context,
		       outcome, rationale, status, origin, version, created_at, updated_at, deleted_at
		FROM decisions
		WHERE workspace_id=$1 AND owner_user_id=$2
	`
	args := []any{scope.WorkspaceID, scope.UserID}
	if projectID != "" {
		query += " AND project_id=$3"
		args = append(args, projectID)
	}
	if !options.IncludeTrashed {
		query += " AND deleted_at IS NULL"
	}
	query += fmt.Sprintf(" ORDER BY created_at, id LIMIT $%d", len(args)+1)
	args = append(args, options.Limit)
	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapWorkDBError(err)
	}
	defer rows.Close()
	result := make([]Decision, 0)
	for rows.Next() {
		var decision Decision
		if err := rows.Scan(&decision.ID, &decision.WorkspaceID, &decision.OwnerUserID, &decision.ProjectID, &decision.Title, &decision.Context, &decision.Outcome, &decision.Rationale, &decision.Status, &decision.Origin, &decision.Version, &decision.CreatedAt, &decision.UpdatedAt, &decision.DeletedAt); err != nil {
			return nil, mapWorkDBError(err)
		}
		result = append(result, decision)
	}
	if err := rows.Err(); err != nil {
		return nil, mapWorkDBError(err)
	}
	return result, nil
}

func (r *PostgresRepository) UpdateProject(ctx context.Context, scope Scope, id string, patch ProjectPatch, expectedVersion int64, mutation Mutation) (Project, error) {
	if err := mutation.Validate(OperationProjectUpdate); err != nil {
		return Project{}, err
	}
	tx, replay, err := r.beginMutation(ctx, scope, OperationProjectUpdate, mutation)
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if replay != nil {
		var result Project
		if err := json.Unmarshal(replay, &result); err != nil {
			return Project{}, fmt.Errorf("decode idempotent project result: %w", err)
		}
		return result, nil
	}
	if err := validateProjectPatch(patch); err != nil {
		return Project{}, err
	}
	project, err := loadProjectForUpdate(ctx, tx, scope, id)
	if err != nil {
		return Project{}, err
	}
	if err := checkVersion(EntityProject, id, project.Version, expectedVersion); err != nil {
		return Project{}, err
	}
	if project.DeletedAt != nil {
		return Project{}, &StateError{EntityType: EntityProject, EntityID: id, State: "trashed", Cause: ErrRecordTrashed}
	}
	updated := project
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

	sets := []string{"version = version + 1", fmt.Sprintf("updated_at = $%d", 5)}
	args := []any{scope.WorkspaceID, scope.UserID, id, expectedVersion, updated.UpdatedAt}
	parameter := 6
	if patch.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", parameter))
		args = append(args, *patch.Name)
		parameter++
	}
	if patch.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", parameter))
		args = append(args, *patch.Description)
		parameter++
	}
	if patch.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", parameter))
		args = append(args, string(*patch.Status))
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE projects SET %s
		WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3 AND version=$4 AND deleted_at IS NULL
	`, strings.Join(sets, ", ")), args...); err != nil {
		return Project{}, mapWorkDBError(err)
	}
	if err := appendHistory(ctx, tx, scope, EntityProject, id, HistoryUpdated, project.Version, updated.Version, mutation.IdempotencyKey, project, updated, updated.UpdatedAt); err != nil {
		return Project{}, err
	}
	if err := rememberMutation(ctx, tx, scope, mutation, OperationProjectUpdate, EntityProject, id, updated); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Project{}, mapWorkDBError(err)
	}
	return updated, nil
}

func (r *PostgresRepository) TrashProject(ctx context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (Project, error) {
	return r.mutateProjectState(ctx, scope, id, expectedVersion, mutation, OperationProjectTrash, HistoryTrashed, true)
}

func (r *PostgresRepository) RestoreProject(ctx context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (Project, error) {
	return r.mutateProjectState(ctx, scope, id, expectedVersion, mutation, OperationProjectRestore, HistoryRestored, false)
}

func (r *PostgresRepository) PurgeProject(ctx context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (PurgeResult, error) {
	if err := mutation.Validate(OperationProjectPurge); err != nil {
		return PurgeResult{}, err
	}
	tx, replay, err := r.beginMutation(ctx, scope, OperationProjectPurge, mutation)
	if err != nil {
		return PurgeResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if replay != nil {
		var result PurgeResult
		if err := json.Unmarshal(replay, &result); err != nil {
			return PurgeResult{}, fmt.Errorf("decode idempotent purge result: %w", err)
		}
		return result, nil
	}
	project, err := loadProjectForUpdate(ctx, tx, scope, id)
	if err != nil {
		return PurgeResult{}, err
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
	dependents, err := projectDependents(ctx, tx, scope, id)
	if err != nil {
		return PurgeResult{}, err
	}
	if len(dependents) > 0 {
		return PurgeResult{}, &DependencyError{EntityType: EntityProject, EntityID: id, Dependents: dependents}
	}
	if err := appendHistory(ctx, tx, scope, EntityProject, id, HistoryPurged, project.Version, project.Version+1, mutation.IdempotencyKey, project, nil, at); err != nil {
		return PurgeResult{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM projects WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3 AND version=$4 AND deleted_at IS NOT NULL`, scope.WorkspaceID, scope.UserID, id, expectedVersion); err != nil {
		return PurgeResult{}, mapWorkDBError(err)
	}
	result := PurgeResult{EntityType: EntityProject, EntityID: id, PurgedAt: at}
	if err := rememberMutation(ctx, tx, scope, mutation, OperationProjectPurge, EntityProject, id, result); err != nil {
		return PurgeResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PurgeResult{}, mapWorkDBError(err)
	}
	return result, nil
}

func (r *PostgresRepository) UpdateTask(ctx context.Context, scope Scope, id string, patch TaskPatch, expectedVersion int64, mutation Mutation) (Task, error) {
	if err := mutation.Validate(OperationTaskUpdate); err != nil {
		return Task{}, err
	}
	tx, replay, err := r.beginMutation(ctx, scope, OperationTaskUpdate, mutation)
	if err != nil {
		return Task{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if replay != nil {
		var result Task
		if err := json.Unmarshal(replay, &result); err != nil {
			return Task{}, fmt.Errorf("decode idempotent task result: %w", err)
		}
		return result, nil
	}
	if err := validateTaskPatch(patch); err != nil {
		return Task{}, err
	}
	task, err := loadTaskForUpdate(ctx, tx, scope, id)
	if err != nil {
		return Task{}, err
	}
	if err := checkVersion(EntityTask, id, task.Version, expectedVersion); err != nil {
		return Task{}, err
	}
	if task.DeletedAt != nil {
		return Task{}, &StateError{EntityType: EntityTask, EntityID: id, State: "trashed", Cause: ErrRecordTrashed}
	}
	updated := task
	if patch.ProjectID != nil {
		updated.ProjectID = *patch.ProjectID
	}
	if err := validateProjectLinkTx(ctx, tx, scope, updated.ProjectID); err != nil {
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

	sets := []string{"version = version + 1", fmt.Sprintf("updated_at = $%d", 5)}
	args := []any{scope.WorkspaceID, scope.UserID, id, expectedVersion, updated.UpdatedAt}
	parameter := 6
	if patch.ProjectID != nil {
		sets = append(sets, fmt.Sprintf("project_id = NULLIF($%d, '')", parameter))
		args = append(args, *patch.ProjectID)
		parameter++
	}
	if patch.Title != nil {
		sets = append(sets, fmt.Sprintf("title = $%d", parameter))
		args = append(args, *patch.Title)
		parameter++
	}
	if patch.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", parameter))
		args = append(args, *patch.Description)
		parameter++
	}
	if patch.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", parameter))
		args = append(args, string(*patch.Status))
		parameter++
	}
	if patch.Priority != nil {
		sets = append(sets, fmt.Sprintf("priority = $%d", parameter))
		args = append(args, string(*patch.Priority))
		parameter++
	}
	if patch.DueAt != nil {
		sets = append(sets, fmt.Sprintf("due_at = $%d", parameter))
		if *patch.DueAt == nil {
			args = append(args, nil)
		} else {
			args = append(args, (*patch.DueAt).UTC())
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE tasks SET %s
		WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3 AND version=$4 AND deleted_at IS NULL
	`, strings.Join(sets, ", ")), args...); err != nil {
		return Task{}, mapWorkDBError(err)
	}
	if err := appendHistory(ctx, tx, scope, EntityTask, id, HistoryUpdated, task.Version, updated.Version, mutation.IdempotencyKey, task, updated, updated.UpdatedAt); err != nil {
		return Task{}, err
	}
	if err := rememberMutation(ctx, tx, scope, mutation, OperationTaskUpdate, EntityTask, id, updated); err != nil {
		return Task{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, mapWorkDBError(err)
	}
	return updated, nil
}

func (r *PostgresRepository) TrashTask(ctx context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (Task, error) {
	return r.mutateTaskState(ctx, scope, id, expectedVersion, mutation, OperationTaskTrash, HistoryTrashed, true)
}

func (r *PostgresRepository) RestoreTask(ctx context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (Task, error) {
	return r.mutateTaskState(ctx, scope, id, expectedVersion, mutation, OperationTaskRestore, HistoryRestored, false)
}

func (r *PostgresRepository) PurgeTask(ctx context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (PurgeResult, error) {
	return r.purgeSimple(ctx, scope, id, expectedVersion, mutation, EntityTask, OperationTaskPurge)
}

func (r *PostgresRepository) UpdateDecision(ctx context.Context, scope Scope, id string, patch DecisionPatch, expectedVersion int64, mutation Mutation) (Decision, error) {
	if err := mutation.Validate(OperationDecisionUpdate); err != nil {
		return Decision{}, err
	}
	tx, replay, err := r.beginMutation(ctx, scope, OperationDecisionUpdate, mutation)
	if err != nil {
		return Decision{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if replay != nil {
		var result Decision
		if err := json.Unmarshal(replay, &result); err != nil {
			return Decision{}, fmt.Errorf("decode idempotent decision result: %w", err)
		}
		return result, nil
	}
	if err := validateDecisionPatch(patch); err != nil {
		return Decision{}, err
	}
	decision, err := loadDecisionForUpdate(ctx, tx, scope, id)
	if err != nil {
		return Decision{}, err
	}
	if err := checkVersion(EntityDecision, id, decision.Version, expectedVersion); err != nil {
		return Decision{}, err
	}
	if decision.DeletedAt != nil {
		return Decision{}, &StateError{EntityType: EntityDecision, EntityID: id, State: "trashed", Cause: ErrRecordTrashed}
	}
	updated := decision
	if patch.ProjectID != nil {
		updated.ProjectID = *patch.ProjectID
	}
	if err := validateProjectLinkTx(ctx, tx, scope, updated.ProjectID); err != nil {
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

	sets := []string{"version = version + 1", fmt.Sprintf("updated_at = $%d", 5)}
	args := []any{scope.WorkspaceID, scope.UserID, id, expectedVersion, updated.UpdatedAt}
	parameter := 6
	if patch.ProjectID != nil {
		sets = append(sets, fmt.Sprintf("project_id = NULLIF($%d, '')", parameter))
		args = append(args, *patch.ProjectID)
		parameter++
	}
	if patch.Title != nil {
		sets = append(sets, fmt.Sprintf("title = $%d", parameter))
		args = append(args, *patch.Title)
		parameter++
	}
	if patch.Context != nil {
		sets = append(sets, fmt.Sprintf("context = $%d", parameter))
		args = append(args, *patch.Context)
		parameter++
	}
	if patch.Outcome != nil {
		sets = append(sets, fmt.Sprintf("outcome = $%d", parameter))
		args = append(args, *patch.Outcome)
		parameter++
	}
	if patch.Rationale != nil {
		sets = append(sets, fmt.Sprintf("rationale = $%d", parameter))
		args = append(args, *patch.Rationale)
		parameter++
	}
	if patch.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", parameter))
		args = append(args, string(*patch.Status))
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE decisions SET %s
		WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3 AND version=$4 AND deleted_at IS NULL
	`, strings.Join(sets, ", ")), args...); err != nil {
		return Decision{}, mapWorkDBError(err)
	}
	if err := appendHistory(ctx, tx, scope, EntityDecision, id, HistoryUpdated, decision.Version, updated.Version, mutation.IdempotencyKey, decision, updated, updated.UpdatedAt); err != nil {
		return Decision{}, err
	}
	if err := rememberMutation(ctx, tx, scope, mutation, OperationDecisionUpdate, EntityDecision, id, updated); err != nil {
		return Decision{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Decision{}, mapWorkDBError(err)
	}
	return updated, nil
}

func (r *PostgresRepository) TrashDecision(ctx context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (Decision, error) {
	return r.mutateDecisionState(ctx, scope, id, expectedVersion, mutation, OperationDecisionTrash, HistoryTrashed, true)
}

func (r *PostgresRepository) RestoreDecision(ctx context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (Decision, error) {
	return r.mutateDecisionState(ctx, scope, id, expectedVersion, mutation, OperationDecisionRestore, HistoryRestored, false)
}

func (r *PostgresRepository) PurgeDecision(ctx context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation) (PurgeResult, error) {
	return r.purgeSimple(ctx, scope, id, expectedVersion, mutation, EntityDecision, OperationDecisionPurge)
}

func (r *PostgresRepository) mutateProjectState(ctx context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation, operation string, action HistoryAction, trash bool) (Project, error) {
	if err := mutation.Validate(operation); err != nil {
		return Project{}, err
	}
	tx, replay, err := r.beginMutation(ctx, scope, operation, mutation)
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if replay != nil {
		var result Project
		if err := json.Unmarshal(replay, &result); err != nil {
			return Project{}, fmt.Errorf("decode idempotent project result: %w", err)
		}
		return result, nil
	}
	project, err := loadProjectForUpdate(ctx, tx, scope, id)
	if err != nil {
		return Project{}, err
	}
	if err := checkVersion(EntityProject, id, project.Version, expectedVersion); err != nil {
		return Project{}, err
	}
	if trash && project.DeletedAt != nil {
		return Project{}, &StateError{EntityType: EntityProject, EntityID: id, State: "trashed", Cause: ErrAlreadyTrashed}
	}
	if !trash && project.DeletedAt == nil {
		return Project{}, &StateError{EntityType: EntityProject, EntityID: id, State: "active", Cause: ErrNotTrashed}
	}
	updated := project
	at := mutationTime(mutation)
	if trash {
		updated.DeletedAt = &at
	} else {
		updated.DeletedAt = nil
	}
	updated.Version++
	updated.UpdatedAt = at
	if _, err := tx.Exec(ctx, `
		UPDATE projects SET deleted_at=$5, version=version+1, updated_at=$6
		WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3 AND version=$4
	`, scope.WorkspaceID, scope.UserID, id, expectedVersion, updated.DeletedAt, at); err != nil {
		return Project{}, mapWorkDBError(err)
	}
	if err := appendHistory(ctx, tx, scope, EntityProject, id, action, project.Version, updated.Version, mutation.IdempotencyKey, project, updated, at); err != nil {
		return Project{}, err
	}
	if err := rememberMutation(ctx, tx, scope, mutation, operation, EntityProject, id, updated); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Project{}, mapWorkDBError(err)
	}
	return updated, nil
}

func (r *PostgresRepository) mutateTaskState(ctx context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation, operation string, action HistoryAction, trash bool) (Task, error) {
	if err := mutation.Validate(operation); err != nil {
		return Task{}, err
	}
	tx, replay, err := r.beginMutation(ctx, scope, operation, mutation)
	if err != nil {
		return Task{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if replay != nil {
		var result Task
		if err := json.Unmarshal(replay, &result); err != nil {
			return Task{}, fmt.Errorf("decode idempotent task result: %w", err)
		}
		return result, nil
	}
	task, err := loadTaskForUpdate(ctx, tx, scope, id)
	if err != nil {
		return Task{}, err
	}
	if err := checkVersion(EntityTask, id, task.Version, expectedVersion); err != nil {
		return Task{}, err
	}
	if trash && task.DeletedAt != nil {
		return Task{}, &StateError{EntityType: EntityTask, EntityID: id, State: "trashed", Cause: ErrAlreadyTrashed}
	}
	if !trash && task.DeletedAt == nil {
		return Task{}, &StateError{EntityType: EntityTask, EntityID: id, State: "active", Cause: ErrNotTrashed}
	}
	updated := task
	at := mutationTime(mutation)
	if trash {
		updated.DeletedAt = &at
	} else {
		updated.DeletedAt = nil
	}
	updated.Version++
	updated.UpdatedAt = at
	if _, err := tx.Exec(ctx, `
		UPDATE tasks SET deleted_at=$5, version=version+1, updated_at=$6
		WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3 AND version=$4
	`, scope.WorkspaceID, scope.UserID, id, expectedVersion, updated.DeletedAt, at); err != nil {
		return Task{}, mapWorkDBError(err)
	}
	if err := appendHistory(ctx, tx, scope, EntityTask, id, action, task.Version, updated.Version, mutation.IdempotencyKey, task, updated, at); err != nil {
		return Task{}, err
	}
	if err := rememberMutation(ctx, tx, scope, mutation, operation, EntityTask, id, updated); err != nil {
		return Task{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, mapWorkDBError(err)
	}
	return updated, nil
}

func (r *PostgresRepository) mutateDecisionState(ctx context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation, operation string, action HistoryAction, trash bool) (Decision, error) {
	if err := mutation.Validate(operation); err != nil {
		return Decision{}, err
	}
	tx, replay, err := r.beginMutation(ctx, scope, operation, mutation)
	if err != nil {
		return Decision{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if replay != nil {
		var result Decision
		if err := json.Unmarshal(replay, &result); err != nil {
			return Decision{}, fmt.Errorf("decode idempotent decision result: %w", err)
		}
		return result, nil
	}
	decision, err := loadDecisionForUpdate(ctx, tx, scope, id)
	if err != nil {
		return Decision{}, err
	}
	if err := checkVersion(EntityDecision, id, decision.Version, expectedVersion); err != nil {
		return Decision{}, err
	}
	if trash && decision.DeletedAt != nil {
		return Decision{}, &StateError{EntityType: EntityDecision, EntityID: id, State: "trashed", Cause: ErrAlreadyTrashed}
	}
	if !trash && decision.DeletedAt == nil {
		return Decision{}, &StateError{EntityType: EntityDecision, EntityID: id, State: "active", Cause: ErrNotTrashed}
	}
	updated := decision
	at := mutationTime(mutation)
	if trash {
		updated.DeletedAt = &at
	} else {
		updated.DeletedAt = nil
	}
	updated.Version++
	updated.UpdatedAt = at
	if _, err := tx.Exec(ctx, `
		UPDATE decisions SET deleted_at=$5, version=version+1, updated_at=$6
		WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3 AND version=$4
	`, scope.WorkspaceID, scope.UserID, id, expectedVersion, updated.DeletedAt, at); err != nil {
		return Decision{}, mapWorkDBError(err)
	}
	if err := appendHistory(ctx, tx, scope, EntityDecision, id, action, decision.Version, updated.Version, mutation.IdempotencyKey, decision, updated, at); err != nil {
		return Decision{}, err
	}
	if err := rememberMutation(ctx, tx, scope, mutation, operation, EntityDecision, id, updated); err != nil {
		return Decision{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Decision{}, mapWorkDBError(err)
	}
	return updated, nil
}

func (r *PostgresRepository) purgeSimple(ctx context.Context, scope Scope, id string, expectedVersion int64, mutation Mutation, entityType EntityType, operation string) (PurgeResult, error) {
	if err := mutation.Validate(operation); err != nil {
		return PurgeResult{}, err
	}
	tx, replay, err := r.beginMutation(ctx, scope, operation, mutation)
	if err != nil {
		return PurgeResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if replay != nil {
		var result PurgeResult
		if err := json.Unmarshal(replay, &result); err != nil {
			return PurgeResult{}, fmt.Errorf("decode idempotent purge result: %w", err)
		}
		return result, nil
	}
	var before any
	var deletedAt *time.Time
	var version int64
	switch entityType {
	case EntityTask:
		task, loadErr := loadTaskForUpdate(ctx, tx, scope, id)
		if loadErr != nil {
			return PurgeResult{}, loadErr
		}
		before, deletedAt, version = task, task.DeletedAt, task.Version
	case EntityDecision:
		decision, loadErr := loadDecisionForUpdate(ctx, tx, scope, id)
		if loadErr != nil {
			return PurgeResult{}, loadErr
		}
		before, deletedAt, version = decision, decision.DeletedAt, decision.Version
	default:
		return PurgeResult{}, &ValidationError{Field: "entityType", Message: "must be task or decision"}
	}
	if err := checkVersion(entityType, id, version, expectedVersion); err != nil {
		return PurgeResult{}, err
	}
	if deletedAt == nil {
		return PurgeResult{}, &StateError{EntityType: entityType, EntityID: id, State: "active", Cause: ErrNotTrashed}
	}
	at := mutationTime(mutation)
	if at.Before(deletedAt.Add(RetentionPeriod)) {
		return PurgeResult{}, &PurgeNotReadyError{EntityType: entityType, EntityID: id, EligibleAt: deletedAt.Add(RetentionPeriod).UTC().Format(time.RFC3339)}
	}
	if err := appendHistory(ctx, tx, scope, entityType, id, HistoryPurged, version, version+1, mutation.IdempotencyKey, before, nil, at); err != nil {
		return PurgeResult{}, err
	}
	table := map[EntityType]string{EntityTask: "tasks", EntityDecision: "decisions"}[entityType]
	if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3 AND version=$4 AND deleted_at IS NOT NULL", table), scope.WorkspaceID, scope.UserID, id, expectedVersion); err != nil {
		return PurgeResult{}, mapWorkDBError(err)
	}
	result := PurgeResult{EntityType: entityType, EntityID: id, PurgedAt: at}
	if err := rememberMutation(ctx, tx, scope, mutation, operation, entityType, id, result); err != nil {
		return PurgeResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PurgeResult{}, mapWorkDBError(err)
	}
	return result, nil
}

func loadProjectForUpdate(ctx context.Context, tx pgx.Tx, scope Scope, id string) (Project, error) {
	var project Project
	err := tx.QueryRow(ctx, `
		SELECT id, workspace_id, owner_user_id, name, description, status, origin,
		       version, created_at, updated_at, deleted_at
		FROM projects
		WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3
		FOR UPDATE
	`, scope.WorkspaceID, scope.UserID, id).Scan(&project.ID, &project.WorkspaceID, &project.OwnerUserID, &project.Name, &project.Description, &project.Status, &project.Origin, &project.Version, &project.CreatedAt, &project.UpdatedAt, &project.DeletedAt)
	if err != nil {
		return Project{}, mapWorkReadError(err, EntityProject, id)
	}
	return project, nil
}

func loadTaskForUpdate(ctx context.Context, tx pgx.Tx, scope Scope, id string) (Task, error) {
	var task Task
	err := tx.QueryRow(ctx, `
		SELECT id, workspace_id, owner_user_id, COALESCE(project_id,''), title, description,
		       status, priority, due_at, origin, version, created_at, updated_at, deleted_at
		FROM tasks
		WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3
		FOR UPDATE
	`, scope.WorkspaceID, scope.UserID, id).Scan(&task.ID, &task.WorkspaceID, &task.OwnerUserID, &task.ProjectID, &task.Title, &task.Description, &task.Status, &task.Priority, &task.DueAt, &task.Origin, &task.Version, &task.CreatedAt, &task.UpdatedAt, &task.DeletedAt)
	if err != nil {
		return Task{}, mapWorkReadError(err, EntityTask, id)
	}
	return task, nil
}

func loadDecisionForUpdate(ctx context.Context, tx pgx.Tx, scope Scope, id string) (Decision, error) {
	var decision Decision
	err := tx.QueryRow(ctx, `
		SELECT id, workspace_id, owner_user_id, COALESCE(project_id,''), title, context,
		       outcome, rationale, status, origin, version, created_at, updated_at, deleted_at
		FROM decisions
		WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3
		FOR UPDATE
	`, scope.WorkspaceID, scope.UserID, id).Scan(&decision.ID, &decision.WorkspaceID, &decision.OwnerUserID, &decision.ProjectID, &decision.Title, &decision.Context, &decision.Outcome, &decision.Rationale, &decision.Status, &decision.Origin, &decision.Version, &decision.CreatedAt, &decision.UpdatedAt, &decision.DeletedAt)
	if err != nil {
		return Decision{}, mapWorkReadError(err, EntityDecision, id)
	}
	return decision, nil
}

func validateProjectLinkTx(ctx context.Context, tx pgx.Tx, scope Scope, projectID string) error {
	if projectID == "" {
		return nil
	}
	var deletedAt *time.Time
	err := tx.QueryRow(ctx, `
		SELECT deleted_at FROM projects
		WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3
	`, scope.WorkspaceID, scope.UserID, projectID).Scan(&deletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &NotFoundError{EntityType: EntityProject, EntityID: projectID}
	}
	if err != nil {
		return mapWorkDBError(err)
	}
	if deletedAt != nil {
		return &StateError{EntityType: EntityProject, EntityID: projectID, State: "trashed", Cause: ErrRecordTrashed}
	}
	return nil
}

func projectDependents(ctx context.Context, tx pgx.Tx, scope Scope, projectID string) ([]EntityType, error) {
	dependents := make([]EntityType, 0, 2)
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE workspace_id=$1 AND owner_user_id=$2 AND project_id=$3`, scope.WorkspaceID, scope.UserID, projectID).Scan(&count); err != nil {
		return nil, mapWorkDBError(err)
	}
	if count > 0 {
		dependents = append(dependents, EntityTask)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM decisions WHERE workspace_id=$1 AND owner_user_id=$2 AND project_id=$3`, scope.WorkspaceID, scope.UserID, projectID).Scan(&count); err != nil {
		return nil, mapWorkDBError(err)
	}
	if count > 0 {
		dependents = append(dependents, EntityDecision)
	}
	return dependents, nil
}

func (r *PostgresRepository) History(ctx context.Context, scope Scope, entityType EntityType, id string) ([]HistoryEvent, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT id, workspace_id, actor_user_id, entity_type, entity_id, action,
		       from_version, to_version, idempotency_key, before_snapshot, after_snapshot, created_at
		FROM work_history
		WHERE workspace_id=$1 AND actor_user_id=$2 AND entity_type=$3 AND entity_id=$4
		ORDER BY created_at, id
	`, scope.WorkspaceID, scope.UserID, string(entityType), id)
	if err != nil {
		return nil, mapWorkDBError(err)
	}
	defer rows.Close()
	result := make([]HistoryEvent, 0)
	for rows.Next() {
		var event HistoryEvent
		var actor *string
		if err := rows.Scan(&event.ID, &event.WorkspaceID, &actor, &event.EntityType, &event.EntityID, &event.Action, &event.FromVersion, &event.ToVersion, &event.IdempotencyKey, &event.Before, &event.After, &event.CreatedAt); err != nil {
			return nil, mapWorkDBError(err)
		}
		event.ActorUserID = scope.UserID
		if actor != nil {
			event.ActorUserID = *actor
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, mapWorkDBError(err)
	}
	return result, nil
}

type mutationReplay struct {
	Operation   string
	RequestHash string
	Result      []byte
}

func (r *PostgresRepository) beginMutation(ctx context.Context, scope Scope, operation string, mutation Mutation) (pgx.Tx, []byte, error) {
	if err := scope.Validate(); err != nil {
		return nil, nil, err
	}
	if err := mutation.Validate(operation); err != nil {
		return nil, nil, err
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, mapWorkDBError(err)
	}
	lockDigest := sha256Sum(scope.WorkspaceID + "\x00" + scope.UserID + "\x00" + mutation.IdempotencyKey)
	key := fmt.Sprintf("%x", lockDigest[:])
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", key); err != nil {
		_ = tx.Rollback(ctx)
		return nil, nil, mapWorkDBError(err)
	}
	var replay mutationReplay
	err = tx.QueryRow(ctx, `
		SELECT operation, request_hash, result
		FROM work_idempotency
		WHERE workspace_id=$1 AND user_id=$2 AND idempotency_key=$3
		FOR UPDATE
	`, scope.WorkspaceID, scope.UserID, mutation.IdempotencyKey).Scan(&replay.Operation, &replay.RequestHash, &replay.Result)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx, nil, nil
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, nil, mapWorkDBError(err)
	}
	if replay.Operation != operation || replay.RequestHash != mutation.RequestHash {
		_ = tx.Rollback(ctx)
		return nil, nil, &IdempotencyConflictError{Key: mutation.IdempotencyKey, ExistingOperation: replay.Operation, RequestedOperation: operation}
	}
	return tx, append([]byte(nil), replay.Result...), nil
}

func appendHistory(ctx context.Context, tx pgx.Tx, scope Scope, entityType EntityType, entityID string, action HistoryAction, fromVersion, toVersion int64, idempotencyKey string, before, after any, at time.Time) error {
	beforeRaw, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("encode work history before: %w", err)
	}
	afterRaw, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode work history after: %w", err)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	historyID := fmt.Sprintf("work-history-%x", stableHistoryHash(scope, entityType, entityID, idempotencyKey))
	_, err = tx.Exec(ctx, `
		INSERT INTO work_history
			(id, workspace_id, actor_user_id, entity_type, entity_id, action, from_version, to_version, idempotency_key, before_snapshot, after_snapshot, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11::jsonb,$12)
	`, historyID, scope.WorkspaceID, scope.UserID, string(entityType), entityID, string(action), fromVersion, toVersion, idempotencyKey, beforeRaw, afterRaw, at.UTC())
	return mapWorkDBError(err)
}

func rememberMutation(ctx context.Context, tx pgx.Tx, scope Scope, mutation Mutation, operation string, entityType EntityType, entityID string, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode work idempotency result: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO work_idempotency
			(workspace_id, user_id, idempotency_key, operation, request_hash, entity_type, entity_id, result)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb)
	`, scope.WorkspaceID, scope.UserID, mutation.IdempotencyKey, operation, mutation.RequestHash, string(entityType), entityID, raw)
	return mapWorkDBError(err)
}

func stableHistoryHash(scope Scope, entityType EntityType, entityID, key string) [32]byte {
	value := scope.WorkspaceID + "\x00" + scope.UserID + "\x00" + string(entityType) + "\x00" + entityID + "\x00" + key
	return sha256Sum(value)
}

func sha256Sum(value string) [32]byte {
	return sha256.Sum256([]byte(value))
}

func positiveVersion(value int64) int64 {
	if value < 1 {
		return 1
	}
	return value
}

func mapWorkInsertError(err error, entityType EntityType, entityID string) error {
	if err == nil {
		return nil
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "23505":
			return ErrAlreadyExists
		case "23503":
			return &NotFoundError{EntityType: entityType, EntityID: entityID}
		case "23514":
			return &ValidationError{Field: string(entityType), Message: "violates a persistence constraint"}
		}
	}
	return mapWorkDBError(err)
}

func mapWorkReadError(err error, entityType EntityType, entityID string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return &NotFoundError{EntityType: entityType, EntityID: entityID}
	}
	return mapWorkDBError(err)
}

func mapWorkDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "23505":
			return ErrAlreadyExists
		case "23503":
			return ErrNotFound
		case "23514":
			return &ValidationError{Field: "persistence", Message: "violates a persistence constraint"}
		}
	}
	return errors.New("work persistence failed")
}
