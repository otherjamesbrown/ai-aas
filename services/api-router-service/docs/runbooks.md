# API Router Service Incident Response Runbook

**Service**: API Router Service  
**Last Updated**: 2025-01-27  
**Owner**: Platform Engineering Team

## Overview

This runbook provides step-by-step procedures for responding to incidents affecting the API Router Service, including service startup/shutdown, health check interpretation, backend degradation, routing policy updates, failover recovery, buffer store issues, and connectivity problems.

## Quick Reference

### Key Endpoints

- **Inference API**: `POST /v1/inference`
- **Health Check**: `GET /v1/status/healthz`
- **Readiness Check**: `GET /v1/status/readyz`
- **Metrics**: `GET /metrics`
- **Admin - Backend Health**: `GET /v1/admin/routing/backends/{backendID}/health`
- **Admin - List Backends**: `GET /v1/admin/routing/backends`
- **Admin - Mark Degraded**: `POST /v1/admin/routing/backends/{backendID}/degrade`
- **Admin - Mark Healthy**: `POST /v1/admin/routing/backends/{backendID}/healthy`
- **Admin - Update Policy**: `POST /v1/admin/routing/policies`
- **Audit Lookup**: `GET /v1/audit/requests/{requestId}`

### Alert Thresholds

- **Error Rate**: > 5% for 5 minutes (critical)
- **Latency P95**: > 3s for 10 minutes (warning)
- **Latency P99**: > 5s for 5 minutes (critical)
- **Failover Rate**: > 10/min for 5 minutes (critical)
- **Backend Unhealthy**: Any backend unhealthy for 5 minutes (warning)
- **Rate Limit Denials**: > 100/min for 5 minutes (warning)
- **Buffer Store Size**: > 1000 records (critical)
- **Buffer Store Age**: > 24 hours (critical)

## Service Lifecycle Procedures

### 1. Service Startup

**Purpose**: Start the API Router Service with all dependencies

#### Prerequisites

1. Verify dependencies are running:
   ```bash
   # Check Redis
   redis-cli -h localhost -p 6379 ping
   
   # Check Kafka (if configured)
   kafka-broker-api-versions --bootstrap-server localhost:9092
   
   # Check Config Service (etcd)
   etcdctl endpoint health
   ```

2. Verify configuration:
   ```bash
   # Check environment variables
   env | grep -E "(REDIS|KAFKA|CONFIG|BACKEND)"
   
   # Verify backend endpoints are configured
   echo $BACKEND_ENDPOINTS
   ```

#### Startup Steps

1. **Start the service**:
   ```bash
   # Local development
   make run
   
   # Production (Kubernetes)
   kubectl rollout restart deployment/api-router-service -n api-router
   ```

2. **Verify startup**:
   ```bash
   # Check logs for initialization
   kubectl logs -f deployment/api-router-service -n api-router
   
   # Look for:
   # - "starting API router service"
   # - "Redis connected for rate limiting"
   # - "Kafka publisher initialized" (if configured)
   # - "backend registry initialized"
   # - "HTTP server starting"
   ```

3. **Verify health endpoints**:
   ```bash
   # Health check
   curl http://localhost:8080/v1/status/healthz
   # Expected: {"status":"healthy"}
   
   # Readiness check
   curl http://localhost:8080/v1/status/readyz
   # Expected: {"status":"ready","components":{...}}
   ```

4. **Verify metrics endpoint**:
   ```bash
   curl http://localhost:8080/metrics | grep api_router
   # Should see various metrics exported
   ```

#### Expected Outcomes

- Service starts without errors
- Health endpoint returns 200 OK
- Readiness endpoint returns 200 OK with all components healthy
- Metrics endpoint exports Prometheus metrics
- Service can route inference requests

### 2. Service Shutdown

**Purpose**: Gracefully shut down the service

#### Shutdown Steps

