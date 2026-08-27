package knowledge

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

func requireInvalidInput(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func TestValidationErrorsAreTypedAndDoNotExposeUnexpectedData(t *testing.T) {
	err := invalidField("field", "must be present")
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error %T is not a ValidationError", err)
	}
	if got, want := validationErr.Error(), "knowledge input is invalid: field: must be present"; got != want {
		t.Fatalf("validation error = %q, want %q", got, want)
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatal("validation errors must unwrap to ErrInvalidInput")
	}
	if got, want := (&ValidationError{Reason: "bad value"}).Error(), "knowledge input is invalid: bad value"; got != want {
		t.Fatalf("field-less validation error = %q, want %q", got, want)
	}
	var nilValidationErr *ValidationError
	if got, want := nilValidationErr.Error(), ErrInvalidInput.Error(); got != want {
		t.Fatalf("nil validation error = %q, want %q", got, want)
	}
}

func TestIdentifierAndTextValidationBoundaries(t *testing.T) {
	for _, value := range []string{"", " ", "\t"} {
		requireInvalidInput(t, requireIdentifier("id", value))
	}
	for _, value := range []string{" id", "id "} {
		requireInvalidInput(t, requireIdentifier("id", value))
	}
	if err := requireIdentifier("id", "id"); err != nil {
		t.Fatalf("valid identifier rejected: %v", err)
	}
	for _, value := range []string{"", " ", "\t"} {
		requireInvalidInput(t, requireText("text", value))
	}
	if err := requireText("text", " text "); err != nil {
		t.Fatalf("human-facing text with surrounding whitespace rejected: %v", err)
	}
}

func TestKnowledgeSourceValidationRejectsInvalidFields(t *testing.T) {
	created := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	valid := KnowledgeSource{
		ID:          "source-1",
		WorkspaceID: "workspace-a",
		Kind:        "manual",
		Name:        "Project notes",
		Version:     1,
		CreatedAt:   created,
		UpdatedAt:   created,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid source rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*KnowledgeSource)
	}{
		{"id", func(value *KnowledgeSource) { value.ID = " " }},
		{"workspace", func(value *KnowledgeSource) { value.WorkspaceID = "" }},
		{"kind", func(value *KnowledgeSource) { value.Kind = " " }},
		{"name", func(value *KnowledgeSource) { value.Name = "" }},
		{"version", func(value *KnowledgeSource) { value.Version = -1 }},
		{"timestamps", func(value *KnowledgeSource) { value.UpdatedAt = value.CreatedAt.Add(-time.Second) }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			value := valid
			testCase.mutate(&value)
			requireInvalidInput(t, value.Validate())
		})
	}
}

func TestSourceItemValidationRejectsInvalidFields(t *testing.T) {
	created := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	valid := SourceItem{
		ID:          "item-1",
		WorkspaceID: "workspace-a",
		SourceID:    "source-1",
		ExternalID:  "external-1",
		Version:     1,
		CreatedAt:   created,
		UpdatedAt:   created,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid source item rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*SourceItem)
	}{
		{"id", func(value *SourceItem) { value.ID = "" }},
		{"workspace", func(value *SourceItem) { value.WorkspaceID = " " }},
		{"source", func(value *SourceItem) { value.SourceID = "" }},
		{"external", func(value *SourceItem) { value.ExternalID = "" }},
		{"current revision", func(value *SourceItem) { value.CurrentRevisionID = " revision-1" }},
		{"version", func(value *SourceItem) { value.Version = -1 }},
		{"timestamps", func(value *SourceItem) { value.UpdatedAt = value.CreatedAt.Add(-time.Second) }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			value := valid
			testCase.mutate(&value)
			requireInvalidInput(t, value.Validate())
		})
	}
}

