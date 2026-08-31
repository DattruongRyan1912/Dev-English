package work

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func newPostgresWorkFixture(t *testing.T) (*PostgresRepository, *Service, context.Context, Scope) {
	t.Helper()
	pool, ctx := openWorkIntegrityDatabase(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "work-boundary-user-" + suffix
	workspaceID := "work-boundary-workspace-" + suffix
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, $1)`, userID); err != nil {
		t.Fatalf("seed boundary user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, owner_user_id, name, slug) VALUES ($1, $2, $3, $4)`, workspaceID, userID, "Boundary test", "boundary-test-"+suffix); err != nil {
		t.Fatalf("seed boundary workspace: %v", err)
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
		t.Fatalf("create boundary repository: %v", err)
	}
	base := time.Date(2026, 8, 28, 2, 0, 0, 0, time.UTC)
	service, err := NewService(repository, WithClock(func() time.Time { return base }))
	if err != nil {
		t.Fatalf("create boundary service: %v", err)
	}
	return repository, service, ctx, Scope{WorkspaceID: workspaceID, UserID: userID}
}

func TestPostgresRepositoryBoundaryStatesAndOptionalLinks(t *testing.T) {
	repository, service, ctx, scope := newPostgresWorkFixture(t)
	base := time.Date(2026, 8, 28, 2, 0, 0, 0, time.UTC)

	if _, err := NewPostgresRepository(nil); err == nil {
		t.Fatal("NewPostgresRepository(nil) accepted a nil pool")
	}

	unlinkedTask, err := service.CreateTask(ctx, scope, CreateTaskInput{
		Title:       "Unlinked task",
		Description: "Can exist without a project",
		Status:      TaskBacklog,
		Priority:    PriorityLow,
	}, "boundary-task-unlinked")
	if err != nil {
		t.Fatalf("create unlinked task: %v", err)
	}
	unlinkedDecision, err := service.CreateDecision(ctx, scope, CreateDecisionInput{
		Title:   "Unlinked decision",
		Outcome: "Keep it independent",
		Status:  DecisionProposed,
	}, "boundary-decision-unlinked")
	if err != nil {
		t.Fatalf("create unlinked decision: %v", err)
	}
	var projectID *string
	if err := repository.Pool.QueryRow(ctx, `SELECT project_id FROM tasks WHERE workspace_id=$1 AND owner_user_id=$2 AND id=$3`, scope.WorkspaceID, scope.UserID, unlinkedTask.ID).Scan(&projectID); err != nil {
		t.Fatalf("read unlinked task project: %v", err)
	}
	if projectID != nil {
		t.Fatalf("unlinked task project_id = %q, want NULL", *projectID)
	}

	if _, err := service.GetTask(ctx, scope, "missing-task"); err == nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing task error = %v, want not found", err)
	}
	if _, err := service.GetDecision(ctx, scope, "missing-decision"); err == nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing decision error = %v, want not found", err)
	}
	if _, err := service.GetProject(ctx, scope, "missing-project"); err == nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing project error = %v, want not found", err)
	}

	projectA, err := service.CreateProject(ctx, scope, CreateProjectInput{Name: "Boundary project A"}, "boundary-project-a")
	if err != nil {
		t.Fatalf("create project A: %v", err)
	}
	projectB, err := service.CreateProject(ctx, scope, CreateProjectInput{Name: "Boundary project B"}, "boundary-project-b")
	if err != nil {
		t.Fatalf("create project B: %v", err)
	}
	linkedTask, err := service.CreateTask(ctx, scope, CreateTaskInput{ProjectID: projectA.ID, Title: "Linked task"}, "boundary-task-linked")
	if err != nil {
		t.Fatalf("create linked task: %v", err)
	}
	linkedDecision, err := service.CreateDecision(ctx, scope, CreateDecisionInput{ProjectID: projectA.ID, Title: "Linked decision", Outcome: "Use evidence"}, "boundary-decision-linked")
	if err != nil {
		t.Fatalf("create linked decision: %v", err)
	}

	newProjectID := projectB.ID
	newTitle := "Moved task"
	newDescription := "Updated description"
	newStatus := TaskInProgress
	newPriority := PriorityUrgent
	dueAt := base.Add(24 * time.Hour)
	dueAtPatch := &dueAt
	updatedTask, err := service.UpdateTask(ctx, scope, linkedTask.ID, TaskPatch{
		ProjectID:   &newProjectID,
		Title:       &newTitle,
		Description: &newDescription,
		Status:      &newStatus,
		Priority:    &newPriority,
		DueAt:       &dueAtPatch,
	}, linkedTask.Version, "boundary-task-update")
	if err != nil {
		t.Fatalf("move/update task: %v", err)
	}
	if updatedTask.ProjectID != projectB.ID || updatedTask.DueAt == nil || updatedTask.Version != 2 {
		t.Fatalf("updated task = %+v, want project B, due date and version 2", updatedTask)
	}

	emptyProjectID := ""
	var clearDueAt *time.Time
	clearedTask, err := service.UpdateTask(ctx, scope, linkedTask.ID, TaskPatch{ProjectID: &emptyProjectID, DueAt: &clearDueAt}, updatedTask.Version, "boundary-task-clear-link")
	if err != nil {
		t.Fatalf("clear task link and due date: %v", err)
	}
	if clearedTask.ProjectID != "" || clearedTask.DueAt != nil || clearedTask.Version != 3 {
		t.Fatalf("cleared task = %+v, want no project/due date and version 3", clearedTask)
	}

	decisionTitle := "Moved decision"
	decisionContext := "Updated context"
	decisionOutcome := "Use the tested path"
	decisionRationale := "It has an explicit source"
	decisionStatus := DecisionAccepted
	updatedDecision, err := service.UpdateDecision(ctx, scope, linkedDecision.ID, DecisionPatch{
		ProjectID: &newProjectID,
		Title:     &decisionTitle,
		Context:   &decisionContext,
		Outcome:   &decisionOutcome,
		Rationale: &decisionRationale,
		Status:    &decisionStatus,
	}, linkedDecision.Version, "boundary-decision-update")
	if err != nil {
		t.Fatalf("move/update decision: %v", err)
	}
	if updatedDecision.ProjectID != projectB.ID || updatedDecision.Status != DecisionAccepted || updatedDecision.Version != 2 {
		t.Fatalf("updated decision = %+v, want project B, accepted and version 2", updatedDecision)
	}
	clearedDecision, err := service.UpdateDecision(ctx, scope, linkedDecision.ID, DecisionPatch{ProjectID: &emptyProjectID}, updatedDecision.Version, "boundary-decision-clear-link")
	if err != nil {
		t.Fatalf("clear decision link: %v", err)
	}
	if clearedDecision.ProjectID != "" || clearedDecision.Version != 3 {
		t.Fatalf("cleared decision = %+v, want no project and version 3", clearedDecision)
	}

	if _, err := service.CreateTask(ctx, scope, CreateTaskInput{ProjectID: "missing-project", Title: "Invalid link"}, "boundary-task-invalid-link"); err == nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("invalid task project error = %v, want not found", err)
	}
	if _, err := service.CreateDecision(ctx, scope, CreateDecisionInput{ProjectID: "missing-project", Title: "Invalid link", Outcome: "Reject"}, "boundary-decision-invalid-link"); err == nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("invalid decision project error = %v, want not found", err)
	}

	if _, err := service.TrashProject(ctx, scope, projectB.ID, projectB.Version, "boundary-project-b-trash"); err != nil {
		t.Fatalf("trash project B: %v", err)
	}
	if _, err := service.TrashProject(ctx, scope, projectB.ID, projectB.Version+1, "boundary-project-b-trash-again"); err == nil || !errors.Is(err, ErrAlreadyTrashed) {
		t.Fatalf("trash project B twice error = %v, want already trashed", err)
	}
	if _, err := service.UpdateTask(ctx, scope, unlinkedTask.ID, TaskPatch{ProjectID: &newProjectID}, unlinkedTask.Version, "boundary-task-trashed-project"); err == nil || !errors.Is(err, ErrRecordTrashed) {
		t.Fatalf("link task to trashed project error = %v, want record trashed", err)
	}
	if _, err := service.UpdateDecision(ctx, scope, unlinkedDecision.ID, DecisionPatch{ProjectID: &newProjectID}, unlinkedDecision.Version, "boundary-decision-trashed-project"); err == nil || !errors.Is(err, ErrRecordTrashed) {
		t.Fatalf("link decision to trashed project error = %v, want record trashed", err)
	}

	trashedTask, err := service.TrashTask(ctx, scope, unlinkedTask.ID, unlinkedTask.Version, "boundary-task-trash")
	if err != nil {
		t.Fatalf("trash unlinked task: %v", err)
	}
	if _, err := service.TrashTask(ctx, scope, unlinkedTask.ID, trashedTask.Version, "boundary-task-trash-again"); err == nil || !errors.Is(err, ErrAlreadyTrashed) {
		t.Fatalf("trash task twice error = %v, want already trashed", err)
	}
	if _, err := service.UpdateTask(ctx, scope, unlinkedTask.ID, TaskPatch{Title: &newTitle}, trashedTask.Version, "boundary-task-update-trashed"); err == nil || !errors.Is(err, ErrRecordTrashed) {
		t.Fatalf("update trashed task error = %v, want record trashed", err)
	}
	if _, err := service.RestoreTask(ctx, scope, unlinkedTask.ID, trashedTask.Version, "boundary-task-restore"); err != nil {
		t.Fatalf("restore task: %v", err)
	}
	if _, err := service.RestoreTask(ctx, scope, unlinkedTask.ID, trashedTask.Version+1, "boundary-task-restore-again"); err == nil || !errors.Is(err, ErrNotTrashed) {
		t.Fatalf("restore task twice error = %v, want not trashed", err)
	}

	trashedDecision, err := service.TrashDecision(ctx, scope, unlinkedDecision.ID, unlinkedDecision.Version, "boundary-decision-trash")
	if err != nil {
		t.Fatalf("trash unlinked decision: %v", err)
	}
	if _, err := service.RestoreDecision(ctx, scope, unlinkedDecision.ID, trashedDecision.Version, "boundary-decision-restore"); err != nil {
		t.Fatalf("restore decision: %v", err)
	}
	if _, err := service.RestoreDecision(ctx, scope, unlinkedDecision.ID, trashedDecision.Version+1, "boundary-decision-restore-again"); err == nil || !errors.Is(err, ErrNotTrashed) {
		t.Fatalf("restore decision twice error = %v, want not trashed", err)
	}

	projects, err := service.ListProjects(ctx, scope, ListOptions{IncludeTrashed: true, Limit: 10})
	if err != nil || len(projects) != 2 {
		t.Fatalf("list projects including trashed = %+v, err=%v", projects, err)
	}
	tasks, err := service.ListTasks(ctx, scope, "", ListOptions{IncludeTrashed: true, Limit: 10})
	if err != nil || len(tasks) != 2 {
		t.Fatalf("list tasks including trashed = %+v, err=%v", tasks, err)
	}
	decisions, err := service.ListDecisions(ctx, scope, "", ListOptions{IncludeTrashed: true, Limit: 10})
	if err != nil || len(decisions) != 2 {
		t.Fatalf("list decisions including trashed = %+v, err=%v", decisions, err)
	}

	if _, err := service.TrashProject(ctx, scope, projectB.ID, projectB.Version+1, "boundary-project-b-trash-stale"); err == nil || !errors.Is(err, ErrAlreadyTrashed) {
		t.Fatalf("stale trash project B error = %v, want already trashed state", err)
	}

	if _, err := service.GetProject(ctx, Scope{WorkspaceID: scope.WorkspaceID, UserID: "another-user"}, projectA.ID); err == nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user project error = %v, want not found", err)
	}
}

