// Package config handles loading and managing configuration for njgit.
// It uses Viper to support multiple configuration sources: files, environment variables, and CLI flags.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config is the main configuration structure for njgit
// It maps directly to the TOML configuration file structure
type Config struct {
	// Git contains all Git repository related configuration
	Git GitConfig `mapstructure:"git"`

	// Nomad contains all Nomad cluster related configuration
	Nomad NomadConfig `mapstructure:"nomad"`

	// Jobs is a list of Nomad jobs to track
	Jobs []JobConfig `mapstructure:"jobs"`

	// Changes contains change detection configuration
	Changes ChangesConfig `mapstructure:"changes"`
}

// GitConfig holds Git repository configuration
type GitConfig struct {
	// Backend specifies which backend to use: "git" or "github-api"
	// "git" - Uses local Git repository (local-only, no remote operations)
	// "github-api" - Uses GitHub REST API directly (GitHub only, no local repo)
	// Default: "git"
	Backend string `mapstructure:"backend"`

	// === Git Backend Configuration ===
	// Git backend is local-only: no clone/push/pull operations
	// User must initialize the repository and manage remotes manually

	// LocalPath is the path to the local Git repository
	// The repository must already exist (user runs 'git init')
	// Default: Current directory "."
	// Used by: git backend
	LocalPath string `mapstructure:"local_path"`

	// === GitHub API Backend Configuration ===

	// Branch is the Git branch to use
	// Default: "main"
	// Used by: github-api backend
	Branch string `mapstructure:"branch"`

	// Owner is the GitHub repository owner (user or organization)
	// Example: "myorg" for github.com/myorg/nomad-jobs
	// Used by: github-api backend
	Owner string `mapstructure:"owner"`

	// Repo is the GitHub repository name
	// Example: "nomad-jobs" for github.com/myorg/nomad-jobs
	// Used by: github-api backend
	Repo string `mapstructure:"repo"`

	// AuthorName is the name to use in Git commits
	// Used by: github-api backend
	AuthorName string `mapstructure:"author_name"`

	// AuthorEmail is the email to use in Git commits
	// Used by: github-api backend
	AuthorEmail string `mapstructure:"author_email"`

	// Token is the GitHub personal access token for API authentication
	// Can also be set via GITHUB_TOKEN or GH_TOKEN environment variables
	// Used by: github-api backend
	// IMPORTANT: For security, prefer environment variables over config file
	Token string `mapstructure:"token"`
}

// NomadConfig holds Nomad cluster configuration
type NomadConfig struct {
	// Address is the Nomad API address
	// Example: "https://nomad.example.com:4646"
	// Can also be set via NOMAD_ADDR environment variable
	Address string `mapstructure:"address"`

	// Token is the Nomad ACL token for authentication
	// Can also be set via NOMAD_TOKEN environment variable
	// IMPORTANT: For security, prefer environment variables over config file
	Token string `mapstructure:"token"`

	// CACert is the path to the CA certificate for TLS verification (optional)
	CACert string `mapstructure:"ca_cert"`

	// TLSSkipVerify skips TLS certificate verification (not recommended for production)
	TLSSkipVerify bool `mapstructure:"tls_skip_verify"`
}

// JobConfig represents a single Nomad job to track
type JobConfig struct {
	// Name is the Nomad job name
	Name string `mapstructure:"name"`

	// Namespace is the Nomad namespace the job belongs to
	// Default is "default" if not specified
	Namespace string `mapstructure:"namespace"`

	// Region is the Nomad region the job belongs to
	// Default is "global" if not specified
	Region string `mapstructure:"region"`
}

// ChangesConfig holds change detection configuration
type ChangesConfig struct {
	// IgnoreFields is a list of field paths to ignore when detecting changes
	// These are typically Nomad internal metadata fields that change on every deployment
	IgnoreFields []string `mapstructure:"ignore_fields"`

	// CommitMetadataOnly determines if we should commit when only metadata changes
	// Default is false - we only commit meaningful changes
	CommitMetadataOnly bool `mapstructure:"commit_metadata_only"`
}

