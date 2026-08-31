package connectors

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	SafeWriteOperationCreateIssue SafeWriteOperation = "github.issue.create"
	SafeWriteOperationAddComment  SafeWriteOperation = "github.issue.comment"
	SafeWriteOperationSetLabels   SafeWriteOperation = "github.issue.labels"
	SafeWriteOperationEntityTrash SafeWriteOperation = "work.entity.trash"
	SafeWriteOperationEntityPurge SafeWriteOperation = "work.entity.purge"
	SafeWriteChallengeTTL                            = 5 * time.Minute
	SafeWriteReceiptPending                          = "pending"
	SafeWriteReceiptAccepted                         = "accepted"
)

type SafeWriteOperation string

// SafeWriteTarget is the canonical action payload used to bind a challenge.
// It intentionally contains no credentials or provider response body.
type SafeWriteTarget struct {
	Operation        SafeWriteOperation
	EntityType       string
	EntityID         string
	ExpectedVersion  int64
	Repository       string
	Issue            int64
	Title            string
	Body             string
	Labels           []string
	ExpectedRevision string
}

func (t SafeWriteTarget) Validate() error {
	if t.Operation == SafeWriteOperationEntityTrash || t.Operation == SafeWriteOperationEntityPurge {
		if !validSafeWriteIdentifier(t.EntityType, 32) || (t.EntityType != "project" && t.EntityType != "task" && t.EntityType != "decision") {
			return ErrInvalidWriteTarget
		}
		if !validSafeWriteIdentifier(t.EntityID, 256) || t.ExpectedVersion < 1 {
			return ErrInvalidWriteTarget
		}
		if strings.TrimSpace(t.Repository) != "" || t.Issue != 0 || strings.TrimSpace(t.Title) != "" || strings.TrimSpace(t.Body) != "" || len(t.Labels) != 0 || strings.TrimSpace(t.ExpectedRevision) != "" {
			return ErrInvalidWriteTarget
		}
		return nil
	}
	if t.EntityType != "" || t.EntityID != "" || t.ExpectedVersion != 0 {
		return ErrInvalidWriteTarget
	}
	if normalizeRepository(t.Repository) == "" {
		return ErrInvalidRepository
	}
	switch t.Operation {
	case SafeWriteOperationCreateIssue:
		if t.Issue != 0 || strings.TrimSpace(t.Title) == "" || len(t.Labels) != 0 {
			return ErrInvalidWriteTarget
		}
	case SafeWriteOperationAddComment:
		if t.Issue < 1 || strings.TrimSpace(t.Body) == "" || strings.TrimSpace(t.Title) != "" || len(t.Labels) != 0 {
			return ErrInvalidWriteTarget
		}
	case SafeWriteOperationSetLabels:
		if t.Issue < 1 || strings.TrimSpace(t.Title) != "" || strings.TrimSpace(t.Body) != "" {
			return ErrInvalidWriteTarget
		}
		if err := validateLabels(t.Labels); err != nil {
			return err
		}
	default:
		return ErrUnsupportedWrite
	}
	return nil
}

func (t SafeWriteTarget) normalizedLabels() []string {
	labels := append([]string(nil), t.Labels...)
	for index := range labels {
		labels[index] = strings.TrimSpace(labels[index])
	}
	sort.Strings(labels)
	return labels
}

func (t SafeWriteTarget) TargetTypeAndID() (string, string) {
	if t.Operation == SafeWriteOperationEntityTrash || t.Operation == SafeWriteOperationEntityPurge {
		return "work_" + strings.TrimSpace(t.EntityType), strings.TrimSpace(t.EntityID)
	}
	if t.Operation == SafeWriteOperationCreateIssue {
		return "github_repository", normalizeRepository(t.Repository)
	}
	return "github_issue", fmt.Sprintf("%s#%d", normalizeRepository(t.Repository), t.Issue)
}

