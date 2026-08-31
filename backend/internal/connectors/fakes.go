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
	RemovedFiles     map[string]time.Time
	Cursors          map[string]DriveSyncState
	SyncRuns         map[string]MemorySyncRun
	HasRevisionError error
	PutRevisionError error
	SaveCursorError  error
}

func NewMemoryDriveRevisionStore() *MemoryDriveRevisionStore {
	return &MemoryDriveRevisionStore{
		Revisions:    make(map[string]DriveSourceItem),
		RemovedFiles: make(map[string]time.Time),
		Cursors:      make(map[string]DriveSyncState),
		SyncRuns:     make(map[string]MemorySyncRun),
	}
}

type MemorySyncRun struct {
	ID           string
	Provider     string
	WorkspaceID  string
	Target       string
	CursorBefore string
	CursorAfter  string
	Status       string
	Summary      SyncRunSummary
	ErrorCode    string
	StartedAt    time.Time
	CompletedAt  time.Time
}

func (s *MemoryDriveRevisionStore) StartSyncRun(ctx context.Context, provider, workspaceID, target, cursorBefore string, startedAt time.Time) (string, error) {
	if err := contextError(ctx); err != nil {
		return "", err
	}
	if strings.TrimSpace(workspaceID) == "" {
		return "", ErrInvalidWorkspaceID
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	id := newSyncRunID(provider, workspaceID, target, startedAt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SyncRuns == nil {
		s.SyncRuns = make(map[string]MemorySyncRun)
	}
	s.SyncRuns[id] = MemorySyncRun{ID: id, Provider: provider, WorkspaceID: workspaceID, Target: target, CursorBefore: cursorBefore, Status: "running", StartedAt: startedAt}
	return id, nil
}

func (s *MemoryDriveRevisionStore) CompleteSyncRun(ctx context.Context, runID, status string, summary SyncRunSummary, cursorAfter, errorCode string, completedAt time.Time) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.SyncRuns[runID]
	if !ok {
		return ErrNotFound
	}
	if run.Status != "running" {
		return ErrNotFound
	}
	run.Status, run.Summary, run.CursorAfter, run.ErrorCode, run.CompletedAt = status, summary, cursorAfter, errorCode, completedAt
	s.SyncRuns[runID] = run
	return nil
}

func (s *MemoryDriveRevisionStore) HasRevision(ctx context.Context, revisionKey string) (bool, error) {
	if err := contextError(ctx); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.HasRevisionError != nil {
		return false, s.HasRevisionError
	}
	_, exists := s.Revisions[revisionKey]
	return exists, nil
}

func (s *MemoryDriveRevisionStore) PutRevision(ctx context.Context, item DriveSourceItem) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := item.Validate(); err != nil {
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
	key := item.RevisionKey()
	if _, exists := s.Revisions[key]; exists {
		return ErrRevisionAlreadyExists
	}
	s.Revisions[key] = item
	if s.RemovedFiles != nil {
		delete(s.RemovedFiles, strings.TrimSpace(item.FileID))
	}
	return nil
}

func (s *MemoryDriveRevisionStore) MarkRemoved(ctx context.Context, fileID string, removedAt time.Time) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return ErrInvalidRevisionIdentity
	}
	if removedAt.IsZero() {
		removedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.RemovedFiles == nil {
		s.RemovedFiles = make(map[string]time.Time)
	}
	s.RemovedFiles[fileID] = removedAt.UTC()
	return nil
}

func (s *MemoryDriveRevisionStore) RemovedAt(fileID string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	removedAt, exists := s.RemovedFiles[strings.TrimSpace(fileID)]
	return removedAt, exists
}

func (s *MemoryDriveRevisionStore) SaveCursor(ctx context.Context, state DriveSyncState) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if state.WorkspaceID == "" {
		return ErrInvalidWorkspaceID
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

func (s *MemoryDriveRevisionStore) LoadCursor(ctx context.Context, workspaceID string) (DriveSyncState, bool, error) {
	if err := contextError(ctx); err != nil {
		return DriveSyncState{}, false, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return DriveSyncState{}, false, ErrInvalidWorkspaceID
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, exists := s.Cursors[workspaceID]
	return state, exists, nil
}

func (s *MemoryDriveRevisionStore) RevisionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Revisions)
}

func (s *MemoryDriveRevisionStore) Cursor(workspaceID string) (DriveSyncState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, exists := s.Cursors[workspaceID]
	return state, exists
}

func (s *MemoryDriveRevisionStore) SyncRunCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.SyncRuns)
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

type MemoryGitHubRevisionStore struct {
	mu               sync.Mutex
	Revisions        map[string]GitHubImportItem
	Cursors          map[string]GitHubSyncState
	SyncRuns         map[string]MemorySyncRun
	HasRevisionError error
	PutRevisionError error
	SaveCursorError  error
}

