# Release Process

This document describes how to create a new release of njgit.

## Semantic Versioning

njgit follows [Semantic Versioning](https://semver.org/):
- **MAJOR** version (x.0.0): Incompatible API changes
- **MINOR** version (0.x.0): New functionality in a backwards compatible manner
- **PATCH** version (0.0.x): Backwards compatible bug fixes

## Automated Release Process

Releases are automatically created when you push a tag matching the pattern `v*.*.*`.

### Step-by-Step Release

1. **Update VERSION file**
   ```bash
   echo "0.2.0" > VERSION
   git add VERSION
   ```

2. **Update CHANGELOG** (if you have one)
   Document what's new, changed, fixed, etc.

3. **Commit the changes**
   ```bash
   git commit -m "Release v0.2.0"
   git push origin main
   ```

4. **Create and push a tag**
   ```bash
   git tag -a v0.2.0 -m "Release version 0.2.0"
   git push origin v0.2.0
   ```

5. **GitHub Actions will automatically:**
   - Build binaries for all platforms (Linux, macOS, Windows)
   - Generate SHA256 checksums
   - Create a GitHub Release
   - Upload all artifacts

## Manual Release (if needed)

If you need to create a release manually:

1. **Build all binaries**
   ```bash
   ./build.sh 0.2.0
   ```

2. **Create checksums**
   ```bash
   cd dist
   sha256sum * > checksums.txt
   ```

3. **Create GitHub Release**
   - Go to GitHub → Releases → Draft a new release
   - Choose tag: `v0.2.0`
   - Upload files from `dist/` directory
   - Publish release

## Version Number Guidelines

### When to increment MAJOR (x.0.0)
- Breaking changes to CLI flags or arguments
- Incompatible config file format changes
- Removal of features

### When to increment MINOR (0.x.0)
- New commands added
- New features added to existing commands
- New configuration options

### When to increment PATCH (0.0.x)
- Bug fixes
- Performance improvements
- Documentation updates
- Dependency updates (security)

## Pre-release Versions

For pre-release versions, use suffixes:
- Alpha: `0.2.0-alpha.1`
- Beta: `0.2.0-beta.1`
- Release Candidate: `0.2.0-rc.1`

Tag format: `v0.2.0-beta.1`

## Release Checklist

Before creating a release:

- [ ] All tests passing (`./run-all-tests.sh`)
- [ ] VERSION file updated
- [ ] CHANGELOG updated (if applicable)
- [ ] Documentation reflects new features
- [ ] README installation instructions current
- [ ] Breaking changes documented
- [ ] Migration guide written (if breaking changes)
- [ ] Tag follows `vX.Y.Z` format

## Example Release Workflow

```bash
# Current version: 0.1.0
# Want to release: 0.2.0 with new features

# 1. Update VERSION
echo "0.2.0" > VERSION

# 2. Test everything
./run-all-tests.sh
./build.sh 0.2.0
./dist/njgit-darwin-arm64 version  # Verify

# 3. Commit and push
git add VERSION
git commit -m "Bump version to 0.2.0"
git push origin main

# 4. Tag and push
git tag -a v0.2.0 -m "Release v0.2.0

- Add feature X
- Fix bug Y
- Improve performance Z"

git push origin v0.2.0

# 5. Wait for GitHub Actions to complete
# 6. Verify release on GitHub
```

## Troubleshooting

### GitHub Actions fails

1. Check the Actions tab on GitHub
2. Review the build logs
3. Fix any issues
4. Delete the tag: `git tag -d v0.2.0 && git push origin :refs/tags/v0.2.0`
5. Try again

### Wrong version in binary

Make sure:
- VERSION file is committed
- Tag is correct format (vX.Y.Z)
- LDFLAGS in GitHub Actions match pkg/version paths

### Release not created

Ensure:
- Tag matches `v*.*.*` pattern (must start with 'v')
- GitHub Actions has write permissions
- GITHUB_TOKEN is available
