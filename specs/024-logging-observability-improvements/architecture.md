# Technical Architecture: Logging & Observability

## Component Specifications

### 1. Loki Stack Deployment

#### Promtail DaemonSet

```yaml
# Deployment target: infra/k8s/monitoring/promtail/
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: promtail
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: promtail
  template:
    spec:
      containers:
        - name: promtail
          image: grafana/promtail:2.9.0
          args:
            - -config.file=/etc/promtail/promtail.yaml
          volumeMounts:
            - name: logs
              mountPath: /var/log
            - name: containers
              mountPath: /var/lib/docker/containers
              readOnly: true
      volumes:
        - name: logs
          hostPath:
            path: /var/log
        - name: containers
          hostPath:
            path: /var/lib/docker/containers
```

#### Promtail Configuration

```yaml
server:
  http_listen_port: 9080

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki.monitoring.svc.cluster.local:3100/loki/api/v1/push

scrape_configs:
  - job_name: kubernetes-pods
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      # Keep only pods with logging enabled
      - source_labels: [__meta_kubernetes_pod_annotation_logging_enabled]
        action: keep
        regex: "true"
      # Extract labels
      - source_labels: [__meta_kubernetes_namespace]
        target_label: namespace
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: pod
      - source_labels: [__meta_kubernetes_pod_label_app]
        target_label: app
    pipeline_stages:
      # Parse JSON logs
      - json:
          expressions:
            level: level
            service: service
            trace_id: trace_id
            request_id: request_id
            message: msg
      # Extract level as label for filtering
      - labels:
          level:
          service:
      # Drop debug logs in production (configurable)
      - match:
          selector: '{level="debug"}'
          stages:
            - drop:
                expression: ".*"
                drop_counter_reason: debug_logs_filtered

  # vLLM / Inference Backend Logs
  - job_name: inference-backends
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - ai-models
            - system
            - kserve
    relabel_configs:
      # Only scrape pods with vllm or inference containers
      - source_labels: [__meta_kubernetes_pod_container_name]
        action: keep
        regex: "(vllm|kserve-container|inference|transformer)"
      # Extract labels
      - source_labels: [__meta_kubernetes_namespace]
        target_label: namespace
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: pod
      - source_labels: [__meta_kubernetes_pod_container_name]
        target_label: container
      # Extract model name from pod label or deployment
      - source_labels: [__meta_kubernetes_pod_label_serving_kserve_io_inferenceservice]
        target_label: model
      - source_labels: [__meta_kubernetes_pod_label_app]
        target_label: app
    pipeline_stages:
      # vLLM outputs mixed format - some JSON, some plain text
      # Try JSON first
      - json:
          expressions:
            level: level
            message: message
            model: model
            gpu_id: gpu_id
      # Fallback: detect log level from text patterns
      - match:
          selector: '{container=~"vllm|kserve-container"}'
          stages:
            - regex:
                expression: '(?P<level>INFO|WARNING|ERROR|DEBUG|CRITICAL):?\s*(?P<message>.*)'
            - labels:
                level:
      # Normalize log levels
      - template:
          source: level
          template: '{{ ToLower .Value }}'
      - labels:
          level:
      # Extract CUDA/GPU errors
      - match:
          selector: '{container=~"vllm|kserve-container"}'
          stages:
            - regex:
                expression: '(?i)(?P<gpu_error>CUDA|OutOfMemory|GPU|torch\.cuda)'
            - labels:
                gpu_error:
      # Extract model loading status
      - match:
          selector: '{container=~"vllm|kserve-container"}'
          stages:
            - regex:
                expression: '(?i)(?P<model_status>Loading model|Model loaded|Failed to load)'
            - labels:
                model_status:
```

#### Loki StatefulSet

```yaml
# Deployment target: infra/k8s/monitoring/loki/
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: loki
  namespace: monitoring
spec:
  serviceName: loki
  replicas: 1  # Single node for development, scale for production
  template:
    spec:
      containers:
        - name: loki
          image: grafana/loki:2.9.0
          args:
            - -config.file=/etc/loki/loki.yaml
          ports:
            - containerPort: 3100
          volumeMounts:
            - name: storage
              mountPath: /loki
  volumeClaimTemplates:
    - metadata:
        name: storage
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 10Gi  # Adjust based on retention needs
```

