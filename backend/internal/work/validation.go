package work

import (
	"strings"
)

const (
	maxIDBytes          = 200
	maxNameBytes        = 200
	maxTitleBytes       = 500
	maxDescriptionBytes = 20000
	maxDecisionBytes    = 20000
	maxIdempotencyBytes = 128
)

func ValidateIdempotencyKey(key string) error {
	if key == "" {
		return &ValidationError{Field: "idempotencyKey", Message: "is required"}
	}
	if key != strings.TrimSpace(key) {
		return &ValidationError{Field: "idempotencyKey", Message: "must not have leading or trailing whitespace"}
	}
	if len(key) > maxIdempotencyBytes {
		return &ValidationError{Field: "idempotencyKey", Message: "must be at most 128 bytes"}
	}
	for _, r := range key {
		if r < 33 || r > 126 {
			return &ValidationError{Field: "idempotencyKey", Message: "must contain printable ASCII characters only"}
		}
	}
	return nil
}

func validateEntityType(entityType EntityType) error {
	switch entityType {
	case EntityProject, EntityTask, EntityDecision:
		return nil
	default:
		return &ValidationError{Field: "entityType", Message: "must be project, task, or decision"}
	}
}

func validateEntityID(field, id string) error {
	if err := validateIdentifier(field, id); err != nil {
		return err
	}
	if len(id) > maxIDBytes {
		return &ValidationError{Field: field, Message: "must be at most 200 bytes"}
	}
	return nil
}

func validateVersion(expected int64) error {
	if expected < 1 {
		return &ValidationError{Field: "expectedVersion", Message: "must be at least 1"}
	}
	return nil
}

func validateProjectCreate(input CreateProjectInput) (CreateProjectInput, error) {
	if input.ID != "" {
		if err := validateEntityID("id", input.ID); err != nil {
			return CreateProjectInput{}, err
		}
	}
	if err := validateText("name", input.Name, true, maxNameBytes); err != nil {
		return CreateProjectInput{}, err
	}
	if err := validateText("description", input.Description, false, maxDescriptionBytes); err != nil {
		return CreateProjectInput{}, err
	}
	if input.Status == "" {
		input.Status = ProjectActive
	}
	if !validProjectStatus(input.Status) {
		return CreateProjectInput{}, &ValidationError{Field: "status", Message: "must be active, on_hold, completed, or archived"}
	}
	return input, nil
}

func validateProjectPatch(patch ProjectPatch) error {
	if patch.Name == nil && patch.Description == nil && patch.Status == nil {
		return &ValidationError{Field: "patch", Message: "must change at least one field"}
	}
	if patch.Name != nil {
		if err := validateText("name", *patch.Name, true, maxNameBytes); err != nil {
			return err
		}
	}
	if patch.Description != nil {
		if err := validateText("description", *patch.Description, false, maxDescriptionBytes); err != nil {
			return err
		}
	}
	if patch.Status != nil && !validProjectStatus(*patch.Status) {
		return &ValidationError{Field: "status", Message: "must be active, on_hold, completed, or archived"}
	}
	return nil
}

func validateTaskCreate(input CreateTaskInput) (CreateTaskInput, error) {
	if input.ID != "" {
		if err := validateEntityID("id", input.ID); err != nil {
			return CreateTaskInput{}, err
		}
	}
	if input.ProjectID != "" {
		if err := validateEntityID("projectId", input.ProjectID); err != nil {
			return CreateTaskInput{}, err
		}
	}
	if err := validateText("title", input.Title, true, maxTitleBytes); err != nil {
		return CreateTaskInput{}, err
	}
	if err := validateText("description", input.Description, false, maxDescriptionBytes); err != nil {
		return CreateTaskInput{}, err
	}
	if input.Status == "" {
		input.Status = TaskTodo
	}
	if !validTaskStatus(input.Status) {
		return CreateTaskInput{}, &ValidationError{Field: "status", Message: "has an unsupported task status"}
	}
	if input.Priority == "" {
		input.Priority = PriorityNormal
	}
	if !validTaskPriority(input.Priority) {
		return CreateTaskInput{}, &ValidationError{Field: "priority", Message: "must be low, normal, high, or urgent"}
	}
	return input, nil
}

