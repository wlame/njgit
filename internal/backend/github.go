package backend

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/wlame/njgit/internal/config"
)

// GitHubBackend implements the Backend interface using the GitHub REST API.
// This backend is stateless - it doesn't maintain a local Git repository.
// All operations are done directly through the GitHub API.
//
// Key features:
// - No local Git repository required
// - Direct API calls to GitHub
// - Automatic commit creation on WriteFile
// - Requires GitHub personal access token for authentication
type GitHubBackend struct {
	config      *config.GitConfig
	httpClient  *http.Client
	baseURL     string
	stagedFiles map[string][]byte // Map of path -> content for files to commit
	fileSHAs    map[string]string // Map of path -> SHA for existing files (needed for updates)
}

// githubFileResponse represents the GitHub API response for file content
// This is what we get back from GET /repos/{owner}/{repo}/contents/{path}
type githubFileResponse struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	SHA     string `json:"sha"`
	Content string `json:"content"` // Base64 encoded
	Size    int    `json:"size"`
	Type    string `json:"type"` // "file" or "dir"
}

// githubCommitRequest represents the request body for creating/updating a file
// This is what we send to PUT /repos/{owner}/{repo}/contents/{path}
type githubCommitRequest struct {
	Message   string           `json:"message"`
	Content   string           `json:"content"` // Base64 encoded
	Branch    string           `json:"branch"`
	SHA       string           `json:"sha,omitempty"` // Required for updates, omit for new files
	Committer *githubCommitter `json:"committer,omitempty"`
}

