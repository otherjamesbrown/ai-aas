# Makefile for Use Case Tests
#
# Targets:
#   test-dev      - Run tests against development cluster
#   test-staging  - Run tests against staging cluster
#   test-uc       - Run specific UC test (usage: make test-uc UC=UC-ORG-001)
#   test          - Run tests (auto-loads credentials from secrets/env/.env)
#   help          - Show this help message
#
# Note: API keys are auto-loaded from secrets/env/.env via the testconfig loader.
# You only need to set AI_AAS_API_ENDPOINT for the target environment.

.PHONY: test-dev test-staging test-uc test help

# Secrets file location (used for Makefile-based loading)
SECRETS_DIR := ../../secrets/env
SECRETS_FILE := $(SECRETS_DIR)/.env

# Development cluster endpoint
DEV_ENDPOINT := https://user-org.dev.otherjamesbrown.com

# Staging cluster endpoint
STAGING_ENDPOINT := https://user-org.staging.otherjamesbrown.com

test-dev:
	@echo "Running UC tests against development cluster..."
	@AI_AAS_API_ENDPOINT=$(DEV_ENDPOINT) go test -v ./... -timeout 10m

test-staging:
	@echo "Running UC tests against staging cluster..."
	@AI_AAS_API_ENDPOINT=$(STAGING_ENDPOINT) go test -v ./... -timeout 10m

test-uc:
	@if [ -z "$(UC)" ]; then \
		echo "Error: UC parameter is required. Usage: make test-uc UC=UC-ORG-001"; \
		exit 1; \
	fi
	@echo "Running specific UC test: $(UC)..."
	@AI_AAS_API_ENDPOINT=$(DEV_ENDPOINT) go test -v ./... -run "$(UC)" -timeout 10m

test:
	@echo "Running UC tests (auto-loading credentials)..."
	@go test -v ./... -timeout 10m

help:
	@echo "Use Case Test Infrastructure"
	@echo ""
	@echo "Targets:"
	@echo "  test-dev      - Run tests against development cluster"
	@echo "  test-staging  - Run tests against staging cluster"
	@echo "  test-uc       - Run specific UC test (usage: make test-uc UC=UC-ORG-001)"
	@echo "  test          - Run tests (requires AI_AAS_API_ENDPOINT to be set)"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Environment Variables:"
	@echo "  AI_AAS_API_ENDPOINT - API endpoint (set automatically by test-dev/test-staging)"
	@echo "  AI_AAS_API_KEY      - API key (auto-loaded from secrets/env/.env)"
	@echo "  UC                  - Use case ID to run (required for test-uc)"
	@echo ""
	@echo "Examples:"
	@echo "  make test-dev              # Run all UC tests against development"
	@echo "  make test-staging          # Run all UC tests against staging"
	@echo "  make test-uc UC=UC-ORG-001 # Run specific UC test against development"
	@echo ""
	@echo "Secrets:"
	@echo "  API keys auto-loaded from: $(SECRETS_FILE)"
	@echo "  Uses testconfig loader from shared/go/testconfig"
