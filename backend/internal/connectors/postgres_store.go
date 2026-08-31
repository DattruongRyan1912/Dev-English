package connectors

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ DriveRevisionStore = (*PostgresRevisionStore)(nil)
var _ DriveRemovalStore = (*PostgresRevisionStore)(nil)
var _ GitHubRevisionStore = (*PostgresGitHubRevisionStore)(nil)

const (
	driveKnowledgeSourceID  = "connector-google-drive"
	gitHubKnowledgeSourceID = "connector-github"
)

// PostgresRevisionStore stores normalized connector output in the canonical
// Knowledge tables. A store is scoped to one workspace so the cursor and
// revision checks cannot accidentally cross tenant boundaries.
type PostgresRevisionStore struct {
	Pool       *pgxpool.Pool
	Workspace  string
	SourceID   string
	Provider   string
	SourceName string
	Embedder   knowledge.EmbeddingProvider
}

func NewPostgresDriveRevisionStore(pool *pgxpool.Pool, workspaceID string) (*PostgresRevisionStore, error) {
	return newPostgresRevisionStore(pool, workspaceID, ProviderGoogleDrive, driveKnowledgeSourceID, "Google Drive")
}

// NewPostgresDriveRevisionStoreWithEmbedder enables passage vectors while
// retaining the original constructor for callers that intentionally run in
// lexical-only mode.
func NewPostgresDriveRevisionStoreWithEmbedder(pool *pgxpool.Pool, workspaceID string, embedder knowledge.EmbeddingProvider) (*PostgresRevisionStore, error) {
	store, err := newPostgresRevisionStore(pool, workspaceID, ProviderGoogleDrive, driveKnowledgeSourceID, "Google Drive")
	if err != nil {
		return nil, err
	}
	store.Embedder = embedder
	return store, nil
}

func NewPostgresGitHubRevisionStore(pool *pgxpool.Pool, workspaceID string) (*PostgresGitHubRevisionStore, error) {
	store, err := newPostgresRevisionStore(pool, workspaceID, ProviderGitHub, gitHubKnowledgeSourceID, "GitHub")
	if err != nil {
		return nil, err
	}
	return &PostgresGitHubRevisionStore{PostgresRevisionStore: store}, nil
}

// NewPostgresGitHubRevisionStoreWithEmbedder is the vector-enabled GitHub
// equivalent of NewPostgresGitHubRevisionStore.
func NewPostgresGitHubRevisionStoreWithEmbedder(pool *pgxpool.Pool, workspaceID string, embedder knowledge.EmbeddingProvider) (*PostgresGitHubRevisionStore, error) {
	store, err := newPostgresRevisionStore(pool, workspaceID, ProviderGitHub, gitHubKnowledgeSourceID, "GitHub")
	if err != nil {
		return nil, err
	}
	store.Embedder = embedder
	return &PostgresGitHubRevisionStore{PostgresRevisionStore: store}, nil
}

// PostgresGitHubRevisionStore is a distinct type because Go does not support
// overloaded PutRevision methods for the Drive and GitHub provider models.
type PostgresGitHubRevisionStore struct {
	*PostgresRevisionStore
}

func newPostgresRevisionStore(pool *pgxpool.Pool, workspaceID, provider, sourceID, sourceName string) (*PostgresRevisionStore, error) {
	if pool == nil {
		return nil, errors.New("connector PostgreSQL pool is required")
	}
	if err := (knowledge.WorkspaceScope{ID: workspaceID}).Validate(); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	return &PostgresRevisionStore{
		Pool:       pool,
		Workspace:  workspaceID,
		SourceID:   stableConnectorID("knowledge-source", workspaceID+":"+sourceID),
		Provider:   provider,
		SourceName: sourceName,
	}, nil
}

