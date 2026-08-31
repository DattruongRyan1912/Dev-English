package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ SafeWriteChallengeStore = (*PostgresSafeWriteChallengeStore)(nil)
var _ SafeWriteChallengeIssuer = (*PostgresSafeWriteChallengeStore)(nil)
var _ SafeWriteChallengeReader = (*PostgresSafeWriteChallengeStore)(nil)
var _ SafeWriteReceiptStore = (*PostgresSafeWriteReceiptStore)(nil)

// PostgresSafeWriteChallengeStore is the durable challenge boundary. The
// target payload is not stored as executable data; only its canonical action
// identity is persisted, so confirmation must submit the same target again.
type PostgresSafeWriteChallengeStore struct {
	Pool *pgxpool.Pool
}

func NewPostgresSafeWriteChallengeStore(pool *pgxpool.Pool) (*PostgresSafeWriteChallengeStore, error) {
	if pool == nil {
		return nil, errors.New("safe-write challenge pool is required")
	}
	return &PostgresSafeWriteChallengeStore{Pool: pool}, nil
}

func (s *PostgresSafeWriteChallengeStore) Put(challenge SafeWriteChallenge) error {
	if s == nil || s.Pool == nil {
		return ErrInvalidChallenge
	}
	if err := challenge.Validate(time.Now().UTC()); err != nil {
		return err
	}
	if strings.TrimSpace(challenge.WorkspaceID) == "" {
		return ErrInvalidWorkspaceID
	}
	provider := ProviderGitHub
	prompt := "Confirm the requested GitHub action"
	if strings.HasPrefix(challenge.TargetType, "work_") {
		provider = ProviderWork
		prompt = "Confirm the requested work action"
	}
	_, err := s.Pool.Exec(context.Background(), `
		INSERT INTO action_challenges
			(id, workspace_id, user_id, action, target_type, target_id, action_hash, prompt, parameters, status, expires_at, version, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'{}'::jsonb,'pending',$9,1,$10,$10)
		ON CONFLICT (id) DO NOTHING
	`, challenge.ID, challenge.WorkspaceID, challenge.UserID, provider, challenge.TargetType, challenge.TargetID, challenge.ActionHash, prompt, challenge.ExpiresAt.UTC(), challenge.CreatedAt.UTC())
	if err != nil {
		return mapConnectorDBError(err)
	}
	return nil
}

func (s *PostgresSafeWriteChallengeStore) Get(ctx context.Context, workspaceID, userID, challengeID string) (SafeWriteChallenge, error) {
	if s == nil || s.Pool == nil || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(userID) == "" || strings.TrimSpace(challengeID) == "" {
		return SafeWriteChallenge{}, ErrInvalidChallenge
	}
	var challenge SafeWriteChallenge
	var usedAt *time.Time
	err := s.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, user_id, target_type, target_id, action_hash, created_at, expires_at, consumed_at
		FROM action_challenges
		WHERE id=$1 AND workspace_id=$2 AND user_id=$3
	`, challengeID, workspaceID, userID).Scan(&challenge.ID, &challenge.WorkspaceID, &challenge.UserID, &challenge.TargetType, &challenge.TargetID, &challenge.ActionHash, &challenge.CreatedAt, &challenge.ExpiresAt, &usedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SafeWriteChallenge{}, ErrInvalidChallenge
	}
	if err != nil {
		return SafeWriteChallenge{}, ErrInvalidChallenge
	}
	challenge.UsedAt = usedAt
	return challenge, nil
}

func (s *PostgresSafeWriteChallengeStore) Consume(ctx context.Context, metadata SafeWriteMetadata, target SafeWriteTarget, now time.Time) error {
	if s == nil || s.Pool == nil {
		return ErrInvalidChallenge
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	hash, err := target.ActionHash()
	if err != nil {
		return err
	}
	targetType, targetID := target.TargetTypeAndID()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ErrInvalidChallenge
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var challenge SafeWriteChallenge
	var usedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT id, workspace_id, user_id, target_type, target_id, action_hash, created_at, expires_at, consumed_at
		FROM action_challenges WHERE id=$1 FOR UPDATE
	`, metadata.ChallengeID).Scan(&challenge.ID, &challenge.WorkspaceID, &challenge.UserID, &challenge.TargetType, &challenge.TargetID, &challenge.ActionHash, &challenge.CreatedAt, &challenge.ExpiresAt, &usedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidChallenge
	}
	if err != nil {
		return ErrInvalidChallenge
	}
	challenge.UsedAt = usedAt
	if challenge.UsedAt != nil {
		return ErrChallengeUsed
	}
	if !challenge.ExpiresAt.After(now) {
		return ErrChallengeExpired
	}
	if challenge.UserID != metadata.UserID || (metadata.WorkspaceID != "" && challenge.WorkspaceID != metadata.WorkspaceID) || challenge.TargetType != targetType || challenge.TargetID != targetID || challenge.ActionHash != metadata.ActionHash || challenge.ActionHash != hash {
		return ErrInvalidChallenge
	}
	used := now.UTC()
	result, err := tx.Exec(ctx, `UPDATE action_challenges SET status='consumed', consumed_at=$2, updated_at=$2, version=version+1 WHERE id=$1 AND consumed_at IS NULL`, metadata.ChallengeID, used)
	if err != nil {
		return ErrInvalidChallenge
	}
	if result.RowsAffected() != 1 {
		return ErrChallengeUsed
	}
	if err := tx.Commit(ctx); err != nil {
		return ErrInvalidChallenge
	}
	return nil
}

