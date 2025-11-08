# Metrics Overview

Automation collects build/test telemetry to meet observability requirements.

- Collector: `scripts/metrics/collector.go`
- Upload: `scripts/metrics/upload.sh`
- Storage: Linode Object Storage (`ai-aas-build-metrics`)
- Schema: See `docs/metrics/report.md`
- Retention: See `docs/metrics/policy.md`

To validate metrics locally:

```bash
METRICS_BUCKET=local-metrics ./scripts/metrics/upload.sh --dry-run
```

For troubleshooting, ensure credentials are configured and review CI logs.

