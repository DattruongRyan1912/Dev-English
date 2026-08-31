package connectors

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

const (
	defaultConnectorPageSize = 100
	maxConnectorPageSize     = 1000
	githubRestartCursor      = "1"
)

var syncRunSequence uint64

// DriveSyncRequest describes one bounded incremental page. Cursor tokens are
// opaque and must only come from DriveReader or a previously committed state.
type DriveSyncRequest struct {
	WorkspaceID string
	Cursor      DriveCursor
	PageSize    int
}

// DriveSyncState is persisted only after every item in a page has been
// accepted by the revision sink. A failed page must be retried with its old
// cursor.
type DriveSyncState struct {
	WorkspaceID string
	Cursor      DriveCursor
	HasMore     bool
	LastSynced  time.Time
}

// DriveRevisionStore is the application-facing sink for normalized Drive
// revisions. Implementations must enforce a unique revision key so a
// concurrent or replayed page remains idempotent.
type DriveRevisionStore interface {
	HasRevision(ctx context.Context, revisionKey string) (bool, error)
	PutRevision(ctx context.Context, item DriveSourceItem) error
	SaveCursor(ctx context.Context, state DriveSyncState) error
}

// DriveRemovalStore is an optional but production-required capability for
// changes-feed consumers. A provider removal must hide the source item from
// retrieval while preserving its immutable revisions for audit/history.
type DriveRemovalStore interface {
	MarkRemoved(ctx context.Context, fileID string, removedAt time.Time) error
}

// DriveCursorReader is an optional capability for durable stores. When a
// caller omits a cursor, the sync service can resume from the last committed
// checkpoint instead of silently starting a second full page walk.
type DriveCursorReader interface {
	LoadCursor(ctx context.Context, workspaceID string) (DriveSyncState, bool, error)
}

// SyncRunSummary is the bounded, non-sensitive evidence recorded for one
// connector page. It intentionally contains counts and status only, never
// provider payloads or credentials.
type SyncRunSummary struct {
	Seen     int
	Upserted int
	Skipped  int
}

// SyncRunRecorder is an optional durable audit capability. A sync service
// records a running row before provider I/O and closes it after the page has
// either committed or failed.
type SyncRunRecorder interface {
	StartSyncRun(ctx context.Context, provider, workspaceID, target, cursorBefore string, startedAt time.Time) (string, error)
	CompleteSyncRun(ctx context.Context, runID, status string, summary SyncRunSummary, cursorAfter, errorCode string, completedAt time.Time) error
}

// DriveSyncService coordinates read-only Drive pages and a revision sink.
// It never receives or stores provider credentials.
type DriveSyncService struct {
	Reader DriveReader
	Store  DriveRevisionStore
	Clock  func() time.Time
}

func NewDriveSyncService(reader DriveReader, store DriveRevisionStore) (*DriveSyncService, error) {
	if reader == nil || store == nil {
		return nil, errors.New("drive sync reader and store are required")
	}
	return &DriveSyncService{Reader: reader, Store: store, Clock: time.Now}, nil
}