1. **Drain in-flight requests** (if using load balancer):
   ```bash
   # Kubernetes: Remove from service endpoints
   kubectl scale deployment/api-router-service --replicas=0 -n api-router
   
   # Wait for pods to terminate gracefully (10s timeout)
   kubectl wait --for=delete pod -l app=api-router-service -n api-router --timeout=30s
   ```

2. **Verify graceful shutdown**:
   ```bash
   # Check logs for:
   # - "shutting down gracefully"
   # - "API router service stopped"
   # - No errors during shutdown
   ```

3. **Verify cleanup**:
   ```bash
   # Check that connections are closed
   # - Redis connections closed
   # - Kafka publisher closed
   # - HTTP server stopped
   ```

#### Expected Outcomes

- All in-flight requests complete or timeout gracefully
- Connections to dependencies are closed cleanly
- No errors in shutdown logs
- Service stops within 10 seconds

## Health Check Interpretation

### Health Endpoint (`/v1/status/healthz`)

**Purpose**: Basic liveness check

**Response Codes**:
- `200 OK`: Service is running
- Any other code: Service is not responding

**Response Format**:
```json
{
  "status": "healthy",
  "timestamp": "2025-01-27T12:00:00Z"
}
```

**Interpretation**:
- `status: "healthy"`: Service is alive and responding
- If non-200: Service may be crashed or not started

### Readiness Endpoint (`/v1/status/readyz`)

**Purpose**: Component-level readiness check

**Response Codes**:
- `200 OK`: All critical components are healthy
- `503 Service Unavailable`: One or more components are degraded

**Response Format** (Healthy):
```json
{
  "status": "ready",
  "components": {
    "redis": "healthy",
    "kafka": "healthy",
    "config_service": "healthy",
    "backend_registry": "healthy"
  },
  "build": {
    "version": "1.0.0",
    "commit": "abc123",
    "build_time": "2025-01-27T10:00:00Z"
  },
  "timestamp": "2025-01-27T12:00:00Z"
}
```

**Response Format** (Degraded):
```json
{
  "status": "degraded",
  "components": {
    "redis": "unhealthy",
    "kafka": "healthy",
    "config_service": "healthy",
    "backend_registry": "healthy"
  },
  ...
}
```

**Component Status Values**:
- `healthy`: Component is operational
- `unhealthy`: Component is not operational (critical)
- `not_configured`: Component is not configured (non-critical)

**Interpretation**:
- All components `healthy`: Service is fully operational
- Any component `unhealthy`: Service is degraded, investigate that component
- `redis: "not_configured"`: Rate limiting disabled (acceptable)
- `kafka: "not_configured"`: Usage tracking disabled (acceptable)
- `config_service: "unhealthy"`: Using cache fallback (may be acceptable)
- `backend_registry: "unhealthy"`: No backends configured (critical)

## Common Incidents

### 1. Backend Degradation

**Alert**: `BackendUnhealthy` or `HighFailoverRate`  
**Severity**: Warning/Critical  
**Symptoms**: Increased failover events, degraded backend health status

#### Investigation Steps

1. **Check backend health status**:
   ```bash
   # Via admin API
   curl http://localhost:8080/v1/admin/routing/backends/{backendID}/health
   
   # Via metrics
   curl http://localhost:8080/metrics | grep router_backend_health_status
   ```

2. **Review backend metrics**:
   ```bash
   # Check error rate
   curl http://localhost:8080/metrics | grep api_router_backend_errors_total
   
   # Check latency
   curl http://localhost:8080/metrics | grep api_router_backend_request_duration_seconds
   ```

3. **Check Grafana dashboard**:
   - Review "Backend Health Status" panel
   - Check "Error Rate by Backend" panel
   - Review "Backend Latency" panel

4. **Check backend service directly**:
   ```bash
   # Test backend health endpoint
   curl http://backend-service:8001/health
   
   # Check backend logs
   kubectl logs -f deployment/backend-service -n backend
   ```

