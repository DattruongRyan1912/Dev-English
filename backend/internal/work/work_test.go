package work

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

var testContext = context.Background()

func newTestService(t *testing.T) (*Service, *MemoryRepository, *time.Time) {
	t.Helper()
	repo := NewMemoryRepository()
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	service, err := NewService(repo, WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service, repo, &now
}

func testScope(workspaceID, userID string) Scope {
	return Scope{WorkspaceID: workspaceID, UserID: userID}
}

func testMutation(key string, at time.Time) Mutation {
	return Mutation{IdempotencyKey: key, RequestHash: strings.Repeat("a", 64), At: at}
}

func mustCreateProject(t *testing.T, service *Service, scope Scope, id, key string) Project {
	t.Helper()
	project, err := service.CreateProject(testContext, scope, CreateProjectInput{
		ID:          id,
		Name:        "Project " + id,
		Description: "A canonical project",
	}, key)
	if err != nil {
		t.Fatalf("CreateProject(%q) error = %v", id, err)
	}
	return project
}

func mustCreateTask(t *testing.T, service *Service, scope Scope, id, projectID, key string) Task {
	t.Helper()
	task, err := service.CreateTask(testContext, scope, CreateTaskInput{
		ID:          id,
		ProjectID:   projectID,
		Title:       "Task " + id,
		Description: "A canonical task",
	}, key)
	if err != nil {
		t.Fatalf("CreateTask(%q) error = %v", id, err)
	}
	return task
}

func mustCreateDecision(t *testing.T, service *Service, scope Scope, id, projectID, key string) Decision {
	t.Helper()
	decision, err := service.CreateDecision(testContext, scope, CreateDecisionInput{
		ID:        id,
		ProjectID: projectID,
		Title:     "Decision " + id,
		Context:   "A context",
		Outcome:   "Keep the canonical path",
		Rationale: "It is explicit and reviewable",
	}, key)
	if err != nil {
		t.Fatalf("CreateDecision(%q) error = %v", id, err)
	}
	return decision
}

func requireIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error = %v, errors.Is(error, %v) = false", err, target)
	}
}

func requireValidation(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error for %s", field)
	}
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %T %v, want *ValidationError", err, err)
	}
	if validation.Field != field {
		t.Fatalf("validation field = %q, want %q", validation.Field, field)
	}
}

func requireNotFound(t *testing.T, err error, entity EntityType, id string) {
	t.Helper()
	var notFoundError *NotFoundError
	if !errors.As(err, &notFoundError) {
		t.Fatalf("error = %T %v, want *NotFoundError", err, err)
	}
	if notFoundError.EntityType != entity || notFoundError.EntityID != id {
		t.Fatalf("not found error = %+v, want type=%q id=%q", notFoundError, entity, id)
	}
}

func requireState(t *testing.T, err error, cause error) {
	t.Helper()
	var stateError *StateError
	if !errors.As(err, &stateError) {
		t.Fatalf("error = %T %v, want *StateError", err, err)
	}
	requireIs(t, err, cause)
}

func requireVersionConflict(t *testing.T, err error, entityType EntityType, id string, expected, actual int64) {
	t.Helper()
	var conflict *VersionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %T %v, want *VersionConflictError", err, err)
	}
	if conflict.EntityType != entityType || conflict.EntityID != id || conflict.Expected != expected || conflict.Actual != actual {
		t.Fatalf("version conflict = %+v, want type=%q id=%q expected=%d actual=%d", conflict, entityType, id, expected, actual)
	}
	requireIs(t, err, ErrVersionConflict)
}

func TestProjectLifecycleHistoryAndRetention(t *testing.T) {
	service, _, now := newTestService(t)
	scope := testScope("workspace-project", "user-project")

	created := mustCreateProject(t, service, scope, "project-1", "project-create")
	if created.Status != ProjectActive || created.Origin != OriginCanonical || created.Version != 1 {
		t.Fatalf("created project defaults = %+v", created)
	}
	if !created.CreatedAt.Equal(*now) || !created.UpdatedAt.Equal(*now) {
		t.Fatalf("created timestamps = %v/%v, want %v", created.CreatedAt, created.UpdatedAt, *now)
	}

	*now = now.Add(time.Hour)
	replayed, err := service.CreateProject(testContext, scope, CreateProjectInput{
		ID:          "project-1",
		Name:        "Project project-1",
		Description: "A canonical project",
	}, "project-create")
	if err != nil {
		t.Fatalf("replayed CreateProject() error = %v", err)
	}
	if !reflect.DeepEqual(replayed, created) {
		t.Fatalf("replayed project = %+v, want original %+v", replayed, created)
	}

	_, err = service.CreateProject(testContext, scope, CreateProjectInput{
		ID:          "project-1",
		Name:        "Changed request",
		Description: "A canonical project",
	}, "project-create")
	var idemConflict *IdempotencyConflictError
	if !errors.As(err, &idemConflict) || idemConflict.ExistingOperation != OperationProjectCreate || idemConflict.RequestedOperation != OperationProjectCreate {
		t.Fatalf("idempotency conflict = %T %v", err, err)
	}
	requireIs(t, err, ErrIdempotencyConflict)

	_, err = service.CreateProject(testContext, scope, CreateProjectInput{
		ID:   "project-1",
		Name: "Another project",
	}, "project-create-duplicate")
	requireIs(t, err, ErrAlreadyExists)

	got, err := service.GetProject(testContext, scope, created.ID)
	if err != nil || !reflect.DeepEqual(got, created) {
		t.Fatalf("GetProject() = %+v, %v; want %+v", got, err, created)
	}
	items, err := service.ListProjects(testContext, scope, ListOptions{})
	if err != nil || len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("ListProjects() = %+v, %v", items, err)
	}

	description := "Updated project"
	*now = now.Add(time.Hour)
	updated, err := service.UpdateProject(testContext, scope, created.ID, ProjectPatch{Description: &description}, 1, "project-update")
	if err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if updated.Version != 2 || updated.Description != description {
		t.Fatalf("updated project = %+v", updated)
	}
	updatedReplay, err := service.UpdateProject(testContext, scope, created.ID, ProjectPatch{Description: &description}, 1, "project-update")
	if err != nil || !reflect.DeepEqual(updatedReplay, updated) {
		t.Fatalf("replayed UpdateProject() = %+v, %v; want %+v", updatedReplay, err, updated)
	}

	otherDescription := "stale"
	_, err = service.UpdateProject(testContext, scope, created.ID, ProjectPatch{Description: &otherDescription}, 1, "project-update-stale")
	requireVersionConflict(t, err, EntityProject, created.ID, 1, 2)

	*now = now.Add(time.Hour)
	trashed, err := service.TrashProject(testContext, scope, created.ID, 2, "project-trash-1")
	if err != nil || trashed.Version != 3 || trashed.DeletedAt == nil {
		t.Fatalf("TrashProject() = %+v, %v", trashed, err)
	}
	items, err = service.ListProjects(testContext, scope, ListOptions{})
	if err != nil || len(items) != 0 {
		t.Fatalf("ListProjects() after trash = %+v, %v", items, err)
	}
	items, err = service.ListProjects(testContext, scope, ListOptions{IncludeTrashed: true})
	if err != nil || len(items) != 1 || items[0].DeletedAt == nil {
		t.Fatalf("ListProjects(include trashed) = %+v, %v", items, err)
	}

	_, err = service.UpdateProject(testContext, scope, created.ID, ProjectPatch{Description: &otherDescription}, 3, "project-update-trashed")
	requireState(t, err, ErrRecordTrashed)
	_, err = service.TrashProject(testContext, scope, created.ID, 3, "project-trash-2")
	requireState(t, err, ErrAlreadyTrashed)

	*now = now.Add(time.Hour)
	restored, err := service.RestoreProject(testContext, scope, created.ID, 3, "project-restore-1")
	if err != nil || restored.Version != 4 || restored.DeletedAt != nil {
		t.Fatalf("RestoreProject() = %+v, %v", restored, err)
	}
	_, err = service.RestoreProject(testContext, scope, created.ID, 4, "project-restore-2")
	requireState(t, err, ErrNotTrashed)
	_, err = service.PurgeProject(testContext, scope, created.ID, 4, "project-purge-active")
	requireState(t, err, ErrNotTrashed)

	*now = now.Add(time.Hour)
	trashedAgain, err := service.TrashProject(testContext, scope, created.ID, 4, "project-trash-3")
	if err != nil || trashedAgain.Version != 5 || trashedAgain.DeletedAt == nil {
		t.Fatalf("second TrashProject() = %+v, %v", trashedAgain, err)
	}
	_, err = service.PurgeProject(testContext, scope, created.ID, 5, "project-purge-early")
	var notReady *PurgeNotReadyError
	if !errors.As(err, &notReady) || notReady.EntityType != EntityProject || notReady.EntityID != created.ID || notReady.EligibleAt == "" {
		t.Fatalf("early purge error = %T %v", err, err)
	}
	requireIs(t, err, ErrPurgeNotReady)

	*now = trashedAgain.DeletedAt.Add(RetentionPeriod)
	purged, err := service.PurgeProject(testContext, scope, created.ID, 5, "project-purge-ready")
	if err != nil || purged.EntityType != EntityProject || purged.EntityID != created.ID || !purged.PurgedAt.Equal(*now) {
		t.Fatalf("PurgeProject() = %+v, %v", purged, err)
	}
	purgedReplay, err := service.PurgeProject(testContext, scope, created.ID, 5, "project-purge-ready")
	if err != nil || !reflect.DeepEqual(purgedReplay, purged) {
		t.Fatalf("replayed PurgeProject() = %+v, %v; want %+v", purgedReplay, err, purged)
	}
	_, err = service.GetProject(testContext, scope, created.ID)
	requireNotFound(t, err, EntityProject, created.ID)

	history, err := service.History(testContext, scope, EntityProject, created.ID)
	if err != nil {
		t.Fatalf("History() after purge error = %v", err)
	}
	wantActions := []HistoryAction{HistoryCreated, HistoryUpdated, HistoryTrashed, HistoryRestored, HistoryTrashed, HistoryPurged}
	if len(history) != len(wantActions) {
		t.Fatalf("history length = %d, want %d: %+v", len(history), len(wantActions), history)
	}
	for index, event := range history {
		if event.Action != wantActions[index] || event.WorkspaceID != scope.WorkspaceID || event.ActorUserID != scope.UserID || event.EntityID != created.ID {
			t.Fatalf("history[%d] = %+v", index, event)
		}
		if event.Before == nil || event.After == nil || event.CreatedAt.IsZero() {
			t.Fatalf("history[%d] snapshots/timestamp = %+v", index, event)
		}
	}
}