type safeWriteReceiptOutput struct {
	Issue   GitHubIssue   `json:"issue,omitempty"`
	Comment GitHubComment `json:"comment,omitempty"`
	Labels  []GitHubLabel `json:"labels,omitempty"`
}

// PostgresSafeWriteReceiptStore persists only normalized provider output. It
// never stores the bearer token or raw provider response body.
type PostgresSafeWriteReceiptStore struct {
	Pool *pgxpool.Pool
}

func NewPostgresSafeWriteReceiptStore(pool *pgxpool.Pool) (*PostgresSafeWriteReceiptStore, error) {
	if pool == nil {
		return nil, errors.New("safe-write receipt pool is required")
	}
	return &PostgresSafeWriteReceiptStore{Pool: pool}, nil
}

func (s *PostgresSafeWriteReceiptStore) Lookup(ctx context.Context, workspaceID, userID string, operation SafeWriteOperation, idempotencyKey string) (SafeWriteReceipt, bool, error) {
	if s == nil || s.Pool == nil {
		return SafeWriteReceipt{}, false, ErrReceiptPersistence
	}
	var receipt SafeWriteReceipt
	var raw []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, user_id, action, challenge_id, idempotency_key, action_hash, target_type, target_id, status, output, COALESCE(completed_at, created_at)
		FROM action_receipts
		WHERE workspace_id=$1 AND user_id=$2 AND action=$3 AND idempotency_key=$4
		ORDER BY created_at DESC LIMIT 1
	`, workspaceID, userID, string(operation), idempotencyKey).Scan(&receipt.ID, &receipt.WorkspaceID, &receipt.UserID, &receipt.Operation, &receipt.ChallengeID, &receipt.IdempotencyKey, &receipt.ActionHash, &receipt.TargetType, &receipt.TargetID, &receipt.Status, &raw, &receipt.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SafeWriteReceipt{}, false, nil
	}
	if err != nil {
		return SafeWriteReceipt{}, false, ErrReceiptPersistence
	}
	var output safeWriteReceiptOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return SafeWriteReceipt{}, false, ErrReceiptPersistence
	}
	receipt.Provider = ProviderForSafeWriteOperation(receipt.Operation)
	receipt.Issue, receipt.Comment, receipt.Labels = output.Issue, output.Comment, output.Labels
	return receipt, true, nil
}

func (s *PostgresSafeWriteReceiptStore) Save(ctx context.Context, receipt SafeWriteReceipt) error {
	if s == nil || s.Pool == nil || strings.TrimSpace(receipt.WorkspaceID) == "" || strings.TrimSpace(receipt.ChallengeID) == "" {
		return ErrReceiptPersistence
	}
	output, err := json.Marshal(safeWriteReceiptOutput{Issue: receipt.Issue, Comment: receipt.Comment, Labels: receipt.Labels})
	if err != nil {
		return ErrReceiptPersistence
	}
	created := receipt.CreatedAt.UTC()
	if created.IsZero() {
		created = time.Now().UTC()
	}
	result, err := s.Pool.Exec(ctx, `
		INSERT INTO action_receipts
			(id, challenge_id, workspace_id, user_id, action, action_hash, target_type, target_id, idempotency_key, status, output, completed_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12,$12,$12)
		ON CONFLICT DO NOTHING
	`, receipt.ID, receipt.ChallengeID, receipt.WorkspaceID, receipt.UserID, string(receipt.Operation), receipt.ActionHash, receipt.TargetType, receipt.TargetID, receipt.IdempotencyKey, receipt.Status, output, created)
	if err != nil {
		return ErrReceiptPersistence
	}
	if result.RowsAffected() == 0 {
		previous, found, lookupErr := s.Lookup(ctx, receipt.WorkspaceID, receipt.UserID, receipt.Operation, receipt.IdempotencyKey)
		if lookupErr != nil {
			return lookupErr
		}
		if !found || previous.ActionHash != receipt.ActionHash {
			return ErrIdempotencyConflict
		}
	}
	return nil
}

// Reserve creates the durable pending receipt before any GitHub request. The
// unique idempotency index is the cross-process lock; a conflict is returned
// to the caller as an existing receipt rather than being silently replaced.
func (s *PostgresSafeWriteReceiptStore) Reserve(ctx context.Context, receipt SafeWriteReceipt) (SafeWriteReceipt, bool, error) {
	if s == nil || s.Pool == nil || strings.TrimSpace(receipt.WorkspaceID) == "" || strings.TrimSpace(receipt.ChallengeID) == "" || strings.TrimSpace(receipt.UserID) == "" || strings.TrimSpace(receipt.IdempotencyKey) == "" || strings.TrimSpace(receipt.ActionHash) == "" || receipt.Status != SafeWriteReceiptPending {
		return SafeWriteReceipt{}, false, ErrReceiptPersistence
	}
	created := receipt.CreatedAt.UTC()
	if created.IsZero() {
		created = time.Now().UTC()
	}
	result, err := s.Pool.Exec(ctx, `
		INSERT INTO action_receipts
			(id, challenge_id, workspace_id, user_id, action, action_hash, target_type, target_id, idempotency_key, status, output, completed_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'{}'::jsonb,NULL,$11,$11)
		ON CONFLICT DO NOTHING
	`, receipt.ID, receipt.ChallengeID, receipt.WorkspaceID, receipt.UserID, string(receipt.Operation), receipt.ActionHash, receipt.TargetType, receipt.TargetID, receipt.IdempotencyKey, SafeWriteReceiptPending, created)
	if err != nil {
		return SafeWriteReceipt{}, false, ErrReceiptPersistence
	}
	if result.RowsAffected() == 1 {
		receipt.CreatedAt = created
		return receipt, false, nil
	}
	previous, found, lookupErr := s.lookupByWorkspace(ctx, receipt.WorkspaceID, receipt.IdempotencyKey)
	if lookupErr != nil {
		return SafeWriteReceipt{}, false, lookupErr
	}
	if !found {
		return SafeWriteReceipt{}, false, ErrReceiptPersistence
	}
	return previous, true, nil
}

// Complete transitions only the reservation created for this exact action.
// A zero-row update is never treated as success: the caller must reconcile the
// durable row instead of issuing another provider mutation.
func (s *PostgresSafeWriteReceiptStore) Complete(ctx context.Context, receipt SafeWriteReceipt) error {
	if s == nil || s.Pool == nil || strings.TrimSpace(receipt.ID) == "" || strings.TrimSpace(receipt.WorkspaceID) == "" || strings.TrimSpace(receipt.UserID) == "" || strings.TrimSpace(receipt.IdempotencyKey) == "" || strings.TrimSpace(receipt.ActionHash) == "" || receipt.Status != SafeWriteReceiptAccepted {
		return ErrReceiptPersistence
	}
	output, err := json.Marshal(safeWriteReceiptOutput{Issue: receipt.Issue, Comment: receipt.Comment, Labels: receipt.Labels})
	if err != nil {
		return ErrReceiptPersistence
	}
	completed := receipt.CreatedAt.UTC()
	if completed.IsZero() {
		completed = time.Now().UTC()
	}
	result, err := s.Pool.Exec(ctx, `
		UPDATE action_receipts
		SET status=$2, output=$3::jsonb, completed_at=$4, updated_at=$4
		WHERE id=$1 AND workspace_id=$5 AND user_id=$6 AND action=$7
		  AND idempotency_key=$8 AND action_hash=$9 AND status=$10
	`, receipt.ID, SafeWriteReceiptAccepted, output, completed, receipt.WorkspaceID, receipt.UserID, string(receipt.Operation), receipt.IdempotencyKey, receipt.ActionHash, SafeWriteReceiptPending)
	if err != nil {
		return ErrReceiptPersistence
	}
	if result.RowsAffected() == 1 {
		return nil
	}
	previous, found, lookupErr := s.lookupByWorkspace(ctx, receipt.WorkspaceID, receipt.IdempotencyKey)
	if lookupErr != nil {
		return lookupErr
	}
	if !found {
		return ErrReceiptPersistence
	}
	if previous.ActionHash != receipt.ActionHash || previous.ID != receipt.ID {
		return ErrIdempotencyConflict
	}
	if previous.Status == SafeWriteReceiptAccepted {
		return nil
	}
	return ErrReceiptUncertain
}

func (s *PostgresSafeWriteReceiptStore) lookupByWorkspace(ctx context.Context, workspaceID, idempotencyKey string) (SafeWriteReceipt, bool, error) {
	if s == nil || s.Pool == nil {
		return SafeWriteReceipt{}, false, ErrReceiptPersistence
	}
	var receipt SafeWriteReceipt
	var raw []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, user_id, action, challenge_id, idempotency_key, action_hash, target_type, target_id, status, output, COALESCE(completed_at, created_at)
		FROM action_receipts
		WHERE workspace_id=$1 AND idempotency_key=$2
		ORDER BY created_at DESC LIMIT 1
	`, workspaceID, idempotencyKey).Scan(&receipt.ID, &receipt.WorkspaceID, &receipt.UserID, &receipt.Operation, &receipt.ChallengeID, &receipt.IdempotencyKey, &receipt.ActionHash, &receipt.TargetType, &receipt.TargetID, &receipt.Status, &raw, &receipt.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SafeWriteReceipt{}, false, nil
	}
	if err != nil {
		return SafeWriteReceipt{}, false, ErrReceiptPersistence
	}
	var output safeWriteReceiptOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return SafeWriteReceipt{}, false, ErrReceiptPersistence
	}
	receipt.Provider = ProviderForSafeWriteOperation(receipt.Operation)
	receipt.Issue, receipt.Comment, receipt.Labels = output.Issue, output.Comment, output.Labels
	return receipt, true, nil
}
