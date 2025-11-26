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
	// URL is required
	if g.URL == "" {
		return fmt.Errorf("url is required")
	}

	// Validate URL format
	// It should be either SSH (git@github.com:user/repo.git) or HTTPS (https://github.com/user/repo.git)
	if !isValidGitURL(g.URL) {
		return fmt.Errorf("invalid git URL: %s (must be SSH or HTTPS format)", g.URL)
	}

	// Branch is required (though we set a default, double-check)
	if g.Branch == "" {
		return fmt.Errorf("branch is required")
	}

	// Validate auth method
	validAuthMethods := []string{"ssh", "token", "auto"}
	if !contains(validAuthMethods, g.AuthMethod) {
		return fmt.Errorf("invalid auth_method: %s (must be one of: %s)",
			g.AuthMethod, strings.Join(validAuthMethods, ", "))
	}

	// AuthorName and AuthorEmail should be set (we have defaults, but validate they're not empty)
	if g.AuthorName == "" {
		return fmt.Errorf("author_name is required")
	}
	if g.AuthorEmail == "" {
		return fmt.Errorf("author_email is required")
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
