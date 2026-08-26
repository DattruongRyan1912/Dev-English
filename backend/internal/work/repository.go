package work

import (
	"context"
	"time"
)

// Mutation is passed to every write method. Repository implementations must
// atomically apply the mutation and remember the key/result, so a retry with
// the same key cannot create a second history event.
type Mutation struct {
	IdempotencyKey string
	RequestHash    string
	At             time.Time
}

const (
	OperationProjectCreate   = "project.create"
	OperationProjectUpdate   = "project.update"
	OperationProjectTrash    = "project.trash"
	OperationProjectRestore  = "project.restore"
	OperationProjectPurge    = "project.purge"
	OperationTaskCreate      = "task.create"
	OperationTaskUpdate      = "task.update"
	OperationTaskTrash       = "task.trash"
	OperationTaskRestore     = "task.restore"
	OperationTaskPurge       = "task.purge"
	OperationDecisionCreate  = "decision.create"
	OperationDecisionUpdate  = "decision.update"
	OperationDecisionTrash   = "decision.trash"
	OperationDecisionRestore = "decision.restore"
	OperationDecisionPurge   = "decision.purge"
)

func (m Mutation) Validate(operation string) error {
	if err := ValidateIdempotencyKey(m.IdempotencyKey); err != nil {
		return err
	}
	if len(m.RequestHash) != 64 {
		return &ValidationError{Field: "requestHash", Message: "must be a SHA-256 hex digest"}
	}
	for _, r := range m.RequestHash {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return &ValidationError{Field: "requestHash", Message: "must be a SHA-256 hex digest"}
		}
	}
	if operation == "" {
		return &ValidationError{Field: "operation", Message: "is required"}
	}
	return nil
}

type Repository interface {
	ProjectRepository
	TaskRepository
	DecisionRepository
	HistoryRepository
}

type ProjectRepository interface {
	CreateProject(context.Context, Scope, Project, Mutation) (Project, error)
	GetProject(context.Context, Scope, string) (Project, error)
	ListProjects(context.Context, Scope, ListOptions) ([]Project, error)
	UpdateProject(context.Context, Scope, string, ProjectPatch, int64, Mutation) (Project, error)
	TrashProject(context.Context, Scope, string, int64, Mutation) (Project, error)
	RestoreProject(context.Context, Scope, string, int64, Mutation) (Project, error)
	PurgeProject(context.Context, Scope, string, int64, Mutation) (PurgeResult, error)
}

type TaskRepository interface {
	CreateTask(context.Context, Scope, Task, Mutation) (Task, error)
	GetTask(context.Context, Scope, string) (Task, error)
	ListTasks(context.Context, Scope, string, ListOptions) ([]Task, error)
	UpdateTask(context.Context, Scope, string, TaskPatch, int64, Mutation) (Task, error)
	TrashTask(context.Context, Scope, string, int64, Mutation) (Task, error)
	RestoreTask(context.Context, Scope, string, int64, Mutation) (Task, error)
	PurgeTask(context.Context, Scope, string, int64, Mutation) (PurgeResult, error)
}

type DecisionRepository interface {
	CreateDecision(context.Context, Scope, Decision, Mutation) (Decision, error)
	GetDecision(context.Context, Scope, string) (Decision, error)
	ListDecisions(context.Context, Scope, string, ListOptions) ([]Decision, error)
	UpdateDecision(context.Context, Scope, string, DecisionPatch, int64, Mutation) (Decision, error)
	TrashDecision(context.Context, Scope, string, int64, Mutation) (Decision, error)
	RestoreDecision(context.Context, Scope, string, int64, Mutation) (Decision, error)
	PurgeDecision(context.Context, Scope, string, int64, Mutation) (PurgeResult, error)
}

// History is append-only from the service consumer's perspective. Concrete
// repositories append it in the same transaction as the entity mutation and
// expose it for audit/history views.
type HistoryRepository interface {
	History(context.Context, Scope, EntityType, string) ([]HistoryEvent, error)
}