func NewMemoryGitHubRevisionStore() *MemoryGitHubRevisionStore {
	return &MemoryGitHubRevisionStore{
		Revisions: make(map[string]GitHubImportItem),
		Cursors:   make(map[string]GitHubSyncState),
		SyncRuns:  make(map[string]MemorySyncRun),
	}
}

func (s *MemoryGitHubRevisionStore) StartSyncRun(ctx context.Context, provider, workspaceID, target, cursorBefore string, startedAt time.Time) (string, error) {
	if err := contextError(ctx); err != nil {
		return "", err
	}
	if strings.TrimSpace(workspaceID) == "" {
		return "", ErrInvalidWorkspaceID
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	id := newSyncRunID(provider, workspaceID, target, startedAt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SyncRuns == nil {
		s.SyncRuns = make(map[string]MemorySyncRun)
	}
	s.SyncRuns[id] = MemorySyncRun{ID: id, Provider: provider, WorkspaceID: workspaceID, Target: target, CursorBefore: cursorBefore, Status: "running", StartedAt: startedAt}
	return id, nil
}

func (s *MemoryGitHubRevisionStore) CompleteSyncRun(ctx context.Context, runID, status string, summary SyncRunSummary, cursorAfter, errorCode string, completedAt time.Time) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.SyncRuns[runID]
	if !ok {
		return ErrNotFound
	}
	if run.Status != "running" {
		return ErrNotFound
	}
	run.Status, run.Summary, run.CursorAfter, run.ErrorCode, run.CompletedAt = status, summary, cursorAfter, errorCode, completedAt
	s.SyncRuns[runID] = run
	return nil
}

func (s *MemoryGitHubRevisionStore) HasRevision(ctx context.Context, revisionKey string) (bool, error) {
	if err := contextError(ctx); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.HasRevisionError != nil {
		return false, s.HasRevisionError
	}
	_, exists := s.Revisions[revisionKey]
	return exists, nil
}

func (s *MemoryGitHubRevisionStore) PutRevision(ctx context.Context, item GitHubImportItem) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := item.Validate(); err != nil {
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
	key := item.RevisionKey()
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
	if state.WorkspaceID == "" {
		return ErrInvalidWorkspaceID
	}
	if normalizeRepository(state.Repository) == "" {
		return ErrInvalidRepository
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SaveCursorError != nil {
		return s.SaveCursorError
	}
	if s.Cursors == nil {
		s.Cursors = make(map[string]GitHubSyncState)
	}
	s.Cursors[state.WorkspaceID+":"+normalizeRepository(state.Repository)] = state
	return nil
}

func (s *MemoryGitHubRevisionStore) LoadCursor(ctx context.Context, workspaceID, repository string) (GitHubSyncState, bool, error) {
	if err := contextError(ctx); err != nil {
		return GitHubSyncState{}, false, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	repository = normalizeRepository(repository)
	if workspaceID == "" {
		return GitHubSyncState{}, false, ErrInvalidWorkspaceID
	}
	if repository == "" {
		return GitHubSyncState{}, false, ErrInvalidRepository
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, exists := s.Cursors[workspaceID+":"+repository]
	return state, exists, nil
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

func (s *MemoryGitHubRevisionStore) SyncRunCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.SyncRuns)
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
	if stringsTrimEmpty(challenge.ID) || stringsTrimEmpty(challenge.UserID) || stringsTrimEmpty(challenge.TargetType) || stringsTrimEmpty(challenge.TargetID) || stringsTrimEmpty(challenge.ActionHash) || challenge.CreatedAt.IsZero() || challenge.ExpiresAt.IsZero() || !challenge.ExpiresAt.After(challenge.CreatedAt) || challenge.ExpiresAt.Sub(challenge.CreatedAt) > SafeWriteChallengeTTL {
		return ErrInvalidChallenge
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Challenges == nil {
		s.Challenges = make(map[string]SafeWriteChallenge)
	}
	s.Challenges[challenge.ID] = challenge
	return nil
}

func (s *MemorySafeWriteChallengeStore) Get(ctx context.Context, workspaceID, userID, challengeID string) (SafeWriteChallenge, error) {
	if err := contextError(ctx); err != nil {
		return SafeWriteChallenge{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, exists := s.Challenges[strings.TrimSpace(challengeID)]
	if !exists || challenge.UserID != strings.TrimSpace(userID) || (strings.TrimSpace(workspaceID) != "" && challenge.WorkspaceID != strings.TrimSpace(workspaceID)) {
		return SafeWriteChallenge{}, ErrInvalidChallenge
	}
	return challenge, nil
}

func (s *MemorySafeWriteChallengeStore) Consume(ctx context.Context, metadata SafeWriteMetadata, target SafeWriteTarget, now time.Time) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, exists := s.Challenges[metadata.ChallengeID]
	if !exists {
		return ErrInvalidChallenge
	}
	if challenge.UsedAt != nil {
		return ErrChallengeUsed
	}
	if !challenge.ExpiresAt.After(now) {
		return ErrChallengeExpired
	}
	targetType, targetID := target.TargetTypeAndID()
	if challenge.UserID != metadata.UserID || (metadata.WorkspaceID != "" && challenge.WorkspaceID != metadata.WorkspaceID) || challenge.TargetType != targetType || challenge.TargetID != targetID || challenge.ActionHash != metadata.ActionHash {
		return ErrInvalidChallenge
	}
	if expected, err := target.ActionHash(); err != nil || expected != challenge.ActionHash {
		return ErrInvalidChallenge
	}
	usedAt := now.UTC()
	challenge.UsedAt = &usedAt
	s.Challenges[metadata.ChallengeID] = challenge
	return nil
}

type MemorySafeWriteReceiptStore struct {
	mu            sync.Mutex
	Receipts      map[string]SafeWriteReceipt
	ReserveError  error
	CompleteError error
	SaveError     error
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

func (s *MemorySafeWriteReceiptStore) Save(ctx context.Context, receipt SafeWriteReceipt) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if stringsTrimEmpty(receipt.ID) || stringsTrimEmpty(receipt.UserID) || stringsTrimEmpty(receipt.IdempotencyKey) || stringsTrimEmpty(receipt.ActionHash) {
		return ErrReceiptPersistence
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SaveError != nil {
		return s.SaveError
	}
	if s.Receipts == nil {
		s.Receipts = make(map[string]SafeWriteReceipt)
	}
	key := receiptStoreKey(receipt.WorkspaceID, receipt.UserID, receipt.Operation, receipt.IdempotencyKey)
	if previous, exists := s.Receipts[key]; exists {
		if previous.ActionHash != receipt.ActionHash {
			return ErrIdempotencyConflict
		}
		return nil
	}
	s.Receipts[key] = cloneSafeWriteReceipt(receipt)
	return nil
}

func (s *MemorySafeWriteReceiptStore) Reserve(ctx context.Context, receipt SafeWriteReceipt) (SafeWriteReceipt, bool, error) {
	if err := contextError(ctx); err != nil {
		return SafeWriteReceipt{}, false, err
	}
	if stringsTrimEmpty(receipt.ID) || stringsTrimEmpty(receipt.UserID) || stringsTrimEmpty(receipt.IdempotencyKey) || stringsTrimEmpty(receipt.ActionHash) || receipt.Status != SafeWriteReceiptPending {
		return SafeWriteReceipt{}, false, ErrReceiptPersistence
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ReserveError != nil {
		return SafeWriteReceipt{}, false, s.ReserveError
	}
	if s.Receipts == nil {
		s.Receipts = make(map[string]SafeWriteReceipt)
	}
	key := receiptStoreKey(receipt.WorkspaceID, receipt.UserID, receipt.Operation, receipt.IdempotencyKey)
	if previous, exists := s.Receipts[key]; exists {
		return cloneSafeWriteReceipt(previous), true, nil
	}
	s.Receipts[key] = cloneSafeWriteReceipt(receipt)
	return cloneSafeWriteReceipt(receipt), false, nil
}

func (s *MemorySafeWriteReceiptStore) Complete(ctx context.Context, receipt SafeWriteReceipt) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if stringsTrimEmpty(receipt.ID) || stringsTrimEmpty(receipt.UserID) || stringsTrimEmpty(receipt.IdempotencyKey) || stringsTrimEmpty(receipt.ActionHash) || receipt.Status != SafeWriteReceiptAccepted {
		return ErrReceiptPersistence
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.CompleteError != nil {
		return s.CompleteError
	}
	key := receiptStoreKey(receipt.WorkspaceID, receipt.UserID, receipt.Operation, receipt.IdempotencyKey)
	previous, exists := s.Receipts[key]
	if !exists {
		return ErrReceiptPersistence
	}
	if previous.ActionHash != receipt.ActionHash || previous.ID != receipt.ID {
		return ErrIdempotencyConflict
	}
	if previous.Status == SafeWriteReceiptAccepted {
		return nil
	}
	if previous.Status != SafeWriteReceiptPending {
		return ErrReceiptPersistence
	}
	s.Receipts[key] = cloneSafeWriteReceipt(receipt)
	return nil
}

func receiptStoreKey(workspaceID, userID string, operation SafeWriteOperation, idempotencyKey string) string {
	return workspaceID + "\x00" + userID + "\x00" + string(operation) + "\x00" + idempotencyKey
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