// Sync processes exactly one provider page. It intentionally does not loop
// through all pages: callers can checkpoint and schedule each continuation
// independently.
func (s *DriveSyncService) Sync(ctx context.Context, request DriveSyncRequest) (result DriveSyncResult, err error) {
	if err := validateSyncRequest(request.WorkspaceID, request.Cursor.Token, request.PageSize); err != nil {
		return result, err
	}
	workspaceID := strings.TrimSpace(request.WorkspaceID)
	pageSize := normalizedPageSize(request.PageSize)
	cursor := normalizedCursor(request.Cursor)
	if cursor.Token == "" {
		if reader, ok := s.Store.(DriveCursorReader); ok {
			checkpoint, found, err := reader.LoadCursor(ctx, workspaceID)
			if err != nil {
				return result, wrapConnectorFailure(ProviderGoogleDrive, "load cursor", err)
			}
			if found {
				if strings.TrimSpace(checkpoint.WorkspaceID) != workspaceID ||
					(checkpoint.HasMore && !checkpoint.Cursor.Valid()) {
					return result, ErrInvalidSyncState
				}
				cursor = normalizedCursor(checkpoint.Cursor)
			}
		}
	}
	clock := s.Clock
	if clock == nil {
		clock = time.Now
	}
	cursorAfter := cursor.Token
	var runID string
	var recorder SyncRunRecorder
	if candidate, ok := s.Store.(SyncRunRecorder); ok {
		recorder = candidate
		runID, err = recorder.StartSyncRun(ctx, ProviderGoogleDrive, workspaceID, "", cursor.Token, clock().UTC())
		if err != nil {
			return result, wrapConnectorFailure(ProviderGoogleDrive, "start sync run", err)
		}
		defer func() {
			status := "succeeded"
			errorCode := ""
			if err != nil {
				status = "failed"
				errorCode = syncRunErrorCode(err)
			}
			completeErr := recorder.CompleteSyncRun(ctx, runID, status, SyncRunSummary{
				Seen: result.Seen, Upserted: result.Upserted, Skipped: result.Skipped,
			}, cursorAfter, errorCode, clock().UTC())
			if err == nil && completeErr != nil {
				err = wrapConnectorFailure(ProviderGoogleDrive, "complete sync run", completeErr)
			}
		}()
	}
	page, err := s.Reader.List(ctx, DriveListRequest{
		WorkspaceID: workspaceID,
		Cursor:      cursor,
		PageSize:    pageSize,
	})
	if err != nil {
		return result, wrapConnectorFailure(ProviderGoogleDrive, "list", err)
	}
	if page.HasMore && !page.NextCursor.Valid() {
		return result, ErrInvalidCursor
	}

	result.Seen = len(page.Items)
	unique, skipped, err := DeduplicateDriveItems(page.Items)
	if err != nil {
		return result, err
	}
	result.Skipped = skipped
	for _, item := range unique {
		if item.Removed {
			removalStore, ok := s.Store.(DriveRemovalStore)
			if !ok {
				return result, ErrRemovalNotSupported
			}
			if removeErr := removalStore.MarkRemoved(ctx, strings.TrimSpace(item.FileID), clock().UTC()); removeErr != nil {
				return result, wrapConnectorFailure(ProviderGoogleDrive, "mark removed", removeErr)
			}
			result.Upserted++
			continue
		}
		exists, existsErr := s.Store.HasRevision(ctx, item.RevisionKey())
		if existsErr != nil {
			return result, wrapConnectorFailure(ProviderGoogleDrive, "check revision", existsErr)
		}
		if exists {
			result.Skipped++
			continue
		}
		if putErr := s.Store.PutRevision(ctx, item); putErr != nil {
			if errors.Is(putErr, ErrRevisionAlreadyExists) {
				result.Skipped++
				continue
			}
			return result, wrapConnectorFailure(ProviderGoogleDrive, "store revision", putErr)
		}
		result.Upserted++
	}

	result.HasMore = page.HasMore
	checkpointCursor := normalizedCursor(page.NextCursor)
	if page.HasMore {
		result.NextCursor = checkpointCursor
	} else if page.CheckpointCursor.Valid() {
		checkpointCursor = normalizedCursor(page.CheckpointCursor)
	}
	cursorAfter = checkpointCursor.Token
	state := DriveSyncState{
		WorkspaceID: workspaceID,
		Cursor:      checkpointCursor,
		HasMore:     result.HasMore,
		LastSynced:  clock().UTC(),
	}
	if saveErr := s.Store.SaveCursor(ctx, state); saveErr != nil {
		return result, wrapConnectorFailure(ProviderGoogleDrive, "save cursor", saveErr)
	}
	return result, nil
}

