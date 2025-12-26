# Model Downloader Service

A dedicated Go-based container image for efficient HuggingFace → S3 transfers.

## Overview

This service provides a reliable way to download models from HuggingFace Hub and upload them to S3-compatible object storage. It wraps the existing `shared/go/modelcache` library that has proven to work reliably with Linode Object Storage and other S3-compatible services.

## Why Go Instead of Python?

1. **Proven code** - Reuses `shared/go/modelcache/pull` which already works with Linode Object Storage
2. **Single binary** - No runtime `pip install`, faster startup, more reliable
3. **Consistent tooling** - Same language as rest of platform
4. **Testable** - Can unit test properly (not embedded strings)

## Environment Variables

### Required

| Variable | Description | Example |
|----------|-------------|---------|
| `MODEL_ID` | HuggingFace model ID | `meta-llama/Llama-2-7b-hf` |
| `S3_BUCKET` | Target S3 bucket | `ai-aas-models` |
| `S3_KEY` | S3 key prefix | `models/llama-7b/main` |

### Optional

| Variable | Description | Default |
|----------|-------------|---------|
| `HF_TOKEN` | HuggingFace API token | - |
| `S3_ENDPOINT` | S3-compatible endpoint URL | - |
| `AWS_ENDPOINT_URL_S3` | Alternative S3 endpoint (fallback) | - |
| `AWS_ACCESS_KEY_ID` | S3 access key | - |
| `AWS_SECRET_ACCESS_KEY` | S3 secret key | - |
| `AWS_REGION` | S3 region | `us-east-1` |
| `S3_UPLOAD_CONCURRENCY` | Concurrent part uploads (use 1 for Linode) | `1` |
| `S3_MULTIPART_THRESHOLD` | File size threshold for multipart (bytes) | `104857600` (100MB) |
| `S3_PART_SIZE` | Multipart part size (bytes) | `52428800` (50MB) |

## Usage

### Docker

```bash
docker run --rm \
  -e MODEL_ID=meta-llama/Llama-2-7b-hf \
  -e S3_BUCKET=ai-aas-models \
  -e S3_KEY=models/llama-7b/main \
  -e S3_ENDPOINT=https://us-southeast-1.linodeobjects.com \
  -e AWS_ACCESS_KEY_ID=<access-key> \
  -e AWS_SECRET_ACCESS_KEY=<secret-key> \
  -e HF_TOKEN=<huggingface-token> \
  ghcr.io/otherjamesbrown/ai-aas/model-downloader:latest
```

### Kubernetes Job

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: download-llama-7b
spec:
  template:
    spec:
      containers:
      - name: downloader
        image: ghcr.io/otherjamesbrown/ai-aas/model-downloader:latest
        env:
        - name: MODEL_ID
          value: meta-llama/Llama-2-7b-hf
        - name: S3_BUCKET
          value: ai-aas-models
        - name: S3_KEY
          value: models/llama-7b/main
        - name: S3_ENDPOINT
          valueFrom:
            secretKeyRef:
              name: s3-credentials
              key: S3_ENDPOINT
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: s3-credentials
              key: AWS_ACCESS_KEY_ID
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: s3-credentials
              key: AWS_SECRET_ACCESS_KEY
        - name: HF_TOKEN
          valueFrom:
            secretKeyRef:
              name: hf-credentials
              key: token
              optional: true
        - name: S3_UPLOAD_CONCURRENCY
          value: "1"
      restartPolicy: OnFailure
```

## Configuration for Different S3 Providers

### Linode Object Storage

Use sequential uploads (concurrency = 1) for reliability:

```yaml
- name: S3_UPLOAD_CONCURRENCY
  value: "1"
- name: S3_MULTIPART_THRESHOLD
  value: "104857600"  # 100MB
- name: S3_PART_SIZE
  value: "52428800"   # 50MB
```

### AWS S3

Can use higher concurrency for better performance:

```yaml
- name: S3_UPLOAD_CONCURRENCY
  value: "4"
- name: S3_MULTIPART_THRESHOLD
  value: "8388608"    # 8MB
- name: S3_PART_SIZE
  value: "8388608"    # 8MB
```

## Building

```bash
# Build binary
go build -o /tmp/model-downloader ./services/model-downloader/cmd

# Build Docker image
docker build -t model-downloader:local -f services/model-downloader/Dockerfile .
```

## Development

```bash
# Run tests
cd services/model-downloader
go test ./...

# Run locally (requires env vars)
export MODEL_ID=meta-llama/Llama-2-7b-hf
export S3_BUCKET=ai-aas-models
export S3_KEY=models/llama-7b/main
export S3_ENDPOINT=https://us-southeast-1.linodeobjects.com
export AWS_ACCESS_KEY_ID=<access-key>
export AWS_SECRET_ACCESS_KEY=<secret-key>
go run ./services/model-downloader/cmd/main.go
```

## Integration with AI Model Operator

The AI Model Operator uses this image in Kubernetes Jobs to download models from HuggingFace Hub. The image is configurable via Helm values:

```yaml
# operators/ai-model-operator/helm-charts/ai-model-operator/values.yaml
downloaderImage: "ghcr.io/otherjamesbrown/ai-aas/model-downloader:latest"
```

See the operator documentation for more details.

## Architecture

This service wraps the existing `shared/go/modelcache` library:

- `shared/go/modelcache/pull` - Orchestrates download and upload
- `shared/go/modelcache/storage/s3.go` - S3 client with multipart upload
- `shared/go/modelcache/huggingface` - HuggingFace Hub client

The S3 client is configured for S3-compatible services:
- Path-style URLs (required for Linode)
- Disabled AWS SDK checksum calculation (for compatibility)
- Sequential part uploads by default (for reliability)

## Troubleshooting

### Connection drops during upload

If you experience connection drops with Linode or other S3-compatible services:

1. Reduce `S3_UPLOAD_CONCURRENCY` to `1` (sequential uploads)
2. Increase `S3_PART_SIZE` to reduce the number of parts
3. Check network stability between the downloader and S3 endpoint

### Authentication failures

Ensure the following:

1. `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` are correct
2. `S3_ENDPOINT` matches your S3 provider's endpoint
3. The bucket exists and credentials have write permissions

### HuggingFace rate limiting

If you hit HuggingFace rate limits:

1. Provide a `HF_TOKEN` for higher rate limits
2. Reduce download concurrency (controlled by pull service, not this binary)

## Related Issues

- `ai-aas-zfs` - Phase 5: Create dedicated model-downloader container image
- `ai-aas-n23` - Fix AI Model Operator: S3 download flow and vLLM model loading
