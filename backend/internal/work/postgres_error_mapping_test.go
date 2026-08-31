package work

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPostgresRepositoryErrorMappingAndHelpers(t *testing.T) {
	if got := mapWorkDBError(nil); got != nil {
		t.Fatalf("mapWorkDBError(nil) = %v, want nil", got)
	}
	if got := mapWorkInsertError(nil, EntityProject, "project"); got != nil {
		t.Fatalf("mapWorkInsertError(nil) = %v, want nil", got)
	}

	tests := []struct {
		name   string
		code   string
		insert error
		want   error
	}{
		{name: "duplicate insert", code: "23505", insert: ErrAlreadyExists, want: ErrAlreadyExists},
		{name: "missing foreign key insert", code: "23503", insert: ErrNotFound, want: ErrNotFound},
		{name: "constraint insert", code: "23514", insert: &ValidationError{}, want: &ValidationError{}},
		{name: "unknown insert", code: "XX000", insert: errors.New("work persistence failed"), want: errors.New("work persistence failed")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := mapWorkInsertError(&pgconn.PgError{Code: tt.code}, EntityTask, "task-1")
			if tt.code == "23514" {
				var validation *ValidationError
				if !errors.As(mapped, &validation) || validation.Field != string(EntityTask) {
					t.Fatalf("mapped constraint error = %T %v, want task validation error", mapped, mapped)
				}
				return
			}
			if tt.code == "XX000" {
				if mapped == nil || mapped.Error() != tt.want.Error() {
					t.Fatalf("mapped unknown error = %v, want %v", mapped, tt.want)
				}
				return
			}
			if !errors.Is(mapped, tt.want) {
				t.Fatalf("mapped error = %T %v, want %v", mapped, mapped, tt.want)
			}
		})
	}

	readMissing := mapWorkReadError(pgx.ErrNoRows, EntityDecision, "decision-1")
	if !errors.Is(readMissing, ErrNotFound) {
		t.Fatalf("missing read error = %v, want not found", readMissing)
	}
	var notFoundError *NotFoundError
	if !errors.As(readMissing, &notFoundError) || notFoundError.EntityType != EntityDecision || notFoundError.EntityID != "decision-1" {
		t.Fatalf("missing read error = %+v, want decision identity", notFoundError)
	}
	readFailure := mapWorkReadError(errors.New("query failed"), EntityTask, "task-1")
	if readFailure == nil || readFailure.Error() != "work persistence failed" {
		t.Fatalf("read failure = %v, want generic persistence error", readFailure)
	}

	if got := mapWorkDBError(pgx.ErrNoRows); !errors.Is(got, ErrNotFound) {
		t.Fatalf("no-row database error = %v, want not found", got)
	}
	for _, tt := range []struct {
		code string
		want error
	}{
		{code: "23505", want: ErrAlreadyExists},
		{code: "23503", want: ErrNotFound},
		{code: "23514", want: &ValidationError{}},
	} {
		mapped := mapWorkDBError(&pgconn.PgError{Code: tt.code})
		if tt.code == "23514" {
			var validation *ValidationError
			if !errors.As(mapped, &validation) || validation.Field != "persistence" {
				t.Fatalf("mapped database constraint error = %T %v, want persistence validation error", mapped, mapped)
			}
			continue
		}
		if !errors.Is(mapped, tt.want) {
			t.Fatalf("mapped database error = %T %v, want %v", mapped, mapped, tt.want)
		}
	}

	if got := positiveVersion(-1); got != 1 {
		t.Fatalf("positiveVersion(-1) = %d, want 1", got)
	}
	if got := positiveVersion(7); got != 7 {
		t.Fatalf("positiveVersion(7) = %d, want 7", got)
	}

	scope := Scope{WorkspaceID: "workspace", UserID: "user"}
	if err := appendHistory(context.Background(), nil, scope, EntityProject, "project", HistoryCreated, 0, 1, "history-before-error", func() {}, nil, mutationAtForTest()); err == nil {
		t.Fatal("appendHistory accepted an unsupported before snapshot")
	}
	if err := appendHistory(context.Background(), nil, scope, EntityProject, "project", HistoryCreated, 0, 1, "history-after-error", nil, func() {}, mutationAtForTest()); err == nil {
		t.Fatal("appendHistory accepted an unsupported after snapshot")
	}
	if err := rememberMutation(context.Background(), nil, scope, testMutation("remember-error", mutationAtForTest()), OperationProjectCreate, EntityProject, "project", func() {}); err == nil {
		t.Fatal("rememberMutation accepted an unsupported result")
	}
}

func mutationAtForTest() time.Time {
	return time.Date(2026, 8, 28, 5, 0, 0, 0, time.UTC)
}