func TestTaskAndDecisionLifecycleAndProjectDependencies(t *testing.T) {
	service, _, now := newTestService(t)
	scope := testScope("workspace-work", "user-work")
	project := mustCreateProject(t, service, scope, "project-work", "project-create")
	secondProject := mustCreateProject(t, service, scope, "project-work-2", "project-create-2")

	due := now.Add(48 * time.Hour)
	task, err := service.CreateTask(testContext, scope, CreateTaskInput{
		ID:          "task-1",
		ProjectID:   project.ID,
		Title:       "Task one",
		Description: "Initial task",
		Status:      TaskBacklog,
		Priority:    PriorityHigh,
		DueAt:       &due,
	}, "task-create")
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if task.Version != 1 || task.Origin != OriginCanonical || task.ProjectID != project.ID || task.DueAt == nil || !task.DueAt.Equal(due) {
		t.Fatalf("created task = %+v", task)
	}

	decision := mustCreateDecision(t, service, scope, "decision-1", project.ID, "decision-create")
	if decision.Version != 1 || decision.Origin != OriginCanonical || decision.ProjectID != project.ID {
		t.Fatalf("created decision = %+v", decision)
	}

	listedTasks, err := service.ListTasks(testContext, scope, project.ID, ListOptions{})
	if err != nil || len(listedTasks) != 1 || listedTasks[0].ID != task.ID {
		t.Fatalf("ListTasks() = %+v, %v", listedTasks, err)
	}
	listedDecisions, err := service.ListDecisions(testContext, scope, project.ID, ListOptions{})
	if err != nil || len(listedDecisions) != 1 || listedDecisions[0].ID != decision.ID {
		t.Fatalf("ListDecisions() = %+v, %v", listedDecisions, err)
	}

	newDue := due.Add(24 * time.Hour)
	newTitle := "Updated task"
	newDescription := "Updated description"
	newStatus := TaskInProgress
	newPriority := PriorityUrgent
	updatedTask, err := service.UpdateTask(testContext, scope, task.ID, TaskPatch{
		ProjectID:   stringPointer(secondProject.ID),
		Title:       &newTitle,
		Description: &newDescription,
		Status:      &newStatus,
		Priority:    &newPriority,
		DueAt:       timePointerPointer(&newDue),
	}, 1, "task-update")
	if err != nil {
		t.Fatalf("UpdateTask() error = %v", err)
	}
	if updatedTask.Version != 2 || updatedTask.ProjectID != secondProject.ID || updatedTask.Title != newTitle || updatedTask.Description != newDescription || updatedTask.Status != newStatus || updatedTask.Priority != newPriority || updatedTask.DueAt == nil || !updatedTask.DueAt.Equal(newDue) {
		t.Fatalf("updated task = %+v", updatedTask)
	}
	clearProject := ""
	clearDue := (*time.Time)(nil)
	updatedTask, err = service.UpdateTask(testContext, scope, task.ID, TaskPatch{ProjectID: &clearProject, DueAt: &clearDue}, 2, "task-clear-links")
	if err != nil || updatedTask.Version != 3 || updatedTask.ProjectID != "" || updatedTask.DueAt != nil {
		t.Fatalf("cleared task links = %+v, %v", updatedTask, err)
	}
	_, err = service.UpdateTask(testContext, scope, task.ID, TaskPatch{Title: &newTitle}, 2, "task-stale")
	requireVersionConflict(t, err, EntityTask, task.ID, 2, 3)

	decisionProject := secondProject.ID
	decisionTitle := "Updated decision"
	decisionContext := "Updated context"
	decisionOutcome := "Use the second project"
	decisionRationale := "The boundary is clearer"
	decisionStatus := DecisionAccepted
	updatedDecision, err := service.UpdateDecision(testContext, scope, decision.ID, DecisionPatch{
		ProjectID: &decisionProject,
		Title:     &decisionTitle,
		Context:   &decisionContext,
		Outcome:   &decisionOutcome,
		Rationale: &decisionRationale,
		Status:    &decisionStatus,
	}, 1, "decision-update")
	if err != nil {
		t.Fatalf("UpdateDecision() error = %v", err)
	}
	if updatedDecision.Version != 2 || updatedDecision.ProjectID != secondProject.ID || updatedDecision.Title != decisionTitle || updatedDecision.Context != decisionContext || updatedDecision.Outcome != decisionOutcome || updatedDecision.Rationale != decisionRationale || updatedDecision.Status != decisionStatus {
		t.Fatalf("updated decision = %+v", updatedDecision)
	}
	dependentTask := mustCreateTask(t, service, scope, "dependent-task", project.ID, "dependent-task-create")
	dependentDecision := mustCreateDecision(t, service, scope, "dependent-decision", project.ID, "dependent-decision-create")

	missingProject := "missing-project"
	_, err = service.UpdateTask(testContext, scope, task.ID, TaskPatch{ProjectID: &missingProject}, 3, "task-missing-project")
	requireNotFound(t, err, EntityProject, missingProject)

	*now = now.Add(time.Hour)
	trashedTask, err := service.TrashTask(testContext, scope, task.ID, 3, "task-trash")
	if err != nil || trashedTask.Version != 4 || trashedTask.DeletedAt == nil {
		t.Fatalf("TrashTask() = %+v, %v", trashedTask, err)
	}
	_, err = service.UpdateTask(testContext, scope, task.ID, TaskPatch{Title: &newTitle}, 4, "task-update-trashed")
	requireState(t, err, ErrRecordTrashed)
	_, err = service.TrashTask(testContext, scope, task.ID, 4, "task-trash-again")
	requireState(t, err, ErrAlreadyTrashed)

	trashedDecision, err := service.TrashDecision(testContext, scope, decision.ID, 2, "decision-trash")
	if err != nil || trashedDecision.Version != 3 || trashedDecision.DeletedAt == nil {
		t.Fatalf("TrashDecision() = %+v, %v", trashedDecision, err)
	}
	_, err = service.UpdateDecision(testContext, scope, decision.ID, DecisionPatch{Title: &decisionTitle}, 3, "decision-update-trashed")
	requireState(t, err, ErrRecordTrashed)

	*now = now.Add(time.Hour)
	trashedProject, err := service.TrashProject(testContext, scope, project.ID, 1, "project-trash")
	if err != nil || trashedProject.Version != 2 || trashedProject.DeletedAt == nil {
		t.Fatalf("TrashProject() with dependents = %+v, %v", trashedProject, err)
	}
	*now = trashedProject.DeletedAt.Add(RetentionPeriod)
	_, err = service.PurgeProject(testContext, scope, project.ID, 2, "project-purge-dependent-ready")
	var dependency *DependencyError
	if !errors.As(err, &dependency) || len(dependency.Dependents) != 2 || dependency.Dependents[0] != EntityTask || dependency.Dependents[1] != EntityDecision {
		t.Fatalf("project dependency error = %T %+v", err, dependency)
	}
	requireIs(t, err, ErrDependenciesExist)

	*now = trashedTask.DeletedAt.Add(RetentionPeriod)
	purgedTask, err := service.PurgeTask(testContext, scope, task.ID, 4, "task-purge")
	if err != nil || purgedTask.EntityType != EntityTask {
		t.Fatalf("PurgeTask() = %+v, %v", purgedTask, err)
	}
	purgedDecision, err := service.PurgeDecision(testContext, scope, decision.ID, 3, "decision-purge")
	if err != nil || purgedDecision.EntityType != EntityDecision {
		t.Fatalf("PurgeDecision() = %+v, %v", purgedDecision, err)
	}
	_, err = service.TrashTask(testContext, scope, dependentTask.ID, dependentTask.Version, "dependent-task-trash")
	if err != nil {
		t.Fatalf("TrashTask(dependent) error = %v", err)
	}
	_, err = service.TrashDecision(testContext, scope, dependentDecision.ID, dependentDecision.Version, "dependent-decision-trash")
	if err != nil {
		t.Fatalf("TrashDecision(dependent) error = %v", err)
	}
	*now = now.Add(RetentionPeriod)
	if _, err = service.PurgeTask(testContext, scope, dependentTask.ID, dependentTask.Version+1, "dependent-task-purge"); err != nil {
		t.Fatalf("PurgeTask(dependent) error = %v", err)
	}
	if _, err = service.PurgeDecision(testContext, scope, dependentDecision.ID, dependentDecision.Version+1, "dependent-decision-purge"); err != nil {
		t.Fatalf("PurgeDecision(dependent) error = %v", err)
	}
	_, err = service.GetTask(testContext, scope, task.ID)
	requireNotFound(t, err, EntityTask, task.ID)
	_, err = service.GetDecision(testContext, scope, decision.ID)
	requireNotFound(t, err, EntityDecision, decision.ID)

	*now = trashedProject.DeletedAt.Add(RetentionPeriod)
	purgedProject, err := service.PurgeProject(testContext, scope, project.ID, 2, "project-purge-dependent-final")
	if err != nil || purgedProject.EntityType != EntityProject {
		t.Fatalf("PurgeProject() after dependents = %+v, %v", purgedProject, err)
	}

	if history, err := service.History(testContext, scope, EntityTask, task.ID); err != nil || len(history) != 5 || history[4].Action != HistoryPurged {
		t.Fatalf("task history = %+v, %v", history, err)
	}
	if history, err := service.History(testContext, scope, EntityDecision, decision.ID); err != nil || len(history) != 4 || history[3].Action != HistoryPurged {
		t.Fatalf("decision history = %+v, %v", history, err)
	}
}

func TestScopeIsolationSameIDReuseAndHistoryValidation(t *testing.T) {
	service, repo, now := newTestService(t)
	scopeA := testScope("workspace-shared", "user-a")
	scopeB := testScope("workspace-shared", "user-b")
	scopeC := testScope("workspace-other", "user-c")

	projectA := mustCreateProject(t, service, scopeA, "shared-id", "create-a")
	description := "A update"
	*now = now.Add(time.Hour)
	projectA, err := service.UpdateProject(testContext, scopeA, projectA.ID, ProjectPatch{Description: &description}, 1, "update-a")
	if err != nil {
		t.Fatalf("UpdateProject(scope A) error = %v", err)
	}
	*now = now.Add(time.Hour)
	projectA, err = service.TrashProject(testContext, scopeA, projectA.ID, projectA.Version, "trash-a")
	if err != nil {
		t.Fatalf("TrashProject(scope A) error = %v", err)
	}
	*now = projectA.DeletedAt.Add(RetentionPeriod)
	if _, err = service.PurgeProject(testContext, scopeA, projectA.ID, projectA.Version, "purge-a"); err != nil {
		t.Fatalf("PurgeProject(scope A) error = %v", err)
	}

	projectB := mustCreateProject(t, service, scopeB, "shared-id", "same-key")
	projectC := mustCreateProject(t, service, scopeC, "shared-id", "same-key")
	if projectB.ID != projectA.ID || projectC.ID != projectA.ID {
		t.Fatalf("same IDs were not preserved: B=%+v C=%+v", projectB, projectC)
	}
	if got, err := service.GetProject(testContext, scopeB, "shared-id"); err != nil || got.Description != projectB.Description {
		t.Fatalf("GetProject(scope B) = %+v, %v", got, err)
	}
	if got, err := service.GetProject(testContext, scopeC, "shared-id"); err != nil || got.Description != projectC.Description {
		t.Fatalf("GetProject(scope C) = %+v, %v", got, err)
	}
	_, err = service.GetProject(testContext, scopeA, "shared-id")
	requireNotFound(t, err, EntityProject, "shared-id")
	if items, err := service.ListProjects(testContext, scopeB, ListOptions{}); err != nil || len(items) != 1 || items[0].ID != projectB.ID {
		t.Fatalf("ListProjects(scope B) = %+v, %v", items, err)
	}
	if items, err := service.ListProjects(testContext, scopeC, ListOptions{}); err != nil || len(items) != 1 || items[0].ID != projectC.ID {
		t.Fatalf("ListProjects(scope C) = %+v, %v", items, err)
	}
	if items, err := service.ListProjects(testContext, scopeA, ListOptions{}); err != nil || len(items) != 0 {
		t.Fatalf("ListProjects(scope A) = %+v, %v", items, err)
	}

	historyB, err := service.History(testContext, scopeB, EntityProject, "shared-id")
	if err != nil || len(historyB) != 1 || historyB[0].ActorUserID != scopeB.UserID || historyB[0].WorkspaceID != scopeB.WorkspaceID {
		t.Fatalf("history B = %+v, %v", historyB, err)
	}
	historyC, err := service.History(testContext, scopeC, EntityProject, "shared-id")
	if err != nil || len(historyC) != 1 || historyC[0].ActorUserID != scopeC.UserID || historyC[0].WorkspaceID != scopeC.WorkspaceID {
		t.Fatalf("history C = %+v, %v", historyC, err)
	}
	historyA, err := service.History(testContext, scopeA, EntityProject, "shared-id")
	if err != nil || len(historyA) != 4 {
		t.Fatalf("history A after purge = %+v, %v", historyA, err)
	}

	// A history key cannot be trusted by itself: validate every event before
	// returning a result so a corrupted later event fails closed as well.
	keyA := memoryHistoryKey{WorkspaceID: scopeA.WorkspaceID, UserID: scopeA.UserID, EntityType: EntityProject, EntityID: "shared-id"}
	for _, test := range []struct {
		name   string
		mutate func(*HistoryEvent)
	}{
		{name: "workspace", mutate: func(event *HistoryEvent) { event.WorkspaceID = "attacker-workspace" }},
		{name: "actor", mutate: func(event *HistoryEvent) { event.ActorUserID = "attacker-user" }},
		{name: "entity type", mutate: func(event *HistoryEvent) { event.EntityType = EntityTask }},
		{name: "entity id", mutate: func(event *HistoryEvent) { event.EntityID = "other-id" }},
	} {
		t.Run("history scope "+test.name, func(t *testing.T) {
			repo.mu.Lock()
			bad := cloneHistory(repo.history[keyA][1])
			test.mutate(&bad)
			repo.history[keyA] = append(repo.history[keyA], bad)
			repo.mu.Unlock()

			_, err := service.History(testContext, scopeA, EntityProject, "shared-id")
			requireNotFound(t, err, EntityProject, "shared-id")

			repo.mu.Lock()
			repo.history[keyA] = repo.history[keyA][:len(repo.history[keyA])-1]
			repo.mu.Unlock()
		})
	}

	historyA, err = service.History(testContext, scopeA, EntityProject, "shared-id")
	if err != nil || len(historyA) != 4 {
		t.Fatalf("history A after repair = %+v, %v", historyA, err)
	}
	originalBefore := append(json.RawMessage(nil), historyA[0].Before...)
	historyA[0].Before[0] = 'x'
	reloaded, err := service.History(testContext, scopeA, EntityProject, "shared-id")
	if err != nil || !reflect.DeepEqual(reloaded[0].Before, originalBefore) {
		t.Fatalf("history snapshot was not cloned: %q vs %q, err=%v", reloaded[0].Before, originalBefore, err)
	}
}

func TestConcurrentUpdatesProduceOneSuccessAndTypedConflicts(t *testing.T) {
	service, _, _ := newTestService(t)
	scope := testScope("workspace-race", "user-race")
	project := mustCreateProject(t, service, scope, "race-project", "race-create")

	const attempts = 32
	var waitGroup sync.WaitGroup
	results := make(chan error, attempts)
	for index := 0; index < attempts; index++ {
		index := index
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			name := fmt.Sprintf("name-%d", index)
			_, err := service.UpdateProject(testContext, scope, project.ID, ProjectPatch{Name: &name}, 1, fmt.Sprintf("race-update-%d", index))
			results <- err
		}()
	}
	waitGroup.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		var conflict *VersionConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("concurrent update error = %T %v, want version conflict", err, err)
		}
		conflicts++
	}
	if successes != 1 || conflicts != attempts-1 {
		t.Fatalf("concurrent updates = successes %d conflicts %d, want 1/%d", successes, conflicts, attempts-1)
	}
	items, err := service.ListProjects(testContext, scope, ListOptions{})
	if err != nil || len(items) != 1 || items[0].Version != 2 {
		t.Fatalf("project after concurrent updates = %+v, %v", items, err)
	}
	history, err := service.History(testContext, scope, EntityProject, project.ID)
	if err != nil || len(history) != 2 {
		t.Fatalf("history after concurrent updates = %+v, %v", history, err)
	}
}

func stringPointer(value string) *string {
	return &value
}

func timePointerPointer(value *time.Time) **time.Time {
	return &value
}

