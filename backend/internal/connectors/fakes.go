package connectors

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// FakeDriveReader is a deterministic, in-memory Drive page adapter for unit
// and integration tests. It never performs network I/O.
type FakeDriveReader struct {
	mu       sync.Mutex
	Pages    []DrivePage
	Errors   map[int]error
	Requests []DriveListRequest
	call     int
}

func (f *FakeDriveReader) List(ctx context.Context, request DriveListRequest) (DrivePage, error) {
	if err := contextError(ctx); err != nil {
		return DrivePage{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	call := f.call
	f.call++
	f.Requests = append(f.Requests, request)
	if err := f.Errors[call]; err != nil {
		return DrivePage{}, err
	}
	if call >= len(f.Pages) {
		return DrivePage{}, nil
	}
	return cloneDrivePage(f.Pages[call]), nil
}

func (f *FakeDriveReader) RequestsSnapshot() []DriveListRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]DriveListRequest(nil), f.Requests...)
}

// MemoryDriveRevisionStore provides deterministic revision uniqueness and
// cursor checkpointing for tests. Failure fields are intentionally explicit so
// callers can exercise rollback behavior without a provider.
type MemoryDriveRevisionStore struct {
	mu               sync.Mutex
	Revisions        map[string]DriveSourceItem
	Cursors          map[string]DriveSyncState
	HasRevisionError error
	PutRevisionError error
	SaveCursorError  error
}

func NewMemoryDriveRevisionStore() *MemoryDriveRevisionStore {
	return &MemoryDriveRevisionStore{
		Revisions: make(map[string]DriveSourceItem),
		Cursors:   make(map[string]DriveSyncState),
	}
}

func (s *MemoryDriveRevisionStore) HasRevision(ctx context.Context, workspaceID, revisionKey string) (bool, error) {
	if err := contextError(ctx); err != nil {
		return false, err
	}
	key, err := scopedRevisionKey(workspaceID, revisionKey)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.HasRevisionError != nil {
		return false, s.HasRevisionError
	}
	_, exists := s.Revisions[key]
	return exists, nil
}

func (s *MemoryDriveRevisionStore) PutRevision(ctx context.Context, workspaceID string, item DriveSourceItem) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := item.Validate(); err != nil {
		return err
	}
	key, err := scopedRevisionKey(workspaceID, item.RevisionKey())
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.PutRevisionError != nil {
		return s.PutRevisionError
	}
	if s.Revisions == nil {
		s.Revisions = make(map[string]DriveSourceItem)
	}
	if _, exists := s.Revisions[key]; exists {
		return ErrRevisionAlreadyExists
	}
	s.Revisions[key] = item
	return nil
}

func (s *MemoryDriveRevisionStore) SaveCursor(ctx context.Context, state DriveSyncState) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	state.WorkspaceID = strings.TrimSpace(state.WorkspaceID)
	state.Cursor = normalizedCursor(state.Cursor)
	if err := state.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SaveCursorError != nil {
		return s.SaveCursorError
	}
	if s.Cursors == nil {
		s.Cursors = make(map[string]DriveSyncState)
	}
	s.Cursors[state.WorkspaceID] = state
	return nil
}

func (s *MemoryDriveRevisionStore) RevisionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Revisions)
}

func (s *MemoryDriveRevisionStore) Cursor(workspaceID string) (DriveSyncState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, exists := s.Cursors[strings.TrimSpace(workspaceID)]
	return state, exists
}

// FakeGitHubReadClient is a deterministic read/import fixture.
type FakeGitHubReadClient struct {
	mu           sync.Mutex
	IssuePages   []GitHubIssuePage
	Issues       map[string]GitHubIssue
	ListErrors   map[int]error
	GetErrors    map[string]error
	ListRequests []GitHubIssueListRequest
	GetRequests  []string
	listCall     int
}

func (f *FakeGitHubReadClient) ListIssues(ctx context.Context, request GitHubIssueListRequest) (GitHubIssuePage, error) {
	if err := contextError(ctx); err != nil {
		return GitHubIssuePage{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	call := f.listCall
	f.listCall++
	f.ListRequests = append(f.ListRequests, request)
	if err := f.ListErrors[call]; err != nil {
		return GitHubIssuePage{}, err
	}
	if call >= len(f.IssuePages) {
		return GitHubIssuePage{}, nil
	}
	return cloneGitHubIssuePage(f.IssuePages[call]), nil
}

func (f *FakeGitHubReadClient) GetIssue(ctx context.Context, repository string, number int64) (GitHubIssue, error) {
	if err := contextError(ctx); err != nil {
		return GitHubIssue{}, err
	}
	key := fmt.Sprintf("%s#%d", normalizeRepository(repository), number)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.GetRequests = append(f.GetRequests, key)
	if err := f.GetErrors[key]; err != nil {
		return GitHubIssue{}, err
	}
	issue, exists := f.Issues[key]
	if !exists {
		return GitHubIssue{}, ErrNotFound
	}
	return issue, nil
}

func (f *FakeGitHubReadClient) ListRequestsSnapshot() []GitHubIssueListRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]GitHubIssueListRequest(nil), f.ListRequests...)
}

