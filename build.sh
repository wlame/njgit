#!/bin/bash
# Build script for ndiff - creates binaries for multiple platforms

set -e

VERSION=${1:-dev}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "Building ndiff version $VERSION"
echo "Commit: $COMMIT"
echo "Build time: $BUILD_TIME"
echo ""

# Build flags to embed version info
LDFLAGS="-X github.com/wlame/ndiff/pkg/version.Version=$VERSION \
         -X github.com/wlame/ndiff/pkg/version.Commit=$COMMIT \
         -X github.com/wlame/ndiff/pkg/version.BuildTime=$BUILD_TIME"

# Create dist directory
mkdir -p dist

# Build for Linux (amd64)
echo "Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/ndiff-linux-amd64 ./cmd/ndiff

# Build for Linux (arm64)
echo "Building for Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/ndiff-linux-arm64 ./cmd/ndiff

# Build for macOS (amd64)
echo "Building for macOS (amd64)..."
GOOS=darwin GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/ndiff-darwin-amd64 ./cmd/ndiff

# Build for macOS (arm64 - Apple Silicon)
echo "Building for macOS (arm64)..."
GOOS=darwin GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/ndiff-darwin-arm64 ./cmd/ndiff

# Build for Windows (amd64)
echo "Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/ndiff-windows-amd64.exe ./cmd/ndiff

echo ""
echo "✅ Build complete! Binaries in dist/"
ls -lh dist/
