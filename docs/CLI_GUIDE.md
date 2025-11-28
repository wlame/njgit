# CLI Usage Guide

Complete guide to using ndiff from the command line.

## Table of Contents

- [Getting Started](#getting-started)
- [Command Reference](#command-reference)
- [Common Workflows](#common-workflows)
- [Environment Variables](#environment-variables)
- [Exit Codes](#exit-codes)

## Getting Started

### First-Time Setup

When you first download ndiff, follow these steps:

#### 1. Initialize Git Repository

ndiff uses a local Git repository that you manage. Initialize it first:

```bash
# Option A: Initialize in current directory
git init -b main
git config user.name "ndiff"
git config user.email "ndiff@localhost"

# Option B: Initialize in a specific directory
mkdir -p ~/repositories/nomad-jobs
cd ~/repositories/nomad-jobs
git init -b main
git config user.name "ndiff"
git config user.email "ndiff@localhost"

# Optional: Add a remote (for manual push/pull)
git remote add origin git@github.com:myorg/nomad-jobs.git
```

#### 2. Initialize Configuration

Run the interactive setup wizard:

```bash
ndiff init
```

This will guide you through:
- Choosing a backend (Git or GitHub API)
- Configuring repository settings
- Setting up Nomad connection
- Adding jobs to track (with regions and namespaces)

Example session:
```
$ ndiff init

🚀 Welcome to ndiff setup!

This wizard will help you create a configuration file.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📦 Backend Selection
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Choose a backend for storing job configurations:
  1) Git Backend (default)
  2) GitHub API Backend

Select backend (git/github-api) [git]: git

Local path (directory containing repository) [.]: /home/user/repositories
Repository name [ndiff-repo]: nomad-jobs
Branch [main]: main

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🔧 Nomad Configuration
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Nomad address [http://localhost:4646]: http://nomad.example.com:4646

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 Jobs to Track
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Job 1 name (empty to finish): web-app
  Namespace [default]: production
  Region [global]: us-east

Job 2 name (empty to finish): api-server
  Namespace [default]: production
  Region [global]: us-east

Job 3 name (empty to finish): cache
  Namespace [default]: default
  Region [global]: global

Job 4 name (empty to finish):

✅ Configuration created successfully!
   Saved to: ndiff.toml
```

#### 3. Verify Configuration

Check that everything is set up correctly:

```bash
ndiff config check
```

This performs comprehensive checks:
- ✅ Configuration file syntax
- ✅ Connection to Nomad
- ✅ Backend access (Git/GitHub)
- ✅ Configured jobs exist
- ✅ Authentication and permissions

Example output:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🔍 Checking ndiff configuration...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1️⃣  Loading configuration file...
   ✅ Configuration file loaded

2️⃣  Validating configuration...
   ✅ Configuration is valid

   📋 Configuration Summary:
      Backend: git
      Local path: /home/user/repositories
      Repository: nomad-jobs
      Branch: main
      Nomad: http://nomad.example.com:4646
      Jobs to track: 3

3️⃣  Testing Nomad connection...
   ✅ Successfully connected to Nomad

4️⃣  Checking configured jobs in Nomad...
   ✅ Job found: us-east/production/web-app
   ✅ Job found: us-east/production/api-server
   ✅ Job found: global/default/cache
   ✅ 3 job(s) found in Nomad

5️⃣  Testing backend connection...
   ✅ Successfully connected to backend (git)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 Check Summary
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

   ✅ Passed: 5

✅ All checks passed! You're ready to use ndiff.
```

#### 4. Start Syncing

Once checks pass, start syncing your jobs:

```bash
# Preview what would change (dry run)
ndiff sync --dry-run

# Actually sync changes
ndiff sync
```

## Command Reference

### `ndiff init`

Interactive setup wizard for first-time configuration.

**Usage:**
```bash
ndiff init [flags]
```

**Flags:**
- `--force` - Overwrite existing configuration file

**Examples:**
```bash
# First-time setup
ndiff init

# Recreate configuration
ndiff init --force
```

**What it does:**
1. Guides you through backend selection
2. Collects repository settings
3. Configures Nomad connection
4. Lets you add jobs to track (with regions and namespaces)
5. Creates `ndiff.toml`

---

### `ndiff config`

Manage configuration settings.

#### `config show`

Display current configuration with sensitive values redacted.

**Usage:**
```bash
ndiff config show
```

**Example output:**
```json
{
  "Git": {
    "backend": "git",
    "local_path": "/home/user/repositories",
    "repo_name": "nomad-jobs",
    "branch": "main"
  },
  "Nomad": {
    "address": "http://localhost:4646",
    "token": "********"
  },
  "Jobs": [
    {
      "name": "web-app",
      "namespace": "production",
      "region": "us-east"
    },
    {
      "name": "cache",
      "namespace": "default",
      "region": "global"
    }
  ]
}
```

#### `config validate`

Validate configuration file syntax and required fields.

**Usage:**
```bash
ndiff config validate
```

**Example:**
```bash
$ ndiff config validate
✅ Configuration is valid
ℹ  Git repository: /home/user/repositories/nomad-jobs
ℹ  Nomad address: http://localhost:4646
ℹ  Tracking 3 jobs across 2 regions
```

#### `config check`

Comprehensive configuration verification with connectivity tests.

**Usage:**
```bash
ndiff config check
```

**What it checks:**
1. Configuration file loads correctly
2. All required fields are present
3. Nomad cluster is reachable
4. Configured jobs exist in Nomad
5. Backend (Git/GitHub) is accessible
6. Authentication works

**Use this after:**
- Initial setup
- Changing configuration
- Troubleshooting connection issues
- Setting up in a new environment

---

### `ndiff sync`

Sync Nomad job configurations to your repository.

**Usage:**
```bash
ndiff sync [flags]
```

**Flags:**
- `--dry-run` - Show what would change without committing
- `--jobs string` - Comma-separated list of jobs to sync (default: all)
- `--verbose` - Show detailed output

**Examples:**

```bash
# Sync all configured jobs
ndiff sync

# Preview changes without committing
ndiff sync --dry-run

# Sync specific jobs only
ndiff sync --jobs web-app,api-server

# Verbose output
ndiff sync --verbose
```

**How it works:**

1. Connects to Nomad
2. Fetches job specifications
3. Normalizes jobs (removes metadata)
4. Converts to HCL format
5. Compares with existing files
6. For changed jobs:
   - Writes HCL file to `region/namespace/job.hcl`
   - Creates commit
   - Commits locally (you push manually when ready)

**Output:**
```bash
$ ndiff sync

ℹ  Starting sync...
ℹ  Loading configuration...
ℹ  Nomad: http://nomad.example.com:4646
ℹ  Backend: git (/home/user/repositories/nomad-jobs)
ℹ  Connecting to Nomad...
✅ Connected to Nomad
ℹ  Setting up backend...
✅ Backend ready (git)
ℹ  Syncing 3 jobs...
ℹ  Checking us-east/production/web-app...
ℹ    us-east/production/web-app: CHANGED
ℹ  Checking us-east/production/api-server...
ℹ  Checking global/default/cache...
✅ Synced 1 jobs with changes:
  - us-east/production/web-app

# Now you can review and push manually:
# cd /home/user/repositories/nomad-jobs
# git log --oneline
# git push origin main
```

**File Organization:**

Jobs are stored in a hierarchical structure:
```
region/namespace/job-name.hcl
```

Example:
```
us-east/
  production/
    web-app.hcl
    api-server.hcl
  staging/
    test-app.hcl
us-west/
  production/
    worker.hcl
global/
  default/
    cache.hcl
```

---

### `ndiff deploy`

Deploy a job from a specific Git commit (rollback or rollforward).

**Usage:**
```bash
ndiff deploy [commit-hash] [flags]
```

**Flags:**
- `--job string` - Job name (optional, auto-detected from commit if not specified)
- `--namespace string` - Nomad namespace (default: "default")
- `--region string` - Nomad region (default: "global")
- `--dry-run` - Show what would be deployed without actually deploying

**Examples:**

```bash
# Auto-detect job from commit (single job in commit)
ndiff deploy abc123

# Specify job explicitly
ndiff deploy abc123 --job web-app

# Specify job with namespace and region
ndiff deploy abc123 --job web-app --namespace production --region us-east

# Preview deployment without actually deploying
ndiff deploy abc123 --job web-app --dry-run

# Verbose output
ndiff deploy abc123 --job web-app --verbose
```

**Auto-detection:**

If you don't specify `--job`, ndiff will try to auto-detect it from the commit:
- Looks at files changed in the commit
- Filters by specified region/namespace
- If exactly one job matches, uses it automatically
- If multiple jobs match, prompts you to specify which one

**Example auto-detection:**
```bash
$ ndiff deploy abc123 --namespace production --region us-east

ℹ  Detecting job from commit abc123...
✅ Auto-detected job: web-app (from us-east/production/web-app.hcl)
ℹ  Deploying us-east/production/web-app from commit abc123...
✅ Job deployed successfully!
```

**Example with multiple jobs:**
```bash
$ ndiff deploy abc123

❌ Multiple jobs changed in this commit:
  - us-east/production/web-app
  - us-east/production/api-server

Please specify which job to deploy with --job flag.
```

**How it works:**

1. Fetches job specification from commit
2. Parses region/namespace from file path: `region/namespace/job.hcl`
3. Validates HCL syntax
4. Connects to Nomad
5. Submits job to correct region and namespace
6. Reports deployment status

**Use cases:**
- **Rollback**: Deploy a previous version after a bad deployment
- **Rollforward**: Re-deploy a specific version
- **Testing**: Deploy historical configurations for testing

---

### `ndiff history`

View commit history for jobs.

**Usage:**
```bash
ndiff history [flags]
```

**Flags:**
- `--job string` - Filter by specific job
- `--namespace string` - Namespace (used with --job, default: "default")
- `--region string` - Region (used with --job, default: "global")
- `--limit int` - Maximum number of commits to show (default: 10, 0 = all)

**Examples:**

```bash
# Show recent commits for all jobs
ndiff history

# Show all commits (no limit)
ndiff history --limit 0

# Show history for specific job
ndiff history --job web-app --namespace production --region us-east

# Show last 5 commits for a job
ndiff history --job web-app --namespace production --region us-east --limit 5

# Show history for job in default namespace and global region
ndiff history --job cache
```

**Output:**
```bash
$ ndiff history --job web-app --namespace production --region us-east

ℹ  Filtering by job: us-east/production/web-app

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📜 Commit History (10 most recent)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Commit: a1b2c3d
Date:   2024-01-15 14:30:22
Author: ndiff <ndiff@localhost>

    Update us-east/production/web-app

Files:
  - us-east/production/web-app.hcl

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Commit: d4e5f6g
Date:   2024-01-14 10:15:33
Author: ndiff <ndiff@localhost>

    Update us-east/production/web-app

Files:
  - us-east/production/web-app.hcl

...
```

**File path format:**

History shows files in the format: `region/namespace/job.hcl`

This makes it easy to:
- See which region/namespace a change belongs to
- Use with `deploy` command
- Understand the scope of changes

---

### `ndiff show`

Display job configuration from a specific commit.

**Usage:**
```bash
ndiff show [commit-hash] [flags]
```

**Flags:**
- `--job string` - Job name to show (optional, shows all if not specified)
- `--namespace string` - Namespace (used with --job, default: "default")
- `--region string` - Region (used with --job, default: "global")

**Examples:**

```bash
# Show all jobs changed in a commit
ndiff show abc123

# Show specific job from commit
ndiff show abc123 --job web-app --namespace production --region us-east

# Show job in default namespace and global region
ndiff show abc123 --job cache
```

**Output:**
```bash
$ ndiff show abc123 --job web-app --namespace production --region us-east

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📄 File: us-east/production/web-app.hcl
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

job "web-app" {
  datacenters = ["dc1"]
  type        = "service"
  
  group "web" {
    count = 3
    
    task "app" {
      driver = "docker"
      
      config {
        image = "myapp:v1.2.3"
      }
    }
  }
}
```

**Use cases:**
- Review what changed in a commit
- Compare different versions
- Verify configuration before deploying

---

### Global Flags

These flags work with all commands:

- `--config string` - Path to config file (default: ndiff.toml)
- `--verbose` - Enable verbose output
- `--help` - Show help for any command

**Examples:**
```bash
# Use custom config file
ndiff --config /etc/ndiff.toml sync

# Verbose output
ndiff --verbose config check

# Get help
ndiff --help
ndiff sync --help
```

---

## Common Workflows

### Scenario 1: First-Time Setup (Local Development)

You've just installed ndiff and want to track your Nomad jobs.

```bash
# Step 1: Initialize Git repository
mkdir -p ~/repositories/nomad-jobs
cd ~/repositories/nomad-jobs
git init -b main
git config user.name "ndiff"
git config user.email "ndiff@localhost"

# Optional: Add remote for backups
git remote add origin git@github.com:myorg/nomad-jobs.git

# Step 2: Create configuration
ndiff init
# Choose: Git backend
# Local path: /home/user/repositories
# Repo name: nomad-jobs
# Add jobs with their regions and namespaces

# Step 3: Verify everything works
ndiff config check

# Step 4: Test with dry run
ndiff sync --dry-run

# Step 5: Start syncing
ndiff sync

# Step 6: Review commits and push
cd ~/repositories/nomad-jobs
git log --oneline
git push origin main
```

### Scenario 2: Multi-Region Setup

You have jobs in multiple Nomad regions.

**Configuration** (ndiff.toml):
```toml
[nomad]
address = "http://nomad.example.com:4646"

[git]
backend = "git"
local_path = "/home/user/repositories"
repo_name = "nomad-jobs"
branch = "main"

# US East region
[[jobs]]
name = "web-app"
namespace = "production"
region = "us-east"

[[jobs]]
name = "api-server"
namespace = "production"
region = "us-east"

# US West region
[[jobs]]
name = "web-app"
namespace = "production"
region = "us-west"

[[jobs]]
name = "worker"
namespace = "production"
region = "us-west"

# Global region
[[jobs]]
name = "cache"
namespace = "default"
region = "global"
```

**Sync all regions:**
```bash
ndiff sync
```

**Result in repository:**
```
us-east/
  production/
    web-app.hcl
    api-server.hcl
us-west/
  production/
    web-app.hcl
    worker.hcl
global/
  default/
    cache.hcl
```

**Deploy to specific region:**
```bash
# Deploy US East web-app
ndiff deploy abc123 --job web-app --namespace production --region us-east

# Deploy US West web-app (different region, same job name)
ndiff deploy def456 --job web-app --namespace production --region us-west
```

**View history per region:**
```bash
# US East web-app history
ndiff history --job web-app --namespace production --region us-east

# US West web-app history
ndiff history --job web-app --namespace production --region us-west
```

### Scenario 3: Rolling Back a Deployment

You deployed a bad configuration and need to rollback.

```bash
# Step 1: Find the last good version
ndiff history --job web-app --namespace production --region us-east

# Output shows:
# Commit: bad123  (current - broken)
# Commit: good456 (last working version)

# Step 2: Preview the rollback
ndiff show good456 --job web-app --namespace production --region us-east

# Step 3: Test deployment (dry run)
ndiff deploy good456 --job web-app --namespace production --region us-east --dry-run

# Step 4: Execute rollback
ndiff deploy good456 --job web-app --namespace production --region us-east

# Step 5: Verify in Nomad
nomad job status -namespace=production web-app
```

### Scenario 4: CI/CD Setup (GitHub Actions)

Setting up ndiff in GitHub Actions workflow.

**Configuration file** (ndiff.toml):
```toml
[git]
backend = "github-api"
owner = "myorg"
repo = "nomad-jobs"
branch = "main"

[nomad]
address = "https://nomad.prod.internal:4646"

[[jobs]]
name = "web-app"
namespace = "production"
region = "us-east"
```

**GitHub Actions workflow**:
```yaml
name: Sync Nomad Jobs
on:
  schedule:
    - cron: '0 * * * *'  # Every hour

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Download ndiff
        run: |
          wget https://github.com/myorg/ndiff/releases/latest/download/ndiff-linux-amd64
          chmod +x ndiff-linux-amd64
          mv ndiff-linux-amd64 /usr/local/bin/ndiff
      
      - name: Verify configuration
        env:
          GITHUB_TOKEN: ${{ secrets.REPO_TOKEN }}
          NOMAD_TOKEN: ${{ secrets.NOMAD_TOKEN }}
        run: ndiff config check
      
      - name: Sync jobs
        env:
          GITHUB_TOKEN: ${{ secrets.REPO_TOKEN }}
          NOMAD_TOKEN: ${{ secrets.NOMAD_TOKEN }}
        run: ndiff sync
```

### Scenario 5: Troubleshooting Connection Issues

You're having problems connecting to Nomad or your repository.

```bash
# Step 1: Verify configuration syntax
ndiff config validate

# Step 2: Run comprehensive check
ndiff config check

# Step 3: If checks fail, debug specific issues:

# For Nomad connection issues:
export NOMAD_ADDR=http://your-nomad:4646
export NOMAD_TOKEN=your-token
ndiff config check

# For Git repository issues:
cd /path/to/repository
git status
git log --oneline

# If repository doesn't exist:
git init -b main
git config user.name "ndiff"
git config user.email "ndiff@localhost"

# For GitHub API backend:
export GITHUB_TOKEN=ghp_yourtoken
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/repos/owner/repo
```

### Scenario 6: Adding New Jobs to Track

You deployed a new job and want to track it.

**Method 1: Edit config file directly**
```bash
# Edit ndiff.toml
vim ndiff.toml

# Add:
# [[jobs]]
# name = "new-job"
# namespace = "production"
# region = "us-east"

# Verify
ndiff config check

# Sync
ndiff sync
```

**Method 2: Recreate config**
```bash
ndiff init --force
# Add new job when prompted
```

### Scenario 7: Monitoring Specific Jobs

You only want to sync certain jobs, not all configured jobs.

```bash
# Sync only web-app and api-server
ndiff sync --jobs web-app,api-server

# Preview changes for specific jobs
ndiff sync --jobs web-app --dry-run
```

## Environment Variables

ndiff respects these environment variables:

### Nomad Configuration

| Variable | Description | Example |
|----------|-------------|---------|
| `NOMAD_ADDR` | Nomad cluster address | `http://localhost:4646` |
| `NOMAD_TOKEN` | Nomad ACL token | `secret-token-here` |
| `NOMAD_CACERT` | Path to CA certificate | `/etc/nomad/ca.crt` |
| `NOMAD_TLS_SKIP_VERIFY` | Skip TLS verification | `true` |

### Git/GitHub Configuration

| Variable | Description | Example |
|----------|-------------|---------|
| `GITHUB_TOKEN` | GitHub personal access token | `ghp_xxxxxxxxxxxx` |
| `GH_TOKEN` | Alternative to GITHUB_TOKEN | `ghp_xxxxxxxxxxxx` |

### Application Configuration

| Variable | Description | Example |
|----------|-------------|---------|
| `NOMAD_CHANGELOG_*` | Override any config value | `NOMAD_CHANGELOG_GIT_BRANCH=develop` |

### Precedence Order

Configuration values are resolved in this order (highest to lowest):

1. Command-line flags
2. Environment variables
3. Configuration file
4. Default values

**Example:**
```bash
# Config file says: branch = "main"
# But you can override with:
export NOMAD_CHANGELOG_GIT_BRANCH=develop
ndiff sync
# Uses 'develop' branch
```

## Exit Codes

ndiff uses standard exit codes:

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error (general) |
| `2` | Configuration error |

**Usage in scripts:**
```bash
#!/bin/bash

# Check configuration before syncing
if ! ndiff config check; then
    echo "Configuration check failed!"
    exit 1
fi

# Sync jobs
if ndiff sync; then
    echo "Sync successful"
else
    echo "Sync failed"
    exit 1
fi
```

## Tips and Best Practices

### 1. Always Initialize Git Repository First

Before running ndiff:
```bash
git init -b main
git config user.name "ndiff"
git config user.email "ndiff@localhost"
```

### 2. Always Run Config Check First

After setup or configuration changes:
```bash
ndiff config check
```

### 3. Use Dry Run for Testing

Before actual sync:
```bash
ndiff sync --dry-run
```

### 4. Use Environment Variables for Secrets

Never commit tokens to config files:
```bash
# Good ✅
export GITHUB_TOKEN=ghp_xxx
export NOMAD_TOKEN=xxx

# Bad ❌
# token = "ghp_xxx" in config file
```

### 5. Verbose Mode for Debugging

When troubleshooting:
```bash
ndiff --verbose sync
```

### 6. Manual Push Workflow

With Git backend, review before pushing:
```bash
# Sync (commits locally)
ndiff sync

# Review
cd /path/to/repo
git log --oneline
git diff origin/main

# Push when ready
git push origin main
```

### 7. Organize by Region/Namespace

Use the hierarchical structure:
```
region/namespace/job.hcl
```

This makes it easy to:
- Find jobs by region
- Group jobs by namespace
- Deploy to correct location

### 8. CI/CD: Check Before Sync

```bash
ndiff config check && ndiff sync
```

### 9. Monitor Sync Failures

Set up alerts for sync failures in your CI/CD system.

## Getting Help

### Command Help

```bash
# General help
ndiff --help

# Command-specific help
ndiff sync --help
ndiff deploy --help
ndiff config --help
```

### Check Configuration

```bash
# Show current config
ndiff config show

# Validate config
ndiff config validate

# Test all connections
ndiff config check
```

### Verbose Output

```bash
ndiff --verbose sync
ndiff --verbose deploy abc123
```

### Documentation

- [Main README](../README.md) - Getting started
- [Backend Guide](BACKENDS.md) - Backend configuration
- [GitHub Issues](https://github.com/wlame/ndiff/issues) - Report bugs

## Quick Reference

```bash
# First-time setup
git init -b main
git config user.name "ndiff"
git config user.email "ndiff@localhost"
ndiff init
ndiff config check
ndiff sync --dry-run
ndiff sync

# Regular usage
ndiff sync                           # Sync all jobs
ndiff sync --jobs web-app            # Sync specific job
ndiff sync --dry-run                 # Preview changes

# Deploy/rollback
ndiff deploy abc123                  # Auto-detect job
ndiff deploy abc123 --job web-app    # Specify job
ndiff deploy abc123 --job web-app --namespace production --region us-east

# History
ndiff history                        # All jobs
ndiff history --job web-app --namespace production --region us-east
ndiff show abc123 --job web-app --namespace production --region us-east

# Configuration
ndiff config show                    # Display config
ndiff config validate                # Validate syntax
ndiff config check                   # Full connectivity check

# With environment variables
export NOMAD_ADDR=http://nomad:4646
export NOMAD_TOKEN=xxx
export GITHUB_TOKEN=ghp_xxx
ndiff sync

# Custom config file
ndiff --config /etc/ndiff.toml sync

# Manual push workflow (Git backend)
ndiff sync
cd /path/to/repo
git log --oneline
git push origin main
```
