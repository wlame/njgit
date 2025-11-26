package git

import (
	"fmt"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// StageFile stages a file for commit
// In Git terminology, this is "git add <file>"
// The file must exist in the working directory
//
// Parameters:
//   - path: Relative path to the file (e.g., "production/web-server.hcl")
//
// Returns:
//   - error: Any error encountered during staging
func (r *Repository) StageFile(path string) error {
	// Get the worktree
	// The worktree is the interface for working with files
	w, err := r.GetWorktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Add the file to the staging area
	// This is equivalent to "git add <path>"
	_, err = w.Add(path)
	if err != nil {
		return fmt.Errorf("failed to stage file %s: %w", path, err)
	}

	return nil
}

// StageAll stages all changed files
// This is equivalent to "git add ."
// Useful when you want to commit all changes at once
//
// Returns:
//   - error: Any error encountered
func (r *Repository) StageAll() error {
	w, err := r.GetWorktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// "." means add all files
	_, err = w.Add(".")
	if err != nil {
		return fmt.Errorf("failed to stage all files: %w", err)
	}

	return nil
}

// Commit creates a commit with the staged changes
// This is equivalent to "git commit -m <message>"
// The commit is created locally; you need to call Push() to send it to the remote
//
// Parameters:
//   - message: Commit message
//   - author: Author name (e.g., "nomad-changelog")
//   - email: Author email (e.g., "bot@example.com")
//
// Returns:
//   - string: The commit hash (SHA)
//   - error: Any error encountered
func (r *Repository) Commit(message, author, email string) (string, error) {
	// Get the worktree
	w, err := r.GetWorktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	// Create the commit options
	// The Author field identifies who made the commit
	// The When field is the commit timestamp
	commitOpts := &git.CommitOptions{
		Author: &object.Signature{
			Name:  author,
			Email: email,
			When:  time.Now(),
		},
	}

	// Create the commit
	// This returns a commit hash (plumbing.Hash)
	hash, err := w.Commit(message, commitOpts)
	if err != nil {
		return "", fmt.Errorf("failed to create commit: %w", err)
	}

	// Return the hash as a string
	// Hash.String() gives us the familiar 40-character SHA
	return hash.String(), nil
}

// HasChanges checks if there are any uncommitted changes
// This is useful to avoid creating empty commits
//
// Returns:
//   - bool: true if there are changes to commit
//   - error: Any error encountered
func (r *Repository) HasChanges() (bool, error) {
	w, err := r.GetWorktree()
	if err != nil {
		return false, fmt.Errorf("failed to get worktree: %w", err)
	}

	// Get the status of the working tree
	// This shows what files have changed, been added, deleted, etc.
	status, err := w.Status()
	if err != nil {
		return false, fmt.Errorf("failed to get status: %w", err)
	}

	// Status.IsClean() returns true if there are no changes
	// We want to return true if there ARE changes, so we negate it
	return !status.IsClean(), nil
}

// Push pushes commits to the remote repository
// This is equivalent to "git push origin <branch>"
// This sends all local commits that aren't on the remote yet
//
// Returns:
//   - error: Any error encountered during push
func (r *Repository) Push() error {
	// Prepare push options
	pushOpts := &git.PushOptions{
		// RemoteName is typically "origin"
		RemoteName: "origin",

		// Authentication
		Auth: r.auth,

		// Progress can be set to os.Stdout to show push progress
		Progress: nil,
	}

	// Perform the push
	err := r.repo.Push(pushOpts)
	if err != nil {
		// Check if this is the "already up-to-date" error
		// This happens when there are no new commits to push
		if err == git.NoErrAlreadyUpToDate {
			// Not really an error
			return nil
		}

		return fmt.Errorf("failed to push to remote: %w", err)
	}

	return nil
}

// CommitAndPush is a convenience function that commits and pushes in one call
// This is the most common workflow: stage files, commit, push
//
// Parameters:
//   - message: Commit message
//   - author: Author name
//   - email: Author email
//
// Returns:
//   - string: Commit hash
//   - error: Any error encountered
func (r *Repository) CommitAndPush(message, author, email string) (string, error) {
	// Create the commit
	hash, err := r.Commit(message, author, email)
	if err != nil {
		return "", err
	}

	// Push to remote
	if err := r.Push(); err != nil {
		return hash, fmt.Errorf("commit succeeded but push failed: %w", err)
	}

	return hash, nil
}

// GetLastCommitMessage returns the message of the last commit
// This is useful for checking what was last committed
//
// Returns:
//   - string: The commit message
//   - error: Any error encountered
func (r *Repository) GetLastCommitMessage() (string, error) {
	// Get the HEAD reference
	// HEAD points to the current commit
	ref, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Get the commit object
	commit, err := r.repo.CommitObject(ref.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to get commit: %w", err)
	}

	return commit.Message, nil
}

// GetLastCommitHash returns the hash of the last commit
//
// Returns:
//   - string: The commit hash (SHA)
//   - error: Any error encountered
func (r *Repository) GetLastCommitHash() (string, error) {
	ref, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	return ref.Hash().String(), nil
}

// GetFileStatus returns the status of a specific file
// This tells you if the file is modified, added, deleted, etc.
//
// Parameters:
//   - path: Relative path to the file
//
// Returns:
//   - string: Status ("unmodified", "modified", "added", "deleted", "untracked")
//   - error: Any error encountered
func (r *Repository) GetFileStatus(path string) (string, error) {
	w, err := r.GetWorktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := w.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	// Get the status for this specific file
	fileStatus := status.File(path)

	// Determine the status
	// The StatusCode is a two-character code like "MM" (modified in worktree and staging)
	switch {
	case fileStatus.Worktree == git.Untracked:
		return "untracked", nil
	case fileStatus.Worktree == git.Modified:
		return "modified", nil
	case fileStatus.Worktree == git.Deleted:
		return "deleted", nil
	case fileStatus.Staging == git.Added:
		return "added", nil
	case fileStatus.Worktree == git.Unmodified && fileStatus.Staging == git.Unmodified:
		return "unmodified", nil
	default:
		return "unknown", nil
	}
}

// ListChangedFiles returns a list of all files that have been modified
// This is useful for seeing what would be committed
//
// Returns:
//   - []string: List of file paths
//   - error: Any error encountered
func (r *Repository) ListChangedFiles() ([]string, error) {
	w, err := r.GetWorktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := w.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	// Collect all changed files
	var changed []string
	for path, fileStatus := range status {
		// Skip unmodified files
		if fileStatus.Worktree == git.Unmodified && fileStatus.Staging == git.Unmodified {
			continue
		}

		changed = append(changed, path)
	}

	return changed, nil
}
