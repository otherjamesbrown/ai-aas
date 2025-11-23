# Dockerfile for building the user-org-service container image.
# This creates a container with both admin-api and reconciler binaries.
# Build context must be repository root to access shared/go module
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy shared module first (dependency)
COPY shared/go /shared/go

# Copy service files
COPY services/user-org-service/go.mod services/user-org-service/go.sum ./
RUN go mod download

# Copy service source code
COPY services/user-org-service .

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

# Install goose for migrations
RUN apk --no-cache add go
WORKDIR /tmp
RUN go install github.com/pressly/goose/v3/cmd/goose@latest
RUN cp /root/go/bin/goose /usr/local/bin/goose

# Copy migrations directory for init container
COPY services/user-org-service/migrations /app/migrations

WORKDIR /app

# No default entrypoint - each container in the deployment specifies its own command/args
# admin-api container uses: args: ["./admin-api"]
# reconciler container uses: args: ["./reconciler"]

