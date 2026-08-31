package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ Repository = (*PostgresRepository)(nil)
var _ Searcher = (*PostgresRepository)(nil)
var _ HybridSearcher = (*PostgresRepository)(nil)
var _ CurrentRevisionSetter = (*PostgresRepository)(nil)
var _ SourceLister = (*PostgresRepository)(nil)
var _ SourceDetailReader = (*PostgresRepository)(nil)
var _ ManualImportIdempotencyRepository = (*PostgresRepository)(nil)
var _ ManualImportRepository = (*PostgresRepository)(nil)

// PostgresRepository is the persistence adapter for the knowledge bounded
// context. Every query includes workspace_id; the application service still
// validates the same boundary before it reaches this adapter.
type PostgresRepository struct {
	Pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, ErrNilRepository
	}
	return &PostgresRepository{Pool: pool}, nil
}

func (r *PostgresRepository) LookupManualImportIdempotency(ctx context.Context, scope WorkspaceScope, userID, idempotencyKey string) (ManualImportIdempotencyRecord, bool, error) {
	if err := scope.Validate(); err != nil {
		return ManualImportIdempotencyRecord{}, false, err
	}
	if err := requireIdentifier("userId", userID); err != nil {
		return ManualImportIdempotencyRecord{}, false, err
	}
	if err := requireIdentifier("idempotencyKey", idempotencyKey); err != nil {
		return ManualImportIdempotencyRecord{}, false, err
	}
	var record ManualImportIdempotencyRecord
	err := r.Pool.QueryRow(ctx, `
		SELECT workspace_id, user_id, operation, idempotency_key, request_hash, response
		FROM knowledge_import_idempotency
		WHERE workspace_id=$1 AND user_id=$2 AND operation=$3 AND idempotency_key=$4
	`, scope.ID, userID, "knowledge.import_manual", idempotencyKey).Scan(
		&record.WorkspaceID, &record.UserID, &record.Operation, &record.IdempotencyKey, &record.RequestHash, &record.ResponseJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return ManualImportIdempotencyRecord{}, false, nil
	}
	if err != nil {
		return ManualImportIdempotencyRecord{}, false, mapKnowledgeError(err)
	}
	return record, true, nil
}

func (r *PostgresRepository) SaveManualImportIdempotency(ctx context.Context, scope WorkspaceScope, record ManualImportIdempotencyRecord) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if record.WorkspaceID != scope.ID {
		return ErrWorkspaceMismatch
	}
	for field, value := range map[string]string{
		"userId": record.UserID, "operation": record.Operation,
		"idempotencyKey": record.IdempotencyKey, "requestHash": record.RequestHash,
	} {
		if err := requireIdentifier(field, value); err != nil {
			return err
		}
	}
	if record.Operation != "knowledge.import_manual" || len(record.ResponseJSON) == 0 {
		return ErrInvalidInput
	}
	result, err := r.Pool.Exec(ctx, `
		INSERT INTO knowledge_import_idempotency
			(workspace_id, user_id, operation, idempotency_key, request_hash, response)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb)
		ON CONFLICT (workspace_id, user_id, operation, idempotency_key) DO NOTHING
	`, record.WorkspaceID, record.UserID, record.Operation, record.IdempotencyKey, record.RequestHash, record.ResponseJSON)
	if err != nil {
		return mapKnowledgeError(err)
	}
	if result.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

