# ai-aas-org CLI Makefile

BINARY_NAME=ai-aas-org
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOGET=$(GOCMD) get

# Build directories
BUILD_DIR=build
DIST_DIR=dist

.PHONY: all build clean test tidy install help

all: build

## build: Build the binary for the current platform
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/ai-aas-org

## build-all: Build for all platforms
build-all: clean
	mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_linux_amd64/$(BINARY_NAME) ./cmd/ai-aas-org
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_linux_arm64/$(BINARY_NAME) ./cmd/ai-aas-org
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_darwin_amd64/$(BINARY_NAME) ./cmd/ai-aas-org
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_darwin_arm64/$(BINARY_NAME) ./cmd/ai-aas-org

## release: Build release archives with version in filename
release: build-all
	cd $(DIST_DIR) && \
	for d in $(BINARY_NAME)_$(VERSION)_*; do \
		tar -czf "$$d.tar.gz" -C "$$d" $(BINARY_NAME); \
	done

## test: Run tests
test:
	$(GOTEST) -v ./...

## coverage: Run tests with coverage
coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## tidy: Tidy and verify dependencies
tidy:
	$(GOMOD) tidy
	$(GOMOD) verify

## install: Install binary to GOPATH/bin (or GOBIN if set)
install:
	$(GOCMD) install $(LDFLAGS) ./cmd/ai-aas-org

## clean: Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	rm -f coverage.out coverage.html

## version: Show version info
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

## help: Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/  /'
