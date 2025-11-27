# Backend Configuration Guide

nomad-changelog supports two different storage backends for tracking Nomad job configurations. This guide explains both backends and helps you choose the right one for your use case.

## Table of Contents

- [Overview](#overview)
- [Backend Comparison](#backend-comparison)
- [Git Backend](#git-backend)
- [GitHub API Backend](#github-api-backend)
- [Switching Backends](#switching-backends)

## Overview

A **backend** is how nomad-changelog stores and tracks changes to your Nomad job configurations. Think of it as the "database" where your job history is saved.

### Available Backends

1. **Git Backend** (default) - Uses a local Git repository
2. **GitHub API Backend** - Uses GitHub REST API directly (no local repository)

## Backend Comparison

| Feature | Git Backend | GitHub API Backend |
|---------|-------------|-------------------|
| **Local Repository** | Yes (cloned to disk) | No (stateless) |
| **Git Providers** | Any (GitHub, GitLab, Bitbucket, etc.) | GitHub only |
| **Authentication** | SSH keys, tokens, or auto | GitHub token required |
| **Disk Space** | Uses disk for repo | Minimal (no repo) |
| **Repository Reuse** | Yes (persists between runs) | N/A (no local repo) |
| **Offline Usage** | Possible (commit locally) | No (requires API access) |
| **Commits per File** | Multiple files per commit | One file per commit* |
| **Performance** | Fast (local operations) | Slower (API calls) |
| **CI/CD Friendly** | Both work well | Better for ephemeral environments |

*The GitHub API doesn't support multi-file commits, so each changed job creates a separate commit.

## Git Backend

The **Git backend** is the default and most flexible option. It clones a Git repository to your local machine and uses standard Git operations (clone, commit, push).

### Key Features

- **Repository Persistence**: The Git backend checks if the repository already exists locally before cloning. If it exists, it opens the existing repository and pulls the latest changes instead of re-cloning. This makes it efficient for repeated runs.
- **Universal Git Support**: Works with any Git provider (GitHub, GitLab, Bitbucket, self-hosted, etc.)
- **Flexible Authentication**: Supports SSH keys, personal access tokens, or automatic detection
- **Offline Commits**: Can commit locally even when offline (push later when online)

### Configuration

#### Minimal Configuration (SSH)

```toml
[git]
backend = "git"  # Optional - "git" is the default
url = "git@github.com:myorg/nomad-jobs.git"
branch = "main"
```

#### HTTPS with Token

```toml
[git]
backend = "git"
url = "https://github.com/myorg/nomad-jobs.git"
branch = "main"
auth_method = "token"

# Set via environment variable (recommended):
# export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
```

#### Full Configuration Options

```toml
[git]
backend = "git"
url = "git@github.com:myorg/nomad-jobs.git"
branch = "main"
auth_method = "ssh"  # Options: "ssh", "token", "auto" (default: "auto")
ssh_key_path = "/home/user/.ssh/id_ed25519"  # Optional - auto-detected if not specified
local_path = "."  # Directory where repo is stored (default: current directory)
repo_name = "nomad-changelog-repo"  # Repository directory name (default: "nomad-changelog-repo")
author_name = "nomad-changelog"
author_email = "nomad-changelog@localhost"
```

### Authentication Methods

#### SSH (Recommended for Local Use)

SSH authentication uses your SSH keys. This is the most convenient for local development.

```toml
[git]
url = "git@github.com:myorg/nomad-jobs.git"
auth_method = "ssh"
```

The Git backend will automatically look for SSH keys in standard locations:
- `~/.ssh/id_ed25519`
- `~/.ssh/id_rsa`
- Or specify a custom path with `ssh_key_path`

#### Token (Recommended for CI/CD)

Token authentication uses a personal access token (PAT) or other Git token.

```toml
[git]
url = "https://github.com/myorg/nomad-jobs.git"
auth_method = "token"
```

Set the token via environment variable:
```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
# Or for other Git providers:
export GH_TOKEN="your-token-here"
```

**Never commit tokens to your config file!** Always use environment variables.

#### Auto (Default)

Auto-detection tries SSH first, then falls back to token if available.

```toml
[git]
url = "git@github.com:myorg/nomad-jobs.git"
auth_method = "auto"  # Or omit - this is the default
```

### Repository Persistence

The Git backend is designed for efficiency with local repository reuse:

1. **First Run**: Clones the repository to `<local_path>/<repo_name>`
2. **Subsequent Runs**: 
   - Checks if `.git` directory exists
   - If yes: Opens existing repository and pulls latest changes
   - If no: Clones a fresh copy
3. **Cleanup**: The repository is **never** automatically deleted

This means:
- ✅ Fast subsequent runs (no re-cloning)
- ✅ Persistent local history
- ✅ Manual cleanup if needed (just delete the directory)

### Use Cases

The Git backend is ideal for:
- Local development and testing
- Self-hosted Git servers
- GitLab, Bitbucket, or other non-GitHub providers
- Environments with persistent storage
- When you need offline commit capability

## GitHub API Backend

The **GitHub API backend** uses the GitHub REST API directly, without maintaining a local Git repository. It's completely stateless.

### Key Features

- **Stateless**: No local repository - all operations via API
- **CI/CD Optimized**: Perfect for ephemeral environments (Docker, Kubernetes, CI runners)
- **Minimal Disk Usage**: Only temporary files, no Git repository
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
author_name = "nomad-changelog"  # Optional - used in commits
author_email = "nomad-changelog@localhost"  # Optional - used in commits

# Token via environment variable (recommended):
# export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
```

### Authentication

The GitHub API backend **requires** a GitHub Personal Access Token (PAT).

#### Creating a GitHub Token

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Click "Generate new token (classic)"
3. Give it a descriptive name (e.g., "nomad-changelog")
4. Select scopes:
   - `repo` (Full control of private repositories)
   - Or `public_repo` (for public repositories only)
5. Click "Generate token"
6. Copy the token (you won't be able to see it again!)

#### Setting the Token

**Option 1: Environment Variable (Recommended)**

```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
nomad-changelog sync
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

### Use Cases

The GitHub API backend is ideal for:
- Kubernetes CronJobs or batch jobs
- Docker containers (ephemeral environments)
- CI/CD pipelines (GitHub Actions, Jenkins, etc.)
- AWS Lambda or other serverless functions
- Environments where disk space is limited
- When you don't need offline capability

## Switching Backends

You can easily switch between backends by changing the `backend` field in your configuration.

### From Git to GitHub API

**Before (Git backend):**
```toml
[git]
backend = "git"
url = "git@github.com:myorg/nomad-jobs.git"
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
url = "git@github.com:myorg/nomad-jobs.git"  # Or HTTPS URL
branch = "main"
```

## Examples

### Example 1: Local Development with SSH

**Scenario**: Developer working locally with SSH keys

```toml
[git]
backend = "git"
url = "git@github.com:myorg/nomad-jobs.git"
branch = "main"
local_path = "/home/user/nomad-sync"
repo_name = "nomad-jobs-repo"
```

**Usage**:
```bash
nomad-changelog sync
# Repository cloned to: /home/user/nomad-sync/nomad-jobs-repo
# Subsequent runs reuse the same repository
```

### Example 2: CI/CD with Git Backend

**Scenario**: Jenkins pipeline using Git with token auth

```toml
[git]
backend = "git"
url = "https://github.com/myorg/nomad-jobs.git"
branch = "main"
auth_method = "token"
local_path = "/tmp/nomad-changelog"
```

**Jenkins Pipeline**:
```groovy
pipeline {
    environment {
        GITHUB_TOKEN = credentials('github-token')
    }
    stages {
        stage('Sync') {
            steps {
                sh 'nomad-changelog sync'
            }
        }
    }
}
```

### Example 3: Kubernetes CronJob with GitHub API

**Scenario**: Kubernetes CronJob running every hour (ephemeral pods)

```toml
[git]
backend = "github-api"
owner = "myorg"
repo = "nomad-jobs"
branch = "main"
```

**Kubernetes Manifest**:
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: nomad-changelog-sync
spec:
  schedule: "0 * * * *"  # Every hour
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: nomad-changelog
            image: myorg/nomad-changelog:latest
            env:
            - name: GITHUB_TOKEN
              valueFrom:
                secretKeyRef:
                  name: github-token
                  key: token
            - name: NOMAD_ADDR
              value: "http://nomad.default.svc.cluster.local:4646"
            command: ["nomad-changelog", "sync"]
          restartPolicy: OnFailure
```

### Example 4: GitLab with Git Backend

**Scenario**: Using GitLab (GitHub API backend won't work)

```toml
[git]
backend = "git"  # GitHub API backend doesn't support GitLab
url = "https://gitlab.com/myorg/nomad-jobs.git"
branch = "main"
auth_method = "token"
```

**Set GitLab token**:
```bash
export GITHUB_TOKEN="glpat-xxxxxxxxxxxx"  # GitLab token works with Git backend
nomad-changelog sync
```

## Troubleshooting

### Git Backend Issues

**Problem**: `failed to clone repository: authentication required`

**Solution**: 
- For SSH: Ensure your SSH key is added to your Git provider
- For HTTPS: Set the `GITHUB_TOKEN` environment variable
- Try `auth_method = "auto"` to let it auto-detect

**Problem**: `not a git repository`

**Solution**: The Git backend expected an existing repo but didn't find one. Delete the directory and let it clone fresh:
```bash
rm -rf /path/to/repo-name
nomad-changelog sync
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

1. **Use Environment Variables for Secrets**: Never commit tokens or SSH key passwords to config files
2. **Choose the Right Backend**: 
   - Git backend for local development and non-GitHub providers
   - GitHub API backend for ephemeral CI/CD environments
3. **Repository Persistence**: If using Git backend, be aware it persists the repository locally - plan for disk usage
4. **CI/CD Credentials**: Use CI/CD secret management (GitHub Secrets, Jenkins Credentials, Kubernetes Secrets)
5. **Test Configuration**: Use `nomad-changelog config validate` to test your configuration

## Further Reading

- [Main README](../README.md) - Getting started guide
- [Configuration Guide](../README.md#configuration) - Full configuration reference
- [GitHub API Documentation](https://docs.github.com/en/rest) - GitHub REST API details