#### Resolution Steps

1. **Mark backend as degraded** (temporary):
   ```bash
   curl -X POST http://localhost:8080/v1/admin/routing/backends/{backendID}/degrade \
     -H "Content-Type: application/json" \
     -d '{"reason": "High error rate detected"}'
   ```
   - This excludes the backend from routing
   - Traffic will route to other backends

2. **Investigate backend issue**:
   - Check backend service logs
   - Verify backend service health
   - Check network connectivity
   - Review backend resource usage (CPU, memory)

3. **Fix backend issue**:
   - Restart backend service if needed
   - Scale backend if resource-constrained
   - Fix configuration issues
   - Resolve network issues

4. **Mark backend as healthy** (after fix):
   ```bash
   curl -X POST http://localhost:8080/v1/admin/routing/backends/{backendID}/healthy \
     -H "Content-Type: application/json"
   ```
   - Backend will be re-included in routing
   - Health monitor will verify recovery

#### Expected Outcomes

- Backend marked as degraded immediately
- Traffic routes to healthy backends
- Backend issue resolved
- Backend marked healthy and traffic restored
- Failover rate returns to normal

### 2. Routing Policy Update

**Purpose**: Update routing policies without service restart

#### Update Steps

1. **Review current policy**:
   ```bash
   curl http://localhost:8080/v1/admin/routing/policies/{orgID}/{model}
   ```

2. **Update policy via admin API**:
   ```bash
   curl -X POST http://localhost:8080/v1/admin/routing/policies \
     -H "Content-Type: application/json" \
     -d '{
       "policy_id": "policy-123",
       "organization_id": "org-456",
       "model": "gpt-4o",
       "backends": [
         {"backend_id": "backend-1", "weight": 80},
         {"backend_id": "backend-2", "weight": 20}
       ],
       "failover_threshold": 3
     }'
   ```

3. **Verify policy update**:
   ```bash
   # Check policy was updated
   curl http://localhost:8080/v1/admin/routing/policies/{orgID}/{model}
   
   # Monitor routing decisions
   curl http://localhost:8080/v1/admin/routing/decisions | jq '.decisions[] | select(.backend_id == "backend-1")'
   ```

4. **Monitor routing metrics**:
   - Check Grafana dashboard "Routing Decisions" panel
   - Verify traffic distribution matches new weights
   - Monitor for increased failover events

#### Rollback Steps

If the update causes issues:

1. **Revert to previous policy**:
   ```bash
   # Restore previous policy configuration
   curl -X POST http://localhost:8080/v1/admin/routing/policies \
     -H "Content-Type: application/json" \
     -d '{...previous_policy...}'
   ```

2. **Or update via Config Service**:
   ```bash
   # Update in etcd (if Config Service is available)
   etcdctl put /api-router/policies/{orgID}/{model} '{...policy_json...}'
   ```

#### Expected Outcomes

- Policy updated without service restart
- Traffic distribution changes according to new weights
- No increase in error rate or failover events
- Routing decisions reflect new policy

### 3. Failover Recovery

**Alert**: `HighFailoverRate`  
**Severity**: Critical  
**Symptoms**: High rate of failover events, multiple backends failing

#### Investigation Steps

1. **Check failover rate**:
   ```bash
   curl http://localhost:8080/metrics | grep router_failover_total
   ```

2. **Review routing decisions**:
   ```bash
   curl http://localhost:8080/v1/admin/routing/decisions | jq '.decisions[] | select(.decision_type == "FAILOVER")'
   ```

3. **Check backend health**:
   ```bash
   curl http://localhost:8080/v1/admin/routing/backends
   ```

4. **Review backend errors**:
   ```bash
   curl http://localhost:8080/metrics | grep api_router_backend_errors_total
   ```

#### Resolution Steps

1. **Identify failing backends**:
   - Review backend health status
   - Check error rates by backend
   - Identify common error patterns

