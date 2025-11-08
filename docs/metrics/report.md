# Sample Metrics Report

The CI pipeline generates JSON telemetry files for each run. Example payload:

```json
{
  "run_id": "ci-2025-11-08-001",
  "service": "user-org-service",
  "command": "make check",
  "status": "success",
  "started_at": "2025-11-08T15:04:05Z",
  "finished_at": "2025-11-08T15:06:07Z",
  "duration_seconds": 122.3,
  "commit_sha": "abcdef1234567890",
  "actor": "dev@example.com",
  "environment": "github-actions",
  "collector_version": "1.0.0"
}
```

To analyze metrics:

```bash
aws s3 ls s3://ai-aas-build-metrics/metrics/2025/11/08/
aws s3 cp s3://ai-aas-build-metrics/metrics/2025/11/08/ci-2025-11-08-001.json -
```

Use these reports to track build/test timing trends, detect regressions, and provide evidence for NFR compliance.

