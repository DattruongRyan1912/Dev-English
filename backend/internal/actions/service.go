// Package actions owns the confirmation-gated application boundary for
// external mutations. It intentionally depends on provider-neutral connector
// contracts and returns assistant-safe receipt projections.
package actions

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

var (
	ErrInvalidScope       = errors.New("action scope is invalid")
	ErrInvalidRequest     = errors.New("action request is invalid")
	ErrActionUnavailable  = errors.New("action service is unavailable")
	ErrChallengeNotFound  = errors.New("action challenge was not found")
	ErrChallengeMismatch  = errors.New("action challenge does not match the requested target")
	ErrUnsupportedAction  = errors.New("action operation is not supported")
	ErrInvalidActionState = errors.New("action challenge state is invalid")
)

const (
	maxIDBytes       = 256
	maxIdempotency   = 256
	actionIDPrefix   = "action-"
	challengePrefix  = "challenge-"
	idempotencyLabel = "action-"
)

// Scope is intentionally an alias-shaped projection of the authenticated
// application scope. The action service never accepts workspace or user data
// from the model-facing target body.
type Scope = assistant.Scope

type CreateChallengeRequest struct {
	Target         connectors.SafeWriteTarget
	IdempotencyKey string
}

type Challenge struct {
	Binding   assistant.ActionBinding
	Target    connectors.SafeWriteTarget
	ExpiresAt time.Time
}

type Confirmation struct {
	Binding  assistant.ActionBinding
	Receipt  assistant.ActionReceipt
	Replayed bool
}

type Service struct {
	GitHub *connectors.GitHubSafeWriteService
	Work   *work.Service
	Issuer connectors.SafeWriteChallengeIssuer
	Reader connectors.SafeWriteChallengeReader
	// Challenges and Receipts are kept on the action service so non-GitHub
	// confirmed actions use the same durable identity and replay boundary as
	// the original GitHub writer.
	Challenges connectors.SafeWriteChallengeStore
	Receipts   connectors.SafeWriteReceiptStore
	Clock      func() time.Time
	Random     io.Reader
}

func NewService(github *connectors.GitHubSafeWriteService, issuer connectors.SafeWriteChallengeIssuer, reader connectors.SafeWriteChallengeReader) (*Service, error) {
	if github == nil || issuer == nil || reader == nil {
		return nil, ErrActionUnavailable
	}
	return &Service{
		GitHub: github, Issuer: issuer, Reader: reader,
		Challenges: github.Challenges, Receipts: github.Receipts,
		Clock: func() time.Time { return time.Now().UTC() }, Random: rand.Reader,
	}, nil
}

// NewServiceWithWork extends the confirmation-gated action boundary to
// canonical internal work mutations while preserving NewService's GitHub
// compatibility contract for existing callers and fixtures.
func NewServiceWithWork(github *connectors.GitHubSafeWriteService, workService *work.Service, issuer connectors.SafeWriteChallengeIssuer, reader connectors.SafeWriteChallengeReader) (*Service, error) {
	service, err := NewService(github, issuer, reader)
	if err != nil || workService == nil {
		if err != nil {
			return nil, err
		}
		return nil, ErrActionUnavailable
	}
	service.Work = workService
	return service, nil
}

func (s *Service) CreateChallenge(ctx context.Context, scope Scope, request CreateChallengeRequest) (Challenge, error) {
	if err := validateScope(scope); err != nil {
		return Challenge{}, err
	}
	if err := request.Target.Validate(); err != nil {
		return Challenge{}, err
	}
	provider := connectors.ProviderForSafeWriteOperation(request.Target.Operation)
	switch request.Target.Operation {
	case connectors.SafeWriteOperationCreateIssue, connectors.SafeWriteOperationAddComment, connectors.SafeWriteOperationSetLabels:
	case connectors.SafeWriteOperationEntityTrash, connectors.SafeWriteOperationEntityPurge:
		if s.Work == nil {
			return Challenge{}, ErrActionUnavailable
		}
	default:
		return Challenge{}, ErrUnsupportedAction
	}
	if request.IdempotencyKey != "" {
		if err := validateBounded(request.IdempotencyKey, maxIdempotency); err != nil {
			return Challenge{}, err
		}
	}
	hash, err := request.Target.ActionHash()
	if err != nil {
		return Challenge{}, err
	}
	now := s.now()
	challengeID, err := s.newChallengeID(scope, hash, now)
	if err != nil {
		return Challenge{}, err
	}
	idempotencyKey := strings.TrimSpace(request.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey, err = s.newIdempotencyKey(scope, hash, now)
		if err != nil {
			return Challenge{}, err
		}
	}
	challenge, err := connectors.NewWorkspaceSafeWriteChallenge(scope.WorkspaceID, challengeID, scope.UserID, request.Target, now)
	if err != nil {
		return Challenge{}, err
	}
	if err := s.Issuer.Put(challenge); err != nil {
		return Challenge{}, err
	}
	targetType, targetID := request.Target.TargetTypeAndID()
	binding := assistant.ActionBinding{
		ActionID:       actionID(hash),
		Scope:          scope,
		Provider:       provider,
		Operation:      string(request.Target.Operation),
		ChallengeID:    challenge.ID,
		IdempotencyKey: idempotencyKey,
		ActionHash:     hash,
		TargetType:     targetType,
		TargetID:       targetID,
	}
	return Challenge{Binding: binding, Target: cloneTarget(request.Target), ExpiresAt: challenge.ExpiresAt}, nil
}

