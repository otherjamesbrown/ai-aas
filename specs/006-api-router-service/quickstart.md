# Quickstart: API Router Service

## Prerequisites

- Go 1.21.x, GNU Make 4.x, Docker / Docker Compose.  
- Access to shared development Redis cluster (`redis://redis-dev.ai-aas.internal:6379`) or local container.  
- Access to Kafka dev cluster (`PLAINTEXT://kafka-dev.ai-aas.internal:9092`) with credentials configured via shared secrets.  
- `buf` CLI for protobuf generation, `vegeta` for load testing.  
- Configure kubecontext to staging cluster for remote deploy validation.

## Bootstrap

```bash
cd /path/to/your/ai-aas
make services/api-router-service/bootstrap
```

This target installs Go modules, generates protobuf/OpenAPI artifacts, and copies sample configs to `services/api-router-service/configs/`.

## Local Development Loop

1. **Run supporting services**:
   ```bash
   cd services/api-router-service
   docker compose -f dev/docker-compose.yml up redis kafka mock-backends
   ```
2. **Start the router**:
   ```bash
   make run
   ```
   The service listens on `http://localhost:8080` (public) and `https://localhost:8443` (admin mTLS via dev certs).
3. **Send a sample inference request**:
   ```bash
   curl -X POST http://localhost:8080/v1/inference \
     -H 'X-API-Key: dev-key-123' \
     -H 'Content-Type: application/json' \
     -d '{"request_id":"11111111-2222-3333-4444-555555555555","model":"gpt-4o","payload":"Hello"}'
   ```
4. **Inspect telemetry**: visit `http://localhost:9090/dashboards/api-router-service` (Grafana dev) to verify RED metrics; check Kafka topic with `scripts/kafka/tail.sh usage.records.v1`.

## Testing

- **Unit tests**: `make test` (runs `go test ./...`).  
- **Contract tests**: `make contract-test` (validates OpenAPI + protobuf artifacts).  
- **Integration tests**: `make integration-test` (brings up docker-compose harness, validates limiter, routing, usage export).  
- **Load tests**: `make load-test` (runs vegeta scenarios; ensure Redis/Kafka containers running).  
- **Chaos drills**: `make chaos-test` (simulates backend degradation and exporter outages).

## Configuration & Overrides

- Runtime configuration lives in Config Service under namespace `services/api-router`. Use the admin CLI:  
  ```bash
  make admin-shell
  routerctl routing overrides set --model gpt-4o --backend secondary --weight 100
  ```
- Local overrides: edit `configs/router.dev.yaml`; the service watches for changes and reloads automatically.

## Deployment

1. Ensure build pipeline green: `make check` and `make integration-test`.  
2. Build and push image: `make image push`.  
3. Deploy to staging: `make deploy ENV=staging`.  
4. Run smoke tests: `make smoke ENV=staging`.  
5. Promote to production via ArgoCD after staging sign-off.

## Troubleshooting

- **401 / auth failures**: verify API key exists and HMAC signature matches. Check shared auth service logs.  
- **402 / limit exceeded**: inspect Redis limiter keys via `scripts/limiter/debug.sh`; reset via admin CLI if needed.  
- **Telemetry backlog**: `make exporter-status` to view buffer depth; follow runbook to drain or scale Kafka.  
- **Routing anomalies**: `make routing-dump` to print active policies from cache and latest Config Service version.