func TestScopeListAndMutationValidation(t *testing.T) {
	validScope := testScope("workspace-valid", "user-valid")
	if err := validScope.Validate(); err != nil {
		t.Fatalf("valid Scope.Validate() error = %v", err)
	}
	for _, test := range []struct {
		name  string
		scope Scope
		field string
	}{
		{name: "missing workspace", scope: Scope{UserID: "user"}, field: "workspaceId"},
		{name: "workspace leading space", scope: Scope{WorkspaceID: " workspace", UserID: "user"}, field: "workspaceId"},
		{name: "workspace internal space", scope: Scope{WorkspaceID: "work space", UserID: "user"}, field: "workspaceId"},
		{name: "missing user", scope: Scope{WorkspaceID: "workspace"}, field: "userId"},
		{name: "user trailing tab", scope: Scope{WorkspaceID: "workspace", UserID: "user\t"}, field: "userId"},
		{name: "user internal newline", scope: Scope{WorkspaceID: "workspace", UserID: "us\ner"}, field: "userId"},
	} {
		t.Run(test.name, func(t *testing.T) {
			requireValidation(t, test.scope.Validate(), test.field)
		})
	}

	if options, err := (ListOptions{}).Normalize(); err != nil || options.Limit != 50 {
		t.Fatalf("default ListOptions.Normalize() = %+v, %v", options, err)
	}
	for _, limit := range []int{-1, 201} {
		_, err := (ListOptions{Limit: limit}).Normalize()
		requireValidation(t, err, "limit")
	}
	if options, err := (ListOptions{Limit: 1, IncludeTrashed: true}).Normalize(); err != nil || options.Limit != 1 || !options.IncludeTrashed {
		t.Fatalf("valid ListOptions.Normalize() = %+v, %v", options, err)
	}

	validHash := strings.Repeat("a", 64)
	validMutation := Mutation{IdempotencyKey: "request-1", RequestHash: validHash, At: time.Now()}
	if err := validMutation.Validate(OperationProjectCreate); err != nil {
		t.Fatalf("valid Mutation.Validate() error = %v", err)
	}
	for _, test := range []struct {
		name      string
		mutation  Mutation
		operation string
		field     string
	}{
		{name: "missing key", mutation: Mutation{RequestHash: validHash}, operation: OperationProjectCreate, field: "idempotencyKey"},
		{name: "key leading space", mutation: Mutation{IdempotencyKey: " request", RequestHash: validHash}, operation: OperationProjectCreate, field: "idempotencyKey"},
		{name: "hash too short", mutation: Mutation{IdempotencyKey: "request", RequestHash: "abc"}, operation: OperationProjectCreate, field: "requestHash"},
		{name: "hash invalid character", mutation: Mutation{IdempotencyKey: "request", RequestHash: strings.Repeat("a", 63) + "g"}, operation: OperationProjectCreate, field: "requestHash"},
		{name: "missing operation", mutation: validMutation, operation: "", field: "operation"},
	} {
		t.Run(test.name, func(t *testing.T) {
			requireValidation(t, test.mutation.Validate(test.operation), test.field)
		})
	}

	for _, key := range []string{"", " request", "request ", strings.Repeat("x", maxIdempotencyBytes+1), "request\x00", "request\x7f", "request\u0080"} {
		if err := ValidateIdempotencyKey(key); err == nil {
			t.Fatalf("ValidateIdempotencyKey(%q) unexpectedly succeeded", key)
		}
	}
	if err := ValidateIdempotencyKey("request/2026-01"); err != nil {
		t.Fatalf("printable ValidateIdempotencyKey() error = %v", err)
	}
}

func TestEntityValidationBranches(t *testing.T) {
	validProject := CreateProjectInput{Name: "Project", Description: "Description"}
	if normalized, err := validateProjectCreate(validProject); err != nil || normalized.Status != ProjectActive {
		t.Fatalf("default project validation = %+v, %v", normalized, err)
	}
	for _, test := range []struct {
		name  string
		input CreateProjectInput
		field string
	}{
		{name: "id whitespace", input: CreateProjectInput{ID: "bad id", Name: "Project"}, field: "id"},
		{name: "id too long", input: CreateProjectInput{ID: strings.Repeat("i", maxIDBytes+1), Name: "Project"}, field: "id"},
		{name: "name required", input: CreateProjectInput{}, field: "name"},
		{name: "name leading whitespace", input: CreateProjectInput{Name: " Project"}, field: "name"},
		{name: "name too long", input: CreateProjectInput{Name: strings.Repeat("n", maxNameBytes+1)}, field: "name"},
		{name: "description leading whitespace", input: CreateProjectInput{Name: "Project", Description: " description"}, field: "description"},
		{name: "description too long", input: CreateProjectInput{Name: "Project", Description: strings.Repeat("d", maxDescriptionBytes+1)}, field: "description"},
		{name: "unsupported status", input: CreateProjectInput{Name: "Project", Status: ProjectStatus("unknown")}, field: "status"},
	} {
		t.Run("project create "+test.name, func(t *testing.T) {
			_, err := validateProjectCreate(test.input)
			requireValidation(t, err, test.field)
		})
	}
	for _, test := range []struct {
		name  string
		patch ProjectPatch
		field string
	}{
		{name: "empty patch", patch: ProjectPatch{}, field: "patch"},
		{name: "invalid name", patch: ProjectPatch{Name: stringPointer(" ")}, field: "name"},
		{name: "invalid description", patch: ProjectPatch{Description: stringPointer(" description")}, field: "description"},
		{name: "invalid status", patch: ProjectPatch{Status: projectStatusPointer("unknown")}, field: "status"},
	} {
		t.Run("project patch "+test.name, func(t *testing.T) {
			requireValidation(t, validateProjectPatch(test.patch), test.field)
		})
	}
	if err := validateProjectPatch(ProjectPatch{Name: stringPointer("Valid"), Description: stringPointer(""), Status: projectStatusPointer(ProjectCompleted)}); err != nil {
		t.Fatalf("valid project patch error = %v", err)
	}

	validTask := CreateTaskInput{Title: "Task"}
	if normalized, err := validateTaskCreate(validTask); err != nil || normalized.Status != TaskTodo || normalized.Priority != PriorityNormal {
		t.Fatalf("default task validation = %+v, %v", normalized, err)
	}
	for _, test := range []struct {
		name  string
		input CreateTaskInput
		field string
	}{
		{name: "id invalid", input: CreateTaskInput{ID: "bad id", Title: "Task"}, field: "id"},
		{name: "project invalid", input: CreateTaskInput{ProjectID: "bad id", Title: "Task"}, field: "projectId"},
		{name: "title required", input: CreateTaskInput{}, field: "title"},
		{name: "title leading whitespace", input: CreateTaskInput{Title: " Task"}, field: "title"},
		{name: "title too long", input: CreateTaskInput{Title: strings.Repeat("t", maxTitleBytes+1)}, field: "title"},
		{name: "description leading whitespace", input: CreateTaskInput{Title: "Task", Description: " description"}, field: "description"},
		{name: "description too long", input: CreateTaskInput{Title: "Task", Description: strings.Repeat("d", maxDescriptionBytes+1)}, field: "description"},
		{name: "unsupported status", input: CreateTaskInput{Title: "Task", Status: TaskStatus("unknown")}, field: "status"},
		{name: "unsupported priority", input: CreateTaskInput{Title: "Task", Priority: TaskPriority("unknown")}, field: "priority"},
	} {
		t.Run("task create "+test.name, func(t *testing.T) {
			_, err := validateTaskCreate(test.input)
			requireValidation(t, err, test.field)
		})
	}
	for _, test := range []struct {
		name  string
		patch TaskPatch
		field string
	}{
		{name: "empty patch", patch: TaskPatch{}, field: "patch"},
		{name: "invalid project", patch: TaskPatch{ProjectID: stringPointer("bad id")}, field: "projectId"},
		{name: "invalid title", patch: TaskPatch{Title: stringPointer(" ")}, field: "title"},
		{name: "invalid description", patch: TaskPatch{Description: stringPointer(" description")}, field: "description"},
		{name: "invalid status", patch: TaskPatch{Status: taskStatusPointer("unknown")}, field: "status"},
		{name: "invalid priority", patch: TaskPatch{Priority: taskPriorityPointer("unknown")}, field: "priority"},
	} {
		t.Run("task patch "+test.name, func(t *testing.T) {
			requireValidation(t, validateTaskPatch(test.patch), test.field)
		})
	}
	if err := validateTaskPatch(TaskPatch{ProjectID: stringPointer(""), Title: stringPointer("Valid"), Description: stringPointer(""), Status: taskStatusPointer(TaskDone), Priority: taskPriorityPointer(PriorityLow), DueAt: timePointerPointer(nil)}); err != nil {
		t.Fatalf("valid task patch error = %v", err)
	}

	validDecision := CreateDecisionInput{Title: "Decision", Outcome: "Outcome"}
	if normalized, err := validateDecisionCreate(validDecision); err != nil || normalized.Status != DecisionProposed {
		t.Fatalf("default decision validation = %+v, %v", normalized, err)
	}
	for _, test := range []struct {
		name  string
		input CreateDecisionInput
		field string
	}{
		{name: "id invalid", input: CreateDecisionInput{ID: "bad id", Title: "Decision", Outcome: "Outcome"}, field: "id"},
		{name: "project invalid", input: CreateDecisionInput{ProjectID: "bad id", Title: "Decision", Outcome: "Outcome"}, field: "projectId"},
		{name: "title required", input: CreateDecisionInput{Outcome: "Outcome"}, field: "title"},
		{name: "title leading whitespace", input: CreateDecisionInput{Title: " Decision", Outcome: "Outcome"}, field: "title"},
		{name: "title too long", input: CreateDecisionInput{Title: strings.Repeat("t", maxTitleBytes+1), Outcome: "Outcome"}, field: "title"},
		{name: "context leading whitespace", input: CreateDecisionInput{Title: "Decision", Context: " context", Outcome: "Outcome"}, field: "context"},
		{name: "context too long", input: CreateDecisionInput{Title: "Decision", Context: strings.Repeat("c", maxDecisionBytes+1), Outcome: "Outcome"}, field: "context"},
		{name: "outcome required", input: CreateDecisionInput{Title: "Decision"}, field: "outcome"},
		{name: "outcome leading whitespace", input: CreateDecisionInput{Title: "Decision", Outcome: " Outcome"}, field: "outcome"},
		{name: "outcome too long", input: CreateDecisionInput{Title: "Decision", Outcome: strings.Repeat("o", maxDecisionBytes+1)}, field: "outcome"},
		{name: "rationale leading whitespace", input: CreateDecisionInput{Title: "Decision", Outcome: "Outcome", Rationale: " rationale"}, field: "rationale"},
		{name: "rationale too long", input: CreateDecisionInput{Title: "Decision", Outcome: "Outcome", Rationale: strings.Repeat("r", maxDecisionBytes+1)}, field: "rationale"},
		{name: "unsupported status", input: CreateDecisionInput{Title: "Decision", Outcome: "Outcome", Status: DecisionStatus("unknown")}, field: "status"},
	} {
		t.Run("decision create "+test.name, func(t *testing.T) {
			_, err := validateDecisionCreate(test.input)
			requireValidation(t, err, test.field)
		})
	}
	for _, test := range []struct {
		name  string
		patch DecisionPatch
		field string
	}{
		{name: "empty patch", patch: DecisionPatch{}, field: "patch"},
		{name: "invalid project", patch: DecisionPatch{ProjectID: stringPointer("bad id")}, field: "projectId"},
		{name: "invalid title", patch: DecisionPatch{Title: stringPointer(" ")}, field: "title"},
		{name: "invalid context", patch: DecisionPatch{Context: stringPointer(" context")}, field: "context"},
		{name: "invalid outcome", patch: DecisionPatch{Outcome: stringPointer(" ")}, field: "outcome"},
		{name: "invalid rationale", patch: DecisionPatch{Rationale: stringPointer(" rationale")}, field: "rationale"},
		{name: "invalid status", patch: DecisionPatch{Status: decisionStatusPointer("unknown")}, field: "status"},
	} {
		t.Run("decision patch "+test.name, func(t *testing.T) {
			requireValidation(t, validateDecisionPatch(test.patch), test.field)
		})
	}
	if err := validateDecisionPatch(DecisionPatch{ProjectID: stringPointer(""), Title: stringPointer("Valid"), Context: stringPointer(""), Outcome: stringPointer("Valid"), Rationale: stringPointer(""), Status: decisionStatusPointer(DecisionRejected)}); err != nil {
		t.Fatalf("valid decision patch error = %v", err)
	}

	if err := validateText("optional", "", false, 1); err != nil {
		t.Fatalf("optional empty text error = %v", err)
	}
	if err := validateText("exact", "x", true, 1); err != nil {
		t.Fatalf("exact text error = %v", err)
	}
	if err := validateEntityID("id", "ok"); err != nil {
		t.Fatalf("valid entity ID error = %v", err)
	}
	if err := validateEntityID("id", strings.Repeat("x", maxIDBytes+1)); err == nil {
		t.Fatal("oversized entity ID unexpectedly validated")
	}
	if err := validateVersion(1); err != nil {
		t.Fatalf("valid version error = %v", err)
	}
	if err := validateVersion(0); err == nil {
		t.Fatal("zero version unexpectedly validated")
	}
	for _, entityType := range []EntityType{EntityProject, EntityTask, EntityDecision} {
		if err := validateEntityType(entityType); err != nil {
			t.Fatalf("valid entity type %q error = %v", entityType, err)
		}
	}
	if err := validateEntityType("unknown"); err == nil {
		t.Fatal("unknown entity type unexpectedly validated")
	}
	for _, value := range []ProjectStatus{ProjectActive, ProjectOnHold, ProjectCompleted, ProjectArchived} {
		if !validProjectStatus(value) {
			t.Fatalf("valid project status %q rejected", value)
		}
	}
	for _, value := range []TaskStatus{TaskBacklog, TaskTodo, TaskInProgress, TaskBlocked, TaskDone, TaskCancelled} {
		if !validTaskStatus(value) {
			t.Fatalf("valid task status %q rejected", value)
		}
	}
	for _, value := range []TaskPriority{PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent} {
		if !validTaskPriority(value) {
			t.Fatalf("valid task priority %q rejected", value)
		}
	}
	for _, value := range []DecisionStatus{DecisionProposed, DecisionAccepted, DecisionRejected, DecisionSuperseded} {
		if !validDecisionStatus(value) {
			t.Fatalf("valid decision status %q rejected", value)
		}
	}
	if validProjectStatus("unknown") || validTaskStatus("unknown") || validTaskPriority("unknown") || validDecisionStatus("unknown") {
		t.Fatal("unknown enum value unexpectedly accepted")
	}
}

func projectStatusPointer(value ProjectStatus) *ProjectStatus { return &value }
func taskStatusPointer(value TaskStatus) *TaskStatus          { return &value }
func taskPriorityPointer(value TaskPriority) *TaskPriority    { return &value }
func decisionStatusPointer(value DecisionStatus) *DecisionStatus {
	return &value
}

func TestJSONSnapshotsAndStoredValueCopies(t *testing.T) {
	service, repo, now := newTestService(t)
	scope := testScope("workspace-copies", "user-copies")
	due := now.Add(time.Hour)
	task := mustCreateProject(t, service, scope, "copy-project", "copy-project-create")
	createdTask, err := service.CreateTask(testContext, scope, CreateTaskInput{ID: "copy-task", ProjectID: task.ID, Title: "Copy task", DueAt: &due}, "copy-task-create")
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if createdTask.DueAt == nil {
		t.Fatal("created task DueAt is nil")
	}
	createdTask.DueAt = &time.Time{}
	loadedTask, err := service.GetTask(testContext, scope, "copy-task")
	if err != nil || loadedTask.DueAt == nil || loadedTask.DueAt.Equal(time.Time{}) {
		t.Fatalf("GetTask() returned aliased DueAt = %+v, %v", loadedTask, err)
	}

	repo.mu.RLock()
	stored := repo.tasks[entityKey(scope, "copy-task")]
	repo.mu.RUnlock()
	if stored.DueAt == nil || stored.DueAt.Equal(time.Time{}) {
		t.Fatalf("stored DueAt was modified through returned value = %+v", stored)
	}

	history, err := service.History(testContext, scope, EntityTask, "copy-task")
	if err != nil || len(history) != 1 {
		t.Fatalf("task history = %+v, %v", history, err)
	}
	var snapshot Task
	if err := json.Unmarshal(history[0].After, &snapshot); err != nil {
		t.Fatalf("unmarshal task snapshot: %v", err)
	}
	if snapshot.ID != "copy-task" || snapshot.ProjectID != task.ID {
		t.Fatalf("task history snapshot = %+v", snapshot)
	}
}

