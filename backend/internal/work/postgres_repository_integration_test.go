package work

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestPostgresRepositoryCreateAndReplayUsesSafeLockKey(t *testing.T) {
	pool, ctx := openWorkIntegrityDatabase(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "work-repository-user-" + suffix
	workspaceID := "work-repository-workspace-" + suffix

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, $1)`, userID); err != nil {
		t.Fatalf("seed repository user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)`, workspaceID, userID, "Repository test", "repository-test-"+suffix); err != nil {
		t.Fatalf("seed repository workspace: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM work_history WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM workspaces WHERE id=$1 AND owner_user_id=$2`, workspaceID, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
	})

	repository, err := NewPostgresRepository(pool)
	if err != nil {
		t.Fatalf("create postgres repository: %v", err)
	}
	service, err := NewService(repository, WithClock(func() time.Time {
		return time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	}))
	if err != nil {
		t.Fatalf("create work service: %v", err)
	}
	scope := Scope{WorkspaceID: workspaceID, UserID: userID}

	project, err := service.CreateProject(ctx, scope, CreateProjectInput{
		Name:        "Repository smoke project",
		Description: "Exercises PostgreSQL enum and idempotency persistence",
		Status:      ProjectActive,
	}, "repository-project-"+suffix)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	replayed, err := service.CreateProject(ctx, scope, CreateProjectInput{
		Name:        "Repository smoke project",
		Description: "Exercises PostgreSQL enum and idempotency persistence",
		Status:      ProjectActive,
	}, "repository-project-"+suffix)
	if err != nil {
		t.Fatalf("replay project: %v", err)
	}
	if replayed.ID != project.ID || replayed.Version != project.Version {
		t.Fatalf("replay returned %+v, want the original project %+v", replayed, project)
	}

	task, err := service.CreateTask(ctx, scope, CreateTaskInput{
		ProjectID:   project.ID,
		Title:       "Repository smoke task",
		Description: "Verify the canonical response",
		Status:      TaskTodo,
		Priority:    PriorityHigh,
	}, "repository-task-"+suffix)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.ProjectID != project.ID || task.Priority != PriorityHigh {
		t.Fatalf("created task lost canonical fields: %+v", task)
	}

	decision, err := service.CreateDecision(ctx, scope, CreateDecisionInput{
		ProjectID: project.ID,
		Title:     "Repository smoke decision",
		Outcome:   "Keep evidence attached",
		Status:    DecisionProposed,
	}, "repository-decision-"+suffix)
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}
	if decision.ProjectID != project.ID || decision.Status != DecisionProposed {
		t.Fatalf("created decision lost canonical fields: %+v", decision)
	}

	history, err := service.History(ctx, scope, EntityProject, project.ID)
	if err != nil {
		t.Fatalf("read project history: %v", err)
	}
	if len(history) != 1 || history[0].Action != HistoryCreated {
		t.Fatalf("project history = %+v, want one created event", history)
	}
}

