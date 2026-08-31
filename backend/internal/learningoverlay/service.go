package learningoverlay

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidInput = errors.New("learning observation input is invalid")
	ErrNotFound     = errors.New("learning observation not found")
)

// Scope is deliberately independent from the legacy learning store. An
// observation may refer to a work item, but it can never become a mutation of
// that item.
type Scope struct {
	WorkspaceID string
	UserID      string
}

func (s Scope) Validate() error {
	if !validIdentifier(s.WorkspaceID) {
		return fmt.Errorf("%w: workspace id is required", ErrInvalidInput)
	}
	if !validIdentifier(s.UserID) {
		return fmt.Errorf("%w: user id is required", ErrInvalidInput)
	}
	return nil
}

type ObservationInput struct {
	SourceType string `json:"sourceType"`
	SourceID   string `json:"sourceId"`
	Skill      string `json:"skill"`
	Prompt     string `json:"prompt"`
	Response   string `json:"response"`
	Feedback   string `json:"feedback"`
}

type Observation struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	UserID      string    `json:"userId"`
	SourceType  string    `json:"sourceType"`
	SourceID    string    `json:"sourceId,omitempty"`
	Skill       string    `json:"skill"`
	Prompt      string    `json:"prompt"`
	Response    string    `json:"response"`
	Feedback    string    `json:"feedback,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (i ObservationInput) validate() error {
	if value := strings.TrimSpace(i.SourceType); value != "" && !validIdentifier(value) {
		return fmt.Errorf("%w: source type is invalid", ErrInvalidInput)
	}
	if value := strings.TrimSpace(i.SourceID); value != "" && !validIdentifier(value) {
		return fmt.Errorf("%w: source id is invalid", ErrInvalidInput)
	}
	if strings.TrimSpace(i.Skill) == "" {
		return fmt.Errorf("%w: skill is required", ErrInvalidInput)
	}
	if strings.TrimSpace(i.Prompt) == "" {
		return fmt.Errorf("%w: prompt is required", ErrInvalidInput)
	}
	if strings.TrimSpace(i.Response) == "" {
		return fmt.Errorf("%w: response is required", ErrInvalidInput)
	}
	if len(i.Skill) > 120 || len(i.Prompt) > 20000 || len(i.Response) > 20000 || len(i.Feedback) > 20000 {
		return fmt.Errorf("%w: observation field is too long", ErrInvalidInput)
	}
	return nil
}

type Repository interface {
	Save(context.Context, Scope, Observation) error
	List(context.Context, Scope, int) ([]Observation, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
	sequence   atomic.Uint64
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, errors.New("learning overlay repository is required")
	}
	return &Service{repository: repository, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s *Service) Record(ctx context.Context, scope Scope, input ObservationInput) (Observation, error) {
	if err := scope.Validate(); err != nil {
		return Observation{}, err
	}
	if err := input.validate(); err != nil {
		return Observation{}, err
	}
	now := s.now().UTC()
	observation := Observation{
		ID:          fmt.Sprintf("learning-observation-%d-%d", now.UnixNano(), s.sequence.Add(1)),
		WorkspaceID: strings.TrimSpace(scope.WorkspaceID),
		UserID:      strings.TrimSpace(scope.UserID),
		SourceType:  strings.TrimSpace(input.SourceType),
		SourceID:    strings.TrimSpace(input.SourceID),
		Skill:       strings.TrimSpace(input.Skill),
		Prompt:      strings.TrimSpace(input.Prompt),
		Response:    strings.TrimSpace(input.Response),
		Feedback:    strings.TrimSpace(input.Feedback),
		CreatedAt:   now,
	}
	if err := s.repository.Save(ctx, scope, observation); err != nil {
		return Observation{}, err
	}
	return observation, nil
}

func (s *Service) List(ctx context.Context, scope Scope, limit int) ([]Observation, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repository.List(ctx, scope, limit)
}

type MemoryRepository struct {
	mu           sync.RWMutex
	observations map[string]Observation
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{observations: make(map[string]Observation)}
}

func (r *MemoryRepository) Save(ctx context.Context, scope Scope, observation Observation) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if observation.WorkspaceID != scope.WorkspaceID || observation.UserID != scope.UserID || observation.ID == "" {
		return fmt.Errorf("%w: observation scope mismatch", ErrInvalidInput)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.observations[observation.ID]; exists {
		return fmt.Errorf("%w: observation already exists", ErrInvalidInput)
	}
	r.observations[observation.ID] = observation
	return nil
}

func (r *MemoryRepository) List(ctx context.Context, scope Scope, limit int) ([]Observation, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Observation, 0)
	for _, item := range r.observations {
		if item.WorkspaceID == scope.WorkspaceID && item.UserID == scope.UserID {
			result = append(result, item)
		}
	}
	for i := 1; i < len(result); i++ {
		current := result[i]
		j := i - 1
		for ; j >= 0 && result[j].CreatedAt.Before(current.CreatedAt); j-- {
			result[j+1] = result[j]
		}
		result[j+1] = current
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func validIdentifier(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 200 || strings.ContainsAny(value, "\r\n\x00") {
		return false
	}
	return true
}

var _ Repository = (*MemoryRepository)(nil)

// PostgresRepository is the durable adapter for the learning overlay. It
// always filters by both workspace and user to prevent observations from
// leaking across accounts.
type PostgresRepository struct {
	Pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, errors.New("learning overlay PostgreSQL pool is required")
	}
	return &PostgresRepository{Pool: pool}, nil
}

func (r *PostgresRepository) Save(ctx context.Context, scope Scope, observation Observation) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if observation.WorkspaceID != scope.WorkspaceID || observation.UserID != scope.UserID {
		return fmt.Errorf("%w: observation scope mismatch", ErrInvalidInput)
	}
	_, err := r.Pool.Exec(ctx, `
		INSERT INTO learning_observations
			(id, workspace_id, user_id, source_type, source_id, skill, prompt, response, feedback, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, observation.ID, scope.WorkspaceID, scope.UserID, observation.SourceType, observation.SourceID,
		observation.Skill, observation.Prompt, observation.Response, observation.Feedback, observation.CreatedAt)
	return mapPostgresError(err)
}

func (r *PostgresRepository) List(ctx context.Context, scope Scope, limit int) ([]Observation, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	rows, err := r.Pool.Query(ctx, `
		SELECT id, workspace_id, user_id, source_type, source_id, skill, prompt, response, feedback, created_at
		FROM learning_observations
		WHERE workspace_id=$1 AND user_id=$2
		ORDER BY created_at DESC, id DESC
		LIMIT $3
	`, scope.WorkspaceID, scope.UserID, limit)
	if err != nil {
		return nil, mapPostgresError(err)
	}
	defer rows.Close()
	result := make([]Observation, 0)
	for rows.Next() {
		var item Observation
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.UserID, &item.SourceType, &item.SourceID, &item.Skill, &item.Prompt, &item.Response, &item.Feedback, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapPostgresError(err)
	}
	return result, nil
}

func mapPostgresError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

var _ Repository = (*PostgresRepository)(nil)