// CommitManualImport reserves the idempotency key and writes the complete
// source graph in one PostgreSQL transaction. The unique-key insert happens
// before the graph writes, so a concurrent request waits for the winner and
// then replays its durable response instead of performing a second commit.
func (r *PostgresRepository) CommitManualImport(ctx context.Context, scope WorkspaceScope, request ManualImportCommitRequest) (ManualImportCommit, error) {
	if r == nil || r.Pool == nil {
		return ManualImportCommit{}, ErrNilRepository
	}
	if err := validateManualImportCommitRequest(scope, request); err != nil {
		return ManualImportCommit{}, err
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return ManualImportCommit{}, fmt.Errorf("begin manual knowledge import: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var reservedKey string
	err = tx.QueryRow(ctx, `
		INSERT INTO knowledge_import_idempotency
			(workspace_id, user_id, operation, idempotency_key, request_hash, response)
		VALUES ($1,$2,$3,$4,$5,'{}'::jsonb)
		ON CONFLICT (workspace_id, user_id, operation, idempotency_key) DO NOTHING
		RETURNING idempotency_key
	`, scope.ID, request.UserID, manualImportOperationName, request.IdempotencyKey, request.RequestHash).Scan(&reservedKey)
	if errors.Is(err, pgx.ErrNoRows) {
		record, lookupErr := manualImportIdempotencyTx(ctx, tx, scope, request.UserID, request.IdempotencyKey)
		if lookupErr != nil {
			return ManualImportCommit{}, lookupErr
		}
		return ManualImportCommit{Record: record, Replayed: true}, nil
	}
	if err != nil {
		return ManualImportCommit{}, mapKnowledgeError(err)
	}
	if reservedKey != request.IdempotencyKey {
		return ManualImportCommit{}, ErrConflict
	}

	bundle := request.Bundle
	source, err := manualImportUpsertSourceTx(ctx, tx, scope, bundle.Source)
	if err != nil {
		return ManualImportCommit{}, err
	}
	item, err := manualImportUpsertSourceItemTx(ctx, tx, scope, bundle.Item)
	if err != nil {
		return ManualImportCommit{}, err
	}
	revision, err := manualImportAppendRevisionTx(ctx, tx, scope, bundle.Revision)
	if err != nil {
		return ManualImportCommit{}, err
	}
	chunk, err := manualImportUpsertChunkTx(ctx, tx, scope, bundle.Chunk)
	if err != nil {
		return ManualImportCommit{}, err
	}
	if err := manualImportSetCurrentRevisionTx(ctx, tx, scope, item.ID, revision.ID); err != nil {
		return ManualImportCommit{}, err
	}

	projection := manualImportProjection(source, revision.ID, chunk.ID)
	responseJSON, err := json.Marshal(projection)
	if err != nil {
		return ManualImportCommit{}, fmt.Errorf("encode manual import replay: %w", err)
	}
	updated, err := tx.Exec(ctx, `
		UPDATE knowledge_import_idempotency
		SET request_hash=$5, response=$6::jsonb
		WHERE workspace_id=$1 AND user_id=$2 AND operation=$3 AND idempotency_key=$4
	`, scope.ID, request.UserID, manualImportOperationName, request.IdempotencyKey, request.RequestHash, responseJSON)
	if err != nil {
		return ManualImportCommit{}, mapKnowledgeError(err)
	}
	if updated.RowsAffected() != 1 {
		return ManualImportCommit{}, ErrConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return ManualImportCommit{}, mapKnowledgeError(err)
	}
	return ManualImportCommit{Projection: projection}, nil
}

func manualImportIdempotencyTx(ctx context.Context, tx pgx.Tx, scope WorkspaceScope, userID, idempotencyKey string) (ManualImportIdempotencyRecord, error) {
	var record ManualImportIdempotencyRecord
	err := tx.QueryRow(ctx, `
		SELECT workspace_id, user_id, operation, idempotency_key, request_hash, response
		FROM knowledge_import_idempotency
		WHERE workspace_id=$1 AND user_id=$2 AND operation=$3 AND idempotency_key=$4
	`, scope.ID, userID, manualImportOperationName, idempotencyKey).Scan(
		&record.WorkspaceID, &record.UserID, &record.Operation, &record.IdempotencyKey, &record.RequestHash, &record.ResponseJSON)
	if err != nil {
		return ManualImportIdempotencyRecord{}, mapKnowledgeError(err)
	}
	return record, nil
}

func manualImportUpsertSourceTx(ctx context.Context, tx pgx.Tx, scope WorkspaceScope, source KnowledgeSource) (KnowledgeSource, error) {
	metadata := source.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return KnowledgeSource{}, fmt.Errorf("encode knowledge metadata: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO knowledge_sources
			(id, workspace_id, kind, name, uri, metadata, version, created_at, updated_at, deleted_at)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9,$10)
		ON CONFLICT (id) DO NOTHING
	`, source.ID, scope.ID, source.Kind, source.Name, source.URI, encoded, positiveVersion(source.Version), source.CreatedAt, source.UpdatedAt, source.DeletedAt)
	if err != nil {
		return KnowledgeSource{}, mapKnowledgeError(err)
	}
	return manualImportGetSourceTx(ctx, tx, scope, source.ID)
}

func manualImportGetSourceTx(ctx context.Context, tx pgx.Tx, scope WorkspaceScope, id string) (KnowledgeSource, error) {
	var source KnowledgeSource
	var metadata []byte
	err := tx.QueryRow(ctx, `
		SELECT id, workspace_id, kind, name, uri, metadata, version, created_at, updated_at, deleted_at
		FROM knowledge_sources
		WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL
	`, scope.ID, id).Scan(&source.ID, &source.WorkspaceID, &source.Kind, &source.Name, &source.URI, &metadata, &source.Version, &source.CreatedAt, &source.UpdatedAt, &source.DeletedAt)
	if err != nil {
		return KnowledgeSource{}, mapKnowledgeError(err)
	}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &source.Metadata); err != nil {
			return KnowledgeSource{}, fmt.Errorf("decode knowledge metadata: %w", err)
		}
	}
	return source, nil
}

func manualImportUpsertSourceItemTx(ctx context.Context, tx pgx.Tx, scope WorkspaceScope, item SourceItem) (SourceItem, error) {
	_, err := tx.Exec(ctx, `
		INSERT INTO source_items
			(id, workspace_id, source_id, external_id, title, uri, mime_type, current_revision_id, version, created_at, updated_at, deleted_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,$11,$12)
		ON CONFLICT (id) DO NOTHING
	`, item.ID, scope.ID, item.SourceID, item.ExternalID, item.Title, item.URI, item.MIMEType, item.CurrentRevisionID, positiveVersion(item.Version), item.CreatedAt, item.UpdatedAt, item.DeletedAt)
	if err != nil {
		return SourceItem{}, mapKnowledgeError(err)
	}
	return manualImportGetSourceItemTx(ctx, tx, scope, item.ID)
}

func manualImportGetSourceItemTx(ctx context.Context, tx pgx.Tx, scope WorkspaceScope, id string) (SourceItem, error) {
	var item SourceItem
	err := tx.QueryRow(ctx, `
		SELECT id, workspace_id, source_id, external_id, title, uri, mime_type,
		       COALESCE(current_revision_id,''), version, created_at, updated_at, deleted_at
		FROM source_items
		WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL
	`, scope.ID, id).Scan(&item.ID, &item.WorkspaceID, &item.SourceID, &item.ExternalID, &item.Title, &item.URI, &item.MIMEType, &item.CurrentRevisionID, &item.Version, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt)
	if err != nil {
		return SourceItem{}, mapKnowledgeError(err)
	}
	return item, nil
}

func manualImportAppendRevisionTx(ctx context.Context, tx pgx.Tx, scope WorkspaceScope, revision SourceRevision) (SourceRevision, error) {
	var modifiedAt any
	if !revision.ModifiedAt.IsZero() {
		modifiedAt = revision.ModifiedAt
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO source_revisions
			(id, workspace_id, source_item_id, revision_key, content_hash, content_type, source_uri, content, modified_at, ingested_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO NOTHING
	`, revision.ID, scope.ID, revision.SourceItemID, revision.RevisionKey, revision.ContentHash, revision.ContentType, revision.SourceURI, revision.Content, modifiedAt, revision.IngestedAt)
	if err != nil {
		return SourceRevision{}, mapKnowledgeError(err)
	}
	existing, err := manualImportGetRevisionTx(ctx, tx, scope, revision.ID)
	if err != nil {
		return SourceRevision{}, err
	}
	if !sameManualImportRevision(existing, revision) {
		return SourceRevision{}, ErrRevisionImmutable
	}
	return existing, nil
}

func manualImportGetRevisionTx(ctx context.Context, tx pgx.Tx, scope WorkspaceScope, id string) (SourceRevision, error) {
	var revision SourceRevision
	var modifiedAt *time.Time
	err := tx.QueryRow(ctx, `
		SELECT id, workspace_id, source_item_id, revision_key, content_hash, content_type,
		       source_uri, content, modified_at, ingested_at
		FROM source_revisions
		WHERE workspace_id=$1 AND id=$2
	`, scope.ID, id).Scan(&revision.ID, &revision.WorkspaceID, &revision.SourceItemID, &revision.RevisionKey, &revision.ContentHash, &revision.ContentType, &revision.SourceURI, &revision.Content, &modifiedAt, &revision.IngestedAt)
	if err != nil {
		return SourceRevision{}, mapKnowledgeError(err)
	}
	revision.ModifiedAt = valueOrZero(modifiedAt)
	return revision, nil
}

func manualImportUpsertChunkTx(ctx context.Context, tx pgx.Tx, scope WorkspaceScope, chunk KnowledgeChunk) (KnowledgeChunk, error) {
	_, err := tx.Exec(ctx, `
		INSERT INTO knowledge_chunks
			(id, workspace_id, revision_id, ordinal, chunk_text, token_count, embedding, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7::vector,$8)
		ON CONFLICT (id) DO NOTHING
	`, chunk.ID, scope.ID, chunk.RevisionID, chunk.Ordinal, chunk.Text, chunk.TokenCount, vectorValue(chunk.Embedding), chunk.CreatedAt)
	if err != nil {
		return KnowledgeChunk{}, mapKnowledgeError(err)
	}
	var existing KnowledgeChunk
	var embedding *string
	err = tx.QueryRow(ctx, `
		SELECT id, workspace_id, revision_id, ordinal, chunk_text, token_count,
		       embedding::text, created_at
		FROM knowledge_chunks
		WHERE workspace_id=$1 AND id=$2
	`, scope.ID, chunk.ID).Scan(&existing.ID, &existing.WorkspaceID, &existing.RevisionID, &existing.Ordinal, &existing.Text, &existing.TokenCount, &embedding, &existing.CreatedAt)
	if err != nil {
		return KnowledgeChunk{}, mapKnowledgeError(err)
	}
	existing.Embedding = parseVector(embedding)
	if !sameManualImportChunk(existing, chunk) {
		return KnowledgeChunk{}, ErrConflict
	}
	return existing, nil
}

func manualImportSetCurrentRevisionTx(ctx context.Context, tx pgx.Tx, scope WorkspaceScope, itemID, revisionID string) error {
	result, err := tx.Exec(ctx, `
		UPDATE source_items AS item
		SET current_revision_id=$3,
		    version=CASE WHEN item.current_revision_id=$3 THEN item.version ELSE item.version+1 END,
		    updated_at=CASE WHEN item.current_revision_id=$3 THEN item.updated_at ELSE now() END
		FROM source_revisions AS revision
		WHERE item.workspace_id=$1 AND item.id=$2
		  AND revision.workspace_id=item.workspace_id
		  AND revision.id=$3 AND revision.source_item_id=item.id
		  AND item.deleted_at IS NULL
	`, scope.ID, itemID, revisionID)
	if err != nil {
		return mapKnowledgeError(err)
	}
	if result.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) CreateSource(ctx context.Context, scope WorkspaceScope, source KnowledgeSource) error {
	if source.Metadata == nil {
		source.Metadata = map[string]any{}
	}
	metadata, err := json.Marshal(source.Metadata)
	if err != nil {
		return fmt.Errorf("encode knowledge metadata: %w", err)
	}
	_, err = r.Pool.Exec(ctx, `
		INSERT INTO knowledge_sources
			(id, workspace_id, kind, name, uri, metadata, version, created_at, updated_at, deleted_at)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9,$10)
	`, source.ID, scope.ID, source.Kind, source.Name, source.URI, metadata, positiveVersion(source.Version), source.CreatedAt, source.UpdatedAt, source.DeletedAt)
	return mapKnowledgeError(err)
}

func (r *PostgresRepository) GetSource(ctx context.Context, scope WorkspaceScope, id string) (KnowledgeSource, error) {
	var source KnowledgeSource
	var metadata []byte
	err := r.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, kind, name, uri, metadata, version, created_at, updated_at, deleted_at
		FROM knowledge_sources
		WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL
	`, scope.ID, id).Scan(&source.ID, &source.WorkspaceID, &source.Kind, &source.Name, &source.URI, &metadata, &source.Version, &source.CreatedAt, &source.UpdatedAt, &source.DeletedAt)
	if err != nil {
		return KnowledgeSource{}, mapKnowledgeError(err)
	}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &source.Metadata); err != nil {
			return KnowledgeSource{}, fmt.Errorf("decode knowledge metadata: %w", err)
		}
	}
	return source, nil
}

func (r *PostgresRepository) ListSources(ctx context.Context, scope WorkspaceScope, limit int) ([]KnowledgeSource, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.Pool.Query(ctx, `
		SELECT id, workspace_id, kind, name, uri, metadata, version, created_at, updated_at, deleted_at
		FROM knowledge_sources
		WHERE workspace_id=$1 AND deleted_at IS NULL
		ORDER BY updated_at DESC, id
		LIMIT $2
	`, scope.ID, limit)
	if err != nil {
		return nil, mapKnowledgeError(err)
	}
	defer rows.Close()
	result := make([]KnowledgeSource, 0)
	for rows.Next() {
		var source KnowledgeSource
		var metadata []byte
		if err := rows.Scan(&source.ID, &source.WorkspaceID, &source.Kind, &source.Name, &source.URI, &metadata, &source.Version, &source.CreatedAt, &source.UpdatedAt, &source.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan knowledge source: %w", err)
		}
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &source.Metadata); err != nil {
				return nil, fmt.Errorf("decode knowledge source metadata: %w", err)
			}
		}
		result = append(result, source)
	}
	if err := rows.Err(); err != nil {
		return nil, mapKnowledgeError(err)
	}
	return result, nil
}

func (r *PostgresRepository) ListSourceItems(ctx context.Context, scope WorkspaceScope, sourceID string, limit int) ([]SourceItem, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if _, err := r.GetSource(ctx, scope, sourceID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.Pool.Query(ctx, `
		SELECT id, workspace_id, source_id, external_id, title, uri, mime_type,
		       COALESCE(current_revision_id,''), version, created_at, updated_at, deleted_at
		FROM source_items
		WHERE workspace_id=$1 AND source_id=$2 AND deleted_at IS NULL
		ORDER BY updated_at DESC, id
		LIMIT $3
	`, scope.ID, sourceID, limit)
	if err != nil {
		return nil, mapKnowledgeError(err)
	}
	defer rows.Close()
	items := make([]SourceItem, 0)
	for rows.Next() {
		var item SourceItem
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.SourceID, &item.ExternalID, &item.Title, &item.URI, &item.MIMEType, &item.CurrentRevisionID, &item.Version, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan knowledge source item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapKnowledgeError(err)
	}
	return items, nil
}

func (r *PostgresRepository) ListRevisions(ctx context.Context, scope WorkspaceScope, itemID string, limit int) ([]SourceRevision, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if _, err := r.GetSourceItem(ctx, scope, itemID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.Pool.Query(ctx, `
		SELECT id, workspace_id, source_item_id, revision_key, content_hash,
		       content_type, source_uri, content, modified_at, ingested_at
		FROM source_revisions
		WHERE workspace_id=$1 AND source_item_id=$2
		ORDER BY ingested_at DESC, id
		LIMIT $3
	`, scope.ID, itemID, limit)
	if err != nil {
		return nil, mapKnowledgeError(err)
	}
	defer rows.Close()
	revisions := make([]SourceRevision, 0)
	for rows.Next() {
		var revision SourceRevision
		var modifiedAt *time.Time
		if err := rows.Scan(&revision.ID, &revision.WorkspaceID, &revision.SourceItemID, &revision.RevisionKey, &revision.ContentHash, &revision.ContentType, &revision.SourceURI, &revision.Content, &modifiedAt, &revision.IngestedAt); err != nil {
			return nil, fmt.Errorf("scan knowledge source revision: %w", err)
		}
		revision.ModifiedAt = valueOrZero(modifiedAt)
		revisions = append(revisions, revision)
	}
	if err := rows.Err(); err != nil {
		return nil, mapKnowledgeError(err)
	}
	return revisions, nil
}

func (r *PostgresRepository) ListChunks(ctx context.Context, scope WorkspaceScope, revisionID string, limit int) ([]KnowledgeChunk, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if _, err := r.GetRevision(ctx, scope, revisionID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.Pool.Query(ctx, `
		SELECT id, workspace_id, revision_id, ordinal, chunk_text, token_count,
		       embedding::text, created_at
		FROM knowledge_chunks
		WHERE workspace_id=$1 AND revision_id=$2
		ORDER BY ordinal, id
		LIMIT $3
	`, scope.ID, revisionID, limit)
	if err != nil {
		return nil, mapKnowledgeError(err)
	}
	defer rows.Close()
	chunks := make([]KnowledgeChunk, 0)
	for rows.Next() {
		var chunk KnowledgeChunk
		var embedding *string
		if err := rows.Scan(&chunk.ID, &chunk.WorkspaceID, &chunk.RevisionID, &chunk.Ordinal, &chunk.Text, &chunk.TokenCount, &embedding, &chunk.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan knowledge chunk: %w", err)
		}
		chunk.Embedding = parseVector(embedding)
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, mapKnowledgeError(err)
	}
	return chunks, nil
}

func (r *PostgresRepository) ListEvidenceForSource(ctx context.Context, scope WorkspaceScope, sourceID string, limit int) ([]ClaimEvidence, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if _, err := r.GetSource(ctx, scope, sourceID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := r.Pool.Query(ctx, `
		SELECT evidence.id, evidence.workspace_id, evidence.claim_id,
		       evidence.source_revision_id, COALESCE(evidence.chunk_id,''),
		       evidence.locator, evidence.quote, evidence.freshness, evidence.created_at
		FROM claim_evidence AS evidence
		JOIN source_revisions AS revision
		  ON revision.workspace_id=evidence.workspace_id AND revision.id=evidence.source_revision_id
		JOIN source_items AS item
		  ON item.workspace_id=revision.workspace_id AND item.id=revision.source_item_id
		WHERE evidence.workspace_id=$1 AND item.source_id=$2
		ORDER BY evidence.created_at, evidence.id
		LIMIT $3
	`, scope.ID, sourceID, limit)
	if err != nil {
		return nil, mapKnowledgeError(err)
	}
	defer rows.Close()
	evidence := make([]ClaimEvidence, 0)
	for rows.Next() {
		var item ClaimEvidence
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.ClaimID, &item.SourceRevisionID, &item.ChunkID, &item.Locator, &item.Quote, &item.Freshness, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan knowledge source evidence: %w", err)
		}
		evidence = append(evidence, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapKnowledgeError(err)
	}
	return evidence, nil
}

func (r *PostgresRepository) CreateSourceItem(ctx context.Context, scope WorkspaceScope, item SourceItem) error {
	_, err := r.Pool.Exec(ctx, `
		INSERT INTO source_items
			(id, workspace_id, source_id, external_id, title, uri, mime_type, current_revision_id, version, created_at, updated_at, deleted_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,$11,$12)
	`, item.ID, scope.ID, item.SourceID, item.ExternalID, item.Title, item.URI, item.MIMEType, item.CurrentRevisionID, positiveVersion(item.Version), item.CreatedAt, item.UpdatedAt, item.DeletedAt)
	return mapKnowledgeError(err)
}

func (r *PostgresRepository) GetSourceItem(ctx context.Context, scope WorkspaceScope, id string) (SourceItem, error) {
	var item SourceItem
	err := r.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, source_id, external_id, title, uri, mime_type,
		       COALESCE(current_revision_id,''), version, created_at, updated_at, deleted_at
		FROM source_items
		WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL
	`, scope.ID, id).Scan(&item.ID, &item.WorkspaceID, &item.SourceID, &item.ExternalID, &item.Title, &item.URI, &item.MIMEType, &item.CurrentRevisionID, &item.Version, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt)
	if err != nil {
		return SourceItem{}, mapKnowledgeError(err)
	}
	return item, nil
}

