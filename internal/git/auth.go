// Package git handles Git repository operations for storing Nomad job specifications.
// It uses go-git (a pure Go implementation) which doesn't require the git binary.
package git

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/wlame/nomad-changelog/internal/config"
)

// ResolveAuth determines the appropriate Git authentication method
// based on the configuration and repository URL.
//
// Authentication precedence:
//  1. Config file settings (git.ssh_key_path, git.github_token)
//  2. Environment variables (GITHUB_TOKEN, GH_TOKEN)
//  3. SSH agent (for SSH URLs)
//  4. Default SSH keys (~/.ssh/id_rsa, etc.)
//
// The auth_method config setting controls the strategy:
//   - "ssh": Use SSH keys only
//   - "token": Use HTTPS token only
//   - "auto": Try SSH first, fall back to token
//
// Parameters:
//   - cfg: Git configuration from config file
//
// Returns:
//   - transport.AuthMethod: The authentication method to use
//   - error: Any error encountered during auth resolution
func ResolveAuth(cfg *config.GitConfig) (transport.AuthMethod, error) {
	switch cfg.AuthMethod {
	case "ssh":
		// Explicit SSH authentication
		return ResolveSSHAuth(cfg)

	case "token":
		// Explicit token authentication
		return ResolveTokenAuth(cfg)

	case "auto":
		// Try SSH first (common for developers)
		auth, err := ResolveSSHAuth(cfg)
		if err == nil {
			return auth, nil
		}

		// SSH failed, try token (common for CI/CD)
		auth, err = ResolveTokenAuth(cfg)
		if err == nil {
			return auth, nil
		}

		// Both failed
		return nil, fmt.Errorf("failed to resolve authentication (tried SSH and token)")

	default:
		return nil, fmt.Errorf("unknown auth method: %s (use 'ssh', 'token', or 'auto')", cfg.AuthMethod)
	}
}

// ResolveSSHAuth creates SSH-based authentication
// This is the preferred method for developers with SSH keys set up.
//
// SSH authentication sources (in order):
//  1. Explicit key path from config (git.ssh_key_path)
//  2. SSH agent (if available)
//  3. Default SSH keys (~/.ssh/id_rsa, ~/.ssh/id_ed25519)
//
// Parameters:
//   - cfg: Git configuration
//
// Returns:
//   - transport.AuthMethod: SSH authentication
//   - error: Error if SSH auth cannot be set up
func ResolveSSHAuth(cfg *config.GitConfig) (transport.AuthMethod, error) {
	// Try SSH agent first (most secure, no key files on disk)
	// The SSH agent is a program that holds private keys in memory
	// It's commonly used on developer machines
	auth, err := ssh.NewSSHAgentAuth("git")
	if err == nil {
		// SSH agent is available and working
		return auth, nil
	}

	// SSH agent not available, try key files
	var keyPath string

	if cfg.SSHKeyPath != "" {
		// Explicit key path from config
		keyPath = cfg.SSHKeyPath
	} else {
		// Try default SSH key locations
		// This checks for common SSH key types
		keyPath, err = findDefaultSSHKey()
		if err != nil {
			return nil, fmt.Errorf("SSH key not found: %w", err)
		}
	}

	// Expand ~ to home directory if present
	// Go doesn't automatically expand ~ like shells do
	if keyPath[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		keyPath = filepath.Join(homeDir, keyPath[1:])
	}

	// Load the SSH key from file
	// The "git" parameter is the username (always "git" for Git servers)
	// The empty string is the passphrase (we don't support passphrases yet)
	publicKeys, err := ssh.NewPublicKeysFromFile("git", keyPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key from %s: %w", keyPath, err)
	}

	return publicKeys, nil
}

// ResolveTokenAuth creates token-based authentication for HTTPS
// This is the preferred method for CI/CD environments and for users
// who don't have SSH keys set up.
//
// Token sources (in order):
//  1. Config file (git.github_token)
//  2. GITHUB_TOKEN environment variable
//  3. GH_TOKEN environment variable (used by GitHub CLI)
//
// Parameters:
//   - cfg: Git configuration
//
// Returns:
//   - transport.AuthMethod: Token authentication
//   - error: Error if no token is found
func ResolveTokenAuth(cfg *config.GitConfig) (transport.AuthMethod, error) {
	var token string

	// Check config file first
	if cfg.GitHubToken != "" {
		token = cfg.GitHubToken
	} else if envToken := os.Getenv("GITHUB_TOKEN"); envToken != "" {
		// Standard GitHub Actions environment variable
		token = envToken
	} else if envToken := os.Getenv("GH_TOKEN"); envToken != "" {
		// GitHub CLI uses this variable
		token = envToken
	} else {
		return nil, fmt.Errorf("no GitHub token found (set via config, GITHUB_TOKEN, or GH_TOKEN)")
	}

	// For GitHub (and most Git hosting services), token auth uses HTTP Basic Auth
	// Username can be anything non-empty; the token goes in the password field
	// This is how GitHub's API authentication works with tokens
	return &http.BasicAuth{
		Username: "nomad-changelog", // Can be any non-empty string
		Password: token,
	}, nil
}

// findDefaultSSHKey searches for SSH keys in standard locations
// This checks for the most common SSH key types in order of preference.
//
// Key types checked (in order):
//  1. id_ed25519 (modern, recommended)
//  2. id_rsa (traditional, widely supported)
//  3. id_ecdsa (less common)
//
// Returns:
//   - string: Path to the found key
//   - error: Error if no key is found
func findDefaultSSHKey() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// List of key types to try, in order of preference
	// Ed25519 is modern and secure, RSA is traditional and widely supported
	keyTypes := []string{
		"id_ed25519", // Modern, recommended by GitHub
		"id_rsa",     // Traditional, most compatible
		"id_ecdsa",   // Less common but still used
	}

	// Check each key type
	for _, keyType := range keyTypes {
		keyPath := filepath.Join(homeDir, ".ssh", keyType)

		// Check if the file exists
		if _, err := os.Stat(keyPath); err == nil {
			// File exists
			return keyPath, nil
		}
	}

	// No keys found
	return "", fmt.Errorf("no SSH keys found in ~/.ssh/ (tried: %v)", keyTypes)
}

// ValidateAuthMethod checks if the authentication method is properly configured
// This is useful for early validation before attempting Git operations.
//
// Note: This doesn't actually test the auth (that would require a network call),
// it just checks that the configuration looks valid.
//
// Parameters:
//   - auth: The authentication method to validate
//
// Returns:
//   - error: nil if valid, error otherwise
func ValidateAuthMethod(auth transport.AuthMethod) error {
	if auth == nil {
		return fmt.Errorf("authentication method is nil")
	}

	// The go-git library handles validation internally
	// We just check that we have something
	return nil
}

// GetAuthDescription returns a human-readable description of the auth method
// This is useful for logging and debugging (without exposing secrets)
//
// Parameters:
//   - auth: The authentication method
//
// Returns:
//   - string: Description of the auth method
func GetAuthDescription(auth transport.AuthMethod) string {
	if auth == nil {
		return "none"
	}

	// Check the concrete type
	// In Go, we use type assertions to determine the actual type
	switch auth.(type) {
	case *ssh.PublicKeys:
		return "SSH key"
	case *http.BasicAuth:
		return "HTTPS token"
	default:
		return "unknown"
	}
}
