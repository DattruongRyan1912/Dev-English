package knowledge

import (
	"context"
	"errors"
	"testing"
)

type fakeRepository struct {
	lastScope WorkspaceScope
	called    string
}

func (f *fakeRepository) capture(scope WorkspaceScope, called string) {
	f.lastScope = scope
	f.called = called
}

func (f *fakeRepository) CreateSource(_ context.Context, scope WorkspaceScope, _ KnowledgeSource) error {
	f.capture(scope, "source")
	return nil
}

func (f *fakeRepository) GetSource(context.Context, WorkspaceScope, string) (KnowledgeSource, error) {
	return KnowledgeSource{}, ErrNotFound
}

func (f *fakeRepository) CreateSourceItem(_ context.Context, scope WorkspaceScope, _ SourceItem) error {
	f.capture(scope, "source-item")
	return nil
}

func (f *fakeRepository) GetSourceItem(context.Context, WorkspaceScope, string) (SourceItem, error) {
	return SourceItem{}, ErrNotFound
}

func (f *fakeRepository) CreateRevision(_ context.Context, scope WorkspaceScope, _ SourceRevision) error {
	f.capture(scope, "revision")
	return nil
}

func (f *fakeRepository) GetRevision(context.Context, WorkspaceScope, string) (SourceRevision, error) {
	return SourceRevision{}, ErrNotFound
}

func (f *fakeRepository) CreateChunk(_ context.Context, scope WorkspaceScope, _ KnowledgeChunk) error {
	f.capture(scope, "chunk")
	return nil
}

func (f *fakeRepository) GetChunk(context.Context, WorkspaceScope, string) (KnowledgeChunk, error) {
	return KnowledgeChunk{}, ErrNotFound
}

func (f *fakeRepository) CreateTopic(_ context.Context, scope WorkspaceScope, _ Topic) error {
	f.capture(scope, "topic")
	return nil
}

func (f *fakeRepository) GetTopic(context.Context, WorkspaceScope, string) (Topic, error) {
	return Topic{}, ErrNotFound
}

func (f *fakeRepository) CreateClaimBundle(_ context.Context, scope WorkspaceScope, _ KnowledgeClaim, _ []ClaimEvidence) error {
	f.capture(scope, "claim")
	return nil
}

func (f *fakeRepository) GetClaim(context.Context, WorkspaceScope, string) (KnowledgeClaim, error) {
	return KnowledgeClaim{}, ErrNotFound
}

func (f *fakeRepository) ListClaimEvidence(context.Context, WorkspaceScope, string) ([]ClaimEvidence, error) {
	return nil, nil
}

func validClaim() KnowledgeClaim {
	return KnowledgeClaim{
		ID:          "claim-1",
		WorkspaceID: "workspace-a",
		Statement:   "The deployment uses PostgreSQL.",
		Certainty:   ClaimCanonical,
		Freshness:   ClaimCurrent,
	}
}

func validEvidence() ClaimEvidence {
	return ClaimEvidence{
		ID:               "evidence-1",
		WorkspaceID:      "workspace-a",
		ClaimID:          "claim-1",
		SourceRevisionID: "revision-1",
		Quote:            "The deployment uses PostgreSQL.",
		Freshness:        EvidenceCurrent,
	}
}

func TestWorkspaceScopeRejectsAmbiguousIdentifiers(t *testing.T) {
	if _, err := NewWorkspaceScope(" workspace-a"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid workspace scope, got %v", err)
	}
	if _, err := NewWorkspaceScope(""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected blank workspace scope to fail, got %v", err)
	}
}

func TestServiceRejectsCrossWorkspaceEntitiesBeforeRepository(t *testing.T) {
	fake := &fakeRepository{}
	service, err := NewService(fake)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := NewWorkspaceScope("workspace-a")
	if err != nil {
		t.Fatal(err)
	}
	err = service.CreateSource(context.Background(), scope, KnowledgeSource{
		ID:          "source-1",
		WorkspaceID: "workspace-b",
		Kind:        "manual",
		Name:        "Other workspace",
	})
	if !errors.Is(err, ErrWorkspaceMismatch) {
		t.Fatalf("expected workspace mismatch, got %v", err)
	}
	if fake.called != "" {
		t.Fatalf("repository must not be called for cross-workspace entity: %s", fake.called)
	}
}

