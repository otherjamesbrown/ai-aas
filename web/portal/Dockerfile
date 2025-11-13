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