func TestServiceConstructionAndValidationSurface(t *testing.T) {
	repo := NewMemoryRepository()
	if _, err := NewService(nil); err == nil {
		t.Fatal("NewService(nil) unexpectedly succeeded")
	} else {
		requireValidation(t, err, "repository")
	}

	fixed := time.Date(2027, time.February, 3, 4, 5, 6, 0, time.FixedZone("test", 7*60*60))
	service, err := NewService(repo, nil, WithClock(nil), WithClock(func() time.Time { return fixed }))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	scope := testScope("workspace-service-validation", "user-service-validation")
	validProjectPatch := ProjectPatch{Name: stringPointer("Valid")}
	validTaskPatch := TaskPatch{Title: stringPointer("Valid")}
	validDecisionPatch := DecisionPatch{Title: stringPointer("Valid")}
	validProject := CreateProjectInput{Name: "Project"}
	validTask := CreateTaskInput{Title: "Task"}
	validDecision := CreateDecisionInput{Title: "Decision", Outcome: "Outcome"}

	if project, err := service.CreateProject(testContext, scope, validProject, "service-clock"); err != nil || !project.CreatedAt.Equal(fixed.UTC()) {
		t.Fatalf("clock option result = %+v, %v", project, err)
	}
	if _, err := service.CreateProject(testContext, Scope{}, validProject, "key"); err != nil {
		requireValidation(t, err, "workspaceId")
	} else {
		t.Fatal("CreateProject() accepted invalid scope")
	}
	if _, err := service.CreateProject(testContext, scope, CreateProjectInput{}, "key"); err != nil {
		requireValidation(t, err, "name")
	} else {
		t.Fatal("CreateProject() accepted invalid input")
	}
	if _, err := service.CreateProject(testContext, scope, validProject, ""); err != nil {
		requireValidation(t, err, "idempotencyKey")
	} else {
		t.Fatal("CreateProject() accepted missing idempotency key")
	}

	if _, err := service.GetProject(testContext, Scope{}, "project"); err != nil {
		requireValidation(t, err, "workspaceId")
	} else {
		t.Fatal("GetProject() accepted invalid scope")
	}
	if _, err := service.GetProject(testContext, scope, "bad id"); err != nil {
		requireValidation(t, err, "id")
	} else {
		t.Fatal("GetProject() accepted invalid ID")
	}
	if _, err := service.ListProjects(testContext, Scope{}, ListOptions{}); err != nil {
		requireValidation(t, err, "workspaceId")
	} else {
		t.Fatal("ListProjects() accepted invalid scope")
	}
	if _, err := service.ListProjects(testContext, scope, ListOptions{Limit: 201}); err != nil {
		requireValidation(t, err, "limit")
	} else {
		t.Fatal("ListProjects() accepted invalid options")
	}

	if _, err := service.UpdateProject(testContext, Scope{}, "project", validProjectPatch, 1, "key"); err != nil {
		requireValidation(t, err, "workspaceId")
	} else {
		t.Fatal("UpdateProject() accepted invalid scope")
	}
	if _, err := service.UpdateProject(testContext, scope, "bad id", validProjectPatch, 1, "key"); err != nil {
		requireValidation(t, err, "id")
	} else {
		t.Fatal("UpdateProject() accepted invalid ID")
	}
	if _, err := service.UpdateProject(testContext, scope, "project", validProjectPatch, 0, "key"); err != nil {
		requireValidation(t, err, "expectedVersion")
	} else {
		t.Fatal("UpdateProject() accepted invalid version")
	}
	if _, err := service.UpdateProject(testContext, scope, "project", ProjectPatch{}, 1, "key"); err != nil {
		requireValidation(t, err, "patch")
	} else {
		t.Fatal("UpdateProject() accepted empty patch")
	}
	if _, err := service.UpdateProject(testContext, scope, "project", validProjectPatch, 1, ""); err != nil {
		requireValidation(t, err, "idempotencyKey")
	} else {
		t.Fatal("UpdateProject() accepted missing idempotency key")
	}

	for _, test := range []struct {
		name string
		call func() error
	}{
		{name: "trash invalid scope", call: func() error {
			_, err := service.TrashProject(testContext, Scope{}, "project", 1, "key")
			return err
		}},
		{name: "trash invalid version", call: func() error {
			_, err := service.TrashProject(testContext, scope, "project", 0, "key")
			return err
		}},
		{name: "trash invalid key", call: func() error {
			_, err := service.TrashProject(testContext, scope, "project", 1, "")
			return err
		}},
		{name: "restore invalid scope", call: func() error {
			_, err := service.RestoreProject(testContext, Scope{}, "project", 1, "key")
			return err
		}},
		{name: "restore invalid version", call: func() error {
			_, err := service.RestoreProject(testContext, scope, "project", 0, "key")
			return err
		}},
		{name: "restore invalid key", call: func() error {
			_, err := service.RestoreProject(testContext, scope, "project", 1, "")
			return err
		}},
		{name: "purge invalid scope", call: func() error {
			_, err := service.PurgeProject(testContext, Scope{}, "project", 1, "key")
			return err
		}},
		{name: "purge invalid version", call: func() error {
			_, err := service.PurgeProject(testContext, scope, "project", 0, "key")
			return err
		}},
		{name: "purge invalid key", call: func() error {
			_, err := service.PurgeProject(testContext, scope, "project", 1, "")
			return err
		}},
	} {
		t.Run("project "+test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("operation unexpectedly succeeded")
			} else if strings.Contains(test.name, "scope") {
				requireValidation(t, err, "workspaceId")
			} else if strings.Contains(test.name, "version") {
				requireValidation(t, err, "expectedVersion")
			} else {
				requireValidation(t, err, "idempotencyKey")
			}
		})
	}

	if _, err := service.CreateTask(testContext, scope, validTask, "service-task"); err != nil {
		t.Fatalf("CreateTask() setup error = %v", err)
	}
	if _, err := service.CreateTask(testContext, Scope{}, validTask, "key"); err != nil {
		requireValidation(t, err, "workspaceId")
	} else {
		t.Fatal("CreateTask() accepted invalid scope")
	}
	if _, err := service.CreateTask(testContext, scope, CreateTaskInput{}, "key"); err != nil {
		requireValidation(t, err, "title")
	} else {
		t.Fatal("CreateTask() accepted invalid input")
	}
	if _, err := service.CreateTask(testContext, scope, validTask, ""); err != nil {
		requireValidation(t, err, "idempotencyKey")
	} else {
		t.Fatal("CreateTask() accepted missing idempotency key")
	}
	if _, err := service.GetTask(testContext, Scope{}, "task"); err != nil {
		requireValidation(t, err, "workspaceId")
	} else {
		t.Fatal("GetTask() accepted invalid scope")
	}
	if _, err := service.GetTask(testContext, scope, "bad id"); err != nil {
		requireValidation(t, err, "id")
	} else {
		t.Fatal("GetTask() accepted invalid ID")
	}
	if _, err := service.ListTasks(testContext, Scope{}, "", ListOptions{}); err != nil {
		requireValidation(t, err, "workspaceId")
	} else {
		t.Fatal("ListTasks() accepted invalid scope")
	}
	if _, err := service.ListTasks(testContext, scope, "bad id", ListOptions{}); err != nil {
		requireValidation(t, err, "projectId")
	} else {
		t.Fatal("ListTasks() accepted invalid project ID")
	}
	if _, err := service.UpdateTask(testContext, Scope{}, "task", validTaskPatch, 1, "key"); err != nil {
		requireValidation(t, err, "workspaceId")
	} else {
		t.Fatal("UpdateTask() accepted invalid scope")
	}
	if _, err := service.UpdateTask(testContext, scope, "bad id", validTaskPatch, 1, "key"); err != nil {
		requireValidation(t, err, "id")
	} else {
		t.Fatal("UpdateTask() accepted invalid ID")
	}
	if _, err := service.UpdateTask(testContext, scope, "task", validTaskPatch, 0, "key"); err != nil {
		requireValidation(t, err, "expectedVersion")
	} else {
		t.Fatal("UpdateTask() accepted invalid version")
	}
	if _, err := service.UpdateTask(testContext, scope, "task", TaskPatch{}, 1, "key"); err != nil {
		requireValidation(t, err, "patch")
	} else {
		t.Fatal("UpdateTask() accepted empty patch")
	}
	if _, err := service.UpdateTask(testContext, scope, "task", validTaskPatch, 1, ""); err != nil {
		requireValidation(t, err, "idempotencyKey")
	} else {
		t.Fatal("UpdateTask() accepted missing idempotency key")
	}

	for _, test := range []struct {
		name string
		call func() error
	}{
		{name: "task trash invalid scope", call: func() error {
			_, err := service.TrashTask(testContext, Scope{}, "task", 1, "key")
			return err
		}},
		{name: "task restore invalid version", call: func() error {
			_, err := service.RestoreTask(testContext, scope, "task", 0, "key")
			return err
		}},
		{name: "task purge invalid key", call: func() error {
			_, err := service.PurgeTask(testContext, scope, "task", 1, "")
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("operation unexpectedly succeeded")
			} else if strings.Contains(test.name, "scope") {
				requireValidation(t, err, "workspaceId")
			} else if strings.Contains(test.name, "version") {
				requireValidation(t, err, "expectedVersion")
			} else {
				requireValidation(t, err, "idempotencyKey")
			}
		})
	}

	if _, err := service.CreateDecision(testContext, scope, validDecision, "service-decision"); err != nil {
		t.Fatalf("CreateDecision() setup error = %v", err)
	}
	if _, err := service.CreateDecision(testContext, Scope{}, validDecision, "key"); err != nil {
		requireValidation(t, err, "workspaceId")
	} else {
		t.Fatal("CreateDecision() accepted invalid scope")
	}
	if _, err := service.CreateDecision(testContext, scope, CreateDecisionInput{}, "key"); err != nil {
		requireValidation(t, err, "title")
	} else {
		t.Fatal("CreateDecision() accepted invalid input")
	}
	if _, err := service.CreateDecision(testContext, scope, validDecision, ""); err != nil {
		requireValidation(t, err, "idempotencyKey")
	} else {
		t.Fatal("CreateDecision() accepted missing idempotency key")
	}
	if _, err := service.GetDecision(testContext, Scope{}, "decision"); err != nil {
		requireValidation(t, err, "workspaceId")
	} else {
		t.Fatal("GetDecision() accepted invalid scope")
	}
	if _, err := service.GetDecision(testContext, scope, "bad id"); err != nil {
		requireValidation(t, err, "id")
	} else {
		t.Fatal("GetDecision() accepted invalid ID")
	}
	if _, err := service.ListDecisions(testContext, Scope{}, "", ListOptions{}); err != nil {
		requireValidation(t, err, "workspaceId")
	} else {
		t.Fatal("ListDecisions() accepted invalid scope")
	}
	if _, err := service.ListDecisions(testContext, scope, "bad id", ListOptions{}); err != nil {
		requireValidation(t, err, "projectId")
	} else {
		t.Fatal("ListDecisions() accepted invalid project ID")
	}
	if _, err := service.UpdateDecision(testContext, Scope{}, "decision", validDecisionPatch, 1, "key"); err != nil {
		requireValidation(t, err, "workspaceId")
	} else {
		t.Fatal("UpdateDecision() accepted invalid scope")
	}
	if _, err := service.UpdateDecision(testContext, scope, "bad id", validDecisionPatch, 1, "key"); err != nil {
		requireValidation(t, err, "id")
	} else {
		t.Fatal("UpdateDecision() accepted invalid ID")
	}
	if _, err := service.UpdateDecision(testContext, scope, "decision", validDecisionPatch, 0, "key"); err != nil {
		requireValidation(t, err, "expectedVersion")
	} else {
		t.Fatal("UpdateDecision() accepted invalid version")
	}
	if _, err := service.UpdateDecision(testContext, scope, "decision", DecisionPatch{}, 1, "key"); err != nil {
		requireValidation(t, err, "patch")
	} else {
		t.Fatal("UpdateDecision() accepted empty patch")
	}
	if _, err := service.UpdateDecision(testContext, scope, "decision", validDecisionPatch, 1, ""); err != nil {
		requireValidation(t, err, "idempotencyKey")
	} else {
		t.Fatal("UpdateDecision() accepted missing idempotency key")
	}

	for _, test := range []struct {
		name string
		call func() error
	}{
		{name: "decision trash invalid scope", call: func() error {
			_, err := service.TrashDecision(testContext, Scope{}, "decision", 1, "key")
			return err
		}},
		{name: "decision restore invalid version", call: func() error {
			_, err := service.RestoreDecision(testContext, scope, "decision", 0, "key")
			return err
		}},
		{name: "decision purge invalid key", call: func() error {
			_, err := service.PurgeDecision(testContext, scope, "decision", 1, "")
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("operation unexpectedly succeeded")
			} else if strings.Contains(test.name, "scope") {
				requireValidation(t, err, "workspaceId")
			} else if strings.Contains(test.name, "version") {
				requireValidation(t, err, "expectedVersion")
			} else {
				requireValidation(t, err, "idempotencyKey")
			}
		})
	}

	if _, err := service.History(testContext, Scope{}, EntityProject, "project"); err != nil {
		requireValidation(t, err, "workspaceId")
	} else {
		t.Fatal("History() accepted invalid scope")
	}
	if _, err := service.History(testContext, scope, EntityType("unknown"), "project"); err != nil {
		requireValidation(t, err, "entityType")
	} else {
		t.Fatal("History() accepted invalid entity type")
	}
	if _, err := service.History(testContext, scope, EntityProject, "bad id"); err != nil {
		requireValidation(t, err, "entityId")
	} else {
		t.Fatal("History() accepted invalid entity ID")
	}
}

