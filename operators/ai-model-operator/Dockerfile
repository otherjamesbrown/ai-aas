# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy shared library first (for better caching)
COPY shared/go /app/shared/go

# Copy operator go mod files
COPY operators/ai-model-operator/go.mod operators/ai-model-operator/go.sum /app/operators/ai-model-operator/

# Update go.mod replace directive to use the copied path
WORKDIR /app/operators/ai-model-operator
RUN go mod edit -replace github.com/ai-aas/shared-go=/app/shared/go
RUN go mod download

# Copy operator source code
COPY operators/ai-model-operator/ /app/operators/ai-model-operator/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /ai-model-operator \
    ./main.go

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /ai-model-operator /app/ai-model-operator

# Change ownership to appuser
RUN chown -R appuser:appuser /app

USER appuser

ENTRYPOINT ["/app/ai-model-operator"]
