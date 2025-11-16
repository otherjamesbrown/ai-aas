# User-Org-Service HTTP Tests

## ✅ Yes, We Have HTTP Tests!

The user-org-service has comprehensive HTTP/integration tests that can be used to test the component over HTTP.

## Test Suites Available

### 1. E2E Test Suite (`cmd/e2e-test/main.go`)

**Purpose**: End-to-end HTTP tests that exercise the complete service via HTTP requests.

**Tests Included**:
- ✅ `TestHealthCheck` - Verifies service is reachable
- ✅ `TestOrganizationLifecycle` - Organization CRUD operations (POST, GET, PATCH)
- ✅ `TestUserInviteFlow` - User invitation and creation
- ✅ `TestUserManagement` - User status updates, profile management, suspend/activate
- ✅ `TestAuthenticationFlow` - Complete auth flow (login, refresh, logout - requires seeded user)

**HTTP Methods Tested**:
- `GET /healthz` - Health check
- `POST /v1/orgs` - Create organization
- `GET /v1/orgs/{orgId}` - Get organization by ID
- `GET /v1/orgs/{slug}` - Get organization by slug
- `PATCH /v1/orgs/{orgId}` - Update organization
- `POST /v1/orgs/{orgId}/invites` - Invite user
- `GET /v1/orgs/{orgId}/users/{userId}` - Get user
- `PATCH /v1/orgs/{orgId}/users/{userId}` - Update user

**Features**:
- Uses standard `net/http` client
- Retry logic with exponential backoff
- JSON request/response handling
- Test context for assertions
- Sequential execution to avoid race conditions

### 2. K6 Smoke Tests (`scripts/smoke-k6.js`)

**Purpose**: Load/performance testing with K6.

**Features**:
- Load testing scenarios
- Performance metrics
- Rate limiting tests

### 3. Integration Tests (Unit Tests with Testcontainers)

**Location**: `internal/storage/postgres/store_test.go`, `internal/oauth/store_test.go`

**Purpose**: Database integration tests using Testcontainers (PostgreSQL in Docker).

## How to Run the E2E Tests

### Prerequisites

1. **Service Running**: User-org-service must be running
   ```bash
   cd services/user-org-service
   make run  # Starts on http://localhost:8081
   ```

2. **Database Migrated**: Migrations must be applied
   ```bash
   make migrate
   ```

### Run Tests Locally

```bash
cd services/user-org-service

# Build and run E2E tests (defaults to http://localhost:8081)
make e2e-test-local

# Or specify custom API URL
API_URL=http://localhost:8081 make e2e-test
```

### Run Tests Against Development Environment

```bash
# Set development API URL
export DEV_API_URL=http://user-org-service.dev.platform.internal:8081

# Run tests
make e2e-test-dev
```

## Test Code Structure

The E2E test suite (`cmd/e2e-test/main.go`) includes:

### Helper Functions

```go
// makeRequest - Performs HTTP request and returns JSON response
func makeRequest(client *http.Client, method, url string, body map[string]any, expectedStatus int) (map[string]any, error)

// makeAuthenticatedRequest - Performs HTTP request with Bearer token
func makeAuthenticatedRequest(client *http.Client, method, url string, body map[string]any, token string, expectedStatus int) (map[string]any, error)

// retryRequest - Retries request with exponential backoff
func retryRequest(client *http.Client, req *http.Request) (*http.Response, error)
```

### Test Functions

Each test function follows this pattern:
```go
func testSomething(tc *testContext, client *http.Client, apiURL string) error {
    // Test implementation using makeRequest()
    return nil // or error
}
```

### Example Test: Organization Lifecycle

```go
func testOrganizationLifecycle(tc *testContext, client *http.Client, apiURL string) error {
    // Create organization
    org, err := makeRequest(client, "POST", apiURL+"/v1/orgs", createReq, http.StatusCreated)
    
    // Get organization by ID
    org2, err := makeRequest(client, "GET", apiURL+"/v1/orgs/"+orgID, nil, http.StatusOK)
    
    // Update organization
    org4, err := makeRequest(client, "PATCH", apiURL+"/v1/orgs/"+orgID, updateReq, http.StatusOK)
    
    return nil
}
```

## What's Missing from E2E Tests

The `testAuthenticationFlow` test currently has limitations:

**Issue**: Login test requires an active user with a known password, but the invite flow doesn't return the temporary password.

**Workarounds**:
1. Use seeded test users (via `cmd/seed/main.go`)
2. Add test endpoint that creates active users directly
3. Have invite response include temp password in dev mode
4. Use recovery flow to set password after invite

**Current Status**: 
- ✅ Organization creation works
- ✅ User invite works (creates user with "invited" status)
- ⚠️ Login test skipped (requires active user + known password)
- ⚠️ Full auth flow (MFA, API key lifecycle) not yet tested

## Using Tests to Verify UI Integration

You can use these E2E tests to verify the UI → user-org-service integration:

1. **Run the E2E tests** to ensure the service works:
   ```bash
   cd services/user-org-service
   make e2e-test-local
   ```

2. **Test login endpoint specifically**:
   ```bash
   # Using seeded user
   curl -X POST http://localhost:8081/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{
       "email": "admin@example.com",
       "password": "nubipwdkryfmtaho123!"
     }'
   ```

3. **Verify CORS headers**:
   ```bash
   curl -X OPTIONS http://localhost:8081/v1/auth/login \
     -H "Origin: http://localhost:5173" \
     -H "Access-Control-Request-Method: POST" \
     -v
   ```

## Summary

✅ **E2E tests exist** and test HTTP endpoints  
✅ **Tests cover** organizations, users, invites, and user management  
✅ **Tests use** standard `net/http` client (same as UI would use)  
⚠️ **Auth tests** have limitations (need seeded users or test endpoints)  
✅ **Can be run** locally or against deployed services

**Next Steps**:
1. Run `make e2e-test-local` to verify service is working
2. Use seeded users for login testing
3. Consider extending tests to cover full auth flow with seeded data

