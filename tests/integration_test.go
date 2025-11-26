// Package tests contains integration tests for nomad-changelog
// These tests use testcontainers to spin up real Nomad and Git environments
package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/wlame/nomad-changelog/internal/config"
	gitpkg "github.com/wlame/nomad-changelog/internal/git"
	"github.com/wlame/nomad-changelog/internal/hcl"
	"github.com/wlame/nomad-changelog/internal/nomad"
)

// TestNomadContainer tests that we can start a Nomad container and connect to it
// This is a basic smoke test to ensure testcontainers works
func TestNomadContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start Nomad container
	nomadC, addr, err := startNomadContainer(ctx)
	require.NoError(t, err, "Failed to start Nomad container")
	defer nomadC.Terminate(ctx)

	t.Logf("Nomad container started at: %s", addr)

	// Create Nomad client
	auth := &nomad.AuthConfig{
		Address: addr,
		// No token needed in dev mode
		Token: "",
	}

	client, err := nomad.NewClient(auth)
	require.NoError(t, err, "Failed to create Nomad client")
	defer client.Close()

	// Wait for Nomad API to be fully ready with retries
	// The log message appears before the API is ready to accept connections
	var pingErr error
	for i := 0; i < 10; i++ {
		pingErr = client.Ping()
		if pingErr == nil {
			break
		}
		t.Logf("Waiting for Nomad API (attempt %d/10)...", i+1)
		time.Sleep(500 * time.Millisecond)
	}
	assert.NoError(t, pingErr, "Failed to ping Nomad after retries")

	t.Log("✅ Successfully connected to Nomad container")
}

// TestFetchAndNormalizeJob tests fetching a job from Nomad and normalizing it
func TestFetchAndNormalizeJob(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start Nomad
	nomadC, addr, err := startNomadContainer(ctx)
	require.NoError(t, err)
	defer nomadC.Terminate(ctx)

	// Create client
	client, err := nomad.NewClient(&nomad.AuthConfig{Address: addr})
	require.NoError(t, err)
	defer client.Close()

	// Wait for Nomad API to be fully ready
	time.Sleep(3 * time.Second)

	// Deploy a test job
	jobID := deployTestJob(t, addr)
	t.Logf("Deployed test job: %s", jobID)

	// Fetch the job
	job, err := client.FetchJobSpec("default", jobID)
	require.NoError(t, err, "Failed to fetch job")
	assert.NotNil(t, job)
	assert.Equal(t, jobID, *job.ID)

	t.Logf("✅ Fetched job: %s", *job.ID)

	// Log some job details to see what the API returns
	t.Logf("Job details:")
	t.Logf("  Type: %v", stringPtr(job.Type))
	t.Logf("  Region: %v", stringPtr(job.Region))
	t.Logf("  Datacenters: %v", job.Datacenters)
	t.Logf("  ModifyIndex: %v", uintPtr(job.ModifyIndex))
	t.Logf("  SubmitTime: %v", int64Ptr(job.SubmitTime))
	t.Logf("  Task Groups: %d", len(job.TaskGroups))

	if len(job.TaskGroups) > 0 {
		tg := job.TaskGroups[0]
		t.Logf("  Task Group[0]: %s", stringPtr(tg.Name))
		t.Logf("    Count: %v", intPtr(tg.Count))
		if len(tg.Tasks) > 0 {
			task := tg.Tasks[0]
			t.Logf("    Task[0]: %s", task.Name)
			t.Logf("      Driver: %s", task.Driver)
			if task.Driver == "docker" {
				image := nomad.GetDockerImage(task)
				t.Logf("      Docker Image: %s", image)
			}
		}
	}

	// Normalize the job
	normalized := nomad.NormalizeJob(job, []string{
		"ModifyIndex", "ModifyTime", "JobModifyIndex",
		"SubmitTime", "CreateIndex", "Status", "StatusDescription",
	})

	// Check that metadata was removed
	assert.Nil(t, normalized.ModifyIndex, "ModifyIndex should be nil after normalization")
	assert.Nil(t, normalized.SubmitTime, "SubmitTime should be nil after normalization")

	t.Log("✅ Job normalized successfully")

	// Convert to HCL
	hclBytes, err := hcl.FormatJobAsHCL(normalized)
	require.NoError(t, err, "Failed to convert to HCL")
	assert.NotEmpty(t, hclBytes)

	t.Logf("✅ Converted to HCL (%d bytes)", len(hclBytes))
	t.Logf("HCL output:\n%s", string(hclBytes))
}