// GitHubImportItem is a normalized, revision-addressable read model. V1
// imports issues; comments can be added later without changing the sync
// protocol because Kind and ExternalID are already part of the identity.
type GitHubImportItem struct {
	Repository string
	Kind       string
	Number     int64
	ExternalID int64
	Title      string
	Body       string
	State      string
	SourceURL  string
	Revision   string
	UpdatedAt  time.Time
}

// GitHubSourceItem is kept as a semantic alias for connector callers that
// model imported records as source items.
type GitHubSourceItem = GitHubImportItem

func (i GitHubImportItem) Validate() error {
	if normalizeRepository(i.Repository) == "" || strings.TrimSpace(i.Kind) == "" || strings.TrimSpace(i.Revision) == "" {
		return ErrInvalidRevisionIdentity
	}
	if i.Number < 1 && i.ExternalID < 1 {
		return ErrInvalidRevisionIdentity
	}
	return nil
}

func (i GitHubImportItem) RevisionKey() string {
	identity := i.ExternalID
	if identity < 1 {
		identity = i.Number
	}
	return strings.Join([]string{
		ProviderGitHub,
		normalizeRepository(i.Repository),
		strings.ToLower(strings.TrimSpace(i.Kind)),
		fmt.Sprintf("%d", identity),
		strings.TrimSpace(i.Revision),
	}, ":")
}

func NewGitHubImportItem(issue GitHubIssue) (GitHubImportItem, error) {
	if err := validateGitHubIssue(issue); err != nil {
		return GitHubImportItem{}, err
	}
	return GitHubImportItem{
		Repository: issue.Repository,
		Kind:       "issue",
		Number:     issue.Number,
		Title:      issue.Title,
		Body:       issue.Body,
		State:      issue.State,
		Revision:   issue.Revision,
		UpdatedAt:  issue.UpdatedAt,
	}, nil
}

type GitHubImportRequest struct {
	WorkspaceID string
	Repository  string
	Cursor      string
	PageSize    int
}

type GitHubImportPage struct {
	Items      []GitHubImportItem
	NextCursor string
	HasMore    bool
}

type GitHubImportResult struct {
	Seen       int
	Upserted   int
	Skipped    int
	NextCursor string
	HasMore    bool
}

type GitHubSyncState struct {
	WorkspaceID string
	Repository  string
	Cursor      string
	HasMore     bool
	LastSynced  time.Time
}

type GitHubRevisionStore interface {
	HasRevision(ctx context.Context, revisionKey string) (bool, error)
	PutRevision(ctx context.Context, item GitHubImportItem) error
	SaveCursor(ctx context.Context, state GitHubSyncState) error
}

// GitHubCursorReader is the repository capability used to resume a bounded
// issue import when the request does not carry an explicit page cursor.
type GitHubCursorReader interface {
	LoadCursor(ctx context.Context, workspaceID, repository string) (GitHubSyncState, bool, error)
}

// GitHubImportService adapts the existing read-only issue client into an
// incremental, idempotent source sync. It never invokes GitHubSafeWriter.
type GitHubImportService struct {
	Reader GitHubReadClient
	Store  GitHubRevisionStore
	Clock  func() time.Time
}

func NewGitHubImportService(reader GitHubReadClient, store GitHubRevisionStore) (*GitHubImportService, error) {
	if reader == nil || store == nil {
		return nil, errors.New("GitHub import reader and store are required")
	}
	return &GitHubImportService{Reader: reader, Store: store, Clock: time.Now}, nil
}

