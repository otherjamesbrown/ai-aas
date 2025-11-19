# vLLM API Router Integration Plan

**Date**: 2025-11-19
**Status**: Planning
**Goal**: Expose vLLM inference API externally via API Router Service

## Current State

### vLLM Deployment
- **Service**: `vllm-gpt-oss-20b.system:8000` (ClusterIP)
- **Model**: unsloth/gpt-oss-20b (20B parameters)
- **GPU**: NVIDIA RTX 4000 Ada (20GB VRAM)
- **Status**: ✅ Running and healthy
- **API**: OpenAI-compatible endpoints
- **Access**: Internal cluster only

### API Router Service
- **Location**: `/home/dev/ai-aas/services/api-router-service`
- **Type**: Go-based API gateway
- **Current Exposure**: External (likely via Ingress/LoadBalancer)
- **Role**: Central gateway for all API requests

## Goals

### Primary Objectives
1. Route external inference requests to vLLM service
2. Maintain OpenAI-compatible API surface
3. Add authentication and rate limiting
4. Enable monitoring and logging
5. Support multiple model backends (future-proof)

### Non-Goals (For Initial Release)
- Load balancing across multiple vLLM instances
- Model routing logic (A/B testing, canary)
- Custom prompt templates
- Fine-tuning endpoints

## Architecture

### Request Flow

```
External Client
    ↓
    ↓ HTTPS
    ↓
Ingress/LoadBalancer
    ↓
    ↓ HTTP
    ↓
API Router Service (api-router-service)
    ↓
    ↓ Authentication/Rate Limiting
    ↓ Request Transformation
    ↓ Logging/Metrics
    ↓
    ↓ HTTP (internal)
    ↓
vLLM Service (vllm-gpt-oss-20b.system:8000)
    ↓
    ↓ GPU Inference
    ↓
Response → API Router → Client
```

### Components

#### 1. API Router Service
**Responsibilities:**
- Accept external requests on `/v1/chat/completions`, `/v1/completions`, `/v1/models`
- Validate authentication (API keys)
- Apply rate limiting per user/API key
- Transform requests if needed
- Proxy to vLLM backend
- Add request/response logging
- Collect metrics (latency, tokens, errors)

**Configuration:**
- vLLM backend URL: `http://vllm-gpt-oss-20b.system:8000`
- Timeout: 120s (for long inference requests)
- Retry policy: 2 retries with exponential backoff

#### 2. vLLM Service
**Responsibilities:**
- Serve OpenAI-compatible API
- Perform GPU inference
- Handle model loading and caching

**No changes required** - already operational

#### 3. External Exposure
**Options:**

**Option A: Ingress (Recommended)**
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: api-router-ingress
spec:
  rules:
  - host: api.yourdomain.com
    http:
      paths:
      - path: /v1
        pathType: Prefix
        backend:
          service:
            name: api-router-service
            port:
              number: 8080
```

**Option B: LoadBalancer**
```yaml
apiVersion: v1
kind: Service
metadata:
  name: api-router-service
spec:
  type: LoadBalancer
  ports:
  - port: 443
    targetPort: 8080
```

## Implementation Plan

### Phase 1: API Router Backend Configuration (Day 1)

#### Step 1.1: Add vLLM Backend Configuration
**File**: `services/api-router-service/config/config.yaml` (or environment variables)

```yaml
backends:
  vllm:
    enabled: true
    url: "http://vllm-gpt-oss-20b.system:8000"
    timeout: 120s
    max_retries: 2
    models:
      - "unsloth/gpt-oss-20b"
      - "gpt-oss-20b"  # Alias
    endpoints:
      - "/v1/chat/completions"
      - "/v1/completions"
      - "/v1/models"
```

#### Step 1.2: Implement Backend Proxy Logic
**File**: `services/api-router-service/internal/handlers/inference.go`

```go
type InferenceHandler struct {
    vllmClient *http.Client
    vllmURL    string
    logger     *log.Logger
    metrics    *metrics.Collector
}