2. **Mark degraded backends**:
   ```bash
   for backend in backend-1 backend-2; do
     curl -X POST http://localhost:8080/v1/admin/routing/backends/$backend/degrade \
       -H "Content-Type: application/json" \
       -d '{"reason": "High failover rate"}'
   done
   ```

3. **Investigate root cause**:
   - Check if all backends are affected (upstream issue)
   - Check if specific backends are affected (backend-specific issue)
   - Review recent deployments or configuration changes

4. **Fix underlying issue**:
   - If upstream: Fix upstream service
   - If backend-specific: Fix individual backends
   - If configuration: Revert or fix configuration

5. **Restore backends**:
   ```bash
   for backend in backend-1 backend-2; do
     # Wait for backend to recover
     curl http://backend-service:8001/health
     
     # Mark as healthy
     curl -X POST http://localhost:8080/v1/admin/routing/backends/$backend/healthy \
       -H "Content-Type: application/json"
   done
   ```

#### Expected Outcomes

- Degraded backends excluded from routing
- Failover rate decreases
- Root cause identified and fixed
- Backends restored and failover rate returns to normal

### 4. Buffer Store Recovery

**Alert**: `BufferStoreSizeHigh` or `BufferStoreAgeHigh`  
**Severity**: Critical  
**Symptoms**: High number of buffered records, old records in buffer

#### Investigation Steps

1. **Check buffer store size**:
   ```bash
   curl http://localhost:8080/metrics | grep api_router_buffer_store_size
   ```

2. **Check buffer store age**:
   ```bash
   curl http://localhost:8080/metrics | grep api_router_buffer_store_age_seconds
   ```

3. **Check Kafka connectivity**:
   ```bash
   # Check readiness endpoint
   curl http://localhost:8080/v1/status/readyz | jq '.components.kafka'
   
   # Test Kafka connectivity
   kafka-broker-api-versions --bootstrap-server localhost:9092
   ```

4. **Check usage record publish metrics**:
   ```bash
   curl http://localhost:8080/metrics | grep api_router_usage_records_published_total
   ```

#### Resolution Steps

1. **If Kafka is unavailable**:
   ```bash
   # Check Kafka service status
   kubectl get pods -n kafka
   kubectl logs -f deployment/kafka -n kafka
   
   # Restore Kafka connectivity
   kubectl rollout restart deployment/kafka -n kafka
   ```

2. **Verify Kafka recovery**:
   ```bash
   # Wait for Kafka to be healthy
   kafka-broker-api-versions --bootstrap-server localhost:9092
   
   # Check readiness endpoint
   curl http://localhost:8080/v1/status/readyz | jq '.components.kafka'
   ```

3. **Monitor buffer drain**:
   ```bash
   # Watch buffer size decrease
   watch -n 5 'curl -s http://localhost:8080/metrics | grep api_router_buffer_store_size'
   
   # Check publish rate increase
   watch -n 5 'curl -s http://localhost:8080/metrics | grep api_router_usage_records_published_total'
   ```

4. **If buffer doesn't drain**:
   ```bash
   # Check retry metrics
   curl http://localhost:8080/metrics | grep api_router_buffer_store_retries_total
   
   # Check for publish errors
   curl http://localhost:8080/metrics | grep 'api_router_usage_records_published_total{status="error"}'
   
   # Review service logs for publish errors
   kubectl logs -f deployment/api-router-service -n api-router | grep -i "publish\|kafka"
   ```

5. **Manual retry** (if needed):
   - Restart the service to trigger buffer retry
   - Or implement manual retry mechanism (if available)

#### Expected Outcomes

- Kafka connectivity restored
- Buffer store size decreases
- Usage records publish successfully
- Buffer store age decreases
- All buffered records eventually published

### 5. Kafka Connectivity Issues

