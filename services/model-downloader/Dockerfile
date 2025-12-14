# Build stage
# NOTE: This Dockerfile should be built from the repo root:
#   docker build -t model-downloader -f services/model-downloader/Dockerfile .
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy shared library first (for better caching)
COPY shared/go /app/shared/go

# Copy service go mod files
COPY services/model-downloader/go.mod services/model-downloader/go.sum /app/services/model-downloader/

# Update go.mod replace directive to use the copied path
WORKDIR /app/services/model-downloader
RUN go mod edit -replace github.com/otherjamesbrown/ai-aas/shared/go=/app/shared/go
RUN go mod download

# Copy service source code
COPY services/model-downloader/ /app/services/model-downloader/

# Build the binary
# Use TARGETARCH from buildx for multi-arch builds
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build \
    -ldflags="-w -s" \
    -o /model-downloader \
    ./cmd/model-downloader

# Final stage
FROM alpine:3.19

# Install ca-certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

# Copy the binary from builder
COPY --from=builder /model-downloader /model-downloader

# Run as non-root user
RUN addgroup -g 1000 downloader && \
    adduser -D -u 1000 -G downloader downloader

USER downloader

ENTRYPOINT ["/model-downloader"]