#### Loki Configuration

```yaml
auth_enabled: false

server:
  http_listen_port: 3100

ingester:
  lifecycler:
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1
  chunk_idle_period: 5m
  chunk_retain_period: 30s

schema_config:
  configs:
    - from: 2024-01-01
      store: boltdb-shipper
      object_store: filesystem
      schema: v11
      index:
        prefix: index_
        period: 24h

storage_config:
  boltdb_shipper:
    active_index_directory: /loki/index
    cache_location: /loki/cache
    shared_store: filesystem
  filesystem:
    directory: /loki/chunks

limits_config:
  enforce_metric_name: false
  reject_old_samples: true
  reject_old_samples_max_age: 168h  # 7 days
  ingestion_rate_mb: 10
  ingestion_burst_size_mb: 20

chunk_store_config:
  max_look_back_period: 336h  # 14 days

table_manager:
  retention_deletes_enabled: true
  retention_period: 336h  # 14 days
```

#### Loki Service

```yaml
# Deployment target: infra/k8s/monitoring/loki/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: loki
  namespace: monitoring
  labels:
    app: loki
spec:
  type: ClusterIP
  ports:
    - port: 3100
      targetPort: 3100
      protocol: TCP
      name: http
  selector:
    app: loki
```

#### Loki Ingress

```yaml
# Deployment target: infra/k8s/monitoring/loki/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: loki
  namespace: monitoring
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    # Optional: Add basic auth for security
    # nginx.ingress.kubernetes.io/auth-type: basic
    # nginx.ingress.kubernetes.io/auth-secret: loki-basic-auth
spec:
  ingressClassName: nginx
  rules:
    # Public DNS endpoint
    - host: loki.dev.otherjamesbrown.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: loki
                port:
                  number: 3100
    # Local DNS endpoint (requires /etc/hosts or DNS setup)
    - host: loki.dev.otherjamesbrown.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: loki
                port:
                  number: 3100
```

#### Grafana Ingress

```yaml
# Deployment target: infra/k8s/monitoring/grafana/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: grafana
  namespace: monitoring
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  ingressClassName: nginx
  rules:
    # Public DNS endpoint
    - host: grafana.dev.otherjamesbrown.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: grafana
                port:
                  number: 3000
    # Local DNS endpoint (requires /etc/hosts or DNS setup)
    - host: grafana.dev.otherjamesbrown.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: grafana
                port:
                  number: 3000
```

### 2. Request/Response Logging Middleware

#### Go Middleware Implementation

