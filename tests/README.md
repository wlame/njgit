# Integration Tests

This directory contains integration tests for njgit using testcontainers-go.

## Overview

The integration tests validate the complete workflow by:
1. Starting a real Nomad server in a Docker container (dev mode)
2. Deploying test jobs to Nomad
3. Fetching, normalizing, and converting jobs to HCL
4. Committing changes to a local Git repository
5. Verifying the end-to-end sync workflow

## Prerequisites

You need Docker running on your system to run these tests. The tests use [testcontainers-go](https://github.com/testcontainers/testcontainers-go) which automatically manages container lifecycle.

### Starting Docker

**If using Colima:**
```bash
colima start
```

**If using Docker Desktop:**
Make sure Docker Desktop is running.

**Verify Docker is running:**
```bash
docker ps
```

## Running the Tests

### Run all integration tests:
```bash
go test -v ./tests/integration_test.go
```

### Run a specific test:
```bash
# Test Nomad container connectivity
go test -v ./tests/integration_test.go -run TestNomadContainer

# Test fetching and normalizing jobs (shows Nomad API responses)
go test -v ./tests/integration_test.go -run TestFetchAndNormalizeJob

# Test Git operations
go test -v ./tests/integration_test.go -run TestGitRepository

# Test complete sync workflow
go test -v ./tests/integration_test.go -run TestFullSyncWorkflow
```

### Skip integration tests:
```bash
go test -v ./tests/integration_test.go -short
```

## What Each Test Does

### 1. TestNomadContainer
Basic smoke test that:
- Starts a Nomad container in dev mode
- Verifies we can connect to the Nomad API
- Tests the Ping() method

**Purpose:** Ensures testcontainers setup works correctly.

### 2. TestFetchAndNormalizeJob
This test demonstrates the core job processing pipeline:
- Starts Nomad container
- Deploys a simple busybox test job
- Fetches the job using the Nomad API
- **Logs detailed information about Nomad API responses** (job type, region, datacenters, modify index, submit time, task groups, etc.)
- Normalizes the job (removes metadata)
- Converts to HCL format

**Purpose:** Shows exactly what data the Nomad API returns and validates our normalization logic.

**Example output:**
```
Job details:
  Type: service
  Region: global
  Datacenters: [dc1]
  ModifyIndex: 12
  SubmitTime: 1234567890
  Task Groups: 1
  Task Group[0]: test-group
    Count: 1
    Task[0]: test-task
      Driver: docker
      Docker Image: busybox:latest
✅ Job normalized successfully
✅ Converted to HCL (XXX bytes)
HCL output:
job "test-job" {
  datacenters = ["dc1"]
  type = "service"
  ...
}
```

### 3. TestGitRepository
Tests Git operations:
- Creates a bare Git repository (simulates remote)
- Clones the repository
- Writes files to the working directory
- Reads files back
- Stages, commits, and pushes changes

**Purpose:** Validates that our Git integration works correctly with local repositories.

### 4. TestFullSyncWorkflow
End-to-end test of the complete sync process:
- Starts Nomad container
- Deploys a test job
- Creates a bare Git repository
- Fetches the job from Nomad
- Normalizes and converts to HCL
- Writes to Git repository
- Commits and pushes
- Fetches the job again (unchanged)
- Verifies change detection works (HCL should be identical)

**Purpose:** Validates that all components work together correctly.

## Test Structure

### Helper Functions

**startNomadContainer(ctx)**: Starts a Nomad container in dev mode
- Uses `hashicorp/nomad:latest` image
- Exposes port 4646
- Runs with `-dev -bind=0.0.0.0` for development mode
- Waits for "Nomad agent started" log message

**deployTestJob(t, nomadAddr)**: Deploys a test job to Nomad
- Creates a simple busybox Docker job
- Registers it using the Nomad API
- Returns the job ID

**createTestJob()**: Creates a test job specification
- Simple service job
- One task group with one task
- Runs `busybox:latest` with sleep command
- Uses minimal resources (100 CPU, 128MB RAM)

**runCommand(dir, name, args...)**: Executes shell commands
- Used for creating bare Git repositories
- Uses `os/exec` package

## Exploring Nomad API Responses

The `TestFetchAndNormalizeJob` test is specifically designed to help you understand what the Nomad API returns. When you run this test, it will:

1. Deploy a real job to Nomad
2. Fetch it using the API
3. Log all important fields from the API response
4. Show the normalized version
5. Display the final HCL output

This is useful for:
- Understanding what metadata Nomad adds
- Seeing what fields need to be ignored for change detection
- Verifying our HCL formatting matches Nomad's expectations
- Learning about the Nomad API structure

## Test Data

Tests use temporary directories that are automatically cleaned up:
- Nomad runs in a container (automatically removed after test)
- Git bare repositories use `os.MkdirTemp()` with automatic cleanup
- No persistent state between test runs

## Troubleshooting

### "rootless Docker not found"
Docker is not running. Start Docker or Colima:
```bash
colima start
# or
# Start Docker Desktop
```

### "Cannot connect to the Docker daemon"
Docker socket is not accessible. Verify Docker is running:
```bash
docker ps
```

### Tests are slow
Integration tests need to download Docker images on first run:
- `hashicorp/nomad:latest` (~100MB)

Subsequent runs will be faster as images are cached.

### Port conflicts
If port 4646 is already in use, testcontainers will automatically map to a different host port. The tests use the dynamically allocated port, so this should not cause issues.

## CI/CD Integration

To run these tests in CI:
```yaml
# GitHub Actions example
- name: Run integration tests
  run: |
    # Start Docker if needed
    go test -v ./tests/integration_test.go
```

For environments without Docker:
```bash
# Skip integration tests
go test -short ./...
```
