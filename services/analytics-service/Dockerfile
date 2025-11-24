# Dockerfile for building the analytics-service container image
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy the entire monorepo structure to maintain relative paths
COPY services/analytics-service ./services/analytics-service
COPY shared ./shared

# Disable Go workspace mode for Docker builds
ENV GOWORK=off

# Download dependencies from the service directory
WORKDIR /build/services/analytics-service
RUN go mod download

# Build the analytics binary
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /bin/analytics ./cmd/analytics-service

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
