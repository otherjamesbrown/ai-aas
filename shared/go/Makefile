.SHELLFLAGS := -eu -o pipefail -c
SHELL := /bin/bash

COVERAGE_TARGET ?= 80

.PHONY: test
test: ## Run shared Go tests with coverage gating
	@echo ">> Running shared-go tests with coverage target $(COVERAGE_TARGET)%"
	@go test ./... -coverprofile=coverage.out -covermode=atomic
	@go tool cover -func=coverage.out | awk -v target=$(COVERAGE_TARGET) 'BEGIN { status=0 } /^total:/ { gsub("%","",$$3); if ($$3+0 < target) { printf "total coverage %.2f%% below target %.2f%%\n", $$3, target; status=1 } } END { exit status }'
	@rm -f coverage.out

