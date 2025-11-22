# Data Model: Load Testing Harness

This document defines the data structures used by the load testing harness.

## Configuration Model

### LoadTestConfig

The central configuration structure read from ConfigMap.

```go
// LoadTestConfig defines the complete configuration for a load test run
type LoadTestConfig struct {
    // Metadata
    APIVersion string `yaml:"apiVersion"` // loadtest.ai-aas.dev/v1
    Kind       string `yaml:"kind"`       // LoadTestScenario
    Metadata   struct {
        Name      string `yaml:"name"`
        Namespace string `yaml:"namespace"`
        TestRunID string `yaml:"testRunId"` // Auto-generated if not provided
    } `yaml:"metadata"`

    // Spec defines the test scenario
    Spec LoadTestSpec `yaml:"spec"`
}

// LoadTestSpec contains all test parameters
type LoadTestSpec struct {
    Organizations OrganizationConfig `yaml:"organizations"`
    Users         UserConfig         `yaml:"users"`
    APIKeys       APIKeyConfig       `yaml:"apiKeys"`
    UserBehavior  BehaviorConfig     `yaml:"userBehavior"`
    LoadPattern   LoadPatternConfig  `yaml:"loadPattern"`
    Workers       WorkerConfig       `yaml:"workers"`
    Targets       TargetConfig       `yaml:"targets"`
    Metrics       MetricsConfig      `yaml:"metrics"`
    Limits        LimitsConfig       `yaml:"limits"`
    Cleanup       CleanupConfig      `yaml:"cleanup"`
}

// OrganizationConfig defines how organizations are created
type OrganizationConfig struct {
    Count          int    `yaml:"count"`          // Total organizations to create
    NamingPattern  string `yaml:"namingPattern"`  // e.g., "loadtest-org-{index}"
    Distribution   string `yaml:"distribution"`   // "even", "weighted", "random"
}

// UserConfig defines user creation parameters
type UserConfig struct {
    PerOrg struct {
        Min          int    `yaml:"min"`          // Minimum users per org
        Max          int    `yaml:"max"`          // Maximum users per org
        Distribution string `yaml:"distribution"` // "random", "gaussian", "uniform"
    } `yaml:"perOrg"`
    NamingPattern string `yaml:"namingPattern"` // e.g., "user-{org}-{index}"
}

// APIKeyConfig defines API key generation
type APIKeyConfig struct {
    PerUser int `yaml:"perUser"` // Number of keys per user (typically 1-2)
}

// BehaviorConfig defines realistic user behavior
type BehaviorConfig struct {
    QuestionsPerSession struct {
        Min          int    `yaml:"min"`
        Max          int    `yaml:"max"`
        Distribution string `yaml:"distribution"` // "gaussian", "uniform", "poisson"
    } `yaml:"questionsPerSession"`

    ThinkTimeSeconds struct {
        Min          int    `yaml:"min"`          // Minimum seconds between questions
        Max          int    `yaml:"max"`          // Maximum seconds between questions
        Distribution string `yaml:"distribution"` // "exponential", "gaussian", "uniform"
    } `yaml:"thinkTimeSeconds"`

    QuestionStrategies []QuestionStrategy `yaml:"questionStrategies"`

    SessionDuration struct {
        Min string `yaml:"min"` // e.g., "5m"
        Max string `yaml:"max"` // e.g., "60m"
    } `yaml:"sessionDuration"`

    ModelPreferences []ModelPreference `yaml:"modelPreferences"`
}

// QuestionStrategy defines question generation approach
type QuestionStrategy struct {
    Name   string `yaml:"name"`   // "mixed", "technical", "historical", etc.
    Weight int    `yaml:"weight"` // Relative weight for random selection
}

// ModelPreference defines which models users request
type ModelPreference struct {
    Model  string `yaml:"model"`  // e.g., "gpt-4o"
    Weight int    `yaml:"weight"` // Relative preference
}

// LoadPatternConfig defines how load scales over time
type LoadPatternConfig struct {
    Type   string       `yaml:"type"`   // "ramp-up", "steady", "spike", "wave"
    Phases []LoadPhase  `yaml:"phases"` // Sequential phases
}

// LoadPhase represents a single phase in the load pattern
type LoadPhase struct {
    Name              string `yaml:"name"`              // e.g., "warm-up"
    Duration          string `yaml:"duration"`          // e.g., "5m"
    TargetActiveUsers int    `yaml:"targetActiveUsers"` // Target user count
}

// WorkerConfig defines Kubernetes worker pod parameters
type WorkerConfig struct {
    Replicas       int               `yaml:"replicas"`       // Number of worker pods
    UsersPerWorker int               `yaml:"usersPerWorker"` // Users simulated per pod
    Resources      ResourceRequests  `yaml:"resources"`
}

// ResourceRequests defines K8s resource requirements
type ResourceRequests struct {
    Requests struct {
        CPU    string `yaml:"cpu"`    // e.g., "500m"
        Memory string `yaml:"memory"` // e.g., "512Mi"
    } `yaml:"requests"`
    Limits struct {
        CPU    string `yaml:"cpu"`
        Memory string `yaml:"memory"`
    } `yaml:"limits"`
}

// TargetConfig defines platform endpoints to test
type TargetConfig struct {
    APIRouterURL string `yaml:"apiRouterUrl"` // e.g., "http://api-router.dev.svc:8080"
    UserOrgURL   string `yaml:"userOrgUrl"`   // e.g., "http://user-org.dev.svc:8081"
}

// MetricsConfig defines metrics collection and export
type MetricsConfig struct {
    PushgatewayURL string   `yaml:"pushgatewayUrl"` // Prometheus Pushgateway
    PrometheusURL  string   `yaml:"prometheusUrl"`  // For querying
    ExportInterval string   `yaml:"exportInterval"` // e.g., "10s"
    Capture        []string `yaml:"capture"`        // Metrics to collect
}

// LimitsConfig defines test safety boundaries
type LimitsConfig struct {
    MaxCostUSD     float64 `yaml:"maxCostUsd"`     // Stop if cost exceeds
    MaxErrorRate   float64 `yaml:"maxErrorRate"`   // Stop if error rate exceeds
    MaxDuration    string  `yaml:"maxDuration"`    // Hard timeout
}

// CleanupConfig defines post-test cleanup behavior
type CleanupConfig struct {
    OnCompletion   bool   `yaml:"onCompletion"`   // Delete test data
    RetainMetrics  bool   `yaml:"retainMetrics"`  // Keep metrics
    ExportResultsTo string `yaml:"exportResultsTo"` // S3/MinIO path
}
```

