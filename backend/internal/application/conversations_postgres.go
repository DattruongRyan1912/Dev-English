package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ ConversationRepository = (*PostgresConversationRepository)(nil)
var _ workspaceEnsurer = (*PostgresWorkspaceRepository)(nil)

var (
	// ErrWorkspaceOwnerUnavailable means the authenticated user is not present
	// in the canonical users table yet. The caller must not create a workspace
	// without a durable owner row.
	ErrWorkspaceOwnerUnavailable = errors.New("workspace owner is not available")
	// ErrWorkspaceMappingRequired is returned when legacy workspace topology is
	// ambiguous. Guessing a workspace would risk exposing or mutating data in
	// the wrong scope, so an operator-owned mapping decision is required.
	ErrWorkspaceMappingRequired = errors.New("workspace mapping requires operator resolution")
	// ErrWorkspaceUnavailable covers an existing workspace that cannot be used
	// by the requested owner, including a deleted workspace or an owner mismatch.
	ErrWorkspaceUnavailable = errors.New("workspace is unavailable")
	ErrWorkspacePersistence = errors.New("workspace persistence failed")
)

type PostgresWorkspaceRepository struct {
	Pool *pgxpool.Pool
}

func (r *PostgresWorkspaceRepository) EnsureWorkspace(ctx context.Context, workspaceID, userID string) error {
	if r == nil || r.Pool == nil {
		return ErrWorkspacePersistence
	}
	if ctx == nil {
		return errors.New("workspace context is required")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	userID = strings.TrimSpace(userID)
	if workspaceID == "" || userID == "" {
		return errors.New("workspace and owner are required")
	}

	// Locking the owner row serializes lazy resolution for one user. Without
	// this transaction two concurrent first requests could both observe an
	// empty topology and race to create or select different workspaces.
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return ErrWorkspacePersistence
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var owner string
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&owner); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrWorkspaceOwnerUnavailable
		}
		return mapWorkspacePersistenceError(err)
	}

	var existingOwner string
	var deletedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT owner_user_id, deleted_at
		FROM workspaces
		WHERE id=$1
		FOR UPDATE
	`, workspaceID).Scan(&existingOwner, &deletedAt)
	if err == nil {
		if existingOwner != userID || deletedAt != nil {
			return ErrWorkspaceUnavailable
		}
		activeCount, err := workspaceCount(tx, ctx, userID, false)
		if err != nil {
			return err
		}
		if activeCount != 1 {
			return ErrWorkspaceMappingRequired
		}
		if err := tx.Commit(ctx); err != nil {
			return mapWorkspacePersistenceError(err)
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return mapWorkspacePersistenceError(err)
	}

	// A missing deterministic target is safe to create only for a user with no
	// workspace history at all. An active or deleted legacy row requires an
	// explicit mapping decision; silently creating a second workspace would
	// make canonical data inaccessible or split the user's scope.
	totalCount, err := workspaceCount(tx, ctx, userID, true)
	if err != nil {
		return err
	}
	if totalCount != 0 {
		return ErrWorkspaceMappingRequired
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO workspaces (id, owner_user_id, name, slug)
		VALUES ($1,$2,$3,$1)
	`, workspaceID, userID, defaultWorkspaceName); err != nil {
		return mapWorkspacePersistenceError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return mapWorkspacePersistenceError(err)
	}
	return nil
}

func workspaceCount(tx pgx.Tx, ctx context.Context, userID string, includeDeleted bool) (int, error) {
	query := `SELECT COUNT(*) FROM workspaces WHERE owner_user_id=$1 AND deleted_at IS NULL`
	if includeDeleted {
		query = `SELECT COUNT(*) FROM workspaces WHERE owner_user_id=$1`
	}
	var count int
	if err := tx.QueryRow(ctx, query, userID).Scan(&count); err != nil {
		return 0, mapWorkspacePersistenceError(err)
	}
	return count, nil
}

func mapWorkspacePersistenceError(err error) error {
	if err == nil {
		return nil
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23503" {
		return ErrWorkspaceOwnerUnavailable
	}
	return ErrWorkspacePersistence
}

type PostgresConversationRepository struct {
	Pool *pgxpool.Pool
}

func NewPostgresConversationRepository(pool *pgxpool.Pool) (*PostgresConversationRepository, error) {
	if pool == nil {
		return nil, errors.New("conversation pool is required")
	}
	return &PostgresConversationRepository{Pool: pool}, nil
}

