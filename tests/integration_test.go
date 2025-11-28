// Package tests contains integration tests for njgit
// These tests use testcontainers to spin up real Nomad and Git environments
package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/wlame/njgit/internal/backend"
	"github.com/wlame/njgit/internal/config"
	gitpkg "github.com/wlame/njgit/internal/git"
	"github.com/wlame/njgit/internal/hcl"
	"github.com/wlame/njgit/internal/nomad"
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
	remoteDir, err := os.MkdirTemp("", "njgit-remote-*")
	require.NoError(t, err)
	defer os.RemoveAll(remoteDir)

	// Initialize bare Git repo (this acts as the "remote")
	err = runCommand(remoteDir, "git", "init", "--bare")
	require.NoError(t, err, "Failed to initialize bare Git repo")

	t.Logf("Created bare Git repo at: %s", remoteDir)

	// Create a temporary working directory to make an initial commit
	// Bare repos need at least one commit before they can be cloned
	workDir, err := os.MkdirTemp("", "njgit-work-*")
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

	// Open the working directory as a local repository
	// (In the new local-only mode, we work directly with the git repo)
	repo, err := gitpkg.NewLocalRepository(workDir)
	require.NoError(t, err)

	t.Log("✅ Opened local repository")

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
	remoteDir, err := os.MkdirTemp("", "njgit-remote-*")
	require.NoError(t, err)
	defer os.RemoveAll(remoteDir)

	err = runCommand(remoteDir, "git", "init", "--bare")
	require.NoError(t, err)

	// Create initial commit (bare repo needs at least one commit)
	workDir, err := os.MkdirTemp("", "njgit-work-*")
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

	// Open local repository directly (local-only mode)
	repo, err := gitpkg.NewLocalRepository(workDir)
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
		"njgit-test",
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

