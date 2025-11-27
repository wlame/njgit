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

#### 1. Initialize Configuration

Run the interactive setup wizard:

```bash
ndiff init
```

This will guide you through:
- Choosing a backend (Git or GitHub API)
- Configuring repository access
- Setting up Nomad connection
- Adding jobs to track

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

...
```

#### 2. Verify Configuration

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
      Git URL: git@github.com:myorg/nomad-jobs.git
      Branch: main
      Nomad: http://nomad.example.com:4646
      Jobs to track: 2

3️⃣  Testing Nomad connection...
   ✅ Successfully connected to Nomad

4️⃣  Checking configured jobs in Nomad...
   ✅ Job found: default/web-app
   ✅ Job found: default/api-server
   ✅ 2 job(s) found in Nomad

5️⃣  Testing backend connection...
   ✅ Successfully connected to backend (git)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 Check Summary
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

   ✅ Passed: 5

✅ All checks passed! You're ready to use ndiff.
```

#### 3. Start Syncing

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
2. Collects repository credentials
3. Configures Nomad connection
4. Lets you add jobs to track
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
    "url": "git@github.com:myorg/nomad-jobs.git",
    "branch": "main",
    "token": "********"
  },
  "Nomad": {
    "address": "http://localhost:4646",
    "token": "********"
  },
  "Jobs": [
    {
      "name": "web-app",
      "namespace": "default"
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
ℹ  Git repository: git@github.com:myorg/nomad-jobs.git
ℹ  Nomad address: http://localhost:4646
ℹ  Tracking 2 jobs
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
- `--no-push` - Commit locally but don't push to remote
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

# Commit locally but don't push
ndiff sync --no-push

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
   - Writes HCL file
   - Creates commit
   - Pushes to remote (unless `--no-push`)

**Output:**
```bash
$ ndiff sync

ℹ  Starting sync...
ℹ  Loading configuration...
ℹ  Nomad: http://nomad.example.com:4646
ℹ  Backend: git (git@github.com:myorg/nomad-jobs.git)
ℹ  Connecting to Nomad...
✅ Connected to Nomad
ℹ  Setting up backend...
✅ Backend ready (git)
ℹ  Syncing 2 jobs...
ℹ  Checking default/web-app...
ℹ    default/web-app: CHANGED
ℹ  Checking default/api-server...
✅ Synced 1 jobs with changes:
  - default/web-app
```

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
# Step 1: Create configuration
ndiff init
# Choose: Git backend
# Enter: git@github.com:myorg/nomad-jobs.git
# Auth: ssh
# Add jobs when prompted

# Step 2: Verify everything works
ndiff config check

# Step 3: Test with dry run
ndiff sync --dry-run

# Step 4: Start syncing
ndiff sync
```

### Scenario 2: CI/CD Setup (GitHub Actions)

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

### Scenario 3: Kubernetes CronJob

Running ndiff as a Kubernetes CronJob.

**ConfigMap** (config.yaml):
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ndiff-config
data:
  ndiff.toml: |
    [git]
    backend = "github-api"
    owner = "myorg"
    repo = "nomad-jobs"
    branch = "main"
    
    [nomad]
    address = "http://nomad.default.svc.cluster.local:4646"
    
    [[jobs]]
    name = "web-app"
    namespace = "default"
```

**CronJob**:
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: ndiff-sync
spec:
  schedule: "0 * * * *"  # Every hour
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: ndiff
            image: myorg/ndiff:latest
            command:
            - /bin/sh
            - -c
            - |
              # Verify config first
              ndiff config check || exit 1
              # Then sync
              ndiff sync
            env:
            - name: GITHUB_TOKEN
              valueFrom:
                secretKeyRef:
                  name: github-token
                  key: token
            volumeMounts:
            - name: config
              mountPath: /config
              readOnly: true
          volumes:
          - name: config
            configMap:
              name: ndiff-config
          restartPolicy: OnFailure
```

### Scenario 4: Troubleshooting Connection Issues

You're having problems connecting to Nomad or your repository.

```bash
# Step 1: Verify configuration syntax
ndiff config validate

# Step 2: Run comprehensive check
ndiff config check

# Step 3: If checks fail, check specific issues:

# For Nomad connection issues:
export NOMAD_ADDR=http://your-nomad:4646
export NOMAD_TOKEN=your-token
ndiff config check

# For Git authentication issues (SSH):
ssh-add -l  # Check loaded keys
ssh -T git@github.com  # Test GitHub SSH

# For Git authentication issues (HTTPS):
export GITHUB_TOKEN=ghp_yourtoken
ndiff config check

# For GitHub API backend:
export GITHUB_TOKEN=ghp_yourtoken
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/repos/owner/repo
```

### Scenario 5: Migrating Between Backends

Switching from Git backend to GitHub API backend.

```bash
# Step 1: Backup current config
cp ndiff.toml ndiff.toml.bak

# Step 2: Create new config with GitHub API backend
ndiff init --force
# Choose: github-api
# Enter GitHub owner and repo

# Step 3: Verify new setup
ndiff config check

# Step 4: Test sync
ndiff sync --dry-run

# If everything works:
ndiff sync
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
# namespace = "default"

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

### 1. Always Run Config Check First

After setup or configuration changes:
```bash
ndiff config check
```

### 2. Use Dry Run for Testing

Before actual sync:
```bash
ndiff sync --dry-run
```

### 3. Use Environment Variables for Secrets

Never commit tokens to config files:
```bash
# Good ✅
export GITHUB_TOKEN=ghp_xxx
export NOMAD_TOKEN=xxx

# Bad ❌
# token = "ghp_xxx" in config file
```

### 4. Verbose Mode for Debugging

When troubleshooting:
```bash
ndiff --verbose sync
```

### 5. Version Control Your Config

Add to `.gitignore`:
```
ndiff-repo/
*.log
```

Commit your config (without secrets):
```
git add ndiff.toml
git commit -m "Add ndiff config"
```

### 6. CI/CD: Check Before Sync

```bash
ndiff config check && ndiff sync
```

### 7. Monitor Sync Failures

Set up alerts for sync failures in your CI/CD system.

## Getting Help

### Command Help

```bash
# General help
ndiff --help

# Command-specific help
ndiff sync --help
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
```

### Documentation

- [Main README](../README.md) - Getting started
- [Backend Guide](BACKENDS.md) - Backend configuration
- [GitHub Issues](https://github.com/wlame/ndiff/issues) - Report bugs

## Quick Reference

```bash
# First-time setup
ndiff init
ndiff config check
ndiff sync --dry-run
ndiff sync

# Regular usage
ndiff sync                    # Sync all jobs
ndiff sync --jobs web-app     # Sync specific job
ndiff sync --dry-run          # Preview changes

# Configuration
ndiff config show             # Display config
ndiff config validate         # Validate syntax
ndiff config check            # Full connectivity check

# With environment variables
export NOMAD_ADDR=http://nomad:4646
export NOMAD_TOKEN=xxx
export GITHUB_TOKEN=ghp_xxx
ndiff sync

# Custom config file
ndiff --config /etc/ndiff.toml sync
```