func (s *PostgresRevisionStore) HasRevision(ctx context.Context, revisionKey string) (bool, error) {
	if err := s.validate(); err != nil {
		return false, err
	}
	var exists bool
	err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM source_revisions
			WHERE workspace_id=$1 AND revision_key=$2
		)
	`, s.Workspace, strings.TrimSpace(revisionKey)).Scan(&exists)
	if err != nil {
		return false, mapConnectorDBError(err)
	}
	return exists, nil
}

func (s *PostgresRevisionStore) PutRevision(ctx context.Context, item DriveSourceItem) error {
	if err := item.Validate(); err != nil {
		return err
	}
	return s.put(ctx, connectorRevision{
		ExternalID:  item.FileID,
		Title:       item.Name,
		MIMEType:    item.MIMEType,
		URI:         item.WebURL,
		RevisionKey: item.RevisionKey(),
		ContentHash: item.ContentHash,
		Content:     item.Text,
		ModifiedAt:  item.ModifiedTime,
		ItemID:      s.scopedConnectorID("drive-item", item.FileID),
		RevisionID:  s.scopedConnectorID("drive-revision", item.RevisionKey()),
		ChunkID:     s.scopedConnectorID("drive-chunk", item.RevisionKey()),
	})
}

// MarkRemoved tombstones the active source item without mutating its
// immutable revisions. Replaying the same removal is a no-op, which keeps
// changes-feed retries idempotent and removes all of the item's chunks from
// retrieval through the existing deleted_at filters.
func (s *PostgresRevisionStore) MarkRemoved(ctx context.Context, fileID string, removedAt time.Time) error {
	if err := s.validate(); err != nil {
		return err
	}
	if s.Provider != ProviderGoogleDrive {
		return ErrRemovalNotSupported
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return ErrInvalidRevisionIdentity
	}
	if removedAt.IsZero() {
		removedAt = time.Now().UTC()
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE source_items
		SET current_revision_id=NULL, deleted_at=$4, version=version+1, updated_at=$4
		WHERE workspace_id=$1
		  AND (source_id=$2 OR source_id=(
			SELECT id FROM knowledge_sources
			WHERE workspace_id=$1 AND kind=$5 AND uri=$6
			ORDER BY deleted_at NULLS FIRST, updated_at DESC
			LIMIT 1
		  ))
		  AND external_id=$3 AND deleted_at IS NULL
	`, s.Workspace, s.SourceID, fileID, removedAt.UTC(), s.Provider, s.sourceURI())
	return mapConnectorDBError(err)
}

func (s *PostgresRevisionStore) SaveCursor(ctx context.Context, state DriveSyncState) error {
	if err := s.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(state.WorkspaceID) != s.Workspace {
		return ErrInvalidWorkspaceID
	}
	return s.saveCursor(ctx, "", state.Cursor.Token, state.HasMore, state.LastSynced)
}

func (s *PostgresRevisionStore) LoadCursor(ctx context.Context, workspaceID string) (DriveSyncState, bool, error) {
	if err := s.validate(); err != nil {
		return DriveSyncState{}, false, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID != s.Workspace {
		return DriveSyncState{}, false, ErrInvalidWorkspaceID
	}
	var state DriveSyncState
	var cursor string
	err := s.Pool.QueryRow(ctx, `
		SELECT cursor, has_more, last_synced_at
		FROM connector_sync_cursors
		WHERE workspace_id=$1 AND provider=$2 AND target=''
	`, s.Workspace, ProviderGoogleDrive).Scan(&cursor, &state.HasMore, &state.LastSynced)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveSyncState{}, false, nil
	}
	if err != nil {
		return DriveSyncState{}, false, mapConnectorDBError(err)
	}
	state.WorkspaceID = s.Workspace
	state.Cursor = DriveCursor{Token: strings.TrimSpace(cursor)}
	if state.HasMore && !state.Cursor.Valid() {
		return DriveSyncState{}, false, ErrInvalidSyncState
	}
	return state, true, nil
}

