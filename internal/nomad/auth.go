// Package nomad provides a wrapper around the Nomad API client.
// It handles authentication, job fetching, and job normalization.
package nomad

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wlame/njgit/internal/config"
)

// AuthConfig holds authentication information for Nomad
// This is separate from the config.NomadConfig to allow for CLI flag overrides
type AuthConfig struct {
	// Address is the Nomad API address (e.g., "https://nomad.example.com:4646")
	Address string

	// Token is the Nomad ACL token for authentication
	Token string

	// CACert is the path to a CA certificate for TLS verification
	CACert string

	// TLSSkipVerify controls whether to skip TLS certificate verification
	// This should be false in production
	TLSSkipVerify bool
}

// ResolveAuth resolves Nomad authentication from multiple sources
// It follows this precedence order (highest to lowest):
//  1. CLI flags (cliToken parameter)
//  2. Config file (cfg parameter)
//  3. Environment variables (NOMAD_ADDR, NOMAD_TOKEN)
//  4. Token file (~/.nomad-token)
//
// This function implements the authentication precedence logic described in the architecture plan.
//
// Parameters:
//   - cfg: Configuration from file
//   - cliToken: Token from CLI flag (optional, can be empty)
//   - cliAddr: Address from CLI flag (optional, can be empty)
//
// Returns:
//   - AuthConfig: Resolved authentication configuration
//   - error: Error if authentication cannot be resolved
func ResolveAuth(cfg *config.NomadConfig, cliToken, cliAddr string) (*AuthConfig, error) {
	auth := &AuthConfig{
		CACert:        cfg.CACert,
		TLSSkipVerify: cfg.TLSSkipVerify,
	}

	// Resolve Address
	// Priority: CLI flag > config > NOMAD_ADDR env var
	if cliAddr != "" {
		// CLI flag has highest priority
		auth.Address = cliAddr
	} else if cfg.Address != "" {
		// Config file is next
		auth.Address = cfg.Address
	} else if envAddr := os.Getenv("NOMAD_ADDR"); envAddr != "" {
		// Environment variable is third
		auth.Address = envAddr
	} else {
		// No address found - this is required
		return nil, fmt.Errorf("Nomad address not configured (set via --nomad-addr flag, config file, or NOMAD_ADDR env var)")
	}

	// Resolve Token
	// Priority: CLI flag > config > NOMAD_TOKEN env var > ~/.nomad-token file
	if cliToken != "" {
		// CLI flag has highest priority
		auth.Token = cliToken
	} else if cfg.Token != "" {
		// Config file is next
		auth.Token = cfg.Token
	} else if envToken := os.Getenv("NOMAD_TOKEN"); envToken != "" {
		// Environment variable is third
		auth.Token = envToken
	} else if fileToken := readTokenFile(); fileToken != "" {
		// Token file is last resort
		auth.Token = fileToken
	}
	// Note: Token is optional (Nomad can run without ACLs)
	// So it's okay if we don't find one

	return auth, nil
}

// readTokenFile attempts to read a Nomad token from ~/.nomad-token
// This is a common location where the Nomad CLI stores tokens
//
// The file should contain just the token string, possibly with whitespace
// that will be trimmed.
//
// Returns:
//   - string: The token, or empty string if file doesn't exist or can't be read
func readTokenFile() string {
	// Get the user's home directory
	// os.UserHomeDir() is the standard way to get the home directory in Go
	// It works across platforms (Linux, macOS, Windows)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// If we can't get the home directory, we can't read the token file
		return ""
	}

	// Construct the path to the token file
	// filepath.Join() is the correct way to build file paths in Go
	// It handles path separators correctly for the OS (/ on Unix, \ on Windows)
	tokenPath := filepath.Join(homeDir, ".nomad-token")

	// Try to read the file
	// os.ReadFile() reads the entire file into memory
	// This is fine for small files like token files
	content, err := os.ReadFile(tokenPath)
	if err != nil {
		// File doesn't exist or can't be read - this is okay
		return ""
	}

	// Convert bytes to string and trim whitespace
	// strings.TrimSpace() removes leading and trailing whitespace (spaces, tabs, newlines)
	return strings.TrimSpace(string(content))
}

// ValidateAuth checks if the authentication configuration is valid
// This performs basic validation without actually connecting to Nomad
func (a *AuthConfig) ValidateAuth() error {
	// Address is required
	if a.Address == "" {
		return fmt.Errorf("Nomad address is required")
	}

	// Basic URL format check
	// In Go, we should ensure the URL has a scheme (http:// or https://)
	if !strings.HasPrefix(a.Address, "http://") && !strings.HasPrefix(a.Address, "https://") {
		return fmt.Errorf("Nomad address must start with http:// or https://, got: %s", a.Address)
	}

	// Token is optional, but if TLS is enabled and no token is provided,
	// we might want to warn the user
	// For now, we'll just validate what we have

	return nil
}

// String returns a string representation of the auth config for logging
// This is useful for debugging, but we need to redact the token
func (a *AuthConfig) String() string {
	token := "none"
	if a.Token != "" {
		// Don't log the actual token - security best practice
		token = "********"
	}

	return fmt.Sprintf("AuthConfig{Address: %s, Token: %s, TLSSkipVerify: %v}",
		a.Address, token, a.TLSSkipVerify)
}