func TestInferredContentValidation(t *testing.T) {
	valid := InferredWorkContent{
		ID:          "inferred-1",
		WorkspaceID: "workspace-inferred",
		OwnerUserID: "user-inferred",
		EntityType:  EntityTask,
		EntityID:    "task-1",
		Content:     "A suggestion",
		Model:       "local-model",
		Origin:      OriginInferred,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid InferredWorkContent.Validate() error = %v", err)
	}
	for _, test := range []struct {
		name  string
		value InferredWorkContent
		field string
	}{
		{name: "invalid scope", value: InferredWorkContent{OwnerUserID: "user", EntityType: EntityTask, Content: "content", Origin: OriginInferred}, field: "workspaceId"},
		{name: "invalid origin", value: func() InferredWorkContent { value := valid; value.Origin = OriginCanonical; return value }(), field: "origin"},
		{name: "invalid entity type", value: func() InferredWorkContent { value := valid; value.EntityType = "unknown"; return value }(), field: "entityType"},
		{name: "empty content", value: func() InferredWorkContent { value := valid; value.Content = " \t"; return value }(), field: "content"},
	} {
		t.Run(test.name, func(t *testing.T) {
			requireValidation(t, test.value.Validate(), test.field)
		})
	}
}

func TestErrorContracts(t *testing.T) {
	var nilValidation *ValidationError
	if got := nilValidation.Error(); got != "validation failed" {
		t.Fatalf("nil ValidationError.Error() = %q", got)
	}
	validation := &ValidationError{Field: "name", Message: "is required"}
	if validation.Error() != "name: is required" {
		t.Fatalf("ValidationError.Error() = %q", validation.Error())
	}

	version := &VersionConflictError{EntityType: EntityTask, EntityID: "task-1", Expected: 1, Actual: 2}
	if version.Error() == "" || version.StatusCode() != 409 || !errors.Is(version, ErrVersionConflict) {
		t.Fatalf("version conflict contract = %q/%d", version.Error(), version.StatusCode())
	}
	idempotency := &IdempotencyConflictError{Key: "key", ExistingOperation: OperationProjectCreate, RequestedOperation: OperationProjectUpdate}
	if idempotency.Error() == "" || idempotency.StatusCode() != 409 || !errors.Is(idempotency, ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict contract = %q/%d", idempotency.Error(), idempotency.StatusCode())
	}
	notFound := &NotFoundError{EntityType: EntityDecision, EntityID: "decision-1"}
	if notFound.Error() == "" || !errors.Is(notFound, ErrNotFound) {
		t.Fatalf("not found contract = %q", notFound.Error())
	}
	purgeNotReady := &PurgeNotReadyError{EntityType: EntityProject, EntityID: "project-1", EligibleAt: time.Now().UTC().Format(time.RFC3339)}
	if purgeNotReady.Error() == "" || purgeNotReady.StatusCode() != 422 || !errors.Is(purgeNotReady, ErrPurgeNotReady) {
		t.Fatalf("purge-not-ready contract = %q/%d", purgeNotReady.Error(), purgeNotReady.StatusCode())
	}
	dependency := &DependencyError{EntityType: EntityProject, EntityID: "project-1", Dependents: []EntityType{EntityTask}}
	if dependency.Error() == "" || dependency.StatusCode() != 409 || !errors.Is(dependency, ErrDependenciesExist) {
		t.Fatalf("dependency contract = %q/%d", dependency.Error(), dependency.StatusCode())
	}
	stateWithoutCause := &StateError{EntityType: EntityProject, EntityID: "project-1", State: "active"}
	if stateWithoutCause.Error() == "" || stateWithoutCause.Unwrap() != nil {
		t.Fatalf("state-without-cause contract = %q/%v", stateWithoutCause.Error(), stateWithoutCause.Unwrap())
	}
	stateWithCause := &StateError{EntityType: EntityProject, EntityID: "project-1", State: "trashed", Cause: ErrRecordTrashed}
	if stateWithCause.Error() == "" || !errors.Is(stateWithCause, ErrRecordTrashed) {
		t.Fatalf("state-with-cause contract = %q", stateWithCause.Error())
	}
}

func TestMemoryHelpersAndStoredValidation(t *testing.T) {
	scope := testScope("workspace-helper", "user-helper")
	otherScope := testScope("workspace-other", "user-other")
	now := time.Date(2026, time.March, 4, 5, 6, 7, 0, time.UTC)
	validProject := Project{ID: "project-helper", WorkspaceID: scope.WorkspaceID, OwnerUserID: scope.UserID, Name: "Project", Description: "Description", Status: ProjectActive, Origin: OriginCanonical, Version: 1}
	validTask := Task{ID: "task-helper", WorkspaceID: scope.WorkspaceID, OwnerUserID: scope.UserID, Title: "Task", Description: "Description", Status: TaskTodo, Priority: PriorityNormal, Origin: OriginCanonical, Version: 1}
	validDecision := Decision{ID: "decision-helper", WorkspaceID: scope.WorkspaceID, OwnerUserID: scope.UserID, Title: "Decision", Context: "Context", Outcome: "Outcome", Rationale: "Rationale", Status: DecisionProposed, Origin: OriginCanonical, Version: 1}

	if err := validateStoredProject(scope, validProject); err != nil {
		t.Fatalf("valid stored project error = %v", err)
	}
	if err := validateStoredTask(scope, validTask); err != nil {
		t.Fatalf("valid stored task error = %v", err)
	}
	if err := validateStoredDecision(scope, validDecision); err != nil {
		t.Fatalf("valid stored decision error = %v", err)
	}
	for _, test := range []struct {
		name  string
		value Project
		field string
	}{
		{name: "scope", value: func() Project { value := validProject; value.WorkspaceID = otherScope.WorkspaceID; return value }(), field: "scope"},
		{name: "name", value: func() Project { value := validProject; value.Name = " name"; return value }(), field: "name"},
		{name: "description", value: func() Project { value := validProject; value.Description = " description"; return value }(), field: "description"},
		{name: "status", value: func() Project { value := validProject; value.Status = "bad"; return value }(), field: "status"},
		{name: "origin", value: func() Project { value := validProject; value.Origin = OriginInferred; return value }(), field: "origin"},
		{name: "version", value: func() Project { value := validProject; value.Version = 2; return value }(), field: "version"},
	} {
		t.Run("stored project "+test.name, func(t *testing.T) {
			requireValidation(t, validateStoredProject(scope, test.value), test.field)
		})
	}
	for _, test := range []struct {
		name  string
		value Task
		field string
	}{
		{name: "scope", value: func() Task { value := validTask; value.OwnerUserID = otherScope.UserID; return value }(), field: "scope"},
		{name: "title", value: func() Task { value := validTask; value.Title = " title"; return value }(), field: "title"},
		{name: "description", value: func() Task { value := validTask; value.Description = " description"; return value }(), field: "description"},
		{name: "status", value: func() Task { value := validTask; value.Status = "bad"; return value }(), field: "status"},
		{name: "priority", value: func() Task { value := validTask; value.Priority = "bad"; return value }(), field: "priority"},
		{name: "origin", value: func() Task { value := validTask; value.Origin = OriginInferred; return value }(), field: "origin"},
		{name: "version", value: func() Task { value := validTask; value.Version = 2; return value }(), field: "version"},
	} {
		t.Run("stored task "+test.name, func(t *testing.T) {
			requireValidation(t, validateStoredTask(scope, test.value), test.field)
		})
	}
	for _, test := range []struct {
		name  string
		value Decision
		field string
	}{
		{name: "scope", value: func() Decision { value := validDecision; value.WorkspaceID = otherScope.WorkspaceID; return value }(), field: "scope"},
		{name: "title", value: func() Decision { value := validDecision; value.Title = " title"; return value }(), field: "title"},
		{name: "context", value: func() Decision { value := validDecision; value.Context = " context"; return value }(), field: "context"},
		{name: "outcome", value: func() Decision { value := validDecision; value.Outcome = " outcome"; return value }(), field: "outcome"},
		{name: "rationale", value: func() Decision { value := validDecision; value.Rationale = " rationale"; return value }(), field: "rationale"},
		{name: "status", value: func() Decision { value := validDecision; value.Status = "bad"; return value }(), field: "status"},
		{name: "origin", value: func() Decision { value := validDecision; value.Origin = OriginInferred; return value }(), field: "origin"},
		{name: "version", value: func() Decision { value := validDecision; value.Version = 2; return value }(), field: "version"},
	} {
		t.Run("stored decision "+test.name, func(t *testing.T) {
			requireValidation(t, validateStoredDecision(scope, test.value), test.field)
		})
	}
	for _, value := range []struct {
		name string
		call func() error
	}{
		{name: "project", call: func() error { value := validProject; value.Origin = ""; return validateStoredProject(scope, value) }},
		{name: "task", call: func() error { value := validTask; value.Origin = ""; return validateStoredTask(scope, value) }},
		{name: "decision", call: func() error { value := validDecision; value.Origin = ""; return validateStoredDecision(scope, value) }},
	} {
		if err := value.call(); err != nil {
			t.Fatalf("empty origin %s error = %v", value.name, err)
		}
	}

	repo := NewMemoryRepository()
	directMutation := Mutation{IdempotencyKey: "direct-create", RequestHash: strings.Repeat("a", 64), At: now}
	createdProject, err := repo.CreateProject(testContext, scope, validProject, directMutation)
	if err != nil || !createdProject.CreatedAt.Equal(now) || !createdProject.UpdatedAt.Equal(now) {
		t.Fatalf("direct project defaults = %+v, %v", createdProject, err)
	}
	if _, err := repo.CreateProject(testContext, Scope{}, validProject, directMutation); err == nil {
		t.Fatal("CreateProject() accepted invalid scope")
	} else {
		requireValidation(t, err, "workspaceId")
	}
	badMutation := directMutation
	badMutation.RequestHash = "not-a-hash"
	if _, err := repo.CreateProject(testContext, scope, validProject, badMutation); err == nil {
		t.Fatal("CreateProject() accepted invalid mutation")
	} else {
		requireValidation(t, err, "requestHash")
	}

	if err := projectInScope(scope.WorkspaceID, scope.UserID, scope); err != true || projectInScope(otherScope.WorkspaceID, scope.UserID, scope) {
		t.Fatal("projectInScope() returned an incorrect result")
	}
	if got := mutationTime(Mutation{At: now}); !got.Equal(now) {
		t.Fatalf("mutationTime(non-zero) = %v, want %v", got, now)
	}
	if got := mutationTime(Mutation{}); got.IsZero() {
		t.Fatal("mutationTime(zero) returned zero")
	}
	if got := stableID(EntityProject, scope, "key"); got != stableID(EntityProject, scope, "key") || got == stableID(EntityProject, otherScope, "key") {
		t.Fatalf("stableID scope separation failed: %q", got)
	}

	if raw, err := marshalSnapshot(nil); err != nil || string(raw) != "{}" {
		t.Fatalf("marshalSnapshot(nil) = %q, %v", raw, err)
	}
	if raw, err := marshalSnapshot(validProject); err != nil || len(raw) == 0 {
		t.Fatalf("marshalSnapshot(value) = %q, %v", raw, err)
	}
	if _, err := marshalSnapshot(make(chan int)); err == nil {
		t.Fatal("marshalSnapshot(channel) unexpectedly succeeded")
	}
	if _, err := newMutation(OperationProjectCreate, "key", func() {}, now); err == nil {
		t.Fatal("newMutation(function payload) unexpectedly succeeded")
	}
	if got := (&Service{}).now(); got.IsZero() {
		t.Fatal("empty service clock returned zero")
	}
	var nilService *Service
	if got := nilService.now(); got.IsZero() {
		t.Fatal("nil service clock returned zero")
	}

	repo.mu.Lock()
	if err := repo.rememberLocked(scope, directMutation, OperationProjectCreate, EntityProject, "project-helper", func() {}); err == nil {
		t.Fatal("rememberLocked(function) unexpectedly succeeded")
	}
	if err := repo.appendHistoryLocked(scope, EntityProject, "project-helper", HistoryUpdated, 1, 2, "key", func() {}, nil, now); err == nil {
		t.Fatal("appendHistoryLocked(function before) unexpectedly succeeded")
	}
	if err := repo.appendHistoryLocked(scope, EntityProject, "project-helper", HistoryUpdated, 1, 2, "key", nil, func() {}, now); err == nil {
		t.Fatal("appendHistoryLocked(function after) unexpectedly succeeded")
	}
	repo.mu.Unlock()

	if _, err := repo.History(testContext, scope, EntityProject, "missing-history"); err == nil {
		t.Fatal("History() for missing entity unexpectedly succeeded")
	} else {
		requireNotFound(t, err, EntityProject, "missing-history")
	}
}

