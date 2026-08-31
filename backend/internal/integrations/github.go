package integrations

import (
	"bytes"
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
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
)

const maxGitHubImportBytes = 512 << 10

type GitHubClient struct {
	APIBaseURL string
	Token      string
	Client     *http.Client
}

func NewGitHubFromEnv() *GitHubClient {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("GITHUB_API_BASE_URL")), "/")
	if base == "" {
		base = "https://api.github.com"
	}
	return &GitHubClient{
		APIBaseURL: base,
		Token:      strings.TrimSpace(os.Getenv("GITHUB_TOKEN")),
		Client:     &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *GitHubClient) Import(ctx context.Context, rawURL string) (domain.GitHubImport, error) {
	owner, repo, kind, number, err := parseGitHubURL(rawURL)
	if err != nil {
		return domain.GitHubImport{}, err
	}
	base := strings.TrimRight(c.APIBaseURL, "/")
	if base == "" {
		return domain.GitHubImport{}, errors.New("GitHub API base URL is not configured")
	}
	repoPath := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
	result := domain.GitHubImport{URL: rawURL, SourceType: "github-readme", Title: owner + "/" + repo + " README"}

	switch kind {
	case "repo", "readme":
		body, _, err := c.request(ctx, base+repoPath+"/readme", "application/vnd.github.raw")
		if err != nil {
			return domain.GitHubImport{}, err
		}
		result.Content = strings.TrimSpace(string(body))
		if kind == "readme" {
			result.Title = owner + "/" + repo + " README"
		}
	case "issue", "pull":
		endpoint := repoPath + "/issues/" + strconv.Itoa(number)
		if kind == "pull" {
			endpoint = repoPath + "/pulls/" + strconv.Itoa(number)
		}
		body, _, err := c.request(ctx, base+endpoint, "application/vnd.github+json")
		if err != nil {
			return domain.GitHubImport{}, err
		}
		var item struct {
			Title string `json:"title"`
			Body  string `json:"body"`
			State string `json:"state"`
			HTML  string `json:"html_url"`
			User  struct {
				Login string `json:"login"`
			} `json:"user"`
		}
		if err := json.Unmarshal(body, &item); err != nil {
			return domain.GitHubImport{}, fmt.Errorf("decode GitHub item: %w", err)
		}
		if strings.TrimSpace(item.Title) == "" {
			return domain.GitHubImport{}, errors.New("GitHub item has no title")
		}
		result.SourceType = "github-" + kind
		result.Title = item.Title
		result.Content = fmt.Sprintf("Title: %s\nState: %s\nAuthor: %s\nURL: %s\n\n%s", item.Title, item.State, item.User.Login, item.HTML, strings.TrimSpace(item.Body))
		if kind == "pull" {
			if files, _, fileErr := c.request(ctx, base+repoPath+"/pulls/"+strconv.Itoa(number)+"/files", "application/vnd.github+json"); fileErr == nil {
				result.Content += "\n\nChanged files:\n" + summarizeFiles(files)
			}
		}
	}
	if strings.TrimSpace(result.Content) == "" {
		return domain.GitHubImport{}, errors.New("GitHub source has no readable content")
	}
	result.Content = truncate(result.Content, maxGitHubImportBytes)
	return result, nil
}

func (c *GitHubClient) request(ctx context.Context, endpoint, accept string) ([]byte, http.Header, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Accept", accept)
	request.Header.Set("User-Agent", "DevEnglish/1.0")
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxGitHubImportBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if len(body) > maxGitHubImportBytes {
		return nil, nil, errors.New("GitHub source is larger than 512 KB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, nil, connectors.NewProviderError(connectors.ProviderGitHub, "request", response.StatusCode, "provider request failed")
	}
	if strings.Contains(response.Header.Get("Content-Type"), "application/json") && strings.HasPrefix(strings.TrimSpace(string(body)), "{") {
		var payload struct {
			Content string `json:"content"`
		}
		if json.Unmarshal(body, &payload) == nil && payload.Content != "" {
			decoded, decodeErr := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload.Content, "\n", ""))
			if decodeErr == nil {
				return decoded, response.Header, nil
			}
		}
	}
	return body, response.Header, nil
}