// ActionHash is deterministic for the same mutation regardless of label
// ordering. It excludes user/challenge/idempotency metadata by design.
func (t SafeWriteTarget) ActionHash() (string, error) {
	if err := t.Validate(); err != nil {
		return "", err
	}
	canonical := struct {
		Operation        SafeWriteOperation `json:"operation"`
		EntityType       string             `json:"entityType,omitempty"`
		EntityID         string             `json:"entityId,omitempty"`
		ExpectedVersion  int64              `json:"expectedVersion,omitempty"`
		Repository       string             `json:"repository"`
		Issue            int64              `json:"issue,omitempty"`
		Title            string             `json:"title,omitempty"`
		Body             string             `json:"body,omitempty"`
		Labels           []string           `json:"labels,omitempty"`
		ExpectedRevision string             `json:"expectedRevision,omitempty"`
	}{
		Operation:        t.Operation,
		EntityType:       strings.TrimSpace(t.EntityType),
		EntityID:         strings.TrimSpace(t.EntityID),
		ExpectedVersion:  t.ExpectedVersion,
		Repository:       normalizeRepository(t.Repository),
		Issue:            t.Issue,
		Title:            t.Title,
		Body:             t.Body,
		Labels:           t.normalizedLabels(),
		ExpectedRevision: strings.TrimSpace(t.ExpectedRevision),
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("canonicalize safe write target: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

// ProviderForSafeWriteOperation maps a canonical action operation to the
// provider family that owns its receipt. The mapping is intentionally small;
// adding a new provider requires a new operation and explicit handler.
func ProviderForSafeWriteOperation(operation SafeWriteOperation) string {
	if operation == SafeWriteOperationEntityTrash || operation == SafeWriteOperationEntityPurge {
		return ProviderWork
	}
	return ProviderGitHub
}

func validSafeWriteIdentifier(value string, limit int) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && trimmed == value && len(trimmed) <= limit && strings.IndexFunc(trimmed, func(r rune) bool { return r < 0x20 || r == 0x7f || r == ' ' }) < 0
}

func (r CreateIssueRequest) Validate() error {
	target := r.Target()
	if err := target.Validate(); err != nil {
		return err
	}
	return r.Metadata.Validate()
}

func (r CreateIssueRequest) Target() SafeWriteTarget {
	return SafeWriteTarget{
		Operation:        SafeWriteOperationCreateIssue,
		Repository:       r.Repository,
		Title:            r.Title,
		Body:             r.Body,
		ExpectedRevision: r.Metadata.ExpectedRevision,
	}
}

func (r CommentIssueRequest) Validate() error {
	target := r.Target()
	if err := target.Validate(); err != nil {
		return err
	}
	return r.Metadata.Validate()
}

func (r CommentIssueRequest) Target() SafeWriteTarget {
	return SafeWriteTarget{
		Operation:        SafeWriteOperationAddComment,
		Repository:       r.Repository,
		Issue:            r.Issue,
		Body:             r.Body,
		ExpectedRevision: r.Metadata.ExpectedRevision,
	}
}

func (r LabelIssueRequest) Validate() error {
	target := r.Target()
	if err := target.Validate(); err != nil {
		return err
	}
	return r.Metadata.Validate()
}

func (r LabelIssueRequest) Target() SafeWriteTarget {
	return SafeWriteTarget{
		Operation:        SafeWriteOperationSetLabels,
		Repository:       r.Repository,
		Issue:            r.Issue,
		Labels:           append([]string(nil), r.Labels...),
		ExpectedRevision: r.Metadata.ExpectedRevision,
	}
}

// SafeWriteChallenge is issued by the application layer and consumed exactly
// once by a guarded writer. ExpiresAt should normally be CreatedAt + 5 min.
type SafeWriteChallenge struct {
	ID          string
	WorkspaceID string
	UserID      string
	TargetType  string
	TargetID    string
	ActionHash  string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	UsedAt      *time.Time
}

func NewSafeWriteChallenge(id, userID string, target SafeWriteTarget, now time.Time) (SafeWriteChallenge, error) {
	return NewWorkspaceSafeWriteChallenge("", id, userID, target, now)
}

// NewWorkspaceSafeWriteChallenge is the application-facing constructor. The
// legacy constructor remains available for low-level fixtures, while real
// application actions must bind the challenge to a workspace as well as a
// user and action hash.
func NewWorkspaceSafeWriteChallenge(workspaceID, id, userID string, target SafeWriteTarget, now time.Time) (SafeWriteChallenge, error) {
	hash, err := target.ActionHash()
	if err != nil {
		return SafeWriteChallenge{}, err
	}
	targetType, targetID := target.TargetTypeAndID()
	challenge := SafeWriteChallenge{
		ID:          strings.TrimSpace(id),
		WorkspaceID: strings.TrimSpace(workspaceID),
		UserID:      strings.TrimSpace(userID),
		TargetType:  targetType,
		TargetID:    targetID,
		ActionHash:  hash,
		CreatedAt:   now.UTC(),
		ExpiresAt:   now.UTC().Add(SafeWriteChallengeTTL),
	}
	if err := challenge.Validate(now); err != nil {
		return SafeWriteChallenge{}, err
	}
	return challenge, nil
}

func (c SafeWriteChallenge) Validate(now time.Time) error {
	if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.UserID) == "" || strings.TrimSpace(c.TargetType) == "" || strings.TrimSpace(c.TargetID) == "" || strings.TrimSpace(c.ActionHash) == "" {
		return ErrInvalidChallenge
	}
	if c.WorkspaceID != "" && strings.IndexFunc(c.WorkspaceID, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return ErrInvalidChallenge
	}
	if c.CreatedAt.IsZero() || c.ExpiresAt.IsZero() || !c.ExpiresAt.After(c.CreatedAt) || c.ExpiresAt.Sub(c.CreatedAt) > SafeWriteChallengeTTL {
		return ErrInvalidChallenge
	}
	if !c.ExpiresAt.After(now) {
		return ErrChallengeExpired
	}
	if c.UsedAt != nil {
		return ErrChallengeUsed
	}
	return nil
}

func (c SafeWriteChallenge) Confirmation(confirmedAt time.Time) SafeWriteMetadata {
	return SafeWriteMetadata{
		WorkspaceID: c.WorkspaceID,
		UserID:      c.UserID,
		ChallengeID: c.ID,
		ActionHash:  c.ActionHash,
		ConfirmedAt: confirmedAt.UTC(),
		ExpiresAt:   c.ExpiresAt.UTC(),
	}
}

type SafeWriteChallengeStore interface {
	Consume(ctx context.Context, metadata SafeWriteMetadata, target SafeWriteTarget, now time.Time) error
}

// SafeWriteChallengeIssuer and SafeWriteChallengeReader are deliberately
// separate from Consume. The application issues a preview first, then reads
// and validates the canonical challenge identity before confirming it.
type SafeWriteChallengeIssuer interface {
	Put(challenge SafeWriteChallenge) error
}

type SafeWriteChallengeReader interface {
	Get(ctx context.Context, workspaceID, userID, challengeID string) (SafeWriteChallenge, error)
}

func (m SafeWriteMetadata) ValidateConfirmation(now time.Time, target SafeWriteTarget) error {
	if strings.TrimSpace(m.UserID) == "" {
		return ErrMissingUserID
	}
	if strings.TrimSpace(m.ChallengeID) == "" || strings.TrimSpace(m.ActionHash) == "" {
		return ErrMissingChallenge
	}
	expectedHash, err := target.ActionHash()
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(m.ActionHash), expectedHash) {
		return ErrInvalidChallenge
	}
	if m.ConfirmedAt.IsZero() || m.ExpiresAt.IsZero() || m.ConfirmedAt.After(now) || !m.ExpiresAt.After(now) || m.ExpiresAt.Before(m.ConfirmedAt) || m.ExpiresAt.Sub(m.ConfirmedAt) > SafeWriteChallengeTTL {
		return ErrChallengeExpired
	}
	return nil
}

