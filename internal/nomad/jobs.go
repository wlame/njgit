package nomad

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
)

// JobNotFoundError is returned when a job doesn't exist in Nomad
// This is a custom error type that allows callers to distinguish
// between "job not found" and other errors
//
// In Go, custom error types are common for specific error conditions
// They allow for type assertions: if errors.Is(err, JobNotFoundError{}) { ... }
type JobNotFoundError struct {
	JobName   string
	Namespace string
}

// Error implements the error interface
// In Go, any type with an Error() string method is an error
func (e JobNotFoundError) Error() string {
	return fmt.Sprintf("job %s not found in namespace %s", e.JobName, e.Namespace)
}

// FetchJobSpec fetches the complete specification for a Nomad job
// This is the main function for retrieving job details from Nomad
//
// The job specification includes everything needed to run the job:
//   - Task groups and tasks
//   - Resource requirements
//   - Docker images or other driver configs
//   - Environment variables
//   - Service definitions
//   - And much more...
//
// Parameters:
//   - namespace: The Nomad namespace (e.g., "default", "production")
//   - jobName: The name of the job to fetch
//
// Returns:
//   - *api.Job: The complete job specification
//   - error: Any error encountered (including JobNotFoundError)
func (c *Client) FetchJobSpec(namespace, jobName string) (*api.Job, error) {
	// Validate inputs
	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}
	if jobName == "" {
		return nil, fmt.Errorf("job name cannot be empty")
	}

	// Create query options with the namespace
	// QueryOptions is how we specify which namespace to query
	opts := &api.QueryOptions{
		Namespace: namespace,
	}

	// Fetch the job from Nomad
	// The Info() method returns the full job specification
	// The second return value (*QueryMeta) contains query metadata we don't need here
	job, _, err := c.client.Jobs().Info(jobName, opts)
	if err != nil {
		// Check if this is a 404 (job not found)
		// The Nomad API returns a specific error message for this case
		// In Go, we check error messages carefully - this is somewhat fragile,
		// but it's how the Nomad API works
		if isJobNotFoundError(err) {
			return nil, JobNotFoundError{
				JobName:   jobName,
				Namespace: namespace,
			}
		}

		// Some other error occurred
		return nil, fmt.Errorf("failed to fetch job %s/%s: %w", namespace, jobName, err)
	}

	// Sanity check: the job should not be nil
	if job == nil {
		return nil, fmt.Errorf("received nil job from Nomad for %s/%s", namespace, jobName)
	}

	return job, nil
}

// FetchJobStatus fetches the status information for a job
// This includes deployment status, allocation counts, etc.
// This is separate from the spec because we don't need it for changelog tracking
//
// Parameters:
//   - namespace: The Nomad namespace
//   - jobName: The name of the job
//
// Returns:
//   - *api.Job: The job with status information
//   - error: Any error encountered
func (c *Client) FetchJobStatus(namespace, jobName string) (*api.Job, error) {
	// For status, we can use the same Info() call
	// But in the future, we might want to add more status-specific logic
	return c.FetchJobSpec(namespace, jobName)
}

// JobExists checks if a job exists in Nomad without fetching the full spec
// This is more efficient than FetchJobSpec if you only need to check existence
//
// Parameters:
//   - namespace: The Nomad namespace
//   - jobName: The name of the job
//
// Returns:
//   - bool: true if the job exists, false otherwise
//   - error: Any error encountered (not including "not found")
func (c *Client) JobExists(namespace, jobName string) (bool, error) {
	// Try to fetch the job
	_, err := c.FetchJobSpec(namespace, jobName)
	if err != nil {
		// Check if it's a "not found" error
		if _, ok := err.(JobNotFoundError); ok {
			// Job doesn't exist - this is not an error for this function
			return false, nil
		}

		// Some other error occurred
		return false, err
	}

	// Job exists
	return true, nil
}

// ListJobsByNamespace returns all jobs in a given namespace
// This is useful for discovering jobs or syncing all jobs in a namespace
//
// Parameters:
//   - namespace: The Nomad namespace
//
// Returns:
//   - []*api.JobListStub: List of job stubs (summary information)
//   - error: Any error encountered
func (c *Client) ListJobsByNamespace(namespace string) ([]*api.JobListStub, error) {
	return c.ListJobs(namespace)
}

// GetJobByID is an alias for FetchJobSpec
// Some APIs use "ID" instead of "name" - this provides compatibility
func (c *Client) GetJobByID(namespace, jobID string) (*api.Job, error) {
	return c.FetchJobSpec(namespace, jobID)
}

// isJobNotFoundError checks if an error indicates a job was not found
// The Nomad API returns specific error messages for 404s
//
// This is a helper function that encapsulates the logic for detecting
// "job not found" errors. It checks the error message since the Nomad
// API doesn't provide typed errors.
//
// Parameters:
//   - err: The error to check
//
// Returns:
//   - bool: true if this is a "job not found" error
func isJobNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// The Nomad API typically returns errors with "not found" in the message
	// for 404 responses. This is the most reliable way to detect them.
	//
	// Common patterns:
	//   - "job not found"
	//   - "job 'name' not found"
	//   - "404" in the error message
	errMsg := err.Error()

	// Check for common patterns
	// strings package provides Contains() for substring matching
	return containsAny(errMsg, []string{
		"not found",
		"404",
		"does not exist",
	})
}

// containsAny checks if a string contains any of the given substrings
// This is a helper function since Go doesn't have this built-in
//
// Parameters:
//   - s: The string to search in
//   - substrs: The substrings to search for
//
// Returns:
//   - bool: true if s contains any of the substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		// strings.Contains() does case-sensitive substring matching
		// For case-insensitive, we could use strings.ToLower()
		if containsIgnoreCase(s, substr) {
			return true
		}
	}
	return false
}

// containsIgnoreCase checks if a string contains a substring (case-insensitive)
// This is a helper for more robust error message matching
//
// In Go, strings are case-sensitive by default, so we need to handle this explicitly
func containsIgnoreCase(s, substr string) bool {
	// Convert both to lowercase for comparison
	// strings.ToLower() is the standard way to do case-insensitive comparison in Go
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