func (f *FakeGitHubReadClient) GetRequestsSnapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.GetRequests...)
}

type MemoryGitHubRevisionStore struct {
	mu               sync.Mutex
	Revisions        map[string]GitHubImportItem
	Cursors          map[string]GitHubSyncState
	HasRevisionError error
	PutRevisionError error
	SaveCursorError  error
}

func NewMemoryGitHubRevisionStore() *MemoryGitHubRevisionStore {
	return &MemoryGitHubRevisionStore{
		Revisions: make(map[string]GitHubImportItem),
		Cursors:   make(map[string]GitHubSyncState),
	}
}

func (s *MemoryGitHubRevisionStore) HasRevision(ctx context.Context, workspaceID, revisionKey string) (bool, error) {
	if err := contextError(ctx); err != nil {
		return false, err
	}
	key, err := scopedRevisionKey(workspaceID, revisionKey)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.HasRevisionError != nil {
		return false, s.HasRevisionError
	}
	_, exists := s.Revisions[key]
	return exists, nil
}

func (s *MemoryGitHubRevisionStore) PutRevision(ctx context.Context, workspaceID string, item GitHubImportItem) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := item.Validate(); err != nil {
		return err
	}
	key, err := scopedRevisionKey(workspaceID, item.RevisionKey())
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.PutRevisionError != nil {
		return s.PutRevisionError
	}
	if s.Revisions == nil {
		s.Revisions = make(map[string]GitHubImportItem)
	}
	if _, exists := s.Revisions[key]; exists {
		return ErrRevisionAlreadyExists
	}
	s.Revisions[key] = item
	return nil
}

func (s *MemoryGitHubRevisionStore) SaveCursor(ctx context.Context, state GitHubSyncState) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	state.WorkspaceID = strings.TrimSpace(state.WorkspaceID)
	state.Repository = normalizeRepository(state.Repository)
	state.Cursor = strings.TrimSpace(state.Cursor)
	if err := state.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SaveCursorError != nil {
		return s.SaveCursorError
	}
	if s.Cursors == nil {
		s.Cursors = make(map[string]GitHubSyncState)
	}
	s.Cursors[state.WorkspaceID+":"+state.Repository] = state
	return nil
}

func (s *MemoryGitHubRevisionStore) Cursor(workspaceID, repository string) (GitHubSyncState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, exists := s.Cursors[strings.TrimSpace(workspaceID)+":"+normalizeRepository(repository)]
	return state, exists
}

func (s *MemoryGitHubRevisionStore) RevisionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Revisions)
}

// FakeGitHubWriter is a deterministic safe-write fixture. It implements only
// the three operations allowed by GitHubSafeWriter.
type FakeGitHubWriter struct {
	mu              sync.Mutex
	NextIssueNumber int64
	Failure         error
	CreateCalls     []CreateIssueRequest
	CommentCalls    []CommentIssueRequest
	LabelCalls      []LabelIssueRequest
	sequence        int64
}

