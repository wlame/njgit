package nomad

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/api"
)

// Client wraps the Nomad API client and provides high-level operations
// This wrapper pattern is common in Go - it allows us to:
//  1. Add logging and error handling
//  2. Provide a simpler interface than the raw API
//  3. Make testing easier (we can mock this interface)
type Client struct {
	// client is the underlying Nomad API client from HashiCorp
	// It's unexported (lowercase) so it's only accessible within this package
	client *api.Client

	// auth stores the authentication configuration used to create this client
	auth *AuthConfig
}

// NewClient creates a new Nomad client with the given authentication
// This is the main entry point for creating a Nomad client
//
// Example usage:
//
//	auth := &AuthConfig{Address: "http://localhost:4646", Token: "..."}
//	client, err := NewClient(auth)
//	if err != nil {
//	    return err
//	}
//	defer client.Close()
//
// Parameters:
//   - auth: Authentication configuration (address, token, TLS settings)
//
// Returns:
//   - *Client: The Nomad client wrapper
//   - error: Any error encountered during client creation
func NewClient(auth *AuthConfig) (*Client, error) {
	// Validate the auth config first
	if err := auth.ValidateAuth(); err != nil {
		return nil, fmt.Errorf("invalid auth config: %w", err)
	}

	// Create the Nomad API config
	// api.DefaultConfig() returns a config with sensible defaults
	config := api.DefaultConfig()

	// Set the address
	// This is where the Nomad API is accessible (e.g., "https://nomad.example.com:4646")
	config.Address = auth.Address

	// Set the token if provided
	// SecretID is the Nomad API's name for ACL tokens
	if auth.Token != "" {
		config.SecretID = auth.Token
	}

	// Configure TLS settings
	// TLSConfig controls how the client handles HTTPS connections
	if auth.TLSSkipVerify {
		// Skip certificate verification (not recommended for production)
		config.TLSConfig.Insecure = true
	}

	if auth.CACert != "" {
		// Use a custom CA certificate for verification
		config.TLSConfig.CACert = auth.CACert
	}

	// Create the actual Nomad API client
	// This is the HashiCorp-provided client that talks to Nomad's HTTP API
	apiClient, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Nomad client: %w", err)
	}

	// Wrap it in our Client struct
	return &Client{
		client: apiClient,
		auth:   auth,
	}, nil
}

// Ping performs a health check against the Nomad API
// This is useful to verify connectivity and authentication before doing real work
//
// In Go, it's common to have a "ping" or "health check" function that validates
// connectivity without doing any real operations.
//
// Returns:
//   - error: nil if Nomad is reachable and responding, error otherwise
func (c *Client) Ping() error {
	// Use the Agent API to get the agent's own information
	// This is a lightweight operation that requires authentication if ACLs are enabled
	// The context.Background() pattern will be explained when we add context support

	// Get the agent's self information
	// This returns information about the Nomad agent we're connected to
	_, err := c.client.Agent().Self()
	if err != nil {
		return fmt.Errorf("failed to ping Nomad: %w", err)
	}

	// If we get here, we successfully talked to Nomad
	return nil
}

// Address returns the Nomad API address this client is connected to
// This is useful for logging and debugging
func (c *Client) Address() string {
	return c.auth.Address
}

// Close cleans up the client and releases any resources
// In Go, it's common to have Close() methods for resources that need cleanup
// Even though the Nomad client doesn't have much to clean up, having this
// method is good practice and makes the interface consistent
//
// Usage pattern:
//
//	client, err := NewClient(auth)
//	if err != nil { return err }
//	defer client.Close()  // Ensures cleanup happens
func (c *Client) Close() error {
	// The Nomad API client doesn't require explicit cleanup
	// But we implement this for consistency and future-proofing
	// If we add connection pooling or caching later, we'd clean it up here
	return nil
}

// ListJobs returns a list of all jobs in the specified namespace
// This is useful for discovering what jobs exist in Nomad
//
// Parameters:
//   - namespace: The Nomad namespace to query (e.g., "default", "production")
//
// Returns:
//   - []*api.JobListStub: A list of job stubs (summary information, not full specs)
//   - error: Any error encountered
func (c *Client) ListJobs(namespace string) ([]*api.JobListStub, error) {
	// Create query options
	// QueryOptions control how the query is executed (namespace, consistency, etc.)
	opts := &api.QueryOptions{
		Namespace: namespace,
	}

	// Query the jobs
	// The List() method returns job stubs, which contain summary info but not full specs
	// The second return value (*QueryMeta) contains metadata about the query (index, etc.)
	// We ignore it with _ since we don't need it for this use case
	jobs, _, err := c.client.Jobs().List(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs in namespace %s: %w", namespace, err)
	}

	return jobs, nil
}

// GetAPIClient returns the underlying Nomad API client
// This is an "escape hatch" for operations we haven't wrapped yet
// In Go, it's common to provide access to the underlying client for advanced use cases
//
// Use this sparingly - prefer adding methods to Client for common operations
func (c *Client) GetAPIClient() *api.Client {
	return c.client
}

// WaitForConnection attempts to connect to Nomad with retries
// This is useful when starting up - Nomad might not be immediately available
//
// Parameters:
//   - maxAttempts: Maximum number of connection attempts
//   - delay: Time to wait between attempts
//
// Returns:
//   - error: nil if connection succeeded, error if all attempts failed
func (c *Client) WaitForConnection(maxAttempts int, delay time.Duration) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := c.Ping()
		if err == nil {
			// Success!
			return nil
		}

		// Store the error
		lastErr = err

		// If this isn't the last attempt, wait before retrying
		if attempt < maxAttempts {
			time.Sleep(delay)
		}
	}

	// All attempts failed
	return fmt.Errorf("failed to connect to Nomad after %d attempts: %w", maxAttempts, lastErr)
}
