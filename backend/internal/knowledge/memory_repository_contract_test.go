package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryRepositorySupportsScopedSourceGraphAndSearch(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryRepository()
	scope := WorkspaceScope{ID: "workspace-memory-a"}
	otherScope := WorkspaceScope{ID: "workspace-memory-b"}
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)

	source := KnowledgeSource{
		ID: "memory-source", WorkspaceID: scope.ID, Kind: "manual", Name: "Release runbook",
		URI: "memory://release", Metadata: map[string]any{"owner": "platform"}, Version: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateSource(ctx, scope, source); err != nil {
		t.Fatalf("CreateSource() error = %v", err)
	}
	if err := repository.CreateSource(ctx, otherScope, KnowledgeSource{
		ID: "other-source", WorkspaceID: otherScope.ID, Kind: "manual", Name: "Other workspace",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateSource(other) error = %v", err)
	}

	item := SourceItem{
		ID: "memory-item", WorkspaceID: scope.ID, SourceID: source.ID, ExternalID: "runbook",
		Title: "Release runbook", URI: source.URI + "#item", MIMEType: "text/plain",
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateSourceItem(ctx, scope, item); err != nil {
		t.Fatalf("CreateSourceItem() error = %v", err)
	}
	revision := SourceRevision{
		ID: "memory-revision", WorkspaceID: scope.ID, SourceItemID: item.ID,
		RevisionKey: "release:v1", ContentHash: "hash-release-v1", ContentType: "text/plain",
		SourceURI: item.URI, Content: "The canonical release procedure uses a verified rollback.",
		ModifiedAt: now, IngestedAt: now,
	}
	if err := repository.CreateRevision(ctx, scope, revision); err != nil {
		t.Fatalf("CreateRevision() error = %v", err)
	}
	chunk := KnowledgeChunk{
		ID: "memory-chunk", WorkspaceID: scope.ID, RevisionID: revision.ID, Ordinal: 0,
		Text: "canonical release procedure verified rollback", TokenCount: 5, CreatedAt: now,
	}
	if err := repository.CreateChunk(ctx, scope, chunk); err != nil {
		t.Fatalf("CreateChunk() error = %v", err)
	}
	if err := repository.SetCurrentRevision(ctx, scope, item.ID, revision.ID); err != nil {
		t.Fatalf("SetCurrentRevision() error = %v", err)
	}

	topic := Topic{
		ID: "memory-topic", WorkspaceID: scope.ID, Name: "Deployments", Description: "Release knowledge",
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateTopic(ctx, scope, topic); err != nil {
		t.Fatalf("CreateTopic() error = %v", err)
	}
	if err := repository.CreateTopic(ctx, scope, Topic{
		ID: "orphan-topic", WorkspaceID: scope.ID, Name: "Orphan", ParentID: "missing-topic",
		CreatedAt: now, UpdatedAt: now,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("orphan CreateTopic() error = %v, want ErrNotFound", err)
	}
	claim := KnowledgeClaim{
		ID: "memory-claim", WorkspaceID: scope.ID, TopicID: topic.ID,
		Statement: "Release rollback is verified.", Certainty: ClaimCanonical, Freshness: ClaimCurrent,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	evidence := ClaimEvidence{
		ID: "memory-evidence", WorkspaceID: scope.ID, ClaimID: claim.ID,
		SourceRevisionID: revision.ID, ChunkID: chunk.ID, Locator: "memory://release#item",
		Quote: "The canonical release procedure uses a verified rollback.", Freshness: EvidenceCurrent, CreatedAt: now,
	}
	missingTopicClaim := claim
	missingTopicClaim.ID = "missing-topic-claim"
	missingTopicClaim.TopicID = "missing-topic"
	missingTopicEvidence := evidence
	missingTopicEvidence.ID = "missing-topic-evidence"
	missingTopicEvidence.ClaimID = missingTopicClaim.ID
	if err := repository.CreateClaimBundle(ctx, scope, missingTopicClaim, []ClaimEvidence{missingTopicEvidence}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("claim with missing topic error = %v, want ErrNotFound", err)
	}
	missingRevisionClaim := claim
	missingRevisionClaim.ID = "missing-revision-claim"
	missingRevisionEvidence := evidence
	missingRevisionEvidence.ID = "missing-revision-evidence"
	missingRevisionEvidence.ClaimID = missingRevisionClaim.ID
	missingRevisionEvidence.SourceRevisionID = "missing-revision"
	if err := repository.CreateClaimBundle(ctx, scope, missingRevisionClaim, []ClaimEvidence{missingRevisionEvidence}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("claim with missing revision error = %v, want ErrNotFound", err)
	}
	if err := repository.CreateClaimBundle(ctx, scope, claim, []ClaimEvidence{evidence}); err != nil {
		t.Fatalf("CreateClaimBundle() error = %v", err)
	}

	sources, err := repository.ListSources(ctx, scope, 10)
	if err != nil || len(sources) != 1 || sources[0].ID != source.ID {
		t.Fatalf("ListSources() = %#v, err=%v", sources, err)
	}
	if sources[0].Metadata["owner"] != "platform" {
		t.Fatalf("ListSources() lost metadata: %#v", sources[0].Metadata)
	}
	otherSources, err := repository.ListSources(ctx, otherScope, 10)
	if err != nil || len(otherSources) != 1 || otherSources[0].ID != "other-source" {
		t.Fatalf("other workspace ListSources() = %#v, err=%v", otherSources, err)
	}

	gotItem, err := repository.GetSourceItem(ctx, scope, item.ID)
	if err != nil || gotItem.CurrentRevisionID != revision.ID {
		t.Fatalf("GetSourceItem() = %+v, err=%v", gotItem, err)
	}
	items, err := repository.ListSourceItems(ctx, scope, source.ID, 10)
	if err != nil || len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("ListSourceItems() = %#v, err=%v", items, err)
	}
	revisions, err := repository.ListRevisions(ctx, scope, item.ID, 10)
	if err != nil || len(revisions) != 1 || revisions[0].ID != revision.ID {
		t.Fatalf("ListRevisions() = %#v, err=%v", revisions, err)
	}
	chunks, err := repository.ListChunks(ctx, scope, revision.ID, 10)
	if err != nil || len(chunks) != 1 || chunks[0].ID != chunk.ID {
		t.Fatalf("ListChunks() = %#v, err=%v", chunks, err)
	}
	gotTopic, err := repository.GetTopic(ctx, scope, topic.ID)
	if err != nil || gotTopic.Name != topic.Name {
		t.Fatalf("GetTopic() = %+v, err=%v", gotTopic, err)
	}
	gotClaim, err := repository.GetClaim(ctx, scope, claim.ID)
	if err != nil || gotClaim.Statement != claim.Statement {
		t.Fatalf("GetClaim() = %+v, err=%v", gotClaim, err)
	}
	evidenceItems, err := repository.ListClaimEvidence(ctx, scope, claim.ID)
	if err != nil || len(evidenceItems) != 1 || evidenceItems[0].Quote != evidence.Quote {
		t.Fatalf("ListClaimEvidence() = %#v, err=%v", evidenceItems, err)
	}

	results, err := repository.Search(ctx, scope, "verified rollback", 10)
	if err != nil || len(results) != 1 {
		t.Fatalf("Search() = %#v, err=%v", results, err)
	}
	if results[0].Source.ID != source.ID || results[0].Revision.ID != revision.ID || results[0].Chunk.ID != chunk.ID {
		t.Fatalf("Search() returned incomplete source graph: %+v", results[0])
	}
	if otherResults, err := repository.Search(ctx, otherScope, "verified rollback", 10); err != nil || len(otherResults) != 0 {
		t.Fatalf("cross-workspace Search() = %#v, err=%v", otherResults, err)
	}

	if _, err := repository.GetClaim(ctx, otherScope, claim.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-workspace GetClaim() error = %v, want ErrNotFound", err)
	}
	if _, err := repository.ListSourceItems(ctx, scope, "missing-source", 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing source ListSourceItems() error = %v, want ErrNotFound", err)
	}
	if _, err := repository.ListChunks(ctx, scope, "missing-revision", 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing revision ListChunks() error = %v, want ErrNotFound", err)
	}
}
