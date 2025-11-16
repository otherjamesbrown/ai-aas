# Request/Response Logging Examples

The new logging middleware is now active and creating detailed logs for all HTTP requests. Here are examples of what you'll see:

## Log Format

### Incoming Request Log
```json
{
  "level": "info",
  "service": "user-org-service-admin-api",
  "method": "GET",
  "path": "/healthz",
  "query": "",
  "remote_addr": "[::1]:39334",
  "request_id": "localhost/wKk0LB1i2K-000001",
  "origin": "http://localhost:5173",
  "user_agent": "curl/8.14.1",
  "time": "2025-11-15T12:49:34Z",
  "message": "incoming request"
}
```

### Request Completed Log
```json
{
  "level": "info",
  "service": "user-org-service-admin-api",
  "method": "GET",
  "path": "/healthz",
  "status": 200,
  "duration_ms": 0.038321,
  "request_id": "localhost/wKk0LB1i2K-000002",
  "cors_origin": "http://localhost:5173",
  "time": "2025-11-15T12:49:34Z",
  "message": "request completed"
}
```

### Route Not Found (404) Log
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

Followed by:
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

## What Gets Logged

### For Every Request:
- **Method**: HTTP method (GET, POST, OPTIONS, etc.)
- **Path**: Request path
- **Query**: Query string parameters
- **Remote Address**: Client IP and port
- **Request ID**: Unique request identifier (for tracing)
- **Origin**: CORS origin header (if present)
- **User Agent**: Browser/client user agent (if present)
- **Status Code**: HTTP response status
- **Duration**: Request processing time in milliseconds
- **CORS Origin**: CORS header value in response (if set)

### Special Cases:
- **404 (Route Not Found)**: Warning log with route details
- **405 (Method Not Allowed)**: Warning log with method/path details
- **OPTIONS Preflight**: Debug log for CORS preflight requests

## Viewing Logs

### Real-time Log Monitoring
```bash
# Follow logs in real-time
tail -f /tmp/user-org-service.log

# Filter for request logs only
tail -f /tmp/user-org-service.log | jq 'select(.message == "incoming request" or .message == "request completed")'

# Filter for errors/warnings
tail -f /tmp/user-org-service.log | jq 'select(.level == "warn" or .level == "error")'
```

### Search Logs
```bash
# Find all 404 errors
cat /tmp/user-org-service.log | jq 'select(.status == 404)'

# Find requests from specific origin
cat /tmp/user-org-service.log | jq 'select(.origin == "http://localhost:5173")'

# Find slow requests (>100ms)
cat /tmp/user-org-service.log | jq 'select(.duration_ms > 100)'

# Find CORS-related requests
cat /tmp/user-org-service.log | jq 'select(.cors_origin != null)'
```

### Pretty Print Logs
```bash
# Format logs for readability
cat /tmp/user-org-service.log | jq -r 'select(.message == "incoming request" or .message == "request completed") | "\(.time) [\(.level)] \(.method) \(.path) -> \(.status // "N/A") (\(.duration_ms // "N/A")ms) [\(.request_id)]"'
```

## Example Log Output

Here's what you'll see when the UI makes requests:

```
2025-11-15T12:49:34Z [info] incoming request | Method: GET | Path: /healthz | Origin: http://localhost:5173
2025-11-15T12:49:34Z [info] request completed | Method: GET | Path: /healthz | Status: 200 | Duration: 0.038ms | CORS: http://localhost:5173

2025-11-15T12:49:36Z [info] incoming request | Method: OPTIONS | Path: /v1/auth/login | Origin: http://localhost:5173
2025-11-15T12:49:36Z [info] request completed | Method: OPTIONS | Path: /v1/auth/login | Status: 204 | Duration: 0.012ms | CORS: http://localhost:5173

2025-11-15T12:49:36Z [info] incoming request | Method: GET | Path: /v1/support/impersonations/current | Origin: http://localhost:5173
2025-11-15T12:49:36Z [warn] route not found | Method: GET | Path: /v1/support/impersonations/current
2025-11-15T12:49:36Z [info] request completed | Method: GET | Path: /v1/support/impersonations/current | Status: 404 | Duration: 0.043ms | CORS: http://localhost:5173
```

## Debugging UI Issues

When debugging CORS or connection issues:

1. **Check if requests are reaching the server**:
   ```bash
   tail -f /tmp/user-org-service.log | grep "incoming request"
   ```

2. **Check CORS headers**:
   ```bash
   tail -f /tmp/user-org-service.log | jq 'select(.cors_origin != null)'
   ```

3. **Check for 404/405 errors**:
   ```bash
   tail -f /tmp/user-org-service.log | jq 'select(.status == 404 or .status == 405)'
   ```

4. **Check request origins**:
   ```bash
   tail -f /tmp/user-org-service.log | jq 'select(.origin != null) | {path: .path, origin: .origin, status: .status}'
   ```

## Log Location

- **Local Development**: `/tmp/user-org-service.log`
- **Production**: Check service logs via Kubernetes/Docker logs
- **Format**: JSON (structured logging via zerolog)

## Performance Impact

The logging middleware is lightweight:
- Minimal overhead (~0.01-0.05ms per request)
- Logs are written asynchronously
- No blocking operations

## Next Steps

1. Monitor logs when UI makes requests
2. Use request IDs to trace requests across services
3. Check CORS origin values to verify allowed origins
4. Use duration_ms to identify slow requests
5. Use status codes to identify errors (404, 405, 500, etc.)