func (r *PostgresRepository) CreateRevision(ctx context.Context, scope WorkspaceScope, revision SourceRevision) error {
	var modifiedAt any
	if !revision.ModifiedAt.IsZero() {
		modifiedAt = revision.ModifiedAt
	}
	_, err := r.Pool.Exec(ctx, `
		INSERT INTO source_revisions
			(id, workspace_id, source_item_id, revision_key, content_hash, content_type, source_uri, content, modified_at, ingested_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, revision.ID, scope.ID, revision.SourceItemID, revision.RevisionKey, revision.ContentHash, revision.ContentType, revision.SourceURI, revision.Content, modifiedAt, revision.IngestedAt)
	return mapKnowledgeError(err)
}

func (r *PostgresRepository) GetRevision(ctx context.Context, scope WorkspaceScope, id string) (SourceRevision, error) {
	var revision SourceRevision
	var modifiedAt *time.Time
	err := r.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, source_item_id, revision_key, content_hash, content_type,
		       source_uri, content, modified_at, ingested_at
		FROM source_revisions
		WHERE workspace_id=$1 AND id=$2
	`, scope.ID, id).Scan(&revision.ID, &revision.WorkspaceID, &revision.SourceItemID, &revision.RevisionKey, &revision.ContentHash, &revision.ContentType, &revision.SourceURI, &revision.Content, &modifiedAt, &revision.IngestedAt)
	if err != nil {
		return SourceRevision{}, mapKnowledgeError(err)
	}
	revision.ModifiedAt = valueOrZero(modifiedAt)
	return revision, nil
}

