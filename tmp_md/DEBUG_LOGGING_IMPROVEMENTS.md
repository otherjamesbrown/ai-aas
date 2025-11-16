# Debug Logging and CORS Improvements

## Summary

Enhanced the user-org-service HTTP server with comprehensive request/response logging and improved CORS handling to help debug UI connection issues.

## Changes Made

### 1. ✅ Comprehensive Request/Response Logging

**Location:** `services/user-org-service/internal/server/server.go`

**Features:**
- **Incoming Request Logging**: Logs method, path, query params, remote address, request ID, origin, user agent, and key headers
- **Response Logging**: Logs method, path, status code, duration, request ID, and CORS origin
- **Status Code Capture**: Wraps ResponseWriter to capture actual HTTP status codes

**Log Format:**
```
incoming request method=POST path=/v1/auth/login request_id=abc123 origin=http://localhost:5173
request completed method=POST path=/v1/auth/login status=200 duration_ms=45 request_id=abc123 cors_origin=http://localhost:5173
```

### 2. ✅ Improved CORS Handling

**Improvements:**
- **CORS on Error Responses**: Added CORS headers to 404 (NotFound) and 405 (MethodNotAllowed) responses
- **Consistent CORS Logic**: Extracted CORS logic into helper functions for consistency
- **Better Header Support**: Added `X-API-Key` to allowed headers
- **CORS Logging**: Added debug logging for CORS preflight requests

**Key Changes:**
- NotFound handler now adds CORS headers before returning 404
- MethodNotAllowed handler adds CORS headers before returning 405
- Both handlers return JSON error responses with route information

### 3. ✅ Route Debugging Endpoint

**New Endpoint:** `GET /debug/routes`

**Purpose:** Lists all registered routes for debugging

**Response Format:**
```json
{
  "routes": [
    {"method": "GET", "route": "/healthz"},
    {"method": "POST", "route": "/v1/auth/login"},
    ...
  ],
  "count": 15
}
```

**Usage:**
```bash
curl http://localhost:8081/debug/routes
```

### 4. ✅ Enhanced Error Responses

**404 (NotFound) Response:**
```json
{
  "error": "route not found",
  "method": "GET",
  "path": "/v1/support/impersonations/current"
}
```

**405 (MethodNotAllowed) Response:**
```json
{
  "error": "method not allowed",
  "method": "OPTIONS",
  "path": "/v1/auth/login"
}
```

Both responses include CORS headers so the browser can read the error message.

## How to Use for Debugging

### 1. Check Service Logs

The service now logs every request with detailed information:

```bash
# View service logs
tail -f /tmp/user-org-service.log

# Or if running in foreground
# Logs will appear in stdout
```

**Example Log Output:**
```
{"level":"info","method":"OPTIONS","path":"/v1/auth/login","query":"","remote_addr":"127.0.0.1:12345","request_id":"abc-123","origin":"http://localhost:5173","message":"incoming request"}
{"level":"debug","method":"OPTIONS","path":"/v1/auth/login","origin":"http://localhost:5173","message":"CORS preflight request handled"}
{"level":"info","method":"OPTIONS","path":"/v1/auth/login","status":204,"duration_ms":1,"request_id":"abc-123","cors_origin":"http://localhost:5173","message":"request completed"}
```

### 2. Check Registered Routes

```bash
curl http://localhost:8081/debug/routes | jq
```

This will show you all available routes and help identify if a route is missing.

### 3. Test CORS Headers

```bash
# Test OPTIONS preflight
curl -X OPTIONS http://localhost:8081/v1/auth/login \
  -H "Origin: http://localhost:5173" \
  -H "Access-Control-Request-Method: POST" \
  -v

# Check response headers
# Should see: Access-Control-Allow-Origin: http://localhost:5173
```

### 4. Test Missing Routes

```bash
# Test a missing route (should return 404 with CORS headers)
curl -X GET http://localhost:8081/v1/support/impersonations/current \
  -H "Origin: http://localhost:5173" \
  -v

# Should see:
# - Status: 404
# - Access-Control-Allow-Origin: http://localhost:5173
# - JSON error response
```

## Common Issues and Solutions

### Issue: CORS Missing Allow Origin

**Symptoms:**
- Browser console shows "CORS header 'Access-Control-Allow-Origin' missing"
- Status code 404 or 405

**Solution:**
- Check logs to see if the request is reaching the server
- Verify the Origin header matches allowed origins (localhost:5173)
- Check if the route exists using `/debug/routes`

### Issue: Route Not Found (404)

**Symptoms:**
- Status 404 with JSON error response
- Route not in `/debug/routes` output

**Solution:**
- Check if the route needs to be registered
- Verify the route path matches exactly (case-sensitive)
- Check if the route requires authentication

### Issue: Method Not Allowed (405)

**Symptoms:**
- Status 405 with JSON error response
- Route exists but method doesn't match

**Solution:**
- Check `/debug/routes` to see what methods are allowed
- Verify the HTTP method matches the route definition
- Check if OPTIONS is handled correctly for CORS preflight

## Log Levels

The logging uses different levels:
- **Info**: Normal request/response logging
- **Debug**: CORS preflight requests
- **Warn**: 404/405 errors (route not found, method not allowed)

To see debug logs, set `LOG_LEVEL=debug` in your environment.

## Next Steps

1. **Restart the service** to pick up the new logging
2. **Check logs** when the UI makes requests
3. **Use `/debug/routes`** to verify routes are registered
4. **Test CORS** with curl to verify headers are set correctly

## Files Modified

- `services/user-org-service/internal/server/server.go`
  - Added comprehensive request/response logging
  - Improved CORS handling for error responses
  - Added NotFound and MethodNotAllowed handlers with CORS
  - Added `/debug/routes` endpoint
  - Added `responseWriter` type for status code capture

## Testing

After restarting the service, you should see:
1. Detailed logs for every request
2. CORS headers on all responses (including errors)
3. JSON error responses for 404/405
4. Route debugging endpoint available

The UI should now be able to see proper error messages even when routes are missing or methods don't match.
