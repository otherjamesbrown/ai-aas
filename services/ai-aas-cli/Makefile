# ai-aas-cli Makefile
# Build and development targets for the AI-AAS CLI tool

BINARY_NAME=ai-aas-cli
BINARY_DIR=bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Platforms for cross-compilation
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build clean test coverage lint fmt vet deps tidy help install

all: deps lint test build

## Build targets
build: ## Build the CLI binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) .

build-all: ## Cross-compile for all platforms
	@echo "Building for all platforms..."
	@mkdir -p $(BINARY_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		OUTPUT=$(BINARY_DIR)/$(BINARY_NAME)-$$GOOS-$$GOARCH; \
		if [ "$$GOOS" = "windows" ]; then OUTPUT="$$OUTPUT.exe"; fi; \
		echo "Building $$OUTPUT..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH $(GOBUILD) $(LDFLAGS) -o $$OUTPUT . ; \
	done

install: build ## Install CLI to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BINARY_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

## Test targets
test: ## Run unit tests
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

test-short: ## Run tests without race detector
	@echo "Running tests (short)..."
	$(GOTEST) -v ./...

test-unit: ## Run only unit tests
	@echo "Running unit tests..."
	$(GOTEST) -v ./tests/unit/...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	$(GOTEST) -v ./tests/integration/...

test-e2e: ## Run end-to-end tests
	@echo "Running E2E tests..."
	$(GOTEST) -v ./tests/e2e/...

coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## Code quality targets
lint: ## Run linter
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

fmt: ## Format code
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOCMD) vet ./...

check: fmt vet lint ## Run all code checks

## Dependency targets
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download

tidy: ## Tidy go.mod
	@echo "Tidying go.mod..."
	$(GOMOD) tidy

vendor: ## Vendor dependencies
	@echo "Vendoring dependencies..."
	$(GOMOD) vendor

## Clean targets
clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

## Development targets
run: ## Run the CLI directly
	@$(GOCMD) run . $(ARGS)

watch: ## Watch for changes and rebuild (requires entr)
	@if command -v entr >/dev/null 2>&1; then \
		find . -name "*.go" | entr -r make build; \
	else \
		echo "entr not installed. Install with: brew install entr"; \
	fi

## Help
help: ## Show this help
	@echo "ai-aas-cli Makefile targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
