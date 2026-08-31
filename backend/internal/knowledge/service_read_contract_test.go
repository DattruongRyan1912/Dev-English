package knowledge

import (
	"context"
	"errors"
	"testing"
)

func TestServiceReadsStayScopedAndComposeSourceDetail(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryRepository()
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := NewWorkspaceScope("service-read-workspace")
	if err != nil {
		t.Fatal(err)
	}

	source := KnowledgeSource{
		ID: "service-source", WorkspaceID: scope.ID, Kind: "manual", Name: "Release guide",
		URI: "memory://release-guide",
	}
	item := SourceItem{
		ID: "service-item", WorkspaceID: scope.ID, SourceID: source.ID,
		ExternalID: "release-guide", Title: source.Name, URI: source.URI,
	}
	revision := SourceRevision{
		ID: "service-revision", WorkspaceID: scope.ID, SourceItemID: item.ID,
		RevisionKey: "release-guide:v1", ContentHash: "service-hash",
		ContentType: "text/plain", SourceURI: item.URI,
		Content: "Use a verified rollback when the release fails.",
	}
	chunk := KnowledgeChunk{
		ID: "service-chunk", WorkspaceID: scope.ID, RevisionID: revision.ID,
		Ordinal: 0, Text: "verified rollback release", TokenCount: 3,
	}
	topic := Topic{ID: "service-topic", WorkspaceID: scope.ID, Name: "Releases"}
	claim := KnowledgeClaim{
		ID: "service-claim", WorkspaceID: scope.ID, TopicID: topic.ID,
		Statement: "Releases use verified rollback.", Certainty: ClaimCanonical,
		Freshness: ClaimCurrent,
	}
	evidence := ClaimEvidence{
		ID: "service-evidence", WorkspaceID: scope.ID, ClaimID: claim.ID,
		SourceRevisionID: revision.ID, ChunkID: chunk.ID,
		Locator: "memory://release-guide#0", Quote: revision.Content,
		Freshness: EvidenceCurrent,
	}

	if err := service.CreateSource(ctx, scope, source); err != nil {
		t.Fatalf("CreateSource() error = %v", err)
	}
	if err := service.CreateSourceItem(ctx, scope, item); err != nil {
		t.Fatalf("CreateSourceItem() error = %v", err)
	}
	if err := service.AppendRevision(ctx, scope, revision); err != nil {
		t.Fatalf("AppendRevision() error = %v", err)
	}
	if err := service.CreateChunk(ctx, scope, chunk); err != nil {
		t.Fatalf("CreateChunk() error = %v", err)
	}
	if err := repository.SetCurrentRevision(ctx, scope, item.ID, revision.ID); err != nil {
		t.Fatalf("SetCurrentRevision() error = %v", err)
	}
	if err := service.CreateTopic(ctx, scope, topic); err != nil {
		t.Fatalf("CreateTopic() error = %v", err)
	}
	if err := service.CreateClaimBundle(ctx, scope, claim, []ClaimEvidence{evidence}); err != nil {
		t.Fatalf("CreateClaimBundle() error = %v", err)
	}

	if got, err := service.GetSource(ctx, scope, source.ID); err != nil || got.ID != source.ID {
		t.Fatalf("GetSource() = %+v, error = %v", got, err)
	}
	if got, err := service.ListSources(ctx, scope, 10); err != nil || len(got) != 1 || got[0].ID != source.ID {
		t.Fatalf("ListSources() = %#v, error = %v", got, err)
	}
	if got, err := service.GetSourceItem(ctx, scope, item.ID); err != nil || got.CurrentRevisionID != revision.ID {
		t.Fatalf("GetSourceItem() = %+v, error = %v", got, err)
	}
	if got, err := service.GetRevision(ctx, scope, revision.ID); err != nil || got.Content != revision.Content {
		t.Fatalf("GetRevision() = %+v, error = %v", got, err)
	}
	if got, err := service.GetChunk(ctx, scope, chunk.ID); err != nil || got.Text != chunk.Text {
		t.Fatalf("GetChunk() = %+v, error = %v", got, err)
	}
	if got, err := service.GetClaim(ctx, scope, claim.ID); err != nil || got.Statement != claim.Statement {
		t.Fatalf("GetClaim() = %+v, error = %v", got, err)
	}
	if got, err := service.ListClaimEvidence(ctx, scope, claim.ID); err != nil || len(got) != 1 || got[0].ID != evidence.ID {
		t.Fatalf("ListClaimEvidence() = %#v, error = %v", got, err)
	}
	if got, err := service.Search(ctx, scope, "verified rollback", 10); err != nil || len(got) != 1 || got[0].Chunk.ID != chunk.ID {
		t.Fatalf("Search() = %#v, error = %v", got, err)
	}
	if got, err := service.Search(ctx, scope, "   ", 10); err != nil || len(got) != 0 {
		t.Fatalf("blank Search() = %#v, error = %v", got, err)
	}

	detail, err := service.GetSourceDetail(ctx, scope, source.ID)
	if err != nil {
		t.Fatalf("GetSourceDetail() error = %v", err)
	}
	if detail.Source.ID != source.ID || len(detail.Items) != 1 || len(detail.Revisions) != 1 || len(detail.Chunks) != 1 || len(detail.Evidence) != 1 {
		t.Fatalf("GetSourceDetail() = %+v", detail)
	}

	otherScope, err := NewWorkspaceScope("service-read-other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetSource(ctx, otherScope, source.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-workspace GetSource() error = %v, want ErrNotFound", err)
	}
	if _, err := service.GetClaim(ctx, otherScope, claim.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-workspace GetClaim() error = %v, want ErrNotFound", err)
	}
}