**Alert**: `KafkaUnhealthy` or `BufferStoreSizeHigh`  
**Severity**: Critical  
**Symptoms**: Kafka marked unhealthy in readiness check, records buffering

#### Investigation Steps

1. **Check Kafka service status**:
   ```bash
   kubectl get pods -n kafka
   kubectl describe pod/kafka-0 -n kafka
   ```

2. **Check Kafka connectivity**:
   ```bash
   kafka-broker-api-versions --bootstrap-server localhost:9092
   ```

3. **Check network connectivity**:
   ```bash
   # From router pod
   kubectl exec -it deployment/api-router-service -n api-router -- \
     nc -zv kafka-service.kafka.svc.cluster.local 9092
   ```

4. **Check Kafka logs**:
   ```bash
   kubectl logs -f deployment/kafka -n kafka
   ```

#### Resolution Steps

1. **If Kafka pod is down**:
   ```bash
   # Restart Kafka
   kubectl rollout restart deployment/kafka -n kafka
   
   # Wait for Kafka to be ready
   kubectl wait --for=condition=ready pod -l app=kafka -n kafka --timeout=5m
   ```

2. **If network issue**:
   ```bash
   # Check NetworkPolicy
   kubectl get networkpolicy -n api-router
   
   # Check service endpoints
   kubectl get endpoints kafka-service -n kafka
   ```

3. **If configuration issue**:
   ```bash
   # Check Kafka broker configuration
   kubectl get configmap kafka-config -n kafka -o yaml
   
   # Verify KAFKA_BROKERS environment variable
   kubectl get deployment/api-router-service -n api-router -o yaml | grep KAFKA
   ```

4. **Verify recovery**:
   ```bash
   # Check readiness endpoint
   curl http://localhost:8080/v1/status/readyz | jq '.components.kafka'
   
   # Monitor buffer drain
   watch -n 5 'curl -s http://localhost:8080/metrics | grep api_router_buffer_store_size'
   ```

#### Expected Outcomes

- Kafka service restored
- Kafka connectivity verified
- Readiness check shows Kafka healthy
- Buffer store drains successfully

### 6. Config Service (etcd) Connectivity Issues

**Alert**: `ConfigServiceUnhealthy`  
**Severity**: Warning  
**Symptoms**: Config Service marked unhealthy, using cache fallback

#### Investigation Steps

1. **Check etcd service status**:
   ```bash
   kubectl get pods -n etcd
   etcdctl endpoint health
   ```

2. **Check etcd connectivity**:
   ```bash
   # From router pod
   kubectl exec -it deployment/api-router-service -n api-router -- \
     nc -zv etcd-service.etcd.svc.cluster.local 2379
   ```

3. **Check readiness endpoint**:
   ```bash
   curl http://localhost:8080/v1/status/readyz | jq '.components.config_service'
   ```

4. **Verify cache fallback**:
   ```bash
   # Check if policies are still available
   curl http://localhost:8080/v1/admin/routing/policies/{orgID}/{model}
   ```

#### Resolution Steps

1. **If etcd is down**:
   ```bash
   # Restart etcd
   kubectl rollout restart deployment/etcd -n etcd
   
   # Wait for etcd to be ready
   kubectl wait --for=condition=ready pod -l app=etcd -n etcd --timeout=5m
   ```

2. **If network issue**:
   ```bash
   # Check NetworkPolicy
   kubectl get networkpolicy -n api-router
   
   # Check service endpoints
   kubectl get endpoints etcd-service -n etcd
   ```

3. **Verify recovery**:
   ```bash
   # Check readiness endpoint
   curl http://localhost:8080/v1/status/readyz | jq '.components.config_service'
   
   # Verify policy updates resume
   # Update a policy in etcd and verify it's picked up
   ```

#### Expected Outcomes

- etcd service restored
- Config Service connectivity verified
- Readiness check shows Config Service healthy
- Policy updates resume (if watch was enabled)

