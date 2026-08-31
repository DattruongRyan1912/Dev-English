package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func boundaryKnowledgeSource(id, workspaceID string, at time.Time) KnowledgeSource {
	return KnowledgeSource{
		ID: id, WorkspaceID: workspaceID, Kind: "manual", Name: "Source " + id,
		URI: "memory://" + id, Version: 1, CreatedAt: at, UpdatedAt: at,
	}
}

func boundaryKnowledgeItem(id, workspaceID, sourceID string, at time.Time) SourceItem {
	return SourceItem{
		ID: id, WorkspaceID: workspaceID, SourceID: sourceID, ExternalID: id,
		Title: "Item " + id, URI: "memory://" + id, MIMEType: "text/plain",
		Version: 1, CreatedAt: at, UpdatedAt: at,
	}
}

func boundaryKnowledgeRevision(id, workspaceID, itemID, content string, at time.Time) SourceRevision {
	return SourceRevision{
		ID: id, WorkspaceID: workspaceID, SourceItemID: itemID,
		RevisionKey: id + ":key", ContentHash: id + ":hash", ContentType: "text/plain",
		SourceURI: "memory://" + itemID, Content: content, ModifiedAt: at, IngestedAt: at,
	}
}

func boundaryKnowledgeChunk(id, workspaceID, revisionID string, ordinal int, text string, at time.Time) KnowledgeChunk {
	return KnowledgeChunk{
		ID: id, WorkspaceID: workspaceID, RevisionID: revisionID, Ordinal: ordinal,
		Text: text, TokenCount: len(text), CreatedAt: at,
	}
}

func boundaryKnowledgeEvidence(id, workspaceID, claimID, revisionID, chunkID string, at time.Time) ClaimEvidence {
	return ClaimEvidence{
		ID: id, WorkspaceID: workspaceID, ClaimID: claimID, SourceRevisionID: revisionID,
		ChunkID: chunkID, Quote: "canonical evidence for " + claimID, Freshness: EvidenceCurrent, CreatedAt: at,
	}
}

