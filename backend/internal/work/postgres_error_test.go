package work

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPostgresErrorMappingPreservesDomainBoundaries(t *testing.T) {
	if mapWorkInsertError(nil, EntityProject, "project-1") != nil {
		t.Fatal("nil insert error was not preserved")
	}
	if mapWorkDBError(nil) != nil {
		t.Fatal("nil database error was not preserved")
	}

	insertCases := []struct {
		name           string
		code           string
		want           error
		wantNotFound   bool
		wantValidation bool
	}{
		{name: "duplicate", code: "23505", want: ErrAlreadyExists},
		{name: "foreign key", code: "23503", wantNotFound: true},
		{name: "check constraint", code: "23514", wantValidation: true},
	}
	for _, testCase := range insertCases {
		t.Run("insert/"+testCase.name, func(t *testing.T) {
			err := mapWorkInsertError(&pgconn.PgError{Code: testCase.code}, EntityTask, "task-1")
			if testCase.want != nil && !errors.Is(err, testCase.want) {
				t.Fatalf("mapped error = %v, want %v", err, testCase.want)
			}
			if testCase.wantNotFound {
				var notFound *NotFoundError
				if !errors.As(err, &notFound) || notFound.EntityType != EntityTask || notFound.EntityID != "task-1" {
					t.Fatalf("foreign-key error = %T %v, want task not-found", err, err)
				}
			}
			if testCase.wantValidation {
				var validation *ValidationError
				if !errors.As(err, &validation) || validation.Field != string(EntityTask) {
					t.Fatalf("constraint error = %T %v, want task validation", err, err)
				}
			}
		})
	}

	unknown := mapWorkInsertError(errors.New("driver broke"), EntityProject, "project-1")
	if unknown == nil || unknown.Error() != "work persistence failed" {
		t.Fatalf("unknown insert error = %v, want redacted persistence error", unknown)
	}
	if err := mapWorkReadError(pgx.ErrNoRows, EntityDecision, "decision-1"); err == nil {
		t.Fatal("missing read row was not mapped")
	}
	if err := mapWorkDBError(pgx.ErrNoRows); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing database row = %v, want ErrNotFound", err)
	}
	if err := mapWorkDBError(&pgconn.PgError{Code: "23505"}); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate database error = %v, want ErrAlreadyExists", err)
	}
	if err := mapWorkDBError(&pgconn.PgError{Code: "23503"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign-key database error = %v, want ErrNotFound", err)
	}
	if err := mapWorkDBError(&pgconn.PgError{Code: "23514"}); err == nil || err.Error() != "persistence: violates a persistence constraint" {
		t.Fatalf("constraint database error = %v, want typed persistence validation", err)
	}
}