```go
// Target: shared/go/middleware/request_logger.go
package middleware

import (
    "bytes"
    "fmt"
    "io"
    "net/http"
    "regexp"
    "time"

    "go.uber.org/zap"
)

// RequestLoggerConfig configures the request logging middleware
type RequestLoggerConfig struct {
    // LogRequestBody enables request body logging (use with caution)
    LogRequestBody bool
    // LogResponseBody enables response body logging (use with caution)
    LogResponseBody bool
    // LogBodyOnError always logs request/response body on 4xx/5xx (recommended)
    LogBodyOnError bool
    // MaxBodyLogSize limits body logging size (default 1KB)
    MaxBodyLogSize int
    // SkipPaths excludes paths from logging (e.g., health checks)
    SkipPaths []string
    // SensitiveHeaders won't be logged
    SensitiveHeaders []string
    // SensitiveBodyFields fields to redact from body logs
    SensitiveBodyFields []string
}

// DefaultConfig returns sensible defaults
func DefaultConfig() RequestLoggerConfig {
    return RequestLoggerConfig{
        LogRequestBody:  false,
        LogResponseBody: false,
        LogBodyOnError:  true,  // Always log bodies on errors for debugging
        MaxBodyLogSize:  1024,
        SkipPaths:       []string{"/healthz", "/readyz", "/metrics"},
        SensitiveHeaders: []string{
            "Authorization",
            "X-API-Key",
            "Cookie",
            "Set-Cookie",
        },
        SensitiveBodyFields: []string{
            "password",
            "token",
            "secret",
            "api_key",
            "apiKey",
            "credit_card",
            "ssn",
        },
    }
}

// RequestLogger returns middleware that logs HTTP requests
func RequestLogger(logger *zap.Logger, config RequestLoggerConfig) func(http.Handler) http.Handler {
    skipSet := make(map[string]bool)
    for _, p := range config.SkipPaths {
        skipSet[p] = true
    }

    sensitiveSet := make(map[string]bool)
    for _, h := range config.SensitiveHeaders {
        sensitiveSet[h] = true
    }

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Skip configured paths
            if skipSet[r.URL.Path] {
                next.ServeHTTP(w, r)
                return
            }

            start := time.Now()

            // Extract request ID from context if present
            requestID := r.Header.Get("X-Request-ID")

            // Build safe headers map
            headers := make(map[string]string)
            for k, v := range r.Header {
                if !sensitiveSet[k] && len(v) > 0 {
                    headers[k] = v[0]
                }
            }

            // Capture request body if needed (for error logging)
            var requestBodySample string
            if config.LogRequestBody || config.LogBodyOnError {
                bodyBytes, _ := io.ReadAll(io.LimitReader(r.Body, int64(config.MaxBodyLogSize)))
                r.Body.Close()
                r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
                requestBodySample = redactSensitiveFields(string(bodyBytes), config.SensitiveBodyFields)
            }

            // Wrap response writer to capture status and body
            wrapped := &responseWriter{
                ResponseWriter:    w,
                statusCode:        200,
                captureBody:       config.LogResponseBody || config.LogBodyOnError,
                maxBodySize:       config.MaxBodyLogSize,
            }

            // Log request start
            logger.Debug("request_started",
                zap.String("method", r.Method),
                zap.String("path", r.URL.Path),
                zap.String("query", r.URL.RawQuery),
                zap.String("remote_addr", r.RemoteAddr),
                zap.String("request_id", requestID),
                zap.Any("headers", headers),
            )

            // Execute handler
            next.ServeHTTP(wrapped, r)

            // Log request completion
            duration := time.Since(start)

            logFunc := logger.Info
            isError := wrapped.statusCode >= 400
            if wrapped.statusCode >= 500 {
                logFunc = logger.Error
            } else if wrapped.statusCode >= 400 {
                logFunc = logger.Warn
            }

            // Build log fields
            fields := []zap.Field{
                zap.String("method", r.Method),
                zap.String("path", r.URL.Path),
                zap.Int("status", wrapped.statusCode),
                zap.Float64("duration_ms", float64(duration.Milliseconds())),
                zap.String("request_id", requestID),
                zap.Int64("response_bytes", wrapped.bytesWritten),
            }

            // Add body samples on errors for debugging
            if isError && config.LogBodyOnError {
                if requestBodySample != "" {
                    fields = append(fields, zap.String("request_body_sample", requestBodySample))
                }
                if wrapped.bodySample != "" {
                    responseBodySample := redactSensitiveFields(wrapped.bodySample, config.SensitiveBodyFields)
                    fields = append(fields, zap.String("response_body_sample", responseBodySample))
                }
            }

            logFunc("request_completed", fields...)
        })
    }
}

// redactSensitiveFields replaces sensitive field values with [REDACTED]
func redactSensitiveFields(body string, sensitiveFields []string) string {
    result := body
    for _, field := range sensitiveFields {
        // Simple JSON field redaction: "field": "value" -> "field": "[REDACTED]"
        patterns := []string{
            fmt.Sprintf(`"%s"\s*:\s*"[^"]*"`, field),
            fmt.Sprintf(`"%s"\s*:\s*'[^']*'`, field),
        }
        for _, pattern := range patterns {
            re := regexp.MustCompile(pattern)
            result = re.ReplaceAllString(result, fmt.Sprintf(`"%s": "[REDACTED]"`, field))
        }
    }
    return result
}

