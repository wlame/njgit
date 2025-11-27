# Nomad Changelog

A stateless CLI tool written in Go that tracks Nomad job configuration changes by syncing them to a Git repository, providing version history and rollback capabilities.

## Prerequisites

- Go 1.21 or later
- Git
- Access to a Nomad cluster
- A Git repository for storing job specifications

## Installation

### Install Go (macOS)

```bash
brew install go
```

After installation, verify:
```bash
go version
```

### Build the Project

```bash
# Initialize the Go module (first time only)
go mod init github.com/wlame/nomad-changelog

# Install dependencies
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get github.com/hashicorp/nomad/api@latest
go get github.com/go-git/go-git/v5@latest
go get github.com/hashicorp/hcl/v2@latest

# Build the binary for your current platform
go build -o nomad-changelog ./cmd/nomad-changelog

# Or use the build script to create binaries for all platforms
./build.sh

# This creates binaries in the dist/ directory:
# - dist/nomad-changelog-linux-amd64
# - dist/nomad-changelog-linux-arm64
# - dist/nomad-changelog-darwin-amd64 (macOS Intel)
# - dist/nomad-changelog-darwin-arm64 (macOS Apple Silicon)
# - dist/nomad-changelog-windows-amd64.exe

# Run it
./nomad-changelog --help
```

### Cross-Compilation

To build for a specific platform:

```bash
# Linux (64-bit)
GOOS=linux GOARCH=amd64 go build -o nomad-changelog-linux ./cmd/nomad-changelog

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o nomad-changelog-darwin ./cmd/nomad-changelog

# Windows
GOOS=windows GOARCH=amd64 go build -o nomad-changelog.exe ./cmd/nomad-changelog
```

## Configuration

nomad-changelog supports two storage backends: **Git** (default) and **GitHub API**. See [Backend Configuration Guide](docs/BACKENDS.md) for detailed information.

### Basic Configuration

Create a `nomad-changelog.toml` file in your project directory:

```toml
# Git repository configuration
[git]
backend = "git"  # Options: "git" (default), "github-api"
url = "git@github.com:myorg/nomad-jobs.git"
branch = "main"
auth_method = "ssh"  # or "token" or "auto"
author_name = "nomad-changelog"
author_email = "bot@example.com"

# Nomad configuration
[nomad]
address = "https://nomad.example.com:4646"
token = ""

# Jobs to track
[[jobs]]
name = "web-server"
namespace = "production"

[[jobs]]
name = "api-server"
namespace = "production"
```

### Backend Options

#### Git Backend with Remote (Default)

Best for standard workflows with any Git provider (GitHub, GitLab, Bitbucket, etc.).

```toml
[git]
backend = "git"
url = "git@github.com:myorg/nomad-jobs.git"
branch = "main"
local_path = "."  # Where to store the local repository
repo_name = "nomad-changelog-repo"  # Repository directory name
```

#### Git Backend (Local-Only Mode)

Best for local development without automatic push/pull. **You manage the repository yourself.**

```toml
[git]
backend = "git"
local_only = true  # Enable local-only mode
local_path = "/home/user/repos"  # Directory containing the repository
repo_name = "nomad-jobs"  # Repository directory name
branch = "main"
```

**Setup Requirements**:
```bash
# Initialize the repository yourself
git init -b main /home/user/repos/nomad-jobs
cd /home/user/repos/nomad-jobs
git config user.name "nomad-changelog"
git config user.email "bot@example.com"

# Optional: Add a remote (you'll push manually)
git remote add origin git@github.com:myorg/nomad-jobs.git
```

**Behavior**:
- ❌ No automatic cloning
- ❌ No automatic push/pull
- ✅ Local commits only
- ✅ You control when to push/pull

#### GitHub API Backend

Best for CI/CD environments (stateless, no local repository).

```toml
[git]
backend = "github-api"
owner = "myorg"  # GitHub organization or username
repo = "nomad-jobs"  # Repository name
branch = "main"
```

**Requires**: Set `GITHUB_TOKEN` environment variable with a GitHub Personal Access Token.

See [Backend Configuration Guide](docs/BACKENDS.md) for detailed comparison and examples.

## Usage

### Quick Start

After downloading the binary, run these commands to get started:

```bash
# 1. Create configuration (interactive wizard)
nomad-changelog init

# 2. Verify everything is set up correctly
nomad-changelog config check

# 3. Preview what would be synced
nomad-changelog sync --dry-run

# 4. Start syncing your jobs
nomad-changelog sync
```

### Common Commands

```bash
# Sync all configured jobs
nomad-changelog sync

# Sync specific jobs only
nomad-changelog sync --jobs web-app,api-server

# Preview changes without committing
nomad-changelog sync --dry-run

# Commit locally but don't push
nomad-changelog sync --no-push

# View version history for all jobs
nomad-changelog history

# View history for a specific job
nomad-changelog history --job web-app --namespace production

# Show detailed commit information with files changed
nomad-changelog history --verbose

# Display a specific version of a job
nomad-changelog show <commit-hash> <job-name>

# Display a specific version with namespace
nomad-changelog show <commit-hash> <job-name> --namespace production

# Deploy (rollback to) a previous version
nomad-changelog deploy <commit-hash> <job-name>

# Preview deployment without actually deploying
nomad-changelog deploy <commit-hash> <job-name> --dry-run

# Check configuration and test connections
nomad-changelog config check

# Show current configuration
nomad-changelog config show

# Validate configuration file
nomad-changelog config validate
```

