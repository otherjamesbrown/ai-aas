# Dockerfile for building the api-router-service container image
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy the entire monorepo structure to maintain relative paths
COPY services/api-router-service ./services/api-router-service
COPY shared ./shared

# Disable Go workspace mode for Docker builds
ENV GOWORK=off

# Download dependencies from the service directory
WORKDIR /build/services/api-router-service
RUN go mod download

# Build the router binary
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /bin/router ./cmd/router

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
