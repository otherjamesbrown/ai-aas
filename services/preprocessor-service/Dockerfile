# Preprocessor Service Dockerfile
# Multi-stage build for minimal runtime image

# =============================================================================
# Stage 1: Builder - Install dependencies and generate proto code
# =============================================================================
FROM python:3.11-slim AS builder

WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first for better caching
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy proto files and generate Python code
COPY proto/ ./proto/
RUN mkdir -p ./src/preprocessor && \
    python -m grpc_tools.protoc \
        -I./proto \
        --python_out=./src/preprocessor \
        --grpc_python_out=./src/preprocessor \
        ./proto/preprocessor.proto

# Fix imports in generated files (grpc_tools generates relative imports incorrectly)
RUN sed -i 's/import preprocessor_pb2/from . import preprocessor_pb2/' \
    ./src/preprocessor/preprocessor_pb2_grpc.py

# Pre-download tokenizers (configs only, not model weights)
# HF_TOKEN is required for gated models like Llama-3
ARG HF_TOKEN
ENV HF_HOME=/app/hf_cache
ENV HF_TOKEN=${HF_TOKEN}

# Download tokenizers for MVP models
# - meta-llama/Llama-3.1-8B-Instruct: Llama-3 chat format
RUN if [ -n "$HF_TOKEN" ]; then \
      python -c "\
from transformers import AutoTokenizer; \
print('Downloading Llama-3.1-8B-Instruct tokenizer...'); \
AutoTokenizer.from_pretrained('meta-llama/Llama-3.1-8B-Instruct', trust_remote_code=True); \
print('Llama tokenizer downloaded.'); \
"; \
    else \
      echo 'HF_TOKEN not provided, skipping tokenizer pre-download'; \
      mkdir -p /app/hf_cache; \
    fi

# Copy source code
COPY src/ ./src/

# =============================================================================
# Stage 2: Runtime - Minimal image for production
# =============================================================================
FROM python:3.11-slim

WORKDIR /app

# Create non-root user for security
RUN groupadd -r preprocessor && useradd -r -g preprocessor preprocessor

# Copy installed packages from builder
COPY --from=builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

# Copy HuggingFace cache directory (may be empty, tokenizers download at runtime)
COPY --from=builder --chown=preprocessor:preprocessor /app/hf_cache /app/hf_cache

# Copy application source
COPY --from=builder --chown=preprocessor:preprocessor /app/src ./src

# Set environment variables
ENV HF_HOME=/app/hf_cache
ENV TRANSFORMERS_OFFLINE=0
ENV PYTHONPATH=/app/src
ENV PYTHONUNBUFFERED=1

# gRPC port
EXPOSE 50051

# Metrics port (Prometheus)
EXPOSE 8000

# Switch to non-root user
USER preprocessor

# Health check using grpc_health_probe pattern
# Note: In Kubernetes, use grpc liveness/readiness probes instead
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD python -c "\
import grpc; \
from grpc_health.v1 import health_pb2, health_pb2_grpc; \
channel = grpc.insecure_channel('localhost:50051'); \
stub = health_pb2_grpc.HealthStub(channel); \
response = stub.Check(health_pb2.HealthCheckRequest(service='preprocessor.PreprocessorService')); \
exit(0 if response.status == health_pb2.HealthCheckResponse.SERVING else 1)"

# Run the gRPC server
CMD ["python", "-m", "preprocessor.main"]