func (h *InferenceHandler) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
    // 1. Validate authentication
    apiKey := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
    if !h.validateAPIKey(apiKey) {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // 2. Apply rate limiting
    if !h.rateLimiter.Allow(apiKey) {
        http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
        return
    }

    // 3. Parse and validate request
    var req ChatCompletionRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    // 4. Log request
    h.logger.Info("Inference request", "model", req.Model, "user", getUserFromAPIKey(apiKey))

    // 5. Proxy to vLLM
    startTime := time.Now()
    resp, err := h.proxyToVLLM(r.Context(), "/v1/chat/completions", req)
    duration := time.Since(startTime)

    // 6. Record metrics
    h.metrics.RecordInference(req.Model, duration, err == nil)

    // 7. Return response
    if err != nil {
        http.Error(w, "Inference failed", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}
```

#### Step 1.3: Add Route Registration
**File**: `services/api-router-service/internal/router/router.go`

```go
func SetupRoutes(r *mux.Router, handlers *handlers.Handlers) {
    // Existing routes...

    // vLLM inference endpoints
    r.HandleFunc("/v1/chat/completions", handlers.Inference.HandleChatCompletions).Methods("POST")
    r.HandleFunc("/v1/completions", handlers.Inference.HandleCompletions).Methods("POST")
    r.HandleFunc("/v1/models", handlers.Inference.HandleListModels).Methods("GET")
}
```

### Phase 2: Authentication & Authorization (Day 1-2)

#### Step 2.1: API Key Management
**File**: `services/api-router-service/internal/auth/apikey.go`

```go
type APIKeyService struct {
    store APIKeyStore  // Database or in-memory
}

func (s *APIKeyService) ValidateAPIKey(key string) (*User, error) {
    // Validate format (e.g., "sk-...")
    if !strings.HasPrefix(key, "sk-") {
        return nil, errors.New("invalid API key format")
    }

    // Check against database/store
    user, err := s.store.GetUserByAPIKey(key)
    if err != nil {
        return nil, err
    }

    // Check if key is active
    if !user.Active {
        return nil, errors.New("API key inactive")
    }

    return user, nil
}
```

#### Step 2.2: Rate Limiting
**File**: `services/api-router-service/internal/ratelimit/ratelimit.go`

```go
type RateLimiter struct {
    limiters sync.Map  // map[apiKey]*rate.Limiter
}

func (rl *RateLimiter) Allow(apiKey string) bool {
    limiter := rl.getLimiter(apiKey)
    return limiter.Allow()
}

func (rl *RateLimiter) getLimiter(apiKey string) *rate.Limiter {
    if limiter, ok := rl.limiters.Load(apiKey); ok {
        return limiter.(*rate.Limiter)
    }

    // Create new limiter: 10 requests per minute
    limiter := rate.NewLimiter(rate.Every(6*time.Second), 10)
    // Use LoadOrStore to prevent race condition - ensures atomicity
    actualLimiter, _ := rl.limiters.LoadOrStore(apiKey, limiter)
    return actualLimiter.(*rate.Limiter)
}
```

### Phase 3: External Exposure (Day 2)

#### Step 3.1: Update API Router Service
**File**: `infra/k8s/api-router-service.yaml`

Ensure the service is properly configured:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: api-router-service
  namespace: system
spec:
  type: ClusterIP
  ports:
  - name: http
    port: 8080
    targetPort: 8080
  selector:
    app: api-router-service
```

#### Step 3.2: Create Ingress
**File**: `infra/k8s/api-router-ingress.yaml`

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: api-router-ingress
  namespace: system
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"  # If using cert-manager
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "120"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - api.yourdomain.com
    secretName: api-router-tls
  rules:
  - host: api.yourdomain.com
    http:
      paths:
      - path: /v1
        pathType: Prefix
        backend:
          service:
            name: api-router-service
            port:
              number: 8080
      - path: /health
        pathType: Exact
        backend:
          service:
            name: api-router-service
            port:
              number: 8080
```

#### Step 3.3: Install Ingress Controller (if not present)
```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install ingress-nginx ingress-nginx/ingress-nginx \
  -n ingress-nginx --create-namespace \
  --set controller.service.type=LoadBalancer
```

### Phase 4: Monitoring & Logging (Day 2-3)

#### Step 4.1: Add Prometheus Metrics
**File**: `services/api-router-service/internal/metrics/metrics.go`

```go
var (
    inferenceRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "inference_requests_total",
            Help: "Total number of inference requests",
        },
        []string{"model", "endpoint", "status"},
    )

    inferenceLatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "inference_latency_seconds",
            Help: "Inference request latency",
            Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
        },
        []string{"model", "endpoint"},
    )

    tokensProcessed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "tokens_processed_total",
            Help: "Total tokens processed",
        },
        []string{"model", "type"},  // type: input or output
    )
)