// githubCommitter represents the committer information
type githubCommitter struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// githubCommitResponse represents the response from creating/updating a file
type githubCommitResponse struct {
	Content struct {
		SHA string `json:"sha"`
	} `json:"content"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// githubErrorResponse represents an error response from the GitHub API
type githubErrorResponse struct {
	Message       string `json:"message"`
	Documentation string `json:"documentation_url"`
}

// NewGitHubBackend creates a new GitHub API backend.
//
// Parameters:
//   - cfg: The Git configuration containing GitHub API settings (owner, repo, token, branch)
//
// Returns:
//   - *GitHubBackend: A new GitHub backend instance
//   - error: Any error encountered during creation
func NewGitHubBackend(cfg *config.GitConfig) (*GitHubBackend, error) {
	// Validate required configuration
	if cfg.Owner == "" {
		return nil, fmt.Errorf("github owner is required for github-api backend")
	}
	if cfg.Repo == "" {
		return nil, fmt.Errorf("github repo is required for github-api backend")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("github token is required for github-api backend (set via GITHUB_TOKEN or GH_TOKEN env var)")
	}

	// Default branch to "main" if not specified
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Construct base URL for API calls
	// GitHub API v3: https://api.github.com/repos/{owner}/{repo}/contents/{path}
	baseURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents", cfg.Owner, cfg.Repo)

	return &GitHubBackend{
		config:      cfg,
		httpClient:  httpClient,
		baseURL:     baseURL,
		stagedFiles: make(map[string][]byte),
		fileSHAs:    make(map[string]string),
	}, nil
}

// Initialize validates the GitHub API credentials and repository access.
// For the GitHub backend, this checks that we can access the repository.
//
// Returns:
//   - error: Any error encountered during initialization
func (g *GitHubBackend) Initialize() error {
	// Try to get the repository root to verify access
	// We use a HEAD request to avoid downloading content
	req, err := http.NewRequest("HEAD", g.baseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("Authorization", "token "+g.config.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Make the request
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to GitHub API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode == 401 {
		return fmt.Errorf("github authentication failed: invalid token")
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("github repository not found: %s/%s", g.config.Owner, g.config.Repo)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("github API error: status %d", resp.StatusCode)
	}

	return nil
}

// ReadFile reads a file from the GitHub repository using the API.
//
// Parameters:
//   - path: The file path relative to repository root (e.g., "default/web-app.hcl")
//
// Returns:
//   - []byte: The file content
//   - error: Any error encountered during reading
func (g *GitHubBackend) ReadFile(path string) ([]byte, error) {
	// Construct URL for this specific file
	url := fmt.Sprintf("%s/%s", g.baseURL, path)

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication and headers
	req.Header.Set("Authorization", "token "+g.config.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Add branch parameter
	q := req.URL.Query()
	q.Add("ref", g.config.Branch)
	req.URL.RawQuery = q.Encode()

	// Make the request
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read file from GitHub: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Handle 404 - file doesn't exist
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	// Handle other errors
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		var errResp githubErrorResponse
		if json.Unmarshal(body, &errResp) == nil {
			return nil, fmt.Errorf("github API error: %s", errResp.Message)
		}
		return nil, fmt.Errorf("github API error: status %d", resp.StatusCode)
	}

	// Parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var fileResp githubFileResponse
	if err := json.Unmarshal(body, &fileResp); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub response: %w", err)
	}

	// Store the SHA for potential updates
	g.fileSHAs[path] = fileResp.SHA

	// Decode base64 content
	content, err := base64.StdEncoding.DecodeString(fileResp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode file content: %w", err)
	}

	return content, nil
}

// WriteFile stages a file to be written to the GitHub repository.
// The file is NOT immediately written - it's staged for the next Commit() call.
//
// Parameters:
//   - path: The file path relative to repository root (e.g., "default/web-app.hcl")
//   - content: The file content to write
//
// Returns:
//   - error: Any error encountered during staging
func (g *GitHubBackend) WriteFile(path string, content []byte) error {
	// Validate path
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Clean the path (remove ./ prefix, etc.)
	path = filepath.Clean(path)
	path = strings.TrimPrefix(path, "./")

	// Stage the file for commit
	g.stagedFiles[path] = content

	return nil
}

// FileExists checks if a file exists in the GitHub repository.
//
// Parameters:
//   - path: The file path relative to repository root
//
// Returns:
//   - bool: true if the file exists, false otherwise
//   - error: Any error encountered during the check
func (g *GitHubBackend) FileExists(path string) (bool, error) {
	// Construct URL for this specific file
	url := fmt.Sprintf("%s/%s", g.baseURL, path)

	// Create HEAD request (don't download content)
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication and headers
	req.Header.Set("Authorization", "token "+g.config.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Add branch parameter
	q := req.URL.Query()
	q.Add("ref", g.config.Branch)
	req.URL.RawQuery = q.Encode()

	// Make the request
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 404 means file doesn't exist
	if resp.StatusCode == 404 {
		return false, nil
	}

	// 200 means file exists
	if resp.StatusCode == 200 {
		return true, nil
	}

	// Any other status is an error
	return false, fmt.Errorf("github API error: status %d", resp.StatusCode)
}

// Commit creates commits for all staged files on GitHub.
// For the GitHub API backend, each file gets its own commit because
// the GitHub API doesn't support multi-file commits.
//
// Parameters:
//   - message: The commit message to use for all commits
//
// Returns:
//   - string: Empty string (GitHub API doesn't return a single commit hash)
//   - error: Any error encountered during commit
func (g *GitHubBackend) Commit(message string) (string, error) {
	// Check if there are any staged files
	if len(g.stagedFiles) == 0 {
		return "", nil // Nothing to commit
	}

	// Commit each staged file
	// NOTE: GitHub API doesn't support multi-file commits, so each file
	// gets its own commit. This is a limitation of the API.
	for path, content := range g.stagedFiles {
		// Check if we need to get the SHA first (for updates)
		sha := g.fileSHAs[path]
		if sha == "" {
			// Try to get the file to see if it exists
			// If it exists, we need its SHA for the update
			exists, err := g.FileExists(path)
			if err != nil {
				return "", fmt.Errorf("failed to check file existence for %s: %w", path, err)
			}

			if exists {
				// File exists, need to read it to get SHA
				_, err := g.ReadFile(path)
				if err != nil {
					return "", fmt.Errorf("failed to get SHA for %s: %w", path, err)
				}
				sha = g.fileSHAs[path]
			}
		}

		// Encode content as base64
		encodedContent := base64.StdEncoding.EncodeToString(content)

		// Create commit request
		commitReq := githubCommitRequest{
			Message: message,
			Content: encodedContent,
			Branch:  g.config.Branch,
			SHA:     sha, // Empty for new files, required for updates
		}

		// Add committer info if provided
		if g.config.AuthorName != "" && g.config.AuthorEmail != "" {
			commitReq.Committer = &githubCommitter{
				Name:  g.config.AuthorName,
				Email: g.config.AuthorEmail,
			}
		}

		// Marshal request to JSON
		reqBody, err := json.Marshal(commitReq)
		if err != nil {
			return "", fmt.Errorf("failed to marshal commit request for %s: %w", path, err)
		}

		// Create HTTP request
		url := fmt.Sprintf("%s/%s", g.baseURL, path)
		req, err := http.NewRequest("PUT", url, bytes.NewReader(reqBody))
		if err != nil {
			return "", fmt.Errorf("failed to create commit request for %s: %w", path, err)
		}

		// Add headers
		req.Header.Set("Authorization", "token "+g.config.Token)
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		req.Header.Set("Content-Type", "application/json")

		// Make the request
		resp, err := g.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to commit file %s: %w", path, err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Check response
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			var errResp githubErrorResponse
			if json.Unmarshal(body, &errResp) == nil {
				return "", fmt.Errorf("failed to commit %s: %s", path, errResp.Message)
			}
			return "", fmt.Errorf("failed to commit %s: status %d", path, resp.StatusCode)
		}

		// Parse response to get the new SHA
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read commit response for %s: %w", path, err)
		}

		var commitResp githubCommitResponse
		if err := json.Unmarshal(body, &commitResp); err != nil {
			return "", fmt.Errorf("failed to parse commit response for %s: %w", path, err)
		}

		// Update the SHA for this file
		g.fileSHAs[path] = commitResp.Content.SHA
	}

	// Clear staged files
	g.stagedFiles = make(map[string][]byte)

	// GitHub API backend doesn't return a single commit hash because
	// each file gets its own commit
	return "", nil
}

// Push is a no-op for the GitHub API backend.
// Commits are already on GitHub after Commit() is called.
//
// Returns:
//   - error: Always nil
func (g *GitHubBackend) Push() error {
	// No-op: commits are already on GitHub
	return nil
}

// Close is a no-op for the GitHub API backend.
// There are no resources to clean up.
//
// Returns:
//   - error: Always nil
func (g *GitHubBackend) Close() error {
	// No-op: nothing to clean up
	return nil
}

// GetName returns the name of this backend for logging and user messages.
//
// Returns:
//   - string: "github-api"
func (g *GitHubBackend) GetName() string {
	return "github-api"
}