func (s *Service) Confirm(ctx context.Context, scope Scope, challengeID string, target connectors.SafeWriteTarget, idempotencyKey string) (Confirmation, error) {
	if err := validateScope(scope); err != nil {
		return Confirmation{}, err
	}
	if err := validateBounded(challengeID, maxIDBytes); err != nil {
		return Confirmation{}, ErrInvalidRequest
	}
	if err := validateBounded(idempotencyKey, maxIdempotency); err != nil {
		return Confirmation{}, ErrInvalidRequest
	}
	if err := target.Validate(); err != nil {
		return Confirmation{}, err
	}
	challenge, err := s.Reader.Get(ctx, scope.WorkspaceID, scope.UserID, challengeID)
	if err != nil {
		if errors.Is(err, connectors.ErrInvalidChallenge) {
			return Confirmation{}, ErrChallengeNotFound
		}
		return Confirmation{}, err
	}
	now := s.now()
	// Do not reject an already-consumed or expired challenge before the guarded
	// writer gets a chance to look up its receipt. A retry with the same
	// idempotency key must return the original receipt without reaching GitHub;
	// the writer still rejects an un-receipted used/expired challenge below.
	if strings.TrimSpace(challenge.ID) == "" || challenge.ID != challengeID ||
		strings.TrimSpace(challenge.WorkspaceID) == "" || strings.TrimSpace(challenge.UserID) == "" ||
		strings.TrimSpace(challenge.TargetType) == "" || strings.TrimSpace(challenge.TargetID) == "" ||
		strings.TrimSpace(challenge.ActionHash) == "" || challenge.CreatedAt.IsZero() || challenge.ExpiresAt.IsZero() ||
		!challenge.ExpiresAt.After(challenge.CreatedAt) || challenge.ExpiresAt.Sub(challenge.CreatedAt) > connectors.SafeWriteChallengeTTL {
		return Confirmation{}, ErrInvalidActionState
	}
	hash, err := target.ActionHash()
	if err != nil {
		return Confirmation{}, err
	}
	targetType, targetID := target.TargetTypeAndID()
	if challenge.WorkspaceID != scope.WorkspaceID || challenge.UserID != scope.UserID || challenge.ActionHash != hash || challenge.TargetType != targetType || challenge.TargetID != targetID {
		return Confirmation{}, ErrChallengeMismatch
	}
	metadata := challenge.Confirmation(now)
	metadata.IdempotencyKey = strings.TrimSpace(idempotencyKey)
	metadata.ExpectedRevision = strings.TrimSpace(target.ExpectedRevision)
	switch target.Operation {
	case connectors.SafeWriteOperationEntityTrash, connectors.SafeWriteOperationEntityPurge:
		return s.confirmWorkMutation(ctx, scope, challenge, target, metadata, hash, targetType, targetID)
	}
	var outcome connectors.GitHubWriteOutcome
	switch target.Operation {
	case connectors.SafeWriteOperationCreateIssue:
		outcome, err = s.GitHub.CreateIssue(ctx, connectors.CreateIssueRequest{Repository: target.Repository, Title: target.Title, Body: target.Body, Metadata: metadata})
	case connectors.SafeWriteOperationAddComment:
		outcome, err = s.GitHub.AddIssueComment(ctx, connectors.CommentIssueRequest{Repository: target.Repository, Issue: target.Issue, Body: target.Body, Metadata: metadata})
	case connectors.SafeWriteOperationSetLabels:
		outcome, err = s.GitHub.SetIssueLabels(ctx, connectors.LabelIssueRequest{Repository: target.Repository, Issue: target.Issue, Labels: append([]string(nil), target.Labels...), Metadata: metadata})
	default:
		return Confirmation{}, ErrUnsupportedAction
	}
	if err != nil {
		return Confirmation{}, err
	}
	binding := assistant.ActionBinding{
		ActionID:       actionID(hash),
		Scope:          scope,
		Provider:       connectors.ProviderGitHub,
		Operation:      string(target.Operation),
		ChallengeID:    challenge.ID,
		IdempotencyKey: metadata.IdempotencyKey,
		ActionHash:     hash,
		TargetType:     targetType,
		TargetID:       targetID,
	}
	receipt := assistant.ActionReceipt{
		ID:             outcome.Receipt.ID,
		ActionID:       binding.ActionID,
		Scope:          scope,
		Provider:       outcome.Receipt.Provider,
		Operation:      string(outcome.Receipt.Operation),
		ChallengeID:    outcome.Receipt.ChallengeID,
		IdempotencyKey: outcome.Receipt.IdempotencyKey,
		ActionHash:     outcome.Receipt.ActionHash,
		TargetType:     outcome.Receipt.TargetType,
		TargetID:       outcome.Receipt.TargetID,
		Status:         outcome.Receipt.Status,
		Replayed:       outcome.Replayed,
	}
	if _, err := assistant.NewActionReceiptAttachment(binding, receipt); err != nil {
		return Confirmation{}, err
	}
	return Confirmation{Binding: binding, Receipt: receipt, Replayed: outcome.Replayed}, nil
}

