# Shared service Makefile template

SERVICE_NAME ?= user-org-service

include ../../templates/service.mk

MIGRATIONS_DIR := $(SERVICE_ROOT)/migrations/sql
SQLC_CONFIG := $(SERVICE_ROOT)/sqlc.yaml
GOOSE ?= goose
SQLC ?= sqlc
USER_ORG_DATABASE_URL ?= $(DATABASE_URL)

.PHONY: migrate
migrate: _ensure-module ## Apply database migrations using goose
	@if [ -z "$(USER_ORG_DATABASE_URL)" ]; then \
		echo "USER_ORG_DATABASE_URL env var must be set (e.g., postgres://user:pass@host:5432/dbname?sslmode=disable)"; \
		exit 1; \
	fi
	@$(GOOSE) -dir "$(MIGRATIONS_DIR)" postgres "$(USER_ORG_DATABASE_URL)" up

.PHONY: rollback
rollback: _ensure-module ## Roll back the last migration
	@if [ -z "$(USER_ORG_DATABASE_URL)" ]; then \
		echo "USER_ORG_DATABASE_URL env var must be set"; \
		exit 1; \
	fi
	@$(GOOSE) -dir "$(MIGRATIONS_DIR)" postgres "$(USER_ORG_DATABASE_URL)" down

.PHONY: schema-drift
schema-drift: _ensure-module ## Check migration status to detect drift
	@if [ -z "$(USER_ORG_DATABASE_URL)" ]; then \
		echo "USER_ORG_DATABASE_URL env var must be set"; \
		exit 1; \
	fi
	@$(GOOSE) -dir "$(MIGRATIONS_DIR)" postgres "$(USER_ORG_DATABASE_URL)" status

.PHONY: sqlc-generate
sqlc-generate: _ensure-module ## Generate typed DB accessors via sqlc
	@command -v $(SQLC) >/dev/null 2>&1 || { echo "sqlc not installed"; exit 1; }
	@$(SQLC) generate -f "$(SQLC_CONFIG)"

.PHONY: build-e2e-test
build-e2e-test: _ensure-module ## Build the e2e-test binary
	@mkdir -p "$(SERVICE_BIN_DIR)"
	@echo "Building e2e-test binary"
	@$(GO) build -trimpath -o "$(SERVICE_BIN_DIR)/e2e-test" ./cmd/e2e-test

.PHONY: e2e-test
e2e-test: build-e2e-test ## Run end-to-end tests (set API_URL to test against deployed service)
	@API_URL=$${API_URL:-http://localhost:8081} "$(SERVICE_BIN_DIR)/e2e-test"

.PHONY: e2e-test-local
e2e-test-local: e2e-test ## Run e2e tests against localhost (default)

.PHONY: e2e-test-dev
e2e-test-dev: build-e2e-test ## Run e2e tests against development environment
	@if [ -z "$$DEV_API_URL" ]; then \
		echo "DEV_API_URL must be set (e.g., http://user-org-service.dev.svc.cluster.local:8081)"; \
		exit 1; \
	fi
	@API_URL=$$DEV_API_URL "$(SERVICE_BIN_DIR)/e2e-test"

.PHONY: docker-build-e2e-test
docker-build-e2e-test: ## Build Docker image for e2e-test
	@docker build -f Dockerfile.e2e-test -t user-org-service-e2e-test:latest .

.PHONY: deploy-e2e-test
deploy-e2e-test: ## Deploy and run e2e-test in development environment
	@./scripts/deploy-e2e-test.sh


