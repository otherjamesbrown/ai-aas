# Admin CLI Makefile

SERVICE_NAME ?= admin-cli

include ../../templates/service.mk

.PHONY: integration-test
integration-test: build ## Run integration tests (requires services to be running)
	@echo "Running integration tests..."
	@if [ -z "$$(command -v go)" ]; then \
		echo "Go not found - skipping tests"; \
		exit 0; \
	fi
	@$(GO) test -v -timeout 60s ./test/integration/...

.PHONY: test-integration
test-integration: integration-test ## Alias for integration-test