// TestGitRepository tests basic Git operations
func TestGitRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary directory for the bare repo (acts as remote)
	remoteDir, err := os.MkdirTemp("", "nomad-changelog-remote-*")
	require.NoError(t, err)
	defer os.RemoveAll(remoteDir)

	// Initialize bare Git repo (this acts as the "remote")
	err = runCommand(remoteDir, "git", "init", "--bare")
	require.NoError(t, err, "Failed to initialize bare Git repo")

	t.Logf("Created bare Git repo at: %s", remoteDir)

	// Create a temporary working directory to make an initial commit
	// Bare repos need at least one commit before they can be cloned
	workDir, err := os.MkdirTemp("", "nomad-changelog-work-*")
	require.NoError(t, err)
	defer os.RemoveAll(workDir)

	// Initialize and make initial commit
	err = runCommand(workDir, "git", "init", "-b", "master")
	require.NoError(t, err)
	err = runCommand(workDir, "git", "config", "user.name", "test")
	require.NoError(t, err)
	err = runCommand(workDir, "git", "config", "user.email", "test@example.com")
	require.NoError(t, err)

	// Create initial file and commit
	initFile := filepath.Join(workDir, "README.md")
	err = os.WriteFile(initFile, []byte("# Test Repository\n"), 0644)
	require.NoError(t, err)
	err = runCommand(workDir, "git", "add", "README.md")
	require.NoError(t, err)
	err = runCommand(workDir, "git", "commit", "-m", "Initial commit")
	require.NoError(t, err)

	// Push to bare repo
	err = runCommand(workDir, "git", "remote", "add", "origin", fmt.Sprintf("file://%s", remoteDir))
	require.NoError(t, err)
	err = runCommand(workDir, "git", "push", "-u", "origin", "master")
	require.NoError(t, err)

	t.Log("✅ Initialized bare repo with initial commit")

	// Create Git config
	cfg := &config.GitConfig{
		URL:         fmt.Sprintf("file://%s", remoteDir),
		Branch:      "master", // Use master to match the branch we created
		AuthMethod:  "auto",
		AuthorName:  "test-bot",
		AuthorEmail: "test@example.com",
	}

	// Create Git client
	client, err := gitpkg.NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	// Clone the repository
	repo, err := client.OpenOrClone()
	require.NoError(t, err)

	t.Log("✅ Cloned repository")

	// Write a test file
	err = repo.WriteFile("test.txt", []byte("Hello, World!"))
	require.NoError(t, err)

	// Check file exists
	exists, err := repo.FileExists("test.txt")
	require.NoError(t, err)
	assert.True(t, exists)

	// Read file back
	content, err := repo.ReadFile("test.txt")
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", string(content))

	t.Log("✅ File operations work")

	// Stage and commit
	err = repo.StageFile("test.txt")
	require.NoError(t, err)

	hash, err := repo.Commit("Initial commit", "test-bot", "test@example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	t.Logf("✅ Created commit: %s", hash[:8])

	// Push to remote
	err = repo.Push()
	require.NoError(t, err)

	t.Log("✅ Pushed to remote")
}

// TestFullSyncWorkflow tests the complete sync workflow
func TestFullSyncWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 1. Start Nomad
	nomadC, nomadAddr, err := startNomadContainer(ctx)
	require.NoError(t, err)
	defer nomadC.Terminate(ctx)

	t.Logf("Nomad started at: %s", nomadAddr)

	// 2. Deploy test job to Nomad
	time.Sleep(3 * time.Second) // Wait for Nomad API to be fully ready
	jobID := deployTestJob(t, nomadAddr)
	t.Logf("Deployed job: %s", jobID)

	// 3. Create bare Git repo with initial commit
	remoteDir, err := os.MkdirTemp("", "nomad-changelog-remote-*")
	require.NoError(t, err)
	defer os.RemoveAll(remoteDir)

	err = runCommand(remoteDir, "git", "init", "--bare")
	require.NoError(t, err)

	// Create initial commit (bare repo needs at least one commit)
	workDir, err := os.MkdirTemp("", "nomad-changelog-work-*")
	require.NoError(t, err)
	defer os.RemoveAll(workDir)

	err = runCommand(workDir, "git", "init", "-b", "master")
	require.NoError(t, err)
	err = runCommand(workDir, "git", "config", "user.name", "test")
	require.NoError(t, err)
	err = runCommand(workDir, "git", "config", "user.email", "test@example.com")
	require.NoError(t, err)

	initFile := filepath.Join(workDir, "README.md")
	err = os.WriteFile(initFile, []byte("# Test\n"), 0644)
	require.NoError(t, err)
	err = runCommand(workDir, "git", "add", "README.md")
	require.NoError(t, err)
	err = runCommand(workDir, "git", "commit", "-m", "Initial commit")
	require.NoError(t, err)
	err = runCommand(workDir, "git", "remote", "add", "origin", fmt.Sprintf("file://%s", remoteDir))
	require.NoError(t, err)
	err = runCommand(workDir, "git", "push", "-u", "origin", "master")
	require.NoError(t, err)

	t.Logf("Git remote at: %s", remoteDir)

	// 4. Set up clients
	nomadClient, err := nomad.NewClient(&nomad.AuthConfig{Address: nomadAddr})
	require.NoError(t, err)
	defer nomadClient.Close()

	gitClient, err := gitpkg.NewClient(&config.GitConfig{
		URL:         fmt.Sprintf("file://%s", remoteDir),
		Branch:      "master", // Use master to match the branch we created
		AuthMethod:  "auto",
		AuthorName:  "nomad-changelog-test",
		AuthorEmail: "test@example.com",
	})
	require.NoError(t, err)
	defer gitClient.Close()

	repo, err := gitClient.OpenOrClone()
	require.NoError(t, err)

	// 5. Fetch job from Nomad
	job, err := nomadClient.FetchJobSpec("default", jobID)
	require.NoError(t, err)

	// 6. Normalize
	normalized := nomad.NormalizeJob(job, []string{
		"ModifyIndex", "ModifyTime", "JobModifyIndex",
		"SubmitTime", "CreateIndex", "Status", "StatusDescription",
	})

	// 7. Convert to HCL
	hclBytes, err := hcl.FormatJobAsHCL(normalized)
	require.NoError(t, err)
	hclBytes = hcl.NormalizeHCL(hclBytes)

	// 8. Write to Git
	filePath := filepath.Join("default", jobID+".hcl")
	err = repo.WriteFile(filePath, hclBytes)
	require.NoError(t, err)

	// 9. Commit and push
	err = repo.StageFile(filePath)
	require.NoError(t, err)

	hash, err := repo.Commit(
		fmt.Sprintf("Add %s", jobID),
		"nomad-changelog-test",
		"test@example.com",
	)
	require.NoError(t, err)

	err = repo.Push()
	require.NoError(t, err)

	t.Logf("✅ Full workflow complete! Commit: %s", hash[:8])

	// 10. Verify: Fetch again and compare
	job2, err := nomadClient.FetchJobSpec("default", jobID)
	require.NoError(t, err)

	normalized2 := nomad.NormalizeJob(job2, []string{
		"ModifyIndex", "ModifyTime", "JobModifyIndex",
		"SubmitTime", "CreateIndex", "Status", "StatusDescription",
	})

	hclBytes2, err := hcl.FormatJobAsHCL(normalized2)
	require.NoError(t, err)
	hclBytes2 = hcl.NormalizeHCL(hclBytes2)

	// Should be identical since nothing changed
	assert.Equal(t, string(hclBytes), string(hclBytes2),
		"HCL should be identical for unchanged job")

	t.Log("✅ Change detection works correctly")
}