// Load reads the configuration from a file and environment variables
// It follows this precedence order (highest to lowest):
//  1. CLI flags (handled by caller)
//  2. Environment variables
//  3. Configuration file
//  4. Default values
//
// Parameters:
//   - configPath: Path to the configuration file. If empty, will look for
//     "njgit.toml" in the current directory
//
// Returns:
//   - *Config: The loaded configuration
//   - error: Any error encountered during loading
func Load(configPath string) (*Config, error) {
	// Create a new Viper instance
	// Viper is a configuration library that supports multiple sources
	v := viper.New()

	// Set configuration file details
	if configPath != "" {
		// User specified a config file path explicitly
		v.SetConfigFile(configPath)
	} else {
		// Look for config file in current directory
		v.SetConfigName("njgit") // Name of config file (without extension)
		v.SetConfigType("toml")  // Config file format
		v.AddConfigPath(".")     // Look in current directory
	}

	// Enable environment variable support
	// This allows setting config values via environment variables
	// Example: NOMAD_CHANGELOG_NOMAD_ADDRESS=http://localhost:4646
	v.SetEnvPrefix("NOMAD_CHANGELOG")
	v.AutomaticEnv()

	// Set default values
	// These are used if no value is provided in the config file or environment
	setDefaults(v)

	// Read the configuration file
	if err := v.ReadInConfig(); err != nil {
		// Check if the error is because the file doesn't exist
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found - this is only an error if user specified a path
			if configPath != "" {
				return nil, fmt.Errorf("config file not found: %s", configPath)
			}
			// Otherwise, we'll use defaults and environment variables
			fmt.Fprintf(os.Stderr, "[WARN] No config file found, using defaults and environment variables\n")
		} else {
			// Some other error reading the config file
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal the configuration into our Config struct
	// This converts the raw configuration data into our typed structure
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}

	// Apply environment variable overrides for specific fields
	// This handles special cases where we want to check multiple env vars
	applyEnvOverrides(&cfg)

	return &cfg, nil
}

// setDefaults sets default values for configuration options
// These defaults are used when no value is provided in the config file or environment
func setDefaults(v *viper.Viper) {
	// Git defaults
	v.SetDefault("git.backend", "git")
	v.SetDefault("git.local_path", ".") // Current directory

	// GitHub API backend defaults
	v.SetDefault("git.branch", "main")
	v.SetDefault("git.author_name", "njgit")
	v.SetDefault("git.author_email", "njgit@localhost")

	// Nomad defaults
	// No defaults for address or token - these must be provided

	// Changes defaults
	// These are Nomad internal fields that should be ignored during change detection
	v.SetDefault("changes.ignore_fields", []string{
		"ModifyIndex",
		"ModifyTime",
		"JobModifyIndex",
		"SubmitTime",
		"CreateIndex",
		"Status",
		"StatusDescription",
	})
	v.SetDefault("changes.commit_metadata_only", false)
}

// applyEnvOverrides applies environment variable overrides for specific fields
// This handles cases where we want to check multiple environment variables
// (e.g., both NOMAD_TOKEN and the prefixed version)
func applyEnvOverrides(cfg *Config) {
	// Override Nomad address from NOMAD_ADDR if set and config is empty
	if cfg.Nomad.Address == "" {
		if addr := os.Getenv("NOMAD_ADDR"); addr != "" {
			cfg.Nomad.Address = addr
		}
	}

	// Override Nomad token from NOMAD_TOKEN if set and config is empty
	if cfg.Nomad.Token == "" {
		if token := os.Getenv("NOMAD_TOKEN"); token != "" {
			cfg.Nomad.Token = token
		} else {
			// Also check for token in ~/.nomad-token file
			if token := readTokenFile(); token != "" {
				cfg.Nomad.Token = token
			}
		}
	}

	// Override GitHub token from GITHUB_TOKEN or GH_TOKEN if set and config is empty
	if cfg.Git.Token == "" {
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			cfg.Git.Token = token
		} else if token := os.Getenv("GH_TOKEN"); token != "" {
			cfg.Git.Token = token
		}
	}

	// Apply defaults to job namespaces and regions if not set
	for i := range cfg.Jobs {
		if cfg.Jobs[i].Namespace == "" {
			cfg.Jobs[i].Namespace = "default"
		}
		if cfg.Jobs[i].Region == "" {
			cfg.Jobs[i].Region = "global"
		}
	}
}

// readTokenFile attempts to read a Nomad token from ~/.nomad-token
// This is a common location for storing the Nomad token
// Returns empty string if file doesn't exist or can't be read
func readTokenFile() string {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Construct path to token file
	tokenPath := filepath.Join(homeDir, ".nomad-token")

	// Read the file
	content, err := os.ReadFile(tokenPath)
	if err != nil {
		return ""
	}

	// Return the token, trimming any whitespace
	return strings.TrimSpace(string(content))
}
