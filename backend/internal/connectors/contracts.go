package connectors

import (
	"context"
	"strings"
	"time"
)

const (
	ProviderGoogleDrive = "google_drive"
	ProviderGitHub      = "github"
)

// DriveCursor is opaque to application services and is only advanced with a
// provider-issued continuation token.
type DriveCursor struct {
	Token string
}

func (c DriveCursor) Valid() bool {
	return strings.TrimSpace(c.Token) != ""
}

// DriveSourceItem is a read-only, revision-addressable Drive item. Content is
// already normalized by the connector; application services never receive an
// access token or provider response body.
type DriveSourceItem struct {
	FileID       string
	Name         string
	MIMEType     string
	WebURL       string
	RevisionID   string
	ContentHash  string
	ModifiedTime time.Time
	Text         string
}

func (i DriveSourceItem) Validate() error {
	if strings.TrimSpace(i.FileID) == "" || strings.TrimSpace(i.RevisionID) == "" {
		return ErrInvalidRevisionIdentity
	}
	return nil
}

func (i DriveSourceItem) RevisionKey() string {
	return ProviderGoogleDrive + ":" + strings.TrimSpace(i.FileID) + ":" + strings.TrimSpace(i.RevisionID)
}

type DriveListRequest struct {
	WorkspaceID string
	Cursor      DriveCursor
	PageSize    int
}

type DrivePage struct {
	Items      []DriveSourceItem
	NextCursor DriveCursor
	HasMore    bool
}

// DriveReader is deliberately read-only. Sync callers must persist NextCursor
// only after successfully processing every item in the page.
type DriveReader interface {
	List(ctx context.Context, request DriveListRequest) (DrivePage, error)
}

type DriveSyncResult struct {
	Seen       int
	Upserted   int
	Skipped    int
	NextCursor DriveCursor
	HasMore    bool
}

// DeduplicateDriveItems gives a sync service an idempotent page primitive. A
// repeated provider revision is skipped, while an item without stable
// identity fails closed instead of creating a fabricated source revision.
func DeduplicateDriveItems(items []DriveSourceItem) ([]DriveSourceItem, int, error) {
	seen := make(map[string]struct{}, len(items))
	unique := make([]DriveSourceItem, 0, len(items))
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

// GitHubIssue, GitHubComment and GitHubLabel are provider-neutral read models.
type GitHubIssue struct {
	Repository string
	Number     int64
	Title      string
	Body       string
	State      string
	Revision   string
	UpdatedAt  time.Time
}

type GitHubComment struct {
	Repository string
	Issue      int64
	ID         int64
	Body       string
	Revision   string
	UpdatedAt  time.Time
}

type GitHubLabel struct {
	Repository string
	Issue      int64
	Name       string
}

type GitHubIssueListRequest struct {
	Repository string
	Cursor     string
	PageSize   int
}

type GitHubIssuePage struct {
	Issues     []GitHubIssue
	NextCursor string
	HasMore    bool
}

type GitHubReadClient interface {
	ListIssues(ctx context.Context, request GitHubIssueListRequest) (GitHubIssuePage, error)
	GetIssue(ctx context.Context, repository string, number int64) (GitHubIssue, error)
}

type SafeWriteMetadata struct {
	WorkspaceID      string
	IdempotencyKey   string
	ExpectedRevision string
	// The following fields are confirmation data issued by the application
	// layer. The guarded service requires them before a provider mutation is
	// allowed.
	UserID      string
	ChallengeID string
	ActionHash  string
	ConfirmedAt time.Time
	ExpiresAt   time.Time
}

func (m SafeWriteMetadata) Validate() error {
	if strings.TrimSpace(m.IdempotencyKey) == "" {
		return ErrMissingIdempotencyKey
	}
	if strings.TrimSpace(m.WorkspaceID) == "" {
		return ErrInvalidWorkspaceID
	}
	return nil
}

type CreateIssueRequest struct {
	Repository string
	Title      string
	Body       string
	Metadata   SafeWriteMetadata
}

type CommentIssueRequest struct {
	Repository string
	Issue      int64
	Body       string
	Metadata   SafeWriteMetadata
}

type LabelIssueRequest struct {
	Repository string
	Issue      int64
	Labels     []string
	Metadata   SafeWriteMetadata
}

// GitHubSafeWriter is limited to the V1 issue/comment/label mutations. The
// application service must validate challenge confirmation before invoking it.
type GitHubSafeWriter interface {
	CreateIssue(ctx context.Context, request CreateIssueRequest) (GitHubIssue, error)
	AddIssueComment(ctx context.Context, request CommentIssueRequest) (GitHubComment, error)
	SetIssueLabels(ctx context.Context, request LabelIssueRequest) ([]GitHubLabel, error)
}
