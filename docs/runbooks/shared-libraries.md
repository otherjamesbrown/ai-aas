# Shared Libraries Observability Runbook

Last updated: 2025-11-11

## Purpose

This runbook supports responders when the shared libraries telemetry or health checks degrade. It complements the default dashboards (`dashboards/grafana/shared-libraries.json`) and alert rules (`dashboards/alerts/shared-libraries.yaml`).

## Key Signals

| Metric / Alert | Description | Location |
| -------------- | ----------- | -------- |
| `shared_telemetry_export_failures_total` | Counter of exporter initialization failures (labels: `service_name`, `exporter`). Values `grpc`, `http`, or `degraded` indicate fallback behaviour. | Prometheus |
| `SharedLibrariesExporterFailures` alert | Fires when ≥5 exporter failures occur within 10 minutes. | Alertmanager |
| `SharedLibrariesHighErrorRate` alert | 5xx ratio >2% over 10 minutes. | Alertmanager |
| `SharedLibrariesLatencyDegradation` alert | HTTP p95 latency >750 ms over 15 minutes. | Alertmanager |
| `SharedLibrariesDatabaseProbeFailure` alert | Database health check success <80% over 10 minutes. | Alertmanager |

Dashboards expose these metrics along with request rate/latency panels. Use the service filter to scope to a single consumer.

## Detection

Alerts route via the `shared-libraries` notification channel. Record the alert payload (service label, environment, timestamps) in the incident timeline.

## Initial Triage

1. **Check Grafana dashboard**  
   Navigate to *Shared Libraries Overview* and confirm:
   - Whether failures correlate with request spikes or deploys.
   - If `shared_telemetry_export_failures_total` shows a spike for a specific exporter.

2. **Inspect logs/traces**  
   Shared middleware logs warnings when authorization fails or exporter fallback occurs. Filter by `service.name` and `request_id` from the alert payload.

3. **Validate configs**  
   Ensure `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, and database DSNs are correct for the affected service. For TypeScript services, re-run:
   ```bash
   npm run build --prefix samples/service-template/ts
   npm test --prefix samples/service-template/ts
   ```

4. **Manual health check**  
   Call the service health endpoint (shared middleware exposes `/healthz` via `dataaccess.Handler`):
   ```bash
   curl -fsS https://<service-host>/healthz
   ```
   A 503 indicates a registered probe is failing (view `database` status in the JSON response).

## Remediation

### Telemetry Exporter Failures

1. **Collector reachability:** verify the OTLP collector is up (`telnet <host> 4317/4318` or `grpcurl`).  
2. **Fallback check:** if `degraded` exporter counts increase, the library is using a no-op tracer provider. Restore connectivity, then restart the service to re-establish the exporter.  
3. **Credentials:** confirm any auth headers (`OTEL_EXPORTER_OTLP_HEADERS`) are valid.

### High Error Rate / Latency

1. Review recent deploys; roll back if necessary.  
2. Confirm downstream dependencies (database, external APIs) are operational.  
3. Use the audit/authorization logs to rule out new policy changes.

### Database Probe Failures

1. Validate database credentials or rotation events.  
2. Check connection limits (`DATABASE_MAX_OPEN_CONNS`, `DATABASE_MAX_IDLE_CONNS`).  
3. If using connection pooling, restart the service after restoring DB access to drop stale connections.

## Verification

1. Re-run smoke tests:
   ```bash
   ./scripts/shared/upgrade-verify.sh
   samples/service-template/scripts/smoke.sh
   ```
2. Ensure alerts auto-resolve and dashboard error/latency panels return to baseline.
3. Capture metrics snapshots in the incident record.

## Preventative Actions

- Keep OTLP collector endpoints highly available.  
- Monitor `shared_telemetry_export_failures_total` trends to catch degradation before alerts fire.  
- Integrate this runbook link into PagerDuty / incident tooling.  
- Schedule regular runs of the upgrade verification script in CI or as part of release checklists.

## Version Skew Guidance

- **Track deployed versions**: Record the Go module tag (`shared-go@vX.Y.Z`) and npm package version (`@ai-aas/shared@X.Y.Z`) per environment. Mismatches can cause telemetry schema drift.
- **Verify consumer manifests**: For Go, run `go list -m all | grep shared-go`; for TypeScript, check `npm ls @ai-aas/shared`. Ensure they align with the latest release notes.
- **Upgrade procedure**:
  1. Run `./scripts/shared/upgrade-verify.sh`.
  2. Bump versions in consumer services (`go.mod`, `package.json`).
  3. Re-run service-specific smoke tests and benchmarks.
  4. Monitor exporter failure metrics for 30 minutes post-deploy.
- **Rollback plan**: If regressions appear, revert to prior tags and rerun benchmarks to confirm recovery. Keep previous release artifacts accessible via GitHub Releases.

## Pilot Lessons & Follow-ups

- Capture integration notes from `billing-api` and `content-ingest` pilots in `docs/adoption/pilot-results.md` once rollouts complete.
- Document policy bundle or config adjustments required during pilots; include mitigation steps here for future adopters.
- Update this section after each pilot retrospective with concrete remediation steps or tooling improvements.


