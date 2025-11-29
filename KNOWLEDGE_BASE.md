# njgit Knowledge Base

This document contains accumulated knowledge about njgit — a comprehensive reference for understanding the entire application, its architecture, features, and design decisions.

## Table of Contents

- [Project Overview](#project-overview)
- [Architecture](#architecture)
- [File Structure](#file-structure)
- [Core Features](#core-features)
- [Backend System](#backend-system)
- [Region and Namespace Support](#region-and-namespace-support)
- [Configuration](#configuration)
- [Commands](#commands)
- [Testing](#testing)
- [Build and Deployment](#build-and-deployment)
- [Design Decisions](#design-decisions)
- [Common Patterns](#common-patterns)
- [Troubleshooting](#troubleshooting)

## Project Overview

### What is njgit?

njgit is a CLI tool that tracks changes to HashiCorp Nomad job configurations by storing them in Git. It automatically syncs job specifications from Nomad to a Git repository, providing version control and change history for your infrastructure.

### Key Capabilities

1. **Automatic Syncing**: Fetches job specs from Nomad and commits changes to Git
2. **Change Detection**: Only commits when jobs actually change (ignores Nomad metadata)
3. **Rollback/Deploy**: Deploy job configurations from any Git commit
4. **History**: View complete change history for any job
5. **Multi-Region**: Supports jobs across multiple Nomad regions and namespaces
6. **Flexible Backends**: Local Git or GitHub API for different use cases


## Architecture

### High-Level Components

```
┌─────────────────────────────────────────────────────┐
│                    CLI Layer                        │
│              (cobra commands)                       │
└─────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────┐
│                 Core Logic Layer                    │
│   (sync, deploy, history, show commands)            │
└─────────────────────────────────────────────────────┘
                        │
        ┌───────────────┴───────────────┐
        ▼                               ▼
┌──────────────────┐          ┌──────────────────────┐
│  Nomad Client    │          │  Backend Interface   │
│  (job fetch)     │          │  (git/github-api)    │
└──────────────────┘          └──────────────────────┘
```

### Package Structure

```
njgit/
├── cmd/njgit/           # Entry point
├── internal/
│   ├── backend/         # Storage backends (git, github-api)
│   ├── commands/        # CLI commands
│   ├── config/          # Configuration management
│   ├── git/             # Git operations wrapper
│   ├── github/          # GitHub API client
│   └── nomad/           # Nomad client wrapper
└── tests/               # Integration tests
```

### Data Flow

#### Sync Command Flow

```
1. Load config → 2. Connect to Nomad → 3. Fetch jobs
                                              │
                                              ▼
                                    4. Normalize job spec
                                       (remove metadata)
                                              │
                                              ▼
                                    5. Convert to HCL
                                              │
                                              ▼
                                    6. Compare with existing
                                              │
                    ┌─────────────────────────┴─────────────────────┐
                    ▼                                               ▼
            Changed: Write file                              Unchanged: Skip
                    │
                    ▼
            7. Git commit (local)
                    │
                    ▼
            8. User pushes manually
```

#### Deploy Command Flow

```
1. Load config → 2. Parse commit hash → 3. Fetch file from commit
                                                      │
                                                      ▼
                                            4. Parse region/namespace
                                               from file path
                                                      │
                                                      ▼
                                            5. Connect to Nomad
                                                      │
                                                      ▼
                                            6. Submit job to
                                               correct region/namespace
```

## File Structure

### Repository Organization

Jobs are stored in a hierarchical structure:

```
<region>/<namespace>/<job-name>.hcl
```

**Example:**
```
global/
  default/
    cache.hcl
    monitoring.hcl
  staging/
    test-app.hcl
us-east/
  production/
    web-app.hcl
    api-server.hcl
    worker.hcl
  development/
    dev-web.hcl
us-west/
  production/
    web-app.hcl
    processor.hcl
```

### Why This Structure?

1. **Mirrors Nomad Organization**: Matches how Nomad organizes jobs (region → namespace → job)
2. **Easy Navigation**: Find any job by knowing its region and namespace
3. **Multi-Region Support**: Same job name can exist in multiple regions
4. **Clear Deployment Target**: File path directly indicates where to deploy
5. **Git-Friendly**: Easy to filter commits by region or namespace

### Path Parsing

The tool parses region and namespace from file paths:

```go
// File: us-east/production/web-app.hcl
// Parsed as:
region    = "us-east"
namespace = "production"
jobName   = "web-app"
```

This is critical for:
- `deploy` command: Knows where to deploy
- `history` command: Filters commits by region/namespace
- `show` command: Finds correct file

## Core Features

### 1. Change Detection

**Problem**: Nomad adds metadata to job specs that changes on every fetch, causing false positives.

**Solution**: Normalize job specs before comparison.

**Implementation** (`internal/nomad/client.go`):

```go
func (c *Client) NormalizeJobSpec(job *api.Job) {
	// Remove fields that Nomad adds/modifies
	job.Status = nil
	job.StatusDescription = nil
	job.Version = nil
	job.SubmitTime = nil
	job.CreateIndex = nil
	job.ModifyIndex = nil
	job.JobModifyIndex = nil
	// ... more fields
}
```

**Result**: Only real configuration changes trigger commits.

### 2. HCL Conversion

**Why HCL?** 
- Human-readable format
- Native Nomad format
- Easy to review in Git diffs
- Can be manually edited

**Implementation**:
Uses Nomad's built-in JSON → HCL converter via `nomad job run -output` internally.

### 3. Auto-Detection

**Feature**: Deploy command can auto-detect job name from commit.

**How it works**:

```go
// Look at files changed in commit
// Example: us-east/production/web-app.hcl
// If user specified --region=us-east --namespace=production
// Only one file matches → auto-detect job name "web-app"
```

**Use case**:
```bash
# Instead of:
njgit deploy abc123 --job web-app --namespace production --region us-east

# Can do:
njgit deploy abc123 --namespace production --region us-east
# Job name auto-detected from commit!
```

### 4. Dry Run

**Feature**: Preview changes without actually executing.

**Available in**:
- `sync --dry-run`: Preview what would be committed
- `deploy --dry-run`: Preview what would be deployed

**Implementation**:
Commands check `--dry-run` flag and skip write operations:
```go
if !dryRun {
    backend.WriteFile(path, content)
    backend.Commit(message)
}
```

## Backend System

### Architecture

**Backend Interface** (`internal/backend/backend.go`):

```go
type Backend interface {
    Setup() error
    FileExists(path string) (bool, error)
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, content []byte) error
    Commit(message string) error
    // ... more methods
}
```

**Two Implementations**:
1. `GitBackend` - Local Git repository
2. `GitHubBackend` - GitHub REST API

### Git Backend (Default)

**Philosophy**: User manages the repository, tool just commits locally.

**Characteristics**:
- ✅ Local-only operations
- ✅ User controls push/pull
- ✅ Works offline
- ✅ No authentication needed
- ✅ Multi-file commits
- ✅ Any Git provider
- ❌ Manual setup required
- ❌ No automatic push

**Setup Requirements**:
```bash
git init -b main
git config user.name "njgit"
git config user.email "njgit@localhost"
```

**Configuration**:
```toml
[git]
backend = "git"
local_path = "/home/user/repositories"
repo_name = "nomad-jobs"
branch = "main"
```

**Implementation** (`internal/backend/git.go`):

```go
func (b *GitBackend) Setup() error {
    // Open existing repository (doesn't create it)
    repo, err := git.PlainOpen(repoPath)
    if err != nil {
        return fmt.Errorf("repository does not exist at %s", repoPath)
    }
    // No clone, no pull, no remote operations
}
```

### GitHub API Backend

**Philosophy**: Stateless, ephemeral, perfect for CI/CD.

**Characteristics**:
- ✅ No local repository
- ✅ Automatic push
- ✅ Perfect for containers/CI
- ✅ Minimal disk usage
- ❌ GitHub only
- ❌ Requires token
- ❌ One commit per file
- ❌ Requires network

**Configuration**:
```toml
[git]
backend = "github-api"
owner = "myorg"
repo = "nomad-jobs"
branch = "main"
```

**Authentication**:
```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
```

**Implementation** (`internal/backend/github.go`):

Uses GitHub REST API:
- `GET /repos/:owner/:repo/contents/:path` - Read files
- `PUT /repos/:owner/:repo/contents/:path` - Write files (creates commit)

**Limitation**: GitHub API doesn't support multi-file commits. Each file change = separate commit.

### When to Use Each Backend

| Scenario | Recommended Backend |
|----------|---------------------|
| Local development | Git |
| Manual review before push | Git |
| GitLab/Bitbucket/self-hosted | Git |
| Offline environments | Git |
| Kubernetes CronJob | GitHub API |
| CI/CD pipelines | GitHub API |
| Docker containers | GitHub API |
| AWS Lambda | GitHub API |

## Region and Namespace Support

### Background

Nomad has three levels of organization:
1. **Region**: Physical datacenter/geographical location (e.g., us-east, us-west, global)
2. **Namespace**: Logical grouping within a region (e.g., production, staging, default)
3. **Job**: The actual workload

### Implementation

**Data Model** (`internal/config/config.go`):

```go
type JobConfig struct {
    Name      string `mapstructure:"name"`
    Namespace string `mapstructure:"namespace"`
    Region    string `mapstructure:"region"`
}
```

**Defaults**:
- Namespace: `"default"` (Nomad's default namespace)
- Region: `"global"` (Nomad's default region)

**File Path Mapping**:
```
JobConfig{
    Name:      "web-app",
    Namespace: "production",
    Region:    "us-east",
}
→ us-east/production/web-app.hcl
```

### Commands with Region Support

All commands support region/namespace:

**Sync**:
```bash
# Config specifies region/namespace per job
[[jobs]]
name = "web-app"
namespace = "production"
region = "us-east"
```

**Deploy**:
```bash
njgit deploy abc123 --job web-app --namespace production --region us-east
```

**History**:
```bash
njgit history --job web-app --namespace production --region us-east
```

**Show**:
```bash
njgit show abc123 --job web-app --namespace production --region us-east
```

### Path Parsing in Deploy

**Critical Feature**: Deploy must parse region/namespace from file path to deploy to correct location.

**Implementation** (`internal/commands/deploy.go`):

```go
func detectJobFromCommit(cfg *config.Config, commitHash, region, namespace string) (string, error) {
    // Get files changed in commit
    commitInfo, _ := repo.GetCommitInfo(commitHash)
    
    // Build target path: region/namespace
    targetPath := filepath.Join(region, namespace)
    
    // Find matching files
    for _, file := range commitInfo.Files {
        dir := filepath.Dir(file)
        
        // Check if file is in target region/namespace
        if dir != targetPath {
            continue
        }
        
        // Extract job name from filename
        base := filepath.Base(file)
        if filepath.Ext(base) == ".hcl" {
            jobName := base[:len(base)-4]
            jobNames[jobName] = true
        }
    }
    
    // If exactly one job found, auto-detect it
    if len(jobNames) == 1 {
        return theJob, nil
    }
}
```

**Example**:
- Commit changes file: `us-east/production/web-app.hcl`
- User runs: `njgit deploy abc123 --region us-east --namespace production`
- Tool auto-detects: job name is "web-app"
- Deploys to: region=us-east, namespace=production

## Configuration

### Configuration Files

**Default Locations**:
1. `./njgit.toml` (current directory)
2. `~/.config/njgit/config.toml` (user config)
3. Custom path via `--config` flag

**Format**: TOML (Tom's Obvious, Minimal Language)

### Configuration Structure

```toml
# Nomad connection
[nomad]
address = "http://localhost:4646"
token = ""  # Or use NOMAD_TOKEN env var

# Git backend configuration
[git]
backend = "git"  # or "github-api"
local_path = "/home/user/repositories"
repo_name = "nomad-jobs"
branch = "main"
author_name = "njgit"
author_email = "njgit@localhost"

# Jobs to track
[[jobs]]
name = "web-app"
namespace = "production"
region = "us-east"

[[jobs]]
name = "api-server"
namespace = "production"
region = "us-east"

[[jobs]]
name = "cache"
namespace = "default"
region = "global"
```

### Environment Variable Overrides

**Pattern**: `NOMAD_CHANGELOG_<SECTION>_<KEY>`

**Examples**:
```bash
# Override Git branch
export NOMAD_CHANGELOG_GIT_BRANCH=develop

# Override Nomad address
export NOMAD_CHANGELOG_NOMAD_ADDRESS=http://nomad.prod:4646
```

**Special Environment Variables**:
```bash
NOMAD_ADDR              # Nomad address
NOMAD_TOKEN             # Nomad ACL token
GITHUB_TOKEN            # GitHub API token
GH_TOKEN                # Alternative to GITHUB_TOKEN
```

### Precedence Order

1. Command-line flags (highest priority)
2. Environment variables
3. Configuration file
4. Default values (lowest priority)

### Configuration Validation

**Three Levels**:

1. **`config show`**: Display current config (redact secrets)
2. **`config validate`**: Check syntax and required fields
3. **`config check`**: Full validation including connectivity tests

**Implementation** (`internal/commands/config.go`):

```go
func configCheck() error {
    // 1. Load config
    cfg, err := config.Load(configFile)
    
    // 2. Validate structure
    if err := cfg.Validate(); err != nil {
        return err
    }
    
    // 3. Test Nomad connection
    nomadClient := nomad.NewClient(cfg)
    if err := nomadClient.Ping(); err != nil {
        return err
    }
    
    // 4. Check jobs exist in Nomad
    for _, job := range cfg.Jobs {
        if _, err := nomadClient.GetJob(job.Name, job.Namespace); err != nil {
            return err
        }
    }
    
    // 5. Test backend connection
    backend := backend.New(cfg)
    if err := backend.Setup(); err != nil {
        return err
    }
}
```

## Commands

### Command Architecture

All commands follow this pattern:

```go
var myCmd = &cobra.Command{
    Use:   "mycommand",
    Short: "Short description",
    Long:  "Long description",
    RunE:  myCommandRun,
}

func myCommandRun(cmd *cobra.Command, args []string) error {
    // 1. Load configuration
    cfg, err := config.Load(configFile)
    
    // 2. Create clients
    nomadClient := nomad.NewClient(cfg)
    backend := backend.New(cfg)
    
    // 3. Execute business logic
    // ...
    
    // 4. Return error or nil
    return nil
}
```

### Sync Command Details

**Purpose**: Sync job specs from Nomad to Git.

**Algorithm**:
```
for each configured job:
    1. Fetch job spec from Nomad
    2. Normalize (remove metadata)
    3. Convert to HCL
    4. Check if file exists in repo
    5. If exists:
        a. Read existing content
        b. Compare with new content
        c. If different: write file, mark as changed
    6. If not exists:
        a. Write file, mark as new
    
After all jobs:
    1. If any changes: create single commit with all changes
    2. Report summary
```

**Commit Message Format**:
```
Update <region>/<namespace>/<job>
```

For multiple jobs in one sync:
```
Update multiple jobs

- us-east/production/web-app
- us-east/production/api-server
```

### Deploy Command Details

**Purpose**: Deploy job from Git commit to Nomad.

**Algorithm**:
```
1. Parse commit hash
2. If job name not specified:
    a. Get files changed in commit
    b. Filter by region/namespace
    c. If exactly 1 file: auto-detect job name
    d. If 0 or >1 files: error, ask user to specify
3. Read job HCL from commit
4. Parse region/namespace from file path
5. Connect to Nomad
6. Submit job to correct region/namespace
7. Report deployment status
```

**Dry Run**:
```bash
njgit deploy abc123 --job web-app --dry-run
```

Shows:
- Which file would be deployed
- Job content
- Target region/namespace
- But doesn't actually submit to Nomad

### History Command Details

**Purpose**: Show Git commit history.

**Options**:
- All jobs: `njgit history`
- Specific job: `njgit history --job web-app --namespace production --region us-east`
- Limit results: `njgit history --limit 5`

**Implementation**:
```go
func getHistory(filePath string, limit int) ([]CommitInfo, error) {
    // Use git log to get commits
    // If filePath specified: filter by that file
    // Parse commit info (hash, date, author, message, files)
    // Return up to 'limit' commits
}
```

**Output Format**:
```
Commit: abc123def456
Date:   2024-01-15 14:30:22
Author: njgit <njgit@localhost>

    Update us-east/production/web-app

Files:
  - us-east/production/web-app.hcl
```

### Show Command Details

**Purpose**: Display file content from specific commit.

**Usage**:
```bash
# Show all files in commit
njgit show abc123

# Show specific file
njgit show abc123 --job web-app --namespace production --region us-east
```

**Implementation**:
```go
func showCommit(commitHash, filePath string) (string, error) {
    // Use git show to get file at commit
    // Return file content
}
```

## Testing

### Test Structure

```
tests/
├── integration_test.go     # End-to-end tests
└── testdata/              # Test fixtures
```

### Integration Tests

**Philosophy**: Test the complete workflow, not individual functions.

**Test Cases**:

1. **Basic Sync**: Sync job to Git
2. **Change Detection**: Only commit when job actually changes
3. **Rollback**: Deploy from previous commit
4. **History**: View commit history
5. **Multi-file Commits**: Multiple jobs in one commit

**Example Test** (`tests/integration_test.go`):

```go
func TestRollback(t *testing.T) {
    // Setup: Create temp dir, init git repo
    // Create config with job
    // Create fake Nomad server
    
    // Version 1: Write initial job config
    backend.WriteFile("global/default/rollback-test.hcl", v1Content)
    backend.Commit("Initial version")
    
    // Version 2: Update job config
    backend.WriteFile("global/default/rollback-test.hcl", v2Content)
    backend.Commit("Update version")
    
    // Get commit history
    commits := repo.GetHistory("global/default/rollback-test.hcl", 0)
    
    // Deploy first commit (rollback)
    deployRun(cmd, []string{firstCommit.Hash})
    
    // Verify deployment
    deployed := nomadServer.GetLastDeployedJob()
    assert.Equal(t, v1Content, deployed)
}
```

### Running Tests

```bash
# All tests
./run-all-tests.sh

# Unit tests only
go test -v ./internal/...

# Integration tests only
go test -v ./tests/...

# Specific test
go test -v ./tests/ -run TestRollback
```

### Test Coverage

Current test coverage:
- ✅ Sync command
- ✅ Deploy command with auto-detection
- ✅ History filtering
- ✅ Change detection
- ✅ Region/namespace path parsing
- ✅ Git backend operations
- ✅ Configuration loading

### Mock Nomad Server

Tests use a fake Nomad server for predictable testing:

```go
type FakeNomadServer struct {
    jobs map[string]*api.Job
    lastDeployed *api.Job
}

func (s *FakeNomadServer) GetJob(name, namespace string) (*api.Job, error) {
    return s.jobs[name], nil
}

func (s *FakeNomadServer) RegisterJob(job *api.Job) error {
    s.lastDeployed = job
    return nil
}
```

## Build and Deployment

### Build Process

**Simple Build**:
```bash
go build -o njgit ./cmd/njgit
```

**Cross-Platform Build**:
```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o njgit-linux-amd64 ./cmd/njgit

# macOS
GOOS=darwin GOARCH=amd64 go build -o njgit-darwin-amd64 ./cmd/njgit
GOOS=darwin GOARCH=arm64 go build -o njgit-darwin-arm64 ./cmd/njgit

# Windows
GOOS=windows GOARCH=amd64 go build -o njgit-windows-amd64.exe ./cmd/njgit
```

### Dependencies

**Direct Dependencies**:
- `github.com/hashicorp/nomad/api` - Nomad client
- `github.com/go-git/go-git/v5` - Git operations
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `github.com/google/go-github/v57` - GitHub API client

**Why These Choices**:
- **Nomad API**: Official HashiCorp client
- **go-git**: Pure Go git library (no external git binary needed)
- **Cobra**: Industry standard for Go CLIs
- **Viper**: Handles config files + env vars + defaults
- **go-github**: Official GitHub API client

### Release Process

1. **Tag Version**:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **Build Binaries**:
   ```bash
   ./build-all.sh
   ```

3. **Create Release**:
   - Upload binaries to GitHub Releases
   - Write changelog
   - Include installation instructions

### Installation

**From Source**:
```bash
git clone https://github.com/myorg/njgit.git
cd njgit
go build -o njgit ./cmd/njgit
mv njgit /usr/local/bin/
```

**From Binary**:
```bash
wget https://github.com/myorg/njgit/releases/latest/download/njgit-linux-amd64
chmod +x njgit-linux-amd64
mv njgit-linux-amd64 /usr/local/bin/njgit
```

## Design Decisions

### 1. Local-Only Git Backend

**Decision**: Git backend doesn't do any remote operations (clone/push/pull).

**Why**:
- ✅ User has full control over when to push
- ✅ Can review commits before pushing
- ✅ Works offline
- ✅ No authentication complexity
- ✅ Works with any Git provider
- ✅ Simpler code

**Trade-offs**:
- ❌ User must initialize repository manually
- ❌ No automatic push (user must push manually)

**Alternative Considered**: Auto-clone/push with authentication.

**Rejected Because**: 
- Too complex (SSH keys, tokens, known_hosts)
- Less control for users
- Doesn't work offline
- Git provider-specific quirks

### 2. Region/Namespace/Job File Structure

**Decision**: Use `region/namespace/job.hcl` file structure.

**Why**:
- ✅ Mirrors Nomad's organization model
- ✅ Clear where each job belongs
- ✅ Easy to find jobs
- ✅ Supports multi-region deployments
- ✅ File path encodes deployment target

**Trade-offs**:
- ❌ Longer file paths
- ❌ More directories

**Alternative Considered**: Flat structure with job names only.

**Rejected Because**:
- Can't have same job name in different regions
- No clear deployment target
- Harder to organize large deployments

### 3. HCL Format for Storage

**Decision**: Store jobs as HCL files, not JSON.

**Why**:
- ✅ Human-readable
- ✅ Native Nomad format
- ✅ Easy to review in Git diffs
- ✅ Can be manually edited
- ✅ Comments supported

**Trade-offs**:
- ❌ Requires conversion from Nomad's JSON API

**Alternative Considered**: Store as JSON.

**Rejected Because**:
- JSON is harder to read
- JSON diffs are harder to review
- Not idiomatic for Nomad

### 4. Normalization for Change Detection

**Decision**: Strip Nomad metadata before comparing jobs.

**Why**:
- ✅ Only commit real configuration changes
- ✅ Avoid false positives
- ✅ Clean Git history

**Fields Removed**:
```go
job.Status
job.StatusDescription
job.Version
job.SubmitTime
job.CreateIndex
job.ModifyIndex
job.JobModifyIndex
```

**Trade-offs**:
- ❌ Can't track Nomad-internal changes
- ❌ Must maintain list of fields to remove

**Alternative Considered**: Store everything, compare everything.

**Rejected Because**:
- Creates noise in Git history
- Every sync would show "changes" even when config identical

### 5. Two Backend Options

**Decision**: Support both local Git and GitHub API backends.

**Why**:
- ✅ Git backend for local development and control
- ✅ GitHub API backend for ephemeral CI/CD environments
- ✅ Each optimized for its use case

**Trade-offs**:
- ❌ More code to maintain
- ❌ Two different behaviors to document

**Alternative Considered**: Only one backend.

**Rejected Because**:
- Local Git doesn't work well in ephemeral containers
- GitHub API doesn't give local control

### 6. Auto-Detection in Deploy

**Decision**: Auto-detect job name from commit when possible.

**Why**:
- ✅ Better UX (less typing)
- ✅ Reduces errors
- ✅ Still allows explicit specification

**How**:
- If commit has one job in specified region/namespace → auto-detect
- If commit has multiple jobs → require explicit --job flag

**Trade-offs**:
- ❌ Slightly more complex logic
- ❌ Can be surprising if auto-detection unexpected

**Alternative Considered**: Always require --job flag.

**Rejected Because**:
- Worse UX
- Most rollbacks are single-job

## Common Patterns

### Pattern 1: Manual Push Workflow

**Scenario**: Review commits before pushing to remote.

**Steps**:
```bash
# 1. Sync (commits locally)
njgit sync

# 2. Review commits
cd /path/to/repo
git log --oneline -5
git show HEAD

# 3. Review diff with remote
git diff origin/main

# 4. Push when satisfied
git push origin main
```

### Pattern 2: Multi-Region Deployment

**Scenario**: Same job deployed to multiple regions.

**Configuration**:
```toml
# US East
[[jobs]]
name = "web-app"
namespace = "production"
region = "us-east"

# US West (same job, different region)
[[jobs]]
name = "web-app"
namespace = "production"
region = "us-west"
```

**Result**:
```
us-east/production/web-app.hcl
us-west/production/web-app.hcl
```

**Deploy to specific region**:
```bash
# Deploy US East only
njgit deploy abc123 --job web-app --namespace production --region us-east

# Deploy US West only
njgit deploy def456 --job web-app --namespace production --region us-west
```

### Pattern 3: Scheduled Syncing

**Scenario**: Automatic sync every hour.

**Cron**:
```bash
# Run every hour
0 * * * * /usr/local/bin/njgit sync

# With log
0 * * * * /usr/local/bin/njgit sync >> /var/log/njgit-sync.log 2>&1
```

**Systemd Timer**:
```ini
# /etc/systemd/system/njgit-sync.timer
[Unit]
Description=njgit sync timer

[Timer]
OnCalendar=hourly
Persistent=true

[Install]
WantedBy=timers.target
```

```ini
# /etc/systemd/system/njgit-sync.service
[Unit]
Description=njgit sync service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/njgit sync
```

### Pattern 4: CI/CD Integration

**Scenario**: Sync from GitHub Actions.

**Workflow**:
```yaml
name: Sync Nomad Jobs
on:
  schedule:
    - cron: '0 * * * *'

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup njgit
        run: |
          wget https://github.com/myorg/njgit/releases/latest/download/njgit-linux-amd64
          chmod +x njgit-linux-amd64
          sudo mv njgit-linux-amd64 /usr/local/bin/njgit
      
      - name: Check config
        env:
          GITHUB_TOKEN: ${{ secrets.REPO_TOKEN }}
          NOMAD_TOKEN: ${{ secrets.NOMAD_TOKEN }}
        run: njgit config check
      
      - name: Sync jobs
        env:
          GITHUB_TOKEN: ${{ secrets.REPO_TOKEN }}
          NOMAD_TOKEN: ${{ secrets.NOMAD_TOKEN }}
        run: njgit sync
```

### Pattern 5: Emergency Rollback

**Scenario**: Production is broken, need to rollback immediately.

**Steps**:
```bash
# 1. Find last known good commit
njgit history --job web-app --namespace production --region us-east --limit 10

# 2. Verify the commit content
njgit show abc123 --job web-app --namespace production --region us-east

# 3. Deploy (rollback)
njgit deploy abc123 --job web-app --namespace production --region us-east

# 4. Verify in Nomad
nomad job status -namespace=production web-app
```

## Troubleshooting

### Problem: "repository does not exist"

**Cause**: Git repository not initialized.

**Solution**:
```bash
cd /path/to/repo
git init -b main
git config user.name "njgit"
git config user.email "njgit@localhost"
```

### Problem: "not a git repository"

**Cause**: Directory exists but isn't a Git repository.

**Solution**:
```bash
cd /path/to/directory
git init -b main
```

### Problem: "failed to connect to Nomad"

**Cause**: Nomad address incorrect or Nomad not reachable.

**Solution**:
```bash
# Check Nomad address
echo $NOMAD_ADDR

# Test connection
curl $NOMAD_ADDR/v1/agent/self

# Set correct address
export NOMAD_ADDR=http://correct-nomad:4646
```

### Problem: "github token is required"

**Cause**: Using github-api backend without token.

**Solution**:
```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
```

### Problem: "job not found"

**Cause**: Job doesn't exist in Nomad, or wrong namespace/region.

**Solution**:
```bash
# Check job exists
nomad job status -namespace=production web-app

# Check region
nomad job status -namespace=production -region=us-east web-app

# Update config with correct namespace/region
```

### Problem: "multiple jobs changed in commit"

**Cause**: Trying to auto-detect job, but commit has multiple jobs.

**Solution**:
```bash
# Specify job explicitly
njgit deploy abc123 --job web-app --namespace production --region us-east
```

### Problem: Changes detected every sync

**Cause**: Normalization might be missing some Nomad metadata fields.

**Investigation**:
```bash
# Run sync with verbose output
njgit --verbose sync

# Check what's changing
cd /path/to/repo
git diff
```

**Solution**:
Add missing fields to normalization in `internal/nomad/client.go`.

### Problem: Deploy fails with "invalid HCL"

**Cause**: HCL file corrupted or manually edited incorrectly.

**Solution**:
```bash
# Validate HCL syntax
nomad job validate /path/to/file.hcl

# Or check file content
njgit show abc123 --job web-app --namespace production --region us-east
```

---

## Quick Reference

### File Paths
```
region/namespace/job.hcl
```

### Configuration Precedence
```
Command flags > Env vars > Config file > Defaults
```

### Common Commands
```bash
# Setup
git init -b main
njgit init
njgit config check

# Daily use
njgit sync
njgit history
njgit deploy <commit>

# Troubleshooting
njgit config check
njgit --verbose sync
```

### Environment Variables
```bash
NOMAD_ADDR              # Nomad address
NOMAD_TOKEN             # Nomad token
GITHUB_TOKEN            # GitHub token
NOMAD_CHANGELOG_*       # Config overrides
```

### Backend Comparison
```
Git:        Local control, manual push, offline
GitHub API: Automatic push, ephemeral, CI/CD
```

---

This knowledge base should help you quickly get back up to speed on any aspect of njgit in future sessions!
