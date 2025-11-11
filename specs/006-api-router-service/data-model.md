# Data Model: API Router Service

## Overview

The router coordinates ingress authentication, routing decisions, quota enforcement, and usage accounting. Entities below describe the logical data structures exchanged between services and persisted in shared systems (Redis, Kafka, configuration cache).

## Entities

### RouteRequest
- **request_id** (UUID, required): Client-supplied idempotency token.  
- **organization_id** (UUID, required): Owning organization.  
- **api_key_id** (UUID, required): Identifier of the API key used.  
- **model** (string, required): Requested model alias.  
- **parameters** (JSON object, optional): Model-specific parameters (temperature, max_tokens, etc.).  
- **payload** (bytes/string, required): Prompt/input body (<=64 KB).  
- **metadata** (map<string,string>, optional): Client-provided tags for audit/analytics.  
- **received_at** (timestamp, server-assigned).  
- **replay_nonce** (string, optional): HMAC nonce used for signature validation.

Relationships: referenced by `RoutingDecision`, `UsageRecord`, `AuditEvent`.

### RoutingPolicy
- **policy_id** (UUID, required).  
- **organization_id** (UUID or `*` for global).  
- **model** (string).  
- **weight_map** (map<backend_id, int>, required): Distribution weights (sum normalized).  
- **failover_threshold** (int, required): Consecutive failures before shifting traffic.  
- **degraded_backends** (set<backend_id>): Backends marked degraded (auto-failover).  
- **allow_list** / **deny_list** (set<api_key_id>): Optional gating per key.  
- **updated_at** (timestamp).  
- **version** (int64): Monotonic version for watch updates.

Relationships: Cached by router replicas; referenced when producing `RoutingDecision`.

### BackendEndpoint
- **backend_id** (UUID, required).  
- **model_variant** (string, required): Backend-specific alias.  
- **uri** (string, required): gRPC/HTTP endpoint.  
- **health_status** (enum: HEALTHY, DEGRADED, UNAVAILABLE).  
- **latency_p95_ms** (int).  
- **error_rate_pct** (float).  
- **capacity_rps** (int).  
- **last_probe_at** (timestamp).  
- **labels** (map<string,string>): Region, provider, tier classifications.

Relationships: Observed by health monitor; referenced in `RoutingDecision` and metrics.

### RoutingDecision
- **decision_id** (UUID).  
- **request_id** (UUID).  
- **selected_backend_id** (UUID).  
- **policy_version** (int64).  
- **reason** (enum: PRIMARY, FAILOVER, OVERRIDE, RATE_LIMIT).  
- **latency_budget_ms** (int).  
- **decision_duration_ms** (int).  
- **created_at** (timestamp).

Stored transiently for logging/metrics; included in usage/audit records.

### BudgetConstraint
- **organization_id** (UUID, required).  
- **period** (enum: DAILY, MONTHLY).  
- **token_limit** (int64).  
- **currency_limit** (decimal).  
- **reset_at** (timestamp).  
- **current_tokens_used** (int64).  
- **current_currency_used** (decimal).  
- **enforced_at** (timestamp).  
- **source** (string): Derived from finance system snapshot.

Relationships: Retrieved from budget service; informs limiter decisions.

### RateLimitWindow
- **organization_id** (UUID).  
- **api_key_id** (UUID).  
- **limit_name** (string).  
- **window_seconds** (int).  
- **max_tokens** (int).  
- **tokens_remaining** (int).  
- **window_reset_at** (timestamp).  
Stored in Redis; updated atomically per request.

### UsageRecord
- **record_id** (UUID).  
- **request_id** (UUID).  
- **organization_id** (UUID).  
- **api_key_id** (UUID).  
- **model** (string).  
- **backend_id** (UUID).  
- **tokens_input** / **tokens_output** (int).  
- **latency_ms** (int).  
- **cost_usd** (decimal).  
- **limit_state** (enum: WITHIN_LIMIT, RATE_LIMITED, BUDGET_EXCEEDED).  
- **decision_reason** (from `RoutingDecision.reason`).  
- **timestamp** (timestamp).  
- **trace_id** / **span_id** (string).  
Published to Kafka and consumed by analytics/finance services.

### AuditEvent
- **event_id** (UUID).  
- **request_id** (UUID).  
- **actor** (string: API key or admin user).  
- **action** (enum: REQUEST_ALLOWED, REQUEST_DENIED, POLICY_UPDATE, LIMIT_RESET).  
- **metadata** (JSON).  
- **created_at** (timestamp).  
Indexed for 90-day retention by compliance services.

## Relationships Overview

- `RouteRequest` → `RoutingDecision` (1:1)  
- `RoutingDecision` → `BackendEndpoint` (many:1)  
- `RouteRequest` → `UsageRecord` (1:1)  
- `BudgetConstraint` & `RateLimitWindow` inform limiter decisions before routing.  
- `AuditEvent` references `RouteRequest` and limiter outcomes for traceability.
