package git

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/wlame/nomad-changelog/internal/config"
)

// Client wraps Git operations and provides a high-level interface
// This pattern is similar to how we wrapped the Nomad API client
type Client struct {
	// config stores the Git configuration
	config *config.GitConfig

	// auth is the authentication method to use for Git operations
	auth transport.AuthMethod

	// workDir is where we clone/work with the repository
	// For stateless operation, this is typically a temp directory
	workDir string
}

// NewClient creates a new Git client with the given configuration
// This initializes authentication but doesn't clone/open the repository yet
//
// Parameters:
//   - cfg: Git configuration from config file
//
// Returns:
//   - *Client: The Git client
//   - error: Any error encountered
func NewClient(cfg *config.GitConfig) (*Client, error) {
	// Resolve authentication
	auth, err := ResolveAuth(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Git authentication: %w", err)
	}

	// Determine working directory
	// For stateless operation, we use a temp directory
	// This ensures we always have a clean state
	workDir := filepath.Join(os.TempDir(), "nomad-changelog-repo")

	return &Client{
		config:  cfg,
		auth:    auth,
		workDir: workDir,
	}, nil
}

// OpenOrClone opens an existing repository or clones it if it doesn't exist
// This is the main entry point for getting a working repository.
//
// Behavior:
//  1. Check if repository already exists locally
//  2. If yes: Open it and pull latest changes
//  3. If no: Clone from remote
//
// For stateless operation, we always start fresh, but this function
// can optimize by reusing an existing clone if it's still valid.
//
// Returns:
//   - *Repository: A wrapper around the git repository
//   - error: Any error encountered
func (c *Client) OpenOrClone() (*Repository, error) {
	// Check if the repository directory exists
	gitDir := filepath.Join(c.workDir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		// Repository exists, try to open it
		repo, err := git.PlainOpen(c.workDir)
		if err != nil {
			// Directory exists but isn't a valid git repo
			// Remove it and clone fresh
			os.RemoveAll(c.workDir)
			return c.clone()
		}

		// Pull latest changes from remote
		// This ensures we have the most recent version
		if err := c.pullLatest(repo); err != nil {
			// Pull failed, might be due to local changes or conflicts
			// For safety, remove and clone fresh
			os.RemoveAll(c.workDir)
			return c.clone()
		}

		return &Repository{
			repo:   repo,
			config: c.config,
			auth:   c.auth,
		}, nil
	}

	// Repository doesn't exist, clone it
	return c.clone()
}

// clone performs the actual Git clone operation
// This is called when we need a fresh copy of the repository
func (c *Client) clone() (*Repository, error) {
	// Ensure the parent directory exists
	if err := os.MkdirAll(filepath.Dir(c.workDir), 0755); err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	// Prepare clone options
	// These control how the repository is cloned
	cloneOpts := &git.CloneOptions{
		// URL of the repository to clone
		URL: c.config.URL,

		// Authentication method (SSH or token)
		Auth: c.auth,

		// Branch to clone
		// We convert the branch name to a Git reference
		// References in Git are like "refs/heads/main"
		ReferenceName: plumbing.NewBranchReferenceName(c.config.Branch),

		// SingleBranch means we only clone the specified branch
		// This is faster than cloning all branches
		SingleBranch: true,

		// Depth 1 means we only get the latest commit (shallow clone)
		// This is much faster for large repositories
		// We don't need the full history for our use case
		Depth: 1,

		// Progress can be set to os.Stdout to show clone progress
		// For now we leave it nil (no progress output)
		Progress: nil,
	}

	// Perform the clone
	repo, err := git.PlainClone(c.workDir, false, cloneOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository %s: %w", c.config.URL, err)
	}

	return &Repository{
		repo:   repo,
		config: c.config,
		auth:   c.auth,
	}, nil
}

// pullLatest pulls the latest changes from the remote repository
// This is called when opening an existing repository to ensure it's up to date
func (c *Client) pullLatest(repo *git.Repository) error {
	// Get the working tree
	// The worktree is where the actual files are (as opposed to the .git directory)
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Prepare pull options
	pullOpts := &git.PullOptions{
		// RemoteName is typically "origin"
		RemoteName: "origin",

		// Authentication
		Auth: c.auth,

		// SingleBranch means only pull the current branch
		SingleBranch: true,

		// Force is false - we don't want to overwrite local changes
		Force: false,
	}

	// Perform the pull
	err = w.Pull(pullOpts)
	if err != nil {
		// git.NoErrAlreadyUpToDate is not actually an error
		// It just means there were no new commits to pull
		if err == git.NoErrAlreadyUpToDate {
			return nil
		}
		return fmt.Errorf("failed to pull latest changes: %w", err)
	}

	return nil
}

// Open opens an existing Git repository at the specified path
// This is used when you want to work with a repository that's already been cloned
//
// Parameters:
//
//	path - The directory path where the repository is located
//
// Returns:
//
//	*Repository - The opened repository
//	error - Any error that occurred
//
// Note: This does NOT pull latest changes. Call repository.Pull() if you need that.
func (c *Client) Open(path string) (*Repository, error) {
	// Check if the path exists and is a valid Git repository
	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("not a git repository: %s", path)
	}

	// Open the repository
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository at %s: %w", path, err)
	}

	return &Repository{
		repo:   repo,
		config: c.config,
		auth:   c.auth,
	}, nil
}

// Clone clones the repository to the specified destination path
// This creates a new clone of the remote repository
//
// Parameters:
//
//	destPath - Where to clone the repository
//
// Returns:
//
//	*Repository - The cloned repository
//	error - Any error that occurred
func (c *Client) Clone(destPath string) (*Repository, error) {
	// Ensure the parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Prepare clone options
	cloneOpts := &git.CloneOptions{
		URL:           c.config.URL,
		Auth:          c.auth,
		ReferenceName: plumbing.NewBranchReferenceName(c.config.Branch),
		SingleBranch:  true,
		Depth:         1,
		Progress:      nil,
	}

	// Perform the clone
	repo, err := git.PlainClone(destPath, false, cloneOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository %s to %s: %w", c.config.URL, destPath, err)
	}

	return &Repository{
		repo:   repo,
		config: c.config,
		auth:   c.auth,
	}, nil
}

// WorkDir returns the working directory path
// This is where the repository is cloned/stored
func (c *Client) WorkDir() string {
	return c.workDir
}

// URL returns the repository URL
func (c *Client) URL() string {
	return c.config.URL
}

// Branch returns the configured branch name
func (c *Client) Branch() string {
	return c.config.Branch
}

// AuthMethod returns the authentication method being used
func (c *Client) AuthMethod() string {
	return GetAuthDescription(c.auth)
}

// Clean removes the working directory
// This is useful for cleaning up after operations
// Be careful: this deletes all local changes!
func (c *Client) Clean() error {
	if c.workDir == "" {
		return nil
	}

	// Check if directory exists
	if _, err := os.Stat(c.workDir); os.IsNotExist(err) {
		return nil
	}

	// Remove the directory and all contents
	return os.RemoveAll(c.workDir)
}

// Close cleans up any resources
// For now, it doesn't do much, but it's good practice to have this method
// In the future, we might add connection pooling or other resources
func (c *Client) Close() error {
	// Currently no cleanup needed
	// The repository is just files on disk
	return nil
}