### Example Configuration

```yaml
apiVersion: loadtest.ai-aas.dev/v1
kind: LoadTestScenario
metadata:
  name: multi-org-realistic-load
  namespace: load-testing
  testRunId: "test-20250127-153045"

spec:
  # Create 3 organizations
  organizations:
    count: 3
    namingPattern: "loadtest-org-{index}"
    distribution: "even"

  # Each org has 10-20 users
  users:
    perOrg:
      min: 10
      max: 20
      distribution: "random"
    namingPattern: "user-{org}-{index}"

  # Each user gets 2 API keys
  apiKeys:
    perUser: 2

  # Realistic user behavior
  userBehavior:
    questionsPerSession:
      min: 10
      max: 50
      distribution: "gaussian"

    thinkTimeSeconds:
      min: 5
      max: 30
      distribution: "exponential"

    questionStrategies:
      - name: "mixed"
        weight: 50
      - name: "technical"
        weight: 30
      - name: "historical"
        weight: 20

    sessionDuration:
      min: "5m"
      max: "60m"

    modelPreferences:
      - model: "gpt-4o"
        weight: 70
      - model: "gpt-3.5-turbo"
        weight: 30

  # Ramp-up load pattern
  loadPattern:
    type: "ramp-up"
    phases:
      - name: "warm-up"
        duration: "2m"
        targetActiveUsers: 10

      - name: "ramp-up"
        duration: "10m"
        targetActiveUsers: 60  # All users active

      - name: "sustained-load"
        duration: "30m"
        targetActiveUsers: 60

      - name: "cool-down"
        duration: "5m"
        targetActiveUsers: 10

  # Worker pod configuration
  workers:
    replicas: 3  # 3 pods, 1 per org
    usersPerWorker: 20
    resources:
      requests:
        cpu: "500m"
        memory: "512Mi"
      limits:
        cpu: "1000m"
        memory: "1Gi"

  # Platform endpoints
  targets:
    apiRouterUrl: "http://api-router-service.development.svc.cluster.local:8080"
    userOrgUrl: "http://user-org-service.development.svc.cluster.local:8081"

  # Metrics configuration
  metrics:
    pushgatewayUrl: "http://prometheus-pushgateway.observability.svc:9091"
    prometheusUrl: "http://prometheus.observability.svc:9090"
    exportInterval: "10s"
    capture:
      - latency_percentiles
      - token_usage
      - cost_per_user
      - error_rate
      - concurrent_users
      - questions_per_second

  # Safety limits
  limits:
    maxCostUsd: 50.00
    maxErrorRate: 0.10  # 10%
    maxDuration: "2h"

  # Cleanup
  cleanup:
    onCompletion: true
    retainMetrics: true
    exportResultsTo: "s3://loadtest-results/{testRunId}/"
```

