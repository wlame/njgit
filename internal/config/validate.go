package config

import (
	"fmt"
	"net/url"
	"strings"
)

// Validate checks if the configuration is valid
// It returns an error if any required fields are missing or invalid
// This should be called after loading the configuration
func (c *Config) Validate() error {
	// Validate Git configuration
	if err := c.Git.Validate(); err != nil {
		return fmt.Errorf("git config: %w", err)
	}

	// Validate Nomad configuration
	if err := c.Nomad.Validate(); err != nil {
		return fmt.Errorf("nomad config: %w", err)
	}

	// Validate Jobs configuration
	if len(c.Jobs) == 0 {
		return fmt.Errorf("no jobs configured - at least one job must be specified")
	}

	// Validate each job
	for i, job := range c.Jobs {
		if err := job.Validate(); err != nil {
			return fmt.Errorf("job[%d]: %w", i, err)
		}
	}

	return nil
}

// Validate checks if the Git configuration is valid
func (g *GitConfig) Validate() error {
	// Default backend to "git" if not specified
	backend := g.Backend
	if backend == "" {
		backend = "git"
	}

	// Validate backend type
	validBackends := []string{"git", "github-api"}
	if !contains(validBackends, backend) {
		return fmt.Errorf("invalid backend: %s (must be one of: %s)",
			backend, strings.Join(validBackends, ", "))
	}

	// Validate based on backend type
	if backend == "git" {
		// Git backend is local-only
		// Requires local_path to point to an existing git repository
		if g.LocalPath == "" {
			return fmt.Errorf("local_path is required for git backend")
		}
	} else if backend == "github-api" {
		// GitHub API backend requires owner, repo, and token
		if g.Owner == "" {
			return fmt.Errorf("owner is required for github-api backend")
		}
		if g.Repo == "" {
			return fmt.Errorf("repo is required for github-api backend")
		}
		if g.Token == "" {
			return fmt.Errorf("token is required for github-api backend (set via GITHUB_TOKEN or GH_TOKEN env var)")
		}

		// Branch is required for GitHub API backend
		if g.Branch == "" {
			return fmt.Errorf("branch is required for github-api backend")
		}

		// AuthorName and AuthorEmail should be set for GitHub API backend
		if g.AuthorName == "" {
			return fmt.Errorf("author_name is required for github-api backend")
		}
		if g.AuthorEmail == "" {
			return fmt.Errorf("author_email is required for github-api backend")
		}
	}

	return nil
}

// Validate checks if the Nomad configuration is valid
func (n *NomadConfig) Validate() error {
	// Address is required
	if n.Address == "" {
		return fmt.Errorf("address is required (can also be set via NOMAD_ADDR environment variable)")
	}

	// Validate address is a valid URL
	_, err := url.Parse(n.Address)
	if err != nil {
		return fmt.Errorf("invalid address URL: %w", err)
	}

	// Token is optional (Nomad can run without ACLs)
	// But if TLS is enabled, we should warn if token is missing
	// For now, we'll just validate if provided

	return nil
}

// Validate checks if a JobConfig is valid
func (j *JobConfig) Validate() error {
	// Name is required
	if j.Name == "" {
		return fmt.Errorf("name is required")
	}

	// Namespace is required (though we set a default, double-check)
	if j.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	return nil
}

// isValidGitURL checks if a string is a valid Git URL
// It accepts both SSH and HTTPS formats
func isValidGitURL(gitURL string) bool {
	// Check for SSH format: git@github.com:user/repo.git
	if strings.HasPrefix(gitURL, "git@") || strings.HasPrefix(gitURL, "ssh://") {
		return true
	}

	// Check for HTTPS format: https://github.com/user/repo.git
	if strings.HasPrefix(gitURL, "https://") || strings.HasPrefix(gitURL, "http://") {
		// Try to parse as URL
		_, err := url.Parse(gitURL)
		return err == nil
	}

	return false
}

// contains checks if a slice contains a specific string
// This is a helper function since Go doesn't have a built-in contains for slices
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