func (r *PostgresRepository) SetCurrentRevision(ctx context.Context, scope WorkspaceScope, itemID, revisionID string) error {
	result, err := r.Pool.Exec(ctx, `
		UPDATE source_items AS item
		SET current_revision_id=$3,
		    version=CASE WHEN item.current_revision_id=$3 THEN item.version ELSE item.version+1 END,
		    updated_at=CASE WHEN item.current_revision_id=$3 THEN item.updated_at ELSE now() END
		FROM source_revisions AS revision
		WHERE item.workspace_id=$1 AND item.id=$2
		  AND revision.workspace_id=item.workspace_id
		  AND revision.id=$3 AND revision.source_item_id=item.id
		  AND item.deleted_at IS NULL
	`, scope.ID, itemID, revisionID)
	if err != nil {
		return mapKnowledgeError(err)
	}
	if result.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) CreateChunk(ctx context.Context, scope WorkspaceScope, chunk KnowledgeChunk) error {
	_, err := r.Pool.Exec(ctx, `
		INSERT INTO knowledge_chunks
			(id, workspace_id, revision_id, ordinal, chunk_text, token_count, embedding, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7::vector,$8)
	`, chunk.ID, scope.ID, chunk.RevisionID, chunk.Ordinal, chunk.Text, chunk.TokenCount, vectorValue(chunk.Embedding), chunk.CreatedAt)
	return mapKnowledgeError(err)
}

