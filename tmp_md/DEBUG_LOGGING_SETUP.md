# Setting LOG_LEVEL to Debug and Viewing Logs in Loki

## Step 1: Set LOG_LEVEL=debug

The `local-dev` environment profile already sets `LOG_LEVEL=debug` by default (see `configs/environments/local-dev.yaml:92`).

However, to ensure your services pick it up, you can:

### Option A: Export in your shell
```bash
export LOG_LEVEL=debug
```

### Option B: Use environment profile activation
```bash
# Activate local-dev profile (which sets LOG_LEVEL=debug)
make env-activate ENVIRONMENT=local-dev

# Verify
make env-show | grep LOG_LEVEL
```

## Step 2: Start Services with Debug Logging

```bash
# Start all services (Loki, Promtail, Postgres, Redis, etc.)
docker compose -f .dev/compose/compose.base.yaml -f .dev/compose/compose.local.yaml up -d

# Start your service with LOG_LEVEL=debug
export LOG_LEVEL=debug
cd services/user-org-service
make run  # or manually: go run cmd/admin-api/main.go

# Or if running via Docker Compose, ensure LOG_LEVEL is passed:
docker compose -f .dev/compose/compose.base.yaml -f .dev/compose/compose.local.yaml run --rm \
  -e LOG_LEVEL=debug \
  user-org-service
```

## Step 3: View Logs in Loki

### Option A: Use the helper script (checks for Grafana, falls back to Loki API)
```bash
# View all logs
make logs-view

# View logs for specific service
make logs-service SERVICE=user-org-service
```

### Option B: Query Loki API directly

Since Grafana is not configured in Docker Compose, use Loki's API directly:

```bash
# Query logs from last hour for all services
curl -G -s "http://localhost:3100/loki/api/v1/query_range" \
  --data-urlencode 'query={environment="local-dev"}' \
  --data-urlencode "start=$(date -u -d '1 hour ago' +%s)000000000" \
  --data-urlencode "end=$(date -u +%s)000000000" \
  | jq '.data.result[].stream, .data.result[].values[]'

# Query logs for specific service
curl -G -s "http://localhost:3100/loki/api/v1/query_range" \
  --data-urlencode 'query={service="user-org-service"}' \
  --data-urlencode "start=$(date -u -d '1 hour ago' +%s)000000000" \
  --data-urlencode "end=$(date -u +%s)000000000" \
  | jq

# Query only debug level logs
curl -G -s "http://localhost:3100/loki/api/v1/query_range" \
  --data-urlencode 'query={environment="local-dev"} |= "debug"' \
  --data-urlencode "start=$(date -u -d '1 hour ago' +%s)000000000" \
  --data-urlencode "end=$(date -u +%s)000000000" \
  | jq
```

### Option C: Install logcli (Loki command-line tool)

```bash
# Download logcli (Linux)
wget https://github.com/grafana/loki/releases/download/v2.9.0/logcli-linux-amd64.zip
unzip logcli-linux-amd64.zip
sudo mv logcli-linux-amd64 /usr/local/bin/logcli
chmod +x /usr/local/bin/logcli

# Query logs
logcli query '{service="user-org-service"}' --addr=http://localhost:3100 --limit=100
logcli query '{environment="local-dev"}' --addr=http://localhost:3100 --limit=100

# Follow logs in real-time
logcli query '{service="user-org-service"}' --addr=http://localhost:3100 --tail
```

## Step 4: Add Grafana (Optional - Better UI)

If you want to use Grafana Explore UI instead of API queries:

```yaml
# Add to .dev/compose/compose.base.yaml
  grafana:
    image: grafana/grafana:latest
    container_name: dev-grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_INSTALL_PLUGINS=
    volumes:
      - grafana-data:/var/lib/grafana
    depends_on:
      - loki
    networks:
      - dev-network

volumes:
  grafana-data:
    driver: local
```

Then:
1. Start Grafana: `docker compose -f .dev/compose/compose.base.yaml -f .dev/compose/compose.local.yaml up -d grafana`
2. Open http://localhost:3000 (login: admin/admin)
3. Add Loki data source: http://loki:3100
4. Use Explore: http://localhost:3000/explore

## Quick Reference

```bash
# Set debug logging
export LOG_LEVEL=debug

# Check Loki is running
curl http://localhost:3100/ready

# Check Promtail is running
curl http://localhost:9080/ready

# View logs for user-org-service via script
make logs-service SERVICE=user-org-service

# Direct Docker logs (simpler, but not via Loki)
make logs-tail SERVICE=user-org-service

# Filter for errors only
make logs-error SERVICE=user-org-service
```

