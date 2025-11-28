# Makefile for njgit

# Variables
BINARY_NAME=njgit
MAIN_PATH=./cmd/njgit
BUILD_DIR=.
GO=go

# Build version information
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go build flags
LDFLAGS=-ldflags "-X github.com/wlame/njgit/pkg/version.Version=$(VERSION) \
	-X github.com/wlame/njgit/pkg/version.Commit=$(COMMIT) \
	-X github.com/wlame/njgit/pkg/version.BuildTime=$(BUILD_TIME)"

.PHONY: all build install clean test deps run help

# Default target
all: build

## help: Show this help message
help:
	@echo "Available targets:"
	@grep -E '^##' Makefile | sed 's/^## /  /'

## deps: Download and install dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

## build: Build the binary
build: deps
	$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

## install: Install the binary to GOPATH/bin
install: deps
	$(GO) install $(LDFLAGS) $(MAIN_PATH)
	@echo "Binary installed to $(GOPATH)/bin/$(BINARY_NAME)"

## clean: Remove built binaries
clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	$(GO) clean

## test: Run tests
test:
	$(GO) test -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out

## fmt: Format Go code
fmt:
	$(GO) fmt ./...

## vet: Run go vet
vet:
	$(GO) vet ./...

## lint: Run golangci-lint (requires golangci-lint to be installed)
lint:
	golangci-lint run

## run: Build and run the binary
run: build
	./$(BINARY_NAME)

## run-help: Build and show help
run-help: build
	./$(BINARY_NAME) --help

## dev-deps: Install development dependencies
dev-deps:
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