func TestPostgresRepositoryFailsClosedWhenPoolIsClosed(t *testing.T) {
	repository, _, ctx, scope := newPostgresWorkFixture(t)
	repository.Pool.Close()
	mutationAt := time.Date(2026, 8, 28, 4, 0, 0, 0, time.UTC)
	mutation := func(key string) Mutation { return testMutation(key, mutationAt) }
	project := Project{ID: "closed-project", Name: "Closed pool project", Status: ProjectActive, Origin: OriginCanonical, Version: 1}
	task := Task{ID: "closed-task", Title: "Closed pool task", Status: TaskTodo, Priority: PriorityNormal, Origin: OriginCanonical, Version: 1}
	decision := Decision{ID: "closed-decision", Title: "Closed pool decision", Outcome: "Fail closed", Status: DecisionProposed, Origin: OriginCanonical, Version: 1}

	tests := []struct {
		name string
		call func() error
	}{
		{name: "create project", call: func() error {
			_, err := repository.CreateProject(ctx, scope, project, mutation("closed-create-project"))
			return err
		}},
		{name: "get project", call: func() error {
			_, err := repository.GetProject(ctx, scope, project.ID)
			return err
		}},
		{name: "list projects", call: func() error {
			_, err := repository.ListProjects(ctx, scope, ListOptions{Limit: 1})
			return err
		}},
		{name: "update project", call: func() error {
			name := "updated"
			_, err := repository.UpdateProject(ctx, scope, project.ID, ProjectPatch{Name: &name}, 1, mutation("closed-update-project"))
			return err
		}},
		{name: "trash project", call: func() error {
			_, err := repository.TrashProject(ctx, scope, project.ID, 1, mutation("closed-trash-project"))
			return err
		}},
		{name: "restore project", call: func() error {
			_, err := repository.RestoreProject(ctx, scope, project.ID, 1, mutation("closed-restore-project"))
			return err
		}},
		{name: "purge project", call: func() error {
			_, err := repository.PurgeProject(ctx, scope, project.ID, 1, mutation("closed-purge-project"))
			return err
		}},
		{name: "create task", call: func() error {
			_, err := repository.CreateTask(ctx, scope, task, mutation("closed-create-task"))
			return err
		}},
		{name: "get task", call: func() error {
			_, err := repository.GetTask(ctx, scope, task.ID)
			return err
		}},
		{name: "list tasks", call: func() error {
			_, err := repository.ListTasks(ctx, scope, "", ListOptions{Limit: 1})
			return err
		}},
		{name: "update task", call: func() error {
			title := "updated"
			_, err := repository.UpdateTask(ctx, scope, task.ID, TaskPatch{Title: &title}, 1, mutation("closed-update-task"))
			return err
		}},
		{name: "trash task", call: func() error {
			_, err := repository.TrashTask(ctx, scope, task.ID, 1, mutation("closed-trash-task"))
			return err
		}},
		{name: "restore task", call: func() error {
			_, err := repository.RestoreTask(ctx, scope, task.ID, 1, mutation("closed-restore-task"))
			return err
		}},
		{name: "purge task", call: func() error {
			_, err := repository.PurgeTask(ctx, scope, task.ID, 1, mutation("closed-purge-task"))
			return err
		}},
		{name: "create decision", call: func() error {
			_, err := repository.CreateDecision(ctx, scope, decision, mutation("closed-create-decision"))
			return err
		}},
		{name: "get decision", call: func() error {
			_, err := repository.GetDecision(ctx, scope, decision.ID)
			return err
		}},
		{name: "list decisions", call: func() error {
			_, err := repository.ListDecisions(ctx, scope, "", ListOptions{Limit: 1})
			return err
		}},
		{name: "update decision", call: func() error {
			title := "updated"
			_, err := repository.UpdateDecision(ctx, scope, decision.ID, DecisionPatch{Title: &title}, 1, mutation("closed-update-decision"))
			return err
		}},
		{name: "trash decision", call: func() error {
			_, err := repository.TrashDecision(ctx, scope, decision.ID, 1, mutation("closed-trash-decision"))
			return err
		}},
		{name: "restore decision", call: func() error {
			_, err := repository.RestoreDecision(ctx, scope, decision.ID, 1, mutation("closed-restore-decision"))
			return err
		}},
		{name: "purge decision", call: func() error {
			_, err := repository.PurgeDecision(ctx, scope, decision.ID, 1, mutation("closed-purge-decision"))
			return err
		}},
		{name: "history", call: func() error {
			_, err := repository.History(ctx, scope, EntityProject, project.ID)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("closed pool operation unexpectedly succeeded")
			}
		})
	}

}