type SafeWriteReceipt struct {
	ID             string
	WorkspaceID    string
	Provider       string
	Operation      SafeWriteOperation
	UserID         string
	ChallengeID    string
	IdempotencyKey string
	ActionHash     string
	TargetType     string
	TargetID       string
	Status         string
	CreatedAt      time.Time
	Issue          GitHubIssue
	Comment        GitHubComment
	Labels         []GitHubLabel
}

type SafeWriteReceiptStore interface {
	Lookup(ctx context.Context, workspaceID, userID string, operation SafeWriteOperation, idempotencyKey string) (SafeWriteReceipt, bool, error)
	// Save remains available for low-level compatibility. New guarded writes
	// use SafeWriteReservationStore so a durable pending row exists before the
	// provider is called.
	Save(ctx context.Context, receipt SafeWriteReceipt) error
}

// SafeWriteReservationStore is the durable two-phase receipt boundary used by
// the production guarded writer. Reserve must be atomic on the idempotency
// identity and Complete must only transition the same pending reservation.
type SafeWriteReservationStore interface {
	Reserve(ctx context.Context, receipt SafeWriteReceipt) (SafeWriteReceipt, bool, error)
	Complete(ctx context.Context, receipt SafeWriteReceipt) error
}

type GitHubWriteOutcome struct {
	Receipt  SafeWriteReceipt
	Replayed bool
}