func validateTaskPatch(patch TaskPatch) error {
	if patch.ProjectID == nil && patch.Title == nil && patch.Description == nil && patch.Status == nil && patch.Priority == nil && patch.DueAt == nil {
		return &ValidationError{Field: "patch", Message: "must change at least one field"}
	}
	if patch.ProjectID != nil && *patch.ProjectID != "" {
		if err := validateEntityID("projectId", *patch.ProjectID); err != nil {
			return err
		}
	}
	if patch.Title != nil {
		if err := validateText("title", *patch.Title, true, maxTitleBytes); err != nil {
			return err
		}
	}
	if patch.Description != nil {
		if err := validateText("description", *patch.Description, false, maxDescriptionBytes); err != nil {
			return err
		}
	}
	if patch.Status != nil && !validTaskStatus(*patch.Status) {
		return &ValidationError{Field: "status", Message: "has an unsupported task status"}
	}
	if patch.Priority != nil && !validTaskPriority(*patch.Priority) {
		return &ValidationError{Field: "priority", Message: "must be low, normal, high, or urgent"}
	}
	return nil
}

func validateDecisionCreate(input CreateDecisionInput) (CreateDecisionInput, error) {
	if input.ID != "" {
		if err := validateEntityID("id", input.ID); err != nil {
			return CreateDecisionInput{}, err
		}
	}
	if input.ProjectID != "" {
		if err := validateEntityID("projectId", input.ProjectID); err != nil {
			return CreateDecisionInput{}, err
		}
	}
	if err := validateText("title", input.Title, true, maxTitleBytes); err != nil {
		return CreateDecisionInput{}, err
	}
	if err := validateText("context", input.Context, false, maxDecisionBytes); err != nil {
		return CreateDecisionInput{}, err
	}
	if err := validateText("outcome", input.Outcome, true, maxDecisionBytes); err != nil {
		return CreateDecisionInput{}, err
	}
	if err := validateText("rationale", input.Rationale, false, maxDecisionBytes); err != nil {
		return CreateDecisionInput{}, err
	}
	if input.Status == "" {
		input.Status = DecisionProposed
	}
	if !validDecisionStatus(input.Status) {
		return CreateDecisionInput{}, &ValidationError{Field: "status", Message: "must be proposed, accepted, rejected, or superseded"}
	}
	return input, nil
}

func validateDecisionPatch(patch DecisionPatch) error {
	if patch.ProjectID == nil && patch.Title == nil && patch.Context == nil && patch.Outcome == nil && patch.Rationale == nil && patch.Status == nil {
		return &ValidationError{Field: "patch", Message: "must change at least one field"}
	}
	if patch.ProjectID != nil && *patch.ProjectID != "" {
		if err := validateEntityID("projectId", *patch.ProjectID); err != nil {
			return err
		}
	}
	if patch.Title != nil {
		if err := validateText("title", *patch.Title, true, maxTitleBytes); err != nil {
			return err
		}
	}
	if patch.Context != nil {
		if err := validateText("context", *patch.Context, false, maxDecisionBytes); err != nil {
			return err
		}
	}
	if patch.Outcome != nil {
		if err := validateText("outcome", *patch.Outcome, true, maxDecisionBytes); err != nil {
			return err
		}
	}
	if patch.Rationale != nil {
		if err := validateText("rationale", *patch.Rationale, false, maxDecisionBytes); err != nil {
			return err
		}
	}
	if patch.Status != nil && !validDecisionStatus(*patch.Status) {
		return &ValidationError{Field: "status", Message: "must be proposed, accepted, rejected, or superseded"}
	}
	return nil
}

func validProjectStatus(value ProjectStatus) bool {
	switch value {
	case ProjectActive, ProjectOnHold, ProjectCompleted, ProjectArchived:
		return true
	default:
		return false
	}
}

func validTaskStatus(value TaskStatus) bool {
	switch value {
	case TaskBacklog, TaskTodo, TaskInProgress, TaskBlocked, TaskDone, TaskCancelled:
		return true
	default:
		return false
	}
}

func validTaskPriority(value TaskPriority) bool {
	switch value {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
		return true
	default:
		return false
	}
}

func validDecisionStatus(value DecisionStatus) bool {
	switch value {
	case DecisionProposed, DecisionAccepted, DecisionRejected, DecisionSuperseded:
		return true
	default:
		return false
	}
}
