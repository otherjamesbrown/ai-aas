# Dockerfile for building the api-router-service container image
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy the service and shared modules
COPY services/api-router-service ./services/api-router-service
COPY shared ./shared

# Create a minimal go.work file for the Docker build
# The use directive tells Go to use the local modules
# The api-router-service go.mod has a replace for the shared module import path
RUN printf '%s\n' \
    'go 1.24.0' \
    '' \
    'use (' \
    '	./services/api-router-service' \
    '	./shared/go' \
    ')' \
    > go.work

# Download dependencies from the service directory
WORKDIR /build/services/api-router-service
RUN go mod download

# Build the router binary with workspace mode
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/router ./cmd/router

# Final stage: minimal runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the router binary
COPY --from=builder /bin/router /app/router

# Expose the API port (default 8080)
EXPOSE 8080

# Run the router service
CMD ["./router"]
