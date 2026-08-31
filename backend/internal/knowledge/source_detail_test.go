package knowledge

import (
	"context"
	"testing"
	"time"
)

func TestSourceDetailGroupsCanonicalTimelineAndEvidence(t *testing.T) {
	ctx := context.Background()
	scope := WorkspaceScope{ID: "workspace-source-detail"}
	now := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	repository := NewMemoryRepository()
	service, err := NewService(repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.CreateSource(ctx, scope, KnowledgeSource{
		ID: "source-release", WorkspaceID: scope.ID, Kind: "manual", Name: "Release runbook",
		URI: "manual://release", Version: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateSource() error = %v", err)
	}
	if err := service.CreateSourceItem(ctx, scope, SourceItem{
		ID: "item-release", WorkspaceID: scope.ID, SourceID: "source-release", ExternalID: "release",
		Title: "Release runbook", MIMEType: "text/plain", Version: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateSourceItem() error = %v", err)
	}
	if err := service.AppendRevision(ctx, scope, SourceRevision{
		ID: "revision-release", WorkspaceID: scope.ID, SourceItemID: "item-release", RevisionKey: "hash-1",
		ContentHash: "hash-1", ContentType: "text/plain", Content: "Use a verified rollback plan.", IngestedAt: now,
	}); err != nil {
		t.Fatalf("AppendRevision() error = %v", err)
	}
	if err := service.CreateChunk(ctx, scope, KnowledgeChunk{
		ID: "chunk-release", WorkspaceID: scope.ID, RevisionID: "revision-release", Text: "Use a verified rollback plan.", TokenCount: 5, CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateChunk() error = %v", err)
	}
	if err := repository.SetCurrentRevision(ctx, scope, "item-release", "revision-release"); err != nil {
		t.Fatalf("SetCurrentRevision() error = %v", err)
	}
	if err := service.CreateClaimBundle(ctx, scope, KnowledgeClaim{
		ID: "claim-release", WorkspaceID: scope.ID, Statement: "Rollback is verified.",
		Certainty: ClaimCanonical, Freshness: ClaimCurrent, Version: 1, CreatedAt: now, UpdatedAt: now,
	}, []ClaimEvidence{{
		ID: "evidence-release", WorkspaceID: scope.ID, ClaimID: "claim-release", SourceRevisionID: "revision-release",
		ChunkID: "chunk-release", Locator: "manual://release#1", Quote: "Use a verified rollback plan.", Freshness: EvidenceCurrent, CreatedAt: now,
	}}); err != nil {
		t.Fatalf("CreateClaimBundle() error = %v", err)
	}

	detail, err := service.GetSourceDetail(ctx, scope, "source-release")
	if err != nil {
		t.Fatalf("GetSourceDetail() error = %v", err)
	}
	if detail.Source.ID != "source-release" || len(detail.Items) != 1 || len(detail.Revisions) != 1 || len(detail.Chunks) != 1 || len(detail.Evidence) != 1 {
		t.Fatalf("GetSourceDetail() = %+v; want one source timeline item at each level", detail)
	}
	if detail.Evidence[0].Quote != "Use a verified rollback plan." {
		t.Fatalf("source evidence quote = %q", detail.Evidence[0].Quote)
	}
}
