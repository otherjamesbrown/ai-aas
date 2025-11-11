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
git clone git@github.com:otherjamesbrown/ai-aas.git
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
  REDIS_URL=redis://localhost:6379
```
- Starts HTTP server on `:8084` with `/analytics/v1` endpoints and background consumers.
- Logs include ingestion lag, dedupe metrics, and health probes.

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
curl -X POST http://localhost:8084/analytics/v1/orgs/00000000-0000-0000-0000-000000000001/exports \
  -H "Authorization: Bearer $ANALYTICS_TOKEN" \
  -d '{"timeRange":{"start":"2025-11-01T00:00:00Z","end":"2025-11-07T23:59:59Z"},"granularity":"daily"}'
```
- Poll `GET /analytics/v1/orgs/{orgId}/exports/{jobId}` until `status` becomes `succeeded`.
- Download CSV via signed URL and validate checksum.

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
- Synthetic freshness probe defined in `tests/analytics/perf/freshness_benchmark_test.go`; schedule nightly CI run with `make analytics-nightly`.
- Alert thresholds:
  - Freshness lag > 600 seconds (critical)
  - Dedup failure rate > 1% (critical)
  - Export job failure twice within 60 minutes (warning)

## 6. Troubleshooting

| Scenario | Resolution |
|----------|------------|
| Consumer stuck with increasing lag | Check RabbitMQ stream depth, verify `analytics-service` pods have sufficient concurrency (`Values.ingestion.consumers`). |
| Timescale aggregate stale | Run `scripts/analytics/run-hourly.sh --refresh` to force refresh; confirm CronJob logs. |
| Export job fails with permission error | Validate S3 credentials, ensure IAM policy allows `PutObject`/`GetObject` in `analytics/exports/` prefix. |
| Dashboard missing data | Compare `freshness_status` table with Redis cache, clear key `analytics:freshness:*` if stale. |
| Local tests unable to start RabbitMQ | Use `docker compose -f analytics/local-dev/docker-compose.yml up -d` (file generated during bootstrap). |

---

**Next Steps**: With the local environment verified, proceed to `/speckit.tasks` to generate implementation tasks, ensure ArgoCD manifests are reviewed, and update `llms.txt` with analytics documentation URLs.

