# Build stage
# NOTE: This Dockerfile should be built from the repo root:
#   docker build -t admin-api-service -f services/admin-api-service/Dockerfile .
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy shared library first (for better caching)
COPY shared/go /app/shared/go

# Copy service go mod files
COPY services/admin-api-service/go.mod services/admin-api-service/go.sum /app/services/admin-api-service/

# Update go.mod replace directive to use the copied path
WORKDIR /app/services/admin-api-service
RUN go mod edit -replace github.com/ai-aas/shared-go=/app/shared/go
RUN go mod download

# Copy service source code
COPY services/admin-api-service/ /app/services/admin-api-service/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /admin-api \
    ./cmd/admin-api

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata wget curl

# Install goose for migrations (pre-built binary to avoid Go version issues)
RUN curl -fsSL https://github.com/pressly/goose/releases/download/v3.22.1/goose_linux_x86_64 -o /usr/local/bin/goose && \
    chmod +x /usr/local/bin/goose

# Create non-root user
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /admin-api /app/admin-api

# Copy migrations directory for init container
COPY db/migrations/operational /app/migrations/sql

# Change ownership to appuser
RUN chown -R appuser:appuser /app

USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Run the binary
ENTRYPOINT ["/app/admin-api"]