func (s *GitHubImportService) SyncIssues(ctx context.Context, request GitHubImportRequest) (result GitHubImportResult, err error) {
	if err := validateSyncRequest(request.WorkspaceID, request.Cursor, request.PageSize); err != nil {
		return result, err
	}
	workspaceID := strings.TrimSpace(request.WorkspaceID)
	repository := normalizeRepository(request.Repository)
	if repository == "" {
		return result, ErrInvalidRepository
	}
	cursor := strings.TrimSpace(request.Cursor)
	if cursor == "" {
		if reader, ok := s.Store.(GitHubCursorReader); ok {
			checkpoint, found, err := reader.LoadCursor(ctx, workspaceID, repository)
			if err != nil {
				return result, wrapConnectorFailure(ProviderGitHub, "load cursor", err)
			}
			if found {
				if strings.TrimSpace(checkpoint.WorkspaceID) != workspaceID ||
					normalizeRepository(checkpoint.Repository) != repository ||
					(checkpoint.HasMore && strings.TrimSpace(checkpoint.Cursor) == "") {
					return result, ErrInvalidSyncState
				}
				cursor = strings.TrimSpace(checkpoint.Cursor)
			}
		}
	}
	clock := s.Clock
	if clock == nil {
		clock = time.Now
	}
	cursorAfter := cursor
	var runID string
	var recorder SyncRunRecorder
	if candidate, ok := s.Store.(SyncRunRecorder); ok {
		recorder = candidate
		runID, err = recorder.StartSyncRun(ctx, ProviderGitHub, workspaceID, repository, cursor, clock().UTC())
		if err != nil {
			return result, wrapConnectorFailure(ProviderGitHub, "start sync run", err)
		}
		defer func() {
			status := "succeeded"
			errorCode := ""
			if err != nil {
				status = "failed"
				errorCode = syncRunErrorCode(err)
			}
			completeErr := recorder.CompleteSyncRun(ctx, runID, status, SyncRunSummary{
				Seen: result.Seen, Upserted: result.Upserted, Skipped: result.Skipped,
			}, cursorAfter, errorCode, clock().UTC())
			if err == nil && completeErr != nil {
				err = wrapConnectorFailure(ProviderGitHub, "complete sync run", completeErr)
			}
		}()
	}
	page, err := s.Reader.ListIssues(ctx, GitHubIssueListRequest{
		Repository: request.Repository,
		Cursor:     cursor,
		PageSize:   normalizedPageSize(request.PageSize),
	})
	if err != nil {
		return result, wrapConnectorFailure(ProviderGitHub, "list issues", err)
	}
	if page.HasMore && strings.TrimSpace(page.NextCursor) == "" {
		return result, ErrInvalidCursor
	}

	result.Seen = len(page.Issues)
	items := make([]GitHubImportItem, 0, len(page.Issues))
	for _, issue := range page.Issues {
		item, itemErr := NewGitHubImportItem(issue)
		if itemErr != nil {
			return result, itemErr
		}
		items = append(items, item)
	}
	unique, skipped, err := DeduplicateGitHubItems(items)
	if err != nil {
		return result, err
	}
	result.Skipped = skipped
	for _, item := range unique {
		exists, existsErr := s.Store.HasRevision(ctx, item.RevisionKey())
		if existsErr != nil {
			return result, wrapConnectorFailure(ProviderGitHub, "check revision", existsErr)
		}
		if exists {
			result.Skipped++
			continue
		}
		if putErr := s.Store.PutRevision(ctx, item); putErr != nil {
			if errors.Is(putErr, ErrRevisionAlreadyExists) {
				result.Skipped++
				continue
			}
			return result, wrapConnectorFailure(ProviderGitHub, "store revision", putErr)
		}
		result.Upserted++
	}
	result.HasMore = page.HasMore
	if page.HasMore {
		result.NextCursor = strings.TrimSpace(page.NextCursor)
	}
	// GitHub's issues API is page-based and has no changes token. Once a full
	// scan completes, persist an explicit page-one restart checkpoint rather
	// than an empty cursor that hides the fact that the next run is a full
	// rescan. This remains safe for updates that move or appear on page one.
	cursorAfter = result.NextCursor
	if !result.HasMore {
		cursorAfter = githubRestartCursor
	}
	if saveErr := s.Store.SaveCursor(ctx, GitHubSyncState{
		WorkspaceID: workspaceID,
		Repository:  repository,
		Cursor:      cursorAfter,
		HasMore:     result.HasMore,
		LastSynced:  clock().UTC(),
	}); saveErr != nil {
		return result, wrapConnectorFailure(ProviderGitHub, "save cursor", saveErr)
	}
	return result, nil
}

