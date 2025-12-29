.PHONY: all build run test lint proto clean docker-build docker-run

# Variables
SERVICE_NAME := preprocessor-service
DOCKER_IMAGE := ghcr.io/otherjamesbrown/$(SERVICE_NAME)
DOCKER_TAG := latest
GRPC_PORT := 50051

all: proto build

# Generate protobuf code
proto:
	python -m grpc_tools.protoc \
		-I./proto \
		--python_out=./src/preprocessor \
		--grpc_python_out=./src/preprocessor \
		./proto/preprocessor.proto
	# Fix imports in generated grpc file
	sed -i 's/import preprocessor_pb2/from . import preprocessor_pb2/' \
		./src/preprocessor/preprocessor_pb2_grpc.py

# Install dependencies
build:
	pip install -r requirements.txt

# Run locally
run:
	PYTHONPATH=./src python -m preprocessor.main

# Run with console logging (for development)
run-dev:
	LOG_FORMAT=console LOG_LEVEL=DEBUG PYTHONPATH=./src python -m preprocessor.main

# Run tests (uses venv if available, falls back to system pytest)
test:
	@if [ -f .venv/bin/pytest ]; then \
		PYTHONPATH=./src .venv/bin/pytest tests/ -v; \
	else \
		PYTHONPATH=./src pytest tests/ -v; \
	fi

# Lint code
lint:
	ruff check src/
	mypy src/

# Format code
format:
	ruff format src/

# Clean generated files
clean:
	rm -f src/preprocessor/preprocessor_pb2.py
	rm -f src/preprocessor/preprocessor_pb2_grpc.py
	rm -rf __pycache__ .pytest_cache .mypy_cache
	find . -type d -name "__pycache__" -exec rm -rf {} +

# Docker build
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Docker run
docker-run:
	docker run -p $(GRPC_PORT):$(GRPC_PORT) $(DOCKER_IMAGE):$(DOCKER_TAG)

# Test with grpcurl
test-health:
	grpcurl -plaintext localhost:$(GRPC_PORT) preprocessor.PreprocessorService/HealthCheck

test-preprocess:
	grpcurl -plaintext -d '{ \
		"model_id": "meta-llama/Llama-3.1-8B-Instruct", \
		"engine_type": "vllm", \
		"messages": [{"role": "user", "content": "Hello!"}] \
	}' localhost:$(GRPC_PORT) preprocessor.PreprocessorService/Preprocess

# Generate Go proto code (for api-router-service integration)
proto-go:
	protoc \
		--go_out=../api-router-service/internal/adapter/preprocessor/pb \
		--go_opt=paths=source_relative \
		--go-grpc_out=../api-router-service/internal/adapter/preprocessor/pb \
		--go-grpc_opt=paths=source_relative \
		-I./proto \
		./proto/preprocessor.proto