func TestTaskAndDecisionMutationReplay(t *testing.T) {
	service, _, now := newTestService(t)
	scope := testScope("workspace-replay", "user-replay")

	taskInput := CreateTaskInput{ID: "replay-task", Title: "Replay task", Description: "Description", Status: TaskTodo, Priority: PriorityNormal}
	task, err := service.CreateTask(testContext, scope, taskInput, "replay-task-create")
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	taskReplay, err := service.CreateTask(testContext, scope, taskInput, "replay-task-create")
	if err != nil || !reflect.DeepEqual(taskReplay, task) {
		t.Fatalf("CreateTask() replay = %+v, %v; want %+v", taskReplay, err, task)
	}
	title := "Replay task updated"
	task, err = service.UpdateTask(testContext, scope, task.ID, TaskPatch{Title: &title}, 1, "replay-task-update")
	if err != nil {
		t.Fatalf("UpdateTask() error = %v", err)
	}
	if replay, err := service.UpdateTask(testContext, scope, task.ID, TaskPatch{Title: &title}, 1, "replay-task-update"); err != nil || !reflect.DeepEqual(replay, task) {
		t.Fatalf("UpdateTask() replay = %+v, %v; want %+v", replay, err, task)
	}
	*now = now.Add(time.Hour)
	task, err = service.TrashTask(testContext, scope, task.ID, 2, "replay-task-trash")
	if err != nil {
		t.Fatalf("TrashTask() error = %v", err)
	}
	if replay, err := service.TrashTask(testContext, scope, task.ID, 2, "replay-task-trash"); err != nil || !reflect.DeepEqual(replay, task) {
		t.Fatalf("TrashTask() replay = %+v, %v; want %+v", replay, err, task)
	}
	task, err = service.RestoreTask(testContext, scope, task.ID, 3, "replay-task-restore")
	if err != nil {
		t.Fatalf("RestoreTask() error = %v", err)
	}
	if replay, err := service.RestoreTask(testContext, scope, task.ID, 3, "replay-task-restore"); err != nil || !reflect.DeepEqual(replay, task) {
		t.Fatalf("RestoreTask() replay = %+v, %v; want %+v", replay, err, task)
	}
	*now = now.Add(time.Hour)
	task, err = service.TrashTask(testContext, scope, task.ID, 4, "replay-task-trash-again")
	if err != nil {
		t.Fatalf("second TrashTask() error = %v", err)
	}
	*now = task.DeletedAt.Add(RetentionPeriod)
	purgeTask, err := service.PurgeTask(testContext, scope, task.ID, 5, "replay-task-purge")
	if err != nil {
		t.Fatalf("PurgeTask() error = %v", err)
	}
	if replay, err := service.PurgeTask(testContext, scope, task.ID, 5, "replay-task-purge"); err != nil || !reflect.DeepEqual(replay, purgeTask) {
		t.Fatalf("PurgeTask() replay = %+v, %v; want %+v", replay, err, purgeTask)
	}

	decisionInput := CreateDecisionInput{ID: "replay-decision", Title: "Replay decision", Context: "Context", Outcome: "Outcome", Rationale: "Rationale", Status: DecisionProposed}
	decision, err := service.CreateDecision(testContext, scope, decisionInput, "replay-decision-create")
	if err != nil {
		t.Fatalf("CreateDecision() error = %v", err)
	}
	if replay, err := service.CreateDecision(testContext, scope, decisionInput, "replay-decision-create"); err != nil || !reflect.DeepEqual(replay, decision) {
		t.Fatalf("CreateDecision() replay = %+v, %v; want %+v", replay, err, decision)
	}
	decisionTitle := "Replay decision updated"
	decision, err = service.UpdateDecision(testContext, scope, decision.ID, DecisionPatch{Title: &decisionTitle}, 1, "replay-decision-update")
	if err != nil {
		t.Fatalf("UpdateDecision() error = %v", err)
	}
	if replay, err := service.UpdateDecision(testContext, scope, decision.ID, DecisionPatch{Title: &decisionTitle}, 1, "replay-decision-update"); err != nil || !reflect.DeepEqual(replay, decision) {
		t.Fatalf("UpdateDecision() replay = %+v, %v; want %+v", replay, err, decision)
	}
	decision, err = service.TrashDecision(testContext, scope, decision.ID, 2, "replay-decision-trash")
	if err != nil {
		t.Fatalf("TrashDecision() error = %v", err)
	}
	if replay, err := service.TrashDecision(testContext, scope, decision.ID, 2, "replay-decision-trash"); err != nil || !reflect.DeepEqual(replay, decision) {
		t.Fatalf("TrashDecision() replay = %+v, %v; want %+v", replay, err, decision)
	}
	decision, err = service.RestoreDecision(testContext, scope, decision.ID, 3, "replay-decision-restore")
	if err != nil {
		t.Fatalf("RestoreDecision() error = %v", err)
	}
	if replay, err := service.RestoreDecision(testContext, scope, decision.ID, 3, "replay-decision-restore"); err != nil || !reflect.DeepEqual(replay, decision) {
		t.Fatalf("RestoreDecision() replay = %+v, %v; want %+v", replay, err, decision)
	}
	decision, err = service.TrashDecision(testContext, scope, decision.ID, 4, "replay-decision-trash-again")
	if err != nil {
		t.Fatalf("second TrashDecision() error = %v", err)
	}
	*now = decision.DeletedAt.Add(RetentionPeriod)
	purgeDecision, err := service.PurgeDecision(testContext, scope, decision.ID, 5, "replay-decision-purge")
	if err != nil {
		t.Fatalf("PurgeDecision() error = %v", err)
	}
	if replay, err := service.PurgeDecision(testContext, scope, decision.ID, 5, "replay-decision-purge"); err != nil || !reflect.DeepEqual(replay, purgeDecision) {
		t.Fatalf("PurgeDecision() replay = %+v, %v; want %+v", replay, err, purgeDecision)
	}
}

