# Feature Specification: Server-Side Model Caching

**Feature ID**: 021-server-side-caching
**Epic**: ai-aas-bkr
**Created**: 2025-12-03
**Status**: Draft

## Overview

Move model caching from client-side (CLI) to server-side (background worker). Currently the CLI downloads models from HuggingFace to the local machine, then uploads to Object Storage. This requires the client to have bandwidth, disk space, and S3 credentials.

### Current State

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   HuggingFace    │ ──▶ │  Local Machine   │ ──▶ │  Object Storage  │
│   (Download)     │     │  (Temp Cache)    │     │  (S3/Linode)     │
└──────────────────┘     └──────────────────┘     └──────────────────┘
                               ▲
                               │
                    CLI runs here, requires:
                    • Bandwidth (2x model size)
                    • Disk space (full model size)
                    • S3 credentials (access/secret keys)
                    • HF token (for gated models)
```

### Target State

```
┌──────────────────┐                              ┌──────────────────┐
│   HuggingFace    │ ────────────────────────────▶│  Object Storage  │
│   (Download)     │                              │  (S3/Linode)     │
└──────────────────┘                              └──────────────────┘
                              ▲
                              │
                   Background Worker
                   (runs on server)

┌──────────────────┐          │
│   CLI / Client   │ ─────────┘
└──────────────────┘
   Only needs:
   • API access
   • No S3 credentials
   • No local bandwidth/space