// GetIssue is a read-only import operation for detail views and manual
// ingestion. It uses the same normalization and revision validation as sync.
func (s *GitHubImportService) GetIssue(ctx context.Context, repository string, number int64) (GitHubImportItem, error) {
	if normalizeRepository(repository) == "" || number < 1 {
		return GitHubImportItem{}, ErrInvalidRepository
	}
	issue, err := s.Reader.GetIssue(ctx, repository, number)
	if err != nil {
		return GitHubImportItem{}, wrapConnectorFailure(ProviderGitHub, "get issue", err)
	}
	return NewGitHubImportItem(issue)
}

func DeduplicateGitHubItems(items []GitHubImportItem) ([]GitHubImportItem, int, error) {
	seen := make(map[string]struct{}, len(items))
	unique := make([]GitHubImportItem, 0, len(items))
	skipped := 0
	for _, item := range items {
		if err := item.Validate(); err != nil {
			return nil, 0, err
		}
		key := item.RevisionKey()
		if _, exists := seen[key]; exists {
			skipped++
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, item)
	}
	return unique, skipped, nil
}

func validateGitHubIssue(issue GitHubIssue) error {
	if normalizeRepository(issue.Repository) == "" || issue.Number < 1 || strings.TrimSpace(issue.Revision) == "" {
		return ErrInvalidRevisionIdentity
	}
	return nil
}

func validateSyncRequest(workspaceID, cursor string, pageSize int) error {
	if strings.TrimSpace(workspaceID) == "" {
		return ErrInvalidWorkspaceID
	}
	if strings.TrimSpace(cursor) == "" && cursor != "" {
		return ErrInvalidCursor
	}
	if pageSize < 0 || pageSize > maxConnectorPageSize {
		return ErrInvalidPageSize
	}
	return nil
}

func syncRunErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrInvalidCursor):
		return "invalid_cursor"
	case errors.Is(err, ErrInvalidSyncState):
		return "invalid_sync_state"
	case errors.Is(err, ErrInvalidRevisionIdentity):
		return "invalid_revision"
	case errors.Is(err, ErrRevisionAlreadyExists):
		return "duplicate_revision"
	default:
		return "sync_failed"
	}
}

func newSyncRunID(provider, workspaceID, target string, startedAt time.Time) string {
	sequence := atomic.AddUint64(&syncRunSequence, 1)
	return stableConnectorID("sync-run", fmt.Sprintf("%s:%s:%s:%d:%d", provider, workspaceID, target, startedAt.UnixNano(), sequence))
}

func normalizedPageSize(pageSize int) int {
	if pageSize == 0 {
		return defaultConnectorPageSize
	}
	return pageSize
}

func normalizedCursor(cursor DriveCursor) DriveCursor {
	return DriveCursor{Token: strings.TrimSpace(cursor.Token)}
}

func normalizeRepository(repository string) string {
	value := strings.ToLower(strings.TrimSpace(repository))
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.Contains(value, " ") {
		return ""
	}
	return value
}

// wrapConnectorFailure preserves errors.Is/errors.As while ensuring an
// arbitrary provider error cannot leak a token or response body.
func wrapConnectorFailure(provider, operation string, err error) error {
	if err == nil {
		return nil
	}
	return &redactedWrappedError{
		message: fmt.Sprintf("%s %s: %s", provider, operation, RedactSecrets(err.Error())),
		cause:   err,
	}
}

type redactedWrappedError struct {
	message string
	cause   error
}

func (e *redactedWrappedError) Error() string { return e.message }
func (e *redactedWrappedError) Unwrap() error { return e.cause }