func RecordInference(model, endpoint string, duration time.Duration, success bool) {
    status := "success"
    if !success {
        status = "error"
    }

    inferenceRequestsTotal.WithLabelValues(model, endpoint, status).Inc()
    inferenceLatency.WithLabelValues(model, endpoint).Observe(duration.Seconds())
}
```

#### Step 4.2: Structured Logging
**File**: `services/api-router-service/internal/logging/logging.go`

```go
func LogInferenceRequest(logger *slog.Logger, req *InferenceRequest, resp *InferenceResponse, duration time.Duration) {
    logger.Info("inference_request",
        slog.String("model", req.Model),
        slog.Int("input_tokens", resp.Usage.PromptTokens),
        slog.Int("output_tokens", resp.Usage.CompletionTokens),
        slog.Float64("duration_seconds", duration.Seconds()),
        slog.String("user", req.User),
    )
}
```

### Phase 5: Testing (Day 3)

#### Test Cases

**1. Basic Functionality**
```bash
# Test chat completions
curl -X POST https://api.yourdomain.com/v1/chat/completions \
  -H "Authorization: Bearer sk-test-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-20b",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 50
  }'
```

**2. Authentication**
```bash
# Test without API key - should fail with 401
curl -X POST https://api.yourdomain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-oss-20b", "messages": [{"role": "user", "content": "Test"}]}'
```

**3. Rate Limiting**
```bash
# Send 15 requests rapidly - should get 429 after 10th request
for i in {1..15}; do
  curl -X POST https://api.yourdomain.com/v1/chat/completions \
    -H "Authorization: Bearer sk-test-key" \
    -H "Content-Type: application/json" \
    -d '{"model": "gpt-oss-20b", "messages": [{"role": "user", "content": "Test"}]}'
  sleep 1
done
```

**4. Model Listing**
```bash
curl https://api.yourdomain.com/v1/models \
  -H "Authorization: Bearer sk-test-key"
```

**5. Error Handling**
```bash
# Test with invalid model
curl -X POST https://api.yourdomain.com/v1/chat/completions \
  -H "Authorization: Bearer sk-test-key" \
  -H "Content-Type: application/json" \
  -d '{"model": "invalid-model", "messages": [{"role": "user", "content": "Test"}]}'
```

**6. Load Testing**
```bash
# Use Apache Bench or k6
ab -n 100 -c 10 -T 'application/json' \
  -H 'Authorization: Bearer sk-test-key' \
  -p request.json \
  https://api.yourdomain.com/v1/chat/completions
```

## Configuration Files

### Environment Variables
```bash
# API Router Service
VLLM_BACKEND_URL=http://vllm-gpt-oss-20b.system:8000
VLLM_TIMEOUT=120s
VLLM_MAX_RETRIES=2

# Authentication
API_KEY_VALIDATION_ENABLED=true
RATE_LIMIT_REQUESTS_PER_MINUTE=10

# Monitoring
PROMETHEUS_ENABLED=true
PROMETHEUS_PORT=9090
LOG_LEVEL=info
```

### Kubernetes ConfigMap
**File**: `infra/k8s/api-router-configmap.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: api-router-config
  namespace: system
data:
  config.yaml: |
    backends:
      vllm:
        enabled: true
        url: "http://vllm-gpt-oss-20b.system:8000"
        timeout: "120s"
        max_retries: 2
        models:
          - "unsloth/gpt-oss-20b"
          - "gpt-oss-20b"

    auth:
      enabled: true
      api_key_prefix: "sk-"

    rate_limit:
      enabled: true
      requests_per_minute: 10
      burst: 20

    monitoring:
      prometheus: true
      metrics_port: 9090
```

## Deployment Steps

### Step-by-Step Execution

#### 1. Update API Router Code
```bash
cd /home/dev/ai-aas/services/api-router-service

# Create new handlers and middleware
# (implement code from Phase 1-2 above)

# Run tests
make test

# Build Docker image
make docker-build

# Push to registry
make docker-push
```

#### 2. Deploy Updated API Router
```bash
cd /home/dev/ai-aas

# Update ConfigMap
kubectl apply -f infra/k8s/api-router-configmap.yaml

# Update Deployment (will trigger rolling update)
kubectl apply -f infra/k8s/api-router-deployment.yaml