func (r *PostgresRepository) GetChunk(ctx context.Context, scope WorkspaceScope, id string) (KnowledgeChunk, error) {
	var chunk KnowledgeChunk
	var embedding *string
	err := r.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, revision_id, ordinal, chunk_text, token_count,
		       embedding::text, created_at
		FROM knowledge_chunks
		WHERE workspace_id=$1 AND id=$2
	`, scope.ID, id).Scan(&chunk.ID, &chunk.WorkspaceID, &chunk.RevisionID, &chunk.Ordinal, &chunk.Text, &chunk.TokenCount, &embedding, &chunk.CreatedAt)
	if err != nil {
		return KnowledgeChunk{}, mapKnowledgeError(err)
	}
	chunk.Embedding = parseVector(embedding)
	return chunk, nil
}

func (r *PostgresRepository) CreateTopic(ctx context.Context, scope WorkspaceScope, topic Topic) error {
	_, err := r.Pool.Exec(ctx, `
		INSERT INTO topics (id, workspace_id, name, description, parent_id, version, created_at, updated_at, deleted_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9)
	`, topic.ID, scope.ID, topic.Name, topic.Description, topic.ParentID, positiveVersion(topic.Version), topic.CreatedAt, topic.UpdatedAt, topic.DeletedAt)
	return mapKnowledgeError(err)
}

func (r *PostgresRepository) GetTopic(ctx context.Context, scope WorkspaceScope, id string) (Topic, error) {
	var topic Topic
	err := r.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, name, description, COALESCE(parent_id,''), version, created_at, updated_at, deleted_at
		FROM topics WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL
	`, scope.ID, id).Scan(&topic.ID, &topic.WorkspaceID, &topic.Name, &topic.Description, &topic.ParentID, &topic.Version, &topic.CreatedAt, &topic.UpdatedAt, &topic.DeletedAt)
	if err != nil {
		return Topic{}, mapKnowledgeError(err)
	}
	return topic, nil
}

