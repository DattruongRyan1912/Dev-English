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
	SafeWriteChallengeTTL                            = 5 * time.Minute
	SafeWriteReceiptPending                          = "pending"
	SafeWriteReceiptUncertain                        = "uncertain"
	SafeWriteReceiptAccepted                         = "accepted"
)

type SafeWriteOperation string

// SafeWriteTarget is the canonical action payload used to bind a challenge.
// It intentionally contains no credentials or provider response body.
type SafeWriteTarget struct {
	Operation        SafeWriteOperation
	Repository       string
	Issue            int64
	Title            string
	Body             string
	Labels           []string
	ExpectedRevision string
}

func (t SafeWriteTarget) Validate() error {
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
		Repository       string             `json:"repository"`
		Issue            int64              `json:"issue,omitempty"`
		Title            string             `json:"title,omitempty"`
		Body             string             `json:"body,omitempty"`
		Labels           []string           `json:"labels,omitempty"`
		ExpectedRevision string             `json:"expectedRevision,omitempty"`
	}{
		Operation:        t.Operation,
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

func NewSafeWriteChallenge(id, workspaceID, userID string, target SafeWriteTarget, now time.Time) (SafeWriteChallenge, error) {
	hash, err := target.ActionHash()
	if err != nil {
		return SafeWriteChallenge{}, err
	}
	targetType, targetID := target.TargetTypeAndID()
	createdAt := now.UTC()
	challenge := SafeWriteChallenge{
		ID:          strings.TrimSpace(id),
		WorkspaceID: strings.TrimSpace(workspaceID),
		UserID:      strings.TrimSpace(userID),
		TargetType:  targetType,
		TargetID:    targetID,
		ActionHash:  hash,
		CreatedAt:   createdAt,
		ExpiresAt:   createdAt.Add(SafeWriteChallengeTTL),
	}
	if err := challenge.Validate(now); err != nil {
		return SafeWriteChallenge{}, err
	}
	return challenge, nil
}

func (c SafeWriteChallenge) Validate(now time.Time) error {
	if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.WorkspaceID) == "" || strings.TrimSpace(c.UserID) == "" || strings.TrimSpace(c.TargetType) == "" || strings.TrimSpace(c.TargetID) == "" || strings.TrimSpace(c.ActionHash) == "" {
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

func (m SafeWriteMetadata) ValidateConfirmation(now time.Time, target SafeWriteTarget) error {
	if strings.TrimSpace(m.UserID) == "" {
		return ErrMissingUserID
	}
	if strings.TrimSpace(m.ChallengeID) == "" || strings.TrimSpace(m.ActionHash) == "" {
		return ErrMissingChallenge
	}
	if strings.TrimSpace(m.WorkspaceID) == "" {
		return ErrInvalidWorkspaceID
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
	Provider       string
	Operation      SafeWriteOperation
	WorkspaceID    string
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
	// Reserve atomically records a pending receipt before an external mutation.
	// The bool is true when an existing receipt owns the same idempotency key.
	Reserve(ctx context.Context, receipt SafeWriteReceipt) (SafeWriteReceipt, bool, error)
	Save(ctx context.Context, receipt SafeWriteReceipt) error
}

type GitHubWriteOutcome struct {
	Receipt   SafeWriteReceipt
	Replayed  bool
	Uncertain bool
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
	request.Metadata = normalizeMetadata(request.Metadata)
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
	request.Metadata = normalizeMetadata(request.Metadata)
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
	request.Metadata = normalizeMetadata(request.Metadata)
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
	if s == nil || s.Writer == nil || s.Challenges == nil || s.Receipts == nil {
		return GitHubWriteOutcome{}, errors.New("GitHub writer, challenge store and receipt store are required")
	}
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
	metadata = normalizeMetadata(metadata)
	clock := s.Clock
	if clock == nil {
		clock = time.Now
	}
	now := clock().UTC()

	// The process-local lock keeps same-instance calls deterministic. Reserve is
	// still required because separate service instances must share the durable
	// idempotency boundary before either one invokes the provider.
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, found, lookupErr := s.Receipts.Lookup(ctx, metadata.WorkspaceID, metadata.UserID, target.Operation, metadata.IdempotencyKey)
	if lookupErr != nil {
		return GitHubWriteOutcome{}, wrapConnectorFailure(ProviderGitHub, "lookup write receipt", lookupErr)
	}
	if found {
		return existingReceiptOutcome(previous, metadata, target, hash)
	}
	if err := metadata.ValidateConfirmation(now, target); err != nil {
		return GitHubWriteOutcome{}, err
	}
	if err := s.Challenges.Consume(ctx, metadata, target, now); err != nil {
		return GitHubWriteOutcome{}, err
	}
	reservation := SafeWriteReceipt{
		ID:             deterministicReceiptID(metadata.WorkspaceID, metadata.UserID, target.Operation, metadata.IdempotencyKey, hash),
		Provider:       ProviderGitHub,
		Operation:      target.Operation,
		WorkspaceID:    metadata.WorkspaceID,
		UserID:         metadata.UserID,
		ChallengeID:    metadata.ChallengeID,
		IdempotencyKey: metadata.IdempotencyKey,
		ActionHash:     hash,
		TargetType:     targetType(target),
		TargetID:       targetID(target),
		Status:         SafeWriteReceiptPending,
		CreatedAt:      now,
	}
	reserved, alreadyReserved, reserveErr := s.Receipts.Reserve(ctx, reservation)
	if reserveErr != nil {
		if errors.Is(reserveErr, ErrIdempotencyConflict) {
			return GitHubWriteOutcome{}, reserveErr
		}
		return GitHubWriteOutcome{}, fmt.Errorf("%w: %s", ErrReceiptPersistence, RedactSecrets(reserveErr.Error()))
	}
	if alreadyReserved {
		return existingReceiptOutcome(reserved, metadata, target, hash)
	}
	partial, err := invoke()
	if err != nil {
		uncertain := reservation
		uncertain.Status = SafeWriteReceiptUncertain
		if saveErr := s.Receipts.Save(ctx, uncertain); saveErr != nil {
			return GitHubWriteOutcome{Receipt: reservation}, fmt.Errorf("%w: %s", ErrReceiptPersistence, RedactSecrets(saveErr.Error()))
		}
		return GitHubWriteOutcome{Receipt: uncertain, Uncertain: true}, newUncertainWriteError(err)
	}
	accepted := reservation
	accepted.Issue = partial.Issue
	accepted.Comment = partial.Comment
	accepted.Labels = append([]GitHubLabel(nil), partial.Labels...)
	accepted.Status = SafeWriteReceiptAccepted
	if err := s.Receipts.Save(ctx, accepted); err != nil {
		if errors.Is(err, ErrIdempotencyConflict) {
			return GitHubWriteOutcome{}, err
		}
		uncertain := reservation
		uncertain.Status = SafeWriteReceiptUncertain
		if uncertainSaveErr := s.Receipts.Save(ctx, uncertain); uncertainSaveErr == nil {
			reservation = uncertain
		}
		return GitHubWriteOutcome{Receipt: reservation, Uncertain: true}, fmt.Errorf("%w: %s", ErrReceiptPersistence, RedactSecrets(err.Error()))
	}
	return GitHubWriteOutcome{Receipt: accepted}, nil
}

func normalizeMetadata(metadata SafeWriteMetadata) SafeWriteMetadata {
	metadata.WorkspaceID = strings.TrimSpace(metadata.WorkspaceID)
	metadata.IdempotencyKey = strings.TrimSpace(metadata.IdempotencyKey)
	metadata.UserID = strings.TrimSpace(metadata.UserID)
	metadata.ChallengeID = strings.TrimSpace(metadata.ChallengeID)
	metadata.ActionHash = strings.ToLower(strings.TrimSpace(metadata.ActionHash))
	return metadata
}

func receiptIdentityMatches(receipt SafeWriteReceipt, metadata SafeWriteMetadata, target SafeWriteTarget, hash string) bool {
	return receipt.WorkspaceID == metadata.WorkspaceID &&
		receipt.UserID == metadata.UserID &&
		receipt.Operation == target.Operation &&
		receipt.IdempotencyKey == metadata.IdempotencyKey &&
		strings.EqualFold(receipt.ActionHash, hash) &&
		receipt.TargetType == targetType(target) &&
		receipt.TargetID == targetID(target)
}

func existingReceiptOutcome(receipt SafeWriteReceipt, metadata SafeWriteMetadata, target SafeWriteTarget, hash string) (GitHubWriteOutcome, error) {
	if !receiptIdentityMatches(receipt, metadata, target, hash) {
		return GitHubWriteOutcome{}, ErrIdempotencyConflict
	}
	switch receipt.Status {
	case SafeWriteReceiptAccepted:
		return GitHubWriteOutcome{Receipt: receipt, Replayed: true}, nil
	case SafeWriteReceiptPending:
		return GitHubWriteOutcome{Receipt: receipt}, ErrWritePending
	case SafeWriteReceiptUncertain:
		return GitHubWriteOutcome{Receipt: receipt, Uncertain: true}, ErrWriteUncertain
	default:
		return GitHubWriteOutcome{}, fmt.Errorf("%w: invalid safe write receipt status", ErrReceiptPersistence)
	}
}

type uncertainWriteError struct {
	cause error
}

func newUncertainWriteError(cause error) error {
	return &uncertainWriteError{cause: cause}
}

func (e *uncertainWriteError) Error() string {
	if e == nil || e.cause == nil {
		return ErrWriteUncertain.Error()
	}
	return fmt.Sprintf("%s: %s", ErrWriteUncertain, RedactSecrets(e.cause.Error()))
}

func (e *uncertainWriteError) Is(target error) bool {
	return target == ErrWriteUncertain
}

func (e *uncertainWriteError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
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
	payload := strings.Join([]string{strings.TrimSpace(workspaceID), strings.TrimSpace(userID), string(operation), strings.TrimSpace(idempotencyKey), actionHash}, "\x00")
	digest := sha256.Sum256([]byte(payload))
	return "github-write-" + hex.EncodeToString(digest[:])
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
