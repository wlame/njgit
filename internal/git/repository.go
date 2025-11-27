package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/wlame/nomad-changelog/internal/config"
)

// Repository represents a Git repository and provides file operations
// This wraps the go-git Repository type and adds convenience methods
type Repository struct {
	// repo is the underlying go-git repository
	repo *git.Repository

	// config is the Git configuration
	config *config.GitConfig

	// auth is the authentication method
	auth transport.AuthMethod
}

// ReadFile reads a file from the repository
// The path is relative to the repository root
//
// Example:
//
//	content, err := repo.ReadFile("production/web-server.hcl")
//
// Parameters:
//   - path: Relative path to the file (e.g., "production/web-server.hcl")
//
// Returns:
//   - []byte: File contents
//   - error: os.ErrNotExist if file doesn't exist, other errors on read failure
func (r *Repository) ReadFile(path string) ([]byte, error) {
	// Get the absolute path
	// We need to combine the repository path with the file path
	absPath := r.getAbsolutePath(path)

	// Read the file
	// os.ReadFile reads the entire file into memory
	// This is fine for our use case (job files are typically small)
	content, err := os.ReadFile(absPath)
	if err != nil {
		// Wrap the error with context
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	return content, nil
}

// WriteFile writes content to a file in the repository
// If the file already exists, it will be overwritten
// Parent directories are created automatically if they don't exist
//
// Example:
//
//	err := repo.WriteFile("production/web-server.hcl", hclContent)
//
// Parameters:
//   - path: Relative path to the file (e.g., "production/web-server.hcl")
//   - content: The content to write
//
// Returns:
//   - error: Any error encountered during writing
func (r *Repository) WriteFile(path string, content []byte) error {
	// Get the absolute path
	absPath := r.getAbsolutePath(path)

	// Ensure the parent directory exists
	// filepath.Dir gets the directory portion of the path
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write the file
	// 0644 means: owner can read/write, group/others can only read
	// This is standard for regular files
	if err := os.WriteFile(absPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}

// FileExists checks if a file exists in the repository
//
// Parameters:
//   - path: Relative path to the file
//
// Returns:
//   - bool: true if the file exists
//   - error: Error if we can't check (not including "not exists")
func (r *Repository) FileExists(path string) (bool, error) {
	absPath := r.getAbsolutePath(path)

	// os.Stat returns information about a file
	// If the file doesn't exist, it returns an error with os.IsNotExist(err) == true
	_, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist - this is not an error for this function
			return false, nil
		}
		// Some other error (permissions, etc.)
		return false, fmt.Errorf("failed to check if %s exists: %w", path, err)
	}

	// File exists
	return true, nil
}

// EnsureDirectory ensures a directory exists in the repository
// This creates the directory and any necessary parent directories
//
// Example:
//
//	err := repo.EnsureDirectory("production")
//
// Parameters:
//   - path: Relative path to the directory
//
// Returns:
//   - error: Any error encountered
func (r *Repository) EnsureDirectory(path string) error {
	absPath := r.getAbsolutePath(path)

	// MkdirAll creates the directory and all parent directories
	// 0755 means: owner can read/write/execute, group/others can read/execute
	// Execute permission is needed to cd into a directory
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	return nil
}

// ListFiles lists all files in a directory
// This is useful for discovering what jobs already exist
//
// Parameters:
//   - dir: Relative directory path (e.g., "production")
//
// Returns:
//   - []string: List of file names (not full paths)
//   - error: Any error encountered
func (r *Repository) ListFiles(dir string) ([]string, error) {
	absPath := r.getAbsolutePath(dir)

	// Check if directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist - return empty list
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to stat %s: %w", dir, err)
	}

	// Check if it's actually a directory
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	// Read the directory
	// ReadDir returns a slice of fs.DirEntry
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	// Extract file names (skip directories)
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// DeleteFile deletes a file from the repository
// This doesn't commit the deletion - you need to call Commit() separately
//
// Parameters:
//   - path: Relative path to the file
//
// Returns:
//   - error: Any error encountered
func (r *Repository) DeleteFile(path string) error {
	absPath := r.getAbsolutePath(path)

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		// File doesn't exist - this is not an error
		return nil
	}

	// Remove the file
	if err := os.Remove(absPath); err != nil {
		return fmt.Errorf("failed to delete %s: %w", path, err)
	}

	return nil
}

// GetRootPath returns the absolute path to the repository root
// This is useful for debugging or constructing paths manually
func (r *Repository) GetRootPath() string {
	// Get the worktree to access the filesystem
	w, err := r.repo.Worktree()
	if err != nil {
		// If we can't get the worktree, something is seriously wrong
		// Return empty string
		return ""
	}

	// The Filesystem().Root() gives us the root path
	return w.Filesystem.Root()
}

