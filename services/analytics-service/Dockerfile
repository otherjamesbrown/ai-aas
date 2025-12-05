# Dockerfile for building the analytics-service container image
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy the service and shared modules
COPY services/analytics-service ./services/analytics-service
COPY shared ./shared

# Create a minimal go.work file for the Docker build
RUN printf '%s\n' \
    'go 1.24.0' \
    '' \
    'use (' \
    '	./services/analytics-service' \
    '	./shared/go' \
    ')' \
    '' \
    'replace github.com/ai-aas/shared-go v0.0.0 => ./shared/go' \
    > go.work

# Download dependencies from the service directory
WORKDIR /build/services/analytics-service
RUN go mod download

# Build the analytics binary with workspace mode
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/analytics ./cmd/analytics-service

# Final stage: minimal runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the analytics binary
COPY --from=builder /bin/analytics /app/analytics

# Expose the API port (default 8084)
EXPOSE 8084

# Run the analytics service
CMD ["./analytics"]