func (r *PostgresConversationRepository) Create(ctx context.Context, scope work.Scope, title string, contexts ...assistant.ContextRef) (Conversation, error) {
	if err := scope.Validate(); err != nil {
		return Conversation{}, err
	}
	contextRef, err := normalizeConversationContext(contexts)
	if err != nil {
		return Conversation{}, err
	}
	now := time.Now().UTC()
	id := conversationID(scope, title, now)
	conversation := Conversation{ID: id, WorkspaceID: scope.WorkspaceID, UserID: scope.UserID, Title: firstLine(title), Status: "active", Context: contextRefPtr(contextRef), CreatedAt: now, UpdatedAt: now, Messages: []ConversationMessage{}}
	_, err = r.Pool.Exec(ctx, `
		INSERT INTO assistant_conversations (id, workspace_id, user_id, title, status, context_type, context_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,'active',$5,$6,$7,$7)
	`, conversation.ID, scope.WorkspaceID, scope.UserID, conversation.Title, contextRef.Type, contextRef.ID, now)
	if err != nil {
		return Conversation{}, mapConversationDBError(err)
	}
	return conversation, nil
}

func (r *PostgresConversationRepository) List(ctx context.Context, scope work.Scope, limit int) ([]ConversationSummary, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.Pool.Query(ctx, `
		SELECT c.id, c.workspace_id, c.user_id, c.title, c.status,
		       c.context_type, c.context_id, c.created_at, c.updated_at,
		       COUNT(m.id)::int
		FROM assistant_conversations c
		LEFT JOIN assistant_messages m
		  ON m.conversation_id=c.id AND m.workspace_id=c.workspace_id AND m.user_id=c.user_id
		WHERE c.workspace_id=$1 AND c.user_id=$2
		GROUP BY c.id, c.workspace_id, c.user_id, c.title, c.status,
		         c.context_type, c.context_id, c.created_at, c.updated_at
		ORDER BY c.updated_at DESC, c.id DESC
		LIMIT $3
	`, scope.WorkspaceID, scope.UserID, limit)
	if err != nil {
		return nil, mapConversationDBError(err)
	}
	defer rows.Close()
	items := make([]ConversationSummary, 0)
	for rows.Next() {
		var item ConversationSummary
		var contextType string
		var contextID string
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.UserID, &item.Title, &item.Status, &contextType, &contextID, &item.CreatedAt, &item.UpdatedAt, &item.MessageCount); err != nil {
			return nil, errors.New("conversation summary could not be read")
		}
		if contextType != "" || contextID != "" {
			contextRef, err := (assistant.ContextRef{Type: contextType, ID: contextID}).Normalize()
			if err != nil {
				return nil, errors.New("conversation context could not be read")
			}
			item.Context = contextRefPtr(contextRef)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.New("conversation summaries could not be read")
	}
	return items, nil
}