## Runtime State Model

### WorkerState

Tracks the state of each worker pod during execution.

```go
// WorkerState represents the runtime state of a worker pod
type WorkerState struct {
    // Identification
    WorkerID   string    `json:"workerId"`   // Unique worker identifier
    PodName    string    `json:"podName"`    // Kubernetes pod name
    TestRunID  string    `json:"testRunId"`  // Associated test run
    StartTime  time.Time `json:"startTime"`
    EndTime    *time.Time `json:"endTime,omitempty"`

    // Configuration
    AssignedOrgIndex int    `json:"assignedOrgIndex"` // Which org this worker handles
    TargetUserCount  int    `json:"targetUserCount"`  // How many users to simulate

    // Lifecycle
    Phase  WorkerPhase `json:"phase"`  // Current phase
    Status WorkerStatus `json:"status"` // Current status

    // Bootstrap tracking
    Organization *OrganizationRecord `json:"organization,omitempty"`
    Users        []UserRecord        `json:"users,omitempty"`
    APIKeys      []APIKeyRecord      `json:"apiKeys,omitempty"`

    // Runtime stats
    ActiveUsers       int       `json:"activeUsers"`
    CompletedSessions int       `json:"completedSessions"`
    TotalRequests     int64     `json:"totalRequests"`
    TotalErrors       int64     `json:"totalErrors"`
    TotalTokens       int64     `json:"totalTokens"`
    TotalCostUSD      float64   `json:"totalCostUsd"`

    // Error tracking
    LastError     *ErrorRecord `json:"lastError,omitempty"`
    ErrorCount    int          `json:"errorCount"`

    // Timestamps
    BootstrapStartTime    *time.Time `json:"bootstrapStartTime,omitempty"`
    BootstrapCompleteTime *time.Time `json:"bootstrapCompleteTime,omitempty"`
    SimulationStartTime   *time.Time `json:"simulationStartTime,omitempty"`
    SimulationEndTime     *time.Time `json:"simulationEndTime,omitempty"`
}

// WorkerPhase represents worker lifecycle phase
type WorkerPhase string

const (
    PhaseInitializing  WorkerPhase = "initializing"
    PhaseBootstrapping WorkerPhase = "bootstrapping"
    PhaseSimulating    WorkerPhase = "simulating"
    PhaseCompleting    WorkerPhase = "completing"
    PhaseCleaningUp    WorkerPhase = "cleaning_up"
    PhaseCompleted     WorkerPhase = "completed"
    PhaseFailed        WorkerPhase = "failed"
)

// WorkerStatus represents detailed status
type WorkerStatus string

const (
    StatusStarting     WorkerStatus = "starting"
    StatusHealthy      WorkerStatus = "healthy"
    StatusDegraded     WorkerStatus = "degraded"
    StatusFailed       WorkerStatus = "failed"
    StatusCompleted    WorkerStatus = "completed"
)

// OrganizationRecord tracks created organization
type OrganizationRecord struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Slug      string    `json:"slug"`
    CreatedAt time.Time `json:"createdAt"`
}

// UserRecord tracks created user
type UserRecord struct {
    ID        string    `json:"id"`
    Email     string    `json:"email"`
    OrgID     string    `json:"orgId"`
    CreatedAt time.Time `json:"createdAt"`
}

// APIKeyRecord tracks created API key
type APIKeyRecord struct {
    ID        string    `json:"id"`
    Key       string    `json:"key"` // Only stored in memory
    UserID    string    `json:"userId"`
    CreatedAt time.Time `json:"createdAt"`
}

// ErrorRecord captures error details
type ErrorRecord struct {
    Timestamp   time.Time `json:"timestamp"`
    Phase       string    `json:"phase"`
    Operation   string    `json:"operation"`
    Message     string    `json:"message"`
    ErrorType   string    `json:"errorType"`
    Retryable   bool      `json:"retryable"`
}
```

