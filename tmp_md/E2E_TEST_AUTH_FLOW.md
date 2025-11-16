# E2E Test Authentication Flow

## How It Works

The E2E test does **NOT** obtain a token from a separate authentication service. Instead, it:

1. **Authenticates directly with user-org-service** itself
2. **Gets a token from user-org-service's own login endpoint**
3. **Uses that token for subsequent requests to user-org-service**

## Flow Diagram

```
┌─────────────────┐
│   E2E Test      │
│   (Go Binary)   │
└────────┬────────┘
         │
         │ 1. POST /v1/auth/login
         │    {email, password}
         ▼
┌──────────────────────┐
│  user-org-service   │
│  (Port 8081)        │
│                     │
│  Login Handler      │
│  ┌──────────────┐   │
│  │ OAuth2       │   │
│  │ Provider     │   │
│  │ (Fosite)     │   │
│  └──────────────┘   │
│         │           │
│         │ Generates │
│         │ token     │
│         ▼           │
│  Returns:           │
│  {                  │
│    "access_token":  │
│    "ory_at_...",   │
│    "expires_in":    │
│    3600             │
│  }                  │
└────────┬────────────┘
         │
         │ 2. Returns access_token
         ▼
┌─────────────────┐
│   E2E Test      │
│   Stores token  │
└────────┬────────┘
         │
         │ 3. Subsequent requests
         │    Authorization: Bearer <token>
         ▼
┌──────────────────────┐
│  user-org-service   │
│  Protected endpoints │
│  - POST /v1/orgs    │
│  - GET /v1/orgs/{id}│
│  - POST /v1/orgs/   │
│    {id}/invites     │
│  etc.               │
└──────────────────────┘
```

## Code Flow

### 1. Test Startup (`main()` function)

```go
// Login with seeded test user to get access token
testEmail := "admin@example.com"
testPassword := "nubipwdkryfmtaho123!"
accessToken, loginErr = login(client, apiURL, testEmail, testPassword)
```

**Location**: `services/user-org-service/cmd/e2e-test/main.go:92`

### 2. Login Function

```go
func login(client *http.Client, apiURL, email, password string) (string, error) {
    loginReq := map[string]any{
        "email":    email,
        "password": password,
    }
    
    // POST to user-org-service's own login endpoint
    loginResp, err := makeRequest(client, "POST", apiURL+"/v1/auth/login", loginReq, http.StatusOK)
    
    // Extract access_token from response
    accessToken, ok := loginResp["access_token"].(string)
    return accessToken, nil
}
```

**Location**: `services/user-org-service/cmd/e2e-test/main.go:160-177`

**Request**:
```http
POST http://localhost:8081/v1/auth/login
Content-Type: application/json

{
  "email": "admin@example.com",
  "password": "nubipwdkryfmtaho123!"
}
```

**Response**:
```json
{
  "access_token": "ory_at_...",
  "expires_in": 3600,
  "token_type": "bearer"
}
```

### 3. Token Generation (Inside user-org-service)

The user-org-service generates tokens **internally** using:

- **OAuth2 Provider**: Fosite library (`github.com/ory/fosite`)
- **Token Storage**: OAuth sessions stored in PostgreSQL
- **Token Type**: Bearer tokens (JWT-like format starting with `ory_at_`)

**Location**: `services/user-org-service/internal/httpapi/auth/handlers.go:141-177`

**Process**:
1. Validates email/password against database
2. Creates OAuth2 session using Fosite
3. Generates access token via `Provider.NewAccessRequest()`
4. Returns token in OAuth2 response format

### 4. Authenticated Requests

After login, tests use the token for protected endpoints:

```go
// Use makeAuthenticatedRequest instead of makeRequest
org, err := makeAuthenticatedRequest(
    client, 
    "POST", 
    apiURL+"/v1/orgs", 
    createReq, 
    token,  // <-- Access token from login
    http.StatusCreated
)
```

**Location**: `services/user-org-service/cmd/e2e-test/main.go:215`

**Request**:
```http
POST http://localhost:8081/v1/orgs
Authorization: Bearer ory_at_...
Content-Type: application/json

{
  "name": "Test Organization",
  "slug": "test-org-12345"
}
```

## Key Points

### ✅ user-org-service is Self-Contained

- **No separate authentication service** - user-org-service generates its own tokens
- **Built-in OAuth2 provider** - Uses Fosite library for OAuth2 flows
- **Token storage** - Tokens and sessions stored in PostgreSQL
- **Password validation** - Validates passwords against database directly

### ✅ Token Flow

1. **Login** → user-org-service validates credentials → generates token → returns token
2. **Authenticated requests** → Test includes token in `Authorization` header → user-org-service validates token → processes request

### ✅ Test Authentication

- **Direct authentication** - Test logs in directly to user-org-service
- **No external dependencies** - Does not require separate auth service
- **Seeded user** - Uses `admin@example.com` / `nubipwdkryfmtaho123!` created by `make seed`

## Architecture

### user-org-service Components

```
user-org-service
├── HTTP API Layer
│   └── /v1/auth/login (Handler)
│       ├── Validates credentials
│       └── Calls OAuth2 Provider
│
├── OAuth2 Provider (Fosite)
│   ├── Password Grant Flow
│   ├── Token Generation
│   └── Session Management
│
├── Storage Layer
│   ├── PostgreSQL (users, orgs, sessions)
│   └── OAuth Store (Fosite storage adapter)
│
└── Security
    ├── Password Hashing (bcrypt)
    └── Token Signing (Fosite HMAC)
```

## Summary

**Question**: Does the test obtain a token from an authentication service and then pass it into user-org-service?

**Answer**: **NO** - The test:
1. Authenticates **directly with user-org-service**
2. Gets a token **from user-org-service's own login endpoint**
3. Uses that token **for subsequent requests to user-org-service**

**user-org-service is self-contained** - it has its own OAuth2 provider (Fosite) that generates tokens internally. There is no separate authentication service.