func (s *Service) now() time.Time {
	if s != nil && s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) challengeStore() connectors.SafeWriteChallengeStore {
	if s != nil && s.Challenges != nil {
		return s.Challenges
	}
	if s != nil && s.GitHub != nil {
		return s.GitHub.Challenges
	}
	return nil
}

func (s *Service) receiptStore() connectors.SafeWriteReceiptStore {
	if s != nil && s.Receipts != nil {
		return s.Receipts
	}
	if s != nil && s.GitHub != nil {
		return s.GitHub.Receipts
	}
	return nil
}

func (s *Service) confirmWorkMutation(ctx context.Context, scope Scope, challenge connectors.SafeWriteChallenge, target connectors.SafeWriteTarget, metadata connectors.SafeWriteMetadata, hash, targetType, targetID string) (Confirmation, error) {
	if s.Work == nil {
		return Confirmation{}, ErrActionUnavailable
	}
	challenges := s.challengeStore()
	receipts := s.receiptStore()
	if challenges == nil || receipts == nil {
		return Confirmation{}, ErrActionUnavailable
	}
	now := s.now()
	previous, found, err := receipts.Lookup(ctx, scope.WorkspaceID, scope.UserID, target.Operation, metadata.IdempotencyKey)
	if err != nil {
		return Confirmation{}, err
	}
	if found {
		if previous.ActionHash != hash || previous.TargetType != targetType || previous.TargetID != targetID {
			return Confirmation{}, connectors.ErrIdempotencyConflict
		}
		binding := assistant.ActionBinding{
			ActionID: actionID(hash), Scope: scope, Provider: connectors.ProviderWork,
			Operation: string(target.Operation), ChallengeID: challenge.ID,
			IdempotencyKey: metadata.IdempotencyKey, ActionHash: hash,
			TargetType: targetType, TargetID: targetID,
		}
		if previous.Status != connectors.SafeWriteReceiptAccepted {
			return Confirmation{}, fmt.Errorf("%w: %s", connectors.ErrReceiptUncertain, previous.ID)
		}
		receipt := actionReceiptFromSafeWrite(binding, previous, true)
		if _, err := assistant.NewActionReceiptAttachment(binding, receipt); err != nil {
			return Confirmation{}, err
		}
		return Confirmation{Binding: binding, Receipt: receipt, Replayed: true}, nil
	}
	if err := metadata.ValidateConfirmation(now, target); err != nil {
		return Confirmation{}, err
	}
	if err := challenge.Validate(now); err != nil {
		return Confirmation{}, err
	}
	if target.Operation == connectors.SafeWriteOperationEntityTrash {
		if err := validateWorkVersion(ctx, s.Work, work.Scope(scope), target); err != nil {
			return Confirmation{}, err
		}
	}
	pending := connectors.SafeWriteReceipt{
		ID:       deterministicWorkReceiptID(scope, target.Operation, metadata.IdempotencyKey, hash),
		Provider: connectors.ProviderWork, WorkspaceID: scope.WorkspaceID,
		Operation: target.Operation, UserID: scope.UserID, ChallengeID: metadata.ChallengeID,
		IdempotencyKey: metadata.IdempotencyKey, ActionHash: hash,
		TargetType: targetType, TargetID: targetID,
		Status: connectors.SafeWriteReceiptPending, CreatedAt: now,
	}
	reservationStore, durable := receipts.(connectors.SafeWriteReservationStore)
	if durable {
		reserved, existed, reserveErr := reservationStore.Reserve(ctx, pending)
		if reserveErr != nil {
			return Confirmation{}, fmt.Errorf("%w: %s", connectors.ErrReceiptPersistence, connectors.RedactSecrets(reserveErr.Error()))
		}
		if existed {
			if reserved.ActionHash != hash || reserved.TargetType != targetType || reserved.TargetID != targetID {
				return Confirmation{}, connectors.ErrIdempotencyConflict
			}
			if reserved.Status == connectors.SafeWriteReceiptAccepted {
				binding := workActionBinding(scope, challenge, metadata, target.Operation, hash, targetType, targetID)
				receipt := actionReceiptFromSafeWrite(binding, reserved, true)
				return Confirmation{Binding: binding, Receipt: receipt, Replayed: true}, nil
			}
			return Confirmation{}, fmt.Errorf("%w: %s", connectors.ErrReceiptUncertain, reserved.ID)
		}
	}
	if err := challenges.Consume(ctx, metadata, target, now); err != nil {
		return Confirmation{}, err
	}
	if err := executeWorkMutation(ctx, s.Work, work.Scope(scope), target, metadata.IdempotencyKey); err != nil {
		return Confirmation{}, err
	}
	accepted := pending
	accepted.Status = connectors.SafeWriteReceiptAccepted
	if durable {
		if err := reservationStore.Complete(ctx, accepted); err != nil {
			return Confirmation{}, fmt.Errorf("%w: %s", connectors.ErrReceiptUncertain, connectors.RedactSecrets(err.Error()))
		}
	} else if err := receipts.Save(ctx, accepted); err != nil {
		return Confirmation{}, fmt.Errorf("%w: %s", connectors.ErrReceiptPersistence, connectors.RedactSecrets(err.Error()))
	}
	binding := workActionBinding(scope, challenge, metadata, target.Operation, hash, targetType, targetID)
	receipt := actionReceiptFromSafeWrite(binding, accepted, false)
	if _, err := assistant.NewActionReceiptAttachment(binding, receipt); err != nil {
		return Confirmation{}, err
	}
	return Confirmation{Binding: binding, Receipt: receipt}, nil
}