### UserSimulationState

Tracks individual user simulation within a worker.

```go
// UserSimulationState represents a single simulated user
type UserSimulationState struct {
    // Identity
    UserID       string `json:"userId"`
    UserEmail    string `json:"userEmail"`
    OrgID        string `json:"orgId"`
    APIKey       string `json:"apiKey"` // Primary key for requests
    WorkerID     string `json:"workerId"`

    // Behavior parameters (from config)
    QuestionStrategy  string        `json:"questionStrategy"`
    TargetQuestions   int           `json:"targetQuestions"`
    ThinkTimeMin      time.Duration `json:"thinkTimeMin"`
    ThinkTimeMax      time.Duration `json:"thinkTimeMax"`
    PreferredModel    string        `json:"preferredModel"`
    SessionDuration   time.Duration `json:"sessionDuration"`

    // Question generation
    QuestionSeed      int64    `json:"questionSeed"` // Unique seed for this user
    QuestionsAsked    int      `json:"questionsAsked"`
    ConversationID    string   `json:"conversationId"`
    Messages          []Message `json:"messages"` // Conversation history

    // Performance metrics
    RequestCount      int64     `json:"requestCount"`
    SuccessCount      int64     `json:"successCount"`
    ErrorCount        int64     `json:"errorCount"`
    TotalLatencyMs    int64     `json:"totalLatencyMs"`
    TotalTokens       int64     `json:"totalTokens"`
    TotalCostUSD      float64   `json:"totalCostUsd"`

    // Latency tracking
    MinLatencyMs      int64   `json:"minLatencyMs"`
    MaxLatencyMs      int64   `json:"maxLatencyMs"`
    LatencySamples    []int64 `json:"latencySamples"` // For percentile calc

    // Lifecycle
    StartTime         time.Time  `json:"startTime"`
    LastRequestTime   time.Time  `json:"lastRequestTime"`
    EndTime           *time.Time `json:"endTime,omitempty"`
    Status            string     `json:"status"` // "active", "completed", "failed"
}

// Message represents a conversation message
type Message struct {
    Role      string    `json:"role"`      // "user" or "assistant"
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
    Tokens    int       `json:"tokens,omitempty"`
}
```

## Metrics Model

### PrometheusMetrics

Defines metrics exported to Prometheus.

