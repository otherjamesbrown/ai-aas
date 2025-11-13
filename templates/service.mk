# Shared service automation template. Included by services/<name>/Makefile.

SERVICE_MAKEFILE := $(lastword $(filter %/Makefile Makefile,$(MAKEFILE_LIST)))
SERVICE_ROOT := $(abspath $(dir $(SERVICE_MAKEFILE)))
PROJECT_ROOT := $(abspath $(SERVICE_ROOT)/../..)

include $(PROJECT_ROOT)/configs/tool-versions.mk

export GOTOOLCHAIN := $(GO_TOOLCHAIN)

SERVICE_NAME ?= $(notdir $(patsubst %/,%,$(SERVICE_ROOT)))
SERVICE_BIN_DIR ?= $(SERVICE_ROOT)/bin

GO ?= go
GOFMT ?= gofmt
GOLANGCI_LINT ?= golangci-lint
GOSEC ?= gosec

GOLANGCI_CONFIG ?= $(PROJECT_ROOT)/configs/golangci.yml
GOSEC_CONFIG ?= $(PROJECT_ROOT)/configs/gosec.toml

SERVICE_BUILD_FLAGS ?=
SERVICE_TEST_FLAGS ?=
SERVICE_PRE_BUILD ?= :
SERVICE_POST_BUILD ?= :
SERVICE_PRE_TEST ?= :
SERVICE_POST_TEST ?= :

SERVICE_CMD_DIR := $(SERVICE_ROOT)/cmd/$(SERVICE_NAME)
ifeq ($(wildcard $(SERVICE_CMD_DIR)),)
	SERVICE_MAIN := .
else
	SERVICE_MAIN := ./cmd/$(SERVICE_NAME)
endif

GO_MODULE_PRESENT := $(wildcard $(SERVICE_ROOT)/go.mod)

.PHONY: _ensure-module
_ensure-module:
	@if [ -z "$(GO_MODULE_PRESENT)" ]; then \
		echo "go.mod not found in $(SERVICE_ROOT). Initialize Go module first." >&2; \
		exit 1; \
	fi

.PHONY: build
build: _ensure-module ## Build service binary
	@mkdir -p "$(SERVICE_BIN_DIR)"
	@echo "Building $(SERVICE_NAME)"
	@if [ "$(SERVICE_MAIN)" = "." ]; then \
		$(SERVICE_PRE_BUILD); \
		$(GO) build $(SERVICE_BUILD_FLAGS) ./...; \
		$(SERVICE_POST_BUILD); \
	else \
		$(SERVICE_PRE_BUILD); \
		$(GO) build -trimpath $(SERVICE_BUILD_FLAGS) -o "$(SERVICE_BIN_DIR)/$(SERVICE_NAME)" "$(SERVICE_MAIN)"; \
		$(SERVICE_POST_BUILD); \
	fi

.PHONY: test
test: _ensure-module ## Run Go tests (excludes integration tests requiring external dependencies)
	@$(SERVICE_PRE_TEST)
	@$(GO) test $(SERVICE_TEST_FLAGS) -short ./...
	@$(SERVICE_POST_TEST)

.PHONY: fmt
fmt: ## Format Go source files
	@GO_FILES="$$(find . -name '*.go' -not -path './vendor/*')"; \
	if [ -n "$$GO_FILES" ]; then \
		$(GOFMT) -w $$GO_FILES; \
	else \
		echo "No Go files to format."; \
	fi

.PHONY: lint
lint: _ensure-module ## Run golangci-lint using shared config
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || { echo "golangci-lint not installed"; exit 1; }
	@$(GOLANGCI_LINT) run ./... --config "$(GOLANGCI_CONFIG)" --exclude '(typecheck)'

.PHONY: security
security: _ensure-module ## Run gosec security scan
	@command -v $(GOSEC) >/dev/null 2>&1 || { echo "gosec not installed"; exit 1; }
	@$(GOSEC) ./...

.PHONY: check
check: ## Run fmt, lint, security, and tests
	@$(MAKE) --no-print-directory fmt
	@$(MAKE) --no-print-directory lint
	@$(MAKE) --no-print-directory security
	@$(MAKE) --no-print-directory test

.PHONY: clean
clean: ## Remove build artifacts and caches
	@rm -rf "$(SERVICE_BIN_DIR)" ./coverage ./dist
	@find . -name '*.out' -delete