type responseWriter struct {
    http.ResponseWriter
    statusCode   int
    bytesWritten int64
    captureBody  bool
    maxBodySize  int
    bodySample   string
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
    n, err := rw.ResponseWriter.Write(b)
    rw.bytesWritten += int64(n)

    // Capture body sample for error logging
    if rw.captureBody && len(rw.bodySample) < rw.maxBodySize {
        remaining := rw.maxBodySize - len(rw.bodySample)
        if len(b) <= remaining {
            rw.bodySample += string(b)
        } else {
            rw.bodySample += string(b[:remaining])
        }
    }

    return n, err
}
```

### 3. Frontend Logger Implementation

```typescript
// Target: web/portal/src/lib/logger.ts

type LogLevel = 'debug' | 'info' | 'warn' | 'error';

interface LogContext {
  [key: string]: unknown;
}

interface LoggerConfig {
  level: LogLevel;
  service: string;
  environment: string;
  enableConsole: boolean;
}

const LOG_LEVELS: Record<LogLevel, number> = {
  debug: 0,
  info: 1,
  warn: 2,
  error: 3,
};

class Logger {
  private config: LoggerConfig;
  private traceId: string | null = null;

  constructor(config: Partial<LoggerConfig> = {}) {
    this.config = {
      level: (import.meta.env.VITE_LOG_LEVEL as LogLevel) || 'info',
      service: 'web-portal',
      environment: import.meta.env.MODE || 'development',
      enableConsole: import.meta.env.DEV,
      ...config,
    };
  }

  setTraceId(traceId: string) {
    this.traceId = traceId;
  }

  private shouldLog(level: LogLevel): boolean {
    return LOG_LEVELS[level] >= LOG_LEVELS[this.config.level];
  }

  private formatMessage(level: LogLevel, message: string, context?: LogContext): string {
    const entry = {
      timestamp: new Date().toISOString(),
      level,
      service: this.config.service,
      environment: this.config.environment,
      message,
      ...(this.traceId && { trace_id: this.traceId }),
      ...context,
    };
    return JSON.stringify(entry);
  }

  private log(level: LogLevel, message: string, context?: LogContext) {
    if (!this.shouldLog(level)) return;

    const formatted = this.formatMessage(level, message, context);

    // In development, use pretty console output
    if (this.config.enableConsole) {
      const consoleFn = level === 'error' ? console.error
        : level === 'warn' ? console.warn
        : level === 'debug' ? console.debug
        : console.log;
      consoleFn(`[${level.toUpperCase()}] ${message}`, context || '');
    }

    // In production, could send to logging endpoint
    if (this.config.environment === 'production' && level === 'error') {
      this.sendToBackend(formatted);
    }
  }

  private async sendToBackend(logEntry: string) {
    try {
      // Fire and forget - don't block on logging
      navigator.sendBeacon('/api/v1/logs', logEntry);
    } catch {
      // Silently fail - logging shouldn't break the app
    }
  }

  debug(message: string, context?: LogContext) {
    this.log('debug', message, context);
  }

  info(message: string, context?: LogContext) {
    this.log('info', message, context);
  }

  warn(message: string, context?: LogContext) {
    this.log('warn', message, context);
  }

  error(message: string, error?: Error, context?: LogContext) {
    this.log('error', message, {
      ...context,
      ...(error && {
        error_name: error.name,
        error_message: error.message,
        error_stack: error.stack,
      }),
    });
  }
}

// Singleton instance
export const logger = new Logger();

// React hook for component logging
export function useLogger(component: string) {
  return {
    debug: (msg: string, ctx?: LogContext) => logger.debug(msg, { component, ...ctx }),
    info: (msg: string, ctx?: LogContext) => logger.info(msg, { component, ...ctx }),
    warn: (msg: string, ctx?: LogContext) => logger.warn(msg, { component, ...ctx }),
    error: (msg: string, err?: Error, ctx?: LogContext) => logger.error(msg, err, { component, ...ctx }),
  };
}
```

### 4. Sentry Integration

```typescript
// Target: web/portal/src/lib/sentry.ts

import * as Sentry from '@sentry/react';
import { BrowserTracing } from '@sentry/tracing';