```

## User Scenarios & Testing

### User Story 1 - Trigger Server-Side Model Pull (Priority: P1)

As a platform operator, I can trigger a model pull operation that runs entirely on the server without needing local bandwidth or S3 credentials.

**Why this priority**: Core value proposition - removes client-side resource requirements.

**Independent Test**: Can be tested by calling the API to create a pull job and verifying the model appears in object storage.

**Acceptance Scenarios**:

1. **Given** a registered model, **When** I call `POST /v1/models/{name}/pull`, **Then** a pull job is created with status `pending` and returns a job ID.
2. **Given** a pending pull job, **When** the worker processes it, **Then** the model files are downloaded to object storage and a cache entry is created.
3. **Given** an in-progress pull job, **When** I query the job status, **Then** I see progress updates (bytes downloaded/total).

---

### User Story 2 - Monitor Pull Job Progress (Priority: P1)

As a platform operator, I can monitor the progress of a server-side pull operation to know when it completes.

**Why this priority**: Users need visibility into long-running operations.

**Independent Test**: Can be tested by polling the job status endpoint during a pull operation.

**Acceptance Scenarios**:

1. **Given** an active pull job, **When** I call `GET /v1/models/{name}/pull/{job_id}`, **Then** I receive current status, progress percentage, and bytes downloaded.
2. **Given** a completed pull job, **When** I query its status, **Then** I see status `complete` with final size and file count.
3. **Given** a failed pull job, **When** I query its status, **Then** I see status `failed` with an error message explaining the failure.

---

### User Story 3 - Cancel Pull Operation (Priority: P2)

As a platform operator, I can cancel a running pull operation if I realize I started the wrong model or revision.

**Why this priority**: Error recovery is important for large, long-running operations.

**Acceptance Scenarios**:

1. **Given** an active pull job, **When** I call `DELETE /v1/models/{name}/pull/{job_id}`, **Then** the job status changes to `cancelled` and download stops.
2. **Given** a completed pull job, **When** I try to cancel it, **Then** I receive an error indicating the job is already complete.

---

### User Story 4 - CLI Server-Side Pull (Priority: P2)

As a platform operator, I can use the CLI with a `--server` flag to trigger server-side pulls instead of local pulls.

**Why this priority**: CLI UX must remain simple and consistent.

**Acceptance Scenarios**:

1. **Given** CLI configured with API access, **When** I run `ai-aas model cache pull llama-7b --server`, **Then** a server-side pull is triggered and progress is displayed.
2. **Given** a running server-side pull, **When** I press Ctrl+C, **Then** the pull job is cancelled on the server.

---

### Edge Cases

- HuggingFace rate limiting or temporary unavailability
- Object storage quota exceeded
- Network interruption during download/upload
- Worker crash mid-operation (job recovery)
- Concurrent pull requests for same model/revision (prevented by DB constraint)
- Gated models requiring HuggingFace token acceptance

## Architecture Decisions

### ADR-001: Worker Deployment Strategy

**Decision**: Embed worker in admin-api-service (not a separate service)

**Context**: We need to decide whether the background worker runs as:
1. Part of admin-api-service (embedded)
2. A separate cache-worker-service

**Rationale**:
- **Simpler deployment**: No additional Helm chart, ArgoCD application, or service to manage
- **Shared database connection**: Worker reuses existing pgx pool
- **Shared credentials**: Worker accesses platform_credentials via existing service methods
- **Sufficient for MVP**: Starting with 1 concurrent job doesn't require independent scaling
- **Easy to extract later**: If we need independent scaling, we can extract to separate service

**Consequences**:
- Worker lifecycle tied to admin-api-service restarts
- Resource limits shared with API service
- Must implement graceful shutdown to not lose in-progress jobs

**Future Consideration**: If pull operations become a bottleneck or require independent scaling, extract to `cache-worker-service` with dedicated resources.

---

### ADR-002: Job Queue Mechanism

**Decision**: Database polling (not Redis or external queue)

**Context**: We need a mechanism for the worker to discover and claim pending jobs.

**Options Considered**:
1. **Database polling**: Worker queries `pull_jobs WHERE status='pending'` periodically
2. **Redis queue**: Publish jobs to Redis, worker subscribes
3. **PostgreSQL LISTEN/NOTIFY**: Database-native pub/sub

**Rationale for database polling**:
- **No additional infrastructure**: No Redis deployment required
- **Atomic job claiming**: `UPDATE ... WHERE status='pending' RETURNING` prevents race conditions
- **Existing partial unique index**: Database already prevents duplicate jobs per model/revision
- **Sufficient throughput**: Model pulls are infrequent (minutes/hours apart) - polling every 5s is fine
- **Simpler failure recovery**: Job status persisted in database survives worker restarts

**Polling Implementation**:
```sql
-- Atomically claim a pending job
UPDATE pull_jobs
SET status = 'downloading', started_at = NOW()
WHERE id = (
    SELECT id FROM pull_jobs
    WHERE status = 'pending'
    ORDER BY started_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;
```

**Consequences**:
- 5-second latency between job creation and pickup (acceptable)
- Slight database load from polling (negligible)
- No complex queue infrastructure to maintain

---

### ADR-003: Progress Reporting Mechanism

**Decision**: Database-based progress with API polling (not WebSocket)

**Context**: Clients need to see download progress for large models.

**Rationale**:
- **Simpler implementation**: No WebSocket infrastructure needed
- **Stateless API**: Works with any HTTP client, including curl
- **CLI can poll**: CLI already handles progress bars; polling every 2s is sufficient
- **Database durability**: Progress survives client disconnects

**Progress Updates**:
- Worker updates `bytes_completed` and `progress` every 10 seconds or 50MB (whichever comes first)
- Client polls `GET /v1/models/{name}/pull/{job_id}` for status
- Response includes: `status`, `progress` (0-100), `bytes_completed`, `bytes_total`

**Future Consideration**: Add WebSocket endpoint for real-time streaming if polling latency becomes an issue.

---

### ADR-004: Credential Access

**Decision**: Worker retrieves credentials from platform_credentials table via existing service methods

**Context**: Worker needs HuggingFace token and S3 credentials to perform downloads/uploads.

**Implementation**:
- Credentials already stored encrypted in `platform_credentials` table
- `admin-api-service` already has credential decryption logic
- Worker reuses existing credential service methods
- No credentials exposed in configuration files or environment variables

**Security**:
- Encryption key for credentials from `CREDENTIAL_ENCRYPTION_KEY` env var
- Credentials never logged or exposed in job status responses
- Failed jobs don't include credential-related details in error messages

---

### ADR-005: Error Handling and Retry Strategy

**Decision**: Fail fast with clear error messages; no automatic retry

**Context**: What happens when downloads fail?

**Rationale**:
- **User-initiated retry**: User can re-trigger pull after investigating failure
- **Clear error messages**: Job stores detailed error in `error_message` column
- **Partial cleanup**: Failed jobs leave partial files in S3 (can be cleaned by GC)
- **No infinite loops**: Automatic retries could repeatedly hit rate limits or quota issues

**Error Categories**:
1. **Transient (could retry)**: Network timeout, HF rate limit (429)
2. **Permanent (no retry)**: Model not found (404), invalid credentials (401/403), S3 quota exceeded

**Future Consideration**: Add configurable retry policy for transient errors.

---

### ADR-006: Graceful Shutdown

**Decision**: Worker finishes current file on SIGTERM, abandons remaining files

**Context**: What happens when admin-api-service restarts during a pull?

**Implementation**:
- On SIGTERM, worker stops picking up new jobs
- Current file upload completes (if possible within 30s grace period)
- Job marked as `failed` with message "Worker shutdown during operation"
- Next startup, orphaned `downloading` jobs older than 1 hour are reset to `pending`

**Job Recovery Query**:
```sql
-- On worker startup, recover orphaned jobs
UPDATE pull_jobs
SET status = 'pending', started_at = NOW()
WHERE status IN ('downloading', 'uploading')
AND started_at < NOW() - INTERVAL '1 hour';
```

## Requirements

### Functional Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-001 | POST /v1/models/{name}/pull creates a pull job and returns job ID | P1 |
| FR-002 | GET /v1/models/{name}/pull lists all pull jobs for a model | P1 |
| FR-003 | GET /v1/models/{name}/pull/{job_id} returns job status and progress | P1 |
| FR-004 | DELETE /v1/models/{name}/pull/{job_id} cancels an active job | P2 |
| FR-005 | Worker polls for pending jobs and executes them | P1 |
| FR-006 | Worker updates progress during download/upload | P1 |
| FR-007 | Worker creates cache entry on successful completion | P1 |
| FR-008 | Worker checks for cancellation and stops promptly | P2 |
| FR-009 | CLI --server flag triggers server-side pull | P2 |
| FR-010 | CLI displays progress by polling job status | P2 |

### Non-Functional Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| NFR-001 | Job pickup latency ≤ 10 seconds from creation | P1 |
| NFR-002 | Progress updates at least every 30 seconds | P1 |
| NFR-003 | Graceful shutdown completes within 30 seconds | P1 |
| NFR-004 | Single concurrent job per worker instance (configurable) | P1 |
| NFR-005 | Memory usage ≤ 512MB during operation | P2 |

## Success Criteria

| ID | Criterion | Measurement |
|----|-----------|-------------|
| SC-001 | Server-side pull completes for 10GB model | Verify files in S3 after pull |
| SC-002 | Progress visible within 30s of job start | Poll API during download |
| SC-003 | Cancellation stops download within 10s | Monitor bytes_completed after cancel |
| SC-004 | Worker recovers after restart | Kill pod, verify job resumes |
| SC-005 | No S3 credentials required by CLI | Run pull with --server, verify no S3 config needed |

## Technical Design

### Component Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     admin-api-service                            │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   Handlers  │  │   Worker    │  │   Services              │ │
│  │             │  │             │  │                         │ │
│  │ POST /pull  │──│ Poll loop   │──│ models.Service          │ │
│  │ GET /pull   │  │ Job exec    │  │ credentials.Service     │ │
│  │ DELETE /pull│  │ Progress    │  │                         │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
│         │                │                    │                 │
│         └────────────────┼────────────────────┘                 │
│                          │                                      │
│                    ┌─────▼─────┐                                │
│                    │ PostgreSQL │                                │
│                    │ pull_jobs  │                                │
│                    │ model_cache│                                │
│                    │ credentials│                                │
│                    └───────────┘                                │
└─────────────────────────────────────────────────────────────────┘
                           │
           ┌───────────────┼───────────────┐
           ▼                               ▼
    ┌─────────────┐                 ┌─────────────┐
    │ HuggingFace │                 │   Object    │
    │     API     │                 │   Storage   │
    └─────────────┘                 └─────────────┘
```

### Sequence Diagram: Server-Side Pull

```
CLI                    API                    Worker               HuggingFace        S3
 │                      │                       │                      │              │
 │ POST /pull           │                       │                      │              │
 │─────────────────────▶│                       │                      │              │
 │                      │ INSERT pull_job       │                      │              │
 │                      │ (status=pending)      │                      │              │
 │ 202 {job_id}         │                       │                      │              │
 │◀─────────────────────│                       │                      │              │
 │                      │                       │                      │              │
 │ GET /pull/{id}       │                       │ poll for pending     │              │
 │─────────────────────▶│                       │ jobs every 5s        │              │
 │ {status: pending}    │                       │                      │              │
 │◀─────────────────────│                       │                      │              │
 │                      │                       │ claim job            │              │
 │                      │                       │ (UPDATE RETURNING)   │              │
 │                      │                       │                      │              │
 │                      │                       │ GET model files      │              │
 │                      │                       │─────────────────────▶│              │
 │                      │                       │ file stream          │              │
 │                      │                       │◀─────────────────────│              │
 │ GET /pull/{id}       │                       │                      │              │
 │─────────────────────▶│                       │                      │              │
 │ {status: downloading,│                       │ UPDATE progress      │              │
 │  progress: 45%}      │                       │                      │              │
 │◀─────────────────────│                       │                      │              │
 │                      │                       │                      │              │
 │                      │                       │ PUT file             │              │
 │                      │                       │─────────────────────────────────────▶│
 │                      │                       │                      │              │
 │                      │                       │ UPDATE status=complete│             │
 │                      │                       │ INSERT cache_entry   │              │
 │                      │                       │                      │              │
 │ GET /pull/{id}       │                       │                      │              │
 │─────────────────────▶│                       │                      │              │
 │ {status: complete}   │                       │                      │              │
 │◀─────────────────────│                       │                      │              │
```

### File Structure

```
services/admin-api-service/
├── internal/
│   ├── worker/                    # NEW: Background worker
│   │   ├── worker.go              # Worker loop and lifecycle
│   │   ├── job.go                 # Job execution logic
│   │   ├── executor.go            # Pull execution (uses shared lib)
│   │   └── worker_test.go         # Unit tests
│   ├── handlers/models/
│   │   ├── handler.go             # Existing handlers
│   │   └── pull.go                # NEW: Pull job handlers
│   └── config/
│       └── config.go              # Add worker config options

shared/go/modelcache/              # NEW: Shared library (from CLI)
├── huggingface/
│   ├── client.go                  # HF API client
│   └── download.go                # Download logic
├── storage/
│   ├── s3.go                      # S3 operations
│   └── manifest.go                # Manifest handling
└── pull/
    └── service.go                 # Orchestration logic
```

### Configuration

```yaml
# values.yaml additions
worker:
  enabled: true
  pollInterval: "5s"
  maxConcurrentJobs: 1
  progressUpdateInterval: "10s"
  shutdownGracePeriod: "30s"
  jobRecoveryAge: "1h"
```

### API Specification

#### POST /v1/models/{name}/pull

Create a new pull job.

**Request**:
```json
{
  "revision": "main"  // optional, defaults to "main"
}
```

**Response** (202 Accepted):
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "model_name": "llama-7b",
  "revision": "main",
  "created_at": "2025-12-03T12:00:00Z"
}
```

**Errors**:
- 404: Model not found
- 409: Pull job already active for this model/revision

#### GET /v1/models/{name}/pull

List pull jobs for a model.

**Response** (200 OK):
```json
{
  "jobs": [
    {
      "job_id": "550e8400-e29b-41d4-a716-446655440000",
      "revision": "main",
      "status": "downloading",
      "progress": 45.5,
      "bytes_completed": 6442450944,
      "bytes_total": 14155776000,
      "started_at": "2025-12-03T12:00:00Z",
      "created_by": "admin"
    }
  ]
}
```

#### GET /v1/models/{name}/pull/{job_id}

Get status of a specific pull job.

**Response** (200 OK):
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "model_name": "llama-7b",
  "revision": "main",
  "status": "downloading",
  "progress": 45.5,
  "bytes_completed": 6442450944,
  "bytes_total": 14155776000,
  "current_file": "model-00002-of-00003.safetensors",
  "started_at": "2025-12-03T12:00:00Z",
  "created_by": "admin"
}
```

