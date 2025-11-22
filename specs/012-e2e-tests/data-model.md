# Data Model: End-to-End Tests

**Feature**: `012-e2e-tests`  
**Date**: 2025-01-27  
**Status**: Draft

## Overview

This document defines the data models for test fixtures, test results, and test artifacts used by the end-to-end test harness. All test data is designed to be deterministic, isolated, and cleanable.

## Test Fixtures

### Organization Fixture

```yaml
organization:
  id: string                    # Auto-generated or test-specific prefix
  name: string                  # Test organization name (e.g., "e2e-test-org-{timestamp}")
  slug: string                  # URL-safe identifier
  created_at: timestamp         # Creation timestamp
  metadata:
    test_run_id: string         # Unique test run identifier
    test_suite: string          # Test suite name (e.g., "happy-path")
    cleanup_after: timestamp    # When to clean up this org
```

### User Fixture

```yaml
user:
  id: string                    # Auto-generated or test-specific prefix
  email: string                 # Test email (e.g., "e2e-user-{timestamp}@test.example.com")
  name: string                  # Display name
  organization_id: string       # Reference to organization fixture
  roles: []string               # User roles (e.g., ["admin", "member"])
  status: string                # "active", "suspended", "invited"
  created_at: timestamp
  metadata:
    test_run_id: string
    test_suite: string
    cleanup_after: timestamp
```

### API Key Fixture

```yaml
api_key:
  id: string                    # Auto-generated
  key: string                   # API key value (masked in logs)
  organization_id: string       # Reference to organization fixture
  name: string                  # Key name (e.g., "e2e-test-key-{timestamp}")
  scopes: []string              # Key scopes (e.g., ["inference:read", "inference:write"])
  expires_at: timestamp        # Optional expiration
  created_at: timestamp
  metadata:
    test_run_id: string
    test_suite: string
    cleanup_after: timestamp
```

### Budget Fixture

```yaml
budget:
  id: string                    # Auto-generated
  organization_id: string       # Reference to organization fixture
  limit: number                 # Budget limit (e.g., 1000.00)
  currency: string              # Currency code (e.g., "USD")
  period: string                # "monthly", "yearly", "lifetime"
  current_usage: number        # Current usage amount
  created_at: timestamp
  metadata:
    test_run_id: string
    test_suite: string
    cleanup_after: timestamp
```

## Test Context

### Test Run Context

```yaml
test_run:
  id: string                    # Unique test run identifier (UUID)
  suite: string                 # Test suite name
  environment: string           # "local", "development", "staging", "production"
  started_at: timestamp
  completed_at: timestamp       # Optional, set on completion
  status: string                # "running", "passed", "failed", "skipped"
  parallel_workers: number      # Number of parallel test workers
  metadata:
    ci_run_id: string           # CI system run ID (e.g., GitHub Actions run ID)
    git_commit: string          # Git commit SHA
    git_branch: string          # Git branch name
```

### Test Case Context

```yaml
test_case:
  id: string                    # Unique test case identifier
  name: string                  # Test case name (e.g., "TestOrganizationLifecycle")
  suite: string                 # Test suite name
  test_run_id: string           # Reference to test run
  started_at: timestamp
  completed_at: timestamp       # Optional
  status: string                # "running", "passed", "failed", "skipped"
  duration_ms: number           # Test execution duration
  fixtures:
    organization_ids: []string   # Organizations created by this test
    user_ids: []string           # Users created by this test
    api_key_ids: []string        # API keys created by this test
  metadata:
    worker_id: string            # Parallel worker identifier
    retry_count: number         # Number of retries (if applicable)
```

## Test Results

### Test Step Result

```yaml
test_step:
  id: string                    # Unique step identifier
  test_case_id: string          # Reference to test case
  name: string                  # Step name (e.g., "CreateOrganization")
  started_at: timestamp
  completed_at: timestamp
  status: string                # "passed", "failed", "skipped"
  duration_ms: number
  request:
    method: string              # HTTP method
    url: string                # Request URL
    headers: map[string]string  # Request headers (sensitive values masked)
    body: string               # Request body (sensitive values masked)
  response:
    status_code: number        # HTTP status code
    headers: map[string]string # Response headers
    body: string               # Response body
    duration_ms: number         # Response duration
  correlation_ids:
    request_id: string          # Request correlation ID
    trace_id: string            # Trace ID (if available)
    span_id: string             # Span ID (if available)
  error: string                 # Error message (if failed)
  assertions: []assertion      # Assertions performed
```

### Assertion

```yaml
assertion:
  type: string                  # "status_code", "body_contains", "header_present", etc.
  expected: any                # Expected value
  actual: any                  # Actual value
  passed: boolean              # Whether assertion passed
  message: string              # Assertion message
```

## Test Artifacts

### Test Artifact

```yaml
artifact:
  id: string                    # Unique artifact identifier
  test_run_id: string           # Reference to test run
  test_case_id: string          # Optional, reference to specific test case
  type: string                  # "log", "screenshot", "har", "json", "xml"
  name: string                  # Artifact name
  path: string                  # File path or storage location
  size_bytes: number           # Artifact size
  created_at: timestamp
  metadata:
    correlation_id: string     # Correlation ID (if applicable)
    service: string             # Service name (if applicable)
```

