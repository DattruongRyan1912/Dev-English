package application

import (
	"context"
	"errors"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewMemory creates the explicit development/test composition. It is not
// selected when a database URL is configured.
func NewMemory() (*App, error) {
	workRepository := work.NewMemoryRepository()
	knowledgeRepository := knowledge.NewMemoryRepository()
	return New(workRepository, knowledgeRepository, NewMemoryConversationRepository(), nil)
}

// NewPostgres creates the durable S1 composition. The caller owns the pool
// lifecycle; the application only owns services and repositories.
func NewPostgres(pool *pgxpool.Pool) (*App, error) {
	if pool == nil {
		return nil, errors.New("application postgres pool is required")
	}
	workRepository, err := work.NewPostgresRepository(pool)
	if err != nil {
		return nil, err
	}
	knowledgeRepository, err := knowledge.NewPostgresRepository(pool)
	if err != nil {
		return nil, err
	}
	conversations, err := NewPostgresConversationRepository(pool)
	if err != nil {
		return nil, err
	}
	return New(workRepository, knowledgeRepository, conversations, &PostgresWorkspaceRepository{Pool: pool})
}

// EnsureDefaultScope is useful for startup smoke tests and local diagnostics;
// request handling calls Scope and therefore also handles newly authenticated
// users lazily.
func (a *App) EnsureDefaultScope(ctx context.Context) error {
	_, _, _, err := a.Scope(ctx)
	return err
}
