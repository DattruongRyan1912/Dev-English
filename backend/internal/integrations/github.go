package integrations

import (
	"context"
	"encoding/base64"
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
		return nil, nil, fmt.Errorf("GitHub returned HTTP %d", response.StatusCode)
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