**Status Values**: `pending`, `downloading`, `uploading`, `complete`, `failed`, `cancelled`

#### DELETE /v1/models/{name}/pull/{job_id}

Cancel an active pull job.

**Response** (200 OK):
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "cancelled"
}
```

**Errors**:
- 404: Job not found
- 409: Job already completed/failed/cancelled

## Testing Strategy

### Unit Tests

| Component | Test Cases |
|-----------|------------|
| Worker | Job claiming atomicity, progress updates, cancellation, graceful shutdown |
| Handlers | Create job, get status, list jobs, cancel job, error cases |
| Shared lib | HF file listing, download with progress, S3 upload, manifest |

### Integration Tests

| Test | Description |
|------|-------------|
| Full pull flow | Create job → worker picks up → files in S3 → cache entry created |
| Cancellation | Create job → start downloading → cancel → verify stopped |
| Recovery | Create job → kill worker → restart → job resumes |
| Concurrent prevention | Create two jobs same model → second returns 409 |

### E2E Tests

| Test | Description |
|------|-------------|
| CLI server-side pull | `ai-aas model cache pull --server` → verify completion |
| CLI cancel | Start pull → Ctrl+C → verify cancelled |

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| HuggingFace rate limiting | Pull fails | Implement backoff, clear error message |
| Large model exhausts memory | OOM kill | Stream downloads, don't buffer entire file |
| S3 quota exceeded | Upload fails | Check quota before starting, clear error |
| Worker crash loses progress | User frustration | Job recovery on startup, resumable downloads |
| Long-running job blocks others | Queue backup | Single concurrent job is acceptable for MVP |

## Implementation Phases

### Phase 1: Core Infrastructure (P1 issues)
1. Shared library extraction (ai-aas-63h)
2. API endpoints (ai-aas-2re)
3. Worker implementation (ai-aas-0zu)

### Phase 2: CLI and Configuration (P2 issues)
1. CLI --server flag (ai-aas-795)
2. Helm chart config (ai-aas-9x4)
3. Unit tests (ai-aas-60j)

### Phase 3: Testing and Documentation (P2-P3 issues)
1. Integration tests (ai-aas-1bv)
2. E2E tests (ai-aas-tzl)
3. CI updates (ai-aas-oja)
4. Documentation (ai-aas-5nh)

## Open Questions

1. **Resumable downloads**: Should we support resuming interrupted downloads at the file level? (Currently: No, restart from beginning)
2. **Concurrent jobs**: Should we allow multiple concurrent pull jobs for different models? (Currently: 1 job at a time)
3. **Notification**: Should we add webhook/notification on job completion? (Currently: No, poll only)

## References

- Epic: ai-aas-bkr
- Related: ai-aas-afi (Pre-cache models to Object Storage)
- Database schema: `db/migrations/20251128_008_create_pull_jobs.sql`
- Existing CLI logic: `services/ai-aas-cli/internal/cache/pull.go`
- Credentials: `db/migrations/20251128_007_create_platform_credentials.sql`
