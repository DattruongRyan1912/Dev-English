package work

import (
	"testing"
	"time"
)

func TestMemoryRepositoryInitializesAndCompletesCanonicalLifecycles(t *testing.T) {
	repository := NewMemoryRepository()
	scope := testScope("memory-boundary-workspace", "memory-boundary-user")
	base := time.Date(2026, 8, 28, 6, 0, 0, 0, time.UTC)

	project, err := repository.CreateProject(testContext, scope, Project{
		ID:          "memory-boundary-project",
		WorkspaceID: scope.WorkspaceID,
		OwnerUserID: scope.UserID,
		Name:        "Memory boundary project",
		Description: "Initial timestamp defaults",
		Status:      ProjectActive,
		Origin:      OriginCanonical,
		Version:     1,
	}, testMutation("memory-boundary-project-create", base))
	if err != nil {
		t.Fatalf("create memory project: %v", err)
	}
	if !project.CreatedAt.Equal(base) || !project.UpdatedAt.Equal(base) {
		t.Fatalf("memory project timestamps = %v/%v, want %v", project.CreatedAt, project.UpdatedAt, base)
	}

	projectName := "Updated memory project"
	projectDescription := "Updated description"
	projectStatus := ProjectCompleted
	project, err = repository.UpdateProject(testContext, scope, project.ID, ProjectPatch{
		Name:        &projectName,
		Description: &projectDescription,
		Status:      &projectStatus,
	}, project.Version, testMutation("memory-boundary-project-update", base.Add(time.Hour)))
	if err != nil || project.Version != 2 || project.Status != ProjectCompleted {
		t.Fatalf("update memory project = %+v, %v", project, err)
	}
	project, err = repository.TrashProject(testContext, scope, project.ID, project.Version, testMutation("memory-boundary-project-trash", base.Add(2*time.Hour)))
	if err != nil || project.DeletedAt == nil {
		t.Fatalf("trash memory project = %+v, %v", project, err)
	}
	project, err = repository.RestoreProject(testContext, scope, project.ID, project.Version, testMutation("memory-boundary-project-restore", base.Add(3*time.Hour)))
	if err != nil || project.DeletedAt != nil {
		t.Fatalf("restore memory project = %+v, %v", project, err)
	}
	project, err = repository.TrashProject(testContext, scope, project.ID, project.Version, testMutation("memory-boundary-project-trash-final", base.Add(4*time.Hour)))
	if err != nil {
		t.Fatalf("trash memory project for purge: %v", err)
	}
	if _, err := repository.PurgeProject(testContext, scope, project.ID, project.Version, testMutation("memory-boundary-project-purge", project.DeletedAt.Add(RetentionPeriod))); err != nil {
		t.Fatalf("purge memory project: %v", err)
	}

	dueAt := base.Add(24 * time.Hour)
	task, err := repository.CreateTask(testContext, scope, Task{
		ID:          "memory-boundary-task",
		WorkspaceID: scope.WorkspaceID,
		OwnerUserID: scope.UserID,
		Title:       "Memory boundary task",
		Description: "Initial timestamp defaults",
		Status:      TaskTodo,
		Priority:    PriorityNormal,
		DueAt:       &dueAt,
		Origin:      OriginCanonical,
		Version:     1,
	}, testMutation("memory-boundary-task-create", base))
	if err != nil {
		t.Fatalf("create memory task: %v", err)
	}
	taskTitle := "Updated memory task"
	taskDescription := "Updated task description"
	taskStatus := TaskDone
	taskPriority := PriorityUrgent
	updatedDueAt := base.Add(48 * time.Hour)
	updatedDuePatch := &updatedDueAt
	task, err = repository.UpdateTask(testContext, scope, task.ID, TaskPatch{
		Title:       &taskTitle,
		Description: &taskDescription,
		Status:      &taskStatus,
		Priority:    &taskPriority,
		DueAt:       &updatedDuePatch,
	}, task.Version, testMutation("memory-boundary-task-update", base.Add(time.Hour)))
	if err != nil || task.Version != 2 || task.Status != TaskDone || task.DueAt == nil {
		t.Fatalf("update memory task = %+v, %v", task, err)
	}
	task, err = repository.TrashTask(testContext, scope, task.ID, task.Version, testMutation("memory-boundary-task-trash", base.Add(2*time.Hour)))
	if err != nil {
		t.Fatalf("trash memory task: %v", err)
	}
	task, err = repository.RestoreTask(testContext, scope, task.ID, task.Version, testMutation("memory-boundary-task-restore", base.Add(3*time.Hour)))
	if err != nil {
		t.Fatalf("restore memory task: %v", err)
	}
	task, err = repository.TrashTask(testContext, scope, task.ID, task.Version, testMutation("memory-boundary-task-trash-final", base.Add(4*time.Hour)))
	if err != nil {
		t.Fatalf("trash memory task for purge: %v", err)
	}
	if _, err := repository.PurgeTask(testContext, scope, task.ID, task.Version, testMutation("memory-boundary-task-purge", task.DeletedAt.Add(RetentionPeriod))); err != nil {
		t.Fatalf("purge memory task: %v", err)
	}

	decision, err := repository.CreateDecision(testContext, scope, Decision{
		ID:          "memory-boundary-decision",
		WorkspaceID: scope.WorkspaceID,
		OwnerUserID: scope.UserID,
		Title:       "Memory boundary decision",
		Context:     "Initial context",
		Outcome:     "Initial outcome",
		Rationale:   "Initial rationale",
		Status:      DecisionProposed,
		Origin:      OriginCanonical,
		Version:     1,
	}, testMutation("memory-boundary-decision-create", base))
	if err != nil {
		t.Fatalf("create memory decision: %v", err)
	}
	decisionTitle := "Updated memory decision"
	decisionContext := "Updated context"
	decisionOutcome := "Updated outcome"
	decisionRationale := "Updated rationale"
	decisionStatus := DecisionAccepted
	decision, err = repository.UpdateDecision(testContext, scope, decision.ID, DecisionPatch{
		Title:     &decisionTitle,
		Context:   &decisionContext,
		Outcome:   &decisionOutcome,
		Rationale: &decisionRationale,
		Status:    &decisionStatus,
	}, decision.Version, testMutation("memory-boundary-decision-update", base.Add(time.Hour)))
	if err != nil || decision.Version != 2 || decision.Status != DecisionAccepted {
		t.Fatalf("update memory decision = %+v, %v", decision, err)
	}
	decision, err = repository.TrashDecision(testContext, scope, decision.ID, decision.Version, testMutation("memory-boundary-decision-trash", base.Add(2*time.Hour)))
	if err != nil {
		t.Fatalf("trash memory decision: %v", err)
	}
	decision, err = repository.RestoreDecision(testContext, scope, decision.ID, decision.Version, testMutation("memory-boundary-decision-restore", base.Add(3*time.Hour)))
	if err != nil {
		t.Fatalf("restore memory decision: %v", err)
	}
	decision, err = repository.TrashDecision(testContext, scope, decision.ID, decision.Version, testMutation("memory-boundary-decision-trash-final", base.Add(4*time.Hour)))
	if err != nil {
		t.Fatalf("trash memory decision for purge: %v", err)
	}
	if _, err := repository.PurgeDecision(testContext, scope, decision.ID, decision.Version, testMutation("memory-boundary-decision-purge", decision.DeletedAt.Add(RetentionPeriod))); err != nil {
		t.Fatalf("purge memory decision: %v", err)
	}
}