// Helper functions

func startNomadContainer(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "hashicorp/nomad:1.6", // Use 1.6 to avoid license issues in newer versions
		ExposedPorts: []string{"4646/tcp"},
		Cmd:          []string{"agent", "-dev", "-bind=0.0.0.0"},
		Env: map[string]string{
			// Skip the Docker warning that Nomad shows
			"NOMAD_SKIP_DOCKER_IMAGE_WARN": "1",
		},
		WaitingFor: wait.ForLog("Nomad agent started").
			WithStartupTimeout(30 * time.Second),
		// Skip ryuk reaper - it has issues with Colima on macOS
		// We'll manually clean up containers instead
		SkipReaper: true,
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", err
	}

	// Get the mapped port
	mappedPort, err := container.MappedPort(ctx, "4646")
	if err != nil {
		container.Terminate(ctx)
		return nil, "", err
	}

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, "", err
	}

	addr := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())
	return container, addr, nil
}

func deployTestJob(t *testing.T, nomadAddr string) string {
	// Create Nomad API client
	config := api.DefaultConfig()
	config.Address = nomadAddr

	client, err := api.NewClient(config)
	require.NoError(t, err)

	// Create a simple test job
	job := createTestJob()

	// Register the job
	_, _, err = client.Jobs().Register(job, nil)
	require.NoError(t, err)

	return *job.ID
}

func createTestJob() *api.Job {
	jobID := "test-job"
	jobName := "test-job"
	jobType := "service"
	datacenters := []string{"dc1"}
	taskGroupName := "test-group"
	count := 1
	taskName := "test-task"

	return &api.Job{
		ID:          &jobID,
		Name:        &jobName,
		Type:        &jobType,
		Datacenters: datacenters,
		TaskGroups: []*api.TaskGroup{
			{
				Name:  &taskGroupName,
				Count: &count,
				Tasks: []*api.Task{
					{
						Name:   taskName,
						Driver: "docker",
						Config: map[string]interface{}{
							"image":   "busybox:latest",
							"command": "/bin/sh",
							"args":    []interface{}{"-c", "sleep 3600"},
						},
						Resources: &api.Resources{
							CPU:      intToPtr(100),
							MemoryMB: intToPtr(128),
						},
					},
				},
			},
		},
	}
}

// runCommand executes a command in the specified directory
// This is used for Git operations in tests (specifically for creating bare repos)
func runCommand(dir string, name string, args ...string) error {
	// Create command using os/exec package
	// exec.Command takes the command name and arguments separately
	cmd := exec.Command(name, args...)

	// Set the working directory for the command
	cmd.Dir = dir

	// Run the command and capture combined output (stdout + stderr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Return error with output for debugging
		return fmt.Errorf("command '%s %v' failed: %w, output: %s",
			name, args, err, string(output))
	}

	return nil
}

// Helper functions for pointer values
func stringPtr(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func intPtr(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func uintPtr(u *uint64) uint64 {
	if u == nil {
		return 0
	}
	return *u
}

func int64Ptr(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func intToPtr(i int) *int {
	return &i
}