func (r *PostgresRepository) CreateClaimBundle(ctx context.Context, scope WorkspaceScope, claim KnowledgeClaim, evidence []ClaimEvidence) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin knowledge claim: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `
		INSERT INTO knowledge_claims
			(id, workspace_id, topic_id, statement, certainty, freshness, version, created_at, updated_at, deleted_at)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8,$9,$10)
	`, claim.ID, scope.ID, claim.TopicID, claim.Statement, claim.Certainty, claim.Freshness, positiveVersion(claim.Version), claim.CreatedAt, claim.UpdatedAt, claim.DeletedAt)
	if err != nil {
		return mapKnowledgeError(err)
	}
	for _, item := range evidence {
		if _, err := tx.Exec(ctx, `
			INSERT INTO claim_evidence
				(id, workspace_id, claim_id, source_revision_id, chunk_id, locator, quote, freshness, created_at)
			VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9)
		`, item.ID, scope.ID, item.ClaimID, item.SourceRevisionID, item.ChunkID, item.Locator, item.Quote, item.Freshness, item.CreatedAt); err != nil {
			return mapKnowledgeError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return mapKnowledgeError(err)
	}
	return nil
}

func (r *PostgresRepository) GetClaim(ctx context.Context, scope WorkspaceScope, id string) (KnowledgeClaim, error) {
	var claim KnowledgeClaim
	err := r.Pool.QueryRow(ctx, `
		SELECT id, workspace_id, COALESCE(topic_id,''), statement, certainty, freshness,
		       version, created_at, updated_at, deleted_at
		FROM knowledge_claims WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL
	`, scope.ID, id).Scan(&claim.ID, &claim.WorkspaceID, &claim.TopicID, &claim.Statement, &claim.Certainty, &claim.Freshness, &claim.Version, &claim.CreatedAt, &claim.UpdatedAt, &claim.DeletedAt)
	if err != nil {
		return KnowledgeClaim{}, mapKnowledgeError(err)
	}
	return claim, nil
}