// ListIssues implements the provider-neutral read contract used by the
// incremental connector service. Pull requests are deliberately excluded:
// V1 imports GitHub issues only, even though GitHub exposes pull requests from
// the same endpoint.
func (c *GitHubClient) ListIssues(ctx context.Context, request connectors.GitHubIssueListRequest) (connectors.GitHubIssuePage, error) {
	repository, err := parseGitHubRepository(request.Repository)
	if err != nil {
		return connectors.GitHubIssuePage{}, err
	}
	if request.PageSize < 1 || request.PageSize > 100 {
		return connectors.GitHubIssuePage{}, connectors.ErrInvalidPageSize
	}
	page := 1
	if cursor := strings.TrimSpace(request.Cursor); cursor != "" {
		page, err = strconv.Atoi(cursor)
		if err != nil || page < 1 {
			return connectors.GitHubIssuePage{}, connectors.ErrInvalidCursor
		}
	}
	base := strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
	if base == "" {
		return connectors.GitHubIssuePage{}, errors.New("GitHub API base URL is not configured")
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues?state=all&per_page=%d&page=%d", base, url.PathEscape(repository.owner), url.PathEscape(repository.name), request.PageSize, page)
	body, headers, err := c.request(ctx, endpoint, "application/vnd.github+json")
	if err != nil {
		return connectors.GitHubIssuePage{}, err
	}
	var payload []githubIssuePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return connectors.GitHubIssuePage{}, connectors.ErrInvalidProviderPayload
	}
	result := connectors.GitHubIssuePage{Issues: make([]connectors.GitHubIssue, 0, len(payload))}
	for _, item := range payload {
		if item.isPullRequest() {
			continue
		}
		issue, normalizeErr := normalizeGitHubIssue(request.Repository, item)
		if normalizeErr != nil {
			return connectors.GitHubIssuePage{}, normalizeErr
		}
		result.Issues = append(result.Issues, issue)
	}
	if nextPage := nextGitHubPage(headers.Get("Link")); nextPage > page {
		result.NextCursor = strconv.Itoa(nextPage)
		result.HasMore = true
	} else if len(payload) >= request.PageSize {
		result.NextCursor = strconv.Itoa(page + 1)
		result.HasMore = true
	}
	return result, nil
}

// GetIssue returns one normalized issue and rejects pull requests at the
// connector boundary so downstream code cannot accidentally treat a PR as a
// canonical issue source.
func (c *GitHubClient) GetIssue(ctx context.Context, repository string, number int64) (connectors.GitHubIssue, error) {
	parsed, err := parseGitHubRepository(repository)
	if err != nil || number < 1 {
		return connectors.GitHubIssue{}, connectors.ErrInvalidRepository
	}
	base := strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
	if base == "" {
		return connectors.GitHubIssue{}, errors.New("GitHub API base URL is not configured")
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/%d", base, url.PathEscape(parsed.owner), url.PathEscape(parsed.name), number)
	body, _, err := c.request(ctx, endpoint, "application/vnd.github+json")
	if err != nil {
		return connectors.GitHubIssue{}, err
	}
	var payload githubIssuePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return connectors.GitHubIssue{}, connectors.ErrInvalidProviderPayload
	}
	if payload.isPullRequest() {
		return connectors.GitHubIssue{}, connectors.ErrInvalidWriteTarget
	}
	return normalizeGitHubIssue(repository, payload)
}

// CreateIssue implements the deliberately small V1 GitHub write surface. It
// accepts only the provider-neutral request; challenge and confirmation
// enforcement stays in connectors.GitHubSafeWriteService.
func (c *GitHubClient) CreateIssue(ctx context.Context, request connectors.CreateIssueRequest) (connectors.GitHubIssue, error) {
	if err := request.Validate(); err != nil {
		return connectors.GitHubIssue{}, err
	}
	parsed, err := parseGitHubRepository(request.Repository)
	if err != nil {
		return connectors.GitHubIssue{}, err
	}
	base := strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
	if base == "" {
		return connectors.GitHubIssue{}, errors.New("GitHub API base URL is not configured")
	}
	body, err := c.writeJSON(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/%s/issues", base, url.PathEscape(parsed.owner), url.PathEscape(parsed.name)), map[string]string{
		"title": request.Title,
		"body":  request.Body,
	})
	if err != nil {
		return connectors.GitHubIssue{}, err
	}
	var payload githubIssuePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return connectors.GitHubIssue{}, connectors.ErrInvalidProviderPayload
	}
	return normalizeGitHubIssue(request.Repository, payload)
}

