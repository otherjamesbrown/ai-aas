# Dockerfile for building the user-org-service container image.
# This creates a container with both admin-api and reconciler binaries.
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build both binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/admin-api ./cmd/admin-api
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/reconciler ./cmd/reconciler

# Final stage: minimal runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy both binaries
COPY --from=builder /bin/admin-api /app/admin-api
COPY --from=builder /bin/reconciler /app/reconciler

# No default entrypoint - each container in the deployment specifies its own command/args
# admin-api container uses: args: ["./admin-api"]
# reconciler container uses: args: ["./reconciler"]

