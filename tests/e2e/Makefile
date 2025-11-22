# Makefile for End-to-End Tests
#
# Targets:
#   test-local    - Run tests against local environment
#   test-dev      - Run tests against development environment
#   test-ci       - Run tests in CI environment
#   test-parallel - Run tests in parallel mode
#   clean         - Clean test artifacts
#   help          - Show this help message

.PHONY: test-local test-dev test-ci test-parallel clean help

# Default environment
TEST_ENV ?= local

# Test execution
test-local:
	@echo "Running e2e tests against local environment..."
	TEST_ENV=local go test -v ./suites/... -timeout 30m

test-dev:
	@echo "Running e2e tests against development environment..."
	@echo "Note: This requires kubectl access to the development cluster"
	@echo "For automatic port-forwarding, use: ./scripts/test-dev.sh"
	@echo "For internet access with IP, use: make test-dev-ip"
	@echo ""
	@if [ -z "$$ADMIN_API_KEY" ]; then \
		echo "Warning: ADMIN_API_KEY not set. Some tests may fail."; \
		echo "Set ADMIN_API_KEY environment variable before running tests."; \
		echo ""; \
	fi
	TEST_ENV=development go test -v ./suites/... -timeout 30m

test-dev-ip:
	@echo "Running e2e tests against development environment using IP address..."
	@echo "Using IP: 172.232.58.222 (no /etc/hosts modification needed)"
	@echo "Note: Using HTTPS as ingress redirects HTTP to HTTPS"
	@echo ""
	@if [ -f .admin-key.env ]; then \
		echo "Loading admin key from .admin-key.env..."; \
		. .admin-key.env; \
	fi
	@if [ -z "$$ADMIN_API_KEY" ]; then \
		echo "Warning: ADMIN_API_KEY not set."; \
		echo "Run './scripts/setup-test-env.sh' to bootstrap admin key, or:"; \
		echo "  export ADMIN_API_KEY=your-admin-api-key"; \
		echo ""; \
	fi
	@TEST_ENV=development \
	 USER_ORG_SERVICE_URL=https://172.232.58.222 \
	 API_ROUTER_SERVICE_URL=https://172.232.58.222 \
	 ANALYTICS_SERVICE_URL=https://172.232.58.222 \
	 go test -v ./suites/... -timeout 30m

setup:
	@echo "Setting up e2e test environment..."
	@./scripts/setup-test-env.sh

test-dev-remote:
	@echo "Running e2e tests against development environment with port-forwarding..."
	@./scripts/test-dev.sh

test-dev-internet:
	@echo "Running e2e tests against development environment via public internet..."
	@echo "Note: This requires ingress to be enabled and services to be publicly accessible"
	@./scripts/test-dev-internet.sh

test-ci:
	@echo "Running e2e tests in CI environment..."
	TEST_ENV=ci go test -v ./suites/... -timeout 30m -json > test-results.json

test-parallel:
	@echo "Running e2e tests in parallel mode..."
	TEST_ENV=$(TEST_ENV) PARALLEL_WORKERS=4 go test -v ./suites/... -timeout 30m -parallel 4

# Cleanup
clean:
	@echo "Cleaning test artifacts..."
	rm -rf artifacts/
	rm -f test-results.json
	rm -f test-results.xml
	rm -f *.log

# Help
help:
	@echo "End-to-End Test Harness"
	@echo ""
	@echo "Targets:"
	@echo "  test-local      - Run tests against local environment"
	@echo "  test-dev        - Run tests against development environment"
	@echo "  test-ci         - Run tests in CI environment"
	@echo "  test-parallel   - Run tests in parallel mode"
	@echo "  clean           - Clean test artifacts"
	@echo "  help            - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  TEST_ENV        - Test environment (local, development, ci)"
	@echo "  PARALLEL_WORKERS - Number of parallel workers (default: 1)"

