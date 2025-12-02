// Package tests contains unit tests that don't require external dependencies
package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gitpkg "github.com/wlame/njgit/internal/git"
	"github.com/wlame/njgit/internal/hcl"
	"github.com/wlame/njgit/internal/nomad"
)

// TestJobNormalization tests that job normalization removes metadata fields
// This test doesn't require Docker or external services
func TestJobNormalization(t *testing.T) {
	// Create a test job with metadata that Nomad would add
	jobID := "example-job"
	jobName := "example-job"
	jobType := "service"
	modifyIndex := uint64(42)
	submitTime := int64(1234567890)

	job := &api.Job{
		ID:          &jobID,
		Name:        &jobName,
		Type:        &jobType,
		Datacenters: []string{"dc1"},
		ModifyIndex: &modifyIndex,
		SubmitTime:  &submitTime,
		Region:      stringToPtr("global"),
		Priority:    intToPtr(50),
	}

	t.Logf("Original job has ModifyIndex: %v", job.ModifyIndex)
	t.Logf("Original job has SubmitTime: %v", job.SubmitTime)

	// Normalize the job
	normalized := nomad.NormalizeJob(job, []string{
		"ModifyIndex", "ModifyTime", "JobModifyIndex",
		"SubmitTime", "CreateIndex", "Status", "StatusDescription",
	})

	// Verify metadata was removed
	assert.Nil(t, normalized.ModifyIndex, "ModifyIndex should be nil after normalization")
	assert.Nil(t, normalized.SubmitTime, "SubmitTime should be nil after normalization")

	// Verify other fields were preserved
	assert.Equal(t, jobID, *normalized.ID)
	assert.Equal(t, jobName, *normalized.Name)
	assert.Equal(t, jobType, *normalized.Type)
	assert.Equal(t, []string{"dc1"}, normalized.Datacenters)

	t.Log("✅ Job normalization successful - metadata removed, data preserved")
}

// TestHCLFormatting tests HCL formatting without external dependencies
func TestHCLFormatting(t *testing.T) {
	// Create a simple job
	jobID := "test-job"
	jobName := "test-job"
	jobType := "batch"
	taskGroupName := "compute"
	count := 3

	job := &api.Job{
		ID:          &jobID,
		Name:        &jobName,
		Type:        &jobType,
		Datacenters: []string{"dc1", "dc2"},
		Region:      stringToPtr("us-west"),
		TaskGroups: []*api.TaskGroup{
			{
				Name:  &taskGroupName,
				Count: &count,
				Tasks: []*api.Task{
					{
						Name:   "worker",
						Driver: "docker",
						Config: map[string]interface{}{
							"image":   "alpine:latest",
							"command": "/bin/sh",
							"args":    []interface{}{"-c", "echo hello"},
						},
						Resources: &api.Resources{
							CPU:      intToPtr(100),
							MemoryMB: intToPtr(256),
						},
					},
				},
			},
		},
	}

	// Convert to HCL
	hclBytes, err := hcl.FormatJobAsHCL(job)
	require.NoError(t, err, "Failed to convert job to HCL")

	hclString := string(hclBytes)

	t.Logf("Generated HCL (%d bytes):\n%s", len(hclBytes), hclString)

	// Verify HCL contains expected elements
	assert.Contains(t, hclString, `job "test-job"`, "HCL should contain job block")
	assert.Contains(t, hclString, `type = "batch"`, "HCL should contain job type")
	assert.Contains(t, hclString, `datacenters = ["dc1", "dc2"]`, "HCL should contain datacenters")
	assert.Contains(t, hclString, `group "compute"`, "HCL should contain task group")
	assert.Contains(t, hclString, `count = 3`, "HCL should contain task group count")
	assert.Contains(t, hclString, `task "worker"`, "HCL should contain task")
	assert.Contains(t, hclString, `driver = "docker"`, "HCL should contain driver")

	t.Log("✅ HCL formatting successful - all expected elements present")
}

