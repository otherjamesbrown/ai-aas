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

# Triton gRPC proto generation
TRITON_PROTO_DIR := $(SERVICE_ROOT)/internal/adapter/triton/proto
TRITON_PB_DIR := $(SERVICE_ROOT)/internal/adapter/triton/pb
TRITON_COMMON_URL := https://raw.githubusercontent.com/triton-inference-server/common/main/protobuf

.PHONY: proto-triton
proto-triton: proto-triton-download proto-triton-generate ## Download and generate Triton gRPC stubs

.PHONY: proto-triton-download
proto-triton-download: ## Download Triton proto files from triton-inference-server/common
	@echo "Downloading Triton proto files..."
	@mkdir -p $(TRITON_PROTO_DIR)
	@curl -sL $(TRITON_COMMON_URL)/grpc_service.proto -o $(TRITON_PROTO_DIR)/grpc_service.proto.tmp
	@curl -sL $(TRITON_COMMON_URL)/model_config.proto -o $(TRITON_PROTO_DIR)/model_config.proto.tmp
	@# Add go_package option if not present
	@if ! grep -q 'option go_package' $(TRITON_PROTO_DIR)/grpc_service.proto.tmp; then \
		sed -i 's/^package inference;$$/package inference;\n\noption go_package = "github.com\/otherjamesbrown\/ai-aas\/services\/api-router-service\/internal\/adapter\/triton\/pb";/' $(TRITON_PROTO_DIR)/grpc_service.proto.tmp; \
	fi
	@if ! grep -q 'option go_package' $(TRITON_PROTO_DIR)/model_config.proto.tmp; then \
		sed -i 's/^package inference;$$/package inference;\n\noption go_package = "github.com\/otherjamesbrown\/ai-aas\/services\/api-router-service\/internal\/adapter\/triton\/pb";/' $(TRITON_PROTO_DIR)/model_config.proto.tmp; \
	fi
	@mv $(TRITON_PROTO_DIR)/grpc_service.proto.tmp $(TRITON_PROTO_DIR)/grpc_service.proto
	@mv $(TRITON_PROTO_DIR)/model_config.proto.tmp $(TRITON_PROTO_DIR)/model_config.proto
	@echo "✓ Proto files downloaded"

.PHONY: proto-triton-generate
proto-triton-generate: ## Generate Go stubs from Triton proto files
	@echo "Generating Go stubs from Triton protos..."
	@command -v protoc >/dev/null 2>&1 || { \
		echo "protoc not installed. Install Protocol Buffers compiler."; \
		exit 1; \
	}
	@command -v protoc-gen-go >/dev/null 2>&1 || { \
		echo "protoc-gen-go not installed. Install with:"; \
		echo "  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; \
		exit 1; \
	}
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || { \
		echo "protoc-gen-go-grpc not installed. Install with:"; \
		echo "  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"; \
		exit 1; \
	}
	@mkdir -p $(TRITON_PB_DIR)
	@protoc --go_out=$(TRITON_PB_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(TRITON_PB_DIR) --go-grpc_opt=paths=source_relative \
		-I=$(TRITON_PROTO_DIR) $(TRITON_PROTO_DIR)/grpc_service.proto $(TRITON_PROTO_DIR)/model_config.proto
	@echo "✓ Go stubs generated in $(TRITON_PB_DIR)"

