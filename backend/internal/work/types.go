package work

import (
	"encoding/json"
	"strings"
	"time"
)

// RetentionPeriod is the minimum age of a soft-deleted record before an
// explicit purge may remove it permanently.
const RetentionPeriod = 30 * 24 * time.Hour

const (
	EntityProject  EntityType = "project"
	EntityTask     EntityType = "task"
	EntityDecision EntityType = "decision"
)

type EntityType string

type DataOrigin string

const (
	// OriginCanonical marks data that is owned by the application and may be
	// used as a source of truth by downstream services.
	OriginCanonical DataOrigin = "canonical"
	// OriginInferred marks model-generated or otherwise unverified content. It
	// is deliberately represented by a separate type and never accepted by the
	// canonical Project, Task, or Decision mutation methods.
	OriginInferred DataOrigin = "inferred"
)

type ProjectStatus string

const (
	ProjectActive    ProjectStatus = "active"
	ProjectOnHold    ProjectStatus = "on_hold"
	ProjectCompleted ProjectStatus = "completed"
	ProjectArchived  ProjectStatus = "archived"
)

type TaskStatus string

const (
	TaskBacklog    TaskStatus = "backlog"
	TaskTodo       TaskStatus = "todo"
	TaskInProgress TaskStatus = "in_progress"
	TaskBlocked    TaskStatus = "blocked"
	TaskDone       TaskStatus = "done"
	TaskCancelled  TaskStatus = "cancelled"
)

type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityNormal TaskPriority = "normal"
	PriorityHigh   TaskPriority = "high"
	PriorityUrgent TaskPriority = "urgent"
)

type DecisionStatus string

const (
	DecisionProposed   DecisionStatus = "proposed"
	DecisionAccepted   DecisionStatus = "accepted"
	DecisionRejected   DecisionStatus = "rejected"
	DecisionSuperseded DecisionStatus = "superseded"
)

type HistoryAction string

const (
	HistoryCreated  HistoryAction = "created"
	HistoryUpdated  HistoryAction = "updated"
	HistoryTrashed  HistoryAction = "trashed"
	HistoryRestored HistoryAction = "restored"
	HistoryPurged   HistoryAction = "purged"
)

// Scope is deliberately explicit instead of being inferred from an entity
// ID. Every repository operation must carry both workspace and user scope.
type Scope struct {
	WorkspaceID string `json:"workspaceId"`
	UserID      string `json:"userId"`
}

type Project struct {
	ID          string        `json:"id"`
	WorkspaceID string        `json:"workspaceId"`
	OwnerUserID string        `json:"ownerUserId"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Status      ProjectStatus `json:"status"`
	Origin      DataOrigin    `json:"origin"`
	Version     int64         `json:"version"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	DeletedAt   *time.Time    `json:"deletedAt,omitempty"`
}

type Task struct {
	ID          string       `json:"id"`
	WorkspaceID string       `json:"workspaceId"`
	OwnerUserID string       `json:"ownerUserId"`
	ProjectID   string       `json:"projectId,omitempty"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      TaskStatus   `json:"status"`
	Priority    TaskPriority `json:"priority"`
	DueAt       *time.Time   `json:"dueAt,omitempty"`
	Origin      DataOrigin   `json:"origin"`
	Version     int64        `json:"version"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
	DeletedAt   *time.Time   `json:"deletedAt,omitempty"`
}

type Decision struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspaceId"`
	OwnerUserID string         `json:"ownerUserId"`
	ProjectID   string         `json:"projectId,omitempty"`
	Title       string         `json:"title"`
	Context     string         `json:"context"`
	Outcome     string         `json:"outcome"`
	Rationale   string         `json:"rationale"`
	Status      DecisionStatus `json:"status"`
	Origin      DataOrigin     `json:"origin"`
	Version     int64          `json:"version"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   *time.Time     `json:"deletedAt,omitempty"`
}

type CreateProjectInput struct {
	ID          string        `json:"id,omitempty"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Status      ProjectStatus `json:"status,omitempty"`
}

