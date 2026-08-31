package knowledge

import (
	"math"
	"strings"
	"time"
)

// EmbeddingDimensions is the locked local multilingual-e5-small vector size.
const EmbeddingDimensions = 384

type ClaimCertainty string

const (
	ClaimCanonical ClaimCertainty = "canonical"
	ClaimInferred  ClaimCertainty = "inferred"
	ClaimUnknown   ClaimCertainty = "unknown"
)

type ClaimFreshness string

const (
	ClaimCurrent          ClaimFreshness = "current"
	ClaimStale            ClaimFreshness = "stale"
	ClaimFreshnessUnknown ClaimFreshness = "unknown"
)

type EvidenceFreshness string

const (
	EvidenceCurrent EvidenceFreshness = "current"
	EvidenceStale   EvidenceFreshness = "stale"
	EvidenceUnknown EvidenceFreshness = "unknown"
)

// WorkspaceScope is required by every repository operation. The scope is
// intentionally a value object so callers cannot accidentally omit it.
type WorkspaceScope struct {
	ID string
}

func NewWorkspaceScope(id string) (WorkspaceScope, error) {
	scope := WorkspaceScope{ID: id}
	if err := scope.Validate(); err != nil {
		return WorkspaceScope{}, err
	}
	return scope, nil
}

func (s WorkspaceScope) Validate() error {
	return requireIdentifier("workspaceId", s.ID)
}