func TestMemoryRepositoryRejectsInvalidAndDuplicateGraphWrites(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryRepository()
	scope := WorkspaceScope{ID: "memory-boundary-workspace"}
	otherScope := WorkspaceScope{ID: "memory-boundary-other"}
	now := time.Date(2026, 8, 29, 8, 0, 0, 0, time.UTC)

	source := boundaryKnowledgeSource("boundary-source", scope.ID, now)
	if err := repository.CreateSource(ctx, WorkspaceScope{}, source); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid source scope error = %v, want ErrInvalidInput", err)
	}
	wrongSource := source
	wrongSource.WorkspaceID = otherScope.ID
	if err := repository.CreateSource(ctx, scope, wrongSource); !errors.Is(err, ErrWorkspaceMismatch) {
		t.Fatalf("cross-workspace source error = %v, want ErrWorkspaceMismatch", err)
	}
	invalidSource := source
	invalidSource.Name = " "
	if err := repository.CreateSource(ctx, scope, invalidSource); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid source error = %v, want ErrInvalidInput", err)
	}
	if err := repository.CreateSource(ctx, scope, source); err != nil {
		t.Fatalf("CreateSource() error = %v", err)
	}
	if err := repository.CreateSource(ctx, scope, source); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate source error = %v, want ErrConflict", err)
	}
	if _, err := repository.GetSource(ctx, scope, "missing-source"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing source error = %v, want ErrNotFound", err)
	}
	deletedAt := now.Add(time.Minute)
	repository.mu.Lock()
	deletedSource := repository.sources[scopedID{scope.ID, source.ID}]
	deletedSource.DeletedAt = &deletedAt
	repository.sources[scopedID{scope.ID, source.ID}] = deletedSource
	repository.mu.Unlock()
	if _, err := repository.GetSource(ctx, scope, source.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted source error = %v, want ErrNotFound", err)
	}
	if got, err := repository.ListSources(ctx, scope, 10); err != nil || len(got) != 0 {
		t.Fatalf("deleted source list = %#v, error = %v", got, err)
	}
	repository.mu.Lock()
	restoredSource := repository.sources[scopedID{scope.ID, source.ID}]
	restoredSource.DeletedAt = nil
	repository.sources[scopedID{scope.ID, source.ID}] = restoredSource
	repository.mu.Unlock()

	item := boundaryKnowledgeItem("boundary-item", scope.ID, source.ID, now)
	missingSourceItem := item
	missingSourceItem.ID = "missing-source-item"
	missingSourceItem.SourceID = "missing-source"
	if err := repository.CreateSourceItem(ctx, scope, missingSourceItem); !errors.Is(err, ErrNotFound) {
		t.Fatalf("item with missing source error = %v, want ErrNotFound", err)
	}
	wrongItem := item
	wrongItem.WorkspaceID = otherScope.ID
	if err := repository.CreateSourceItem(ctx, scope, wrongItem); !errors.Is(err, ErrWorkspaceMismatch) {
		t.Fatalf("cross-workspace item error = %v, want ErrWorkspaceMismatch", err)
	}
	if err := repository.CreateSourceItem(ctx, scope, item); err != nil {
		t.Fatalf("CreateSourceItem() error = %v", err)
	}
	if err := repository.CreateSourceItem(ctx, scope, item); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate item error = %v, want ErrConflict", err)
	}
	if _, err := repository.GetSourceItem(ctx, scope, "missing-item"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing item error = %v, want ErrNotFound", err)
	}
	repository.mu.Lock()
	deletedItem := repository.items[scopedID{scope.ID, item.ID}]
	deletedItem.DeletedAt = &deletedAt
	repository.items[scopedID{scope.ID, item.ID}] = deletedItem
	repository.mu.Unlock()
	if _, err := repository.GetSourceItem(ctx, scope, item.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted item error = %v, want ErrNotFound", err)
	}
	repository.mu.Lock()
	restoredItem := repository.items[scopedID{scope.ID, item.ID}]
	restoredItem.DeletedAt = nil
	repository.items[scopedID{scope.ID, item.ID}] = restoredItem
	repository.mu.Unlock()

	revision := boundaryKnowledgeRevision("boundary-revision", scope.ID, item.ID, "alpha canonical text", now)
	missingItemRevision := revision
	missingItemRevision.ID = "missing-item-revision"
	missingItemRevision.SourceItemID = "missing-item"
	if err := repository.CreateRevision(ctx, scope, missingItemRevision); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revision with missing item error = %v, want ErrNotFound", err)
	}
	invalidRevision := revision
	invalidRevision.Content = " "
	if err := repository.CreateRevision(ctx, scope, invalidRevision); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid revision error = %v, want ErrInvalidInput", err)
	}
	if err := repository.CreateRevision(ctx, scope, revision); err != nil {
		t.Fatalf("CreateRevision() error = %v", err)
	}
	if err := repository.CreateRevision(ctx, scope, revision); !errors.Is(err, ErrConflict) {
		t.Fatalf("same immutable revision error = %v, want ErrConflict", err)
	}
	changedRevision := revision
	changedRevision.Content = "changed canonical text"
	if err := repository.CreateRevision(ctx, scope, changedRevision); !errors.Is(err, ErrRevisionImmutable) {
		t.Fatalf("changed immutable revision error = %v, want ErrRevisionImmutable", err)
	}
	if _, err := repository.GetRevision(ctx, scope, "missing-revision"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing revision error = %v, want ErrNotFound", err)
	}

	chunk := boundaryKnowledgeChunk("boundary-chunk", scope.ID, revision.ID, 0, "alpha canonical text", now)
	missingRevisionChunk := chunk
	missingRevisionChunk.ID = "missing-revision-chunk"
	missingRevisionChunk.RevisionID = "missing-revision"
	if err := repository.CreateChunk(ctx, scope, missingRevisionChunk); !errors.Is(err, ErrNotFound) {
		t.Fatalf("chunk with missing revision error = %v, want ErrNotFound", err)
	}
	invalidChunk := chunk
	invalidChunk.Text = " "
	if err := repository.CreateChunk(ctx, scope, invalidChunk); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid chunk error = %v, want ErrInvalidInput", err)
	}
	if err := repository.CreateChunk(ctx, scope, chunk); err != nil {
		t.Fatalf("CreateChunk() error = %v", err)
	}
	if err := repository.CreateChunk(ctx, scope, chunk); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate chunk error = %v, want ErrConflict", err)
	}
	if _, err := repository.GetChunk(ctx, scope, "missing-chunk"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing chunk error = %v, want ErrNotFound", err)
	}

	secondItem := boundaryKnowledgeItem("boundary-item-two", scope.ID, source.ID, now.Add(time.Second))
	if err := repository.CreateSourceItem(ctx, scope, secondItem); err != nil {
		t.Fatalf("second CreateSourceItem() error = %v", err)
	}
	if err := repository.SetCurrentRevision(ctx, scope, "missing-item", revision.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing current-revision item error = %v, want ErrNotFound", err)
	}
	if err := repository.SetCurrentRevision(ctx, scope, item.ID, "missing-revision"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing current-revision revision error = %v, want ErrNotFound", err)
	}
	if err := repository.SetCurrentRevision(ctx, scope, secondItem.ID, revision.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-item current revision error = %v, want ErrNotFound", err)
	}
	if err := repository.SetCurrentRevision(ctx, scope, item.ID, revision.ID); err != nil {
		t.Fatalf("SetCurrentRevision() error = %v", err)
	}
	updatedItem, err := repository.GetSourceItem(ctx, scope, item.ID)
	if err != nil || updatedItem.CurrentRevisionID != revision.ID || updatedItem.Version != 2 {
		t.Fatalf("updated current revision item = %+v, error = %v", updatedItem, err)
	}
	if err := repository.SetCurrentRevision(ctx, scope, item.ID, revision.ID); err != nil {
		t.Fatalf("idempotent SetCurrentRevision() error = %v", err)
	}

	if err := repository.CreateTopic(ctx, scope, Topic{ID: "invalid-topic", WorkspaceID: scope.ID, Name: " "}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid topic error = %v, want ErrInvalidInput", err)
	}
	if err := repository.CreateTopic(ctx, scope, Topic{ID: "orphan-topic-boundary", WorkspaceID: scope.ID, Name: "Orphan", ParentID: "missing-topic"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("orphan topic error = %v, want ErrNotFound", err)
	}
	topic := Topic{ID: "boundary-topic", WorkspaceID: scope.ID, Name: "Boundary topic", CreatedAt: now, UpdatedAt: now}
	if err := repository.CreateTopic(ctx, scope, topic); err != nil {
		t.Fatalf("CreateTopic() error = %v", err)
	}
	if err := repository.CreateTopic(ctx, scope, Topic{ID: "boundary-child-topic", WorkspaceID: scope.ID, Name: "Child", ParentID: topic.ID}); err != nil {
		t.Fatalf("child CreateTopic() error = %v", err)
	}
	if err := repository.CreateTopic(ctx, scope, topic); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate topic error = %v, want ErrConflict", err)
	}
	if _, err := repository.GetTopic(ctx, scope, "missing-topic"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing topic error = %v, want ErrNotFound", err)
	}
	repository.mu.Lock()
	deletedTopic := repository.topics[scopedID{scope.ID, topic.ID}]
	deletedTopic.DeletedAt = &deletedAt
	repository.topics[scopedID{scope.ID, topic.ID}] = deletedTopic
	repository.mu.Unlock()
	if _, err := repository.GetTopic(ctx, scope, topic.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted topic error = %v, want ErrNotFound", err)
	}
	if err := repository.CreateTopic(ctx, scope, Topic{ID: "deleted-parent-child", WorkspaceID: scope.ID, Name: "Child", ParentID: topic.ID}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted parent topic error = %v, want ErrNotFound", err)
	}
	repository.mu.Lock()
	deletedTopic = repository.topics[scopedID{scope.ID, topic.ID}]
	deletedTopic.DeletedAt = nil
	repository.topics[scopedID{scope.ID, topic.ID}] = deletedTopic
	repository.mu.Unlock()

	canonicalClaim := KnowledgeClaim{ID: "boundary-claim", WorkspaceID: scope.ID, TopicID: topic.ID, Statement: "Canonical boundary fact", Certainty: ClaimCanonical, Freshness: ClaimCurrent}
	evidence := boundaryKnowledgeEvidence("boundary-evidence", scope.ID, canonicalClaim.ID, revision.ID, chunk.ID, now)
	if err := repository.CreateClaimBundle(ctx, scope, canonicalClaim, nil); !errors.Is(err, ErrEvidenceRequired) {
		t.Fatalf("evidence-free canonical claim error = %v, want ErrEvidenceRequired", err)
	}
	missingTopicClaim := canonicalClaim
	missingTopicClaim.ID = "missing-topic-claim-boundary"
	missingTopicClaim.TopicID = "missing-topic"
	missingTopicEvidence := evidence
	missingTopicEvidence.ID = "missing-topic-evidence-boundary"
	missingTopicEvidence.ClaimID = missingTopicClaim.ID
	if err := repository.CreateClaimBundle(ctx, scope, missingTopicClaim, []ClaimEvidence{missingTopicEvidence}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing-topic claim error = %v, want ErrNotFound", err)
	}
	missingRevisionClaim := canonicalClaim
	missingRevisionClaim.ID = "missing-revision-claim-boundary"
	missingRevisionEvidence := evidence
	missingRevisionEvidence.ID = "missing-revision-evidence-boundary"
	missingRevisionEvidence.ClaimID = missingRevisionClaim.ID
	missingRevisionEvidence.SourceRevisionID = "missing-revision"
	if err := repository.CreateClaimBundle(ctx, scope, missingRevisionClaim, []ClaimEvidence{missingRevisionEvidence}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing-revision claim error = %v, want ErrNotFound", err)
	}
	missingChunkClaim := canonicalClaim
	missingChunkClaim.ID = "missing-chunk-claim-boundary"
	missingChunkEvidence := evidence
	missingChunkEvidence.ID = "missing-chunk-evidence-boundary"
	missingChunkEvidence.ClaimID = missingChunkClaim.ID
	missingChunkEvidence.ChunkID = "missing-chunk"
	if err := repository.CreateClaimBundle(ctx, scope, missingChunkClaim, []ClaimEvidence{missingChunkEvidence}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing-chunk claim error = %v, want ErrNotFound", err)
	}
	if err := repository.CreateClaimBundle(ctx, scope, canonicalClaim, []ClaimEvidence{evidence}); err != nil {
		t.Fatalf("CreateClaimBundle() error = %v", err)
	}
	if err := repository.CreateClaimBundle(ctx, scope, canonicalClaim, []ClaimEvidence{evidence}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate claim error = %v, want ErrConflict", err)
	}
	if _, err := repository.GetClaim(ctx, scope, "missing-claim"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing claim error = %v, want ErrNotFound", err)
	}
	if got, err := repository.ListClaimEvidence(ctx, scope, canonicalClaim.ID); err != nil || len(got) != 1 || got[0].ID != evidence.ID {
		t.Fatalf("claim evidence = %#v, error = %v", got, err)
	}
	if _, err := repository.ListClaimEvidence(ctx, scope, "missing-claim"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing claim evidence error = %v, want ErrNotFound", err)
	}
	repository.mu.Lock()
	deletedClaim := repository.claims[scopedID{scope.ID, canonicalClaim.ID}]
	deletedClaim.DeletedAt = &deletedAt
	repository.claims[scopedID{scope.ID, canonicalClaim.ID}] = deletedClaim
	repository.mu.Unlock()
	if _, err := repository.GetClaim(ctx, scope, canonicalClaim.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted claim error = %v, want ErrNotFound", err)
	}
}

