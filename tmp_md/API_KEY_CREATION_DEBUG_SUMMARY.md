# API Key Creation 401 Error - Debug Summary

## Issue Overview

The E2E test for API key creation is failing because:
1. **Login from browser returns 500** (while curl login works fine)
2. **API key creation requests return 401** (missing Authorization header)
3. The 401s occur because login failed, so no token is stored in `sessionStorage`

## Current Status

### ✅ What's Working

1. **Backend Service**: `user-org-service` is running and healthy on port 8081
2. **Direct API Calls**: 
   - Login via `curl` → Returns 200 with `access_token` ✓
   - API key creation via `curl` with token → Returns 201 with API key ✓
3. **Logging**: New zap-based logging is working and providing detailed debug information
4. **Code Changes**: Frontend updated to call `user-org-service` directly (bypassing api-router)

### ❌ What's Failing

1. **Browser Login**: 
   - Playwright test login → Returns 500 error
   - Logs show: `"status":500,"duration_ms":"310.439566ms"` for browser requests
   - No error details in logs (need to check for panic/recovery)

2. **API Key Creation from Browser**:
   - Requests return 401 Unauthorized
   - Logs show: `"RequireAuth: missing authorization header"`
   - Root cause: Login failed, so `sessionStorage.getItem('auth_token')` returns `null`

## Root Cause Analysis

### The Chain of Failure

```
Browser Login Request
  ↓
Returns 500 (instead of 200)
  ↓
No access_token stored in sessionStorage
  ↓
API Key Creation Request
  ↓
No Authorization header (token is null)
  ↓
RequireAuth middleware rejects with 401
```

### Why Login Fails from Browser but Works with curl

**Possible causes:**
1. **CORS issues**: Browser sends preflight OPTIONS request, but actual POST might have different headers
2. **Request format**: Browser might be sending request in a format the handler doesn't expect
3. **Error handling**: The login handler might be panicking or returning 500 without logging the error
4. **Frontend code not rebuilt**: The web portal might still be using old code that calls api-router instead of user-org-service

## Code Changes Made

### Frontend (`web/portal/src/providers/AuthProvider.tsx`)
- ✅ Updated `loginWithPassword` to call `user-org-service` directly at `http://localhost:8081/v1/auth/login`
- ✅ Updated `userinfo` fetch to also use `user-org-service` directly
- ✅ Uses `axios` directly instead of `httpClient` (which was configured for api-router)

### Backend (`services/user-org-service/internal/httpapi/auth/handlers.go`)
- ✅ Already has verbose zap logging added
- ✅ Login handler includes detailed debug logs

### Backend (`services/user-org-service/internal/httpapi/middleware/auth.go`)
- ✅ Updated to use zap logging (was zerolog)
- ✅ Verbose logging shows:
  - Authorization header presence
  - Token validation steps
  - Session extraction
  - Success/failure reasons

## Log Evidence

### Successful curl Login
```
{"level":"info","path":"/v1/auth/login","status":200,"duration_ms":"389.830924ms"}
```

### Failed Browser Login
```
{"level":"info","path":"/v1/auth/login","status":500,"duration_ms":"310.439566ms","user_agent":"Mozilla/5.0 (Windows NT 10.0; Win64; x64)..."}
```

### API Key Creation Without Token
```
{"level":"debug","message":"RequireAuth: checking authorization header","has_auth_header":false}
{"level":"warn","message":"RequireAuth: missing authorization header"}
{"level":"info","status":401}
```

## Next Steps to Debug

1. **Check for panic/recovery in logs**: The 500 error might be from a panic that's being recovered
2. **Compare request payloads**: Check what curl sends vs what browser sends
3. **Check frontend build**: Ensure web portal is rebuilt with latest changes
4. **Add error logging**: Ensure login handler logs errors before returning 500
5. **Test login endpoint directly**: Use browser dev tools to inspect the actual request/response

## Files Modified

- `web/portal/src/providers/AuthProvider.tsx` - Updated to use user-org-service directly
- `web/portal/src/features/admin/api/apiKeys.ts` - Already updated to check for token
- `services/user-org-service/internal/httpapi/middleware/auth.go` - Updated to zap logging
- `services/user-org-service/internal/httpapi/auth/handlers.go` - Already has verbose logging

## Test Status

- ✅ Backend health check: PASSING
- ✅ curl login: PASSING  
- ✅ curl API key creation: PASSING
- ❌ Browser login: FAILING (500 error)
- ❌ Browser API key creation: FAILING (401 - no token from failed login)
- ❌ E2E test: FAILING (timeout waiting for API key creation modal)

