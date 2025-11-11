.SHELLFLAGS := -eu -o pipefail -c
SHELL := /bin/bash

ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
SERVICES := $(shell find services -mindepth 1 -maxdepth 1 -type d ! -name '_template' -exec basename {} \;)
SERVICE ?= all
NAME ?=
RUN_ID ?= $(shell date +%s)

SHARED_GO_DIR := $(ROOT_DIR)/shared/go
SHARED_TS_DIR := $(ROOT_DIR)/shared/ts
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
	@echo ">> Testing shared Go libraries"
	@cd $(SHARED_GO_DIR) && go test ./...

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

