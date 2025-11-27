package backend

import (
	"fmt"

	"github.com/wlame/ndiff/internal/config"
)

// NewBackend creates a new backend based on the configuration.
// This is a factory function that instantiates the correct backend type
// based on the "backend" field in the Git configuration.
//
// Supported backends:
//   - "git" (default): Local Git repository backend using go-git
//   - "github-api": GitHub REST API backend (stateless, no local repo)
//
// Parameters:
//   - cfg: The Git configuration containing backend type and settings
//
// Returns:
//   - Backend: The instantiated backend interface
//   - error: Any error encountered during backend creation
//
// Example:
//
//	backend, err := NewBackend(cfg.Git)
//	if err != nil {
//	    return fmt.Errorf("failed to create backend: %w", err)
//	}
//	defer backend.Close()
//
//	if err := backend.Initialize(); err != nil {
//	    return fmt.Errorf("failed to initialize backend: %w", err)
//	}
func NewBackend(cfg *config.GitConfig) (Backend, error) {
	// Default to "git" backend if not specified
	backendType := cfg.Backend
	if backendType == "" {
		backendType = "git"
	}

	// Create the appropriate backend based on type
	switch backendType {
	case "git":
		// Git backend - uses local Git repository
		return NewGitBackend(cfg)

	case "github-api":
		// GitHub API backend - uses GitHub REST API directly
		return NewGitHubBackend(cfg)

	default:
		// Unknown backend type
		return nil, fmt.Errorf("unsupported backend type: %s (supported: git, github-api)", backendType)
	}
}