// AddIssueComment adds one issue comment. No other GitHub write endpoint is
// exposed through this client in V1.
func (c *GitHubClient) AddIssueComment(ctx context.Context, request connectors.CommentIssueRequest) (connectors.GitHubComment, error) {
	if err := request.Validate(); err != nil {
		return connectors.GitHubComment{}, err
	}
	parsed, err := parseGitHubRepository(request.Repository)
	if err != nil {
		return connectors.GitHubComment{}, err
	}
	base := strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
	if base == "" {
		return connectors.GitHubComment{}, errors.New("GitHub API base URL is not configured")
	}
	body, err := c.writeJSON(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", base, url.PathEscape(parsed.owner), url.PathEscape(parsed.name), request.Issue), map[string]string{"body": request.Body})
	if err != nil {
		return connectors.GitHubComment{}, err
	}
	var payload struct {
		ID        int64  `json:"id"`
		Body      string `json:"body"`
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.ID < 1 || strings.TrimSpace(payload.Body) == "" {
		return connectors.GitHubComment{}, connectors.ErrInvalidProviderPayload
	}
	updatedAt, err := parseGitHubTime(payload.UpdatedAt)
	if err != nil {
		return connectors.GitHubComment{}, connectors.ErrInvalidProviderPayload
	}
	hash := sha256.Sum256([]byte(strings.Join([]string{parsed.owner + "/" + parsed.name, strconv.FormatInt(request.Issue, 10), strconv.FormatInt(payload.ID, 10), payload.Body, updatedAt.UTC().Format(time.RFC3339Nano)}, "\x00")))
	return connectors.GitHubComment{Repository: parsed.owner + "/" + parsed.name, Issue: request.Issue, ID: payload.ID, Body: payload.Body, Revision: hex.EncodeToString(hash[:]), UpdatedAt: updatedAt}, nil
}

// SetIssueLabels replaces the issue labels through GitHub's label endpoint.
// The input is normalized and the response is reduced to label identities.
func (c *GitHubClient) SetIssueLabels(ctx context.Context, request connectors.LabelIssueRequest) ([]connectors.GitHubLabel, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	parsed, err := parseGitHubRepository(request.Repository)
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
	if base == "" {
		return nil, errors.New("GitHub API base URL is not configured")
	}
	body, err := c.writeJSON(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/%s/issues/%d/labels", base, url.PathEscape(parsed.owner), url.PathEscape(parsed.name), request.Issue), map[string][]string{"labels": request.Labels})
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, connectors.ErrInvalidProviderPayload
	}
	labels := make([]connectors.GitHubLabel, 0, len(payload))
	for _, item := range payload {
		if strings.TrimSpace(item.Name) == "" {
			return nil, connectors.ErrInvalidProviderPayload
		}
		labels = append(labels, connectors.GitHubLabel{Repository: parsed.owner + "/" + parsed.name, Issue: request.Issue, Name: item.Name})
	}
	return labels, nil
}

func (c *GitHubClient) writeJSON(ctx context.Context, method, endpoint string, payload any) ([]byte, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, connectors.ErrInvalidProviderPayload
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "DevEnglish/1.0")
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxGitHubImportBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxGitHubImportBytes {
		return nil, connectors.ErrInvalidProviderPayload
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, connectors.NewProviderError(connectors.ProviderGitHub, "write", response.StatusCode, "provider request failed")
	}
	return body, nil
}

type githubRepository struct {
	owner string
	name  string
}

func parseGitHubRepository(value string) (githubRepository, error) {
	parts := strings.Split(strings.Trim(strings.TrimSpace(value), "/"), "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" || strings.Contains(value, " ") {
		return githubRepository{}, connectors.ErrInvalidRepository
	}
	return githubRepository{owner: parts[0], name: parts[1]}, nil
}

type githubIssuePayload struct {
	NodeID      string          `json:"node_id"`
	Number      int64           `json:"number"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	State       string          `json:"state"`
	UpdatedAt   string          `json:"updated_at"`
	PullRequest json.RawMessage `json:"pull_request"`
}

func (payload githubIssuePayload) isPullRequest() bool {
	value := strings.TrimSpace(string(payload.PullRequest))
	return value != "" && value != "null" && value != "{}"
}

func normalizeGitHubIssue(repository string, payload githubIssuePayload) (connectors.GitHubIssue, error) {
	parsed, err := parseGitHubRepository(repository)
	if err != nil || payload.Number < 1 || strings.TrimSpace(payload.Title) == "" {
		return connectors.GitHubIssue{}, connectors.ErrInvalidProviderPayload
	}
	updatedAt, err := parseGitHubTime(payload.UpdatedAt)
	if err != nil {
		return connectors.GitHubIssue{}, connectors.ErrInvalidProviderPayload
	}
	state := strings.ToLower(strings.TrimSpace(payload.State))
	if state == "" {
		state = "unknown"
	}
	revisionInput := strings.Join([]string{parsed.owner + "/" + parsed.name, strconv.FormatInt(payload.Number, 10), payload.NodeID, payload.Title, payload.Body, state, updatedAt.UTC().Format(time.RFC3339Nano)}, "\x00")
	hash := sha256.Sum256([]byte(revisionInput))
	return connectors.GitHubIssue{
		Repository: parsed.owner + "/" + parsed.name,
		Number:     payload.Number,
		Title:      strings.TrimSpace(payload.Title),
		Body:       payload.Body,
		State:      state,
		Revision:   hex.EncodeToString(hash[:]),
		UpdatedAt:  updatedAt,
	}, nil
}

func parseGitHubTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}

func nextGitHubPage(link string) int {
	for _, segment := range strings.Split(link, ",") {
		if !strings.Contains(segment, `rel="next"`) {
			continue
		}
		start, end := strings.Index(segment, "<"), strings.Index(segment, ">")
		if start < 0 || end <= start {
			continue
		}
		parsed, err := url.Parse(segment[start+1 : end])
		if err != nil {
			continue
		}
		page, err := strconv.Atoi(parsed.Query().Get("page"))
		if err == nil && page > 0 {
			return page
		}
	}
	return 0
}

func parseGitHubURL(raw string) (owner, repo, kind string, number int, err error) {
	parsed, parseErr := url.Parse(strings.TrimSpace(raw))
	if parseErr != nil || parsed.Scheme != "https" {
		return "", "", "", 0, errors.New("GitHub URL must use https")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return "", "", "", 0, errors.New("only github.com URLs are supported")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", "", 0, errors.New("GitHub URL must include owner and repository")
	}
	owner, repo = parts[0], strings.TrimSuffix(parts[1], ".git")
	if len(parts) == 2 {
		return owner, repo, "repo", 0, nil
	}
	if len(parts) >= 4 && (parts[2] == "issues" || parts[2] == "pull" || parts[2] == "pulls") {
		parsedNumber, numberErr := strconv.Atoi(parts[3])
		if numberErr != nil || parsedNumber < 1 {
			return "", "", "", 0, errors.New("GitHub issue or pull request number is invalid")
		}
		kind = "issue"
		if parts[2] == "pull" || parts[2] == "pulls" {
			kind = "pull"
		}
		return owner, repo, kind, parsedNumber, nil
	}
	if len(parts) >= 5 && parts[2] == "blob" && strings.EqualFold(parts[len(parts)-1], "README.md") {
		return owner, repo, "readme", 0, nil
	}
	return "", "", "", 0, errors.New("supported GitHub sources are repository README, issue or pull request URLs")
}

func summarizeFiles(payload []byte) string {
	var files []struct {
		Filename  string `json:"filename"`
		Status    string `json:"status"`
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
	}
	if json.Unmarshal(payload, &files) != nil {
		return ""
	}
	lines := make([]string, 0, len(files))
	for _, file := range files {
		lines = append(lines, fmt.Sprintf("- %s (%s, +%d/-%d)", file.Filename, file.Status, file.Additions, file.Deletions))
	}
	return strings.Join(lines, "\n")
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
