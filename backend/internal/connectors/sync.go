package connectors

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	defaultConnectorPageSize = 100
	maxConnectorPageSize     = 1000
)

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

func (s DriveSyncState) Validate() error {
	if strings.TrimSpace(s.WorkspaceID) == "" {
		return ErrInvalidWorkspaceID
	}
	if s.HasMore && !s.Cursor.Valid() {
		return ErrInvalidCursor
	}
	return nil
}

// DriveRevisionStore is the application-facing sink for normalized Drive
// revisions. Implementations must enforce a unique revision key within the
// supplied workspace so a replayed page cannot suppress another workspace.
type DriveRevisionStore interface {
	HasRevision(ctx context.Context, workspaceID, revisionKey string) (bool, error)
	PutRevision(ctx context.Context, workspaceID string, item DriveSourceItem) error
	SaveCursor(ctx context.Context, state DriveSyncState) error
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
func (s *DriveSyncService) Sync(ctx context.Context, request DriveSyncRequest) (DriveSyncResult, error) {
	var result DriveSyncResult
	if s == nil || s.Reader == nil || s.Store == nil {
		return result, errors.New("drive sync reader and store are required")
	}
	workspaceID := strings.TrimSpace(request.WorkspaceID)
	if err := validateSyncRequest(workspaceID, request.Cursor.Token, request.PageSize); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	pageSize := normalizedPageSize(request.PageSize)
	cursor := normalizedCursor(request.Cursor)
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
		exists, existsErr := s.Store.HasRevision(ctx, workspaceID, item.RevisionKey())
		if existsErr != nil {
			return result, wrapConnectorFailure(ProviderGoogleDrive, "check revision", existsErr)
		}
		if exists {
			result.Skipped++
			continue
		}
		if putErr := s.Store.PutRevision(ctx, workspaceID, item); putErr != nil {
			if errors.Is(putErr, ErrRevisionAlreadyExists) {
				result.Skipped++
				continue
			}
			return result, wrapConnectorFailure(ProviderGoogleDrive, "store revision", putErr)
		}
		result.Upserted++
	}

	result.HasMore = page.HasMore
	if page.HasMore {
		result.NextCursor = normalizedCursor(page.NextCursor)
	}
	state := DriveSyncState{
		WorkspaceID: workspaceID,
		Cursor:      result.NextCursor,
		HasMore:     result.HasMore,
		LastSynced:  syncNow(s.Clock),
	}
	if err := state.Validate(); err != nil {
		return result, err
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
		Repository: normalizeRepository(issue.Repository),
		Kind:       "issue",
		Number:     issue.Number,
		Title:      issue.Title,
		Body:       issue.Body,
		State:      issue.State,
		Revision:   strings.TrimSpace(issue.Revision),
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

func (s GitHubSyncState) Validate() error {
	if strings.TrimSpace(s.WorkspaceID) == "" {
		return ErrInvalidWorkspaceID
	}
	if normalizeRepository(s.Repository) == "" {
		return ErrInvalidRepository
	}
	if s.HasMore && strings.TrimSpace(s.Cursor) == "" {
		return ErrInvalidCursor
	}
	return nil
}

type GitHubRevisionStore interface {
	HasRevision(ctx context.Context, workspaceID, revisionKey string) (bool, error)
	PutRevision(ctx context.Context, workspaceID string, item GitHubImportItem) error
	SaveCursor(ctx context.Context, state GitHubSyncState) error
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

func (s *GitHubImportService) SyncIssues(ctx context.Context, request GitHubImportRequest) (GitHubImportResult, error) {
	var result GitHubImportResult
	if s == nil || s.Reader == nil || s.Store == nil {
		return result, errors.New("GitHub import reader and store are required")
	}
	workspaceID := strings.TrimSpace(request.WorkspaceID)
	if err := validateSyncRequest(workspaceID, request.Cursor, request.PageSize); err != nil {
		return result, err
	}
	repository := normalizeRepository(request.Repository)
	if repository == "" {
		return result, ErrInvalidRepository
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	page, err := s.Reader.ListIssues(ctx, GitHubIssueListRequest{
		Repository: repository,
		Cursor:     strings.TrimSpace(request.Cursor),
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
		exists, existsErr := s.Store.HasRevision(ctx, workspaceID, item.RevisionKey())
		if existsErr != nil {
			return result, wrapConnectorFailure(ProviderGitHub, "check revision", existsErr)
		}
		if exists {
			result.Skipped++
			continue
		}
		if putErr := s.Store.PutRevision(ctx, workspaceID, item); putErr != nil {
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
	state := GitHubSyncState{
		WorkspaceID: workspaceID,
		Repository:  repository,
		Cursor:      result.NextCursor,
		HasMore:     result.HasMore,
		LastSynced:  syncNow(s.Clock),
	}
	if err := state.Validate(); err != nil {
		return result, err
	}
	if saveErr := s.Store.SaveCursor(ctx, state); saveErr != nil {
		return result, wrapConnectorFailure(ProviderGitHub, "save cursor", saveErr)
	}
	return result, nil
}

// GetIssue is a read-only import operation for detail views and manual
// ingestion. It uses the same normalization and revision validation as sync.
func (s *GitHubImportService) GetIssue(ctx context.Context, repository string, number int64) (GitHubImportItem, error) {
	if s == nil || s.Reader == nil {
		return GitHubImportItem{}, errors.New("GitHub import reader is required")
	}
	normalizedRepository := normalizeRepository(repository)
	if normalizedRepository == "" || number < 1 {
		return GitHubImportItem{}, ErrInvalidRepository
	}
	if err := ctx.Err(); err != nil {
		return GitHubImportItem{}, err
	}
	issue, err := s.Reader.GetIssue(ctx, normalizedRepository, number)
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

func scopedRevisionKey(workspaceID, revisionKey string) (string, error) {
	workspace := strings.TrimSpace(workspaceID)
	key := strings.TrimSpace(revisionKey)
	if workspace == "" {
		return "", ErrInvalidWorkspaceID
	}
	if key == "" {
		return "", ErrInvalidRevisionIdentity
	}
	return workspace + "\x00" + key, nil
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

func syncNow(clock func() time.Time) time.Time {
	if clock == nil {
		clock = time.Now
	}
	return clock().UTC()
}

// wrapConnectorFailure preserves errors.Is/errors.As while ensuring an
// arbitrary provider error cannot leak a token or response body.
func wrapConnectorFailure(provider, operation string, err error) error {
	if err == nil {
		return nil
	}
	return &redactedError{
		message: fmt.Sprintf("%s %s: %s", RedactSecrets(provider), RedactSecrets(operation), RedactSecrets(err.Error())),
		cause:   err,
	}
}