For complete CLI documentation with all scenarios, see [CLI Usage Guide](docs/CLI_GUIDE.md).

## Version History and Rollback

nomad-changelog tracks all job configuration changes in Git, allowing you to view history and rollback to previous versions.

### View History

The `history` command shows all commits with job configuration changes:

```bash
# View all history
nomad-changelog history

# Filter by job name
nomad-changelog history --job web-app

# Filter by namespace
nomad-changelog history --namespace production

# Filter by both
nomad-changelog history --job web-app --namespace production

# Show detailed information including files changed
nomad-changelog history --verbose

# Limit number of commits shown
nomad-changelog history --max-count 10
```

**Example output:**
```
commit a1b2c3d4
Author: nomad-changelog <bot@example.com>
Date:   2 hours ago

    Update job: web-app (namespace: production)

    Files changed:
      production/web-app.nomad

─────────────────────────────────────────────────────

commit e5f6g7h8
Author: nomad-changelog <bot@example.com>
Date:   1 day ago

    Update job: api-server (namespace: production)
```

**For GitHub API backend**, the tool provides direct links to view commits on GitHub.

### Show Specific Version

The `show` command displays the job configuration from a specific commit:

```bash
# Show a specific version
nomad-changelog show a1b2c3d4 web-app

# Specify namespace if needed
nomad-changelog show a1b2c3d4 web-app --namespace production
```

**For Git backend**: Displays the HCL content from that commit
**For GitHub API backend**: Provides a GitHub URL to view the file

### Rollback (Deploy Previous Version)

The `deploy` command redeploys a job from a previous commit:

```bash
# Deploy a previous version
nomad-changelog deploy a1b2c3d4 web-app

# Specify namespace
nomad-changelog deploy a1b2c3d4 web-app --namespace production

# Preview without actually deploying
nomad-changelog deploy a1b2c3d4 web-app --dry-run
```

**What happens during rollback:**
1. Retrieves the job HCL from the specified commit
2. Parses the HCL into a Nomad job specification
3. Submits the job to Nomad for deployment
4. Returns an evaluation ID for tracking

**Example workflow:**
```bash
# 1. View history to find the version you want
nomad-changelog history --job web-app

# 2. Show the specific version to verify it's correct
nomad-changelog show a1b2c3d4 web-app

# 3. Deploy it
nomad-changelog deploy a1b2c3d4 web-app

# Output:
# ✅ Successfully deployed job web-app from commit a1b2c3d4
# Evaluation ID: 8f9e0a1b-2c3d-4e5f-6a7b-8c9d0e1f2a3b
#
# You can check the deployment status with:
#   nomad eval status 8f9e0a1b-2c3d-4e5f-6a7b-8c9d0e1f2a3b
```

**Important notes:**
- The deploy command works with both Git and GitHub API backends
- The deployed job will immediately start running in Nomad
- Use `--dry-run` to preview the job specification before deploying
- After rollback, running `sync` will detect if the deployed version differs from the current Nomad state

## Authentication

### Nomad Authentication

The tool looks for Nomad credentials in this order:
1. `--nomad-token` CLI flag
2. `nomad.token` in config file
3. `NOMAD_TOKEN` environment variable
4. `~/.nomad-token` file

### Git Authentication

**Git Backend with SSH (recommended for development)**:
- Uses SSH key from `git.ssh_key_path` in config
- Falls back to `~/.ssh/id_ed25519`, `~/.ssh/id_rsa`, or SSH agent

**Git Backend with HTTPS Token (recommended for CI/CD)**:
- Set `GITHUB_TOKEN` or `GH_TOKEN` environment variable
- Use `auth_method = "token"` in config

**GitHub API Backend**:
- Requires `GITHUB_TOKEN` or `GH_TOKEN` environment variable
- Token must have `repo` scope for private repos

For detailed backend configuration, see [Backend Configuration Guide](docs/BACKENDS.md).

## Project Structure

```
nomad-changelog/
├── cmd/
│   └── nomad-changelog/
│       └── main.go              # Entry point
├── internal/
│   ├── backend/                 # Storage backends (Git, GitHub API)
│   ├── commands/                # CLI commands
│   ├── config/                  # Configuration management
│   ├── nomad/                   # Nomad API client
│   ├── git/                     # Git operations (go-git library)
│   └── hcl/                     # HCL formatting
├── docs/
│   └── BACKENDS.md              # Backend configuration guide
└── tests/                       # Integration and unit tests
```

## Development Status

✅ **Core Features Complete** ✅

This project is functional and ready for use!

### Completed
- [x] Architecture design
- [x] Project structure setup
- [x] Configuration loading (TOML, environment variables)
- [x] Nomad client implementation
- [x] Git operations (via go-git)
- [x] Dual backend support (Git + GitHub API)
- [x] Change detection
- [x] CLI commands (sync, config, history, show, deploy)
- [x] Version history and rollback functionality
- [x] HCL formatting and normalization
- [x] Unit tests
- [x] Integration tests (including rollback workflow)
- [x] Documentation

## License

MIT License (see LICENSE file)

## Contributing

This is a personal project, but contributions are welcome! Please open an issue to discuss proposed changes.
