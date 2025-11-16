.SHELLFLAGS := -eu -o pipefail -c
SHELL := /bin/bash

ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
SERVICES := $(shell find services -mindepth 1 -maxdepth 1 -type d ! -name '_template' -exec basename {} \;)
SERVICE ?= all
NAME ?=
RUN_ID ?= $(shell date +%s)

SHARED_GO_DIR := $(ROOT_DIR)/shared/go
SHARED_TS_DIR := $(ROOT_DIR)/shared/ts
SHARED_GO_COVERAGE_TARGET ?= 80
SHARED_TS_NODE_MODULES := $(SHARED_TS_DIR)/node_modules
SHARED_TS_TEST_DIR := $(ROOT_DIR)/tests/ts/unit
SHARED_TS_TEST_NODE_MODULES := $(SHARED_TS_TEST_DIR)/node_modules

include configs/tool-versions.mk

export GOTOOLCHAIN := $(GO_TOOLCHAIN)

.DEFAULT_GOAL := help

_BOLD := \033[1m
_DIM := \033[2m
_GREEN := \033[32m
_RED := \033[31m
_RESET := \033[0m

.PHONY: help
help: ## Display available Make targets with descriptions
	@printf "\n${_BOLD}AI-AAS Automation${_RESET}\n"
	@printf "${_DIM}Use \`make <target>\` to run a task. Pass SERVICE=<name> when applicable.${_RESET}\n\n"
	@awk 'BEGIN {FS = ":.*##"; printf "%-20s %s\n", "Target", "Description"} \
		/^[-_a-zA-Z0-9%]+:.*##/ {gsub(/^@?/, "", $$1); printf "  %-18s %s\n", $$1, $$2} \
		/^##@/ {printf "\n%s\n", substr($$0, 4)}' $(MAKEFILE_LIST)

.PHONY: ensure-services-dir
ensure-services-dir:
	@mkdir -p services

define RUN_SERVICE_TARGET
	@if [ -z "$(SERVICES)" ]; then \
		echo "No services found under services/. Skipping."; \
	elif [ "$(SERVICE)" = "all" ]; then \
		for svc in $(SERVICES); do \
			echo ">> Running $1 for $$svc"; \
			$(MAKE) --no-print-directory -C services/$$svc $1 || exit $$?; \
		done; \
	else \
		if [ ! -d "services/$(SERVICE)" ]; then \
			echo "Service '$(SERVICE)' not found under services/." >&2; \
			exit 1; \
		fi; \
		echo ">> Running $1 for $(SERVICE)"; \
		$(MAKE) --no-print-directory -C services/$(SERVICE) $1; \
	fi
endef

.PHONY: shared-go-build
shared-go-build: ## Build shared Go libraries
	@echo ">> Building shared Go libraries"
	@cd $(SHARED_GO_DIR) && go build ./...

.PHONY: shared-go-test
shared-go-test: ## Run Go unit tests for shared libraries
	@echo ">> Testing shared Go libraries (coverage target $(SHARED_GO_COVERAGE_TARGET)%)"
	@cd $(SHARED_GO_DIR) && go test ./... -coverprofile=coverage.out -covermode=atomic
	@cd $(SHARED_GO_DIR) && go tool cover -func=coverage.out | awk -v target=$(SHARED_GO_COVERAGE_TARGET) 'BEGIN { status=0 } /^total:/ { gsub("%","",$$3); if ($$3+0 < target) { printf "total coverage %.2f%% below target %.2f%%\n", $$3, target; status=1 } } END { exit status }'
	@cd $(SHARED_GO_DIR) && rm -f coverage.out

.PHONY: shared-go-lint
shared-go-lint: ## Run go vet for shared libraries
	@echo ">> Vetting shared Go libraries"
	@cd $(SHARED_GO_DIR) && go vet ./...

.PHONY: shared-go-check
shared-go-check: shared-go-build shared-go-test shared-go-lint ## Run build/test/vet for shared Go libraries

$(SHARED_TS_NODE_MODULES):
	@echo ">> Installing dependencies for shared TypeScript libraries"
	@cd $(SHARED_TS_DIR) && npm install

$(SHARED_TS_TEST_NODE_MODULES):
	@echo ">> Installing dependencies for shared TypeScript unit tests"
	@cd $(SHARED_TS_TEST_DIR) && npm install

.PHONY: shared-ts-install
shared-ts-install: $(SHARED_TS_NODE_MODULES) $(SHARED_TS_TEST_NODE_MODULES) ## Install dependencies for shared TypeScript libraries and tests

.PHONY: shared-ts-build
shared-ts-build: shared-ts-install ## Build shared TypeScript libraries
	@echo ">> Building shared TypeScript libraries"
	@cd $(SHARED_TS_DIR) && npm run build

.PHONY: shared-ts-test
shared-ts-test: shared-ts-install ## Run TypeScript unit tests for shared libraries
	@echo ">> Running shared TypeScript unit tests"
	@cd $(SHARED_TS_DIR) && npm run test
	@cd $(SHARED_TS_TEST_DIR) && npm run test

.PHONY: shared-ts-lint
shared-ts-lint: shared-ts-install ## Run ESLint against shared TypeScript sources
	@echo ">> Linting shared TypeScript libraries"
	@cd $(SHARED_TS_DIR) && npm run lint
	@cd $(SHARED_TS_TEST_DIR) && npm run lint

.PHONY: shared-ts-check
shared-ts-check: shared-ts-build shared-ts-test shared-ts-lint ## Run build/test/lint for shared TypeScript libraries

.PHONY: shared-build
shared-build: shared-go-build shared-ts-build ## Build all shared libraries

.PHONY: shared-test
shared-test: shared-go-test shared-ts-test ## Test all shared libraries

.PHONY: shared-check
shared-check: shared-go-check shared-ts-check ## Run checks for shared Go and TypeScript libraries

.PHONY: build
build: ## Build Go services (SERVICE=all|<name>)
	$(call RUN_SERVICE_TARGET,build)

.PHONY: build-all
build-all: ## Build all services (alias for build SERVICE=all)
	@$(MAKE) --no-print-directory build SERVICE=all

.PHONY: test
test: ## Run tests for services (SERVICE=all|<name>)
	$(call RUN_SERVICE_TARGET,test)

.PHONY: fmt
fmt: ## Run formatting commands for services (SERVICE=all|<name>)
	$(call RUN_SERVICE_TARGET,fmt)
	@if [ "$(SERVICE)" = "all" ]; then \
		$(MAKE) --no-print-directory gofmt; \
	fi

.PHONY: lint
lint: ## Run golangci-lint for services (SERVICE=all|<name>)
	$(call RUN_SERVICE_TARGET,lint)

.PHONY: security
security: ## Run gosec security scan (SERVICE=all|<name>)
	$(call RUN_SERVICE_TARGET,security)

.PHONY: clean
clean: ## Clean build artifacts (SERVICE=all|<name>)
	$(call RUN_SERVICE_TARGET,clean)

.PHONY: check
check: ## Run format, lint, security, and tests (SERVICE=all|<name>, METRICS=true to emit metrics)
	@status=success; \
	if [ "$(SERVICE)" = "all" ]; then \
		if [ -z "$(SERVICES)" ]; then \
			echo "No services found under services/. Skipping."; \
		else \
			for svc in $(SERVICES); do \
				echo ">> Running check for $$svc"; \
				if ! $(MAKE) --no-print-directory -C services/$$svc check; then \
					status=failure; \
				fi; \
			done; \
		fi; \
	else \
		if [ ! -d "services/$(SERVICE)" ]; then \
			echo "Service '$(SERVICE)' not found under services/." >&2; \
			exit 1; \
		fi; \
		echo ">> Running check for $(SERVICE)"; \
		if ! $(MAKE) --no-print-directory -C services/$(SERVICE) check; then \
			status=failure; \
		fi; \
	fi; \
	echo ">> Running shared library checks"; \
	if ! $(MAKE) --no-print-directory shared-check; then \
		status=failure; \
	fi; \
	if [ "$(METRICS)" = "true" ]; then \
		echo "Collecting metrics with run id $(RUN_ID) (status=$$status)"; \
		go run ./scripts/metrics/collector.go \
			--run-id "$(RUN_ID)" \
			--service "$(SERVICE)" \
			--command "make check" \
			--status "$$status" \
			--duration 0 >/dev/null || true; \
	fi; \
	if [ "$$status" != "success" ]; then \
		exit 1; \
	fi

.PHONY: ci-local
ci-local: ## Execute GitHub Actions workflow locally via \`act\` (WORKFLOW=ci)
	@WORKFLOW=${WORKFLOW:-ci} ACTOR=${ACTOR:-automation@ai-aas.dev} "$(ROOT_DIR)/scripts/ci/run-local.sh"

.PHONY: ci-remote
ci-remote: ## Trigger GitHub Actions workflow_dispatch run (SERVICE|REF|NOTES variables supported)
	@SERVICE_ARG=$${SERVICE:-all}; \
	REF_ARG=$${REF:-$(shell git rev-parse --abbrev-ref HEAD)}; \
	NOTES_ARG=$${NOTES:-"Triggered via make ci-remote"}; \
	SERVICE="$$SERVICE_ARG" REF="$$REF_ARG" NOTES="$$NOTES_ARG" "$(ROOT_DIR)/scripts/ci/trigger-remote.sh"

.PHONY: service-new
service-new: ensure-services-dir ## Generate a new service skeleton (NAME=<service-name>)
	@test -n "$(NAME)" || (echo "NAME variable required, e.g. make service-new NAME=billing-service" >&2 && exit 1)
	@scripts/service/new.sh "$(NAME)"

.PHONY: bootstrap
bootstrap: ## Run local environment bootstrap script
	@./scripts/setup/bootstrap.sh

.PHONY: bootstrap-check
bootstrap-check: ## Verify bootstrap prerequisites without installing
	@./scripts/setup/bootstrap.sh --check-only

.PHONY: metrics-upload
metrics-upload: ## Upload metrics artifact (FILE=<path>, optional METRICS_BUCKET/METRICS_PREFIX)
	@test -n "$(FILE)" || (echo "FILE variable required, e.g. make metrics-upload FILE=scripts/metrics/output/run.json" >&2 && exit 1)
	@./scripts/metrics/upload.sh "$(FILE)"

.PHONY: db-migrate-status
db-migrate-status: ## Display latest migration status for operational and analytics databases
	@./scripts/db/apply.sh --status

.PHONY: db-docs-generate
db-docs-generate: ## Generate schema documentation artifacts (dictionary + ERD)
	@./scripts/db/docgen.sh generate

.PHONY: db-docs-validate
db-docs-validate: ## Validate schema documentation consistency with live database state
	@./scripts/db/docgen.sh validate

.PHONY: analytics-rollup-run
analytics-rollup-run: ## Execute analytics rollup (PERIOD=hourly|daily, requires migrate.env or DB_URL)
	@./scripts/analytics/run-hourly.sh $(if $(PERIOD),--period $(PERIOD),)

.PHONY: analytics-verify
analytics-verify: ## Run analytics reconciliation tests (requires migrate.env or DB_URL)
	@./scripts/analytics/verify.sh

.PHONY: gofmt
gofmt: ## Run gofmt across repository
	@find . -name '*.go' -not -path './vendor/*' -exec gofmt -w {} +

.PHONY: version
version: ## Display pinned tool versions
	@printf "Go: %s\n" "$(GO_VERSION)"
	@printf "golangci-lint: %s\n" "$(GOLANGCI_LINT_VERSION)"
	@printf "gosec: %s\n" "$(GOSEC_VERSION)"
	@printf "act: %s\n" "$(ACT_VERSION)"
	@printf "aws-cli: %s\n" "$(AWS_CLI_VERSION)"
	@printf "minio-client: %s\n" "$(MINIO_CLIENT_VERSION)"
	@printf "terraform: %s\n" "$(TERRAFORM_VERSION)"
	@printf "helm: %s\n" "$(HELM_VERSION)"
	@printf "tfsec: %s\n" "$(TFSEC_VERSION)"
	@printf "tflint: %s\n" "$(TFLINT_VERSION)"
	@printf "terratest: %s\n" "$(TERRATEST_VERSION)"

INFRA_TERRAFORM_MAKE := $(ROOT_DIR)/infra/terraform/Makefile

.PHONY: infra-init infra-fmt infra-validate infra-plan infra-apply infra-destroy infra-drift infra-state-pull
infra-%: ## Run Terraform $* target (ENV=<environment>)
	@$(MAKE) --no-print-directory -f $(INFRA_TERRAFORM_MAKE) $* \
		ENV="$(ENV)" \
		PLAN_ARGS="$(PLAN_ARGS)" \
		APPLY_ARGS="$(APPLY_ARGS)" \
		DESTROY_ARGS="$(DESTROY_ARGS)" \
		DRIFT_ARGS="$(DRIFT_ARGS)"

##@ Dev Environment - Remote Workspace

# Remote workspace lifecycle commands with TTL enforcement and audit logging.
# All operations are logged to ~/.ai-aas/workspace-audit.log for traceability.
# TTL warnings are shown when workspace age approaches expiration (default: 24h).

.PHONY: remote-provision remote-up remote-status remote-logs remote-stop remote-reset remote-destroy remote-secrets
remote-provision: ## Provision remote workspace (WORKSPACE_NAME=, WORKSPACE_OWNER= required; default TTL: 24h)
	@./scripts/dev/remote_provision.sh apply --workspace $(WORKSPACE_NAME) --owner $(WORKSPACE_OWNER)

remote-up: ## Start dev stack on remote workspace (WORKSPACE_HOST=, WORKSPACE_NAME= required)
	@./scripts/dev/remote_lifecycle.sh up --workspace-host $(WORKSPACE_HOST) --workspace $(WORKSPACE_NAME)

remote-status: ## Check remote workspace status (WORKSPACE_HOST=, WORKSPACE_NAME= required; JSON=true for JSON output)
	@./scripts/dev/remote_lifecycle.sh status --workspace-host $(WORKSPACE_HOST) --workspace $(WORKSPACE_NAME) $(if $(JSON),--json,)

remote-logs: ## View logs from remote workspace (WORKSPACE_HOST=, WORKSPACE_NAME= required; COMPONENT= optional)
	@./scripts/dev/remote_lifecycle.sh logs --workspace-host $(WORKSPACE_HOST) --workspace $(WORKSPACE_NAME) $(if $(COMPONENT),--component $(COMPONENT),)

remote-stop: ## Stop dev stack on remote workspace (WORKSPACE_HOST=, WORKSPACE_NAME= required; alias for remote-down)
	@./scripts/dev/remote_lifecycle.sh stop --workspace-host $(WORKSPACE_HOST) --workspace $(WORKSPACE_NAME)

remote-reset: ## Reset remote dev stack (WORKSPACE_HOST=, WORKSPACE_NAME= required; removes all data, enforces 90-day log retention)
	@./scripts/dev/remote_lifecycle.sh reset --workspace-host $(WORKSPACE_HOST) --workspace $(WORKSPACE_NAME)

remote-destroy: ## Destroy remote workspace completely (WORKSPACE_HOST=, WORKSPACE_NAME= required; stops stack, removes data, cleans systemd)
	@./scripts/dev/remote_lifecycle.sh destroy --workspace-host $(WORKSPACE_HOST) --workspace $(WORKSPACE_NAME)

remote-secrets: ## Sync secrets from GitHub to .env files (WORKSPACE_NAME= optional)
	@cd cmd/secrets-sync && go run . --verbose $(if $(WORKSPACE_NAME),--workspace $(WORKSPACE_NAME),)

.PHONY: dev-status
dev-status: ## Check dev stack component health (MODE=local|remote; HOST= required for remote; JSON=true for JSON; --diagnose for diagnostics)
	@cd cmd/dev-status && go run . --mode $(if $(MODE),$(MODE),local) $(if $(HOST),--host $(HOST),) $(if $(JSON),--json,) $(if $(HUMAN),--human,) $(if $(DIAGNOSE),--diagnose,)

##@ Dev Environment - Local Development

# Local development stack lifecycle commands.
# Port conflicts are detected and remediation guidance is provided via 'make diagnose'.
# Use environment variables to override default ports (see .specify/local/ports.yaml).

.PHONY: up status logs stop reset seed-data diagnose
up: ## Start local dev stack (postgres, redis, nats, minio, mock-inference)
	@./scripts/dev/local_lifecycle.sh up

status: ## Check local dev stack status (JSON=true for JSON output; use 'make diagnose' for detailed diagnostics)
	@./scripts/dev/local_lifecycle.sh status $(if $(JSON),--json,)

logs: ## View logs from local dev stack (COMPONENT= optional; e.g., COMPONENT=postgres)
	@./scripts/dev/local_lifecycle.sh logs $(if $(COMPONENT),--component $(COMPONENT),)

logs-view: ## View logs via Loki/Grafana (SERVICE= optional service name; opens Grafana Explore or provides Loki query URL)
	@./scripts/dev/log-view.sh $(if $(SERVICE),SERVICE=$(SERVICE),)

logs-tail: ## Tail logs from Docker Compose services (SERVICE= optional service name; follows logs by default)
	@./scripts/dev/log-tail.sh $(if $(SERVICE),SERVICE=$(SERVICE),) --follow

logs-service: ## View logs for specific service from Loki (SERVICE= required service name)
	@if [ -z "$(SERVICE)" ]; then \
		echo "Error: SERVICE is required"; \
		echo "Usage: make logs-service SERVICE=user-org-service"; \
		exit 1; \
	fi
	@./scripts/dev/log-view.sh SERVICE=$(SERVICE)

logs-error: ## Filter logs for error level entries (SERVICE= optional service name)
	@if [ -n "$(SERVICE)" ]; then \
		./scripts/dev/log-tail.sh SERVICE=$(SERVICE) --follow | grep -i "error\|fatal\|panic" || true; \
	else \
		./scripts/dev/log-tail.sh --follow | grep -i "error\|fatal\|panic" || true; \
	fi

logs-verbose: ## Set LOG_LEVEL=debug and restart services (requires services to be running)
	@echo "Setting LOG_LEVEL=debug in environment..."
	@export LOG_LEVEL=debug
	@echo "Note: Services need to be restarted to pick up LOG_LEVEL change"
	@echo "Run: make stop && make up"
	@echo "Or restart specific service: docker compose -f .dev/compose/compose.base.yaml -f .dev/compose/compose.local.yaml restart <service>"

stop: ## Stop local dev stack (graceful shutdown)
	@./scripts/dev/local_lifecycle.sh down

reset: ## Reset local dev stack (stops stack, removes volumes/data, restarts, re-seeds sample data)
	@./scripts/dev/local_lifecycle.sh reset

seed-data: ## Seed local database with sample data (.dev/data/seed.sql)
	@./scripts/dev/local_lifecycle.sh seed-data

diagnose: ## Diagnose local dev stack (check port conflicts, config files, Docker network; provides remediation guidance)
	@./scripts/dev/local_lifecycle.sh diagnose

##@ Environment Management

env-activate: ## Activate environment profile (ENVIRONMENT=local-dev|remote-dev|production)
	@if [ -z "$(ENVIRONMENT)" ]; then \
		echo "Error: ENVIRONMENT is required"; \
		echo "Usage: make env-activate ENVIRONMENT=local-dev"; \
		exit 1; \
	fi
	@./configs/manage-env.sh activate $(ENVIRONMENT)

env-show: ## Show current environment configuration (COMPONENT=optional component name)
	@./configs/manage-env.sh show $(if $(COMPONENT),$(COMPONENT),)

env-list: ## List available environment profiles
	@./configs/manage-env.sh list

env-validate: ## Validate current environment profile configuration
	@./configs/manage-env.sh validate

env-diff: ## Compare two environment profiles (ENV1=<env1> ENV2=<env2>)
	@if [ -z "$(ENV1)" ] || [ -z "$(ENV2)" ]; then \
		echo "Error: ENV1 and ENV2 are required"; \
		echo "Usage: make env-diff ENV1=local-dev ENV2=remote-dev"; \
		exit 1; \
	fi
	@./configs/manage-env.sh diff $(ENV1) $(ENV2)

env-export: ## Export environment variables (FORMAT=env|yaml|json, default: env)
	@./configs/manage-env.sh export $(if $(FORMAT),$(FORMAT),env)

env-component-status: ## Show status of all components in current environment
	@./configs/manage-env.sh component-status

env-sync: ## Sync environment variables to active profile (updates .env.* files)
	@./configs/manage-env.sh sync

secrets-sync: ## Sync secrets for active environment (MODE=local|remote|production, default: from active env)
	@if [ -z "$(MODE)" ]; then \
		if [ ! -f configs/environments/.current-env ]; then \
			echo "Error: No environment activated. Run 'make env-activate ENVIRONMENT=local-dev' first"; \
			exit 1; \
		fi; \
		CURRENT_ENV=$$(cat configs/environments/.current-env); \
		if [ "$$CURRENT_ENV" = "local-dev" ]; then \
			./scripts/secrets/sync.sh local || echo "Secrets sync not implemented yet"; \
		elif [ "$$CURRENT_ENV" = "remote-dev" ]; then \
			./scripts/secrets/sync.sh remote || echo "Secrets sync not implemented yet"; \
		else \
			./scripts/secrets/sync.sh production || echo "Secrets sync not implemented yet"; \
		fi; \
	else \
		./scripts/secrets/sync.sh $(MODE) || echo "Secrets sync not implemented yet"; \
	fi