export function initSentry() {
  if (!import.meta.env.VITE_SENTRY_DSN) {
    console.warn('Sentry DSN not configured, error tracking disabled');
    return;
  }

  Sentry.init({
    dsn: import.meta.env.VITE_SENTRY_DSN,
    environment: import.meta.env.MODE,

    integrations: [
      new BrowserTracing({
        // Trace all requests to our API
        tracePropagationTargets: [
          'localhost',
          /^https:\/\/api\./,
        ],
      }),
      new Sentry.Replay({
        // Session replay for error reproduction
        maskAllText: false,
        blockAllMedia: false,
      }),
    ],

    // Performance monitoring
    tracesSampleRate: import.meta.env.PROD ? 0.1 : 1.0,

    // Session replay - only on errors
    replaysSessionSampleRate: 0,
    replaysOnErrorSampleRate: 1.0,

    // Filter out noise
    ignoreErrors: [
      // Browser extensions
      'ResizeObserver loop',
      // Network errors that aren't actionable
      'Network request failed',
      'Failed to fetch',
    ],

    beforeSend(event) {
      // Scrub sensitive data
      if (event.request?.headers) {
        delete event.request.headers['Authorization'];
        delete event.request.headers['X-API-Key'];
      }
      return event;
    },
  });
}

// Export for ErrorBoundary integration
export { Sentry };
```

### 5. Updated ErrorBoundary

```typescript
// Target: web/portal/src/components/ErrorBoundary.tsx (update)

import { Component, ErrorInfo, ReactNode } from 'react';
import { Sentry } from '@/lib/sentry';
import { logger } from '@/lib/logger';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
  errorId: string | null;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null, errorId: null };
  }

  static getDerivedStateFromError(error: Error): Partial<State> {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    // Log to our logger
    logger.error('React ErrorBoundary caught error', error, {
      componentStack: errorInfo.componentStack,
    });

    // Send to Sentry with context
    const errorId = Sentry.captureException(error, {
      contexts: {
        react: {
          componentStack: errorInfo.componentStack,
        },
      },
    });

    this.setState({ errorId });
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      return (
        <div className="error-boundary">
          <h2>Something went wrong</h2>
          <p>We've been notified and are looking into it.</p>
          {this.state.errorId && (
            <p className="error-id">
              Error ID: <code>{this.state.errorId}</code>
            </p>
          )}
          <button onClick={() => window.location.reload()}>
            Refresh Page
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
```

### 6. Grafana Dashboard Configuration

```json
{
  "title": "AI-AAS Service Logs",
  "panels": [
    {
      "title": "Log Volume by Service",
      "type": "timeseries",
      "targets": [
        {
          "expr": "sum by (service) (count_over_time({namespace=\"default\"}[5m]))",
          "legendFormat": "{{service}}"
        }
      ]
    },
    {
      "title": "Error Rate",
      "type": "stat",
      "targets": [
        {
          "expr": "sum(count_over_time({level=\"error\"}[1h]))"
        }
      ]
    },
    {
      "title": "Recent Errors",
      "type": "logs",
      "targets": [
        {
          "expr": "{level=\"error\"} | json"
        }
      ]
    },
    {
      "title": "Request Latency (p99)",
      "type": "timeseries",
      "targets": [
        {
          "expr": "histogram_quantile(0.99, sum by (le, service) (rate(http_request_duration_seconds_bucket[5m])))"
        }
      ]
    }
  ]
}
```

## Integration Points

### Service Integration Checklist

Each Go service needs:

1. **Import request logger middleware**
   ```go
   import "github.com/ai-aas/shared/go/middleware"
   ```

2. **Add to router chain**
   ```go
   r.Use(middleware.RequestLogger(logger, middleware.DefaultConfig()))
   ```

3. **Add pod annotation for Promtail**
   ```yaml
   annotations:
     logging.enabled: "true"
   ```

### Frontend Integration Checklist

1. **Install Sentry SDK**
   ```bash
   npm install @sentry/react @sentry/tracing
   ```

2. **Initialize in main.tsx**
   ```typescript
   import { initSentry } from '@/lib/sentry';
   initSentry();
   ```

3. **Replace console.log calls**
   ```typescript
   // Before
   console.log('User logged in', { userId });

   // After
   logger.info('User logged in', { userId });
   ```

4. **Configure environment variables**
   ```bash
   VITE_SENTRY_DSN=https://xxx@sentry.io/xxx
   VITE_LOG_LEVEL=info
   ```