func TestPostgresRepositoryMutationsAreVersionedAtomicAndPurgeable(t *testing.T) {
	pool, ctx := openWorkIntegrityDatabase(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "work-mutation-user-" + suffix
	workspaceID := "work-mutation-workspace-" + suffix
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, $1)`, userID); err != nil {
		t.Fatalf("seed mutation user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)`, workspaceID, userID, "Mutation test", "mutation-test-"+suffix); err != nil {
		t.Fatalf("seed mutation workspace: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM work_history WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM workspaces WHERE id=$1 AND owner_user_id=$2`, workspaceID, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
	})

	repository, err := NewPostgresRepository(pool)
	if err != nil {
		t.Fatalf("create postgres mutation repository: %v", err)
	}
	base := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	service, err := NewService(repository, WithClock(func() time.Time { return base }))
	if err != nil {
		t.Fatalf("create mutation service: %v", err)
	}
	scope := Scope{WorkspaceID: workspaceID, UserID: userID}

	project, err := service.CreateProject(ctx, scope, CreateProjectInput{Name: "Versioned project"}, "mutation-project-create-"+suffix)
	if err != nil {
		t.Fatalf("create mutation project: %v", err)
	}
	name := "Updated project"
	updated, err := service.UpdateProject(ctx, scope, project.ID, ProjectPatch{Name: &name}, project.Version, "mutation-project-update-"+suffix)
	if err != nil {
		t.Fatalf("update mutation project: %v", err)
	}
	if updated.Version != 2 || updated.Name != name {
		t.Fatalf("updated project = %+v, want version 2 and updated name", updated)
	}
	replayed, err := service.UpdateProject(ctx, scope, project.ID, ProjectPatch{Name: &name}, project.Version, "mutation-project-update-"+suffix)
	if err != nil {
		t.Fatalf("replay mutation project update: %v", err)
	}
	if replayed.Version != updated.Version || replayed.UpdatedAt != updated.UpdatedAt {
		t.Fatalf("replayed project = %+v, want original update %+v", replayed, updated)
	}
	if _, err := service.UpdateProject(ctx, scope, project.ID, ProjectPatch{Name: &name}, project.Version, "mutation-project-conflict-"+suffix); err == nil || !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale update error = %v, want version conflict", err)
	}

	trashed, err := service.TrashProject(ctx, scope, project.ID, updated.Version, "mutation-project-trash-"+suffix)
	if err != nil {
		t.Fatalf("trash mutation project: %v", err)
	}
	restored, err := service.RestoreProject(ctx, scope, project.ID, trashed.Version, "mutation-project-restore-"+suffix)
	if err != nil {
		t.Fatalf("restore mutation project: %v", err)
	}
	if restored.DeletedAt != nil || restored.Version != 4 {
		t.Fatalf("restored project = %+v, want active version 4", restored)
	}

	task, err := service.CreateTask(ctx, scope, CreateTaskInput{ProjectID: project.ID, Title: "Dependent task"}, "mutation-task-create-"+suffix)
	if err != nil {
		t.Fatalf("create dependent task: %v", err)
	}
	decision, err := service.CreateDecision(ctx, scope, CreateDecisionInput{ProjectID: project.ID, Title: "Dependent decision", Outcome: "Keep it"}, "mutation-decision-create-"+suffix)
	if err != nil {
		t.Fatalf("create dependent decision: %v", err)
	}
	if _, err := service.TrashTask(ctx, scope, task.ID, task.Version, "mutation-task-trash-"+suffix); err != nil {
		t.Fatalf("trash dependent task: %v", err)
	}
	if _, err := service.TrashDecision(ctx, scope, decision.ID, decision.Version, "mutation-decision-trash-"+suffix); err != nil {
		t.Fatalf("trash dependent decision: %v", err)
	}
	trashed, err = service.TrashProject(ctx, scope, project.ID, restored.Version, "mutation-project-trash-again-"+suffix)
	if err != nil {
		t.Fatalf("trash project for purge: %v", err)
	}

	future := base.Add(RetentionPeriod + time.Hour)
	futureService, err := NewService(repository, WithClock(func() time.Time { return future }))
	if err != nil {
		t.Fatalf("create future mutation service: %v", err)
	}
	if _, err := futureService.PurgeProject(ctx, scope, project.ID, trashed.Version, "mutation-project-purge-blocked-"+suffix); err == nil || !errors.Is(err, ErrDependenciesExist) {
		t.Fatalf("purge project with dependents error = %v, want dependency error", err)
	}
	if _, err := futureService.PurgeTask(ctx, scope, task.ID, 2, "mutation-task-purge-"+suffix); err != nil {
		t.Fatalf("purge dependent task: %v", err)
	}
	if _, err := futureService.PurgeDecision(ctx, scope, decision.ID, 2, "mutation-decision-purge-"+suffix); err != nil {
		t.Fatalf("purge dependent decision: %v", err)
	}
	purged, err := futureService.PurgeProject(ctx, scope, project.ID, trashed.Version, "mutation-project-purge-"+suffix)
	if err != nil {
		t.Fatalf("purge project: %v", err)
	}
	if purged.EntityType != EntityProject || purged.EntityID != project.ID {
		t.Fatalf("purge result = %+v, want project %s", purged, project.ID)
	}
	if _, err := futureService.GetProject(ctx, scope, project.ID); err == nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("get purged project error = %v, want not found", err)
	}
	history, err := futureService.History(ctx, scope, EntityProject, project.ID)
	if err != nil {
		t.Fatalf("read purged project history: %v", err)
	}
	if len(history) != 6 || history[len(history)-1].Action != HistoryPurged {
		t.Fatalf("purged project history length/action = %d/%s, want 6/purged", len(history), history[len(history)-1].Action)
	}
}

