package knowledge

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestPostgresRepositoryCRUDAndSearch(t *testing.T) {
	pool, ctx := openKnowledgeTestDatabase(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "knowledge-adapter-user-" + suffix
	workspaceID := "knowledge-adapter-workspace-" + suffix
	sourceID := "knowledge-adapter-source-" + suffix
	itemID := "knowledge-adapter-item-" + suffix
	revisionID := "knowledge-adapter-revision-" + suffix
	chunkID := "knowledge-adapter-chunk-" + suffix
	topicID := "knowledge-adapter-topic-" + suffix
	childTopicID := "knowledge-adapter-child-topic-" + suffix
	claimID := "knowledge-adapter-claim-" + suffix
	evidenceID := "knowledge-adapter-evidence-" + suffix

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, $2)`, userID, userID); err != nil {
		t.Fatalf("seed knowledge adapter user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO workspaces (id, owner_user_id, name, slug)
		VALUES ($1, $2, $3, $4)
	`, workspaceID, userID, "Knowledge adapter", "knowledge-adapter-"+suffix); err != nil {
		t.Fatalf("seed knowledge adapter workspace: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `UPDATE source_items SET current_revision_id=NULL WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM claim_evidence WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM knowledge_claims WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM knowledge_chunks WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM source_revisions WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM source_items WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM topics WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM knowledge_sources WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM workspaces WHERE id=$1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
	})

	repository, err := NewPostgresRepository(pool)
	if err != nil {
		t.Fatalf("NewPostgresRepository() error = %v", err)
	}
	scope := WorkspaceScope{ID: workspaceID}
	now := time.Now().UTC().Truncate(time.Microsecond)
	source := KnowledgeSource{
		ID: sourceID, WorkspaceID: workspaceID, Kind: "manual", Name: "Adapter source",
		URI: "manual://adapter/" + suffix, Metadata: map[string]any{"origin": "integration"}, CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateSource(ctx, scope, source); err != nil {
		t.Fatalf("CreateSource() error = %v", err)
	}
	if err := repository.CreateSource(ctx, scope, source); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate CreateSource() error = %v, want ErrConflict", err)
	}
	gotSource, err := repository.GetSource(ctx, scope, sourceID)
	if err != nil || gotSource.Name != source.Name || gotSource.Metadata["origin"] != "integration" {
		t.Fatalf("GetSource() = %+v, err=%v", gotSource, err)
	}
	sources, err := repository.ListSources(ctx, scope, 0)
	if err != nil || len(sources) != 1 || sources[0].ID != sourceID {
		t.Fatalf("ListSources() = %#v, err=%v", sources, err)
	}

	item := SourceItem{
		ID: itemID, WorkspaceID: workspaceID, SourceID: sourceID, ExternalID: "adapter-item",
		Title: "Adapter item", URI: source.URI + "#item", MIMEType: "text/plain", CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateSourceItem(ctx, scope, item); err != nil {
		t.Fatalf("CreateSourceItem() error = %v", err)
	}
	if err := repository.CreateSourceItem(ctx, scope, item); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate CreateSourceItem() error = %v, want ErrConflict", err)
	}
	if got, err := repository.GetSourceItem(ctx, scope, itemID); err != nil || got.ExternalID != item.ExternalID {
		t.Fatalf("GetSourceItem() = %+v, err=%v", got, err)
	}
	items, err := repository.ListSourceItems(ctx, scope, sourceID, 0)
	if err != nil || len(items) != 1 || items[0].ID != itemID {
		t.Fatalf("ListSourceItems() = %#v, err=%v", items, err)
	}

	revision := SourceRevision{
		ID: revisionID, WorkspaceID: workspaceID, SourceItemID: itemID, RevisionKey: "adapter-revision",
		ContentHash: "adapter-hash", ContentType: "text/plain", SourceURI: item.URI,
		Content: "The adapter uses PostgreSQL and hybrid retrieval.", ModifiedAt: now, IngestedAt: now,
	}
	if err := repository.CreateRevision(ctx, scope, revision); err != nil {
		t.Fatalf("CreateRevision() error = %v", err)
	}
	if err := repository.CreateRevision(ctx, scope, revision); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate CreateRevision() error = %v, want ErrConflict", err)
	}
	if got, err := repository.GetRevision(ctx, scope, revisionID); err != nil || got.Content != revision.Content || !got.ModifiedAt.Equal(now) {
		t.Fatalf("GetRevision() = %+v, err=%v", got, err)
	}
	revisions, err := repository.ListRevisions(ctx, scope, itemID, 0)
	if err != nil || len(revisions) != 1 || revisions[0].ID != revisionID {
		t.Fatalf("ListRevisions() = %#v, err=%v", revisions, err)
	}

	embedding := make([]float32, EmbeddingDimensions)
	embedding[0] = 1
	chunk := KnowledgeChunk{
		ID: chunkID, WorkspaceID: workspaceID, RevisionID: revisionID, Ordinal: 0,
		Text: "PostgreSQL hybrid retrieval", TokenCount: 3, Embedding: embedding, CreatedAt: now,
	}
	if err := repository.CreateChunk(ctx, scope, chunk); err != nil {
		t.Fatalf("CreateChunk() error = %v", err)
	}
	if err := repository.CreateChunk(ctx, scope, chunk); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate CreateChunk() error = %v, want ErrConflict", err)
	}
	gotChunk, err := repository.GetChunk(ctx, scope, chunkID)
	if err != nil || len(gotChunk.Embedding) != EmbeddingDimensions || gotChunk.Embedding[0] != 1 {
		t.Fatalf("GetChunk() = embedding:%d first:%v err=%v", len(gotChunk.Embedding), gotChunk.Embedding[0], err)
	}
	chunks, err := repository.ListChunks(ctx, scope, revisionID, 0)
	if err != nil || len(chunks) != 1 || chunks[0].ID != chunkID {
		t.Fatalf("ListChunks() = %#v, err=%v", chunks, err)
	}
	if err := repository.SetCurrentRevision(ctx, scope, itemID, revisionID); err != nil {
		t.Fatalf("SetCurrentRevision() error = %v", err)
	}
	if err := repository.SetCurrentRevision(ctx, scope, itemID, revisionID); err != nil {
		t.Fatalf("idempotent SetCurrentRevision() error = %v", err)
	}

	topic := Topic{ID: topicID, WorkspaceID: workspaceID, Name: "Adapter topic", Description: "Topic", CreatedAt: now, UpdatedAt: now}
	if err := repository.CreateTopic(ctx, scope, topic); err != nil {
		t.Fatalf("CreateTopic() error = %v", err)
	}
	childTopic := Topic{ID: childTopicID, WorkspaceID: workspaceID, Name: "Adapter child", ParentID: topicID, CreatedAt: now, UpdatedAt: now}
	if err := repository.CreateTopic(ctx, scope, childTopic); err != nil {
		t.Fatalf("CreateTopic(child) error = %v", err)
	}
	if got, err := repository.GetTopic(ctx, scope, childTopicID); err != nil || got.ParentID != topicID {
		t.Fatalf("GetTopic() = %+v, err=%v", got, err)
	}

	claim := KnowledgeClaim{
		ID: claimID, WorkspaceID: workspaceID, TopicID: childTopicID,
		Statement: "The adapter uses PostgreSQL.", Certainty: ClaimCanonical, Freshness: ClaimCurrent,
		CreatedAt: now, UpdatedAt: now,
	}
	evidence := ClaimEvidence{
		ID: evidenceID, WorkspaceID: workspaceID, ClaimID: claimID, SourceRevisionID: revisionID,
		ChunkID: chunkID, Locator: "adapter://1", Quote: "The adapter uses PostgreSQL.", Freshness: EvidenceCurrent, CreatedAt: now,
	}
	if err := repository.CreateClaimBundle(ctx, scope, claim, []ClaimEvidence{evidence}); err != nil {
		t.Fatalf("CreateClaimBundle() error = %v", err)
	}
	if err := repository.CreateClaimBundle(ctx, scope, claim, []ClaimEvidence{evidence}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate CreateClaimBundle() error = %v, want ErrConflict", err)
	}
	if got, err := repository.GetClaim(ctx, scope, claimID); err != nil || got.TopicID != childTopicID {
		t.Fatalf("GetClaim() = %+v, err=%v", got, err)
	}
	evidenceItems, err := repository.ListClaimEvidence(ctx, scope, claimID)
	if err != nil || len(evidenceItems) != 1 || evidenceItems[0].ID != evidenceID {
		t.Fatalf("ListClaimEvidence() = %#v, err=%v", evidenceItems, err)
	}
	sourceEvidence, err := repository.ListEvidenceForSource(ctx, scope, sourceID, 0)
	if err != nil || len(sourceEvidence) != 1 || sourceEvidence[0].ID != evidenceID {
		t.Fatalf("ListEvidenceForSource() = %#v, err=%v", sourceEvidence, err)
	}

	lexical, err := repository.Search(ctx, scope, "PostgreSQL", 0)
	if err != nil || len(lexical) != 1 || lexical[0].Chunk.ID != chunkID || lexical[0].Source.Metadata["origin"] != "integration" {
		t.Fatalf("Search() = %#v, err=%v", lexical, err)
	}
	naturalLanguage, err := repository.Search(ctx, scope, "What does the PostgreSQL retrieval approach use?", 0)
	if err != nil || len(naturalLanguage) != 1 || naturalLanguage[0].Chunk.ID != chunkID {
		t.Fatalf("natural-language Search() = %#v, err=%v", naturalLanguage, err)
	}
	if empty, err := repository.Search(ctx, scope, " ", 20); err != nil || len(empty) != 0 {
		t.Fatalf("empty Search() = %#v, err=%v", empty, err)
	}
	hybrid, err := repository.SearchHybrid(ctx, scope, "PostgreSQL", embedding, 0)
	if err != nil || len(hybrid) != 1 || hybrid[0].Chunk.ID != chunkID || len(hybrid[0].Chunk.Embedding) != EmbeddingDimensions {
		t.Fatalf("SearchHybrid() = %#v, err=%v", hybrid, err)
	}
	if _, err := repository.SearchHybrid(ctx, scope, "PostgreSQL", []float32{1}, 20); err == nil {
		t.Fatal("invalid hybrid embedding unexpectedly accepted")
	}
}