### Test Report

```yaml
test_report:
  test_run_id: string           # Reference to test run
  format: string                # "junit", "json", "html"
  path: string                  # Report file path
  summary:
    total_tests: number
    passed: number
    failed: number
    skipped: number
    duration_ms: number
  created_at: timestamp
```

## Test Configuration

### Test Environment Configuration

```yaml
test_config:
  environment: string           # "local", "development", "staging", "production"
  api_urls:
    user_org_service: string    # User-org service API URL
    api_router_service: string  # API router service URL
    analytics_service: string   # Analytics service URL
  credentials:
    admin_api_key: string       # Admin API key (masked in logs)
    test_user_email: string     # Test user email
    test_user_password: string  # Test user password (masked)
  timeouts:
    request_timeout_ms: number  # HTTP request timeout
    test_timeout_ms: number     # Test case timeout
    cleanup_timeout_ms: number  # Cleanup timeout
  retries:
    max_retries: number         # Maximum retry attempts
    retry_delay_ms: number      # Delay between retries
    backoff_multiplier: number  # Exponential backoff multiplier
  parallel:
    enabled: boolean            # Enable parallel execution
    workers: number             # Number of parallel workers
  cleanup:
    enabled: boolean            # Enable automatic cleanup
    delay_seconds: number       # Delay before cleanup
  artifacts:
    enabled: boolean            # Enable artifact collection
    output_dir: string          # Artifact output directory
    include_request_bodies: boolean  # Include request bodies in artifacts
    include_response_bodies: boolean  # Include response bodies in artifacts
```

## Test Isolation

### Resource Namespace

```yaml
namespace:
  prefix: string                # Test resource prefix (e.g., "e2e-{test_run_id}")
  pattern: string               # Naming pattern for resources
  ttl_seconds: number           # Time-to-live for resources
  cleanup_strategy: string      # "immediate", "delayed", "manual"
```

### Test Data Isolation

- Each test run uses a unique namespace prefix
- Test fixtures are tagged with `test_run_id` for cleanup
- Parallel test workers use distinct resource namespaces
- Test data is isolated by timestamp and worker ID

## Correlation IDs

### Request Correlation

```yaml
correlation:
  request_id: string            # Request correlation ID (UUID)
  trace_id: string              # Distributed trace ID
  span_id: string               # Span ID
  test_case_id: string          # Test case identifier
  test_step_id: string          # Test step identifier
  service: string               # Service name
  timestamp: timestamp          # Request timestamp
```

Correlation IDs enable:
- Linking test steps to service logs
- Tracing requests across service boundaries
- Debugging failures with full context
- Verifying audit log entries

## Examples

### Example Test Fixture

```json
{
  "organization": {
    "id": "org-e2e-abc123",
    "name": "e2e-test-org-20250127-120000",
    "slug": "e2e-test-org-20250127-120000",
    "metadata": {
      "test_run_id": "run-xyz789",
      "test_suite": "happy-path",
      "cleanup_after": "2025-01-27T12:30:00Z"
    }
  },
  "user": {
    "id": "user-e2e-def456",
    "email": "e2e-user-20250127-120000@test.example.com",
    "organization_id": "org-e2e-abc123",
    "roles": ["admin"]
  },
  "api_key": {
    "id": "key-e2e-ghi789",
    "organization_id": "org-e2e-abc123",
    "name": "e2e-test-key-20250127-120000"
  }
}
```

### Example Test Step Result

```json
{
  "id": "step-123",
  "test_case_id": "test-org-lifecycle",
  "name": "CreateOrganization",
  "status": "passed",
  "duration_ms": 245,
  "request": {
    "method": "POST",
    "url": "https://user-org-service.dev.platform.internal/v1/organizations",
    "headers": {
      "Authorization": "Bearer ***",
      "Content-Type": "application/json"
    },
    "body": "{\"name\":\"e2e-test-org-20250127-120000\"}"
  },
  "response": {
    "status_code": 201,
    "body": "{\"id\":\"org-e2e-abc123\",\"name\":\"e2e-test-org-20250127-120000\"}",
    "duration_ms": 245
  },
  "correlation_ids": {
    "request_id": "req-xyz789",
    "trace_id": "trace-abc123"
  }
}
```

### Example Test Report

```json
{
  "test_run_id": "run-xyz789",
  "format": "json",
  "summary": {
    "total_tests": 15,
    "passed": 14,
    "failed": 1,
    "skipped": 0,
    "duration_ms": 125000
  },
  "test_cases": [
    {
      "id": "test-org-lifecycle",
      "name": "TestOrganizationLifecycle",
      "status": "passed",
      "duration_ms": 5000
    },
    {
      "id": "test-budget-enforcement",
      "name": "TestBudgetEnforcement",
      "status": "failed",
      "duration_ms": 3000,
      "error": "Expected status 403, got 200"
    }
  ]
}
```