type KnowledgeSource struct {
	ID          string
	WorkspaceID string
	Kind        string
	Name        string
	URI         string
	Metadata    map[string]any
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func (s KnowledgeSource) Validate() error {
	if err := requireIdentifier("id", s.ID); err != nil {
		return err
	}
	if err := requireIdentifier("workspaceId", s.WorkspaceID); err != nil {
		return err
	}
	if err := requireText("kind", s.Kind); err != nil {
		return err
	}
	if err := requireText("name", s.Name); err != nil {
		return err
	}
	if s.Version < 0 {
		return invalidField("version", "must not be negative")
	}
	return validateTimestampOrder(s.CreatedAt, s.UpdatedAt)
}

type SourceItem struct {
	ID                string
	WorkspaceID       string
	SourceID          string
	ExternalID        string
	Title             string
	URI               string
	MIMEType          string
	CurrentRevisionID string
	Version           int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

func (i SourceItem) Validate() error {
	if err := requireIdentifier("id", i.ID); err != nil {
		return err
	}
	if err := requireIdentifier("workspaceId", i.WorkspaceID); err != nil {
		return err
	}
	if err := requireIdentifier("sourceId", i.SourceID); err != nil {
		return err
	}
	if err := requireIdentifier("externalId", i.ExternalID); err != nil {
		return err
	}
	if i.CurrentRevisionID != "" {
		if err := requireIdentifier("currentRevisionId", i.CurrentRevisionID); err != nil {
			return err
		}
	}
	if i.Version < 0 {
		return invalidField("version", "must not be negative")
	}
	return validateTimestampOrder(i.CreatedAt, i.UpdatedAt)
}

// SourceRevision is append-only. There is deliberately no update method in
// RevisionRepository; a changed source item must create a new revision.
type SourceRevision struct {
	ID           string
	WorkspaceID  string
	SourceItemID string
	RevisionKey  string
	ContentHash  string
	ContentType  string
	SourceURI    string
	Content      string
	ModifiedAt   time.Time
	IngestedAt   time.Time
}

func (r SourceRevision) Validate() error {
	if err := requireIdentifier("id", r.ID); err != nil {
		return err
	}
	if err := requireIdentifier("workspaceId", r.WorkspaceID); err != nil {
		return err
	}
	if err := requireIdentifier("sourceItemId", r.SourceItemID); err != nil {
		return err
	}
	if err := requireIdentifier("revisionKey", r.RevisionKey); err != nil {
		return err
	}
	if err := requireIdentifier("contentHash", r.ContentHash); err != nil {
		return err
	}
	if err := requireText("content", r.Content); err != nil {
		return err
	}
	return nil
}

// AssertRevisionImmutable compares all persisted revision fields. It is
// useful for repository adapters and tests that want to fail closed on an
// attempted overwrite.
func AssertRevisionImmutable(before, after SourceRevision) error {
	if before.ID != after.ID {
		return nil
	}
	if before != after {
		return ErrRevisionImmutable
	}
	return nil
}

type KnowledgeChunk struct {
	ID          string
	WorkspaceID string
	RevisionID  string
	Ordinal     int
	Text        string
	TokenCount  int
	Embedding   []float32
	CreatedAt   time.Time
}

func (c KnowledgeChunk) Validate() error {
	if err := requireIdentifier("id", c.ID); err != nil {
		return err
	}
	if err := requireIdentifier("workspaceId", c.WorkspaceID); err != nil {
		return err
	}
	if err := requireIdentifier("revisionId", c.RevisionID); err != nil {
		return err
	}
	if err := requireText("text", c.Text); err != nil {
		return err
	}
	if c.Ordinal < 0 {
		return invalidField("ordinal", "must not be negative")
	}
	if c.TokenCount < 0 {
		return invalidField("tokenCount", "must not be negative")
	}
	if len(c.Embedding) != 0 && len(c.Embedding) != EmbeddingDimensions {
		return invalidField("embedding", "must have exactly 384 dimensions when provided")
	}
	for index, value := range c.Embedding {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return invalidField("embedding", "must not contain NaN or infinity")
		}
		if index >= EmbeddingDimensions {
			break
		}
	}
	return nil
}

type Topic struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	ParentID    string
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func (t Topic) Validate() error {
	if err := requireIdentifier("id", t.ID); err != nil {
		return err
	}
	if err := requireIdentifier("workspaceId", t.WorkspaceID); err != nil {
		return err
	}
	if err := requireIdentifier("name", t.Name); err != nil {
		return err
	}
	if t.ParentID != "" {
		if err := requireIdentifier("parentId", t.ParentID); err != nil {
			return err
		}
	}
	if t.Version < 0 {
		return invalidField("version", "must not be negative")
	}
	return validateTimestampOrder(t.CreatedAt, t.UpdatedAt)
}

type KnowledgeClaim struct {
	ID          string
	WorkspaceID string
	TopicID     string
	Statement   string
	Certainty   ClaimCertainty
	Freshness   ClaimFreshness
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func (c KnowledgeClaim) Validate() error {
	if err := requireIdentifier("id", c.ID); err != nil {
		return err
	}
	if err := requireIdentifier("workspaceId", c.WorkspaceID); err != nil {
		return err
	}
	if c.TopicID != "" {
		if err := requireIdentifier("topicId", c.TopicID); err != nil {
			return err
		}
	}
	if err := requireText("statement", c.Statement); err != nil {
		return err
	}
	if !validClaimCertainty(c.Certainty) {
		return invalidField("certainty", "must be canonical, inferred, or unknown")
	}
	if !validClaimFreshness(c.Freshness) {
		return invalidField("freshness", "must be current, stale, or unknown")
	}
	if c.Version < 0 {
		return invalidField("version", "must not be negative")
	}
	return validateTimestampOrder(c.CreatedAt, c.UpdatedAt)
}

type ClaimEvidence struct {
	ID               string
	WorkspaceID      string
	ClaimID          string
	SourceRevisionID string
	ChunkID          string
	Locator          string
	Quote            string
	Freshness        EvidenceFreshness
	CreatedAt        time.Time
}

// SourceDetail is the bounded read model used by the workspace UI and
// adapters that need to explain where a source came from. Revisions and
// chunks remain immutable; this type only groups them for inspection.
type SourceDetail struct {
	Source    KnowledgeSource
	Items     []SourceItem
	Revisions []SourceRevision
	Chunks    []KnowledgeChunk
	Evidence  []ClaimEvidence
}

func (e ClaimEvidence) Validate() error {
	if err := requireIdentifier("id", e.ID); err != nil {
		return err
	}
	if err := requireIdentifier("workspaceId", e.WorkspaceID); err != nil {
		return err
	}
	if err := requireIdentifier("claimId", e.ClaimID); err != nil {
		return err
	}
	if err := requireIdentifier("sourceRevisionId", e.SourceRevisionID); err != nil {
		return err
	}
	if e.ChunkID != "" {
		if err := requireIdentifier("chunkId", e.ChunkID); err != nil {
			return err
		}
	}
	if err := requireText("quote", e.Quote); err != nil {
		return err
	}
	if !validEvidenceFreshness(e.Freshness) {
		return invalidField("freshness", "must be current, stale, or unknown")
	}
	return nil
}

func validateTimestampOrder(createdAt, updatedAt time.Time) error {
	if !createdAt.IsZero() && !updatedAt.IsZero() && updatedAt.Before(createdAt) {
		return invalidField("updatedAt", "must not be before createdAt")
	}
	return nil
}

func validClaimCertainty(value ClaimCertainty) bool {
	return value == ClaimCanonical || value == ClaimInferred || value == ClaimUnknown
}

func validClaimFreshness(value ClaimFreshness) bool {
	return value == ClaimCurrent || value == ClaimStale || value == ClaimFreshnessUnknown
}

func validEvidenceFreshness(value EvidenceFreshness) bool {
	return value == EvidenceCurrent || value == EvidenceStale || value == EvidenceUnknown
}

// ValidateClaimBundle enforces the grounding boundary. Canonical claims need
// at least one evidence record; a claim with no evidence must remain inferred
// or unknown. A current claim cannot hide stale/unknown supporting evidence.
func ValidateClaimBundle(claim KnowledgeClaim, evidence []ClaimEvidence) error {
	if err := claim.Validate(); err != nil {
		return err
	}
	if claim.Certainty == ClaimCanonical && len(evidence) == 0 {
		return ErrEvidenceRequired
	}
	seen := make(map[string]struct{}, len(evidence))
	currentEvidence := 0
	for _, item := range evidence {
		if err := item.Validate(); err != nil {
			return err
		}
		if item.WorkspaceID != claim.WorkspaceID {
			return ErrWorkspaceMismatch
		}
		if item.ClaimID != claim.ID {
			return invalidField("evidence.claimId", "must match claim id")
		}
		if _, exists := seen[item.ID]; exists {
			return ErrDuplicateEvidence
		}
		seen[item.ID] = struct{}{}
		switch item.Freshness {
		case EvidenceCurrent:
			currentEvidence++
		case EvidenceStale, EvidenceUnknown:
			if claim.Certainty == ClaimCanonical && claim.Freshness == ClaimCurrent {
				return ErrStaleEvidence
			}
		}
	}
	if claim.Certainty == ClaimCanonical && currentEvidence == 0 {
		return ErrCanonicalEvidenceStale
	}
	return nil
}

// NormalizeForComparison trims only human-facing text. IDs and workspace
// identifiers are intentionally not normalized; validation rejects ambiguous
// whitespace so persistence keys remain deterministic.
func (c KnowledgeClaim) NormalizeForComparison() KnowledgeClaim {
	c.Statement = strings.TrimSpace(c.Statement)
	return c
}
