OPA ?= opa
BUNDLE_OUTPUT ?= dist/service-template.tar.gz
REGISTRY ?= registry.ai-aas.dev/policies
BUNDLE_NAME ?= service-template

.PHONY: bundle
bundle: clean ## Build OPA bundle archive locally
	@echo ">> Building OPA bundle for $(BUNDLE_NAME)"
	$(OPA) build -b . -o "$(BUNDLE_OUTPUT)"

.PHONY: push
push: bundle ## Push the OPA bundle to the configured registry (stub)
	@echo ">> (stub) Push $(BUNDLE_OUTPUT) to $(REGISTRY)/$(BUNDLE_NAME)"
	@echo "   Update this target with registry credentials before production usage."

.PHONY: clean
clean: ## Remove generated artifacts
	@rm -rf dist

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  bundle  - Build the OPA policy bundle archive"
	@echo "  push    - Build and push the bundle (stub implementation)"
	@echo "  clean   - Remove build artifacts"