func TestPostgresRepositoryRejectsCorruptReplayResults(t *testing.T) {
	repository, _, ctx, scope := newPostgresWorkFixture(t)
	corruptResult := []byte(`"corrupt"`)
	seedReplay := func(t *testing.T, key, operation string, entityType EntityType, entityID string) Mutation {
		t.Helper()
		mutation := testMutation(key, mutationAtForTest())
		if _, err := repository.Pool.Exec(ctx, `
			INSERT INTO work_idempotency
				(workspace_id, user_id, idempotency_key, operation, request_hash, entity_type, entity_id, result)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb)
		`, scope.WorkspaceID, scope.UserID, key, operation, mutation.RequestHash, string(entityType), entityID, corruptResult); err != nil {
			t.Fatalf("seed corrupt replay %s: %v", operation, err)
		}
		return mutation
	}

	project := Project{ID: "corrupt-project", Name: "Corrupt replay", Status: ProjectActive, Origin: OriginCanonical, Version: 1}
	task := Task{ID: "corrupt-task", Title: "Corrupt replay", Status: TaskTodo, Priority: PriorityNormal, Origin: OriginCanonical, Version: 1}
	decision := Decision{ID: "corrupt-decision", Title: "Corrupt replay", Outcome: "Reject", Status: DecisionProposed, Origin: OriginCanonical, Version: 1}
	projectPatch := ProjectPatch{}
	taskPatch := TaskPatch{}
	decisionPatch := DecisionPatch{}
	operations := []struct {
		name string
		call func(Mutation) error
	}{
		{name: OperationProjectCreate, call: func(m Mutation) error {
			_, err := repository.CreateProject(ctx, scope, project, m)
			return err
		}},
		{name: OperationProjectUpdate, call: func(m Mutation) error {
			_, err := repository.UpdateProject(ctx, scope, project.ID, projectPatch, 1, m)
			return err
		}},
		{name: OperationProjectTrash, call: func(m Mutation) error {
			_, err := repository.TrashProject(ctx, scope, project.ID, 1, m)
			return err
		}},
		{name: OperationProjectRestore, call: func(m Mutation) error {
			_, err := repository.RestoreProject(ctx, scope, project.ID, 1, m)
			return err
		}},
		{name: OperationProjectPurge, call: func(m Mutation) error {
			_, err := repository.PurgeProject(ctx, scope, project.ID, 1, m)
			return err
		}},
		{name: OperationTaskCreate, call: func(m Mutation) error {
			_, err := repository.CreateTask(ctx, scope, task, m)
			return err
		}},
		{name: OperationTaskUpdate, call: func(m Mutation) error {
			_, err := repository.UpdateTask(ctx, scope, task.ID, taskPatch, 1, m)
			return err
		}},
		{name: OperationTaskTrash, call: func(m Mutation) error {
			_, err := repository.TrashTask(ctx, scope, task.ID, 1, m)
			return err
		}},
		{name: OperationTaskRestore, call: func(m Mutation) error {
			_, err := repository.RestoreTask(ctx, scope, task.ID, 1, m)
			return err
		}},
		{name: OperationTaskPurge, call: func(m Mutation) error {
			_, err := repository.PurgeTask(ctx, scope, task.ID, 1, m)
			return err
		}},
		{name: OperationDecisionCreate, call: func(m Mutation) error {
			_, err := repository.CreateDecision(ctx, scope, decision, m)
			return err
		}},
		{name: OperationDecisionUpdate, call: func(m Mutation) error {
			_, err := repository.UpdateDecision(ctx, scope, decision.ID, decisionPatch, 1, m)
			return err
		}},
		{name: OperationDecisionTrash, call: func(m Mutation) error {
			_, err := repository.TrashDecision(ctx, scope, decision.ID, 1, m)
			return err
		}},
		{name: OperationDecisionRestore, call: func(m Mutation) error {
			_, err := repository.RestoreDecision(ctx, scope, decision.ID, 1, m)
			return err
		}},
		{name: OperationDecisionPurge, call: func(m Mutation) error {
			_, err := repository.PurgeDecision(ctx, scope, decision.ID, 1, m)
			return err
		}},
	}
	for index, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			key := fmt.Sprintf("corrupt-replay-%02d", index)
			entityType := EntityProject
			entityID := project.ID
			if strings.HasPrefix(operation.name, "task.") {
				entityType = EntityTask
				entityID = task.ID
			} else if strings.HasPrefix(operation.name, "decision.") {
				entityType = EntityDecision
				entityID = decision.ID
			}
			mutation := seedReplay(t, key, operation.name, entityType, entityID)
			if err := operation.call(mutation); err == nil {
				t.Fatalf("%s replay unexpectedly succeeded", operation.name)
			} else if !strings.Contains(err.Error(), "decode idempotent") {
				t.Fatalf("%s replay error = %v, want decode error", operation.name, err)
			}
		})
	}
}

func TestPostgresRepositoryHistoryDefaultsZeroTimestamp(t *testing.T) {
	repository, _, ctx, scope := newPostgresWorkFixture(t)
	tx, err := repository.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin history timestamp transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	if err := appendHistory(ctx, tx, scope, EntityProject, "zero-time-history", HistoryCreated, 0, 1, "zero-time-history", nil, nil, time.Time{}); err != nil {
		t.Fatalf("append history with zero timestamp: %v", err)
	}
}