func workActionBinding(scope Scope, challenge connectors.SafeWriteChallenge, metadata connectors.SafeWriteMetadata, operation connectors.SafeWriteOperation, hash, targetType, targetID string) assistant.ActionBinding {
	return assistant.ActionBinding{
		ActionID: actionID(hash), Scope: scope, Provider: connectors.ProviderWork,
		Operation: string(operation), ChallengeID: challenge.ID,
		IdempotencyKey: metadata.IdempotencyKey, ActionHash: hash,
		TargetType: targetType, TargetID: targetID,
	}
}

func actionReceiptFromSafeWrite(binding assistant.ActionBinding, receipt connectors.SafeWriteReceipt, replayed bool) assistant.ActionReceipt {
	return assistant.ActionReceipt{
		ID: receipt.ID, ActionID: binding.ActionID, Scope: binding.Scope,
		Provider: receipt.Provider, Operation: string(receipt.Operation),
		ChallengeID: receipt.ChallengeID, IdempotencyKey: receipt.IdempotencyKey,
		ActionHash: receipt.ActionHash, TargetType: receipt.TargetType,
		TargetID: receipt.TargetID, Status: receipt.Status, Replayed: replayed,
	}
}

func validateWorkVersion(ctx context.Context, service *work.Service, scope work.Scope, target connectors.SafeWriteTarget) error {
	var version int64
	switch target.EntityType {
	case string(work.EntityProject):
		project, err := service.GetProject(ctx, scope, target.EntityID)
		if err != nil {
			return err
		}
		version = project.Version
	case string(work.EntityTask):
		task, err := service.GetTask(ctx, scope, target.EntityID)
		if err != nil {
			return err
		}
		version = task.Version
	case string(work.EntityDecision):
		decision, err := service.GetDecision(ctx, scope, target.EntityID)
		if err != nil {
			return err
		}
		version = decision.Version
	default:
		return connectors.ErrInvalidWriteTarget
	}
	if version != target.ExpectedVersion {
		return &work.VersionConflictError{EntityType: work.EntityType(target.EntityType), EntityID: target.EntityID, Expected: target.ExpectedVersion, Actual: version}
	}
	return nil
}