**Note**: Service can operate with cache fallback, but policy updates won't be picked up until connectivity is restored.

### 7. Rate Limit Troubleshooting

**Alert**: `RateLimitDenialsHigh`  
**Severity**: Warning  
**Symptoms**: High rate of 429 responses, rate limit denials

#### Investigation Steps

1. **Check rate limit denials**:
   ```bash
   curl http://localhost:8080/metrics | grep api_router_rate_limit_denials_total
   ```

2. **Check Redis connectivity**:
   ```bash
   # Check readiness endpoint
   curl http://localhost:8080/v1/status/readyz | jq '.components.redis'
   
   # Test Redis
   redis-cli -h localhost -p 6379 ping
   ```

3. **Check rate limit configuration**:
   ```bash
   # Check environment variables
   kubectl get deployment/api-router-service -n api-router -o yaml | grep RATE_LIMIT
   ```

4. **Review rate limit metrics by organization**:
   ```bash
   curl http://localhost:8080/metrics | grep api_router_rate_limit_denials_total
   ```

#### Resolution Steps

1. **If Redis is unavailable**:
   ```bash
   # Rate limiting is disabled when Redis is unavailable
   # Check if this is expected behavior
   # If rate limiting is required, restore Redis connectivity
   ```

2. **If rate limits are too low**:
   ```bash
   # Update rate limit configuration
   # Edit deployment to increase RATE_LIMIT_DEFAULT_RPS or RATE_LIMIT_BURST_SIZE
   kubectl edit deployment/api-router-service -n api-router
   
   # Or update via Config Service (if supported)
   ```

3. **If specific organization is hitting limits**:
   ```bash
   # Check which organization is hitting limits
   curl http://localhost:8080/metrics | grep api_router_rate_limit_denials_total
   
   # Contact organization to reduce request rate
   # Or increase limits for that organization (if supported)
   ```

#### Expected Outcomes

- Rate limit denials decrease
- Redis connectivity restored (if needed)
- Rate limits adjusted appropriately
- Service operates within acceptable denial rate

### 8. Budget Enforcement Troubleshooting

**Alert**: `BudgetDenialsHigh` or `QuotaDenialsHigh`  
**Severity**: Warning  
**Symptoms**: High rate of 402 responses, budget/quota denials

#### Investigation Steps

1. **Check budget/quota denials**:
   ```bash
   curl http://localhost:8080/metrics | grep api_router_budget_denials_total
   curl http://localhost:8080/metrics | grep api_router_quota_denials_total
   ```

2. **Check budget service connectivity**:
   ```bash
   # Check if budget service is configured
   kubectl get deployment/api-router-service -n api-router -o yaml | grep BUDGET_SERVICE
   
   # Test budget service (if endpoint known)
   curl http://budget-service:8082/health
   ```

3. **Review denial metrics by organization**:
   ```bash
   curl http://localhost:8080/metrics | grep api_router_budget_denials_total
   ```

#### Resolution Steps

1. **If budget service is unavailable**:
   ```bash
   # Check budget service status
   kubectl get pods -n budget-service
   kubectl logs -f deployment/budget-service -n budget-service
   
   # Restore budget service
   kubectl rollout restart deployment/budget-service -n budget-service
   ```

2. **If organization has exhausted budget**:
   ```bash
   # This is expected behavior
   # Contact organization to:
   # - Increase budget allocation
   # - Reduce usage
   # - Wait for budget reset period
   ```

3. **If budget service returns incorrect data**:
   ```bash
   # Escalate to budget service team
   # Check budget service logs
   # Verify budget calculations
   ```

#### Expected Outcomes

- Budget service connectivity restored (if needed)
- Budget denials are expected for organizations that exceeded limits
- Budget service returns correct data
- Organizations can request budget increases if needed

## 15-Minute Recovery Procedures

### Scenario: Complete Service Outage

**Goal**: Restore service within 15 minutes