type CreateTaskInput struct {
	ID          string       `json:"id,omitempty"`
	ProjectID   string       `json:"projectId,omitempty"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      TaskStatus   `json:"status,omitempty"`
	Priority    TaskPriority `json:"priority,omitempty"`
	DueAt       *time.Time   `json:"dueAt,omitempty"`
}

type CreateDecisionInput struct {
	ID        string         `json:"id,omitempty"`
	ProjectID string         `json:"projectId,omitempty"`
	Title     string         `json:"title"`
	Context   string         `json:"context"`
	Outcome   string         `json:"outcome"`
	Rationale string         `json:"rationale"`
	Status    DecisionStatus `json:"status,omitempty"`
}

// Patches use pointers so an empty description or a cleared project link can
// be distinguished from a field that was not part of the update.
type ProjectPatch struct {
	Name        *string        `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	Status      *ProjectStatus `json:"status,omitempty"`
}

type TaskPatch struct {
	ProjectID   *string       `json:"projectId,omitempty"`
	Title       *string       `json:"title,omitempty"`
	Description *string       `json:"description,omitempty"`
	Status      *TaskStatus   `json:"status,omitempty"`
	Priority    *TaskPriority `json:"priority,omitempty"`
	DueAt       **time.Time   `json:"dueAt,omitempty"`
}

type DecisionPatch struct {
	ProjectID *string         `json:"projectId,omitempty"`
	Title     *string         `json:"title,omitempty"`
	Context   *string         `json:"context,omitempty"`
	Outcome   *string         `json:"outcome,omitempty"`
	Rationale *string         `json:"rationale,omitempty"`
	Status    *DecisionStatus `json:"status,omitempty"`
}

type ListOptions struct {
	IncludeTrashed bool `json:"includeTrashed"`
	Limit          int  `json:"limit"`
}

type HistoryEvent struct {
	ID             string          `json:"id"`
	WorkspaceID    string          `json:"workspaceId"`
	ActorUserID    string          `json:"actorUserId"`
	EntityType     EntityType      `json:"entityType"`
	EntityID       string          `json:"entityId"`
	Action         HistoryAction   `json:"action"`
	FromVersion    int64           `json:"fromVersion"`
	ToVersion      int64           `json:"toVersion"`
	IdempotencyKey string          `json:"idempotencyKey"`
	Before         json.RawMessage `json:"before"`
	After          json.RawMessage `json:"after"`
	CreatedAt      time.Time       `json:"createdAt"`
}

// InferredWorkContent is intentionally not assignable to any canonical work
// entity. Assistant suggestions can be persisted by a future assistant
// module, but must be reviewed and converted into an explicit canonical
// mutation before entering this package's Project/Task/Decision repository.
type InferredWorkContent struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspaceId"`
	OwnerUserID string     `json:"ownerUserId"`
	EntityType  EntityType `json:"entityType"`
	EntityID    string     `json:"entityId,omitempty"`
	Content     string     `json:"content"`
	Model       string     `json:"model"`
	Evidence    []string   `json:"evidence,omitempty"`
	Origin      DataOrigin `json:"origin"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type PurgeResult struct {
	EntityType EntityType `json:"entityType"`
	EntityID   string     `json:"entityId"`
	PurgedAt   time.Time  `json:"purgedAt"`
}

func (s Scope) Validate() error {
	if err := validateIdentifier("workspaceId", s.WorkspaceID); err != nil {
		return err
	}
	return validateIdentifier("userId", s.UserID)
}

func (o ListOptions) Normalize() (ListOptions, error) {
	if o.Limit == 0 {
		o.Limit = 50
	}
	if o.Limit < 1 || o.Limit > 200 {
		return ListOptions{}, &ValidationError{Field: "limit", Message: "must be between 1 and 200"}
	}
	return o, nil
}

func (c InferredWorkContent) Validate() error {
	if err := (Scope{WorkspaceID: c.WorkspaceID, UserID: c.OwnerUserID}).Validate(); err != nil {
		return err
	}
	if c.Origin != OriginInferred {
		return &ValidationError{Field: "origin", Message: "inferred content must have origin=inferred"}
	}
	if c.EntityType != EntityProject && c.EntityType != EntityTask && c.EntityType != EntityDecision {
		return &ValidationError{Field: "entityType", Message: "must be project, task, or decision"}
	}
	if strings.TrimSpace(c.Content) == "" {
		return &ValidationError{Field: "content", Message: "is required"}
	}
	return nil
}
