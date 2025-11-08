# Metrics Retention Policy

- **Storage**: Linode Object Storage bucket `ai-aas-build-metrics`.
- **Format**: JSON documents emitted by `scripts/metrics/collector.go`.
- **Path structure**: `metrics/YYYY/MM/DD/<service>/<run-id>.json`.
- **Retention**: Keep artifacts for at least 30 days; automated lifecycle job deletes older objects.
- **Access**: Restricted to CI service account and platform engineering group.

## Lifecycle Automation

1. Create bucket lifecycle rule via Linode API: [POST /object-storage/buckets/{cluster}/{bucket}/lifecycle](https://techdocs.akamai.com/linode-api/reference/post-object-storage-bucket).
2. Schedule nightly GitHub Actions job to verify cleanup succeeded and notify on drift.
3. For audit requests, export data using `scripts/metrics/upload.sh --download`.

All changes to retention or structure must be communicated via release notes.

