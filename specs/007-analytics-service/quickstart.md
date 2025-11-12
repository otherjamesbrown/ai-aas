# Quickstart: Analytics Service

**Branch**: `007-analytics-service`  
**Date**: 2025-11-11  
**Audience**: Platform engineers, data engineers, and analysts onboarding to the analytics stack

---

## 1. Prerequisites

1. Confirm platform context
   - Kubernetes clusters run on **Akamai Linode (LKE)**; analytics resources deploy via ArgoCD.
   - RabbitMQ brokers are provisioned through `infra/terraform` modules (`rabbitmq_analytics` cluster).
   - PostgreSQL analytics schema resides in the managed Postgres instance defined in spec `003-database-schemas`.
2. Install required tooling (macOS/Linux/WSL2):
   ```bash
   ./scripts/setup/bootstrap.sh --check-only
   ```
   - Verifies Go ≥ 1.21, Docker, `make`, `psql`, `kubectl`, `helm`, `mc` (MinIO/AWS CLI).
3. Authenticate to shared services:
   ```bash
   export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
   linode-cli profile set analytics-dev    # or use SSO per docs/platform/linode-access.md
   ./scripts/platform/linode_helpers.sh ensure-rabbitmq
   ```
4. Grant S3-compatible access for export delivery (MinIO or Linode object storage):
   ```bash
   mc alias set analytics-s3 https://us-east-1.linodeobjects.com ACCESS_KEY SECRET_KEY
   ```

## 2. One-time Setup

```bash
git clone git@github.com:<organization>/ai-aas.git
cd ai-aas
SPECIFY_FEATURE=007-analytics-service make analytics-bootstrap
```

Expected outcome:
- Analytics service module added to `go.work`.
- Database migrations applied to local dev database (`db/migrations/analytics`).
- RabbitMQ stream `analytics.usage.v1` and dead-letter queue created (development only).
- Grafana dashboard JSONs synced under `dashboards/analytics/*.json`.

## 3. Local Development Workflow

### Run the Analytics Service Locally
```bash
cd services/analytics-service
make run \
  RABBITMQ_URL=amqp://guest:guest@localhost:5672 \
  DATABASE_URL=postgres://analytics:analytics@localhost:5432/ai_aas?sslmode=disable \
  REDIS_URL=redis://localhost:6379 \
  S3_ENDPOINT=https://us-east-1.linodeobjects.com \
  S3_ACCESS_KEY=your-access-key \
  S3_SECRET_KEY=your-secret-key \
  S3_BUCKET=analytics-exports
```
- Starts HTTP server on `:8084` with `/analytics/v1` endpoints and background consumers.
- Logs include ingestion lag, dedupe metrics, and health probes.
- RBAC middleware is enabled by default (can be disabled for development via `ENABLE_RBAC=false`).
- Export worker processes pending export jobs every 30 seconds (configurable via `EXPORT_WORKER_INTERVAL`).

### Seed Sample Events
```bash
scripts/analytics/run-hourly.sh --seed
```
- Publishes synthetic events for two orgs and three models.
- Verifies dedupe logic and triggers continuous aggregate refresh.

### Verify Data Quality
```bash
scripts/analytics/verify.sh
```
- Executes SQL reconciliation tests in `analytics/tests`.
- Fails fast if hourly/daily rollups drift >0.5% or freshness exceeds 5 minutes.

### Trigger Export Flow
```bash
# Create export job (requires RBAC headers)
curl -X POST http://localhost:8084/analytics/v1/orgs/00000000-0000-0000-0000-000000000001/exports \
  -H "X-Actor-Subject: user-123" \
  -H "X-Actor-Roles: analytics:exports:create,admin" \
  -H "Content-Type: application/json" \
  -d '{"timeRange":{"start":"2025-11-01T00:00:00Z","end":"2025-11-07T23:59:59Z"},"granularity":"daily"}'

# Response includes jobId - use it to check status
export JOB_ID="<jobId-from-response>"

# Poll job status until succeeded
curl "http://localhost:8084/analytics/v1/orgs/00000000-0000-0000-0000-000000000001/exports/$JOB_ID" \
  -H "X-Actor-Subject: user-123" \
  -H "X-Actor-Roles: analytics:exports:read,admin"

# Download CSV via signed URL (when status is succeeded)
curl "http://localhost:8084/analytics/v1/orgs/00000000-0000-0000-0000-000000000001/exports/$JOB_ID/download" \
  -H "X-Actor-Subject: user-123" \
  -H "X-Actor-Roles: analytics:exports:download,admin" \
  -L -o export.csv
```
- Export jobs are processed asynchronously by the export worker
- CSV files are stored in Linode Object Storage (S3-compatible)
- Signed URLs expire after 24 hours (configurable via `EXPORT_SIGNED_URL_TTL`)

## 4. Kubernetes Deployment

1. Ensure ArgoCD is synced:
   ```bash
   make gitops-sync SERVICE=analytics-service ENV=development
   ```
2. Apply analytics Helm values:
   ```bash
   helm upgrade --install analytics-service charts/analytics-service \
     --namespace analytics \
     -f gitops/clusters/development/analytics-service.values.yaml
   ```
3. Confirm pods and CronJobs:
   ```bash
   kubectl get pods -n analytics
   kubectl get cronjobs -n analytics
   ```
4. Run smoke test:
   ```bash
   tests/analytics/integration/smoke.sh
   ```
   - Ensures ingestion consumer connected, publish test events, verifies rollup presence.

## 5. Observability & Dashboards

- Prometheus metrics available at `/metrics`; add scrape config via `gitops/templates/argocd-values.yaml`.
- Grafana dashboards located in `dashboards/analytics/usage.json` and `dashboards/analytics/reliability.json`.
- Performance benchmarks defined in `services/analytics-service/test/perf/freshness_benchmark_test.go`; run with `go test -bench=. -benchmem`.
- Audit logging captures all authorization decisions (allowed/denied) with actor, action, and outcome.
- Alert thresholds:
  - Freshness lag > 600 seconds (critical)
  - Dedup failure rate > 1% (critical)
  - Export job failure twice within 60 minutes (warning)
  - Error rate > 5% for 5 minutes (critical)
  - Latency P95 > 2s for 10 minutes (warning)
  - Latency P99 > 5s for 5 minutes (critical)

## 6. Troubleshooting

| Scenario | Resolution |
|----------|------------|
| Consumer stuck with increasing lag | Check RabbitMQ stream depth, verify `analytics-service` pods have sufficient concurrency (`Values.ingestion.consumers`). |
| Timescale aggregate stale | Run `scripts/analytics/run-hourly.sh --refresh` to force refresh; confirm CronJob logs. |
| Export job fails with permission error | Validate Linode Object Storage credentials (`S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`), ensure bucket exists and credentials have `PutObject`/`GetObject` permissions. |
| RBAC middleware rejecting requests | Ensure `X-Actor-Subject` and `X-Actor-Roles` headers are set. For development, set `ENABLE_RBAC=false` to disable RBAC. |
| Export job stuck in pending | Check export worker logs, verify S3 credentials are configured, check worker concurrency settings (`EXPORT_WORKER_CONCURRENCY`). |
| Dashboard missing data | Compare `freshness_status` table with Redis cache, clear key `analytics:freshness:*` if stale. |
| Local tests unable to start RabbitMQ | Use `docker compose -f analytics/local-dev/docker-compose.yml up -d` (file generated during bootstrap). |

---

**Next Steps**: With the local environment verified, proceed to `/speckit.tasks` to generate implementation tasks, ensure ArgoCD manifests are reviewed, and update `llms.txt` with analytics documentation URLs.

