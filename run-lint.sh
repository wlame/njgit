#!/bin/bash
# Run golangci-lint for njgit

set -e  # Exit on first error

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo "=================================="
echo "  njgit - Lint Check"
echo "=================================="
echo ""

# Ensure Go bin directory is in PATH
export PATH="$(go env GOPATH)/bin:$PATH"

# Check if golangci-lint is installed
if ! command -v golangci-lint &> /dev/null; then
    echo -e "${YELLOW}⚠️  golangci-lint not found${NC}"
    echo ""
    echo "Install golangci-lint:"
    echo "  https://golangci-lint.run/usage/install/"
    echo ""
    echo "Quick install:"
    echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$(go env GOPATH)/bin"
    echo ""
    exit 1
fi

# Run golangci-lint
echo -e "${BLUE}Running golangci-lint...${NC}"
echo ""

if golangci-lint run --timeout=5m; then
    echo ""
    echo -e "${GREEN}✅ Lint check passed${NC}"
    echo ""
else
    echo ""
    echo -e "${RED}❌ Lint check failed${NC}"
    echo ""
    echo "Fix the issues above and try again"
    exit 1
fi