func TestSourceRevisionValidationRejectsInvalidFields(t *testing.T) {
	valid := SourceRevision{
		ID:           "revision-1",
		WorkspaceID:  "workspace-a",
		SourceItemID: "item-1",
		RevisionKey:  "provider:revision-1",
		ContentHash:  "hash-1",
		Content:      "immutable source text",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid source revision rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*SourceRevision)
	}{
		{"id", func(value *SourceRevision) { value.ID = "" }},
		{"workspace", func(value *SourceRevision) { value.WorkspaceID = "" }},
		{"source item", func(value *SourceRevision) { value.SourceItemID = " " }},
		{"revision key", func(value *SourceRevision) { value.RevisionKey = "" }},
		{"content hash", func(value *SourceRevision) { value.ContentHash = " " }},
		{"content", func(value *SourceRevision) { value.Content = "\t" }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			value := valid
			testCase.mutate(&value)
			requireInvalidInput(t, value.Validate())
		})
	}
}

func TestKnowledgeChunkValidationCoversEmbeddingAndNumericEdges(t *testing.T) {
	valid := KnowledgeChunk{
		ID:          "chunk-1",
		WorkspaceID: "workspace-a",
		RevisionID:  "revision-1",
		Text:        "source text",
		TokenCount:  1,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid chunk rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*KnowledgeChunk)
	}{
		{"id", func(value *KnowledgeChunk) { value.ID = "" }},
		{"workspace", func(value *KnowledgeChunk) { value.WorkspaceID = " " }},
		{"revision", func(value *KnowledgeChunk) { value.RevisionID = "" }},
		{"text", func(value *KnowledgeChunk) { value.Text = " " }},
		{"ordinal", func(value *KnowledgeChunk) { value.Ordinal = -1 }},
		{"token count", func(value *KnowledgeChunk) { value.TokenCount = -1 }},
		{"dimension", func(value *KnowledgeChunk) { value.Embedding = make([]float32, EmbeddingDimensions-1) }},
		{"nan", func(value *KnowledgeChunk) { value.Embedding = []float32{float32(math.NaN())} }},
		{"positive infinity", func(value *KnowledgeChunk) { value.Embedding = []float32{float32(math.Inf(1))} }},
		{"negative infinity", func(value *KnowledgeChunk) { value.Embedding = []float32{float32(math.Inf(-1))} }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			value := valid
			testCase.mutate(&value)
			requireInvalidInput(t, value.Validate())
		})
	}
	valid.Embedding = make([]float32, EmbeddingDimensions)
	if err := valid.Validate(); err != nil {
		t.Fatalf("384-dimensional embedding rejected: %v", err)
	}
}

func TestTopicValidationRejectsInvalidFields(t *testing.T) {
	created := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	valid := Topic{
		ID:          "topic-1",
		WorkspaceID: "workspace-a",
		Name:        "Persistence",
		ParentID:    "topic-parent",
		Version:     1,
		CreatedAt:   created,
		UpdatedAt:   created,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid topic rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*Topic)
	}{
		{"id", func(value *Topic) { value.ID = "" }},
		{"workspace", func(value *Topic) { value.WorkspaceID = " " }},
		{"name", func(value *Topic) { value.Name = "" }},
		{"parent", func(value *Topic) { value.ParentID = " parent" }},
		{"version", func(value *Topic) { value.Version = -1 }},
		{"timestamps", func(value *Topic) { value.UpdatedAt = value.CreatedAt.Add(-time.Second) }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			value := valid
			testCase.mutate(&value)
			requireInvalidInput(t, value.Validate())
		})
	}
	valid.ParentID = ""
	if err := valid.Validate(); err != nil {
		t.Fatalf("optional parent rejected when omitted: %v", err)
	}
}

