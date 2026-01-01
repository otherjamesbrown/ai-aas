# Makefile for Use Case Tests
#
# Targets:
#   test-dev      - Run tests against development cluster (api.dev.otherjamesbrown.com)
#   test-staging  - Run tests against staging cluster (api.staging.otherjamesbrown.com)
#   test-uc       - Run specific UC test (usage: make test-uc UC=UC-ORG-001)
#   help          - Show this help message

.PHONY: test-dev test-staging test-uc help

# Secrets file location
SECRETS_DIR := ../../secrets/env
SECRETS_FILE := $(SECRETS_DIR)/.env

# Load environment-specific API keys from secrets
# Development uses MASTER_ADMIN_API_KEY, staging uses STAGING_MASTER_ADMIN_API_KEY

test-dev:
	@echo "Running UC tests against development cluster..."
	@if [ ! -f $(SECRETS_FILE) ]; then \
		echo "Error: Secrets file not found at $(SECRETS_FILE)"; \
		exit 1; \
	fi
	@export $$(grep -v '^#' $(SECRETS_FILE) | grep MASTER_ADMIN_API_KEY | head -1 | xargs) && \
	if [ -z "$$MASTER_ADMIN_API_KEY" ]; then \
		echo "Error: MASTER_ADMIN_API_KEY not found in $(SECRETS_FILE)"; \
		exit 1; \
	fi && \
	AI_AAS_API_ENDPOINT=https://user-org.dev.otherjamesbrown.com \
	AI_AAS_API_KEY=$$MASTER_ADMIN_API_KEY \
	go test -v ./... -timeout 10m

test-staging:
	@echo "Running UC tests against staging cluster..."
	@if [ ! -f $(SECRETS_FILE) ]; then \
		echo "Error: Secrets file not found at $(SECRETS_FILE)"; \
		exit 1; \
	fi
	@export $$(grep -v '^#' $(SECRETS_FILE) | grep STAGING_MASTER_ADMIN_API_KEY | head -1 | xargs) && \
	if [ -z "$$STAGING_MASTER_ADMIN_API_KEY" ]; then \
		echo "Error: STAGING_MASTER_ADMIN_API_KEY not found in $(SECRETS_FILE)"; \
		exit 1; \
	fi && \
	AI_AAS_API_ENDPOINT=https://user-org.staging.otherjamesbrown.com \
	AI_AAS_API_KEY=$$STAGING_MASTER_ADMIN_API_KEY \
	go test -v ./... -timeout 10m

test-uc:
	@if [ -z "$(UC)" ]; then \
		echo "Error: UC parameter is required. Usage: make test-uc UC=UC-ORG-001"; \
		exit 1; \
	fi
	@echo "Running specific UC test: $(UC)..."
	@if [ ! -f $(SECRETS_FILE) ]; then \
		echo "Error: Secrets file not found at $(SECRETS_FILE)"; \
		exit 1; \
	fi
	@export $$(grep -v '^#' $(SECRETS_FILE) | grep MASTER_ADMIN_API_KEY | head -1 | xargs) && \
	if [ -z "$$MASTER_ADMIN_API_KEY" ]; then \
		echo "Error: MASTER_ADMIN_API_KEY not found in $(SECRETS_FILE)"; \
		exit 1; \
	fi && \
	AI_AAS_API_ENDPOINT=https://user-org.dev.otherjamesbrown.com \
	AI_AAS_API_KEY=$$MASTER_ADMIN_API_KEY \
	go test -v ./... -run "$(UC)" -timeout 10m

help:
	@echo "Use Case Test Infrastructure"
	@echo ""
	@echo "Targets:"
	@echo "  test-dev      - Run tests against development cluster (api.dev.otherjamesbrown.com)"
	@echo "  test-staging  - Run tests against staging cluster (api.staging.otherjamesbrown.com)"
	@echo "  test-uc       - Run specific UC test (usage: make test-uc UC=UC-ORG-001)"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Environment Variables:"
	@echo "  UC            - Use case ID to run (required for test-uc)"
	@echo ""
	@echo "Examples:"
	@echo "  make test-dev              # Run all UC tests against development"
	@echo "  make test-staging          # Run all UC tests against staging"
	@echo "  make test-uc UC=UC-ORG-001 # Run specific UC test against development"
	@echo ""
	@echo "Secrets:"
	@echo "  API keys are loaded from: $(SECRETS_FILE)"
	@echo "  Development:  MASTER_ADMIN_API_KEY"
	@echo "  Staging:      STAGING_MASTER_ADMIN_API_KEY"