func (r *PostgresConversationRepository) Get(ctx context.Context, scope work.Scope, id string) (Conversation, error) {
	if err := scope.Validate(); err != nil {
		return Conversation{}, err
	}
	var conversation Conversation
	var contextType string
	var contextID string
	if id == "latest" {
		idQuery := `
			SELECT id, workspace_id, user_id, title, status, context_type, context_id, created_at, updated_at
			FROM assistant_conversations
			WHERE workspace_id=$1 AND user_id=$2
			ORDER BY updated_at DESC, id DESC LIMIT 1
		`
		if err := r.Pool.QueryRow(ctx, idQuery, scope.WorkspaceID, scope.UserID).Scan(&conversation.ID, &conversation.WorkspaceID, &conversation.UserID, &conversation.Title, &conversation.Status, &contextType, &contextID, &conversation.CreatedAt, &conversation.UpdatedAt); err != nil {
			return Conversation{}, mapConversationDBError(err)
		}
	} else if err := r.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, user_id, title, status, context_type, context_id, created_at, updated_at
		FROM assistant_conversations
		WHERE id=$1 AND workspace_id=$2 AND user_id=$3
	`, id, scope.WorkspaceID, scope.UserID).Scan(&conversation.ID, &conversation.WorkspaceID, &conversation.UserID, &conversation.Title, &conversation.Status, &contextType, &contextID, &conversation.CreatedAt, &conversation.UpdatedAt); err != nil {
		return Conversation{}, mapConversationDBError(err)
	}
	if contextType != "" || contextID != "" {
		contextRef, err := (assistant.ContextRef{Type: contextType, ID: contextID}).Normalize()
		if err != nil {
			return Conversation{}, errors.New("conversation context could not be read")
		}
		conversation.Context = contextRefPtr(contextRef)
	}
	rows, err := r.Pool.Query(ctx, `
		SELECT id, role, content, response, created_at
		FROM assistant_messages
		WHERE conversation_id=$1 AND workspace_id=$2 AND user_id=$3
		ORDER BY created_at, id
	`, conversation.ID, scope.WorkspaceID, scope.UserID)
	if err != nil {
		return Conversation{}, mapConversationDBError(err)
	}
	defer rows.Close()
	conversation.Messages = make([]ConversationMessage, 0)
	for rows.Next() {
		var message ConversationMessage
		var raw []byte
		if err := rows.Scan(&message.ID, &message.Role, &message.Content, &raw, &message.CreatedAt); err != nil {
			return Conversation{}, errors.New("conversation message could not be read")
		}
		if message.Role == "assistant" && len(raw) > 0 && string(raw) != "{}" && string(raw) != "null" {
			var response assistant.AssistantResponse
			if err := json.Unmarshal(raw, &response); err != nil {
				return Conversation{}, errors.New("conversation response could not be read")
			}
			message.Response = &response
		}
		conversation.Messages = append(conversation.Messages, message)
	}
	if err := rows.Err(); err != nil {
		return Conversation{}, errors.New("conversation messages could not be read")
	}
	return conversation, nil
}

func (r *PostgresConversationRepository) Append(ctx context.Context, scope work.Scope, conversationID, userMessage string, response assistant.AssistantResponse) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return errors.New("conversation transaction could not start")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists string
	if err := tx.QueryRow(ctx, `
		SELECT id FROM assistant_conversations
		WHERE id=$1 AND workspace_id=$2 AND user_id=$3 FOR UPDATE
	`, conversationID, scope.WorkspaceID, scope.UserID).Scan(&exists); err != nil {
		return mapConversationDBError(err)
	}
	now := time.Now().UTC()
	userID := messageID(conversationID, "user", userMessage, now)
	assistantRaw, err := json.Marshal(response)
	if err != nil {
		return errors.New("assistant response could not be encoded")
	}
	assistantID := messageID(conversationID, "assistant", response.Answer, now.Add(time.Microsecond))
	if _, err := tx.Exec(ctx, `
		INSERT INTO assistant_messages (id, conversation_id, workspace_id, user_id, role, content, response, created_at)
		VALUES ($1,$2,$3,$4,'user',$5,'{}'::jsonb,$6)
	`, userID, conversationID, scope.WorkspaceID, scope.UserID, strings.TrimSpace(userMessage), now); err != nil {
		return mapConversationDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO assistant_messages (id, conversation_id, workspace_id, user_id, role, content, response, created_at)
		VALUES ($1,$2,$3,$4,'assistant',$5,$6::jsonb,$7)
	`, assistantID, conversationID, scope.WorkspaceID, scope.UserID, response.Answer, assistantRaw, now.Add(time.Microsecond)); err != nil {
		return mapConversationDBError(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE assistant_conversations SET updated_at=$4 WHERE id=$1 AND workspace_id=$2 AND user_id=$3`, conversationID, scope.WorkspaceID, scope.UserID, now); err != nil {
		return mapConversationDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return mapConversationDBError(err)
	}
	return nil
}

func conversationID(scope work.Scope, title string, at time.Time) string {
	hash := sha256.Sum256([]byte(scope.WorkspaceID + "\x00" + scope.UserID + "\x00" + title + "\x00" + at.Format(time.RFC3339Nano)))
	return "conversation-" + hex.EncodeToString(hash[:10])
}

func messageID(conversationID, role, content string, at time.Time) string {
	hash := sha256.Sum256([]byte(conversationID + "\x00" + role + "\x00" + content + "\x00" + at.Format(time.RFC3339Nano)))
	return "message-" + hex.EncodeToString(hash[:10])
}

func mapConversationDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConversationNotFound
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		return errors.New("conversation record already exists")
	}
	return errors.New("conversation persistence failed")
}
