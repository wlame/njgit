# Backend Configuration Guide

ndiff supports two different storage backends for tracking Nomad job configurations. This guide explains both backends and helps you choose the right one for your use case.

## Table of Contents

- [Overview](#overview)
- [Backend Comparison](#backend-comparison)
- [Git Backend (Default)](#git-backend-default)
- [GitHub API Backend](#github-api-backend)
- [Switching Backends](#switching-backends)

## Overview

A **backend** is how ndiff stores and tracks changes to your Nomad job configurations. Think of it as the "database" where your job history is saved.

### Available Backends

1. **Git Backend** (default) - Uses a local Git repository that you manage
2. **GitHub API Backend** - Uses GitHub REST API directly (no local repository)

## Backend Comparison

| Feature | Git (Local) | GitHub API |
|---------|-------------|------------|
| **Local Repository** | Yes (user-managed) | No (stateless) |
| **Git Providers** | Any (local only) | GitHub only |
| **Automatic Push** | No (manual) | Yes |
| **Automatic Pull** | No (manual) | N/A |
| **Authentication** | Not needed | GitHub token required |
| **Repository Reuse** | Yes | N/A |
| **Offline Usage** | Yes (fully offline) | No |
| **User Control** | Full manual control | Automatic |
| **Multi-file Commits** | Yes | No* |
| **Best For** | Local development, full control | CI/CD ephemeral environments |

*The GitHub API doesn't support multi-file commits, so each changed job creates a separate commit.

## Git Backend (Default)

The **Git backend** is the default option. It uses a local Git repository that you initialize and manage yourself. The tool commits changes locally - you decide when and how to push to remote repositories.

### Key Features

- **User-Controlled Repository**: You initialize and manage the Git repository yourself
- **No Automatic Operations**: Tool never clones, pushes, or pulls
- **Local Commits Only**: Changes are committed locally - you decide when to push
- **Optional Remote**: You can configure remotes and push/pull manually whenever you want
- **Fully Offline**: Works completely offline without any remote connectivity
- **Multi-file Commits**: All changed jobs in a single sync are committed together

### When to Use Git Backend

- **Local Development**: When you want full control over Git operations
- **Manual Review**: Review commits before pushing to remote
- **Any Git Provider**: Works with GitHub, GitLab, Bitbucket, self-hosted, etc.
- **Offline Environments**: No network connectivity required
- **Standard Git Workflows**: Use any Git commands you want

### Configuration

#### Minimal Configuration

```toml
[git]
backend = "git"  # Optional - "git" is the default
local_path = "."  # Directory containing the repository (default: current directory)
repo_name = "ndiff-repo"  # Repository directory name (default: "ndiff-repo")
branch = "main"
author_name = "ndiff"
author_email = "ndiff@localhost"
```

#### Full Configuration Options

```toml
[git]
backend = "git"
local_path = "/home/user/repositories"  # Directory containing the repository
repo_name = "nomad-jobs"  # Repository directory name
branch = "main"
author_name = "ndiff"
author_email = "bot@example.com"
```

**All fields explained**:
- `backend` - Always "git" for this backend (default if omitted)
- `local_path` - Directory containing the repository (default: "." = current directory)
- `repo_name` - Repository directory name (default: "ndiff-repo")
- `branch` - Branch to commit to (default: "main")
- `author_name` - Name used in Git commits
- `author_email` - Email used in Git commits

**Not used** (these are ignored by the Git backend):
- `url` - Not needed (repository is local)
- `auth_method` - Not needed (no remote operations)
- `ssh_key_path` - Not needed
- `token` - Not needed

### Setup Instructions

#### 1. Initialize the Repository

You must initialize the Git repository yourself before using ndiff:

```bash
# Option A: Initialize in current directory
git init -b main
git config user.name "ndiff"
git config user.email "ndiff@localhost"

# Then configure ndiff to use current directory
# (default config uses "." as local_path)
```

```bash
# Option B: Initialize in a specific directory
mkdir -p /home/user/repositories
cd /home/user/repositories
git init -b main nomad-jobs
cd nomad-jobs
git config user.name "ndiff"
git config user.email "ndiff@localhost"

# Then configure ndiff with:
# local_path = "/home/user/repositories"
# repo_name = "nomad-jobs"
```

#### 2. Optional: Add a Remote

If you want to push changes to a remote repository manually later:

```bash
# For GitHub (SSH)
git remote add origin git@github.com:myorg/nomad-jobs.git

# For GitHub (HTTPS)
git remote add origin https://github.com/myorg/nomad-jobs.git

# For GitLab
git remote add origin git@gitlab.com:myorg/nomad-jobs.git

# For any other Git server
git remote add origin git@git.example.com:path/to/repo.git
```

#### 3. Configure ndiff

Create or edit `~/.config/ndiff/config.toml`:

```toml
[nomad]
address = "http://localhost:4646"

[git]
backend = "git"
local_path = "/home/user/repositories"  # Or "." for current directory
repo_name = "nomad-jobs"
branch = "main"
author_name = "ndiff"
author_email = "ndiff@localhost"

[[jobs]]
name = "my-job"
namespace = "default"
region = "global"
```

#### 4. Run Sync

```bash
ndiff sync
```

The tool will commit changes locally. You can push manually whenever you want:

```bash
cd /home/user/repositories/nomad-jobs
git push origin main
```

### Workflow Examples

#### Example 1: Local Development with Manual Push

Perfect for developers who want to review commits before pushing:

```bash
# Sync changes (commits locally)
ndiff sync

# Review commits
cd /home/user/repositories/nomad-jobs
git log --oneline

# Review changes
git diff origin/main

# Push when ready
git push origin main
```

#### Example 2: Fully Offline Usage

For environments without internet access:

```bash
# Initialize local repository (one-time setup)
cd /opt/nomad-tracking
git init -b main
git config user.name "ndiff"
git config user.email "ndiff@localhost"

# Sync creates local commits (no network needed)
ndiff sync

# All history is stored locally
git log --oneline
ndiff history
```

#### Example 3: Using with Different Remotes

Push to different remotes as needed:

```bash
# Add multiple remotes
git remote add github git@github.com:myorg/nomad-jobs.git
git remote add gitlab git@gitlab.com:myorg/nomad-jobs.git
git remote add backup git@backup.example.com:nomad-jobs.git

# Push to specific remote
git push github main
git push gitlab main

# Or push to all
git remote | xargs -I {} git push {} main
```

### File Organization

Jobs are stored in a hierarchical structure:

```
<region>/<namespace>/<job-name>.hcl
```

**Example:**
```
global/
  default/
    web-app.hcl
    api-server.hcl
  production/
    worker.hcl
us-west/
  default/
    cache.hcl
```

This structure:
- Mirrors Nomad's region/namespace organization
- Makes it easy to find specific jobs
- Supports multi-region deployments
- Allows namespace-specific configurations

### Advantages

- ✅ **Full Control**: You decide exactly when to push/pull
- ✅ **Offline Operation**: Works without any network connectivity
- ✅ **No Authentication Needed**: No SSH keys or tokens required for local commits
- ✅ **Standard Git Workflows**: Use any Git commands you want
- ✅ **Manual Review**: Review all commits before pushing
- ✅ **Any Git Provider**: Works with any Git server when you push manually
- ✅ **Multi-file Commits**: All changes in one sync → one commit

### Limitations

- ❌ **Manual Setup Required**: Must initialize repository yourself
- ❌ **No Automatic Push**: Changes stay local until you push manually
- ❌ **No Automatic Pull**: Won't pull remote changes automatically
- ❌ **Repository Must Exist**: Tool errors if repository doesn't exist

### Best For

- Local development where you want manual control over push/pull
- Offline environments
- Testing and experimentation without remote impact
- Complex Git workflows requiring manual intervention
- Any Git provider (GitHub, GitLab, Bitbucket, self-hosted, etc.)

### Error Messages

**"repository does not exist at /path/to/repo"**:
- **Cause**: Repository doesn't exist
- **Solution**: Run `git init -b main /path/to/repo` to create it

**"not a git repository"**:
- **Cause**: Directory exists but isn't a Git repository
- **Solution**: Run `git init -b main` in that directory

## GitHub API Backend

The **GitHub API backend** uses the GitHub REST API directly, without maintaining a local Git repository. It's completely stateless.

### Key Features

- **Stateless**: No local repository - all operations via API
- **CI/CD Optimized**: Perfect for ephemeral environments (Docker, Kubernetes, CI runners)
- **Minimal Disk Usage**: Only temporary files, no Git repository
- **Automatic Push**: Commits are immediately pushed to GitHub
- **GitHub Only**: Requires GitHub (won't work with GitLab, Bitbucket, etc.)

### Configuration

#### Minimal Configuration

```toml
[git]
backend = "github-api"
owner = "myorg"
repo = "nomad-jobs"
branch = "main"
```

Set your GitHub token via environment variable:
```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
# Or:
export GH_TOKEN="ghp_xxxxxxxxxxxx"
```

#### Full Configuration Options

```toml
[git]
backend = "github-api"
owner = "myorg"  # GitHub organization or username
repo = "nomad-jobs"  # Repository name
branch = "main"
author_name = "ndiff"  # Optional - used in commits
author_email = "ndiff@localhost"  # Optional - used in commits

# Token via environment variable (recommended):
# export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
```

### Authentication

The GitHub API backend **requires** a GitHub Personal Access Token (PAT).

#### Creating a GitHub Token

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Click "Generate new token (classic)"
3. Give it a descriptive name (e.g., "ndiff")
4. Select scopes:
   - `repo` (Full control of private repositories)
   - Or `public_repo` (for public repositories only)
5. Click "Generate token"
6. Copy the token (you won't be able to see it again!)

#### Setting the Token

**Option 1: Environment Variable (Recommended)**

```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
ndiff sync
```

**Option 2: Config File (Not Recommended)**

```toml
[git]
token = "ghp_xxxxxxxxxxxx"  # ⚠️ NOT RECOMMENDED - use env var instead
```

**Security Best Practice**: Always use environment variables for tokens, never commit them to config files!

### Limitations

The GitHub API backend has some limitations compared to the Git backend:

1. **One Commit per File**: The GitHub API doesn't support multi-file commits. Each changed job creates a separate commit.
2. **GitHub Only**: Only works with GitHub, not other Git providers
3. **Requires Network**: Cannot work offline (no local repository)
4. **API Rate Limits**: Subject to GitHub API rate limits (usually not a problem for typical usage)
5. **No Local History**: No local Git repository, all operations are remote

### Use Cases

The GitHub API backend is ideal for:
- Kubernetes CronJobs or batch jobs
- Docker containers (ephemeral environments)
- CI/CD pipelines (GitHub Actions, Jenkins, etc.)
- AWS Lambda or other serverless functions
- Environments where disk space is limited
- When you don't need offline capability or local history

### Example: Kubernetes CronJob

**Configuration:**
```toml
[git]
backend = "github-api"
owner = "myorg"
repo = "nomad-jobs"
branch = "main"
```

**Kubernetes Manifest:**
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
            env:
            - name: GITHUB_TOKEN
              valueFrom:
                secretKeyRef:
                  name: github-token
                  key: token
            - name: NOMAD_ADDR
              value: "http://nomad.default.svc.cluster.local:4646"
            command: ["ndiff", "sync"]
          restartPolicy: OnFailure
```

## Switching Backends

You can easily switch between backends by changing the `backend` field in your configuration.

### From Git to GitHub API

**Before (Git backend):**
```toml
[git]
backend = "git"
local_path = "/home/user/repositories"
repo_name = "nomad-jobs"
branch = "main"
```

**After (GitHub API backend):**
```toml
[git]
backend = "github-api"
owner = "myorg"
repo = "nomad-jobs"
branch = "main"
```

Don't forget to set `GITHUB_TOKEN` environment variable!

### From GitHub API to Git

**Before (GitHub API backend):**
```toml
[git]
backend = "github-api"
owner = "myorg"
repo = "nomad-jobs"
branch = "main"
```

**After (Git backend):**
```toml
[git]
backend = "git"
local_path = "/home/user/repositories"
repo_name = "nomad-jobs"
branch = "main"
```

Don't forget to initialize the repository with `git init`!

## Troubleshooting

### Git Backend Issues

**Problem**: `repository does not exist at /path`

**Solution**: Initialize the Git repository:
```bash
cd /path
git init -b main
git config user.name "ndiff"
git config user.email "ndiff@localhost"
```

**Problem**: `not a git repository`

**Solution**: The directory exists but isn't a Git repository. Initialize it:
```bash
cd /path/to/directory
git init -b main
```

**Problem**: `failed to commit: Author identity unknown`

**Solution**: Configure Git user in the repository:
```bash
cd /path/to/repo
git config user.name "ndiff"
git config user.email "ndiff@localhost"
```

### GitHub API Backend Issues

**Problem**: `github token is required`

**Solution**: Set the GitHub token via environment variable:
```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
```

**Problem**: `github authentication failed: invalid token`

**Solution**: 
- Verify your token is correct
- Ensure the token has `repo` scope
- Check if the token has expired

**Problem**: `github repository not found`

**Solution**:
- Verify `owner` and `repo` are correct
- Ensure your token has access to the repository
- For private repos, token needs `repo` scope

## Best Practices

1. **Use Environment Variables for Secrets**: Never commit tokens to config files
2. **Choose the Right Backend**: 
   - Git backend for local development and full control
   - GitHub API backend for ephemeral CI/CD environments
3. **Manual Push Workflow**: With Git backend, review commits before pushing to remote
4. **CI/CD Credentials**: Use CI/CD secret management (GitHub Secrets, Jenkins Credentials, Kubernetes Secrets)
5. **Repository Initialization**: Always initialize the Git repository before first use
6. **Regular Backups**: Even with local-only mode, consider pushing to remote for backups

## Further Reading

- [Main README](../README.md) - Getting started guide
- [CLI Guide](CLI_GUIDE.md) - Detailed command reference
- [Configuration Reference](../README.md#configuration) - Full configuration options
