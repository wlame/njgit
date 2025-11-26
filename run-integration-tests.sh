#!/bin/bash
# Helper script to run integration tests with proper environment setup

# Set DOCKER_HOST for Colima
export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"

# Disable ryuk reaper (has issues with Colima on macOS)
export TESTCONTAINERS_RYUK_DISABLED=true

echo "Running integration tests with:"
echo "  DOCKER_HOST=$DOCKER_HOST"
echo "  TESTCONTAINERS_RYUK_DISABLED=$TESTCONTAINERS_RYUK_DISABLED"
echo ""

# Run the tests
go test -v ./tests/integration_test.go "$@"