// TestHCLComparison tests that HCL normalization allows byte-level comparison
func TestHCLComparison(t *testing.T) {
	// Create two identical jobs with different metadata
	job1 := createSampleJob("my-job", uint64(10), int64(1000))
	job2 := createSampleJob("my-job", uint64(20), int64(2000)) // Different metadata

	// Normalize both
	norm1 := nomad.NormalizeJob(job1, []string{
		"ModifyIndex", "ModifyTime", "JobModifyIndex",
		"SubmitTime", "CreateIndex", "Status", "StatusDescription",
	})
	norm2 := nomad.NormalizeJob(job2, []string{
		"ModifyIndex", "ModifyTime", "JobModifyIndex",
		"SubmitTime", "CreateIndex", "Status", "StatusDescription",
	})

	// Convert to HCL
	hcl1, err := hcl.FormatJobAsHCL(norm1)
	require.NoError(t, err)
	hcl1 = hcl.NormalizeHCL(hcl1)

	hcl2, err := hcl.FormatJobAsHCL(norm2)
	require.NoError(t, err)
	hcl2 = hcl.NormalizeHCL(hcl2)

	// Should be identical despite different metadata
	assert.Equal(t, string(hcl1), string(hcl2),
		"Normalized HCL should be identical for jobs with same content but different metadata")

	t.Log("✅ HCL comparison works - identical jobs produce identical HCL")

	// Now create a job with actual differences
	job3 := createSampleJob("my-job", uint64(30), int64(3000))
	job3.Datacenters = []string{"dc1", "dc2", "dc3"} // Different datacenters

	norm3 := nomad.NormalizeJob(job3, []string{
		"ModifyIndex", "ModifyTime", "JobModifyIndex",
		"SubmitTime", "CreateIndex", "Status", "StatusDescription",
	})

	hcl3, err := hcl.FormatJobAsHCL(norm3)
	require.NoError(t, err)
	hcl3 = hcl.NormalizeHCL(hcl3)

	// Should be different
	assert.NotEqual(t, string(hcl1), string(hcl3),
		"Jobs with different content should produce different HCL")

	t.Log("✅ Change detection works - different jobs produce different HCL")
}

// TestCompareHCL tests the HCL comparison utility function
func TestCompareHCL(t *testing.T) {
	// Same content with different whitespace
	hcl1 := []byte(`job "test" {
  type = "service"
  datacenters = ["dc1"]
}`)

	hcl2 := []byte("job \"test\" {\r\n  type = \"service\"  \r\n  datacenters = [\"dc1\"]\r\n}\r\n")

	// CompareHCL should normalize and compare
	result := hcl.CompareHCL(hcl1, hcl2)
	assert.True(t, result, "HCL with different whitespace should be considered equal")

	t.Log("✅ HCL comparison handles whitespace normalization")

	// Different content
	hcl3 := []byte(`job "test" {
  type = "batch"
  datacenters = ["dc1"]
}`)

	result = hcl.CompareHCL(hcl1, hcl3)
	assert.False(t, result, "HCL with different content should not be equal")

	t.Log("✅ HCL comparison detects content differences")
}

// Helper function to create sample jobs for testing
func createSampleJob(id string, modifyIndex uint64, submitTime int64) *api.Job {
	jobType := "service"
	taskGroupName := "web"
	count := 1

	return &api.Job{
		ID:          &id,
		Name:        &id,
		Type:        &jobType,
		Datacenters: []string{"dc1"},
		Region:      stringToPtr("global"),
		ModifyIndex: &modifyIndex,
		SubmitTime:  &submitTime,
		TaskGroups: []*api.TaskGroup{
			{
				Name:  &taskGroupName,
				Count: &count,
				Tasks: []*api.Task{
					{
						Name:   "server",
						Driver: "docker",
						Config: map[string]interface{}{
							"image": "nginx:latest",
						},
						Resources: &api.Resources{
							CPU:      intToPtr(500),
							MemoryMB: intToPtr(512),
						},
					},
				},
			},
		},
	}
}