// getAbsolutePath converts a relative path to an absolute path
// This is a helper function used internally
func (r *Repository) getAbsolutePath(relativePath string) string {
	rootPath := r.GetRootPath()
	return filepath.Join(rootPath, relativePath)
}

// GetWorktree returns the underlying git worktree
// This is useful for more advanced Git operations
// The worktree is where the actual files are (as opposed to .git directory)
func (r *Repository) GetWorktree() (*git.Worktree, error) {
	return r.repo.Worktree()
}

// GetRepository returns the underlying go-git repository
// This is an "escape hatch" for operations we haven't wrapped yet
func (r *Repository) GetRepository() *git.Repository {
	return r.repo
}

// CommitInfo represents information about a Git commit
type CommitInfo struct {
	Hash     string    // Short commit hash (8 characters)
	FullHash string    // Full commit hash
	Message  string    // Commit message
	Author   string    // Author name
	Email    string    // Author email
	Date     time.Time // Commit date
	Files    []string  // Files changed in this commit
}

// GetHistory returns the commit history for a specific file or all files
// If path is empty, returns all commits
// If path is specified, returns only commits that touched that file
//
// Parameters:
//   - path: File path to filter by (empty for all commits)
//   - maxCount: Maximum number of commits to return (0 for unlimited)
//
// Returns:
//   - []CommitInfo: List of commits
//   - error: Any error that occurred
func (r *Repository) GetHistory(path string, maxCount int) ([]CommitInfo, error) {
	var commits []CommitInfo

	// Get HEAD reference
	ref, err := r.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Get commit log
	logOptions := &git.LogOptions{
		From: ref.Hash(),
	}
	if path != "" {
		logOptions.PathFilter = func(p string) bool {
			return p == path
		}
	}

	commitIter, err := r.repo.Log(logOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get log: %w", err)
	}
	defer commitIter.Close()

	count := 0
	err = commitIter.ForEach(func(c *object.Commit) error {
		if maxCount > 0 && count >= maxCount {
			return storer.ErrStop
		}

		// Get files changed in this commit
		var files []string
		if c.NumParents() > 0 {
			parent, err := c.Parent(0)
			if err == nil {
				changes, err := parent.Patch(c)
				if err == nil {
					for _, fileStat := range changes.FilePatches() {
						from, to := fileStat.Files()
						if to != nil {
							files = append(files, to.Path())
						} else if from != nil {
							files = append(files, from.Path())
						}
					}
				}
			}
		}

		hash := c.Hash.String()
		shortHash := hash
		if len(hash) > 8 {
			shortHash = hash[:8]
		}

		commits = append(commits, CommitInfo{
			Hash:     shortHash,
			FullHash: hash,
			Message:  strings.TrimSpace(c.Message),
			Author:   c.Author.Name,
			Email:    c.Author.Email,
			Date:     c.Author.When,
			Files:    files,
		})

		count++
		return nil
	})

	if err != nil && err != storer.ErrStop {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return commits, nil
}

// GetFileAtCommit retrieves the content of a file at a specific commit
//
// Parameters:
//   - commitHash: The commit hash (can be short or full)
//   - path: The file path relative to repository root
//
// Returns:
//   - []byte: File content
//   - error: Any error that occurred
func (r *Repository) GetFileAtCommit(commitHash, path string) ([]byte, error) {
	// Resolve the commit hash
	hash := plumbing.NewHash(commitHash)
	commit, err := r.repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit %s: %w", commitHash, err)
	}

	// Get the tree for this commit
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}

	// Get the file from the tree
	file, err := tree.File(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file %s at commit %s: %w", path, commitHash, err)
	}

	// Read the file contents
	content, err := file.Contents()
	if err != nil {
		return nil, fmt.Errorf("failed to read file contents: %w", err)
	}

	return []byte(content), nil
}

// Pull fetches and merges changes from the remote repository
// This updates the local repository with the latest changes from the remote
//
// Returns:
//
//	error - Any error that occurred, or nil if successful
//
// Note: If the repository is already up to date, this is not an error
func (r *Repository) Pull() error {
	// Get the worktree (working directory of the repository)
	w, err := r.GetWorktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Pull from the remote
	// This fetches changes and merges them into the current branch
	err = w.Pull(&git.PullOptions{
		RemoteName: "origin",
		Auth:       r.auth,
	})

	// Check if we're already up to date
	// This is not an error - it just means no changes were fetched
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to pull: %w", err)
	}

	return nil
}
