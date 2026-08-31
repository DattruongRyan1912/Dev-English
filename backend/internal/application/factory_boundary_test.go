package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPostgresCompositionRequiresDurablePool(t *testing.T) {
	if _, err := NewPostgres(nil); err == nil {
		t.Fatal("NewPostgres(nil) should reject a missing pool")
	}
	if _, err := NewPostgresConversationRepository(nil); err == nil {
		t.Fatal("NewPostgresConversationRepository(nil) should reject a missing pool")
	}
}

func TestConversationPersistenceErrorsMapWithoutLeakingDriverDetails(t *testing.T) {
	if mapConversationDBError(nil) != nil {
		t.Fatal("nil database error should remain nil")
	}
	if !errors.Is(mapConversationDBError(pgx.ErrNoRows), ErrConversationNotFound) {
		t.Fatalf("no-rows mapping = %v, want ErrConversationNotFound", mapConversationDBError(pgx.ErrNoRows))
	}
	duplicate := &pgconn.PgError{Code: "23505", Message: "duplicate assistant conversation", Detail: "sensitive database detail"}
	if got := mapConversationDBError(duplicate); got == nil || !strings.Contains(got.Error(), "already exists") || strings.Contains(got.Error(), duplicate.Detail) {
		t.Fatalf("duplicate mapping = %v, want safe conflict message", got)
	}
	if got := mapConversationDBError(errors.New("postgres password leaked")); got == nil || got.Error() != "conversation persistence failed" || strings.Contains(got.Error(), "password") {
		t.Fatalf("generic mapping = %v, want redacted persistence error", got)
	}
}

func TestWorkspacePersistenceErrorsMapWithoutLeakingDriverDetails(t *testing.T) {
	if mapWorkspacePersistenceError(nil) != nil {
		t.Fatal("nil database error should remain nil")
	}
	foreignKey := &pgconn.PgError{Code: "23503", Message: "owner detail", Detail: "sensitive owner detail"}
	if got := mapWorkspacePersistenceError(foreignKey); !errors.Is(got, ErrWorkspaceOwnerUnavailable) || strings.Contains(got.Error(), foreignKey.Detail) {
		t.Fatalf("foreign-key mapping = %v, want safe owner error", got)
	}
	if got := mapWorkspacePersistenceError(errors.New("postgres password leaked")); !errors.Is(got, ErrWorkspacePersistence) || strings.Contains(got.Error(), "password") {
		t.Fatalf("generic mapping = %v, want redacted persistence error", got)
	}
}

func TestPostgresWorkspaceRepositoryRejectsMissingPool(t *testing.T) {
	if err := (&PostgresWorkspaceRepository{}).EnsureWorkspace(context.Background(), "workspace", "user"); !errors.Is(err, ErrWorkspacePersistence) {
		t.Fatalf("missing pool error = %v, want ErrWorkspacePersistence", err)
	}
}