// TestHistoryOutputFormat tests the new one-line-per-commit history format
func TestHistoryOutputFormat(t *testing.T) {
	// Create a temporary directory for the Git repo
	tempDir, err := os.MkdirTemp("", "njgit-history-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize Git repo
	err = runCommand(tempDir, "git", "init", "-b", "main")
	require.NoError(t, err)
	err = runCommand(tempDir, "git", "config", "user.name", "test")
	require.NoError(t, err)
	err = runCommand(tempDir, "git", "config", "user.email", "test@example.com")
	require.NoError(t, err)

	// Create some commits with job files
	commits := []struct {
		file    string
		content string
		message string
	}{
		{
			file:    "global/default/web-server.hcl",
			content: "job \"web-server\" { type = \"service\" }",
			message: "Add web-server job",
		},
		{
			file:    "global/default/api-server.hcl",
			content: "job \"api-server\" { type = \"service\" }",
			message: "Add api-server job",
		},
		{
			file:    "us-west/production/database.hcl",
			content: "job \"database\" { type = \"service\" }",
			message: "Add database job in us-west/production",
		},
	}

	for _, c := range commits {
		// Create directory structure
		dir := filepath.Join(tempDir, filepath.Dir(c.file))
		err = os.MkdirAll(dir, 0755)
		require.NoError(t, err)

		// Write file
		filePath := filepath.Join(tempDir, c.file)
		err = os.WriteFile(filePath, []byte(c.content), 0644)
		require.NoError(t, err)

		// Add and commit
		err = runCommand(tempDir, "git", "add", c.file)
		require.NoError(t, err)
		err = runCommand(tempDir, "git", "commit", "-m", c.message)
		require.NoError(t, err)

		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Open repository and get history
	repo, err := gitpkg.NewLocalRepository(tempDir)
	require.NoError(t, err)

	history, err := repo.GetHistory("", 0)
	require.NoError(t, err)
	assert.Len(t, history, 3, "Should have 3 commits")

	t.Log("Testing history output format:")

	// Test that we can format output correctly
	for _, commit := range history {
		// Format date as YYYY-MM-DD HH:MM
		dateStr := commit.Date.Format("2006-01-02 15:04")

		// Extract job name from files
		jobName := ""
		if len(commit.Files) > 0 {
			filePath := commit.Files[0]
			jobName = strings.TrimSuffix(filePath, ".hcl")
		}

		// Get first line of commit message
		messageLines := strings.Split(commit.Message, "\n")
		firstLine := messageLines[0]

		// Format output
		var output string
		if jobName != "" {
			output = fmt.Sprintf("%s %s %s %s", dateStr, commit.Hash, jobName, firstLine)
		} else {
			output = fmt.Sprintf("%s %s %s", dateStr, commit.Hash, firstLine)
		}

		t.Logf("  %s", output)

		// Verify format
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}`, output, "Should start with date in YYYY-MM-DD HH:MM format")
		assert.Contains(t, output, commit.Hash, "Should contain commit hash")
		if len(commit.Files) > 0 {
			// Should contain the job path (without .hcl)
			expectedJobPath := strings.TrimSuffix(commit.Files[0], ".hcl")
			assert.Contains(t, output, expectedJobPath, "Should contain job path")
		}
		assert.Contains(t, output, firstLine, "Should contain commit message")

		// Verify it's a single line (no newlines except at the end)
		lines := strings.Split(strings.TrimSpace(output), "\n")
		assert.Len(t, lines, 1, "Output should be a single line")
	}

	t.Log("✅ History output format is correct")
}

// TestHistoryLimit tests that history limit works correctly
func TestHistoryLimit(t *testing.T) {
	// Create a temporary directory for the Git repo
	tempDir, err := os.MkdirTemp("", "njgit-history-limit-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize Git repo
	err = runCommand(tempDir, "git", "init", "-b", "main")
	require.NoError(t, err)
	err = runCommand(tempDir, "git", "config", "user.name", "test")
	require.NoError(t, err)
	err = runCommand(tempDir, "git", "config", "user.email", "test@example.com")
	require.NoError(t, err)

	// Create 10 commits
	for i := 1; i <= 10; i++ {
		file := fmt.Sprintf("job-%d.hcl", i)
		content := fmt.Sprintf("job \"job-%d\" {}", i)
		message := fmt.Sprintf("Commit %d", i)

		filePath := filepath.Join(tempDir, file)
		err = os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)

		err = runCommand(tempDir, "git", "add", file)
		require.NoError(t, err)
		err = runCommand(tempDir, "git", "commit", "-m", message)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)
	}

	// Open repository
	repo, err := gitpkg.NewLocalRepository(tempDir)
	require.NoError(t, err)

	// Test unlimited (0)
	history, err := repo.GetHistory("", 0)
	require.NoError(t, err)
	assert.Len(t, history, 10, "Unlimited should return all 10 commits")

	// Test limit of 5
	history, err = repo.GetHistory("", 5)
	require.NoError(t, err)
	assert.Len(t, history, 5, "Limit 5 should return 5 commits")

	// Test limit of 1
	history, err = repo.GetHistory("", 1)
	require.NoError(t, err)
	assert.Len(t, history, 1, "Limit 1 should return 1 commit")

	// Verify the single commit is the most recent one
	assert.Contains(t, history[0].Message, "Commit 10", "Should return the most recent commit")

	t.Log("✅ History limit works correctly")
}

// TestHistoryDateFormat tests that the date formatting is consistent
func TestHistoryDateFormat(t *testing.T) {
	testCases := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "Standard date",
			time:     time.Date(2025, 11, 2, 12, 34, 0, 0, time.UTC),
			expected: "2025-11-02 12:34",
		},
		{
			name:     "Single digit day and hour",
			time:     time.Date(2025, 1, 5, 9, 7, 0, 0, time.UTC),
			expected: "2025-01-05 09:07",
		},
		{
			name:     "End of year",
			time:     time.Date(2024, 12, 31, 23, 59, 0, 0, time.UTC),
			expected: "2024-12-31 23:59",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formatted := tc.time.Format("2006-01-02 15:04")
			assert.Equal(t, tc.expected, formatted, "Date format should match YYYY-MM-DD HH:MM")
		})
	}

	t.Log("✅ Date format is consistent")
}
