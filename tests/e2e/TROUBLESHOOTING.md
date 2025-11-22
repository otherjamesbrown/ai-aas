# E2E Test Troubleshooting Guide

This guide helps diagnose and resolve common issues when running end-to-end tests.

## Common Issues

### 1. DNS Resolution Failures

**Symptoms:**
```
dial tcp: lookup api.dev.ai-aas.local: no such host
```

**Solutions:**

**Option A: Use IP address directly**
```bash
export USER_ORG_SERVICE_URL=https://172.232.58.222
export API_ROUTER_SERVICE_URL=https://172.232.58.222
make test-dev-ip
```

**Option B: Add to /etc/hosts (requires sudo)**
```bash
echo "172.232.58.222 api.dev.ai-aas.local" | sudo tee -a /etc/hosts
```

**Option C: Use port-forwarding**
```bash
kubectl port-forward -n development svc/user-org-service 8081:8081
export USER_ORG_SERVICE_URL=http://localhost:8081
```

### 2. Authentication Failures

**Symptoms:**
```
status 401, body: {"error":"missing X-API-Key header","code":"AUTH_INVALID"}
```

**Solutions:**

1. **Bootstrap admin key:**
   ```bash
   cd tests/e2e
   make setup
   ```

2. **Set ADMIN_API_KEY manually:**
   ```bash
   export ADMIN_API_KEY=your-admin-api-key
   ```

3. **Check .admin-key.env:**
   ```bash
   cat tests/e2e/.admin-key.env
   ```

### 3. Ingress Routing Issues

**Symptoms:**
```
status 404, body: 404 page not found
```

**Solutions:**

1. **Verify Host header is set:**
   - The test client automatically sets Host header when using IP addresses
   - Check that `api.dev.ai-aas.local` is the correct hostname

2. **Verify ingress is configured:**
   ```bash
   kubectl get ingress -n development
   ```

3. **Check service endpoints:**
   ```bash
   kubectl get svc -n development
   ```

### 4. TLS/SSL Certificate Errors

**Symptoms:**
```
x509: certificate signed by unknown authority
```

**Solutions:**

1. **Skip TLS verification (development only):**
   - The test client automatically skips TLS verification for development
   - Verify `TEST_ENV=development` is set

2. **Use HTTP instead of HTTPS (if available):**
   ```bash
   export USER_ORG_SERVICE_URL=http://localhost:8081
   ```

### 5. Timeout Errors

**Symptoms:**
```
context deadline exceeded
```

**Solutions:**

1. **Increase timeout:**
   ```bash
   export REQUEST_TIMEOUT_MS=60000  # 60 seconds
   ```

2. **Check service health:**
   ```bash
   curl https://api.dev.ai-aas.local/health
   ```

3. **Verify network connectivity:**
   ```bash
   ping api.dev.ai-aas.local
   ```

### 6. Test Data Cleanup Failures

**Symptoms:**
```
Failed to cleanup resource: org_abc123
```

**Solutions:**

1. **Manual cleanup:**
   - Check test logs for resource IDs
   - Manually delete resources via API or admin interface

2. **Check fixture registration:**
   - Ensure fixtures are registered with `ctx.Fixtures.Register()`
   - Verify cleanup is called: `defer ctx.Cleanup()`

### 7. Parallel Execution Issues

**Symptoms:**
```
Test data leakage between parallel workers
```

**Solutions:**

1. **Use unique resource names:**
   ```go
   name := ctx.GenerateResourceName("org")
   ```

2. **Verify worker isolation:**
   - Each worker should have a unique `WORKER_ID`
   - Resources should be prefixed with worker ID

3. **Run sequentially:**
   ```bash
   PARALLEL_WORKERS=1 make test-dev
   ```

### 8. Mock Backend Issues

**Symptoms:**
```
Mock backend not responding
```

**Solutions:**

1. **Verify mock is started:**
   ```go
   mockBackend := utils.NewMockBackend()
   defer mockBackend.Close()  // Important!
   ```

2. **Check mock URL:**
   ```go
   url := mockBackend.URL()
   ```

3. **Verify mock response:**
   ```go
   mockBackend.SetResponse(utils.MockResponse{
       StatusCode: 200,
       Body: []byte(`{"status":"ok"}`),
   })
   ```

## Debugging Tips

### Enable Verbose Logging

```bash
go test -v ./suites/... -timeout 30m
```

### Check Test Artifacts

```bash
ls -la tests/e2e/artifacts/
cat tests/e2e/artifacts/request-*.json
```

### Inspect Correlation IDs

```go
corrID := utils.GenerateCorrelationID()
t.Logf("Correlation ID: %s", corrID)
// Use this ID to trace requests in service logs
```

### Verify Service Connectivity

```bash
# Check user-org-service
curl -k https://172.232.58.222/health \
  -H "Host: api.dev.ai-aas.local"

# Check api-router-service
curl -k https://172.232.58.222/v1/status/healthz \
  -H "Host: api.dev.ai-aas.local"
```

### Check Kubernetes Resources

```bash
# List pods
kubectl get pods -n development

# Check service logs
kubectl logs -n development deployment/user-org-service --tail=100

# Check ingress
kubectl describe ingress -n development
```

## Getting Help

1. **Check test logs:** Look for correlation IDs and error messages
2. **Review artifacts:** Check `tests/e2e/artifacts/` for request/response details
3. **Verify environment:** Ensure services are running and accessible
4. **Check documentation:** See `specs/012-e2e-tests/quickstart.md`

## Reporting Issues

When reporting issues, include:
- Test suite name and test function
- Error message and stack trace
- Correlation ID (if available)
- Environment details (`TEST_ENV`, service URLs)
- Test artifacts (request/response bodies)

