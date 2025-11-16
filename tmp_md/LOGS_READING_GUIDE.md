# Reading the New Request/Response Logs

## ✅ Logging is Active!

The new comprehensive request/response logging middleware is now working. Here's what you can see in the logs:

## Log Location

**File**: `/tmp/user-org-service.log`

## What's Being Logged

### 1. **Incoming Request Logs**
Every HTTP request generates an "incoming request" log entry with:
- Method (GET, POST, OPTIONS, etc.)
- Path (e.g., `/v1/auth/login`)
- Query string
- Remote address (client IP and port)
- Request ID (for tracing)
- Origin (CORS origin header)
- User Agent (browser/client)

**Example**:
```json
{
  "level": "info",
  "service": "user-org-service-admin-api",
  "method": "GET",
  "path": "/v1/support/impersonations/current",
  "query": "",
  "remote_addr": "127.0.0.1:38300",
  "request_id": "localhost/wKk0LB1i2K-000004",
  "origin": "http://localhost:5173",
  "user_agent": "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:145.0) Gecko/20100101 Firefox/145.0",
  "time": "2025-11-15T12:49:46Z",
  "message": "incoming request"
}
```

### 2. **Request Completed Logs**
Every response generates a "request completed" log entry with:
- Method and path
- Status code (200, 404, 405, etc.)
- Duration in milliseconds
- Request ID (matches incoming request)
- CORS origin (if CORS headers were added)

**Example**:
```json
{
  "level": "info",
  "service": "user-org-service-admin-api",
  "method": "GET",
  "path": "/v1/support/impersonations/current",
  "status": 404,
  "duration_ms": 0.043341,
  "request_id": "localhost/wKk0LB1i2K-000003",
  "cors_origin": "http://localhost:5173",
  "time": "2025-11-15T12:49:36Z",
  "message": "request completed"
}
```

### 3. **Error/Warning Logs**
Special logs for errors:
- **404 (Route Not Found)**: Warning log before the response
- **405 (Method Not Allowed)**: Warning log before the response

**Example (404)**:
```json
{
  "level": "warn",
  "service": "user-org-service-admin-api",
  "method": "GET",
  "path": "/v1/support/impersonations/current",
  "request_id": "localhost/wKk0LB1i2K-000003",
  "time": "2025-11-15T12:49:36Z",
  "message": "route not found"
}
```

**Example (405)**:
```json
{
  "level": "warn",
  "service": "user-org-service-admin-api",
  "method": "POST",
  "path": "/healthz",
  "request_id": "localhost/wKk0LB1i2K-000007",
  "time": "2025-11-15T12:50:10Z",
  "message": "method not allowed"
}
```

## Real Examples from Current Logs

### Successful Request (200)
```
incoming: GET /healthz from http://localhost:5173
completed: GET /healthz -> 200 (0.038ms) [CORS: http://localhost:5173]
```

### Missing Route (404)
```
incoming: GET /v1/support/impersonations/current from http://localhost:5173
WARNING: route not found - GET /v1/support/impersonations/current
completed: GET /v1/support/impersonations/current -> 404 (0.043ms) [CORS: http://localhost:5173]
```

### Method Not Allowed (405)
```
incoming: POST /healthz from http://localhost:5173
WARNING: method not allowed - POST /healthz
completed: POST /healthz -> 405 (0.043ms) [CORS: http://localhost:5173]
```

### Debug Routes Endpoint
```
incoming: GET /debug/routes from http://localhost:5173
completed: GET /debug/routes -> 200 (0.225ms) [CORS: http://localhost:5173]
```

## How to Read Logs

### View All Recent Requests
```bash
tail -f /tmp/user-org-service.log | grep -E "(incoming request|request completed|route not found|method not allowed)"
```

### View Only Errors/Warnings
```bash
tail -f /tmp/user-org-service.log | grep -E "(route not found|method not allowed)"
```

### View Requests from UI (localhost:5173)
```bash
tail -f /tmp/user-org-service.log | grep "http://localhost:5173"
```

### View Slow Requests (>10ms)
```bash
cat /tmp/user-org-service.log | grep "request completed" | grep -E "duration_ms\":[1-9][0-9]"
```

### View All 404 Errors
```bash
cat /tmp/user-org-service.log | grep '"status":404'
```

### View All 405 Errors
```bash
cat /tmp/user-org-service.log | grep '"status":405'
```

## Current Log Statistics

From the current log file:
- **Total log lines**: 23
- **Request logs**: 14 (incoming + completed)
- **Error logs**: 4 (404 and 405 warnings)

## What This Tells Us

From the logs, we can see:

1. ✅ **CORS is working**: All responses show `cors_origin: "http://localhost:5173"` when origin is present
2. ✅ **404 errors include CORS**: The `/v1/support/impersonations/current` route returns 404 with CORS headers
3. ✅ **405 errors include CORS**: POST to GET-only routes return 405 with CORS headers
4. ✅ **Request tracking**: Every request has a unique `request_id` for tracing
5. ✅ **Performance**: All requests complete in <1ms (very fast)

## Debugging UI Issues

When the UI makes requests, you'll see:

1. **If request reaches server**: Look for "incoming request" log
2. **If CORS is applied**: Check for `cors_origin` in "request completed" log
3. **If route exists**: Check for "route not found" warning
4. **If method matches**: Check for "method not allowed" warning
5. **Response status**: Check `status` field in "request completed" log

## Example: Debugging the UI Error

From the UI error you showed:
- Request: `GET /v1/support/impersonations/current`
- Error: CORS Missing Allow Origin

**What the logs show**:
```json
{"level":"info","method":"GET","path":"/v1/support/impersonations/current","origin":"http://localhost:5173","message":"incoming request"}
{"level":"warn","method":"GET","path":"/v1/support/impersonations/current","message":"route not found"}
{"level":"info","method":"GET","path":"/v1/support/impersonations/current","status":404,"cors_origin":"http://localhost:5173","message":"request completed"}
```

**Analysis**:
- ✅ Request is reaching the server
- ✅ CORS headers ARE being added (`cors_origin` is present)
- ⚠️ Route doesn't exist (404)
- ✅ CORS headers are on the 404 response

The CORS issue should now be resolved! The route `/v1/support/impersonations/current` doesn't exist, but at least the browser can now read the 404 error message because CORS headers are present.

## Next Steps

1. **Monitor logs in real-time** when testing the UI:
   ```bash
   tail -f /tmp/user-org-service.log
   ```

2. **Check for missing routes**:
   ```bash
   curl http://localhost:8081/debug/routes | jq
   ```

3. **Verify CORS on all responses**:
   - All responses should have `cors_origin` in logs
   - All error responses (404/405) should include CORS headers

The logging is now comprehensive and will help debug any UI connection issues!
