# Dockerfile for building the api-router-service container image
FROM golang:1.24-alpine AS builder

# Install wget for downloading tiktoken vocabularies
RUN apk --no-cache add wget

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

# Pre-download tiktoken vocabularies to avoid runtime network dependency
# tiktoken uses SHA1 hash of the URL as the cache filename
# cl100k_base: sha1("https://openaipublic.blob.core.windows.net/encodings/cl100k_base.tiktoken")
# o200k_base: sha1("https://openaipublic.blob.core.windows.net/encodings/o200k_base.tiktoken")
RUN mkdir -p /tiktoken_cache && \
    wget -q -O /tiktoken_cache/9b5ad71b2ce5302211f9c61530b329a4922fc6a4 \
        https://openaipublic.blob.core.windows.net/encodings/cl100k_base.tiktoken && \
    wget -q -O /tiktoken_cache/fb374d419588a4632f3f557e76b4b70aebbca790 \
        https://openaipublic.blob.core.windows.net/encodings/o200k_base.tiktoken

# Build the router binary with workspace mode
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/router ./cmd/router

# Final stage: minimal runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the router binary
COPY --from=builder /bin/router /app/router

# Copy tiktoken vocabulary cache from builder stage
COPY --from=builder /tiktoken_cache /tiktoken_cache

# Set TIKTOKEN_CACHE_DIR to use pre-downloaded vocabularies
ENV TIKTOKEN_CACHE_DIR=/tiktoken_cache

# Expose the API port (default 8080)
EXPOSE 8080

# Run the router service
CMD ["./router"]
