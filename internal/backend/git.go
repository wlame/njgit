// Package backend provides storage backend implementations for ndiff
package backend

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wlame/ndiff/internal/config"
	gitpkg "github.com/wlame/ndiff/internal/git"
)

// GitBackend implements the Backend interface using a local Git repository
// This backend works with an existing local Git repository only
//
// Key features:
//   - Local-only operation (no remote operations)
//   - User manages repository initialization (git init)
//   - User manages remotes and push/pull manually
//   - Uses git config for author name/email
type GitBackend struct {
	config      *config.GitConfig
	repository  *gitpkg.Repository
	localPath   string   // Path to local repo (e.g., ".")
	stagedFiles []string // Files staged for commit
}

// NewGitBackend creates a new Git backend instance
// This validates the configuration but does NOT open the repository yet
// Call Initialize() to actually open the repository
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
	// LocalPath is required and must point to existing Git repository
	if cfg.LocalPath == "" {
		return nil, fmt.Errorf("git.local_path is required for git backend")
	}

	return &GitBackend{
		config:      cfg,
		localPath:   cfg.LocalPath,
		stagedFiles: make([]string, 0),
	}, nil
}

// Initialize prepares the Git backend for use
// Opens the existing local Git repository
// The repository must already exist (user runs 'git init')
//
// Returns:
//
//	error - Any error that occurred
func (g *GitBackend) Initialize() error {
	// Check if local repository exists
	gitDir := filepath.Join(g.localPath, ".git")
	stat, err := os.Stat(gitDir)
	repoExists := err == nil && stat.IsDir()

	if !repoExists {
		return fmt.Errorf("git repository not found at %s\n\n"+
			"Please initialize a Git repository first:\n"+
			"  cd %s\n"+
			"  git init\n\n"+
			"Then run ndiff again.", g.localPath, g.localPath)
	}

	// Open the existing repository
	g.repository, err = gitpkg.NewLocalRepository(g.localPath)
	if err != nil {
		return fmt.Errorf("failed to open local repository at %s: %w", g.localPath, err)
	}

	fmt.Printf("📁 Using local repository at: %s\n", g.localPath)
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
// Author name/email are taken from git config (user must configure)
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

	// Create the commit (uses git config for author)
	hash, err := g.repository.Commit(message, "", "")
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

// Push is a no-op for local Git backend
// User must manually push to remote using git commands
//
// Returns:
//
//	error - Always returns nil
func (g *GitBackend) Push() error {
	// Git backend is local-only - no push operation
	// User manages remotes and push/pull manually
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
//	string - Backend name with local path
func (g *GitBackend) GetName() string {
	return fmt.Sprintf("Git (local: %s)", g.localPath)
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