// GitHubSafeWriteService is the guarded facade over the low-level V1 writer.
// The facade validates target hash, consumes a one-time challenge, and stores
// a deterministic receipt before returning success.
type GitHubSafeWriteService struct {
	Writer     GitHubSafeWriter
	Challenges SafeWriteChallengeStore
	Receipts   SafeWriteReceiptStore
	Clock      func() time.Time
	mu         sync.Mutex
}

func NewGitHubSafeWriteService(writer GitHubSafeWriter, challenges SafeWriteChallengeStore, receipts SafeWriteReceiptStore) (*GitHubSafeWriteService, error) {
	if writer == nil || challenges == nil || receipts == nil {
		return nil, errors.New("GitHub writer, challenge store and receipt store are required")
	}
	return &GitHubSafeWriteService{Writer: writer, Challenges: challenges, Receipts: receipts, Clock: time.Now}, nil
}

func (s *GitHubSafeWriteService) CreateIssue(ctx context.Context, request CreateIssueRequest) (GitHubWriteOutcome, error) {
	if err := request.Validate(); err != nil {
		return GitHubWriteOutcome{}, err
	}
	target := request.Target()
	return s.execute(ctx, target, request.Metadata, func() (SafeWriteReceipt, error) {
		issue, err := s.Writer.CreateIssue(ctx, request)
		if err != nil {
			return SafeWriteReceipt{}, wrapConnectorFailure(ProviderGitHub, string(target.Operation), err)
		}
		if err := validateWrittenIssue(issue, target); err != nil {
			return SafeWriteReceipt{}, err
		}
		return SafeWriteReceipt{Issue: issue}, nil
	})
}

func (s *GitHubSafeWriteService) AddIssueComment(ctx context.Context, request CommentIssueRequest) (GitHubWriteOutcome, error) {
	if err := request.Validate(); err != nil {
		return GitHubWriteOutcome{}, err
	}
	target := request.Target()
	return s.execute(ctx, target, request.Metadata, func() (SafeWriteReceipt, error) {
		comment, err := s.Writer.AddIssueComment(ctx, request)
		if err != nil {
			return SafeWriteReceipt{}, wrapConnectorFailure(ProviderGitHub, string(target.Operation), err)
		}
		if err := validateWrittenComment(comment, target); err != nil {
			return SafeWriteReceipt{}, err
		}
		return SafeWriteReceipt{Comment: comment}, nil
	})
}

func (s *GitHubSafeWriteService) SetIssueLabels(ctx context.Context, request LabelIssueRequest) (GitHubWriteOutcome, error) {
	if err := request.Validate(); err != nil {
		return GitHubWriteOutcome{}, err
	}
	target := request.Target()
	return s.execute(ctx, target, request.Metadata, func() (SafeWriteReceipt, error) {
		labels, err := s.Writer.SetIssueLabels(ctx, request)
		if err != nil {
			return SafeWriteReceipt{}, wrapConnectorFailure(ProviderGitHub, string(target.Operation), err)
		}
		if err := validateWrittenLabels(labels, target); err != nil {
			return SafeWriteReceipt{}, err
		}
		return SafeWriteReceipt{Labels: append([]GitHubLabel(nil), labels...)}, nil
	})
}

