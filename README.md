# ndiff

A lightweight CLI tool that tracks HashiCorp Nomad job configuration changes using Git, providing version history and rollback capabilities.

## What is ndiff?

ndiff automatically syncs your Nomad job specifications to Git, creating a version-controlled history of all changes. It detects configuration changes, commits them with detailed messages, and allows you to view history and rollback to previous versions.

**Key Features:**
- 🔄 **Automatic change detection** - Only commits when jobs actually change
- 📜 **Version history** - Track all configuration changes over time
- ⏮️  **Easy rollback** - Deploy any previous version with one command
- 🌍 **Multi-region support** - Organize jobs by region/namespace/name
- 🔧 **Flexible backends** - Local Git or GitHub API
- 🚀 **Simple setup** - Interactive wizard gets you started in minutes

## Quick Start

### 1. Install

**Download the binary** for your platform from [releases](https://github.com/wlame/ndiff/releases), or build from source:

```bash
git clone https://github.com/wlame/ndiff
cd ndiff
go build -o ndiff ./cmd/ndiff
```

### 2. Initialize Configuration

```bash
ndiff init
```

This interactive wizard will ask you:
- **Backend**: Choose `git` (local Git repo) or `github-api` (stateless)
- **Repository path**: Where to store job files (for git backend)
- **Nomad address**: Your Nomad cluster URL
- **Jobs to track**: Which jobs to monitor

### 3. Set Up Git Repository

For the git backend, initialize your repository:

```bash
cd .  # Your configured local_path
git init
git config user.name "Your Name"
git config user.email "you@example.com"

# Optional: Add remote for backup/sharing
git remote add origin git@github.com:yourorg/nomad-jobs.git
```

### 4. Start Tracking

```bash
# Sync your jobs to Git
ndiff sync

# View history
ndiff history

# Deploy a previous version
ndiff deploy <commit-hash> <job-name>
```

## How It Works

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

### Workflow

1. **Sync**: `ndiff sync` fetches job specs from Nomad, converts them to HCL, and commits changes
2. **History**: `ndiff history` shows all commits with job changes
3. **Show**: `ndiff show <commit>` displays a specific version
4. **Deploy**: `ndiff deploy <commit> <job>` rolls back to a previous version

## Configuration

### Configuration File

Create `ndiff.toml` (or run `ndiff init`):

```toml
# Git backend (local-only)
[git]
backend = "git"
local_path = "."  # Path to your Git repository

# Nomad cluster
[nomad]
address = "http://localhost:4646"
# token = ""  # Or set NOMAD_TOKEN env var

# Jobs to track
[[jobs]]
name = "web-app"
namespace = "default"
region = "global"

[[jobs]]
name = "api-server"
namespace = "production"  
region = "us-west"
```

### Backend Options

#### Git Backend (Recommended)

**Local-only mode** - You manage the repository:

```toml
[git]
backend = "git"
local_path = "."
```

**Features:**
- Local commits only (no automatic push/pull)
- You control remotes and synchronization
- Full Git flexibility
- Works offline

**Setup:**
```bash
git init
git remote add origin <your-repo-url>  # Optional
# Use git push/pull manually when you want
```

#### GitHub API Backend

**Stateless mode** - No local repository:

```toml
[git]
backend = "github-api"
owner = "myorg"
repo = "nomad-jobs"
branch = "main"
author_name = "ndiff"
author_email = "bot@example.com"
```

**Features:**
- No local storage required
- Automatic push to GitHub
- Good for CI/CD environments
- Requires `GITHUB_TOKEN` environment variable

## Commands

### `ndiff init`

Interactive setup wizard. Creates `ndiff.toml` configuration file.

```bash
ndiff init
ndiff init --force  # Overwrite existing config
```

### `ndiff sync`

Fetches jobs from Nomad, detects changes, and commits to Git.

```bash
ndiff sync                    # Sync all configured jobs
ndiff sync --dry-run         # Preview changes without committing
ndiff sync --jobs web-app    # Sync specific jobs only
```

**What it does:**
1. Fetches job spec from Nomad
2. Converts to HCL format
3. Compares with last committed version
4. Commits if changed (with detailed message)

### `ndiff history`

Shows commit history for jobs.

```bash
ndiff history                              # Show all commits
ndiff history --job web-app                # Filter by job
ndiff history --namespace production       # Filter by namespace
ndiff history --region us-west             # Filter by region
ndiff history --limit 10                   # Show last 10 commits
```

### `ndiff show`

Displays job configuration from a specific commit.

```bash
ndiff show abc123                               # Interactive file selection
ndiff show abc123 --job web-app                 # Show specific job
ndiff show abc123 --job web-app --region global --namespace default
```

### `ndiff deploy`

Deploys a job from a specific commit (rollback feature).

```bash
ndiff deploy abc123                    # Auto-detect job from commit
ndiff deploy abc123 web-app           # Deploy specific job
ndiff deploy abc123 web-app --region us-west --namespace production
ndiff deploy abc123 web-app --dry-run  # Preview without deploying
```

**Auto-detection:** If the commit only changed one job, ndiff automatically detects which job to deploy.

### `ndiff config`

Manage configuration.

```bash
ndiff config check     # Validate config and test connections
ndiff config show      # Display current configuration
```

## Examples

### Basic Workflow

```bash
# 1. Set up
ndiff init
git init

# 2. Track your jobs
ndiff sync

# 3. Make changes in Nomad UI or nomad CLI
nomad job run web-app.nomad

# 4. Sync changes to Git  
ndiff sync
# Creates commit: "Update global/default/web-app"

# 5. View history
ndiff history --job web-app
# Shows all changes to web-app

# 6. Rollback to previous version
ndiff deploy abc123 web-app
# Deploys the version from commit abc123
```

### CI/CD Integration

```yaml
# .github/workflows/nomad-backup.yml
name: Backup Nomad Jobs
on:
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours
jobs:
  backup:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Sync Nomad jobs
        env:
          NOMAD_ADDR: ${{ secrets.NOMAD_ADDR }}
          NOMAD_TOKEN: ${{ secrets.NOMAD_TOKEN }}
        run: |
          curl -L https://github.com/wlame/ndiff/releases/latest/download/ndiff-linux-amd64 -o ndiff
          chmod +x ndiff
          ./ndiff sync
          git push
```

### Multi-Region Setup

```toml
[[jobs]]
name = "web-app"
namespace = "default"
region = "us-east"

[[jobs]]
name = "web-app"
namespace = "default"
region = "us-west"

[[jobs]]
name = "web-app"
namespace = "default"
region = "eu-central"
```

Files created:
```
us-east/default/web-app.hcl
us-west/default/web-app.hcl
eu-central/default/web-app.hcl
```

## Authentication

### Nomad

Set one of these (in priority order):
1. `NOMAD_TOKEN` environment variable
2. `nomad.token` in config file
3. `~/.nomad-token` file

### GitHub API Backend

Required for `github-api` backend:

```bash
export GITHUB_TOKEN="ghp_..."
```

Token needs `repo` scope for private repositories.

## Advanced Usage

### Change Detection

ndiff intelligently ignores Nomad-internal fields that change on every deployment:

- `ModifyIndex`
- `ModifyTime`
- `JobModifyIndex`
- `SubmitTime`
- `CreateIndex`
- `Status`
- `StatusDescription`

Only real configuration changes trigger commits.

### File Structure

```
your-repo/
├── global/
│   └── default/
│       ├── web-app.hcl
│       └── api-server.hcl
└── us-west/
    └── production/
        └── worker.hcl
```

Each `.hcl` file contains the complete Nomad job specification in HCL format.

## Troubleshooting

### "git repository not found"

**Problem:** Git backend can't find your repository.

**Solution:**
```bash
cd <local_path>  # Path from your config
git init
git config user.name "Your Name"
git config user.email "you@example.com"
```

### "failed to connect to Nomad"

**Problem:** Can't reach Nomad cluster.

**Solutions:**
- Check `nomad.address` in config
- Verify `NOMAD_ADDR` environment variable
- Test: `curl $NOMAD_ADDR/v1/agent/self`
- Check ACL token if Nomad has ACLs enabled

### "no jobs configured"

**Problem:** No jobs listed in config file.

**Solution:** Add jobs to `ndiff.toml`:
```toml
[[jobs]]
name = "your-job"
namespace = "default"
region = "global"
```

### Configuration validation

```bash
ndiff config check
```

This validates your config and tests connections to Nomad and Git.

## Building from Source

### Prerequisites

- Go 1.21 or later
- Git

### Build

```bash
git clone https://github.com/wlame/ndiff
cd ndiff
go build -o ndiff ./cmd/ndiff
```

### Cross-compilation

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o ndiff-linux ./cmd/ndiff

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o ndiff-darwin-arm64 ./cmd/ndiff

# Windows
GOOS=windows GOARCH=amd64 go build -o ndiff.exe ./cmd/ndiff
```

Or use the included build script:

```bash
./build.sh
# Creates binaries in dist/ directory
```

### Run Tests

```bash
# Unit tests
go test ./internal/...

# Integration tests (requires Docker)
./run-all-tests.sh
```

## Project Structure

```
ndiff/
├── cmd/ndiff/          # Main entry point
├── internal/
│   ├── backend/        # Storage backends (Git, GitHub API)
│   ├── commands/       # CLI commands (sync, deploy, history, etc.)
│   ├── config/         # Configuration management
│   ├── git/            # Git operations
│   ├── hcl/            # HCL formatting
│   └── nomad/          # Nomad API client
├── tests/              # Integration tests
├── docs/               # Additional documentation
│   ├── BACKENDS.md     # Backend comparison and setup
│   └── CLI_GUIDE.md    # Detailed CLI usage
└── README.md           # This file
```

## Documentation

- [Backend Configuration Guide](docs/BACKENDS.md) - Detailed backend comparison
- [CLI Usage Guide](docs/CLI_GUIDE.md) - Complete command reference

## Contributing

Contributions welcome! Please open an issue first to discuss proposed changes.

## License

MIT License - see LICENSE file for details

## Related Projects

- [HashiCorp Nomad](https://www.nomadproject.io/) - The workload orchestrator this tool supports
- [go-git](https://github.com/go-git/go-git) - Pure Go Git implementation used by this project