func (f *FakeGitHubWriter) CreateIssue(ctx context.Context, request CreateIssueRequest) (GitHubIssue, error) {
	if err := contextError(ctx); err != nil {
		return GitHubIssue{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CreateCalls = append(f.CreateCalls, request)
	if f.Failure != nil {
		return GitHubIssue{}, f.Failure
	}
	if f.NextIssueNumber < 1 {
		f.NextIssueNumber = 1
	}
	f.sequence++
	number := f.NextIssueNumber
	f.NextIssueNumber++
	return GitHubIssue{
		Repository: request.Repository,
		Number:     number,
		Title:      request.Title,
		Body:       request.Body,
		State:      "open",
		Revision:   fmt.Sprintf("fixture-%d", f.sequence),
		UpdatedAt:  time.Unix(f.sequence, 0).UTC(),
	}, nil
}

func (f *FakeGitHubWriter) AddIssueComment(ctx context.Context, request CommentIssueRequest) (GitHubComment, error) {
	if err := contextError(ctx); err != nil {
		return GitHubComment{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CommentCalls = append(f.CommentCalls, request)
	if f.Failure != nil {
		return GitHubComment{}, f.Failure
	}
	f.sequence++
	return GitHubComment{
		Repository: request.Repository,
		Issue:      request.Issue,
		ID:         f.sequence,
		Body:       request.Body,
		Revision:   fmt.Sprintf("fixture-comment-%d", f.sequence),
		UpdatedAt:  time.Unix(f.sequence, 0).UTC(),
	}, nil
}

func (f *FakeGitHubWriter) SetIssueLabels(ctx context.Context, request LabelIssueRequest) ([]GitHubLabel, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.LabelCalls = append(f.LabelCalls, request)
	if f.Failure != nil {
		return nil, f.Failure
	}
	labels := make([]GitHubLabel, 0, len(request.Labels))
	for _, name := range request.Labels {
		labels = append(labels, GitHubLabel{Repository: request.Repository, Issue: request.Issue, Name: name})
	}
	return labels, nil
}

// MemorySafeWriteChallengeStore is a one-use challenge fixture with the same
// user/target/hash checks expected from a persistent implementation.
type MemorySafeWriteChallengeStore struct {
	mu         sync.Mutex
	Challenges map[string]SafeWriteChallenge
}

func NewMemorySafeWriteChallengeStore() *MemorySafeWriteChallengeStore {
	return &MemorySafeWriteChallengeStore{Challenges: make(map[string]SafeWriteChallenge)}
}

func (s *MemorySafeWriteChallengeStore) Put(challenge SafeWriteChallenge) error {
	if err := challenge.Validate(challenge.CreatedAt); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Challenges == nil {
		s.Challenges = make(map[string]SafeWriteChallenge)
	}
	if _, exists := s.Challenges[challenge.ID]; exists {
		return ErrInvalidChallenge
	}
	s.Challenges[challenge.ID] = challenge
	return nil
}

func (s *MemorySafeWriteChallengeStore) Consume(ctx context.Context, metadata SafeWriteMetadata, target SafeWriteTarget, now time.Time) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, exists := s.Challenges[strings.TrimSpace(metadata.ChallengeID)]
	if !exists {
		return ErrInvalidChallenge
	}
	if challenge.UsedAt != nil {
		return ErrChallengeUsed
	}
	if !challenge.ExpiresAt.After(now) {
		return ErrChallengeExpired
	}
	if err := metadata.ValidateConfirmation(now, target); err != nil {
		return err
	}
	targetType, targetID := target.TargetTypeAndID()
	if challenge.WorkspaceID != strings.TrimSpace(metadata.WorkspaceID) || challenge.UserID != strings.TrimSpace(metadata.UserID) || challenge.TargetType != targetType || challenge.TargetID != targetID || !strings.EqualFold(challenge.ActionHash, strings.TrimSpace(metadata.ActionHash)) {
		return ErrInvalidChallenge
	}
	if metadata.ConfirmedAt.Before(challenge.CreatedAt) || !metadata.ExpiresAt.Equal(challenge.ExpiresAt) {
		return ErrInvalidChallenge
	}
	if expected, err := target.ActionHash(); err != nil || !strings.EqualFold(expected, challenge.ActionHash) {
		return ErrInvalidChallenge
	}
	usedAt := now.UTC()
	challenge.UsedAt = &usedAt
	s.Challenges[strings.TrimSpace(metadata.ChallengeID)] = challenge
	return nil
}

func (s *MemorySafeWriteChallengeStore) Challenge(id string) (SafeWriteChallenge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, exists := s.Challenges[strings.TrimSpace(id)]
	if challenge.UsedAt != nil {
		usedAt := *challenge.UsedAt
		challenge.UsedAt = &usedAt
	}
	return challenge, exists
}

type MemorySafeWriteReceiptStore struct {
	mu       sync.Mutex
	Receipts map[string]SafeWriteReceipt
}

func NewMemorySafeWriteReceiptStore() *MemorySafeWriteReceiptStore {
	return &MemorySafeWriteReceiptStore{Receipts: make(map[string]SafeWriteReceipt)}
}

func (s *MemorySafeWriteReceiptStore) Lookup(ctx context.Context, workspaceID, userID string, operation SafeWriteOperation, idempotencyKey string) (SafeWriteReceipt, bool, error) {
	if err := contextError(ctx); err != nil {
		return SafeWriteReceipt{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	receipt, exists := s.Receipts[receiptStoreKey(workspaceID, userID, operation, idempotencyKey)]
	if !exists {
		return SafeWriteReceipt{}, false, nil
	}
	return cloneSafeWriteReceipt(receipt), true, nil
}

func (s *MemorySafeWriteReceiptStore) Reserve(ctx context.Context, receipt SafeWriteReceipt) (SafeWriteReceipt, bool, error) {
	if err := contextError(ctx); err != nil {
		return SafeWriteReceipt{}, false, err
	}
	if err := validateReceiptForStore(receipt, SafeWriteReceiptPending); err != nil {
		return SafeWriteReceipt{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Receipts == nil {
		s.Receipts = make(map[string]SafeWriteReceipt)
	}
	key := receiptStoreKey(receipt.WorkspaceID, receipt.UserID, receipt.Operation, receipt.IdempotencyKey)
	if previous, exists := s.Receipts[key]; exists {
		if !receiptIdentityEqual(previous, receipt) {
			return SafeWriteReceipt{}, false, ErrIdempotencyConflict
		}
		return cloneSafeWriteReceipt(previous), true, nil
	}
	s.Receipts[key] = cloneSafeWriteReceipt(receipt)
	return cloneSafeWriteReceipt(receipt), false, nil
}

func (s *MemorySafeWriteReceiptStore) Save(ctx context.Context, receipt SafeWriteReceipt) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := validateReceiptForStore(receipt, receipt.Status); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Receipts == nil {
		s.Receipts = make(map[string]SafeWriteReceipt)
	}
	key := receiptStoreKey(receipt.WorkspaceID, receipt.UserID, receipt.Operation, receipt.IdempotencyKey)
	if previous, exists := s.Receipts[key]; exists {
		if !receiptIdentityEqual(previous, receipt) {
			return ErrIdempotencyConflict
		}
		if receiptStatusRank(receipt.Status) < receiptStatusRank(previous.Status) {
			return nil
		}
		if previous.Status == SafeWriteReceiptAccepted {
			return nil
		}
	}
	s.Receipts[key] = cloneSafeWriteReceipt(receipt)
	return nil
}

func (s *MemorySafeWriteReceiptStore) ReceiptCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Receipts)
}

func receiptStoreKey(workspaceID, userID string, operation SafeWriteOperation, idempotencyKey string) string {
	return strings.TrimSpace(workspaceID) + "\x00" + strings.TrimSpace(userID) + "\x00" + string(operation) + "\x00" + strings.TrimSpace(idempotencyKey)
}

func contextError(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func stringsTrimEmpty(value string) bool {
	return strings.TrimSpace(value) == ""
}

func validateReceiptForStore(receipt SafeWriteReceipt, expectedStatus string) error {
	if stringsTrimEmpty(receipt.ID) || stringsTrimEmpty(receipt.WorkspaceID) || stringsTrimEmpty(receipt.UserID) || receipt.Operation == "" || stringsTrimEmpty(receipt.IdempotencyKey) || stringsTrimEmpty(receipt.ActionHash) || stringsTrimEmpty(receipt.TargetType) || stringsTrimEmpty(receipt.TargetID) || receipt.CreatedAt.IsZero() {
		return ErrReceiptPersistence
	}
	switch receipt.Status {
	case SafeWriteReceiptPending, SafeWriteReceiptUncertain, SafeWriteReceiptAccepted:
	default:
		return ErrReceiptPersistence
	}
	if expectedStatus != "" && receipt.Status != expectedStatus {
		return ErrReceiptPersistence
	}
	return nil
}

func receiptIdentityEqual(left, right SafeWriteReceipt) bool {
	return strings.TrimSpace(left.WorkspaceID) == strings.TrimSpace(right.WorkspaceID) &&
		strings.TrimSpace(left.UserID) == strings.TrimSpace(right.UserID) &&
		left.Operation == right.Operation &&
		strings.TrimSpace(left.IdempotencyKey) == strings.TrimSpace(right.IdempotencyKey) &&
		strings.EqualFold(strings.TrimSpace(left.ActionHash), strings.TrimSpace(right.ActionHash)) &&
		strings.TrimSpace(left.TargetType) == strings.TrimSpace(right.TargetType) &&
		strings.TrimSpace(left.TargetID) == strings.TrimSpace(right.TargetID)
}

func receiptStatusRank(status string) int {
	switch status {
	case SafeWriteReceiptPending:
		return 1
	case SafeWriteReceiptUncertain:
		return 2
	case SafeWriteReceiptAccepted:
		return 3
	default:
		return 0
	}
}

func cloneDrivePage(page DrivePage) DrivePage {
	page.Items = append([]DriveSourceItem(nil), page.Items...)
	return page
}

func cloneGitHubIssuePage(page GitHubIssuePage) GitHubIssuePage {
	page.Issues = append([]GitHubIssue(nil), page.Issues...)
	return page
}

func cloneSafeWriteReceipt(receipt SafeWriteReceipt) SafeWriteReceipt {
	receipt.Labels = append([]GitHubLabel(nil), receipt.Labels...)
	return receipt
}
