# Service Configuration Template for Local/Remote Development
# Copy this file to .env.local or .env.linode and customize for your service.
# This template maps values from the secrets bundle and dev stack connection strings.

# Service Configuration
SERVICE_NAME=${SERVICE_NAME:-my-service}
SERVICE_ADDRESS=${SERVICE_ADDRESS:-:8080}
ENVIRONMENT=${ENVIRONMENT:-development}

# Database Connection (PostgreSQL)
# For local: Uses ports from .specify/local/ports.yaml
# For remote: Uses workspace private IP or hostname
DATABASE_DSN=${DATABASE_DSN:-postgres://postgres:postgres@localhost:${POSTGRES_PORT:-5432}/ai_aas?sslmode=disable}
DATABASE_MAX_IDLE_CONNS=${DATABASE_MAX_IDLE_CONNS:-2}
DATABASE_MAX_OPEN_CONNS=${DATABASE_MAX_OPEN_CONNS:-10}
DATABASE_CONN_MAX_LIFETIME=${DATABASE_CONN_MAX_LIFETIME:-5m}

# Redis Connection
# For local: Uses port from .specify/local/ports.yaml
# For remote: Uses workspace private IP or hostname
REDIS_ADDR=${REDIS_ADDR:-localhost:${REDIS_PORT:-6379}}
REDIS_PASSWORD=${REDIS_PASSWORD:-}
REDIS_DB=${REDIS_DB:-0}

# NATS Connection
# For local: Uses ports from .specify/local/ports.yaml
# For remote: Uses workspace private IP or hostname
NATS_URL=${NATS_URL:-nats://localhost:${NATS_CLIENT_PORT:-4222}}
NATS_HTTP_ADDR=${NATS_HTTP_ADDR:-localhost:${NATS_HTTP_PORT:-8222}}

# MinIO (S3-compatible) Connection
# For local: Uses ports from .specify/local/ports.yaml
# For remote: Uses workspace private IP or hostname
MINIO_ENDPOINT=${MINIO_ENDPOINT:-http://localhost:${MINIO_API_PORT:-9000}}
MINIO_ACCESS_KEY=${MINIO_ACCESS_KEY:-minioadmin}
MINIO_SECRET_KEY=${MINIO_SECRET_KEY:-minioadmin}
MINIO_USE_SSL=${MINIO_USE_SSL:-false}
MINIO_BUCKET=${MINIO_BUCKET:-ai-aas-artifacts}

# Mock Inference Service
# For local: Uses port from .specify/local/ports.yaml
# For remote: Uses workspace private IP or hostname
MOCK_INFERENCE_ENDPOINT=${MOCK_INFERENCE_ENDPOINT:-http://localhost:${MOCK_INFERENCE_PORT:-8000}}

# Observability (OpenTelemetry)
OTEL_EXPORTER_OTLP_ENDPOINT=${OTEL_EXPORTER_OTLP_ENDPOINT:-localhost:4317}
OTEL_EXPORTER_OTLP_PROTOCOL=${OTEL_EXPORTER_OTLP_PROTOCOL:-grpc}
OTEL_EXPORTER_OTLP_HEADERS=${OTEL_EXPORTER_OTLP_HEADERS:-}
OTEL_EXPORTER_OTLP_INSECURE=${OTEL_EXPORTER_OTLP_INSECURE:-true}

# Service-Specific Configuration
# Add your service-specific environment variables here
# Examples:
# API_KEY=${API_KEY:-}
# WEBHOOK_URL=${WEBHOOK_URL:-}
# LOG_LEVEL=${LOG_LEVEL:-info}

# Mode Selection
# Set DEV_MODE=local for local stack or DEV_MODE=remote for remote workspace
DEV_MODE=${DEV_MODE:-local}

# Connection String Helpers (derived from above)
# These are convenience variables computed from the base config
POSTGRES_HOST=${POSTGRES_HOST:-localhost}
POSTGRES_PORT=${POSTGRES_PORT:-5432}
POSTGRES_USER=${POSTGRES_USER:-postgres}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-postgres}
POSTGRES_DB=${POSTGRES_DB:-ai_aas}

# Example service connection string construction:
# DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"