```go
// PrometheusMetrics defines all metrics exported by workers
type PrometheusMetrics struct {
    // Worker-level metrics
    WorkerStatus          *prometheus.GaugeVec   // Current worker status
    WorkerUsersActive     *prometheus.GaugeVec   // Active user simulations
    WorkerOrgsCreated     *prometheus.CounterVec // Organizations created
    WorkerBootstrapTime   *prometheus.HistogramVec // Bootstrap duration

    // User-level request metrics
    UserRequestsTotal     *prometheus.CounterVec   // Total requests per user
    UserLatency           *prometheus.SummaryVec   // Latency distribution
    UserTokensTotal       *prometheus.CounterVec   // Tokens consumed
    UserCostUSD           *prometheus.GaugeVec     // Cost incurred
    UserErrorsTotal       *prometheus.CounterVec   // Errors encountered

    // Test-level aggregate metrics
    TestDuration          *prometheus.GaugeVec     // Total test duration
    TestCostTotal         *prometheus.GaugeVec     // Aggregate cost
    TestRequestsPerSecond *prometheus.GaugeVec     // Throughput
    TestErrorRate         *prometheus.GaugeVec     // Error percentage
    TestActiveUsers       *prometheus.GaugeVec     // Current active users

    // Question generation metrics
    QuestionGenTime       *prometheus.HistogramVec // Time to generate questions
    QuestionUniqueness    *prometheus.GaugeVec     // Uniqueness percentage
}

// Metric labels
const (
    LabelTestRunID    = "test_run_id"
    LabelWorkerID     = "worker_id"
    LabelOrgID        = "org_id"
    LabelUserID       = "user_id"
    LabelPhase        = "phase"
    LabelStatus       = "status"
    LabelErrorType    = "error_type"
    LabelModel        = "model"
    LabelStrategy     = "strategy"
)
```

### MetricsSnapshot

Periodic snapshot of metrics for export.

```go
// MetricsSnapshot represents a point-in-time metrics snapshot
type MetricsSnapshot struct {
    Timestamp       time.Time           `json:"timestamp"`
    TestRunID       string              `json:"testRunId"`
    WorkerID        string              `json:"workerId"`

    // Aggregate metrics
    ActiveUsers     int                 `json:"activeUsers"`
    TotalRequests   int64               `json:"totalRequests"`
    TotalErrors     int64               `json:"totalErrors"`
    TotalTokens     int64               `json:"totalTokens"`
    TotalCostUSD    float64             `json:"totalCostUsd"`

    // Performance metrics
    RequestsPerSec  float64             `json:"requestsPerSec"`
    AvgLatencyMs    float64             `json:"avgLatencyMs"`
    P50LatencyMs    int64               `json:"p50LatencyMs"`
    P90LatencyMs    int64               `json:"p90LatencyMs"`
    P95LatencyMs    int64               `json:"p95LatencyMs"`
    P99LatencyMs    int64               `json:"p99LatencyMs"`
    ErrorRate       float64             `json:"errorRate"`

    // Per-user breakdown
    UserMetrics     []UserMetrics       `json:"userMetrics,omitempty"`
}

// UserMetrics represents metrics for a single user
type UserMetrics struct {
    UserID          string  `json:"userId"`
    Requests        int64   `json:"requests"`
    Errors          int64   `json:"errors"`
    AvgLatencyMs    float64 `json:"avgLatencyMs"`
    Tokens          int64   `json:"tokens"`
    CostUSD         float64 `json:"costUsd"`
}
```

## Question Generator Model

### QuestionGeneratorConfig

Configuration for the embedded question generator.

```go
// QuestionGeneratorConfig configures question generation
type QuestionGeneratorConfig struct {
    Strategy       string `json:"strategy"`       // "mixed", "historical", etc.
    Seed           int64  `json:"seed"`           // Unique seed per user
    QuestionsCount int    `json:"questionsCount"` // How many to generate
}

// GeneratedQuestion represents a generated question
type GeneratedQuestion struct {
    Text         string    `json:"text"`
    Strategy     string    `json:"strategy"`
    Seed         int64     `json:"seed"`
    Index        int       `json:"index"`
    GeneratedAt  time.Time `json:"generatedAt"`
    Hash         string    `json:"hash"` // For uniqueness checking
}
```

## Database Schema (Optional - for test result storage)

If persisting test results beyond Prometheus:

