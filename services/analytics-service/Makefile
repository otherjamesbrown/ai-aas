# Analytics Service Makefile

SERVICE_NAME ?= analytics-service

include ../../templates/service.mk

SERVICE_CMD_DIR := $(SERVICE_ROOT)/cmd/analytics-service

.PHONY: run
run: build ## Run the analytics service locally
	@$(SERVICE_BIN_DIR)/$(SERVICE_NAME)

.PHONY: dev-up
dev-up: ## Start local development dependencies (Postgres, Redis, RabbitMQ)
	@docker compose -f $(SERVICE_ROOT)/dev/docker-compose.yml up -d

.PHONY: dev-down
dev-down: ## Stop local development dependencies
	@docker compose -f $(SERVICE_ROOT)/dev/docker-compose.yml down

.PHONY: dev-logs
dev-logs: ## View logs from local development dependencies
	@docker compose -f $(SERVICE_ROOT)/dev/docker-compose.yml logs -f