func (r *PostgresRepository) ListClaimEvidence(ctx context.Context, scope WorkspaceScope, claimID string) ([]ClaimEvidence, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT id, workspace_id, claim_id, source_revision_id, COALESCE(chunk_id,''), locator, quote, freshness, created_at
		FROM claim_evidence WHERE workspace_id=$1 AND claim_id=$2 ORDER BY created_at, id
	`, scope.ID, claimID)
	if err != nil {
		return nil, mapKnowledgeError(err)
	}
	defer rows.Close()
	result := make([]ClaimEvidence, 0)
	for rows.Next() {
		var item ClaimEvidence
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.ClaimID, &item.SourceRevisionID, &item.ChunkID, &item.Locator, &item.Quote, &item.Freshness, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan claim evidence: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapKnowledgeError(err)
	}
	if len(result) == 0 {
		if _, err := r.GetClaim(ctx, scope, claimID); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *PostgresRepository) Search(ctx context.Context, scope WorkspaceScope, query string, limit int) ([]SearchResult, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []SearchResult{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	lexicalQuery := postgresLexicalQuery(query)
	rows, err := r.Pool.Query(ctx, `
		SELECT
			s.id, s.workspace_id, s.kind, s.name, s.uri, s.metadata, s.version,
			s.created_at, s.updated_at, s.deleted_at,
			i.id, i.workspace_id, i.source_id, i.external_id, i.title, i.uri,
			i.mime_type, COALESCE(i.current_revision_id,''), i.version, i.created_at,
			i.updated_at, i.deleted_at,
			r.id, r.workspace_id, r.source_item_id, r.revision_key, r.content_hash,
			r.content_type, r.source_uri, r.content, r.modified_at, r.ingested_at,
			c.id, c.workspace_id, c.revision_id, c.ordinal, c.chunk_text,
			c.token_count, c.embedding::text, c.created_at
		FROM knowledge_chunks AS c
		JOIN source_revisions AS r ON r.workspace_id=c.workspace_id AND r.id=c.revision_id
		JOIN source_items AS i ON i.workspace_id=r.workspace_id AND i.id=r.source_item_id
		JOIN knowledge_sources AS s ON s.workspace_id=i.workspace_id AND s.id=i.source_id
		WHERE c.workspace_id=$1
		  AND s.deleted_at IS NULL AND i.deleted_at IS NULL
		  AND i.current_revision_id=r.id
		  AND (
			c.search_vector @@ to_tsquery('simple',$2)
			OR lower(concat_ws(' ', s.name, s.uri, i.title, i.uri, r.content, c.chunk_text)) LIKE lower('%' || $3 || '%')
		  )
		ORDER BY (c.search_vector @@ to_tsquery('simple',$2)) DESC, c.ordinal, c.id
		LIMIT $4
	`, scope.ID, lexicalQuery, query, limit)
	if err != nil {
		return nil, mapKnowledgeError(err)
	}
	defer rows.Close()
	result := make([]SearchResult, 0)
	for rows.Next() {
		var item SearchResult
		var sourceMetadata []byte
		var sourceDeleted, itemDeleted *time.Time
		var revisionModified *time.Time
		var embedding *string
		if err := rows.Scan(
			&item.Source.ID, &item.Source.WorkspaceID, &item.Source.Kind, &item.Source.Name, &item.Source.URI, &sourceMetadata, &item.Source.Version,
			&item.Source.CreatedAt, &item.Source.UpdatedAt, &sourceDeleted,
			&item.Item.ID, &item.Item.WorkspaceID, &item.Item.SourceID, &item.Item.ExternalID, &item.Item.Title, &item.Item.URI,
			&item.Item.MIMEType, &item.Item.CurrentRevisionID, &item.Item.Version, &item.Item.CreatedAt, &item.Item.UpdatedAt, &itemDeleted,
			&item.Revision.ID, &item.Revision.WorkspaceID, &item.Revision.SourceItemID, &item.Revision.RevisionKey, &item.Revision.ContentHash,
			&item.Revision.ContentType, &item.Revision.SourceURI, &item.Revision.Content, &revisionModified, &item.Revision.IngestedAt,
			&item.Chunk.ID, &item.Chunk.WorkspaceID, &item.Chunk.RevisionID, &item.Chunk.Ordinal, &item.Chunk.Text,
			&item.Chunk.TokenCount, &embedding, &item.Chunk.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan knowledge search result: %w", err)
		}
		item.Source.DeletedAt = sourceDeleted
		item.Item.DeletedAt = itemDeleted
		item.Revision.ModifiedAt = valueOrZero(revisionModified)
		item.Chunk.Embedding = parseVector(embedding)
		if len(sourceMetadata) > 0 {
			if err := json.Unmarshal(sourceMetadata, &item.Source.Metadata); err != nil {
				return nil, fmt.Errorf("decode knowledge search metadata: %w", err)
			}
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapKnowledgeError(err)
	}
	return result, nil
}

// SearchHybrid combines PostgreSQL full-text ranking with pgvector cosine
// ranking using reciprocal rank fusion. The lexical Search method remains the
// safe fallback when the local embedding sidecar is unavailable.
func (r *PostgresRepository) SearchHybrid(ctx context.Context, scope WorkspaceScope, query string, embedding []float32, limit int) ([]SearchResult, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []SearchResult{}, nil
	}
	if len(embedding) != EmbeddingDimensions {
		return nil, invalidField("embedding", "must have exactly 384 dimensions")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	lexicalQuery := postgresLexicalQuery(query)
	rows, err := r.Pool.Query(ctx, `
		WITH candidates AS (
			SELECT c.id AS chunk_id,
			       row_number() OVER (
			       ORDER BY (c.search_vector @@ to_tsquery('simple',$2)) DESC,
			                c.ordinal, c.id
			       ) AS lexical_rank,
			       row_number() OVER (
			       ORDER BY (c.embedding IS NULL), c.embedding <=> $4::vector,
				                c.ordinal, c.id
			       ) AS semantic_rank
			FROM knowledge_chunks AS c
			JOIN source_revisions AS r ON r.workspace_id=c.workspace_id AND r.id=c.revision_id
			JOIN source_items AS i ON i.workspace_id=r.workspace_id AND i.id=r.source_item_id
			JOIN knowledge_sources AS s ON s.workspace_id=i.workspace_id AND s.id=i.source_id
			WHERE c.workspace_id=$1
			  AND s.deleted_at IS NULL AND i.deleted_at IS NULL
			  AND i.current_revision_id=r.id
			  AND (
				c.search_vector @@ to_tsquery('simple',$2)
				OR lower(concat_ws(' ', s.name, s.uri, i.title, i.uri, r.content, c.chunk_text)) LIKE lower('%' || $3 || '%')
				OR c.embedding IS NOT NULL
			  )
		), ranked AS (
			SELECT chunk_id,
			       1.0 / (60.0 + lexical_rank) + 1.0 / (60.0 + semantic_rank) AS rrf
			FROM candidates
		)
		SELECT
			s.id, s.workspace_id, s.kind, s.name, s.uri, s.metadata, s.version,
			s.created_at, s.updated_at, s.deleted_at,
			i.id, i.workspace_id, i.source_id, i.external_id, i.title, i.uri,
			i.mime_type, COALESCE(i.current_revision_id,''), i.version, i.created_at,
			i.updated_at, i.deleted_at,
			r.id, r.workspace_id, r.source_item_id, r.revision_key, r.content_hash,
			r.content_type, r.source_uri, r.content, r.modified_at, r.ingested_at,
			c.id, c.workspace_id, c.revision_id, c.ordinal, c.chunk_text,
			c.token_count, c.embedding::text, c.created_at
		FROM ranked
		JOIN knowledge_chunks AS c ON c.id=ranked.chunk_id
		JOIN source_revisions AS r ON r.workspace_id=c.workspace_id AND r.id=c.revision_id
		JOIN source_items AS i ON i.workspace_id=r.workspace_id AND i.id=r.source_item_id
		JOIN knowledge_sources AS s ON s.workspace_id=i.workspace_id AND s.id=i.source_id
		ORDER BY ranked.rrf DESC, c.ordinal, c.id
		LIMIT $5
	`, scope.ID, lexicalQuery, query, vectorValue(embedding), limit)
	if err != nil {
		return nil, mapKnowledgeError(err)
	}
	defer rows.Close()
	result := make([]SearchResult, 0)
	for rows.Next() {
		var item SearchResult
		var sourceMetadata []byte
		var sourceDeleted, itemDeleted *time.Time
		var revisionModified *time.Time
		var vector *string
		if err := rows.Scan(
			&item.Source.ID, &item.Source.WorkspaceID, &item.Source.Kind, &item.Source.Name, &item.Source.URI, &sourceMetadata, &item.Source.Version,
			&item.Source.CreatedAt, &item.Source.UpdatedAt, &sourceDeleted,
			&item.Item.ID, &item.Item.WorkspaceID, &item.Item.SourceID, &item.Item.ExternalID, &item.Item.Title, &item.Item.URI,
			&item.Item.MIMEType, &item.Item.CurrentRevisionID, &item.Item.Version, &item.Item.CreatedAt, &item.Item.UpdatedAt, &itemDeleted,
			&item.Revision.ID, &item.Revision.WorkspaceID, &item.Revision.SourceItemID, &item.Revision.RevisionKey, &item.Revision.ContentHash,
			&item.Revision.ContentType, &item.Revision.SourceURI, &item.Revision.Content, &revisionModified, &item.Revision.IngestedAt,
			&item.Chunk.ID, &item.Chunk.WorkspaceID, &item.Chunk.RevisionID, &item.Chunk.Ordinal, &item.Chunk.Text,
			&item.Chunk.TokenCount, &vector, &item.Chunk.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan hybrid knowledge result: %w", err)
		}
		item.Source.DeletedAt = sourceDeleted
		item.Item.DeletedAt = itemDeleted
		item.Revision.ModifiedAt = valueOrZero(revisionModified)
		item.Chunk.Embedding = parseVector(vector)
		if len(sourceMetadata) > 0 {
			if err := json.Unmarshal(sourceMetadata, &item.Source.Metadata); err != nil {
				return nil, fmt.Errorf("decode hybrid knowledge metadata: %w", err)
			}
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapKnowledgeError(err)
	}
	return result, nil
}

// postgresLexicalQuery turns natural-language input into a safe OR tsquery.
// plainto_tsquery requires every term to match, which is a poor fit for an
// assistant question containing words that are absent from the source text.
// Only Unicode letters and digits are retained, so provider/user input cannot
// inject tsquery operators or produce a syntax error in the database adapter.
func postgresLexicalQuery(query string) string {
	terms := make([]string, 0)
	seen := make(map[string]struct{})
	word := make([]rune, 0, 24)
	flush := func() {
		if len(word) == 0 {
			return
		}
		term := string(word)
		if _, exists := seen[term]; !exists {
			seen[term] = struct{}{}
			terms = append(terms, term)
		}
		word = word[:0]
	}
	for _, value := range strings.ToLower(query) {
		if unicode.IsLetter(value) || unicode.IsDigit(value) {
			word = append(word, value)
			continue
		}
		flush()
	}
	flush()
	return strings.Join(terms, " | ")
}

func positiveVersion(value int64) int64 {
	if value < 1 {
		return 1
	}
	return value
}

func vectorValue(values []float32) any {
	if len(values) == 0 {
		return nil
	}
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.FormatFloat(float64(value), 'g', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func parseVector(value *string) []float32 {
	if value == nil {
		return nil
	}
	raw := strings.TrimSpace(*value)
	raw = strings.TrimPrefix(strings.TrimSuffix(raw, "]"), "[")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]float32, 0, len(parts))
	for _, part := range parts {
		parsed, err := strconv.ParseFloat(strings.TrimSpace(part), 32)
		if err != nil {
			return nil
		}
		result = append(result, float32(parsed))
	}
	return result
}

func valueOrZero(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func mapKnowledgeError(err error) error {
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
			return ErrConflict
		case "23503":
			return ErrNotFound
		case "55000":
			return ErrRevisionImmutable
		}
	}
	return fmt.Errorf("knowledge persistence failed")
}
