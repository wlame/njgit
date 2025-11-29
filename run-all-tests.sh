#!/bin/bash
# Run all tests for njgit

set -e  # Exit on first error

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "=================================="
echo "  njgit - All Tests"
echo "=================================="
echo ""

# Set environment for integration tests (Colima)
export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"
export TESTCONTAINERS_RYUK_DISABLED=true

# Track results
LINT_PASSED=0
UNIT_PASSED=0
INTEGRATION_PASSED=0

# Run linter
echo -e "${BLUE}[1/3] Running linter...${NC}"
echo ""
if bash run-lint.sh; then
    LINT_PASSED=1
    echo ""
    echo -e "${GREEN}✅ Lint check passed${NC}"
else
    echo ""
    echo -e "${RED}❌ Lint check failed${NC}"
    exit 1
fi

echo ""
echo "=================================="
echo ""

# Run unit tests
echo -e "${BLUE}[2/3] Running unit tests...${NC}"
echo ""
if go test -v -short ./tests; then
    UNIT_PASSED=1
    echo ""
    echo -e "${GREEN}✅ Unit tests passed${NC}"
else
    echo ""
    echo -e "${RED}❌ Unit tests failed${NC}"
    exit 1
fi

echo ""
echo "=================================="
echo ""

# Run integration tests
echo -e "${BLUE}[3/3] Running integration tests...${NC}"
echo "  DOCKER_HOST=$DOCKER_HOST"
echo "  TESTCONTAINERS_RYUK_DISABLED=$TESTCONTAINERS_RYUK_DISABLED"
echo ""
if go test -v -run "TestNomadContainer|TestFetchAndNormalizeJob|TestGitRepository|TestFullSyncWorkflow|TestHistoryAndDeploy" ./tests; then
    INTEGRATION_PASSED=1
    echo ""
    echo -e "${GREEN}✅ Integration tests passed${NC}"
else
    echo ""
    echo -e "${RED}❌ Integration tests failed${NC}"
    exit 1
fi

echo ""
echo "=================================="
echo -e "${GREEN}  All Checks Passed! ✅${NC}"
echo "=================================="
echo ""
echo "Summary:"
echo "  Lint check: ✅ Passed"
echo "  Unit tests: ✅ Passed"
echo "  Integration tests: ✅ Passed"
echo ""