func (s *PostgresGitHubRevisionStore) PutRevision(ctx context.Context, item GitHubImportItem) error {
	if err := item.Validate(); err != nil {
		return err
	}
	identity := item.ExternalID
	if identity < 1 {
		identity = item.Number
	}
	repository := normalizeRepository(item.Repository)
	uri := fmt.Sprintf("https://github.com/%s/issues/%d", repository, identity)
	content := fmt.Sprintf("Title: %s\nState: %s\nURL: %s\n\n%s", item.Title, item.State, uri, strings.TrimSpace(item.Body))
	contentHash := sha256.Sum256([]byte(content))
	return s.put(ctx, connectorRevision{
		ExternalID:  fmt.Sprintf("%s#%d", repository, identity),
		Title:       item.Title,
		MIMEType:    "text/plain",
		URI:         uri,
		RevisionKey: item.RevisionKey(),
		ContentHash: hex.EncodeToString(contentHash[:]),
		Content:     content,
		ModifiedAt:  item.UpdatedAt,
		ItemID:      s.scopedConnectorID("github-item", repository+fmt.Sprintf("#%d", identity)),
		RevisionID:  s.scopedConnectorID("github-revision", item.RevisionKey()),
		ChunkID:     s.scopedConnectorID("github-chunk", item.RevisionKey()),
	})
}

func (s *PostgresGitHubRevisionStore) SaveCursor(ctx context.Context, state GitHubSyncState) error {
	if err := s.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(state.WorkspaceID) != s.Workspace || normalizeRepository(state.Repository) == "" {
		return ErrInvalidWorkspaceID
	}
	return s.saveCursor(ctx, normalizeRepository(state.Repository), state.Cursor, state.HasMore, state.LastSynced)
}