func TestPostgresRepositoryListsAndUpdatesAllEntities(t *testing.T) {
	pool, ctx := openWorkIntegrityDatabase(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "work-list-update-user-" + suffix
	workspaceID := "work-list-update-workspace-" + suffix
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, $1)`, userID); err != nil {
		t.Fatalf("seed list/update user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)`, workspaceID, userID, "List and update test", "list-update-test-"+suffix); err != nil {
		t.Fatalf("seed list/update workspace: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM work_history WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM workspaces WHERE id=$1 AND owner_user_id=$2`, workspaceID, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
	})

	repository, err := NewPostgresRepository(pool)
	if err != nil {
		t.Fatalf("create list/update repository: %v", err)
	}
	base := time.Date(2026, 8, 28, 1, 0, 0, 0, time.UTC)
	service, err := NewService(repository, WithClock(func() time.Time { return base }))
	if err != nil {
		t.Fatalf("create list/update service: %v", err)
	}
	scope := Scope{WorkspaceID: workspaceID, UserID: userID}

	project, err := service.CreateProject(ctx, scope, CreateProjectInput{
		Name:        "List/update project",
		Description: "Before update",
		Status:      ProjectActive,
	}, "list-update-project-create-"+suffix)
	if err != nil {
		t.Fatalf("create list/update project: %v", err)
	}
	task, err := service.CreateTask(ctx, scope, CreateTaskInput{
		ProjectID:   project.ID,
		Title:       "List/update task",
		Description: "Before update",
		Status:      TaskTodo,
		Priority:    PriorityNormal,
	}, "list-update-task-create-"+suffix)
	if err != nil {
		t.Fatalf("create list/update task: %v", err)
	}
	decision, err := service.CreateDecision(ctx, scope, CreateDecisionInput{
		ProjectID: project.ID,
		Title:     "List/update decision",
		Context:   "Before update",
		Outcome:   "Keep the current path",
		Rationale: "It is reversible",
		Status:    DecisionProposed,
	}, "list-update-decision-create-"+suffix)
	if err != nil {
		t.Fatalf("create list/update decision: %v", err)
	}

	gotProject, err := service.GetProject(ctx, scope, project.ID)
	if err != nil || gotProject.ID != project.ID {
		t.Fatalf("get project = %+v, err=%v", gotProject, err)
	}
	projects, err := service.ListProjects(ctx, scope, ListOptions{Limit: 10})
	if err != nil || len(projects) != 1 || projects[0].ID != project.ID {
		t.Fatalf("list projects = %+v, err=%v", projects, err)
	}
	gotTask, err := service.GetTask(ctx, scope, task.ID)
	if err != nil || gotTask.ID != task.ID {
		t.Fatalf("get task = %+v, err=%v", gotTask, err)
	}
	tasksByProject, err := service.ListTasks(ctx, scope, project.ID, ListOptions{Limit: 10})
	if err != nil || len(tasksByProject) != 1 || tasksByProject[0].ID != task.ID {
		t.Fatalf("list tasks by project = %+v, err=%v", tasksByProject, err)
	}
	tasks, err := service.ListTasks(ctx, scope, "", ListOptions{Limit: 10})
	if err != nil || len(tasks) != 1 || tasks[0].ID != task.ID {
		t.Fatalf("list all tasks = %+v, err=%v", tasks, err)
	}
	gotDecision, err := service.GetDecision(ctx, scope, decision.ID)
	if err != nil || gotDecision.ID != decision.ID {
		t.Fatalf("get decision = %+v, err=%v", gotDecision, err)
	}
	decisionsByProject, err := service.ListDecisions(ctx, scope, project.ID, ListOptions{Limit: 10})
	if err != nil || len(decisionsByProject) != 1 || decisionsByProject[0].ID != decision.ID {
		t.Fatalf("list decisions by project = %+v, err=%v", decisionsByProject, err)
	}
	decisions, err := service.ListDecisions(ctx, scope, "", ListOptions{Limit: 10})
	if err != nil || len(decisions) != 1 || decisions[0].ID != decision.ID {
		t.Fatalf("list all decisions = %+v, err=%v", decisions, err)
	}

	projectName := "Updated project"
	projectDescription := "After update"
	projectStatus := ProjectOnHold
	updatedProject, err := service.UpdateProject(ctx, scope, project.ID, ProjectPatch{
		Name:        &projectName,
		Description: &projectDescription,
		Status:      &projectStatus,
	}, project.Version, "list-update-project-update-"+suffix)
	if err != nil || updatedProject.Version != 2 || updatedProject.Status != projectStatus {
		t.Fatalf("updated project = %+v, err=%v", updatedProject, err)
	}

	dueAt := base.Add(48 * time.Hour)
	dueAtPatch := &dueAt
	updatedTaskTitle := "Updated task"
	updatedTaskDescription := "After update"
	updatedTaskStatus := TaskInProgress
	updatedTaskPriority := PriorityUrgent
	updatedTask, err := service.UpdateTask(ctx, scope, task.ID, TaskPatch{
		ProjectID:   &project.ID,
		Title:       &updatedTaskTitle,
		Description: &updatedTaskDescription,
		Status:      &updatedTaskStatus,
		Priority:    &updatedTaskPriority,
		DueAt:       &dueAtPatch,
	}, task.Version, "list-update-task-update-"+suffix)
	if err != nil || updatedTask.Version != 2 || updatedTask.DueAt == nil || updatedTask.Priority != updatedTaskPriority {
		t.Fatalf("updated task = %+v, err=%v", updatedTask, err)
	}

	decisionTitle := "Updated decision"
	decisionContext := "After update context"
	decisionOutcome := "Use the tested path"
	decisionRationale := "It has evidence"
	decisionStatus := DecisionAccepted
	updatedDecision, err := service.UpdateDecision(ctx, scope, decision.ID, DecisionPatch{
		ProjectID: &project.ID,
		Title:     &decisionTitle,
		Context:   &decisionContext,
		Outcome:   &decisionOutcome,
		Rationale: &decisionRationale,
		Status:    &decisionStatus,
	}, decision.Version, "list-update-decision-update-"+suffix)
	if err != nil || updatedDecision.Version != 2 || updatedDecision.Status != decisionStatus {
		t.Fatalf("updated decision = %+v, err=%v", updatedDecision, err)
	}

	trashedTask, err := service.TrashTask(ctx, scope, task.ID, updatedTask.Version, "list-update-task-trash-"+suffix)
	if err != nil || trashedTask.DeletedAt == nil || trashedTask.Version != 3 {
		t.Fatalf("trashed task = %+v, err=%v", trashedTask, err)
	}
	activeTasks, err := service.ListTasks(ctx, scope, "", ListOptions{Limit: 10})
	if err != nil || len(activeTasks) != 0 {
		t.Fatalf("active tasks after trash = %+v, err=%v", activeTasks, err)
	}
	allTasks, err := service.ListTasks(ctx, scope, "", ListOptions{IncludeTrashed: true, Limit: 10})
	if err != nil || len(allTasks) != 1 || allTasks[0].DeletedAt == nil {
		t.Fatalf("all tasks after trash = %+v, err=%v", allTasks, err)
	}
	restoredTask, err := service.RestoreTask(ctx, scope, task.ID, trashedTask.Version, "list-update-task-restore-"+suffix)
	if err != nil || restoredTask.DeletedAt != nil || restoredTask.Version != 4 {
		t.Fatalf("restored task = %+v, err=%v", restoredTask, err)
	}

	trashedDecision, err := service.TrashDecision(ctx, scope, decision.ID, updatedDecision.Version, "list-update-decision-trash-"+suffix)
	if err != nil || trashedDecision.DeletedAt == nil || trashedDecision.Version != 3 {
		t.Fatalf("trashed decision = %+v, err=%v", trashedDecision, err)
	}
	activeDecisions, err := service.ListDecisions(ctx, scope, "", ListOptions{Limit: 10})
	if err != nil || len(activeDecisions) != 0 {
		t.Fatalf("active decisions after trash = %+v, err=%v", activeDecisions, err)
	}
	allDecisions, err := service.ListDecisions(ctx, scope, "", ListOptions{IncludeTrashed: true, Limit: 10})
	if err != nil || len(allDecisions) != 1 || allDecisions[0].DeletedAt == nil {
		t.Fatalf("all decisions after trash = %+v, err=%v", allDecisions, err)
	}
	restoredDecision, err := service.RestoreDecision(ctx, scope, decision.ID, trashedDecision.Version, "list-update-decision-restore-"+suffix)
	if err != nil || restoredDecision.DeletedAt != nil || restoredDecision.Version != 4 {
		t.Fatalf("restored decision = %+v, err=%v", restoredDecision, err)
	}

	if _, err := service.GetProject(ctx, Scope{WorkspaceID: workspaceID, UserID: "other-user-" + suffix}, project.ID); err == nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user project lookup error = %v, want not found", err)
	}
}