func TestCanonicalClaimRequiresCurrentEvidence(t *testing.T) {
	claim := validClaim()
	if err := ValidateClaimBundle(claim, nil); !errors.Is(err, ErrEvidenceRequired) {
		t.Fatalf("expected evidence requirement, got %v", err)
	}
	stale := validEvidence()
	stale.Freshness = EvidenceStale
	if err := ValidateClaimBundle(claim, []ClaimEvidence{stale}); !errors.Is(err, ErrStaleEvidence) {
		t.Fatalf("expected stale evidence rejection, got %v", err)
	}
	if err := ValidateClaimBundle(claim, []ClaimEvidence{validEvidence()}); err != nil {
		t.Fatalf("expected current evidence to satisfy canonical claim: %v", err)
	}
}

func TestEvidenceFreeClaimMustBeNonCanonical(t *testing.T) {
	claim := validClaim()
	claim.Certainty = ClaimInferred
	if err := ValidateClaimBundle(claim, nil); err != nil {
		t.Fatalf("inferred claim without evidence should be valid: %v", err)
	}
	claim.Certainty = ClaimUnknown
	if err := ValidateClaimBundle(claim, nil); err != nil {
		t.Fatalf("unknown claim without evidence should be valid: %v", err)
	}
}

func TestClaimBundleRejectsDuplicateOrMismatchedEvidence(t *testing.T) {
	claim := validClaim()
	first := validEvidence()
	second := validEvidence()
	second.Quote = "same evidence id"
	if err := ValidateClaimBundle(claim, []ClaimEvidence{first, second}); !errors.Is(err, ErrDuplicateEvidence) {
		t.Fatalf("expected duplicate evidence rejection, got %v", err)
	}
	wrongClaim := validEvidence()
	wrongClaim.ID = "evidence-2"
	wrongClaim.ClaimID = "claim-2"
	if err := ValidateClaimBundle(claim, []ClaimEvidence{wrongClaim}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected mismatched claim evidence rejection, got %v", err)
	}
}

func TestRevisionIsAppendOnlyAndFullFieldComparison(t *testing.T) {
	revision := SourceRevision{
		ID:           "revision-1",
		WorkspaceID:  "workspace-a",
		SourceItemID: "item-1",
		RevisionKey:  "provider:revision-1",
		ContentHash:  "hash-1",
		Content:      "immutable source text",
	}
	if err := revision.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := AssertRevisionImmutable(revision, revision); err != nil {
		t.Fatalf("same revision should compare equal: %v", err)
	}
	changed := revision
	changed.Content = "new source text"
	if !errors.Is(AssertRevisionImmutable(revision, changed), ErrRevisionImmutable) {
		t.Fatal("content overwrite must be rejected")
	}
	changed = revision
	changed.ContentHash = "hash-2"
	if !errors.Is(AssertRevisionImmutable(revision, changed), ErrRevisionImmutable) {
		t.Fatal("content hash overwrite must be rejected")
	}
}

func TestChunkEmbeddingIsOptionalButFixedDimension(t *testing.T) {
	chunk := KnowledgeChunk{ID: "chunk-1", WorkspaceID: "workspace-a", RevisionID: "revision-1", Text: "text"}
	if err := chunk.Validate(); err != nil {
		t.Fatalf("nil embedding should permit FTS fallback: %v", err)
	}
	chunk.Embedding = make([]float32, EmbeddingDimensions-1)
	if !errors.Is(chunk.Validate(), ErrInvalidInput) {
		t.Fatal("wrong embedding dimension must fail")
	}
	chunk.Embedding = make([]float32, EmbeddingDimensions)
	if err := chunk.Validate(); err != nil {
		t.Fatalf("384-dimensional embedding should pass: %v", err)
	}
}

func TestServicePassesValidatedScopeToRepository(t *testing.T) {
	fake := &fakeRepository{}
	service, err := NewService(fake)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := NewWorkspaceScope("workspace-a")
	if err != nil {
		t.Fatal(err)
	}
	source := KnowledgeSource{ID: "source-1", WorkspaceID: scope.ID, Kind: "manual", Name: "Project notes"}
	if err := service.CreateSource(context.Background(), scope, source); err != nil {
		t.Fatal(err)
	}
	if fake.called != "source" || fake.lastScope != scope {
		t.Fatalf("repository received wrong scope/call: call=%q scope=%+v", fake.called, fake.lastScope)
	}
	if err := service.CreateClaimBundle(context.Background(), scope, validClaim(), []ClaimEvidence{validEvidence()}); err != nil {
		t.Fatal(err)
	}
	if fake.called != "claim" || fake.lastScope != scope {
		t.Fatalf("claim repository received wrong scope/call: call=%q scope=%+v", fake.called, fake.lastScope)
	}
}