```sql
-- Load test runs
CREATE TABLE load_test_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_run_id     VARCHAR(255) UNIQUE NOT NULL,
    config          JSONB NOT NULL,
    status          VARCHAR(50) NOT NULL,
    start_time      TIMESTAMP NOT NULL,
    end_time        TIMESTAMP,
    total_workers   INT NOT NULL,
    total_orgs      INT NOT NULL,
    total_users     INT NOT NULL,
    total_requests  BIGINT DEFAULT 0,
    total_errors    BIGINT DEFAULT 0,
    total_cost_usd  DECIMAL(10, 2) DEFAULT 0,
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

-- Worker execution records
CREATE TABLE load_test_workers (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_run_id         VARCHAR(255) NOT NULL REFERENCES load_test_runs(test_run_id),
    worker_id           VARCHAR(255) NOT NULL,
    pod_name            VARCHAR(255),
    assigned_org_index  INT NOT NULL,
    phase               VARCHAR(50) NOT NULL,
    status              VARCHAR(50) NOT NULL,
    bootstrap_time_ms   INT,
    simulation_time_ms  BIGINT,
    requests_count      BIGINT DEFAULT 0,
    errors_count        BIGINT DEFAULT 0,
    tokens_count        BIGINT DEFAULT 0,
    cost_usd            DECIMAL(10, 2) DEFAULT 0,
    created_at          TIMESTAMP DEFAULT NOW(),
    updated_at          TIMESTAMP DEFAULT NOW(),
    UNIQUE(test_run_id, worker_id)
);

-- User simulation records
CREATE TABLE load_test_user_simulations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_run_id         VARCHAR(255) NOT NULL,
    worker_id           VARCHAR(255) NOT NULL,
    user_id             VARCHAR(255) NOT NULL,
    org_id              VARCHAR(255) NOT NULL,
    question_strategy   VARCHAR(50) NOT NULL,
    requests_count      BIGINT DEFAULT 0,
    success_count       BIGINT DEFAULT 0,
    error_count         BIGINT DEFAULT 0,
    avg_latency_ms      DECIMAL(10, 2),
    p95_latency_ms      INT,
    p99_latency_ms      INT,
    total_tokens        BIGINT DEFAULT 0,
    total_cost_usd      DECIMAL(10, 2) DEFAULT 0,
    start_time          TIMESTAMP NOT NULL,
    end_time            TIMESTAMP,
    created_at          TIMESTAMP DEFAULT NOW(),
    FOREIGN KEY (test_run_id, worker_id) REFERENCES load_test_workers(test_run_id, worker_id)
);

-- Metrics snapshots for historical analysis
CREATE TABLE load_test_metrics_snapshots (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_run_id         VARCHAR(255) NOT NULL REFERENCES load_test_runs(test_run_id),
    snapshot_time       TIMESTAMP NOT NULL,
    active_users        INT NOT NULL,
    requests_per_sec    DECIMAL(10, 2),
    avg_latency_ms      DECIMAL(10, 2),
    p95_latency_ms      INT,
    error_rate          DECIMAL(5, 4),
    total_cost_usd      DECIMAL(10, 2),
    metrics_json        JSONB,
    created_at          TIMESTAMP DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX idx_load_test_runs_test_run_id ON load_test_runs(test_run_id);
CREATE INDEX idx_load_test_runs_status ON load_test_runs(status);
CREATE INDEX idx_load_test_workers_test_run ON load_test_workers(test_run_id);
CREATE INDEX idx_user_sims_test_run ON load_test_user_simulations(test_run_id);
CREATE INDEX idx_metrics_snapshots_test_run ON load_test_metrics_snapshots(test_run_id);
CREATE INDEX idx_metrics_snapshots_time ON load_test_metrics_snapshots(snapshot_time);
```

## Relationships

```
LoadTestRun (1) ──── (N) WorkerPod
                         │
                         ├─ (1) Organization
                         │
                         └─ (N) UserSimulation
                                │
                                └─ (N) GeneratedQuestion
                                       │
                                       └─ (N) Request
                                              │
                                              └─ (1) MetricsSnapshot
```

This data model supports:
- Complete test configuration via YAML
- Runtime state tracking for debugging
- Comprehensive metrics collection
- Optional persistence for historical analysis
- Efficient querying and aggregation