# Verify deployment
kubectl rollout status deployment/api-router-service -n system
```

#### 3. Install Ingress Controller (if needed)
```bash
# Check if ingress controller exists
kubectl get pods -n ingress-nginx

# If not, install it
helm install ingress-nginx ingress-nginx/ingress-nginx \
  -n ingress-nginx --create-namespace \
  --set controller.service.type=LoadBalancer
```

#### 4. Create Ingress Resource
```bash
# Apply ingress configuration
kubectl apply -f infra/k8s/api-router-ingress.yaml

# Get external IP
kubectl get ingress -n system api-router-ingress

# Update DNS to point to external IP
# (manual step in your DNS provider)
```

#### 5. Test End-to-End
```bash
# Wait for DNS propagation
# Test health endpoint
curl https://api.yourdomain.com/health

# Test inference
curl -X POST https://api.yourdomain.com/v1/chat/completions \
  -H "Authorization: Bearer sk-test-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-20b",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 50
  }'
```

## Rollback Plan

### If Deployment Fails

**Option 1: Rollback Deployment**
```bash
kubectl rollout undo deployment/api-router-service -n system
```

**Option 2: Use kubectl rollout undo (Recommended)**
```bash
# Rollback to previous revision
kubectl rollout undo deployment/api-router-service -n system

# Or rollback to specific revision
kubectl rollout undo deployment/api-router-service -n system --to-revision=2

# Verify rollback status
kubectl rollout status deployment/api-router-service -n system
```

**Note**: For GitOps workflows, consider reverting the Git commit and letting ArgoCD/Flux handle the rollback automatically.

### If Ingress Issues

**Remove Ingress**
```bash
kubectl delete ingress api-router-ingress -n system
```

**Temporary LoadBalancer**
```bash
kubectl patch service api-router-service -n system \
  -p '{"spec":{"type":"LoadBalancer"}}'
```

## Security Considerations

### 1. API Key Storage
- **DO NOT** store API keys in plaintext
- Use Kubernetes Secrets or external secret management (Vault, AWS Secrets Manager)
- Rotate API keys regularly

### 2. TLS/HTTPS
- Use cert-manager for automatic TLS certificate management
- Enforce HTTPS only (redirect HTTP to HTTPS)
- Use strong cipher suites

### 3. Network Policies
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: api-router-network-policy
  namespace: system
spec:
  podSelector:
    matchLabels:
      app: api-router-service
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: vllm
    ports:
    - protocol: TCP
      port: 8000
```

### 4. Input Validation
- Validate all request parameters
- Set maximum token limits
- Sanitize user inputs
- Implement request size limits

## Cost & Performance Considerations

### Resource Allocation
```yaml
# API Router Service
resources:
  requests:
    memory: "256Mi"
    cpu: "500m"
  limits:
    memory: "512Mi"
    cpu: "1000m"
```

### Scaling Strategy
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: api-router-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: api-router-service
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### Connection Pooling
```go
// Configure HTTP client for vLLM backend
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 100,
    IdleConnTimeout:     90 * time.Second,
}

client := &http.Client{
    Transport: transport,
    Timeout:   120 * time.Second,
}
```

## Success Criteria

- [ ] External API accessible via HTTPS
- [ ] Authentication working (API keys validated)
- [ ] Rate limiting functional
- [ ] Requests successfully routed to vLLM
- [ ] Response times < 5s for typical requests
- [ ] Error rate < 1%
- [ ] Metrics exposed and Prometheus scraping
- [ ] Logs structured and searchable
- [ ] TLS certificates valid
- [ ] Load testing passed (100 concurrent users)

## Timeline

- **Day 1**: Phases 1-2 (Backend configuration, authentication)
- **Day 2**: Phase 3 (External exposure, ingress setup)
- **Day 3**: Phases 4-5 (Monitoring, testing)
- **Day 4**: Final testing and production deployment

## References

- OpenAI API Documentation: https://platform.openai.com/docs/api-reference
- vLLM API Documentation: https://docs.vllm.ai/
- Kubernetes Ingress: https://kubernetes.io/docs/concepts/services-networking/ingress/
- Rate Limiting Patterns: https://cloud.google.com/architecture/rate-limiting-strategies-techniques

## Next Steps

1. Review and approve this plan
2. Create implementation tickets/tasks
3. Begin Phase 1 implementation
4. Set up development/staging environment for testing
5. Schedule production deployment window