func TestKnowledgeClaimAndEvidenceValidationRejectInvalidFields(t *testing.T) {
	created := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	claim := validClaim()
	claim.TopicID = "topic-1"
	claim.Version = 1
	claim.CreatedAt = created
	claim.UpdatedAt = created
	if err := claim.Validate(); err != nil {
		t.Fatalf("valid claim rejected: %v", err)
	}
	claimCases := []struct {
		name   string
		mutate func(*KnowledgeClaim)
	}{
		{"id", func(value *KnowledgeClaim) { value.ID = "" }},
		{"workspace", func(value *KnowledgeClaim) { value.WorkspaceID = " " }},
		{"topic", func(value *KnowledgeClaim) { value.TopicID = " topic-1" }},
		{"statement", func(value *KnowledgeClaim) { value.Statement = " " }},
		{"certainty", func(value *KnowledgeClaim) { value.Certainty = "unsupported" }},
		{"freshness", func(value *KnowledgeClaim) { value.Freshness = "unsupported" }},
		{"version", func(value *KnowledgeClaim) { value.Version = -1 }},
		{"timestamps", func(value *KnowledgeClaim) { value.UpdatedAt = value.CreatedAt.Add(-time.Second) }},
	}
	for _, testCase := range claimCases {
		t.Run("claim "+testCase.name, func(t *testing.T) {
			value := claim
			testCase.mutate(&value)
			requireInvalidInput(t, value.Validate())
		})
	}
	claim.TopicID = ""
	if err := claim.Validate(); err != nil {
		t.Fatalf("optional topic rejected when omitted: %v", err)
	}

	evidence := validEvidence()
	evidence.ChunkID = "chunk-1"
	evidence.CreatedAt = created
	if err := evidence.Validate(); err != nil {
		t.Fatalf("valid evidence rejected: %v", err)
	}
	evidenceCases := []struct {
		name   string
		mutate func(*ClaimEvidence)
	}{
		{"id", func(value *ClaimEvidence) { value.ID = "" }},
		{"workspace", func(value *ClaimEvidence) { value.WorkspaceID = " " }},
		{"claim", func(value *ClaimEvidence) { value.ClaimID = "" }},
		{"source revision", func(value *ClaimEvidence) { value.SourceRevisionID = "" }},
		{"chunk", func(value *ClaimEvidence) { value.ChunkID = " chunk-1" }},
		{"quote", func(value *ClaimEvidence) { value.Quote = "\t" }},
		{"freshness", func(value *ClaimEvidence) { value.Freshness = "unsupported" }},
	}
	for _, testCase := range evidenceCases {
		t.Run("evidence "+testCase.name, func(t *testing.T) {
			value := evidence
			testCase.mutate(&value)
			requireInvalidInput(t, value.Validate())
		})
	}
	evidence.ChunkID = ""
	if err := evidence.Validate(); err != nil {
		t.Fatalf("optional chunk rejected when omitted: %v", err)
	}
}

func TestValidateClaimBundleMatchesGroundingAndFreshnessRules(t *testing.T) {
	claim := validClaim()
	current := validEvidence()

	invalidClaim := claim
	invalidClaim.Statement = ""
	requireInvalidInput(t, ValidateClaimBundle(invalidClaim, nil))

	wrongWorkspace := current
	wrongWorkspace.WorkspaceID = "workspace-b"
	if !errors.Is(ValidateClaimBundle(claim, []ClaimEvidence{wrongWorkspace}), ErrWorkspaceMismatch) {
		t.Fatal("evidence from another workspace must be rejected")
	}
	invalidEvidence := current
	invalidEvidence.Quote = ""
	requireInvalidInput(t, ValidateClaimBundle(claim, []ClaimEvidence{invalidEvidence}))

	staleCanonical := claim
	staleCanonical.Freshness = ClaimStale
	staleEvidence := current
	staleEvidence.Freshness = EvidenceStale
	if !errors.Is(ValidateClaimBundle(staleCanonical, []ClaimEvidence{staleEvidence}), ErrCanonicalEvidenceStale) {
		t.Fatal("canonical claim without current evidence must be rejected")
	}
	unknownEvidence := current
	unknownEvidence.Freshness = EvidenceUnknown
	if !errors.Is(ValidateClaimBundle(staleCanonical, []ClaimEvidence{unknownEvidence}), ErrCanonicalEvidenceStale) {
		t.Fatal("canonical claim with unknown-only evidence must be rejected")
	}

	withMixedEvidence := claim
	second := current
	second.ID = "evidence-2"
	second.Freshness = EvidenceStale
	if !errors.Is(ValidateClaimBundle(withMixedEvidence, []ClaimEvidence{current, second}), ErrStaleEvidence) {
		t.Fatal("current canonical claim must reject mixed stale evidence")
	}
	if err := ValidateClaimBundle(withMixedEvidence, []ClaimEvidence{current}); err != nil {
		t.Fatalf("current canonical claim with current evidence rejected: %v", err)
	}

	for _, certainty := range []ClaimCertainty{ClaimInferred, ClaimUnknown} {
		nonCanonical := claim
		nonCanonical.Certainty = certainty
		stale := current
		stale.Freshness = EvidenceStale
		if err := ValidateClaimBundle(nonCanonical, []ClaimEvidence{stale}); err != nil {
			t.Fatalf("%s claim should allow stale evidence: %v", certainty, err)
		}
	}
}

