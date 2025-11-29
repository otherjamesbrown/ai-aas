# Research: Model Management

**Feature**: 020-model-management
**Date**: 2025-11-28

## Technology Decisions

### 1. HuggingFace Hub Integration

**Decision**: Use HuggingFace Hub REST API directly with custom Go client

**Rationale**:
- No official Go SDK for HuggingFace Hub
- REST API is well-documented and stable
- Custom client allows fine-grained control over download resume, progress reporting
- Can implement exactly the features needed (model info, download, license check)

**Alternatives Considered**:
- `huggingface_hub` Python library via subprocess: Rejected due to Python dependency requirement
- Shell out to `huggingface-cli`: Rejected due to output parsing fragility
- Third-party Go clients: None mature enough for production use

**Key API Endpoints**:
- `GET /api/models/{repo_id}` - Model metadata, gating status
- `GET /api/models/{repo_id}/tree/{revision}` - File listing
- `GET /{repo_id}/resolve/{revision}/{filename}` - File download
- Headers: `Authorization: Bearer {hf_token}` for gated models

### 2. Object Storage Client

**Decision**: Use `aws-sdk-go-v2` with S3-compatible endpoint configuration

**Rationale**:
- Linode Object Storage is S3-compatible
- `aws-sdk-go-v2` is the standard, well-maintained SDK
- Supports multipart uploads natively
- Handles retries and backoff automatically

**Alternatives Considered**:
- `minio-go`: Good option but less ecosystem support
- Direct HTTP: Too much boilerplate for multipart uploads

**Configuration**:
```go
cfg, _ := config.LoadDefaultConfig(ctx,
    config.WithEndpointResolverWithOptions(
        aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
            return aws.Endpoint{URL: "https://us-east-1.linodeobjects.com"}, nil
        }),
    ),
)
```

### 3. Kubernetes Client

**Decision**: Use `client-go` with kubeconfig from CLI config

**Rationale**:
- Standard Kubernetes Go client
- Supports all needed operations (InferenceService CRDs, pods, events)
- Can use existing kubeconfig files
- Well-documented, production-proven

**Key Operations**:
- Create/Delete InferenceService (KServe CRD)
- Watch pod status
- Get pod logs
- List events

### 4. Configuration Storage

**Decision**: Local config file (`~/.ai-aas/config.yaml`) + server-side credentials

**Rationale**:
- Local config for: API endpoint, default environment, local preferences
- Server-side for: HF tokens, S3 credentials (encrypted in DB via admin-api)
- Follows security best practice: sensitive data not on disk
- CLI validates config exists on each command

**Config File Structure**:
```yaml
api:
  endpoint: https://api.ai-aas.example.com
  key: ak_xxx...
defaults:
  environment: development
```

### 5. Progress Reporting

**Decision**: Use `github.com/schollz/progressbar/v3` for terminal progress

**Rationale**:
- Well-maintained, popular library
- Supports download progress with ETA
- Works well with concurrent operations
- Graceful degradation for non-TTY output

**Alternatives Considered**:
- `mpb`: More powerful but more complex
- Custom implementation: Unnecessary complexity

### 6. Database Migrations

**Decision**: SQL migrations in `db/migrations/` following existing pattern

**Rationale**:
- Consistent with existing project structure
- Uses existing migration tooling (golang-migrate)
- Clear versioning with timestamps

### 7. Resume Support for Downloads

**Decision**: Implement chunked download with local manifest tracking

**Rationale**:
- Large model files (10-100GB) require resume capability
- Track downloaded chunks in local temp file
- On resume, check existing chunks via byte ranges
- HuggingFace supports Range headers

**Implementation Approach**:
```
~/.ai-aas/downloads/
├── {model-name}/
│   ├── .progress.json    # Tracks completed chunks
│   ├── file1.part        # Partial downloads
│   └── file2.part
```

### 8. Validation Framework

**Decision**: Layered validation with pluggable checks

**Rationale**:
- Each layer (registry, cache, deployment, endpoint, router) has independent checks
- Checks can run in parallel where possible
- Clear remediation messages for each failure
- JSON output for CI/CD integration

**Check Interface**:
```go
type Check interface {
    Name() string
    Layer() Layer
    Run(ctx context.Context, model *Model, env string) (*Result, error)
}
```

## Integration Patterns

### Admin API Integration

The CLI communicates with admin-api-service for all server-side operations:

| CLI Command | API Endpoint | Method |
|-------------|--------------|--------|
| `model add` | `/api/v1/models` | POST |
| `model list` | `/api/v1/models` | GET |
| `model info` | `/api/v1/models/{name}` | GET |
| `model remove` | `/api/v1/models/{name}` | DELETE |
| `credentials set` | `/api/v1/credentials` | POST |
| `credentials list` | `/api/v1/credentials` | GET |
| `model deploy` | `/api/v1/deployments` | POST |
| `model validate` | `/api/v1/models/{name}/validate` | POST |

### KServe InferenceService Pattern

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: llama-3-8b
  namespace: ai-aas-{environment}
spec:
  predictor:
    model:
      modelFormat:
        name: vllm
      storageUri: s3://ai-aas-models/llama-3-8b/v1.0.0-abc123/
      resources:
        limits:
          nvidia.com/gpu: "1"
          memory: "48Gi"
        requests:
          nvidia.com/gpu: "1"
          memory: "48Gi"
```

## Security Considerations

### Credential Handling

1. **HF Token Flow**:
   - User provides token via `credentials set hf-token`
   - CLI sends to admin-api over HTTPS
   - Admin-api encrypts with AES-256 and stores in DB
   - Token retrieved only during `model pull`, held in memory only

2. **S3 Credentials**:
   - Stored encrypted in DB
   - CLI retrieves for upload/download operations
   - Consider: IAM roles for production (future enhancement)

3. **API Key**:
   - Stored locally in config file
   - Used for all admin-api requests
   - Should have expiration/rotation (handled by admin-api)

### Audit Trail

All operations logged to `model_state_history` table:
- Who performed the action
- When it was performed
- What changed
- Optional reason text

## Open Questions Resolved

| Question | Resolution |
|----------|------------|
| Go vs Python CLI | Go - aligns with existing codebase, single binary distribution |
| Direct DB vs API | Via API - separation of concerns, existing auth/audit |
| Local vs remote config | Hybrid - local for preferences, remote for credentials |
| Custom vs library progress | Library (progressbar) - battle-tested, maintained |

