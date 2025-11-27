// Package backend provides storage backend implementations for nomad-changelog
package backend

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wlame/nomad-changelog/internal/config"
	gitpkg "github.com/wlame/nomad-changelog/internal/git"
)

// GitBackend implements the Backend interface using a local Git repository
// This backend clones and maintains a local Git repository on disk
//
// Key features:
//   - Supports any Git provider (GitHub, GitLab, Bitbucket, self-hosted)
//   - Reuses local repository across runs (no re-cloning)
//   - Supports SSH keys and tokens
//   - Standard Git workflow
type GitBackend struct {
	config      *config.GitConfig
	client      *gitpkg.Client
	repository  *gitpkg.Repository
	localPath   string   // Full path to local repo (e.g., "./nomad-changelog-repo")
	stagedFiles []string // Files staged for commit
}

// NewGitBackend creates a new Git backend instance
// This validates the configuration but does NOT clone/open the repository yet
// Call Initialize() to actually set up the repository
//
// Parameters:
//
//	cfg - Git configuration from the config file
//
// Returns:
//
//	*GitBackend - The backend instance
//	error - Any validation errors
func NewGitBackend(cfg *config.GitConfig) (*GitBackend, error) {
	// Validate required configuration
	// URL is only required for remote mode
	if !cfg.LocalOnly && cfg.URL == "" {
		return nil, fmt.Errorf("git.url is required for git backend (or set local_only = true)")
	}

	// Determine full local path where the repo will be stored
	// LocalPath defaults to "." (current directory)
	// RepoName defaults to "nomad-changelog-repo"
	localPath := filepath.Join(cfg.LocalPath, cfg.RepoName)

	return &GitBackend{
		config:      cfg,
		localPath:   localPath,
		stagedFiles: make([]string, 0),
	}, nil
}

// Initialize prepares the Git backend for use
// Behavior depends on local_only configuration:
//   - Local-only mode: Opens existing repository (must already exist)
//   - Remote mode: Opens existing repository and pulls, or clones if doesn't exist
//
// Returns:
//
//	error - Any error that occurred
func (g *GitBackend) Initialize() error {
	// Check if local repository exists
	gitDir := filepath.Join(g.localPath, ".git")
	stat, err := os.Stat(gitDir)
	repoExists := err == nil && stat.IsDir()

	if g.config.LocalOnly {
		// Local-only mode: repository MUST exist
		if !repoExists {
			return fmt.Errorf("local-only mode requires existing git repository at %s (run 'git init' first)", g.localPath)
		}

		// Open the existing repository (no client needed for local-only)
		g.repository, err = gitpkg.NewLocalRepository(g.localPath)
		if err != nil {
			return fmt.Errorf("failed to open local repository at %s: %w", g.localPath, err)
		}

		fmt.Printf("📁 Using local-only repository at: %s\n", g.localPath)
		return nil
	}

	// Remote mode: create Git client for clone/push operations
	g.client, err = gitpkg.NewClient(g.config)
	if err != nil {
		return fmt.Errorf("failed to create git client: %w", err)
	}

	if repoExists {
		// Repository exists locally - open it
		g.repository, err = g.client.Open(g.localPath)
		if err != nil {
			return fmt.Errorf("failed to open existing repository at %s: %w", g.localPath, err)
		}

		// Pull latest changes from remote
		if err := g.repository.Pull(); err != nil {
			return fmt.Errorf("failed to pull latest changes: %w", err)
		}
	} else {
		// Repository doesn't exist - clone it
		g.repository, err = g.client.Clone(g.localPath)
		if err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
	}

	return nil
}

// ReadFile reads a file from the Git repository
// The path is relative to the repository root
//
// Parameters:
//
//	path - Relative path to the file (e.g., "default/web-app.hcl")
//
// Returns:
//
//	[]byte - File content
//	error - Any error that occurred
func (g *GitBackend) ReadFile(path string) ([]byte, error) {
	return g.repository.ReadFile(path)
}

// WriteFile writes a file to the Git repository
// This writes to the working directory but does NOT commit yet
// The file is tracked for the next Commit() call
//
// Parameters:
//
//	path - Relative path to the file (e.g., "default/web-app.hcl")
//	content - File content to write
//
// Returns:
//
//	error - Any error that occurred
func (g *GitBackend) WriteFile(path string, content []byte) error {
	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := g.repository.EnsureDirectory(dir); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write the file
	if err := g.repository.WriteFile(path, content); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Track this file for commit
	g.stagedFiles = append(g.stagedFiles, path)

	return nil
}

// FileExists checks if a file exists in the Git repository
//
// Parameters:
//
//	path - Relative path to the file
//
// Returns:
//
//	bool - true if file exists
//	error - Any error that occurred
func (g *GitBackend) FileExists(path string) (bool, error) {
	return g.repository.FileExists(path)
}

// Commit creates a Git commit with all staged files
// This commits all files that were written since the last Commit() call
//
// Parameters:
//
//	message - Commit message
//
// Returns:
//
//	string - Commit hash (first 8 characters)
//	error - Any error that occurred
func (g *GitBackend) Commit(message string) (string, error) {
	// Stage all tracked files
	for _, path := range g.stagedFiles {
		if err := g.repository.StageFile(path); err != nil {
			return "", fmt.Errorf("failed to stage file %s: %w", path, err)
		}
	}

	// Create the commit
	hash, err := g.repository.Commit(
		message,
		g.config.AuthorName,
		g.config.AuthorEmail,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create commit: %w", err)
	}

	// Clear staged files list
	g.stagedFiles = make([]string, 0)

	// Return first 8 characters of hash
	if len(hash) > 8 {
		return hash[:8], nil
	}
	return hash, nil
}

// Push pushes all commits to the remote repository
// In local-only mode, this is a no-op
//
// Returns:
//
//	error - Any error that occurred
func (g *GitBackend) Push() error {
	// Skip push in local-only mode
	if g.config.LocalOnly {
		fmt.Println("⏭️  Skipping push (local-only mode)")
		return nil
	}

	if err := g.repository.Push(); err != nil {
		return fmt.Errorf("failed to push to remote: %w", err)
	}
	return nil
}

// Close cleans up resources used by the backend
// Note: This does NOT delete the local repository
// The local repo is kept for reuse on the next run
//
// Returns:
//
//	error - Always returns nil for Git backend
func (g *GitBackend) Close() error {
	// Nothing to clean up - we keep the local repo for reuse
	return nil
}

// GetName returns a human-readable name for this backend
// Used for logging and user messages
//
// Returns:
//
//	string - Backend name with repository URL or local path
func (g *GitBackend) GetName() string {
	if g.config.LocalOnly {
		return fmt.Sprintf("Git (local-only: %s)", g.localPath)
	}
	return fmt.Sprintf("Git (%s)", g.config.URL)
}

// GetRepository returns the underlying Git repository
// This is useful for advanced operations like history retrieval
//
// Returns:
//
//	*gitpkg.Repository - The Git repository instance
func (g *GitBackend) GetRepository() *gitpkg.Repository {
	return g.repository
}
