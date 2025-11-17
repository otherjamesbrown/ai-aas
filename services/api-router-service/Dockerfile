# Dockerfile for building the api-router-service container image
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./

# Disable Go workspace mode for Docker builds
ENV GOWORK=off

RUN go mod download

# Copy source code
COPY . .

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