func TestServiceMutationValidationBranches(t *testing.T) {
	service, _, _ := newTestService(t)
	scope := testScope("workspace-service-mutations", "user-service-mutations")
	projectInput := CreateProjectInput{Name: "Project"}
	taskInput := CreateTaskInput{Title: "Task"}
	decisionInput := CreateDecisionInput{Title: "Decision", Outcome: "Outcome"}
	projectPatch := ProjectPatch{Name: stringPointer("Project updated")}
	taskPatch := TaskPatch{Title: stringPointer("Task updated")}
	decisionPatch := DecisionPatch{Title: stringPointer("Decision updated")}

	if _, err := service.CreateProject(testContext, scope, projectInput, ""); err == nil {
		t.Fatal("CreateProject() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}
	if _, err := service.CreateTask(testContext, scope, taskInput, ""); err == nil {
		t.Fatal("CreateTask() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}
	if _, err := service.CreateDecision(testContext, scope, decisionInput, ""); err == nil {
		t.Fatal("CreateDecision() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}

	if _, err := service.UpdateProject(testContext, scope, "project", projectPatch, 1, ""); err == nil {
		t.Fatal("UpdateProject() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}
	if _, err := service.UpdateTask(testContext, scope, "task", taskPatch, 1, ""); err == nil {
		t.Fatal("UpdateTask() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}
	if _, err := service.UpdateDecision(testContext, scope, "decision", decisionPatch, 1, ""); err == nil {
		t.Fatal("UpdateDecision() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}

	if _, err := service.TrashProject(testContext, scope, "project", 1, ""); err == nil {
		t.Fatal("TrashProject() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}
	if _, err := service.RestoreProject(testContext, scope, "project", 1, ""); err == nil {
		t.Fatal("RestoreProject() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}
	if _, err := service.PurgeProject(testContext, scope, "project", 1, ""); err == nil {
		t.Fatal("PurgeProject() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}
	if _, err := service.TrashTask(testContext, scope, "task", 1, ""); err == nil {
		t.Fatal("TrashTask() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}
	if _, err := service.RestoreTask(testContext, scope, "task", 1, ""); err == nil {
		t.Fatal("RestoreTask() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}
	if _, err := service.PurgeTask(testContext, scope, "task", 1, ""); err == nil {
		t.Fatal("PurgeTask() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}
	if _, err := service.TrashDecision(testContext, scope, "decision", 1, ""); err == nil {
		t.Fatal("TrashDecision() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}
	if _, err := service.RestoreDecision(testContext, scope, "decision", 1, ""); err == nil {
		t.Fatal("RestoreDecision() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}
	if _, err := service.PurgeDecision(testContext, scope, "decision", 1, ""); err == nil {
		t.Fatal("PurgeDecision() accepted an empty idempotency key")
	} else {
		requireValidation(t, err, "idempotencyKey")
	}

	if _, err := service.ListTasks(testContext, scope, "", ListOptions{Limit: 201}); err == nil {
		t.Fatal("ListTasks() accepted an invalid limit")
	} else {
		requireValidation(t, err, "limit")
	}
	if _, err := service.ListDecisions(testContext, scope, "", ListOptions{Limit: 201}); err == nil {
		t.Fatal("ListDecisions() accepted an invalid limit")
	} else {
		requireValidation(t, err, "limit")
	}
}

func TestServiceTrashRestorePurgeValidationBranches(t *testing.T) {
	service, _, _ := newTestService(t)
	scope := testScope("workspace-service-lifecycle-validation", "user-service-lifecycle-validation")
	for _, test := range []struct {
		name  string
		call  func(Scope, int64, string) error
		cause string
	}{
		{name: "task trash scope", call: func(scope Scope, version int64, key string) error {
			_, err := service.TrashTask(testContext, scope, "task", version, key)
			return err
		}, cause: "workspaceId"},
		{name: "task trash version", call: func(scope Scope, version int64, key string) error {
			_, err := service.TrashTask(testContext, scope, "task", version, key)
			return err
		}, cause: "expectedVersion"},
		{name: "task restore scope", call: func(scope Scope, version int64, key string) error {
			_, err := service.RestoreTask(testContext, scope, "task", version, key)
			return err
		}, cause: "workspaceId"},
		{name: "task restore version", call: func(scope Scope, version int64, key string) error {
			_, err := service.RestoreTask(testContext, scope, "task", version, key)
			return err
		}, cause: "expectedVersion"},
		{name: "task purge scope", call: func(scope Scope, version int64, key string) error {
			_, err := service.PurgeTask(testContext, scope, "task", version, key)
			return err
		}, cause: "workspaceId"},
		{name: "task purge version", call: func(scope Scope, version int64, key string) error {
			_, err := service.PurgeTask(testContext, scope, "task", version, key)
			return err
		}, cause: "expectedVersion"},
		{name: "decision trash scope", call: func(scope Scope, version int64, key string) error {
			_, err := service.TrashDecision(testContext, scope, "decision", version, key)
			return err
		}, cause: "workspaceId"},
		{name: "decision trash version", call: func(scope Scope, version int64, key string) error {
			_, err := service.TrashDecision(testContext, scope, "decision", version, key)
			return err
		}, cause: "expectedVersion"},
		{name: "decision restore scope", call: func(scope Scope, version int64, key string) error {
			_, err := service.RestoreDecision(testContext, scope, "decision", version, key)
			return err
		}, cause: "workspaceId"},
		{name: "decision restore version", call: func(scope Scope, version int64, key string) error {
			_, err := service.RestoreDecision(testContext, scope, "decision", version, key)
			return err
		}, cause: "expectedVersion"},
		{name: "decision purge scope", call: func(scope Scope, version int64, key string) error {
			_, err := service.PurgeDecision(testContext, scope, "decision", version, key)
			return err
		}, cause: "workspaceId"},
		{name: "decision purge version", call: func(scope Scope, version int64, key string) error {
			_, err := service.PurgeDecision(testContext, scope, "decision", version, key)
			return err
		}, cause: "expectedVersion"},
	} {
		t.Run(test.name, func(t *testing.T) {
			requestScope := scope
			version := int64(1)
			if strings.HasSuffix(test.name, "scope") {
				requestScope = Scope{}
			}
			if strings.HasSuffix(test.name, "version") {
				version = 0
			}
			requireValidation(t, test.call(requestScope, version, "valid-key"), test.cause)
		})
	}
}

func TestTaskDecisionScopeIsolationAndSameIDReuse(t *testing.T) {
	service, _, now := newTestService(t)
	scopeA := testScope("workspace-child-shared", "user-child-a")
	scopeB := testScope("workspace-child-shared", "user-child-b")
	scopeC := testScope("workspace-child-other", "user-child-c")

	taskA := mustCreateTask(t, service, scopeA, "shared-task-id", "", "same-task-key")
	*now = now.Add(time.Hour)
	taskA, err := service.TrashTask(testContext, scopeA, taskA.ID, taskA.Version, "same-task-trash")
	if err != nil {
		t.Fatalf("TrashTask(scope A) error = %v", err)
	}
	*now = taskA.DeletedAt.Add(RetentionPeriod)
	if _, err := service.PurgeTask(testContext, scopeA, taskA.ID, taskA.Version, "same-task-purge"); err != nil {
		t.Fatalf("PurgeTask(scope A) error = %v", err)
	}

	decisionA := mustCreateDecision(t, service, scopeA, "shared-decision-id", "", "same-decision-key")
	*now = now.Add(time.Hour)
	decisionA, err = service.TrashDecision(testContext, scopeA, decisionA.ID, decisionA.Version, "same-decision-trash")
	if err != nil {
		t.Fatalf("TrashDecision(scope A) error = %v", err)
	}
	*now = decisionA.DeletedAt.Add(RetentionPeriod)
	if _, err := service.PurgeDecision(testContext, scopeA, decisionA.ID, decisionA.Version, "same-decision-purge"); err != nil {
		t.Fatalf("PurgeDecision(scope A) error = %v", err)
	}

	taskB := mustCreateTask(t, service, scopeB, taskA.ID, "", "same-task-key")
	taskC := mustCreateTask(t, service, scopeC, taskA.ID, "", "same-task-key")
	decisionB := mustCreateDecision(t, service, scopeB, decisionA.ID, "", "same-decision-key")
	decisionC := mustCreateDecision(t, service, scopeC, decisionA.ID, "", "same-decision-key")
	if got, err := service.GetTask(testContext, scopeB, taskA.ID); err != nil || got.OwnerUserID != scopeB.UserID {
		t.Fatalf("GetTask(scope B) = %+v, %v", got, err)
	}
	if got, err := service.GetTask(testContext, scopeC, taskA.ID); err != nil || got.WorkspaceID != scopeC.WorkspaceID {
		t.Fatalf("GetTask(scope C) = %+v, %v", got, err)
	}
	if _, err := service.GetTask(testContext, scopeA, taskA.ID); err == nil {
		t.Fatal("GetTask(scope A) found a purged record")
	} else {
		requireNotFound(t, err, EntityTask, taskA.ID)
	}
	if got, err := service.GetDecision(testContext, scopeB, decisionA.ID); err != nil || got.OwnerUserID != scopeB.UserID {
		t.Fatalf("GetDecision(scope B) = %+v, %v", got, err)
	}
	if got, err := service.GetDecision(testContext, scopeC, decisionA.ID); err != nil || got.WorkspaceID != scopeC.WorkspaceID {
		t.Fatalf("GetDecision(scope C) = %+v, %v", got, err)
	}
	if _, err := service.GetDecision(testContext, scopeA, decisionA.ID); err == nil {
		t.Fatal("GetDecision(scope A) found a purged record")
	} else {
		requireNotFound(t, err, EntityDecision, decisionA.ID)
	}
	if items, err := service.ListTasks(testContext, scopeB, "", ListOptions{}); err != nil || len(items) != 1 || items[0].ID != taskB.ID {
		t.Fatalf("ListTasks(scope B) = %+v, %v", items, err)
	}
	if items, err := service.ListTasks(testContext, scopeC, "", ListOptions{}); err != nil || len(items) != 1 || items[0].ID != taskC.ID {
		t.Fatalf("ListTasks(scope C) = %+v, %v", items, err)
	}
	if items, err := service.ListTasks(testContext, scopeA, "", ListOptions{}); err != nil || len(items) != 0 {
		t.Fatalf("ListTasks(scope A) = %+v, %v", items, err)
	}
	if items, err := service.ListDecisions(testContext, scopeB, "", ListOptions{}); err != nil || len(items) != 1 || items[0].ID != decisionB.ID {
		t.Fatalf("ListDecisions(scope B) = %+v, %v", items, err)
	}
	if items, err := service.ListDecisions(testContext, scopeC, "", ListOptions{}); err != nil || len(items) != 1 || items[0].ID != decisionC.ID {
		t.Fatalf("ListDecisions(scope C) = %+v, %v", items, err)
	}
	if items, err := service.ListDecisions(testContext, scopeA, "", ListOptions{}); err != nil || len(items) != 0 {
		t.Fatalf("ListDecisions(scope A) = %+v, %v", items, err)
	}

	for _, test := range []struct {
		name       string
		scope      Scope
		entityType EntityType
		id         string
		wantUser   string
		wantWork   string
	}{
		{name: "task B", scope: scopeB, entityType: EntityTask, id: taskB.ID, wantUser: scopeB.UserID, wantWork: scopeB.WorkspaceID},
		{name: "task C", scope: scopeC, entityType: EntityTask, id: taskC.ID, wantUser: scopeC.UserID, wantWork: scopeC.WorkspaceID},
		{name: "decision B", scope: scopeB, entityType: EntityDecision, id: decisionB.ID, wantUser: scopeB.UserID, wantWork: scopeB.WorkspaceID},
		{name: "decision C", scope: scopeC, entityType: EntityDecision, id: decisionC.ID, wantUser: scopeC.UserID, wantWork: scopeC.WorkspaceID},
	} {
		t.Run(test.name, func(t *testing.T) {
			history, err := service.History(testContext, test.scope, test.entityType, test.id)
			if err != nil || len(history) != 1 {
				t.Fatalf("History() = %+v, %v", history, err)
			}
			if history[0].ActorUserID != test.wantUser || history[0].WorkspaceID != test.wantWork {
				t.Fatalf("History() scope = %+v", history[0])
			}
		})
	}
	if history, err := service.History(testContext, scopeA, EntityTask, taskA.ID); err != nil || len(history) != 3 || history[2].Action != HistoryPurged {
		t.Fatalf("purged task history = %+v, %v", history, err)
	}
	if history, err := service.History(testContext, scopeA, EntityDecision, decisionA.ID); err != nil || len(history) != 3 || history[2].Action != HistoryPurged {
		t.Fatalf("purged decision history = %+v, %v", history, err)
	}
}

func TestMemoryRepositoryWriteScopeValidation(t *testing.T) {
	_, repo, now := newTestService(t)
	invalidScope := Scope{}
	mutation := testMutation("invalid-scope", *now)
	project := Project{ID: "project", Version: 1}
	task := Task{ID: "task", Version: 1}
	decision := Decision{ID: "decision", Version: 1}

	for _, test := range []struct {
		name string
		call func() error
	}{
		{name: "create project", call: func() error {
			_, err := repo.CreateProject(testContext, invalidScope, project, mutation)
			return err
		}},
		{name: "update project", call: func() error {
			_, err := repo.UpdateProject(testContext, invalidScope, "project", ProjectPatch{}, 1, mutation)
			return err
		}},
		{name: "trash project", call: func() error {
			_, err := repo.TrashProject(testContext, invalidScope, "project", 1, mutation)
			return err
		}},
		{name: "restore project", call: func() error {
			_, err := repo.RestoreProject(testContext, invalidScope, "project", 1, mutation)
			return err
		}},
		{name: "purge project", call: func() error {
			_, err := repo.PurgeProject(testContext, invalidScope, "project", 1, mutation)
			return err
		}},
		{name: "create task", call: func() error {
			_, err := repo.CreateTask(testContext, invalidScope, task, mutation)
			return err
		}},
		{name: "update task", call: func() error {
			_, err := repo.UpdateTask(testContext, invalidScope, "task", TaskPatch{}, 1, mutation)
			return err
		}},
		{name: "trash task", call: func() error {
			_, err := repo.TrashTask(testContext, invalidScope, "task", 1, mutation)
			return err
		}},
		{name: "restore task", call: func() error {
			_, err := repo.RestoreTask(testContext, invalidScope, "task", 1, mutation)
			return err
		}},
		{name: "purge task", call: func() error {
			_, err := repo.PurgeTask(testContext, invalidScope, "task", 1, mutation)
			return err
		}},
		{name: "create decision", call: func() error {
			_, err := repo.CreateDecision(testContext, invalidScope, decision, mutation)
			return err
		}},
		{name: "update decision", call: func() error {
			_, err := repo.UpdateDecision(testContext, invalidScope, "decision", DecisionPatch{}, 1, mutation)
			return err
		}},
		{name: "trash decision", call: func() error {
			_, err := repo.TrashDecision(testContext, invalidScope, "decision", 1, mutation)
			return err
		}},
		{name: "restore decision", call: func() error {
			_, err := repo.RestoreDecision(testContext, invalidScope, "decision", 1, mutation)
			return err
		}},
		{name: "purge decision", call: func() error {
			_, err := repo.PurgeDecision(testContext, invalidScope, "decision", 1, mutation)
			return err
		}},
		{name: "history", call: func() error {
			_, err := repo.History(testContext, invalidScope, EntityProject, "project")
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			requireValidation(t, test.call(), "workspaceId")
		})
	}
}

func TestMemoryRepositoryProjectErrorBranches(t *testing.T) {
	service, repo, now := newTestService(t)
	scope := testScope("workspace-project-errors", "user-project-errors")
	otherScope := testScope("workspace-project-errors-other", "user-project-errors-other")
	project := mustCreateProject(t, service, scope, "project-errors", "project-errors-create")
	_ = mustCreateProject(t, service, scope, "project-errors-second", "project-errors-second-create")

	if items, err := repo.ListProjects(testContext, scope, ListOptions{Limit: 1}); err != nil || len(items) != 1 {
		t.Fatalf("ListProjects(limit) = %+v, %v", items, err)
	}
	if _, err := repo.ListProjects(testContext, scope, ListOptions{Limit: 201}); err == nil {
		t.Fatal("ListProjects() accepted an invalid limit")
	} else {
		requireValidation(t, err, "limit")
	}
	if _, err := repo.GetProject(testContext, otherScope, project.ID); err == nil {
		t.Fatal("GetProject() crossed a workspace boundary")
	} else {
		requireNotFound(t, err, EntityProject, project.ID)
	}

	invalid := project
	invalid.ID = "project-errors-invalid"
	invalid.Name = ""
	if _, err := repo.CreateProject(testContext, scope, invalid, testMutation("project-errors-invalid", *now)); err == nil {
		t.Fatal("CreateProject() accepted invalid stored data")
	} else {
		requireValidation(t, err, "name")
	}
	if _, err := repo.CreateProject(testContext, scope, project, testMutation("project-errors-duplicate", *now)); err == nil {
		t.Fatal("CreateProject() accepted a duplicate entity")
	} else {
		requireIs(t, err, ErrAlreadyExists)
	}

	name := "Updated project"
	if _, err := repo.UpdateProject(testContext, scope, project.ID, ProjectPatch{}, 1, testMutation("project-errors-empty-patch", *now)); err == nil {
		t.Fatal("UpdateProject() accepted an empty patch")
	} else {
		requireValidation(t, err, "patch")
	}
	if _, err := repo.UpdateProject(testContext, scope, "missing-project", ProjectPatch{Name: &name}, 1, testMutation("project-errors-update-missing", *now)); err == nil {
		t.Fatal("UpdateProject() accepted a missing entity")
	} else {
		requireNotFound(t, err, EntityProject, "missing-project")
	}
	if _, err := repo.UpdateProject(testContext, scope, project.ID, ProjectPatch{Name: &name}, 0, testMutation("project-errors-update-stale", *now)); err == nil {
		t.Fatal("UpdateProject() accepted a stale version")
	} else {
		requireVersionConflict(t, err, EntityProject, project.ID, 0, 1)
	}

	if _, err := repo.TrashProject(testContext, scope, "missing-project", 1, testMutation("project-errors-trash-missing", *now)); err == nil {
		t.Fatal("TrashProject() accepted a missing entity")
	} else {
		requireNotFound(t, err, EntityProject, "missing-project")
	}
	if _, err := repo.TrashProject(testContext, scope, project.ID, 0, testMutation("project-errors-trash-stale", *now)); err == nil {
		t.Fatal("TrashProject() accepted a stale version")
	} else {
		requireVersionConflict(t, err, EntityProject, project.ID, 0, 1)
	}
	trashed, err := repo.TrashProject(testContext, scope, project.ID, 1, testMutation("project-errors-trash", now.Add(time.Hour)))
	if err != nil {
		t.Fatalf("TrashProject() error = %v", err)
	}
	if _, err := repo.RestoreProject(testContext, scope, project.ID, 1, testMutation("project-errors-restore-stale", *now)); err == nil {
		t.Fatal("RestoreProject() accepted a stale version")
	} else {
		requireVersionConflict(t, err, EntityProject, project.ID, 1, trashed.Version)
	}
	restored, err := repo.RestoreProject(testContext, scope, project.ID, trashed.Version, testMutation("project-errors-restore", now.Add(2*time.Hour)))
	if err != nil {
		t.Fatalf("RestoreProject() error = %v", err)
	}
	if _, err := repo.RestoreProject(testContext, scope, project.ID, restored.Version, testMutation("project-errors-restore-active", *now)); err == nil {
		t.Fatal("RestoreProject() accepted an active entity")
	} else {
		requireState(t, err, ErrNotTrashed)
	}
	if _, err := repo.PurgeProject(testContext, scope, project.ID, restored.Version, testMutation("project-errors-purge-active", *now)); err == nil {
		t.Fatal("PurgeProject() accepted an active entity")
	} else {
		requireState(t, err, ErrNotTrashed)
	}

	trashedAgain, err := repo.TrashProject(testContext, scope, project.ID, restored.Version, testMutation("project-errors-trash-again", now.Add(3*time.Hour)))
	if err != nil {
		t.Fatalf("second TrashProject() error = %v", err)
	}
	if _, err := repo.PurgeProject(testContext, scope, project.ID, restored.Version, testMutation("project-errors-purge-stale", *now)); err == nil {
		t.Fatal("PurgeProject() accepted a stale version")
	} else {
		requireVersionConflict(t, err, EntityProject, project.ID, restored.Version, trashedAgain.Version)
	}
	if _, err := repo.PurgeProject(testContext, scope, project.ID, trashedAgain.Version, testMutation("project-errors-purge-early", trashedAgain.DeletedAt.Add(RetentionPeriod-time.Hour))); err == nil {
		t.Fatal("PurgeProject() ignored retention")
	} else {
		requireIs(t, err, ErrPurgeNotReady)
	}
	if _, err := repo.PurgeProject(testContext, scope, project.ID, trashedAgain.Version, testMutation("project-errors-purge", trashedAgain.DeletedAt.Add(RetentionPeriod))); err != nil {
		t.Fatalf("PurgeProject() error = %v", err)
	}

	for _, test := range []struct {
		name string
		call func() error
	}{
		{name: "update after purge", call: func() error {
			_, err := repo.UpdateProject(testContext, scope, project.ID, ProjectPatch{Name: &name}, trashedAgain.Version, testMutation("project-errors-update-after-purge", *now))
			return err
		}},
		{name: "trash after purge", call: func() error {
			_, err := repo.TrashProject(testContext, scope, project.ID, trashedAgain.Version, testMutation("project-errors-trash-after-purge", *now))
			return err
		}},
		{name: "restore after purge", call: func() error {
			_, err := repo.RestoreProject(testContext, scope, project.ID, trashedAgain.Version, testMutation("project-errors-restore-after-purge", *now))
			return err
		}},
		{name: "purge after purge", call: func() error {
			_, err := repo.PurgeProject(testContext, scope, project.ID, trashedAgain.Version, testMutation("project-errors-purge-after-purge", *now))
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			requireNotFound(t, test.call(), EntityProject, project.ID)
		})
	}
}

func TestMemoryRepositoryTaskDecisionErrorBranches(t *testing.T) {
	service, repo, now := newTestService(t)
	scope := testScope("workspace-child-errors", "user-child-errors")
	otherScope := testScope("workspace-child-errors-other", "user-child-errors-other")
	project := mustCreateProject(t, service, scope, "child-errors-project", "child-errors-project-create")
	trashedProject := mustCreateProject(t, service, scope, "child-errors-trashed-project", "child-errors-trashed-project-create")
	if _, err := service.TrashProject(testContext, scope, trashedProject.ID, 1, "child-errors-trashed-project-trash"); err != nil {
		t.Fatalf("TrashProject() setup error = %v", err)
	}

	task := mustCreateTask(t, service, scope, "child-errors-task", "", "child-errors-task-create")
	_ = mustCreateTask(t, service, scope, "child-errors-task-second", project.ID, "child-errors-task-second-create")
	if items, err := repo.ListTasks(testContext, scope, "", ListOptions{Limit: 1}); err != nil || len(items) != 1 {
		t.Fatalf("ListTasks(limit) = %+v, %v", items, err)
	}
	if _, err := repo.ListTasks(testContext, scope, "missing-project", ListOptions{}); err == nil {
		t.Fatal("ListTasks() accepted a missing project filter")
	} else {
		requireNotFound(t, err, EntityProject, "missing-project")
	}
	if _, err := repo.ListTasks(testContext, scope, "", ListOptions{Limit: 201}); err == nil {
		t.Fatal("ListTasks() accepted an invalid limit")
	} else {
		requireValidation(t, err, "limit")
	}
	if _, err := repo.GetTask(testContext, otherScope, task.ID); err == nil {
		t.Fatal("GetTask() crossed a workspace boundary")
	} else {
		requireNotFound(t, err, EntityTask, task.ID)
	}

	invalidTask := task
	invalidTask.ID = "child-errors-invalid-task"
	invalidTask.Title = ""
	if _, err := repo.CreateTask(testContext, scope, invalidTask, testMutation("child-errors-invalid-task", *now)); err == nil {
		t.Fatal("CreateTask() accepted invalid stored data")
	} else {
		requireValidation(t, err, "title")
	}
	if _, err := repo.CreateTask(testContext, scope, task, testMutation("child-errors-task-duplicate", *now)); err == nil {
		t.Fatal("CreateTask() accepted a duplicate entity")
	} else {
		requireIs(t, err, ErrAlreadyExists)
	}
	emptyTaskPatch := TaskPatch{}
	if _, err := repo.UpdateTask(testContext, scope, task.ID, emptyTaskPatch, 1, testMutation("child-errors-task-empty-patch", *now)); err == nil {
		t.Fatal("UpdateTask() accepted an empty patch")
	} else {
		requireValidation(t, err, "patch")
	}
	taskTitle := "Updated task"
	if _, err := repo.UpdateTask(testContext, scope, "missing-task", TaskPatch{Title: &taskTitle}, 1, testMutation("child-errors-task-update-missing", *now)); err == nil {
		t.Fatal("UpdateTask() accepted a missing entity")
	} else {
		requireNotFound(t, err, EntityTask, "missing-task")
	}
	if _, err := repo.UpdateTask(testContext, scope, task.ID, TaskPatch{Title: &taskTitle}, 0, testMutation("child-errors-task-update-stale", *now)); err == nil {
		t.Fatal("UpdateTask() accepted a stale version")
	} else {
		requireVersionConflict(t, err, EntityTask, task.ID, 0, 1)
	}
	missingProject := "missing-project"
	if _, err := repo.UpdateTask(testContext, scope, task.ID, TaskPatch{ProjectID: &missingProject}, 1, testMutation("child-errors-task-missing-project", *now)); err == nil {
		t.Fatal("UpdateTask() accepted a missing project link")
	} else {
		requireNotFound(t, err, EntityProject, missingProject)
	}
	if _, err := repo.UpdateTask(testContext, scope, task.ID, TaskPatch{ProjectID: &trashedProject.ID}, 1, testMutation("child-errors-task-trashed-project", *now)); err == nil {
		t.Fatal("UpdateTask() accepted a trashed project link")
	} else {
		requireState(t, err, ErrRecordTrashed)
	}
	if _, err := repo.TrashTask(testContext, scope, "missing-task", 1, testMutation("child-errors-task-trash-missing", *now)); err == nil {
		t.Fatal("TrashTask() accepted a missing entity")
	} else {
		requireNotFound(t, err, EntityTask, "missing-task")
	}
	if _, err := repo.TrashTask(testContext, scope, task.ID, 0, testMutation("child-errors-task-trash-stale", *now)); err == nil {
		t.Fatal("TrashTask() accepted a stale version")
	} else {
		requireVersionConflict(t, err, EntityTask, task.ID, 0, 1)
	}
	trashedTask, err := repo.TrashTask(testContext, scope, task.ID, 1, testMutation("child-errors-task-trash", now.Add(time.Hour)))
	if err != nil {
		t.Fatalf("TrashTask() error = %v", err)
	}
	if _, err := repo.TrashTask(testContext, scope, task.ID, trashedTask.Version, testMutation("child-errors-task-trash-again", *now)); err == nil {
		t.Fatal("TrashTask() accepted an already trashed entity")
	} else {
		requireState(t, err, ErrAlreadyTrashed)
	}
	if _, err := repo.RestoreTask(testContext, scope, task.ID, 1, testMutation("child-errors-task-restore-stale", *now)); err == nil {
		t.Fatal("RestoreTask() accepted a stale version")
	} else {
		requireVersionConflict(t, err, EntityTask, task.ID, 1, trashedTask.Version)
	}
	restoredTask, err := repo.RestoreTask(testContext, scope, task.ID, trashedTask.Version, testMutation("child-errors-task-restore", now.Add(2*time.Hour)))
	if err != nil {
		t.Fatalf("RestoreTask() error = %v", err)
	}
	if _, err := repo.RestoreTask(testContext, scope, task.ID, restoredTask.Version, testMutation("child-errors-task-restore-active", *now)); err == nil {
		t.Fatal("RestoreTask() accepted an active entity")
	} else {
		requireState(t, err, ErrNotTrashed)
	}
	if _, err := repo.PurgeTask(testContext, scope, task.ID, restoredTask.Version, testMutation("child-errors-task-purge-active", *now)); err == nil {
		t.Fatal("PurgeTask() accepted an active entity")
	} else {
		requireState(t, err, ErrNotTrashed)
	}
	trashedTaskAgain, err := repo.TrashTask(testContext, scope, task.ID, restoredTask.Version, testMutation("child-errors-task-trash-again-valid", now.Add(3*time.Hour)))
	if err != nil {
		t.Fatalf("second TrashTask() error = %v", err)
	}
	if _, err := repo.PurgeTask(testContext, scope, task.ID, restoredTask.Version, testMutation("child-errors-task-purge-stale", *now)); err == nil {
		t.Fatal("PurgeTask() accepted a stale version")
	} else {
		requireVersionConflict(t, err, EntityTask, task.ID, restoredTask.Version, trashedTaskAgain.Version)
	}
	if _, err := repo.PurgeTask(testContext, scope, task.ID, trashedTaskAgain.Version, testMutation("child-errors-task-purge-early", trashedTaskAgain.DeletedAt.Add(RetentionPeriod-time.Hour))); err == nil {
		t.Fatal("PurgeTask() ignored retention")
	} else {
		requireIs(t, err, ErrPurgeNotReady)
	}
	if _, err := repo.PurgeTask(testContext, scope, task.ID, trashedTaskAgain.Version, testMutation("child-errors-task-purge", trashedTaskAgain.DeletedAt.Add(RetentionPeriod))); err != nil {
		t.Fatalf("PurgeTask() error = %v", err)
	}

	decision := mustCreateDecision(t, service, scope, "child-errors-decision", "", "child-errors-decision-create")
	_ = mustCreateDecision(t, service, scope, "child-errors-decision-second", project.ID, "child-errors-decision-second-create")
	if items, err := repo.ListDecisions(testContext, scope, "", ListOptions{Limit: 1}); err != nil || len(items) != 1 {
		t.Fatalf("ListDecisions(limit) = %+v, %v", items, err)
	}
	if _, err := repo.ListDecisions(testContext, scope, "missing-project", ListOptions{}); err == nil {
		t.Fatal("ListDecisions() accepted a missing project filter")
	} else {
		requireNotFound(t, err, EntityProject, "missing-project")
	}
	if _, err := repo.ListDecisions(testContext, scope, "", ListOptions{Limit: 201}); err == nil {
		t.Fatal("ListDecisions() accepted an invalid limit")
	} else {
		requireValidation(t, err, "limit")
	}
	if _, err := repo.GetDecision(testContext, otherScope, decision.ID); err == nil {
		t.Fatal("GetDecision() crossed a workspace boundary")
	} else {
		requireNotFound(t, err, EntityDecision, decision.ID)
	}

	invalidDecision := decision
	invalidDecision.ID = "child-errors-invalid-decision"
	invalidDecision.Outcome = ""
	if _, err := repo.CreateDecision(testContext, scope, invalidDecision, testMutation("child-errors-invalid-decision", *now)); err == nil {
		t.Fatal("CreateDecision() accepted invalid stored data")
	} else {
		requireValidation(t, err, "outcome")
	}
	if _, err := repo.CreateDecision(testContext, scope, decision, testMutation("child-errors-decision-duplicate", *now)); err == nil {
		t.Fatal("CreateDecision() accepted a duplicate entity")
	} else {
		requireIs(t, err, ErrAlreadyExists)
	}
	if _, err := repo.UpdateDecision(testContext, scope, decision.ID, DecisionPatch{}, 1, testMutation("child-errors-decision-empty-patch", *now)); err == nil {
		t.Fatal("UpdateDecision() accepted an empty patch")
	} else {
		requireValidation(t, err, "patch")
	}
	decisionTitle := "Updated decision"
	if _, err := repo.UpdateDecision(testContext, scope, "missing-decision", DecisionPatch{Title: &decisionTitle}, 1, testMutation("child-errors-decision-update-missing", *now)); err == nil {
		t.Fatal("UpdateDecision() accepted a missing entity")
	} else {
		requireNotFound(t, err, EntityDecision, "missing-decision")
	}
	if _, err := repo.UpdateDecision(testContext, scope, decision.ID, DecisionPatch{Title: &decisionTitle}, 0, testMutation("child-errors-decision-update-stale", *now)); err == nil {
		t.Fatal("UpdateDecision() accepted a stale version")
	} else {
		requireVersionConflict(t, err, EntityDecision, decision.ID, 0, 1)
	}
	if _, err := repo.UpdateDecision(testContext, scope, decision.ID, DecisionPatch{ProjectID: &missingProject}, 1, testMutation("child-errors-decision-missing-project", *now)); err == nil {
		t.Fatal("UpdateDecision() accepted a missing project link")
	} else {
		requireNotFound(t, err, EntityProject, missingProject)
	}
	if _, err := repo.TrashDecision(testContext, scope, "missing-decision", 1, testMutation("child-errors-decision-trash-missing", *now)); err == nil {
		t.Fatal("TrashDecision() accepted a missing entity")
	} else {
		requireNotFound(t, err, EntityDecision, "missing-decision")
	}
	if _, err := repo.TrashDecision(testContext, scope, decision.ID, 0, testMutation("child-errors-decision-trash-stale", *now)); err == nil {
		t.Fatal("TrashDecision() accepted a stale version")
	} else {
		requireVersionConflict(t, err, EntityDecision, decision.ID, 0, 1)
	}
	trashedDecision, err := repo.TrashDecision(testContext, scope, decision.ID, 1, testMutation("child-errors-decision-trash", now.Add(time.Hour)))
	if err != nil {
		t.Fatalf("TrashDecision() error = %v", err)
	}
	if _, err := repo.TrashDecision(testContext, scope, decision.ID, trashedDecision.Version, testMutation("child-errors-decision-trash-again", *now)); err == nil {
		t.Fatal("TrashDecision() accepted an already trashed entity")
	} else {
		requireState(t, err, ErrAlreadyTrashed)
	}
	if _, err := repo.RestoreDecision(testContext, scope, decision.ID, 1, testMutation("child-errors-decision-restore-stale", *now)); err == nil {
		t.Fatal("RestoreDecision() accepted a stale version")
	} else {
		requireVersionConflict(t, err, EntityDecision, decision.ID, 1, trashedDecision.Version)
	}
	restoredDecision, err := repo.RestoreDecision(testContext, scope, decision.ID, trashedDecision.Version, testMutation("child-errors-decision-restore", now.Add(2*time.Hour)))
	if err != nil {
		t.Fatalf("RestoreDecision() error = %v", err)
	}
	if _, err := repo.RestoreDecision(testContext, scope, decision.ID, restoredDecision.Version, testMutation("child-errors-decision-restore-active", *now)); err == nil {
		t.Fatal("RestoreDecision() accepted an active entity")
	} else {
		requireState(t, err, ErrNotTrashed)
	}
	if _, err := repo.PurgeDecision(testContext, scope, decision.ID, restoredDecision.Version, testMutation("child-errors-decision-purge-active", *now)); err == nil {
		t.Fatal("PurgeDecision() accepted an active entity")
	} else {
		requireState(t, err, ErrNotTrashed)
	}
	trashedDecisionAgain, err := repo.TrashDecision(testContext, scope, decision.ID, restoredDecision.Version, testMutation("child-errors-decision-trash-again-valid", now.Add(3*time.Hour)))
	if err != nil {
		t.Fatalf("second TrashDecision() error = %v", err)
	}
	if _, err := repo.PurgeDecision(testContext, scope, decision.ID, restoredDecision.Version, testMutation("child-errors-decision-purge-stale", *now)); err == nil {
		t.Fatal("PurgeDecision() accepted a stale version")
	} else {
		requireVersionConflict(t, err, EntityDecision, decision.ID, restoredDecision.Version, trashedDecisionAgain.Version)
	}
	if _, err := repo.PurgeDecision(testContext, scope, decision.ID, trashedDecisionAgain.Version, testMutation("child-errors-decision-purge-early", trashedDecisionAgain.DeletedAt.Add(RetentionPeriod-time.Hour))); err == nil {
		t.Fatal("PurgeDecision() ignored retention")
	} else {
		requireIs(t, err, ErrPurgeNotReady)
	}
	if _, err := repo.PurgeDecision(testContext, scope, decision.ID, trashedDecisionAgain.Version, testMutation("child-errors-decision-purge", trashedDecisionAgain.DeletedAt.Add(RetentionPeriod))); err != nil {
		t.Fatalf("PurgeDecision() error = %v", err)
	}

	for _, test := range []struct {
		name string
		call func() error
	}{
		{name: "task update after purge", call: func() error {
			_, err := repo.UpdateTask(testContext, scope, task.ID, TaskPatch{Title: &taskTitle}, 4, testMutation("child-errors-task-update-after-purge", *now))
			return err
		}},
		{name: "task trash after purge", call: func() error {
			_, err := repo.TrashTask(testContext, scope, task.ID, 4, testMutation("child-errors-task-trash-after-purge", *now))
			return err
		}},
		{name: "task restore after purge", call: func() error {
			_, err := repo.RestoreTask(testContext, scope, task.ID, 4, testMutation("child-errors-task-restore-after-purge", *now))
			return err
		}},
		{name: "task purge after purge", call: func() error {
			_, err := repo.PurgeTask(testContext, scope, task.ID, 4, testMutation("child-errors-task-purge-after-purge", *now))
			return err
		}},
		{name: "decision update after purge", call: func() error {
			_, err := repo.UpdateDecision(testContext, scope, decision.ID, DecisionPatch{Title: &decisionTitle}, 4, testMutation("child-errors-decision-update-after-purge", *now))
			return err
		}},
		{name: "decision trash after purge", call: func() error {
			_, err := repo.TrashDecision(testContext, scope, decision.ID, 4, testMutation("child-errors-decision-trash-after-purge", *now))
			return err
		}},
		{name: "decision restore after purge", call: func() error {
			_, err := repo.RestoreDecision(testContext, scope, decision.ID, 4, testMutation("child-errors-decision-restore-after-purge", *now))
			return err
		}},
		{name: "decision purge after purge", call: func() error {
			_, err := repo.PurgeDecision(testContext, scope, decision.ID, 4, testMutation("child-errors-decision-purge-after-purge", *now))
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			entity := EntityTask
			id := task.ID
			if strings.HasPrefix(test.name, "decision") {
				entity = EntityDecision
				id = decision.ID
			}
			requireNotFound(t, test.call(), entity, id)
		})
	}
}
