// Package backend provides an abstraction for different storage backends
// (Git repository or GitHub API) for storing Nomad job configurations.
package backend

// Backend is the interface that all storage backends must implement.
// This allows the sync command to work with different storage mechanisms
// without knowing the implementation details.
type Backend interface {
	// Initialize prepares the backend for use.
	// For Git: clones or opens the repository
	// For GitHub API: validates credentials
	Initialize() error

	// ReadFile reads a file from the backend.
	// path is relative to the repository root (e.g., "default/web-app.hcl")
	// Returns the file content, or an error if the file doesn't exist or can't be read.
	ReadFile(path string) ([]byte, error)

	// WriteFile writes a file to the backend.
	// path is relative to the repository root (e.g., "default/web-app.hcl")
	// content is the file content to write
	// This does NOT commit - it just stages the change.
	WriteFile(path string, content []byte) error

	// FileExists checks if a file exists in the backend.
	// path is relative to the repository root
	FileExists(path string) (bool, error)

	// Commit creates a commit with the staged changes.
	// message is the commit message
	// This creates ONE commit with all staged files.
	// Returns the commit hash (or empty string for GitHub API).
	Commit(message string) (string, error)

	// Push pushes the commits to the remote.
	// For Git: pushes to the remote repository
	// For GitHub API: this is a no-op (commits are already on GitHub)
	Push() error

	// Close cleans up any resources used by the backend.
	// For Git: cleans up the repository handle (but keeps local clone)
	// For GitHub API: this is a no-op
	Close() error

	// GetName returns a human-readable name for this backend
	// Used for logging and user messages
	GetName() string
}