func (s *GitHubSafeWriteService) execute(ctx context.Context, target SafeWriteTarget, metadata SafeWriteMetadata, invoke func() (SafeWriteReceipt, error)) (GitHubWriteOutcome, error) {
	// The process mutex is only a local optimization. The reservation store is
	// the durable multi-instance boundary and must be created before provider
	// I/O, so a lost response cannot cause a second external mutation on retry.
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := metadata.Validate(); err != nil {
		return GitHubWriteOutcome{}, err
	}
	if strings.TrimSpace(metadata.UserID) == "" {
		return GitHubWriteOutcome{}, ErrMissingUserID
	}
	hash, err := target.ActionHash()
	if err != nil {
		return GitHubWriteOutcome{}, err
	}
	clock := s.Clock
	if clock == nil {
		clock = time.Now
	}
	now := clock().UTC()
	previous, found, lookupErr := s.Receipts.Lookup(ctx, metadata.WorkspaceID, metadata.UserID, target.Operation, metadata.IdempotencyKey)
	if lookupErr != nil {
		return GitHubWriteOutcome{}, wrapConnectorFailure(ProviderGitHub, "lookup write receipt", lookupErr)
	}
	if found {
		if previous.ActionHash != hash || previous.TargetType != targetType(target) || previous.TargetID != targetID(target) {
			return GitHubWriteOutcome{}, ErrIdempotencyConflict
		}
		if previous.Status == SafeWriteReceiptPending {
			return GitHubWriteOutcome{Receipt: previous}, fmt.Errorf("%w: %s", ErrReceiptUncertain, previous.ID)
		}
		if previous.Status != SafeWriteReceiptAccepted {
			return GitHubWriteOutcome{Receipt: previous}, fmt.Errorf("%w: %s", ErrReceiptUncertain, previous.ID)
		}
		return GitHubWriteOutcome{Receipt: previous, Replayed: true}, nil
	}
	if err := metadata.ValidateConfirmation(now, target); err != nil {
		return GitHubWriteOutcome{}, err
	}
	if reader, ok := s.Challenges.(SafeWriteChallengeReader); ok {
		challenge, readErr := reader.Get(ctx, metadata.WorkspaceID, metadata.UserID, metadata.ChallengeID)
		if readErr != nil {
			return GitHubWriteOutcome{}, readErr
		}
		if err := challenge.Validate(now); err != nil {
			return GitHubWriteOutcome{}, err
		}
		challengeType, challengeID := challenge.TargetType, challenge.TargetID
		if challenge.WorkspaceID != metadata.WorkspaceID || challenge.UserID != metadata.UserID ||
			challenge.ID != metadata.ChallengeID || challenge.ActionHash != metadata.ActionHash ||
			challengeType != targetType(target) || challengeID != targetID(target) {
			return GitHubWriteOutcome{}, ErrInvalidChallenge
		}
	}
	pending := SafeWriteReceipt{
		ID:             deterministicReceiptID(metadata.WorkspaceID, metadata.UserID, target.Operation, metadata.IdempotencyKey, hash),
		Provider:       ProviderGitHub,
		WorkspaceID:    metadata.WorkspaceID,
		Operation:      target.Operation,
		UserID:         metadata.UserID,
		ChallengeID:    metadata.ChallengeID,
		IdempotencyKey: metadata.IdempotencyKey,
		ActionHash:     hash,
		TargetType:     targetType(target),
		TargetID:       targetID(target),
		Status:         SafeWriteReceiptPending,
		CreatedAt:      now,
	}
	reservationStore, durable := s.Receipts.(SafeWriteReservationStore)
	if durable {
		reserved, existed, reserveErr := reservationStore.Reserve(ctx, pending)
		if reserveErr != nil {
			return GitHubWriteOutcome{}, fmt.Errorf("%w: %s", ErrReceiptPersistence, RedactSecrets(reserveErr.Error()))
		}
		if existed {
			if reserved.ActionHash != hash || reserved.TargetType != pending.TargetType || reserved.TargetID != pending.TargetID {
				return GitHubWriteOutcome{}, ErrIdempotencyConflict
			}
			if reserved.Status == SafeWriteReceiptAccepted {
				return GitHubWriteOutcome{Receipt: reserved, Replayed: true}, nil
			}
			return GitHubWriteOutcome{Receipt: reserved}, fmt.Errorf("%w: %s", ErrReceiptUncertain, reserved.ID)
		}
	}
	if err := s.Challenges.Consume(ctx, metadata, target, now); err != nil {
		return GitHubWriteOutcome{}, err
	}
	partial, err := invoke()
	if err != nil {
		return GitHubWriteOutcome{}, err
	}
	partial.ID = pending.ID
	partial.Provider = ProviderGitHub
	partial.WorkspaceID = metadata.WorkspaceID
	partial.Operation = target.Operation
	partial.UserID = metadata.UserID
	partial.ChallengeID = metadata.ChallengeID
	partial.IdempotencyKey = metadata.IdempotencyKey
	partial.ActionHash = hash
	partial.TargetType = targetType(target)
	partial.TargetID = targetID(target)
	partial.Status = SafeWriteReceiptAccepted
	partial.CreatedAt = pending.CreatedAt
	if durable {
		if err := reservationStore.Complete(ctx, partial); err != nil {
			return GitHubWriteOutcome{Receipt: pending}, fmt.Errorf("%w: %s", ErrReceiptUncertain, RedactSecrets(err.Error()))
		}
	} else if err := s.Receipts.Save(ctx, partial); err != nil {
		return GitHubWriteOutcome{}, fmt.Errorf("%w: %s", ErrReceiptPersistence, RedactSecrets(err.Error()))
	}
	return GitHubWriteOutcome{Receipt: partial}, nil
}

