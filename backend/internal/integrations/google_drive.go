package integrations

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	defaultGoogleDriveAPIBase  = "https://www.googleapis.com/drive/v3"
	googleDriveHTTPTimeout     = 30 * time.Second
	maxGoogleDriveBodyBytes    = 512 << 10
	driveBootstrapCursorPrefix = "devenglish-drive-bootstrap-v1:"
)

// GoogleDriveReader is the read-only Drive connector used by the incremental
// connector service. It only keeps the access token in memory and never puts
// it in a provider error or a normalized knowledge record.
type GoogleDriveReader struct {
	APIBaseURL      string
	AccessToken     string
	TokenSource     oauth2.TokenSource
	Client          *http.Client
	MaxContentBytes int64
	// UseChangesAPI keeps the production connector on Drive's change feed.
	// It is opt-out only for the legacy files-list fixture and compatibility
	// tests; NewGoogleDriveFromEnv enables it by default.
	UseChangesAPI bool
}

func NewGoogleDriveFromEnv() *GoogleDriveReader {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("GOOGLE_DRIVE_API_BASE_URL")), "/")
	if base == "" {
		base = defaultGoogleDriveAPIBase
	}
	token := strings.TrimSpace(os.Getenv("GOOGLE_DRIVE_ACCESS_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("DRIVE_ACCESS_TOKEN"))
	}
	client := &http.Client{Timeout: googleDriveHTTPTimeout}
	var tokenSource oauth2.TokenSource
	refreshToken := strings.TrimSpace(os.Getenv("GOOGLE_DRIVE_REFRESH_TOKEN"))
	clientID := strings.TrimSpace(os.Getenv("GOOGLE_DRIVE_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("GOOGLE_DRIVE_CLIENT_SECRET"))
	if refreshToken != "" && clientID != "" && clientSecret != "" {
		config := &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     google.Endpoint,
			Scopes:       []string{"https://www.googleapis.com/auth/drive.readonly"},
		}
		tokenSource = newGoogleDriveTokenSource(config, refreshToken, client)
	}
	return &GoogleDriveReader{
		APIBaseURL:      base,
		AccessToken:     token,
		TokenSource:     tokenSource,
		Client:          client,
		MaxContentBytes: maxGoogleDriveBodyBytes,
		UseChangesAPI:   true,
	}
}

func newGoogleDriveTokenSource(config *oauth2.Config, refreshToken string, client *http.Client) oauth2.TokenSource {
	if config == nil || strings.TrimSpace(refreshToken) == "" {
		return nil
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &googleDriveTokenSource{
		config:       config,
		refreshToken: strings.TrimSpace(refreshToken),
		client:       client,
	}
}

// googleDriveTokenSource keeps refreshed credentials in memory while allowing
// each Drive request to propagate its cancellation context to the OAuth token
// endpoint. oauth2.TokenSource itself has no context-aware method, so the
// reader uses tokenContextSource when the concrete source supports it.
type googleDriveTokenSource struct {
	config       *oauth2.Config
	refreshToken string
	client       *http.Client

	mu    sync.Mutex
	token *oauth2.Token
}

func (s *googleDriveTokenSource) Token() (*oauth2.Token, error) {
	return s.TokenContext(context.Background())
}

func (s *googleDriveTokenSource) TokenContext(ctx context.Context) (*oauth2.Token, error) {
	if s == nil || s.config == nil || strings.TrimSpace(s.refreshToken) == "" {
		return nil, errors.New("Google Drive access token refresh is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != nil && s.token.Valid() {
		return s.token, nil
	}
	oauthContext := context.WithValue(ctx, oauth2.HTTPClient, s.client)
	token, err := s.config.TokenSource(oauthContext, &oauth2.Token{RefreshToken: s.refreshToken}).Token()
	if err != nil {
		return nil, err
	}
	if token == nil || strings.TrimSpace(token.AccessToken) == "" {
		return nil, errors.New("Google Drive access token refresh returned no token")
	}
	s.token = token
	return token, nil
}

type tokenContextSource interface {
	TokenContext(context.Context) (*oauth2.Token, error)
}

func (r *GoogleDriveReader) Configured() bool {
	return r != nil && (strings.TrimSpace(r.AccessToken) != "" || r.TokenSource != nil)
}

func (r *GoogleDriveReader) List(ctx context.Context, request connectors.DriveListRequest) (connectors.DrivePage, error) {
	if r == nil || strings.TrimSpace(r.APIBaseURL) == "" || !r.Configured() {
		return connectors.DrivePage{}, errors.New("Google Drive read connector is not configured")
	}
	if strings.TrimSpace(request.WorkspaceID) == "" {
		return connectors.DrivePage{}, connectors.ErrInvalidWorkspaceID
	}
	if request.PageSize < 1 || request.PageSize > 1000 {
		return connectors.DrivePage{}, connectors.ErrInvalidPageSize
	}
	if r.UseChangesAPI {
		return r.listChanges(ctx, request)
	}
	return r.listFiles(ctx, request)
}

func (r *GoogleDriveReader) listFiles(ctx context.Context, request connectors.DriveListRequest) (connectors.DrivePage, error) {
	values := url.Values{
		"q":        []string{"trashed = false"},
		"pageSize": []string{strconv.Itoa(request.PageSize)},
		"fields":   []string{"nextPageToken,files(id,name,mimeType,webViewLink,modifiedTime,headRevisionId,md5Checksum)"},
		"orderBy":  []string{"modifiedTime desc,name"},
	}
	if token := strings.TrimSpace(request.Cursor.Token); token != "" {
		values.Set("pageToken", token)
	}
	var payload struct {
		NextPageToken string `json:"nextPageToken"`
		Files         []struct {
			ID             string `json:"id"`
			Name           string `json:"name"`
			MIMEType       string `json:"mimeType"`
			WebViewLink    string `json:"webViewLink"`
			ModifiedTime   string `json:"modifiedTime"`
			HeadRevisionID string `json:"headRevisionId"`
			MD5Checksum    string `json:"md5Checksum"`
		} `json:"files"`
	}
	if err := r.getJSON(ctx, "/files?"+values.Encode(), &payload); err != nil {
		return connectors.DrivePage{}, err
	}
	result := connectors.DrivePage{Items: make([]connectors.DriveSourceItem, 0, len(payload.Files))}
	for _, file := range payload.Files {
		if !readableDriveMIME(file.MIMEType) || strings.TrimSpace(file.ID) == "" {
			continue
		}
		content, err := r.readFile(ctx, file.ID, file.MIMEType)
		if err != nil {
			return connectors.DrivePage{}, err
		}
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		modifiedAt, err := parseDriveTime(file.ModifiedTime)
		if err != nil {
			return connectors.DrivePage{}, fmt.Errorf("Google Drive returned an invalid modified time")
		}
		revisionID := strings.TrimSpace(file.HeadRevisionID)
		if revisionID == "" {
			revisionID = strings.TrimSpace(file.MD5Checksum)
		}
		if revisionID == "" {
			revisionID = modifiedAt.UTC().Format(time.RFC3339Nano)
		}
		hash := sha256.Sum256([]byte(content))
		webURL := strings.TrimSpace(file.WebViewLink)
		if webURL == "" {
			webURL = "https://drive.google.com/open?id=" + url.QueryEscape(file.ID)
		}
		result.Items = append(result.Items, connectors.DriveSourceItem{
			FileID:       file.ID,
			Name:         strings.TrimSpace(file.Name),
			MIMEType:     file.MIMEType,
			WebURL:       webURL,
			RevisionID:   revisionID,
			ContentHash:  hex.EncodeToString(hash[:]),
			ModifiedTime: modifiedAt,
			Text:         content,
		})
	}
	if token := strings.TrimSpace(payload.NextPageToken); token != "" {
		result.NextCursor = connectors.DriveCursor{Token: token}
		result.HasMore = true
	}
	return result, nil
}

// listChanges uses the Drive v3 changes feed instead of repeatedly scanning
// the entire files collection. The first call obtains a start page token;
// later calls receive that token from the connector checkpoint. A completed
// feed returns newStartPageToken, which is persisted as CheckpointCursor by
// the sync service for the next poll.
func (r *GoogleDriveReader) listChanges(ctx context.Context, request connectors.DriveListRequest) (connectors.DrivePage, error) {
	pageToken := strings.TrimSpace(request.Cursor.Token)
	if pageToken == "" {
		var start struct {
			StartPageToken string `json:"startPageToken"`
		}
		values := url.Values{"supportsAllDrives": []string{"true"}}
		if err := r.getJSON(ctx, "/changes/startPageToken?"+values.Encode(), &start); err != nil {
			return connectors.DrivePage{}, err
		}
		pageToken = strings.TrimSpace(start.StartPageToken)
		if pageToken == "" {
			return connectors.DrivePage{}, errors.New("Google Drive returned an empty start page token")
		}
		// Establish the checkpoint before walking the current collection. This
		// gives the bootstrap a bounded full snapshot while the subsequent
		// changes feed closes the race with files modified during the walk.
		bootstrapRequest := request
		bootstrapRequest.Cursor = connectors.DriveCursor{}
		page, err := r.listFiles(ctx, bootstrapRequest)
		if err != nil {
			return connectors.DrivePage{}, err
		}
		if page.HasMore {
			if !page.NextCursor.Valid() {
				return connectors.DrivePage{}, connectors.ErrInvalidCursor
			}
			page.NextCursor = connectors.DriveCursor{Token: encodeDriveBootstrapCursor(pageToken, page.NextCursor.Token)}
		} else {
			page.CheckpointCursor = connectors.DriveCursor{Token: pageToken}
		}
		return page, nil
	}
	if strings.HasPrefix(pageToken, driveBootstrapCursorPrefix) {
		bootstrapStart, filesPageToken, isBootstrap := decodeDriveBootstrapCursor(pageToken)
		if !isBootstrap {
			return connectors.DrivePage{}, connectors.ErrInvalidCursor
		}
		bootstrapRequest := request
		bootstrapRequest.Cursor = connectors.DriveCursor{Token: filesPageToken}
		page, err := r.listFiles(ctx, bootstrapRequest)
		if err != nil {
			return connectors.DrivePage{}, err
		}
		if page.HasMore {
			if !page.NextCursor.Valid() {
				return connectors.DrivePage{}, connectors.ErrInvalidCursor
			}
			page.NextCursor = connectors.DriveCursor{Token: encodeDriveBootstrapCursor(bootstrapStart, page.NextCursor.Token)}
		} else {
			page.CheckpointCursor = connectors.DriveCursor{Token: bootstrapStart}
		}
		return page, nil
	}
	values := url.Values{
		"includeItemsFromAllDrives": []string{"true"},
		"includeRemoved":            []string{"true"},
		"pageSize":                  []string{strconv.Itoa(request.PageSize)},
		"pageToken":                 []string{pageToken},
		"supportsAllDrives":         []string{"true"},
		"spaces":                    []string{"drive"},
		"fields":                    []string{"nextPageToken,newStartPageToken,changes(fileId,removed,file(id,name,mimeType,webViewLink,modifiedTime,headRevisionId,md5Checksum,trashed))"},
	}
	var payload struct {
		NextPageToken     string `json:"nextPageToken"`
		NewStartPageToken string `json:"newStartPageToken"`
		Changes           []struct {
			FileID  string `json:"fileId"`
			Removed bool   `json:"removed"`
			File    *struct {
				ID             string `json:"id"`
				Name           string `json:"name"`
				MIMEType       string `json:"mimeType"`
				WebViewLink    string `json:"webViewLink"`
				ModifiedTime   string `json:"modifiedTime"`
				HeadRevisionID string `json:"headRevisionId"`
				MD5Checksum    string `json:"md5Checksum"`
				Trashed        bool   `json:"trashed"`
			} `json:"file"`
		} `json:"changes"`
	}
	if err := r.getJSON(ctx, "/changes?"+values.Encode(), &payload); err != nil {
		return connectors.DrivePage{}, err
	}
	result := connectors.DrivePage{Items: make([]connectors.DriveSourceItem, 0, len(payload.Changes))}
	for _, change := range payload.Changes {
		if change.Removed || (change.File != nil && change.File.Trashed) {
			fileID := strings.TrimSpace(change.FileID)
			if change.File != nil && strings.TrimSpace(change.File.ID) != "" {
				fileID = strings.TrimSpace(change.File.ID)
			}
			if fileID == "" {
				continue
			}
			result.Items = append(result.Items, driveRemovalTombstone(fileID))
			continue
		}
		if change.File == nil {
			continue
		}
		file := change.File
		if !readableDriveMIME(file.MIMEType) || strings.TrimSpace(file.ID) == "" {
			continue
		}
		content, err := r.readFile(ctx, file.ID, file.MIMEType)
		if err != nil {
			return connectors.DrivePage{}, err
		}
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		modifiedAt, err := parseDriveTime(file.ModifiedTime)
		if err != nil {
			return connectors.DrivePage{}, errors.New("Google Drive returned an invalid modified time")
		}
		revisionID := strings.TrimSpace(file.HeadRevisionID)
		if revisionID == "" {
			revisionID = strings.TrimSpace(file.MD5Checksum)
		}
		if revisionID == "" {
			revisionID = modifiedAt.UTC().Format(time.RFC3339Nano)
		}
		hash := sha256.Sum256([]byte(content))
		webURL := strings.TrimSpace(file.WebViewLink)
		if webURL == "" {
			webURL = "https://drive.google.com/open?id=" + url.QueryEscape(file.ID)
		}
		result.Items = append(result.Items, connectors.DriveSourceItem{
			FileID:       file.ID,
			Name:         strings.TrimSpace(file.Name),
			MIMEType:     file.MIMEType,
			WebURL:       webURL,
			RevisionID:   revisionID,
			ContentHash:  hex.EncodeToString(hash[:]),
			ModifiedTime: modifiedAt,
			Text:         content,
		})
	}
	if next := strings.TrimSpace(payload.NextPageToken); next != "" {
		result.NextCursor = connectors.DriveCursor{Token: next}
		result.HasMore = true
	} else if next := strings.TrimSpace(payload.NewStartPageToken); next != "" {
		result.CheckpointCursor = connectors.DriveCursor{Token: next}
	}
	return result, nil
}

func encodeDriveBootstrapCursor(startPageToken, filesPageToken string) string {
	payload := strings.TrimSpace(startPageToken) + "\x00" + strings.TrimSpace(filesPageToken)
	return driveBootstrapCursorPrefix + base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func decodeDriveBootstrapCursor(cursor string) (string, string, bool) {
	cursor = strings.TrimSpace(cursor)
	if !strings.HasPrefix(cursor, driveBootstrapCursorPrefix) {
		return "", "", false
	}
	encoded := strings.TrimPrefix(cursor, driveBootstrapCursorPrefix)
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(decoded), "\x00", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func driveRemovalTombstone(fileID string) connectors.DriveSourceItem {
	fileID = strings.TrimSpace(fileID)
	hash := sha256.Sum256([]byte("drive-removed:" + fileID))
	return connectors.DriveSourceItem{
		FileID:      fileID,
		Name:        "(removed)",
		MIMEType:    "application/x-devenglish-tombstone",
		RevisionID:  "removed:" + fileID,
		ContentHash: hex.EncodeToString(hash[:]),
		Removed:     true,
	}
}

func readableDriveMIME(mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(mimeType, "application/vnd.google-apps.") {
		return mimeType == "application/vnd.google-apps.document" ||
			mimeType == "application/vnd.google-apps.spreadsheet" ||
			mimeType == "application/vnd.google-apps.presentation"
	}
	return strings.HasPrefix(mimeType, "text/") || mimeType == "application/json" || mimeType == "application/xml" || mimeType == "application/yaml"
}

func (r *GoogleDriveReader) readFile(ctx context.Context, fileID, mimeType string) (string, error) {
	endpoint := "/files/" + url.PathEscape(fileID) + "?alt=media"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "application/vnd.google-apps.") {
		endpoint = "/files/" + url.PathEscape(fileID) + "/export?mimeType=text%2Fplain"
	}
	var body []byte
	if err := r.getBytes(ctx, endpoint, &body); err != nil {
		return "", err
	}
	return string(body), nil
}

func (r *GoogleDriveReader) getJSON(ctx context.Context, endpoint string, target any) error {
	body, err := r.do(ctx, http.MethodGet, endpoint, "application/json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, target); err != nil {
		return errors.New("Google Drive returned an invalid response")
	}
	return nil
}

func (r *GoogleDriveReader) getBytes(ctx context.Context, endpoint string, target *[]byte) error {
	body, err := r.do(ctx, http.MethodGet, endpoint, "text/plain")
	if err != nil {
		return err
	}
	*target = body
	return nil
}

func (r *GoogleDriveReader) do(ctx context.Context, method, endpoint, accept string) ([]byte, error) {
	base := strings.TrimRight(r.APIBaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, method, base+endpoint, nil)
	if err != nil {
		return nil, err
	}
	bearerToken, err := r.bearerToken(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("User-Agent", "DevEnglish/1.0")
	client := r.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	maxBytes := r.MaxContentBytes
	if maxBytes <= 0 {
		maxBytes = maxGoogleDriveBodyBytes
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > int(maxBytes) {
		return nil, errors.New("Google Drive source is larger than 512 KB")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, connectors.NewProviderError(connectors.ProviderGoogleDrive, "request", resp.StatusCode, "provider request failed")
	}
	return body, nil
}

func (r *GoogleDriveReader) bearerToken(ctx context.Context) (string, error) {
	if r == nil {
		return "", errors.New("Google Drive read connector is not configured")
	}
	if token := strings.TrimSpace(r.AccessToken); token != "" {
		return token, nil
	}
	if r.TokenSource == nil {
		return "", errors.New("Google Drive read connector is not configured")
	}
	var token *oauth2.Token
	var err error
	if contextSource, ok := r.TokenSource.(tokenContextSource); ok {
		token, err = contextSource.TokenContext(ctx)
	} else {
		token, err = r.TokenSource.Token()
	}
	if err != nil {
		return "", errors.New("Google Drive access token refresh failed")
	}
	if token == nil || strings.TrimSpace(token.AccessToken) == "" {
		return "", errors.New("Google Drive access token refresh returned no token")
	}
	return strings.TrimSpace(token.AccessToken), nil
}

func parseDriveTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, errors.New("missing time")
	}
	return time.Parse(time.RFC3339Nano, value)
}