func executeWorkMutation(ctx context.Context, service *work.Service, scope work.Scope, target connectors.SafeWriteTarget, idempotencyKey string) error {
	switch target.EntityType {
	case string(work.EntityProject):
		var err error
		if target.Operation == connectors.SafeWriteOperationEntityPurge {
			_, err = service.PurgeProject(ctx, scope, target.EntityID, target.ExpectedVersion, idempotencyKey)
		} else {
			_, err = service.TrashProject(ctx, scope, target.EntityID, target.ExpectedVersion, idempotencyKey)
		}
		return err
	case string(work.EntityTask):
		var err error
		if target.Operation == connectors.SafeWriteOperationEntityPurge {
			_, err = service.PurgeTask(ctx, scope, target.EntityID, target.ExpectedVersion, idempotencyKey)
		} else {
			_, err = service.TrashTask(ctx, scope, target.EntityID, target.ExpectedVersion, idempotencyKey)
		}
		return err
	case string(work.EntityDecision):
		var err error
		if target.Operation == connectors.SafeWriteOperationEntityPurge {
			_, err = service.PurgeDecision(ctx, scope, target.EntityID, target.ExpectedVersion, idempotencyKey)
		} else {
			_, err = service.TrashDecision(ctx, scope, target.EntityID, target.ExpectedVersion, idempotencyKey)
		}
		return err
	default:
		return connectors.ErrInvalidWriteTarget
	}
}

func deterministicWorkReceiptID(scope Scope, operation connectors.SafeWriteOperation, idempotencyKey, hash string) string {
	// Keep the same deterministic identity algorithm as the GitHub facade while
	// using a work-specific namespace to make provider receipts self-describing.
	payload, _ := json.Marshal(struct {
		WorkspaceID    string                        `json:"workspaceID"`
		UserID         string                        `json:"userID"`
		Operation      connectors.SafeWriteOperation `json:"operation"`
		IdempotencyKey string                        `json:"idempotencyKey"`
		ActionHash     string                        `json:"actionHash"`
	}{scope.WorkspaceID, scope.UserID, operation, strings.TrimSpace(idempotencyKey), hash})
	digest := sha256.Sum256([]byte(payload))
	return "work-write-" + hex.EncodeToString(digest[:])
}

func (s *Service) newChallengeID(scope Scope, hash string, now time.Time) (string, error) {
	return s.randomID(challengePrefix, scope, hash, now)
}

func (s *Service) newIdempotencyKey(scope Scope, hash string, now time.Time) (string, error) {
	return s.randomID(idempotencyLabel, scope, hash, now)
}

func (s *Service) randomID(prefix string, scope Scope, hash string, now time.Time) (string, error) {
	buffer := make([]byte, 12)
	reader := io.Reader(rand.Reader)
	if s != nil && s.Random != nil {
		reader = s.Random
	}
	if _, err := io.ReadFull(reader, buffer); err != nil {
		return "", fmt.Errorf("allocate action identity: %w", err)
	}
	seed := sha256.Sum256([]byte(scope.WorkspaceID + "\x00" + scope.UserID + "\x00" + hash + "\x00" + now.Format(time.RFC3339Nano) + "\x00" + hex.EncodeToString(buffer)))
	return prefix + hex.EncodeToString(seed[:12]), nil
}

func actionID(hash string) string {
	if len(hash) > 16 {
		hash = hash[:16]
	}
	return actionIDPrefix + hash
}

func validateScope(scope Scope) error {
	if err := validateBounded(scope.WorkspaceID, maxIDBytes); err != nil {
		return ErrInvalidScope
	}
	if err := validateBounded(scope.UserID, maxIDBytes); err != nil {
		return ErrInvalidScope
	}
	return nil
}

func validateBounded(value string, limit int) error {
	if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value || utf8.RuneCountInString(value) > limit || strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return ErrInvalidRequest
	}
	return nil
}

func cloneTarget(target connectors.SafeWriteTarget) connectors.SafeWriteTarget {
	target.Labels = append([]string(nil), target.Labels...)
	return target
}