func TestAssertRevisionImmutableChecksEveryPersistedField(t *testing.T) {
	base := SourceRevision{
		ID:           "revision-1",
		WorkspaceID:  "workspace-a",
		SourceItemID: "item-1",
		RevisionKey:  "provider:revision-1",
		ContentHash:  "hash-1",
		ContentType:  "text/plain",
		SourceURI:    "https://example.test/source",
		Content:      "immutable source text",
		ModifiedAt:   time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC),
		IngestedAt:   time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC),
	}
	if err := AssertRevisionImmutable(base, base); err != nil {
		t.Fatalf("identical revision rejected: %v", err)
	}
	differentID := base
	differentID.ID = "revision-2"
	if err := AssertRevisionImmutable(base, differentID); err != nil {
		t.Fatalf("different revision IDs should represent an append: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*SourceRevision)
	}{
		{"workspace", func(value *SourceRevision) { value.WorkspaceID = "workspace-b" }},
		{"source item", func(value *SourceRevision) { value.SourceItemID = "item-2" }},
		{"revision key", func(value *SourceRevision) { value.RevisionKey = "provider:revision-2" }},
		{"content hash", func(value *SourceRevision) { value.ContentHash = "hash-2" }},
		{"content type", func(value *SourceRevision) { value.ContentType = "text/markdown" }},
		{"source URI", func(value *SourceRevision) { value.SourceURI = "https://example.test/other" }},
		{"content", func(value *SourceRevision) { value.Content = "changed" }},
		{"modified at", func(value *SourceRevision) { value.ModifiedAt = value.ModifiedAt.Add(time.Minute) }},
		{"ingested at", func(value *SourceRevision) { value.IngestedAt = value.IngestedAt.Add(time.Minute) }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			changed := base
			testCase.mutate(&changed)
			if !errors.Is(AssertRevisionImmutable(base, changed), ErrRevisionImmutable) {
				t.Fatalf("field %s can be overwritten", testCase.name)
			}
		})
	}
}

func TestNormalizeForComparisonOnlyTrimsHumanText(t *testing.T) {
	claim := validClaim()
	claim.Statement = "  The deployment uses PostgreSQL.  "
	normalized := claim.NormalizeForComparison()
	if normalized.Statement != "The deployment uses PostgreSQL." {
		t.Fatalf("normalized statement = %q", normalized.Statement)
	}
	if normalized.ID != claim.ID || normalized.WorkspaceID != claim.WorkspaceID {
		t.Fatal("normalization must not change persistence identifiers")
	}
}

func TestNewServiceRejectsNilRepository(t *testing.T) {
	if _, err := NewService(nil); !errors.Is(err, ErrNilRepository) {
		t.Fatalf("NewService(nil) error = %v, want ErrNilRepository", err)
	}
}