// TestHistoryAndDeploy tests the history viewing and deploy (rollback) functionality
func TestHistoryAndDeploy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start Nomad container
	nomadContainer, nomadAddr, err := startNomadContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to start Nomad container: %v", err)
	}
	defer nomadContainer.Terminate(ctx)

	t.Logf("Nomad started at: %s", nomadAddr)

	// Create Nomad client
	nomadConfig := api.DefaultConfig()
	nomadConfig.Address = nomadAddr
	apiClient, err := api.NewClient(nomadConfig)
	if err != nil {
		t.Fatalf("Failed to create Nomad API client: %v", err)
	}

	// Wait for Nomad to be ready
	for i := 1; i <= 10; i++ {
		t.Logf("Waiting for Nomad API (attempt %d/10)...", i)
		_, err := apiClient.Agent().Self()
		if err == nil {
			break
		}
		if i == 10 {
			t.Fatalf("Nomad not ready after 10 attempts: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Log("✅ Successfully connected to Nomad container")

	// Create a test job with version 1
	jobV1 := createTestJob()
	jobV1.Name = stringToPtr("rollback-test")
	jobV1.ID = stringToPtr("rollback-test")
	jobV1.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"image":   "busybox:latest",
		"command": "/bin/sh",
		"args":    []string{"-c", "echo 'version 1' && sleep 3600"},
	}

	// Deploy version 1 to Nomad
	_, _, err = apiClient.Jobs().Register(jobV1, nil)
	if err != nil {
		t.Fatalf("Failed to deploy job v1: %v", err)
	}
	t.Log("Deployed job version 1")

	// Create local Git repo (local-only mode)
	localDir, err := os.MkdirTemp("", "njgit-local-*")
	if err != nil {
		t.Fatalf("Failed to create local dir: %v", err)
	}
	defer os.RemoveAll(localDir)

	t.Logf("Git local repo at: %s", localDir)

	// Initialize local Git repo
	cmd := exec.Command("git", "init", "--initial-branch=main", localDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init local repo: %v", err)
	}

	// Configure git user for this repo
	exec.Command("git", "-C", localDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", localDir, "config", "user.name", "test").Run()

	// Create initial commit
	os.WriteFile(filepath.Join(localDir, "README.md"), []byte("# Test"), 0644)
	exec.Command("git", "-C", localDir, "add", ".").Run()
	exec.Command("git", "-C", localDir, "commit", "-m", "Initial commit").Run()

	// Create config (simplified for local-only git backend)
	cfg := &config.Config{
		Git: config.GitConfig{
			Backend:   "git",
			LocalPath: localDir,
		},
		Nomad: config.NomadConfig{
			Address: nomadAddr,
		},
		Jobs: []config.JobConfig{
			{Name: "rollback-test", Namespace: "default", Region: "global"},
		},
	}

	// Sync version 1
	backend, err := backend.NewBackend(&cfg.Git)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	if err := backend.Initialize(); err != nil {
		t.Fatalf("Failed to initialize backend: %v", err)
	}

	// Create our nomad client wrapper
	auth := &nomad.AuthConfig{
		Address: nomadAddr,
	}
	nomadClient, err := nomad.NewClient(auth)
	if err != nil {
		t.Fatalf("Failed to create nomad client: %v", err)
	}
	defer nomadClient.Close()

	// Fetch and write v1
	spec, err := nomadClient.FetchJobSpec("default", "rollback-test")
	if err != nil {
		t.Fatalf("Failed to fetch job: %v", err)
	}

	normalized := nomad.NormalizeJob(spec, cfg.Changes.IgnoreFields)
	hclBytes, err := hcl.FormatJobAsHCL(normalized)
	if err != nil {
		t.Fatalf("Failed to format HCL: %v", err)
	}

	if err := backend.WriteFile("global/default/rollback-test.hcl", hclBytes); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	commitV1, err := backend.Commit("Update rollback-test\n\nDeployed version 1")
	if err != nil {
		t.Fatalf("Failed to commit v1: %v", err)
	}

	if err := backend.Push(); err != nil {
		t.Fatalf("Failed to push v1: %v", err)
	}

	t.Logf("✅ Committed version 1: %s", commitV1)

	// Now update the job to version 2
	jobV2 := createTestJob()
	jobV2.Name = stringToPtr("rollback-test")
	jobV2.ID = stringToPtr("rollback-test")
	jobV2.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"image":   "busybox:latest",
		"command": "/bin/sh",
		"args":    []string{"-c", "echo 'version 2' && sleep 3600"},
	}

	_, _, err = apiClient.Jobs().Register(jobV2, nil)
	if err != nil {
		t.Fatalf("Failed to deploy job v2: %v", err)
	}
	t.Log("Deployed job version 2")

	// Sync version 2
	spec, err = nomadClient.FetchJobSpec("default", "rollback-test")
	if err != nil {
		t.Fatalf("Failed to fetch job v2: %v", err)
	}

	normalized = nomad.NormalizeJob(spec, cfg.Changes.IgnoreFields)
	hclBytes, err = hcl.FormatJobAsHCL(normalized)
	if err != nil {
		t.Fatalf("Failed to format HCL v2: %v", err)
	}

	if err := backend.WriteFile("global/default/rollback-test.hcl", hclBytes); err != nil {
		t.Fatalf("Failed to write file v2: %v", err)
	}

	commitV2, err := backend.Commit("Update rollback-test\n\nDeployed version 2")
	if err != nil {
		t.Fatalf("Failed to commit v2: %v", err)
	}

	if err := backend.Push(); err != nil {
		t.Fatalf("Failed to push v2: %v", err)
	}

	t.Logf("✅ Committed version 2: %s", commitV2)

	// Now test history retrieval by opening the repository
	repo, err := gitpkg.NewLocalRepository(cfg.Git.LocalPath)
	if err != nil {
		t.Fatalf("Failed to open repo: %v", err)
	}

	// Get history
	commits, err := repo.GetHistory("global/default/rollback-test.hcl", 0)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}

	if len(commits) < 2 {
		t.Fatalf("Expected at least 2 commits, got %d", len(commits))
	}

	t.Logf("✅ Found %d commits in history", len(commits))

	// The first commit in history should be v2 (most recent)
	// The second should be v1
	firstCommit := commits[0]
	secondCommit := commits[1]

	t.Logf("Latest commit: %s - %s", firstCommit.Hash, firstCommit.Message)
	t.Logf("Previous commit: %s - %s", secondCommit.Hash, secondCommit.Message)

	// Get file content from v1 commit
	v1Content, err := repo.GetFileAtCommit(secondCommit.FullHash, "global/default/rollback-test.hcl")
	if err != nil {
		t.Fatalf("Failed to get file at v1 commit: %v", err)
	}

	// Parse the HCL (pass Nomad address since ParseHCL makes a request to Nomad)
	jobFromV1, err := hcl.ParseHCL(v1Content, nomadAddr)
	if err != nil {
		t.Fatalf("Failed to parse HCL from v1: %v", err)
	}

	t.Logf("✅ Retrieved version 1 from commit %s", secondCommit.Hash)

	// Deploy v1 (rollback)
	evalID, err := nomadClient.DeployJob(jobFromV1)
	if err != nil {
		t.Fatalf("Failed to deploy v1: %v", err)
	}

	t.Logf("✅ Deployed version 1 (rollback), evaluation: %s", evalID)

	// Verify the job was actually deployed
	deployedJob, _, err := apiClient.Jobs().Info("rollback-test", nil)
	if err != nil {
		t.Fatalf("Failed to get deployed job info: %v", err)
	}

	if deployedJob == nil {
		t.Fatal("Deployed job is nil")
	}

	// Check that the job config matches v1 (has "version 1" in args)
	if len(deployedJob.TaskGroups) > 0 && len(deployedJob.TaskGroups[0].Tasks) > 0 {
		task := deployedJob.TaskGroups[0].Tasks[0]
		config, ok := task.Config["args"]
		if !ok {
			t.Fatal("Task has no args in config")
		}

		argsSlice, ok := config.([]interface{})
		if !ok {
			t.Fatalf("Args is not a slice: %T", config)
		}

		argsStr := fmt.Sprintf("%v", argsSlice)
		if !strings.Contains(argsStr, "version 1") {
			t.Errorf("Expected version 1 in args, got: %v", argsSlice)
		} else {
			t.Log("✅ Verified rollback successful - job has version 1 config")
		}
	}

	t.Log("✅ History and deploy (rollback) workflow complete!")
}