#### Steps (Time-boxed)

1. **0-2 minutes: Assess situation**
   ```bash
   # Check service status
   kubectl get pods -n api-router
   kubectl logs -f deployment/api-router-service -n api-router --tail=100
   
   # Check health endpoints
   curl http://localhost:8080/v1/status/healthz
   curl http://localhost:8080/v1/status/readyz
   ```

2. **2-5 minutes: Identify root cause**
   - Check readiness endpoint components
   - Review recent deployments
   - Check dependency services (Redis, Kafka, etcd)
   - Review error logs

3. **5-10 minutes: Apply fix**
   - Restart service if needed
   - Restore dependency connectivity
   - Fix configuration issues
   - Mark degraded backends if needed

4. **10-15 minutes: Verify recovery**
   ```bash
   # Verify health endpoints
   curl http://localhost:8080/v1/status/healthz
   curl http://localhost:8080/v1/status/readyz
   
   # Test inference request
   curl -X POST http://localhost:8080/v1/inference \
     -H "X-API-Key: test-key" \
     -H "Content-Type: application/json" \
     -d '{"request_id":"test-123","model":"gpt-4o","payload":"test"}'
   
   # Monitor metrics
   curl http://localhost:8080/metrics | grep api_router_backend_requests_total
   ```

#### Escalation

If service cannot be restored within 15 minutes:
- Escalate to Platform Engineering Manager
- Consider rolling back to previous deployment
- Activate incident response team

## Monitoring and Dashboards

### Grafana Dashboard

- **Operational Dashboard**: `/dashboards/api-router-operational`
- **Location**: `deployments/helm/api-router-service/dashboards/api-router.json`

### Key Metrics to Monitor

1. **Request Rate**: `rate(api_router_backend_requests_total[5m])`
2. **Latency P95**: `histogram_quantile(0.95, api_router_backend_request_duration_seconds)`
3. **Error Rate**: `rate(api_router_backend_errors_total[5m]) / rate(api_router_backend_requests_total[5m])`
4. **Failover Rate**: `rate(router_failover_total[5m])`
5. **Backend Health**: `router_backend_health_status`
6. **Rate Limit Denials**: `rate(api_router_rate_limit_denials_total[5m])`
7. **Budget Denials**: `rate(api_router_budget_denials_total[5m])`
8. **Buffer Store Size**: `api_router_buffer_store_size`
9. **Buffer Store Age**: `api_router_buffer_store_age_seconds`

## Escalation

### On-Call Rotation

- **Primary**: Platform Engineering Team
- **Secondary**: Backend Engineering Team
- **Escalation**: Engineering Manager

### When to Escalate

- Service is completely down
- Cannot restore service within 15 minutes
- Data loss detected (usage records)
- Buffer store size > 10,000 records
- Multiple backends failing simultaneously
- Security incident suspected

### Escalation Contacts

- **Platform On-Call**: Check PagerDuty
- **Engineering Manager**: Check team directory
- **Security Team**: security@company.com (for security incidents)

## Post-Incident

### Required Actions

1. **Document Timeline**: Record incident start/end times, key events
2. **Root Cause Analysis**: Identify root cause and contributing factors
3. **Update Runbook**: Add any new procedures discovered during incident
4. **Create Incident Report**: Document findings, impact, and improvements
5. **Review Metrics**: Analyze metrics during incident window

### Common Follow-ups

- Review alert thresholds
- Improve error handling
- Enhance monitoring coverage
- Update documentation
- Implement preventive measures
- Review capacity planning

## Related Documentation

- [API Router Service README](../README.md)
- [API Router Service Spec](../../specs/006-api-router-service/spec.md)
- [Infrastructure Overview](../../docs/platform/infrastructure-overview.md)
- [Observability Guide](../../docs/platform/observability-guide.md)
- [Grafana Dashboard](../deployments/helm/api-router-service/dashboards/api-router.json)