func TestMemoryRepositoryBoundsListsAndSearch(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryRepository()
	scope := WorkspaceScope{ID: "memory-bounds-workspace"}
	otherScope := WorkspaceScope{ID: "memory-bounds-other"}
	now := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)

	for _, source := range []KnowledgeSource{
		boundaryKnowledgeSource("source-a", scope.ID, now),
		boundaryKnowledgeSource("source-b", scope.ID, now),
		boundaryKnowledgeSource("source-other", otherScope.ID, now),
	} {
		if err := repository.CreateSource(ctx, WorkspaceScope{ID: source.WorkspaceID}, source); err != nil {
			t.Fatalf("CreateSource(%s) error = %v", source.ID, err)
		}
	}
	if got, err := repository.ListSources(ctx, WorkspaceScope{}, 10); !errors.Is(err, ErrInvalidInput) || got != nil {
		t.Fatalf("invalid ListSources() = %#v, error = %v", got, err)
	}
	if got, err := repository.ListSources(ctx, scope, 0); err != nil || len(got) != 2 || got[0].ID != "source-a" {
		t.Fatalf("default ListSources() = %#v, error = %v", got, err)
	}
	if got, err := repository.ListSources(ctx, scope, 201); err != nil || len(got) != 2 {
		t.Fatalf("oversized ListSources() = %#v, error = %v", got, err)
	}
	if got, err := repository.ListSources(ctx, scope, 1); err != nil || len(got) != 1 || got[0].ID != "source-a" {
		t.Fatalf("bounded ListSources() = %#v, error = %v", got, err)
	}

	itemA := boundaryKnowledgeItem("item-a", scope.ID, "source-a", now)
	itemB := boundaryKnowledgeItem("item-b", scope.ID, "source-a", now)
	itemOther := boundaryKnowledgeItem("item-other", otherScope.ID, "source-other", now)
	for _, item := range []SourceItem{itemA, itemB, itemOther} {
		itemScope := WorkspaceScope{ID: item.WorkspaceID}
		if err := repository.CreateSourceItem(ctx, itemScope, item); err != nil {
			t.Fatalf("CreateSourceItem(%s) error = %v", item.ID, err)
		}
	}
	if got, err := repository.ListSourceItems(ctx, scope, "source-a", 0); err != nil || len(got) != 2 || got[0].ID != "item-a" {
		t.Fatalf("default ListSourceItems() = %#v, error = %v", got, err)
	}
	if got, err := repository.ListSourceItems(ctx, scope, "source-a", 1); err != nil || len(got) != 1 || got[0].ID != "item-a" {
		t.Fatalf("bounded ListSourceItems() = %#v, error = %v", got, err)
	}
	if _, err := repository.ListSourceItems(ctx, scope, "missing-source", 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing source item list error = %v, want ErrNotFound", err)
	}

	revisionA := boundaryKnowledgeRevision("revision-a", scope.ID, itemA.ID, "alpha beta", now)
	revisionB := boundaryKnowledgeRevision("revision-b", scope.ID, itemA.ID, "alpha", now)
	revisionOther := boundaryKnowledgeRevision("revision-other", otherScope.ID, itemOther.ID, "alpha", now)
	for _, revision := range []SourceRevision{revisionA, revisionB, revisionOther} {
		revisionScope := WorkspaceScope{ID: revision.WorkspaceID}
		if err := repository.CreateRevision(ctx, revisionScope, revision); err != nil {
			t.Fatalf("CreateRevision(%s) error = %v", revision.ID, err)
		}
	}
	if got, err := repository.ListRevisions(ctx, scope, itemA.ID, 0); err != nil || len(got) != 2 || got[0].ID != "revision-a" {
		t.Fatalf("default ListRevisions() = %#v, error = %v", got, err)
	}
	if got, err := repository.ListRevisions(ctx, scope, itemA.ID, 1); err != nil || len(got) != 1 || got[0].ID != "revision-a" {
		t.Fatalf("bounded ListRevisions() = %#v, error = %v", got, err)
	}
	if _, err := repository.ListRevisions(ctx, scope, "missing-item", 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing item revisions error = %v, want ErrNotFound", err)
	}

	chunks := []KnowledgeChunk{
		boundaryKnowledgeChunk("chunk-a0", scope.ID, revisionA.ID, 0, "alpha beta", now),
		boundaryKnowledgeChunk("chunk-a1", scope.ID, revisionA.ID, 1, "alpha", now),
		boundaryKnowledgeChunk("chunk-b0", scope.ID, revisionB.ID, 0, "alpha beta gamma", now),
	}
	for _, chunk := range chunks {
		if err := repository.CreateChunk(ctx, scope, chunk); err != nil {
			t.Fatalf("CreateChunk(%s) error = %v", chunk.ID, err)
		}
	}
	if got, err := repository.ListChunks(ctx, scope, revisionA.ID, 0); err != nil || len(got) != 2 || got[0].ID != "chunk-a0" {
		t.Fatalf("default ListChunks() = %#v, error = %v", got, err)
	}
	if got, err := repository.ListChunks(ctx, scope, revisionA.ID, 1); err != nil || len(got) != 1 || got[0].ID != "chunk-a0" {
		t.Fatalf("bounded ListChunks() = %#v, error = %v", got, err)
	}
	if _, err := repository.ListChunks(ctx, scope, "missing-revision", 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing revision chunks error = %v, want ErrNotFound", err)
	}

	if err := repository.SetCurrentRevision(ctx, scope, itemA.ID, revisionA.ID); err != nil {
		t.Fatalf("SetCurrentRevision(revision-a) error = %v", err)
	}
	itemB.CurrentRevisionID = "missing-revision"
	if err := repository.CreateSourceItem(ctx, scope, itemB); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate item with missing current revision error = %v, want ErrConflict", err)
	}
	if got, err := repository.Search(ctx, WorkspaceScope{}, "alpha", 10); !errors.Is(err, ErrInvalidInput) || got != nil {
		t.Fatalf("invalid Search() = %#v, error = %v", got, err)
	}
	if got, err := repository.Search(ctx, scope, " a ", 10); err != nil || len(got) != 0 {
		t.Fatalf("short-term Search() = %#v, error = %v", got, err)
	}
	if got, err := repository.Search(ctx, scope, "ALPHA alpha beta", 0); err != nil || len(got) != 2 || got[0].Chunk.ID != "chunk-a0" || got[1].Chunk.ID != "chunk-a1" {
		t.Fatalf("default Search() = %#v, error = %v", got, err)
	}
	if got, err := repository.Search(ctx, scope, "alpha beta", 1); err != nil || len(got) != 1 || got[0].Chunk.ID != "chunk-a0" {
		t.Fatalf("bounded Search() = %#v, error = %v", got, err)
	}
	if got, err := repository.Search(ctx, otherScope, "alpha", 10); err != nil || len(got) != 0 {
		t.Fatalf("cross-workspace Search() = %#v, error = %v", got, err)
	}

	deletedAt := now.Add(time.Minute)
	repository.mu.Lock()
	deletedItem := repository.items[scopedID{scope.ID, itemA.ID}]
	deletedItem.DeletedAt = &deletedAt
	repository.items[scopedID{scope.ID, itemA.ID}] = deletedItem
	deletedSource := repository.sources[scopedID{scope.ID, "source-a"}]
	deletedSource.DeletedAt = &deletedAt
	repository.sources[scopedID{scope.ID, "source-a"}] = deletedSource
	repository.mu.Unlock()
	if got, err := repository.ListSourceItems(ctx, scope, "source-a", 10); !errors.Is(err, ErrNotFound) || got != nil {
		t.Fatalf("deleted source item list = %#v, error = %v", got, err)
	}
	if got, err := repository.Search(ctx, scope, "alpha", 10); err != nil || len(got) != 0 {
		t.Fatalf("deleted search graph = %#v, error = %v", got, err)
	}

	if got := searchTerms("Alpha alpha a beta"); len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("searchTerms() = %#v", got)
	}
	if got := countTermMatches([]string{"alpha", "beta"}, "ALPHA only"); got != 1 {
		t.Fatalf("countTermMatches() = %d, want 1", got)
	}
}