func TestPostgresRepositoryPurgeAndReplayBoundaries(t *testing.T) {
	repository, service, ctx, scope := newPostgresWorkFixture(t)
	base := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)

	project, err := service.CreateProject(ctx, scope, CreateProjectInput{Name: "Purge project"}, "purge-project-create")
	if err != nil {
		t.Fatalf("create purge project: %v", err)
	}
	if _, err := service.CreateProject(ctx, scope, CreateProjectInput{Name: "Changed request"}, "purge-project-create"); err == nil || !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict = %v, want conflict", err)
	}

	trashedProject, err := service.TrashProject(ctx, scope, project.ID, project.Version, "purge-project-trash")
	if err != nil {
		t.Fatalf("trash purge project: %v", err)
	}
	if _, err := service.TrashProject(ctx, scope, project.ID, trashedProject.Version, "purge-project-trash-again"); err == nil || !errors.Is(err, ErrAlreadyTrashed) {
		t.Fatalf("trash project twice = %v, want already trashed", err)
	}
	restoredProject, err := service.RestoreProject(ctx, scope, project.ID, trashedProject.Version, "purge-project-restore")
	if err != nil {
		t.Fatalf("restore purge project: %v", err)
	}
	if _, err := service.RestoreProject(ctx, scope, project.ID, restoredProject.Version, "purge-project-restore-again"); err == nil || !errors.Is(err, ErrNotTrashed) {
		t.Fatalf("restore active project = %v, want not trashed", err)
	}
	trashedProject, err = service.TrashProject(ctx, scope, project.ID, restoredProject.Version, "purge-project-trash-final")
	if err != nil {
		t.Fatalf("trash purge project for final purge: %v", err)
	}
	if _, err := service.PurgeProject(ctx, scope, project.ID, trashedProject.Version, "purge-project-too-early"); err == nil || !errors.Is(err, ErrPurgeNotReady) {
		t.Fatalf("early project purge = %v, want not ready", err)
	}

	futureService, err := NewService(repository, WithClock(func() time.Time { return base.Add(RetentionPeriod + time.Hour) }))
	if err != nil {
		t.Fatalf("create future purge service: %v", err)
	}
	purgedProject, err := futureService.PurgeProject(ctx, scope, project.ID, trashedProject.Version, "purge-project-final")
	if err != nil {
		t.Fatalf("purge project: %v", err)
	}
	replayedProject, err := futureService.PurgeProject(ctx, scope, project.ID, trashedProject.Version, "purge-project-final")
	if err != nil || replayedProject != purgedProject {
		t.Fatalf("replay project purge = %+v, %v; want %+v", replayedProject, err, purgedProject)
	}

	task, err := service.CreateTask(ctx, scope, CreateTaskInput{Title: "Purge task"}, "purge-task-create")
	if err != nil {
		t.Fatalf("create purge task: %v", err)
	}
	if _, err := service.PurgeTask(ctx, scope, task.ID, task.Version, "purge-task-active"); err == nil || !errors.Is(err, ErrNotTrashed) {
		t.Fatalf("purge active task = %v, want not trashed", err)
	}
	trashedTask, err := service.TrashTask(ctx, scope, task.ID, task.Version, "purge-task-trash")
	if err != nil {
		t.Fatalf("trash purge task: %v", err)
	}
	if _, err := service.PurgeTask(ctx, scope, task.ID, trashedTask.Version, "purge-task-too-early"); err == nil || !errors.Is(err, ErrPurgeNotReady) {
		t.Fatalf("early task purge = %v, want not ready", err)
	}
	purgedTask, err := futureService.PurgeTask(ctx, scope, task.ID, trashedTask.Version, "purge-task-final")
	if err != nil {
		t.Fatalf("purge task: %v", err)
	}
	if replay, err := futureService.PurgeTask(ctx, scope, task.ID, trashedTask.Version, "purge-task-final"); err != nil || replay != purgedTask {
		t.Fatalf("replay task purge = %+v, %v; want %+v", replay, err, purgedTask)
	}

	decision, err := service.CreateDecision(ctx, scope, CreateDecisionInput{Title: "Purge decision", Outcome: "Keep evidence"}, "purge-decision-create")
	if err != nil {
		t.Fatalf("create purge decision: %v", err)
	}
	if _, err := service.PurgeDecision(ctx, scope, decision.ID, decision.Version, "purge-decision-active"); err == nil || !errors.Is(err, ErrNotTrashed) {
		t.Fatalf("purge active decision = %v, want not trashed", err)
	}
	trashedDecision, err := service.TrashDecision(ctx, scope, decision.ID, decision.Version, "purge-decision-trash")
	if err != nil {
		t.Fatalf("trash purge decision: %v", err)
	}
	if _, err := service.PurgeDecision(ctx, scope, decision.ID, trashedDecision.Version, "purge-decision-too-early"); err == nil || !errors.Is(err, ErrPurgeNotReady) {
		t.Fatalf("early decision purge = %v, want not ready", err)
	}
	purgedDecision, err := futureService.PurgeDecision(ctx, scope, decision.ID, trashedDecision.Version, "purge-decision-final")
	if err != nil {
		t.Fatalf("purge decision: %v", err)
	}
	if replay, err := futureService.PurgeDecision(ctx, scope, decision.ID, trashedDecision.Version, "purge-decision-final"); err != nil || replay != purgedDecision {
		t.Fatalf("replay decision purge = %+v, %v; want %+v", replay, err, purgedDecision)
	}

	directProject := Project{
		ID:          "direct-version-project",
		Name:        "Direct repository project",
		Description: "Exercises repository version normalization",
		Status:      ProjectActive,
		Origin:      OriginCanonical,
		Version:     0,
		CreatedAt:   base,
		UpdatedAt:   base,
	}
	direct, err := repository.CreateProject(ctx, scope, directProject, testMutation("purge-direct-project", base))
	if err != nil {
		t.Fatalf("direct project with zero version: %v", err)
	}
	if direct.Version != 1 {
		t.Fatalf("direct project version = %d, want normalized version 1", direct.Version)
	}
}