func targetType(target SafeWriteTarget) string {
	value, _ := target.TargetTypeAndID()
	return value
}

func targetID(target SafeWriteTarget) string {
	_, value := target.TargetTypeAndID()
	return value
}

func deterministicReceiptID(workspaceID, userID string, operation SafeWriteOperation, idempotencyKey, actionHash string) string {
	payload, _ := json.Marshal(struct {
		WorkspaceID    string             `json:"workspaceID"`
		UserID         string             `json:"userID"`
		Operation      SafeWriteOperation `json:"operation"`
		IdempotencyKey string             `json:"idempotencyKey"`
		ActionHash     string             `json:"actionHash"`
	}{
		WorkspaceID:    strings.TrimSpace(workspaceID),
		UserID:         strings.TrimSpace(userID),
		Operation:      operation,
		IdempotencyKey: strings.TrimSpace(idempotencyKey),
		ActionHash:     actionHash,
	})
	digest := sha256.Sum256([]byte(payload))
	prefix := "github-write-"
	if ProviderForSafeWriteOperation(operation) == ProviderWork {
		prefix = "work-write-"
	}
	return prefix + hex.EncodeToString(digest[:])
}

func validateLabels(labels []string) error {
	seen := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		value := strings.TrimSpace(label)
		if value == "" {
			return ErrInvalidWriteTarget
		}
		if _, exists := seen[value]; exists {
			return ErrInvalidWriteTarget
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateWrittenIssue(issue GitHubIssue, target SafeWriteTarget) error {
	if normalizeRepository(issue.Repository) == "" || issue.Number < 1 || normalizeRepository(issue.Repository) != normalizeRepository(target.Repository) {
		return ErrInvalidProviderPayload
	}
	return nil
}

func validateWrittenComment(comment GitHubComment, target SafeWriteTarget) error {
	if normalizeRepository(comment.Repository) == "" || normalizeRepository(comment.Repository) != normalizeRepository(target.Repository) || comment.Issue != target.Issue || comment.ID < 1 {
		return ErrInvalidProviderPayload
	}
	return nil
}

func validateWrittenLabels(labels []GitHubLabel, target SafeWriteTarget) error {
	for _, label := range labels {
		if normalizeRepository(label.Repository) == "" || normalizeRepository(label.Repository) != normalizeRepository(target.Repository) || label.Issue != target.Issue || strings.TrimSpace(label.Name) == "" {
			return ErrInvalidProviderPayload
		}
	}
	return nil
}
