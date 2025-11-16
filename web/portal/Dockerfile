# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Install pnpm
RUN npm install -g pnpm

# Copy workspace configuration
COPY pnpm-workspace.yaml ./
COPY pnpm-lock.yaml ./

# Copy package files
COPY web/portal/package.json ./web/portal/
COPY shared/ts/package.json ./shared/ts/

# Install dependencies (pnpm workspace will handle the rest)
RUN pnpm install --frozen-lockfile --filter @ai-aas/web-portal...

# Copy source files
COPY web/portal ./web/portal
COPY shared/ts ./shared/ts

# Build the application
# Accept build args for Vite environment variables
ARG VITE_API_BASE_URL
ARG VITE_OAUTH_CLIENT_ID
ARG VITE_OAUTH_ISSUER_URL
ARG VITE_OAUTH_REDIRECT_URI
ARG VITE_FEATURE_FLAGS_API_URL
ARG VITE_OTEL_EXPORTER_OTLP_ENDPOINT
ARG VITE_OTEL_SERVICE_NAME
ARG VITE_OTEL_SERVICE_VERSION

# Set as environment variables for Vite build
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
ENV VITE_OAUTH_CLIENT_ID=${VITE_OAUTH_CLIENT_ID}
ENV VITE_OAUTH_ISSUER_URL=${VITE_OAUTH_ISSUER_URL}
ENV VITE_OAUTH_REDIRECT_URI=${VITE_OAUTH_REDIRECT_URI}
ENV VITE_FEATURE_FLAGS_API_URL=${VITE_FEATURE_FLAGS_API_URL}
ENV VITE_OTEL_EXPORTER_OTLP_ENDPOINT=${VITE_OTEL_EXPORTER_OTLP_ENDPOINT}
ENV VITE_OTEL_SERVICE_NAME=${VITE_OTEL_SERVICE_NAME}
ENV VITE_OTEL_SERVICE_VERSION=${VITE_OTEL_SERVICE_VERSION}

WORKDIR /app/web/portal
RUN pnpm run build

# Production stage
FROM nginx:alpine

# Copy built assets from builder
COPY --from=builder /app/web/portal/dist /usr/share/nginx/html

# Copy nginx configuration
COPY web/portal/deployments/nginx/default.conf /etc/nginx/conf.d/default.conf

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost/healthz || exit 1

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]

