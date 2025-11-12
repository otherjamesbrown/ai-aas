# API Router Service Makefile

SERVICE_NAME ?= api-router-service

include ../../templates/service.mk

SERVICE_CMD_DIR := $(SERVICE_ROOT)/cmd/router

.PHONY: run
run: build ## Run the router service locally
	@$(SERVICE_BIN_DIR)/$(SERVICE_NAME)

.PHONY: dev-up
dev-up: ## Start local development dependencies (Redis, Kafka, mock backends)
	@docker compose -f $(SERVICE_ROOT)/dev/docker-compose.yml up -d

.PHONY: dev-down
dev-down: ## Stop local development dependencies
	@docker compose -f $(SERVICE_ROOT)/dev/docker-compose.yml down

.PHONY: dev-logs
dev-logs: ## View logs from local development dependencies
	@docker compose -f $(SERVICE_ROOT)/dev/docker-compose.yml logs -f

.PHONY: contract-test
contract-test: _ensure-module ## Run contract tests (validates OpenAPI + protobuf artifacts)
	@echo "Contract tests - TODO: implement with buf"
	@# TODO: Add buf validation once contracts are generated

.PHONY: integration-test
integration-test: _ensure-module ## Run integration tests using docker-compose harness
	@echo "Integration tests - TODO: implement"
	@# TODO: Start docker-compose, run tests, cleanup

.PHONY: contracts
contracts: _ensure-module ## Generate contracts from OpenAPI spec (validates and generates)
	@$(MAKE) contracts-validate
	@$(MAKE) contracts-generate
	@echo "✓ Contracts generated successfully"

.PHONY: contracts-validate
contracts-validate: _ensure-module ## Validate OpenAPI specification only
	@echo "Validating OpenAPI specification..."
	@cd $(SERVICE_ROOT) && go run ./cmd/contracts -validate || { \
		echo "OpenAPI validation failed. Install spectral for better validation:"; \
		echo "  npm install -g @stoplight/spectral-cli"; \
		exit 1; \
	}

.PHONY: contracts-generate
contracts-generate: _ensure-module ## Generate Go types from OpenAPI spec only
	@echo "Generating Go types from OpenAPI specification..."
	@command -v oapi-codegen >/dev/null 2>&1 || { \
		echo "oapi-codegen not installed. Install with:"; \
		echo "  go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest"; \
		exit 1; \
	}
	@cd $(SERVICE_ROOT) && go run ./cmd/contracts -generate || { \
		echo "Contract generation failed"; \
		exit 1; \
	}