func (s *PostgresGitHubRevisionStore) LoadCursor(ctx context.Context, workspaceID, repository string) (GitHubSyncState, bool, error) {
	if err := s.validate(); err != nil {
		return GitHubSyncState{}, false, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	repository = normalizeRepository(repository)
	if workspaceID != s.Workspace {
		return GitHubSyncState{}, false, ErrInvalidWorkspaceID
	}
	if repository == "" {
		return GitHubSyncState{}, false, ErrInvalidRepository
	}
	var state GitHubSyncState
	err := s.Pool.QueryRow(ctx, `
		SELECT cursor, has_more, last_synced_at
		FROM connector_sync_cursors
		WHERE workspace_id=$1 AND provider=$2 AND target=$3
	`, s.Workspace, ProviderGitHub, repository).Scan(&state.Cursor, &state.HasMore, &state.LastSynced)
	if errors.Is(err, pgx.ErrNoRows) {
		return GitHubSyncState{}, false, nil
	}
	if err != nil {
		return GitHubSyncState{}, false, mapConnectorDBError(err)
	}
	state.WorkspaceID = s.Workspace
	state.Repository = repository
	if state.HasMore && strings.TrimSpace(state.Cursor) == "" {
		return GitHubSyncState{}, false, ErrInvalidSyncState
	}
	return state, true, nil
}

func (s *PostgresRevisionStore) StartSyncRun(ctx context.Context, provider, workspaceID, target, cursorBefore string, startedAt time.Time) (string, error) {
	if err := s.validate(); err != nil {
		return "", err
	}
	if strings.TrimSpace(provider) != s.Provider || strings.TrimSpace(workspaceID) != s.Workspace {
		return "", ErrInvalidWorkspaceID
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	runID := newSyncRunID(provider, workspaceID, target, startedAt)
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO connector_sync_runs
			(id, workspace_id, provider, target, cursor_before, status, started_at)
		VALUES ($1,$2,$3,$4,$5,'running',$6)
	`, runID, s.Workspace, provider, strings.TrimSpace(target), strings.TrimSpace(cursorBefore), startedAt.UTC())
	if err != nil {
		return "", mapConnectorDBError(err)
	}
	return runID, nil
}

func (s *PostgresRevisionStore) CompleteSyncRun(ctx context.Context, runID, status string, summary SyncRunSummary, cursorAfter, errorCode string, completedAt time.Time) error {
	if err := s.validate(); err != nil {
		return err
	}
	status = strings.TrimSpace(status)
	if status != "succeeded" && status != "failed" {
		return errors.New("connector sync run status is invalid")
	}
	if strings.TrimSpace(runID) == "" || summary.Seen < 0 || summary.Upserted < 0 || summary.Skipped < 0 {
		return errors.New("connector sync run is invalid")
	}
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	result, err := s.Pool.Exec(ctx, `
		UPDATE connector_sync_runs
		SET status=$2, cursor_after=$3, seen_count=$4, upserted_count=$5,
		    skipped_count=$6, error_code=$7, completed_at=$8
		WHERE id=$1 AND workspace_id=$9 AND provider=$10 AND status='running'
	`, strings.TrimSpace(runID), status, strings.TrimSpace(cursorAfter), summary.Seen,
		summary.Upserted, summary.Skipped, strings.TrimSpace(errorCode), completedAt.UTC(), s.Workspace, s.Provider)
	if err != nil {
		return mapConnectorDBError(err)
	}
	if result.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

type connectorRevision struct {
	ExternalID  string
	Title       string
	MIMEType    string
	URI         string
	RevisionKey string
	ContentHash string
	Content     string
	ModifiedAt  time.Time
	ItemID      string
	RevisionID  string
	ChunkID     string
	Embedding   []float32
}

func (s *PostgresRevisionStore) put(ctx context.Context, input connectorRevision) error {
	if err := s.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(input.ExternalID) == "" || strings.TrimSpace(input.RevisionKey) == "" || strings.TrimSpace(input.Content) == "" {
		return ErrInvalidRevisionIdentity
	}
	if strings.TrimSpace(input.ContentHash) == "" {
		hash := sha256.Sum256([]byte(input.Content))
		input.ContentHash = hex.EncodeToString(hash[:])
	}
	now := time.Now().UTC()
	if input.ModifiedAt.IsZero() {
		input.ModifiedAt = now
	}
	input.Embedding = s.embedDocument(ctx, input.Content)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return mapConnectorDBError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	sourceID := s.SourceID
	var existingSourceID string
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM knowledge_sources
		WHERE workspace_id=$1 AND (id=$2 OR (kind=$3 AND uri=$4))
		ORDER BY deleted_at NULLS FIRST, updated_at DESC
		LIMIT 1
	`, s.Workspace, s.SourceID, s.Provider, s.sourceURI()).Scan(&existingSourceID)
	if err == nil {
		sourceID = existingSourceID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return mapConnectorDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO knowledge_sources (id, workspace_id, kind, name, uri, metadata, version, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,'{}'::jsonb,1,$6,$6)
		ON CONFLICT (workspace_id,id) DO UPDATE
		SET updated_at=EXCLUDED.updated_at, deleted_at=NULL
	`, sourceID, s.Workspace, s.Provider, s.SourceName, s.sourceURI(), now); err != nil {
		return mapConnectorDBError(err)
	}
	// Older connector rows used a hash that was not workspace-scoped. Reuse
	// the existing item identity when one is already present so an upgrade
	// does not create a duplicate item or violate the active external index.
	var existingItemID string
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM source_items
		WHERE workspace_id=$1 AND source_id=$2 AND external_id=$3
		ORDER BY deleted_at NULLS FIRST, updated_at DESC
		LIMIT 1
	`, s.Workspace, sourceID, input.ExternalID).Scan(&existingItemID)
	if err == nil {
		input.ItemID = existingItemID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return mapConnectorDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO source_items (id, workspace_id, source_id, external_id, title, uri, mime_type, version, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,1,$8,$8)
		ON CONFLICT (workspace_id,id) DO UPDATE
		SET title=EXCLUDED.title, uri=EXCLUDED.uri, mime_type=EXCLUDED.mime_type,
		    version=source_items.version+1, updated_at=EXCLUDED.updated_at, deleted_at=NULL
	`, input.ItemID, s.Workspace, sourceID, input.ExternalID, input.Title, input.URI, input.MIMEType, now); err != nil {
		return mapConnectorDBError(err)
	}
	result, err := tx.Exec(ctx, `
		INSERT INTO source_revisions
			(id, workspace_id, source_item_id, revision_key, content_hash, content_type, source_uri, content, modified_at, ingested_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (workspace_id, source_item_id, revision_key) DO NOTHING
	`, input.RevisionID, s.Workspace, input.ItemID, input.RevisionKey, input.ContentHash, input.MIMEType, input.URI, input.Content, input.ModifiedAt, now)
	if err != nil {
		return mapConnectorDBError(err)
	}
	if result.RowsAffected() == 0 {
		return ErrRevisionAlreadyExists
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO knowledge_chunks (id, workspace_id, revision_id, ordinal, chunk_text, token_count, embedding, created_at)
		VALUES ($1,$2,$3,0,$4,$5,$6::vector,$7)
		ON CONFLICT (workspace_id,id) DO NOTHING
	`, input.ChunkID, s.Workspace, input.RevisionID, input.Content, len(strings.Fields(input.Content)), connectorVectorValue(input.Embedding), now); err != nil {
		return mapConnectorDBError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE source_items AS item
		SET current_revision_id=$3, version=item.version+1, updated_at=$4
		WHERE item.workspace_id=$1 AND item.id=$2 AND item.deleted_at IS NULL
		  AND (
			item.current_revision_id IS NULL OR
			COALESCE((SELECT modified_at FROM source_revisions WHERE workspace_id=item.workspace_id AND id=item.current_revision_id),
			         (SELECT ingested_at FROM source_revisions WHERE workspace_id=item.workspace_id AND id=item.current_revision_id))
			<= COALESCE((SELECT modified_at FROM source_revisions WHERE workspace_id=$1 AND id=$3),
			            (SELECT ingested_at FROM source_revisions WHERE workspace_id=$1 AND id=$3))
		  )
	`, s.Workspace, input.ItemID, input.RevisionID, now); err != nil {
		return mapConnectorDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return mapConnectorDBError(err)
	}
	return nil
}

func (s *PostgresRevisionStore) embedDocument(ctx context.Context, content string) []float32 {
	if s == nil || s.Embedder == nil {
		return nil
	}
	provider, ok := s.Embedder.(knowledge.DocumentEmbeddingProvider)
	if !ok {
		return nil
	}
	vector, err := provider.EmbedDocument(ctx, content)
	if err != nil {
		if ctx != nil && ctx.Err() != nil {
			return nil
		}
		return nil
	}
	if len(vector) != knowledge.EmbeddingDimensions {
		return nil
	}
	return vector
}

func connectorVectorValue(values []float32) any {
	if len(values) == 0 {
		return nil
	}
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.FormatFloat(float64(value), 'g', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (s *PostgresRevisionStore) saveCursor(ctx context.Context, target, cursor string, hasMore bool, lastSynced time.Time) error {
	if lastSynced.IsZero() {
		lastSynced = time.Now().UTC()
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO connector_sync_cursors (workspace_id, provider, target, cursor, has_more, last_synced_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$6)
		ON CONFLICT (workspace_id, provider, target) DO UPDATE
		SET cursor=EXCLUDED.cursor, has_more=EXCLUDED.has_more,
		    last_synced_at=EXCLUDED.last_synced_at, updated_at=EXCLUDED.updated_at
	`, s.Workspace, s.Provider, target, strings.TrimSpace(cursor), hasMore, lastSynced.UTC())
	return mapConnectorDBError(err)
}

func (s *PostgresRevisionStore) sourceURI() string {
	if s.Provider == ProviderGoogleDrive {
		return "drive://workspace"
	}
	return "github://workspace"
}

func (s *PostgresRevisionStore) validate() error {
	if s == nil || s.Pool == nil {
		return errors.New("connector PostgreSQL store is not configured")
	}
	if err := (knowledge.WorkspaceScope{ID: s.Workspace}).Validate(); err != nil {
		return err
	}
	return nil
}

func stableConnectorID(prefix, value string) string {
	hash := sha256.Sum256([]byte(value))
	return prefix + "-" + hex.EncodeToString(hash[:12])
}

func (s *PostgresRevisionStore) scopedConnectorID(prefix, value string) string {
	return stableConnectorID(prefix, s.Workspace+":"+value)
}

func mapConnectorDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "23505":
			return ErrRevisionAlreadyExists
		case "23503":
			return ErrNotFound
		case "23514":
			return ErrInvalidRevisionIdentity
		}
	}
	return errors.New("connector persistence failed")
}
