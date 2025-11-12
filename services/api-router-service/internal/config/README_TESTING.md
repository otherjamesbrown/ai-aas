# Testing Config Service Integration

## Running Tests

### Basic Tests (No etcd Required)

The test suite includes tests that verify fallback behavior when etcd is unavailable:

```bash
go test ./internal/config -v
```

These tests verify:
- ✅ Loading policies from cache when etcd is unavailable
- ✅ Policy lookup with cache hits
- ✅ Global policy fallback
- ✅ Error handling when no sources are available

### Full Integration Tests (etcd Required)

To run tests that require etcd, you need a running etcd instance:

#### Option 1: Using Docker

```bash
# Start etcd in Docker
docker run -d \
  --name etcd-test \
  -p 2379:2379 \
  -p 2380:2380 \
  quay.io/coreos/etcd:v3.6.6 \
  etcd \
  --name etcd-test \
  --data-dir /etcd-data \
  --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://localhost:2379 \
  --listen-peer-urls http://0.0.0.0:2380 \
  --initial-advertise-peer-urls http://localhost:2380 \
  --initial-cluster etcd-test=http://localhost:2380 \
  --initial-cluster-token etcd-cluster-1 \
  --initial-cluster-state new

# Run tests
ETCD_ENDPOINT=localhost:2379 go test ./internal/config -v

# Cleanup
docker stop etcd-test && docker rm etcd-test
```

#### Option 2: Using Local etcd Installation

If you have etcd installed locally:

```bash
# Start etcd (in a separate terminal)
etcd --listen-client-urls http://localhost:2379 --advertise-client-urls http://localhost:2379

# Run tests
ETCD_ENDPOINT=localhost:2379 go test ./internal/config -v
```

## Test Coverage

The test suite covers:

1. **Cache Fallback** (`TestLoader_Load_FromCache`)
   - Verifies policies load from cache when etcd is unavailable

2. **etcd Integration** (`TestLoader_Load_FromEtcd`)
   - Verifies policies load from etcd and are cached locally

3. **Policy Lookup** (`TestLoader_GetPolicy_CacheHit`)
   - Verifies cache hit behavior

4. **Global Policy Fallback** (`TestLoader_GetPolicy_GlobalFallback`)
   - Verifies org-specific requests fallback to global policies

5. **etcd Lookup** (`TestLoader_GetPolicy_FromEtcd`)
   - Verifies etcd lookup on cache miss

6. **Watch Updates** (`TestLoader_Watch_UpdatesCache`)
   - Verifies real-time policy updates via etcd watch

7. **Connection Management** (`TestLoader_Stop_ClosesConnection`)
   - Verifies proper cleanup on stop

8. **Error Handling** (`TestLoader_Load_NoSourcesAvailable`)
   - Verifies error when no policies available

## Manual Testing

You can manually test the integration by storing a policy in etcd:

```bash
# Store a test policy
etcdctl put /api-router/policies/*/gpt-4o '{
  "policy_id": "test-policy",
  "organization_id": "*",
  "model": "gpt-4o",
  "backends": [
    {
      "backend_id": "backend-1",
      "weight": 70
    },
    {
      "backend_id": "backend-2",
      "weight": 30
    }
  ],
  "failover_threshold": 3,
  "updated_at": "2025-01-01T00:00:00Z",
  "version": 1
}'

# Verify it's stored
etcdctl get /api-router/policies/*/gpt-4o
```

Then run the service and verify it loads the policy:

```bash
CONFIG_SERVICE_ENDPOINT=localhost:2379 go run ./cmd/router
```