func TestServiceValidatesAndRoutesEveryOperation(t *testing.T) {
	fake := &fakeRepository{}
	service, err := NewService(fake)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := NewWorkspaceScope("workspace-a")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	operations := []struct {
		name string
		call func() error
		want string
	}{
		{
			name: "source",
			call: func() error {
				return service.CreateSource(ctx, scope, KnowledgeSource{
					ID: "source-2", WorkspaceID: scope.ID, Kind: "manual", Name: "Notes",
				})
			},
			want: "source",
		},
		{
			name: "source item",
			call: func() error {
				return service.CreateSourceItem(ctx, scope, SourceItem{
					ID: "item-2", WorkspaceID: scope.ID, SourceID: "source-2", ExternalID: "external-2",
				})
			},
			want: "source-item",
		},
		{
			name: "revision",
			call: func() error {
				return service.AppendRevision(ctx, scope, SourceRevision{
					ID: "revision-2", WorkspaceID: scope.ID, SourceItemID: "item-2",
					RevisionKey: "provider:revision-2", ContentHash: "hash-2", Content: "text",
				})
			},
			want: "revision",
		},
		{
			name: "chunk",
			call: func() error {
				return service.CreateChunk(ctx, scope, KnowledgeChunk{
					ID: "chunk-2", WorkspaceID: scope.ID, RevisionID: "revision-2", Text: "text",
				})
			},
			want: "chunk",
		},
		{
			name: "topic",
			call: func() error {
				return service.CreateTopic(ctx, scope, Topic{
					ID: "topic-2", WorkspaceID: scope.ID, Name: "Persistence",
				})
			},
			want: "topic",
		},
		{
			name: "claim",
			call: func() error {
				claim := validClaim()
				claim.ID = "claim-2"
				evidence := validEvidence()
				evidence.ID = "evidence-2"
				evidence.ClaimID = claim.ID
				return service.CreateClaimBundle(ctx, scope, claim, []ClaimEvidence{evidence})
			},
			want: "claim",
		},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			if err := operation.call(); err != nil {
				t.Fatalf("operation rejected: %v", err)
			}
			if fake.called != operation.want || fake.lastScope != scope {
				t.Fatalf("repository call = %q scope = %+v, want %q %+v", fake.called, fake.lastScope, operation.want, scope)
			}
		})
	}
}

func TestServiceRejectsInvalidScopeAndEntityBeforeRepository(t *testing.T) {
	fake := &fakeRepository{}
	service, err := NewService(fake)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.CreateSource(context.Background(), WorkspaceScope{}, KnowledgeSource{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid scope error = %v, want ErrInvalidInput", err)
	}
	if fake.called != "" {
		t.Fatalf("repository called for invalid scope: %q", fake.called)
	}

	scope, err := NewWorkspaceScope("workspace-a")
	if err != nil {
		t.Fatal(err)
	}
	invalidSource := KnowledgeSource{ID: "source-3", WorkspaceID: scope.ID, Kind: "manual"}
	if err := service.CreateSource(context.Background(), scope, invalidSource); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid entity error = %v, want ErrInvalidInput", err)
	}
	if fake.called != "" {
		t.Fatalf("repository called for invalid entity: %q", fake.called)
	}

	claim := validClaim()
	claim.ID = "claim-3"
	claim.WorkspaceID = "workspace-b"
	if err := service.CreateClaimBundle(context.Background(), scope, claim, nil); !errors.Is(err, ErrWorkspaceMismatch) {
		t.Fatalf("cross-workspace claim error = %v, want ErrWorkspaceMismatch", err)
	}
	if fake.called != "" {
		t.Fatalf("repository called for cross-workspace claim: %q", fake.called)
	}

	if err := service.CreateClaimBundle(context.Background(), scope, validClaim(), nil); !errors.Is(err, ErrEvidenceRequired) {
		t.Fatalf("invalid claim bundle error = %v, want ErrEvidenceRequired", err)
	}
	if fake.called != "" {
		t.Fatalf("repository called for invalid claim bundle: %q", fake.called)
	}
}
