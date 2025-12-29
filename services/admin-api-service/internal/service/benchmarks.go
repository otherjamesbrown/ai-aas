package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/repository"
	"go.uber.org/zap"
)

// Default guidellm-runner service URL (matches Helm release name pattern)
const defaultGuideLLMRunnerURL = "http://guidellm-runner-development.development.svc.cluster.local:8080"

// BenchmarkService handles benchmark business logic
type BenchmarkService struct {
	repo        *repository.BenchmarkRepository
	guidellmURL string
	httpClient  *http.Client
	logger      *zap.Logger
}

// BenchmarkServiceConfig holds configuration for the benchmark service
type BenchmarkServiceConfig struct {
	GuideLLMRunnerURL string
	HTTPTimeout       time.Duration
}

// NewBenchmarkService creates a new benchmark service
func NewBenchmarkService(repo *repository.BenchmarkRepository, logger *zap.Logger) *BenchmarkService {
	return NewBenchmarkServiceWithConfig(repo, logger, BenchmarkServiceConfig{})
}

// NewBenchmarkServiceWithConfig creates a new benchmark service with configuration
func NewBenchmarkServiceWithConfig(repo *repository.BenchmarkRepository, logger *zap.Logger, cfg BenchmarkServiceConfig) *BenchmarkService {
	guidellmURL := cfg.GuideLLMRunnerURL
	if guidellmURL == "" {
		guidellmURL = defaultGuideLLMRunnerURL
	}

	timeout := cfg.HTTPTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &BenchmarkService{
		repo:        repo,
		guidellmURL: guidellmURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

// ============================================================================
// Scenario Operations
// ============================================================================

// ListScenarios retrieves benchmark scenarios with pagination
func (s *BenchmarkService) ListScenarios(ctx context.Context, params domain.BenchmarkScenarioListParams) (*domain.BenchmarkScenarioListResponse, error) {
	return s.repo.ListScenarios(ctx, params)
}

// GetScenario retrieves a scenario by name
func (s *BenchmarkService) GetScenario(ctx context.Context, name string) (*domain.BenchmarkScenario, error) {
	return s.repo.GetScenarioByName(ctx, name)
}

// SyncScenarios syncs scenarios from the config repository
// This upserts scenarios from the provided list and optionally removes scenarios not in the list
func (s *BenchmarkService) SyncScenarios(ctx context.Context, scenarios []domain.BenchmarkScenarioUpsert, deleteOrphans bool) (*SyncScenariosResult, error) {
	result := &SyncScenariosResult{
		Created: []string{},
		Updated: []string{},
		Deleted: []string{},
	}

	// Track scenario names for orphan detection
	syncedNames := make(map[string]bool)

	for _, scenario := range scenarios {
		// Check if exists
		existing, err := s.repo.GetScenarioByName(ctx, scenario.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing scenario: %w", err)
		}

		_, err = s.repo.UpsertScenario(ctx, &scenario)
		if err != nil {
			return nil, fmt.Errorf("failed to upsert scenario %s: %w", scenario.Name, err)
		}

		if existing == nil {
			result.Created = append(result.Created, scenario.Name)
		} else {
			result.Updated = append(result.Updated, scenario.Name)
		}

		syncedNames[scenario.Name] = true
	}

	// Delete orphan scenarios if requested
	if deleteOrphans {
		listParams := domain.BenchmarkScenarioListParams{Limit: 500}
		existing, err := s.repo.ListScenarios(ctx, listParams)
		if err != nil {
			return nil, fmt.Errorf("failed to list existing scenarios: %w", err)
		}

		for _, scenario := range existing.Scenarios {
			if !syncedNames[scenario.Name] {
				if err := s.repo.DeleteScenario(ctx, scenario.Name); err != nil {
					s.logger.Warn("failed to delete orphan scenario",
						zap.String("name", scenario.Name),
						zap.Error(err))
				} else {
					result.Deleted = append(result.Deleted, scenario.Name)
				}
			}
		}
	}

	s.logger.Info("scenarios synced",
		zap.Int("created", len(result.Created)),
		zap.Int("updated", len(result.Updated)),
		zap.Int("deleted", len(result.Deleted)),
	)

	return result, nil
}

// SyncScenariosResult holds the result of a scenario sync operation
type SyncScenariosResult struct {
	Created []string `json:"created"`
	Updated []string `json:"updated"`
	Deleted []string `json:"deleted"`
}

// ============================================================================
// Target Operations
// ============================================================================

// CreateTarget creates a new benchmark target
func (s *BenchmarkService) CreateTarget(ctx context.Context, create *domain.BenchmarkTargetCreate) (*domain.BenchmarkTarget, error) {
	// Check for duplicate name+environment
	existing, err := s.repo.GetTargetByNameAndEnv(ctx, create.Name, create.Environment)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing target: %w", err)
	}
	if existing != nil {
		return nil, &ConflictError{Message: fmt.Sprintf("target with name '%s' already exists in environment '%s'", create.Name, create.Environment)}
	}

	// Verify scenario exists
	scenario, err := s.repo.GetScenarioByName(ctx, create.ScenarioName)
	if err != nil {
		return nil, fmt.Errorf("failed to check scenario: %w", err)
	}
	if scenario == nil {
		return nil, &BenchmarkValidationError{Message: fmt.Sprintf("scenario '%s' does not exist", create.ScenarioName)}
	}

	target, err := s.repo.CreateTarget(ctx, create)
	if err != nil {
		s.logger.Error("failed to create benchmark target",
			zap.String("name", create.Name),
			zap.Error(err))
		return nil, err
	}

	s.logger.Info("benchmark target created",
		zap.String("target_id", target.ID.String()),
		zap.String("name", target.Name),
		zap.String("environment", target.Environment),
	)

	return target, nil
}

// GetTarget retrieves a target by ID
func (s *BenchmarkService) GetTarget(ctx context.Context, id uuid.UUID) (*domain.BenchmarkTarget, error) {
	return s.repo.GetTargetByID(ctx, id)
}

// ListTargets retrieves targets with filtering and pagination
func (s *BenchmarkService) ListTargets(ctx context.Context, params domain.BenchmarkTargetListParams) (*domain.BenchmarkTargetListResponse, error) {
	return s.repo.ListTargets(ctx, params)
}

// UpdateTarget updates specific fields of a target
func (s *BenchmarkService) UpdateTarget(ctx context.Context, id uuid.UUID, update *domain.BenchmarkTargetUpdate) (*domain.BenchmarkTarget, error) {
	// Validate status if provided
	if update.Status != nil && !domain.IsValidBenchmarkTargetStatus(*update.Status) {
		return nil, &BenchmarkValidationError{Message: fmt.Sprintf("invalid status: %s. Must be one of: active, paused, disabled", *update.Status)}
	}

	// Verify scenario exists if changing
	if update.ScenarioName != nil {
		scenario, err := s.repo.GetScenarioByName(ctx, *update.ScenarioName)
		if err != nil {
			return nil, fmt.Errorf("failed to check scenario: %w", err)
		}
		if scenario == nil {
			return nil, &BenchmarkValidationError{Message: fmt.Sprintf("scenario '%s' does not exist", *update.ScenarioName)}
		}
	}

	target, err := s.repo.UpdateTarget(ctx, id, update)
	if err != nil {
		return nil, err
	}

	if target != nil {
		s.logger.Info("benchmark target updated",
			zap.String("target_id", target.ID.String()),
			zap.String("name", target.Name),
		)
	}

	return target, nil
}

// DeleteTarget removes a target by ID
func (s *BenchmarkService) DeleteTarget(ctx context.Context, id uuid.UUID) error {
	// Check if target exists
	target, err := s.repo.GetTargetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get target: %w", err)
	}
	if target == nil {
		return nil // Nothing to delete
	}

	// Stop the target if running before deleting
	if target.Status == "active" && target.ScheduleEnabled {
		if err := s.stopTargetOnRunner(ctx, target); err != nil {
			s.logger.Warn("failed to stop target on runner before deletion",
				zap.String("target_id", id.String()),
				zap.Error(err))
		}
	}

	if err := s.repo.DeleteTarget(ctx, id); err != nil {
		return fmt.Errorf("failed to delete target: %w", err)
	}

	s.logger.Info("benchmark target deleted",
		zap.String("target_id", id.String()),
		zap.String("name", target.Name),
	)

	return nil
}

// StartTarget starts the benchmark schedule for a target
func (s *BenchmarkService) StartTarget(ctx context.Context, id uuid.UUID) (*domain.BenchmarkTarget, error) {
	target, err := s.repo.GetTargetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get target: %w", err)
	}
	if target == nil {
		return nil, nil
	}

	// Already active?
	if target.Status == "active" && target.ScheduleEnabled {
		return target, nil
	}

	// Register/start target with guidellm-runner
	if err := s.startTargetOnRunner(ctx, target); err != nil {
		return nil, fmt.Errorf("failed to start target on runner: %w", err)
	}

	// Update target status
	statusActive := "active"
	scheduleEnabled := true
	update := &domain.BenchmarkTargetUpdate{
		Status:          &statusActive,
		ScheduleEnabled: &scheduleEnabled,
	}

	target, err = s.repo.UpdateTarget(ctx, id, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update target status: %w", err)
	}

	s.logger.Info("benchmark target started",
		zap.String("target_id", id.String()),
		zap.String("name", target.Name),
	)

	return target, nil
}

// StopTarget stops the benchmark schedule for a target
func (s *BenchmarkService) StopTarget(ctx context.Context, id uuid.UUID) (*domain.BenchmarkTarget, error) {
	target, err := s.repo.GetTargetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get target: %w", err)
	}
	if target == nil {
		return nil, nil
	}

	// Stop target on guidellm-runner
	if err := s.stopTargetOnRunner(ctx, target); err != nil {
		s.logger.Warn("failed to stop target on runner",
			zap.String("target_id", id.String()),
			zap.Error(err))
		// Continue anyway to update local status
	}

	// Update target status
	statusPaused := "paused"
	scheduleEnabled := false
	update := &domain.BenchmarkTargetUpdate{
		Status:          &statusPaused,
		ScheduleEnabled: &scheduleEnabled,
	}

	target, err = s.repo.UpdateTarget(ctx, id, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update target status: %w", err)
	}

	s.logger.Info("benchmark target stopped",
		zap.String("target_id", id.String()),
		zap.String("name", target.Name),
	)

	return target, nil
}

// ============================================================================
// Run Operations
// ============================================================================

// ListRuns retrieves benchmark runs with filtering and pagination
func (s *BenchmarkService) ListRuns(ctx context.Context, params domain.BenchmarkRunListParams) (*domain.BenchmarkRunListResponse, error) {
	return s.repo.ListRuns(ctx, params)
}

// GetRun retrieves a run by ID
func (s *BenchmarkService) GetRun(ctx context.Context, id uuid.UUID) (*domain.BenchmarkRun, error) {
	return s.repo.GetRunByID(ctx, id)
}

// TriggerRun triggers a manual benchmark run for a target
func (s *BenchmarkService) TriggerRun(ctx context.Context, targetID uuid.UUID, triggeredByUser *string) (*domain.BenchmarkRun, error) {
	// Get target
	target, err := s.repo.GetTargetByID(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get target: %w", err)
	}
	if target == nil {
		return nil, nil
	}

	// Check target is not disabled
	if target.Status == "disabled" {
		return nil, &BenchmarkValidationError{Message: "cannot trigger run on a disabled target"}
	}

	// Create run record
	create := &domain.BenchmarkRunCreate{
		TargetID:        targetID,
		ScenarioName:    target.ScenarioName,
		TriggeredBy:     "manual",
		TriggeredByUser: triggeredByUser,
	}

	run, err := s.repo.CreateRun(ctx, create)
	if err != nil {
		return nil, fmt.Errorf("failed to create run: %w", err)
	}

	// Trigger on guidellm-runner
	if err := s.triggerRunOnRunner(ctx, target, run); err != nil {
		s.logger.Error("failed to trigger run on runner",
			zap.String("target_id", targetID.String()),
			zap.String("run_id", run.ID.String()),
			zap.Error(err))

		// Update run status to failed
		statusFailed := "failed"
		errMsg := err.Error()
		s.repo.UpdateRun(ctx, run.ID, &domain.BenchmarkRunUpdate{
			Status:       &statusFailed,
			ErrorMessage: &errMsg,
		})

		return nil, fmt.Errorf("failed to trigger run on runner: %w", err)
	}

	s.logger.Info("benchmark run triggered",
		zap.String("target_id", targetID.String()),
		zap.String("run_id", run.ID.String()),
	)

	return run, nil
}

// ============================================================================
// GuideLLM Runner Integration
// ============================================================================

// runnerTargetRequest represents a request to the guidellm-runner API
// Field names match guidellm-runner's AddTargetRequest
type runnerTargetRequest struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Model       string `json:"model"`
	Environment string `json:"environment,omitempty"`
	APIKey      string `json:"api_key,omitempty"`
	Profile     string `json:"profile,omitempty"`
	Rate        *int   `json:"rate,omitempty"`
	MaxSeconds  *int   `json:"max_seconds,omitempty"`
	RequestType string `json:"request_type,omitempty"` // chat_completions or text_completions
}

// runnerTriggerRequest represents a trigger request to the guidellm-runner API
type runnerTriggerRequest struct {
	RunID           string                 `json:"run_id"`
	ConfigOverrides map[string]interface{} `json:"config_overrides,omitempty"`
}

func (s *BenchmarkService) startTargetOnRunner(ctx context.Context, target *domain.BenchmarkTarget) error {
	// Determine endpoint URL - use target's override or default to API router
	endpointURL := "https://api.dev.otherjamesbrown.com"
	if target.EndpointURL != nil && *target.EndpointURL != "" {
		endpointURL = *target.EndpointURL
	}

	reqBody := runnerTargetRequest{
		Name:        target.Name,
		URL:         endpointURL,
		Model:       target.ModelName,
		Environment: target.Environment,
	}

	// Look up scenario to get benchmark configuration
	if target.ScenarioName != "" {
		scenario, err := s.repo.GetScenarioByName(ctx, target.ScenarioName)
		if err != nil {
			s.logger.Warn("failed to get scenario for target",
				zap.String("target", target.Name),
				zap.String("scenario", target.ScenarioName),
				zap.Error(err))
		} else if scenario != nil && scenario.Config != nil {
			// Extract defaults from scenario config
			if defaults, ok := scenario.Config["defaults"].(map[string]interface{}); ok {
				if profile, ok := defaults["profile"].(string); ok {
					reqBody.Profile = profile
				}
				if rate, ok := defaults["rate"].(float64); ok {
					rateInt := int(rate)
					reqBody.Rate = &rateInt
				}
				if maxSeconds, ok := defaults["max_seconds"].(float64); ok {
					maxSecondsInt := int(maxSeconds)
					reqBody.MaxSeconds = &maxSecondsInt
				}
				if requestType, ok := defaults["request_type"].(string); ok {
					reqBody.RequestType = requestType
				}
			}
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// POST to create/update target, then start it
	url := fmt.Sprintf("%s/api/targets", s.guidellmURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register target: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("runner returned error: %s (status %d)", string(bodyBytes), resp.StatusCode)
	}

	// Start the target
	startURL := fmt.Sprintf("%s/api/targets/%s/start", s.guidellmURL, target.Name)
	startReq, err := http.NewRequestWithContext(ctx, http.MethodPost, startURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create start request: %w", err)
	}

	startResp, err := s.httpClient.Do(startReq)
	if err != nil {
		return fmt.Errorf("failed to start target: %w", err)
	}
	defer startResp.Body.Close()

	if startResp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(startResp.Body)
		return fmt.Errorf("runner returned error on start: %s (status %d)", string(bodyBytes), startResp.StatusCode)
	}

	return nil
}

func (s *BenchmarkService) stopTargetOnRunner(ctx context.Context, target *domain.BenchmarkTarget) error {
	url := fmt.Sprintf("%s/api/targets/%s/stop", s.guidellmURL, target.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to stop target: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("runner returned error: %s (status %d)", string(bodyBytes), resp.StatusCode)
	}

	return nil
}

// registerTargetOnRunner creates/updates a target on the guidellm-runner
// This ensures the target exists before triggering a run
func (s *BenchmarkService) registerTargetOnRunner(ctx context.Context, target *domain.BenchmarkTarget) error {
	// Determine endpoint URL - use target's override or default to API router
	endpointURL := "https://api.dev.otherjamesbrown.com"
	if target.EndpointURL != nil && *target.EndpointURL != "" {
		endpointURL = *target.EndpointURL
	}

	reqBody := runnerTargetRequest{
		Name:        target.Name,
		URL:         endpointURL,
		Model:       target.ModelName,
		Environment: target.Environment,
	}

	// Look up scenario to get benchmark configuration
	if target.ScenarioName != "" {
		scenario, err := s.repo.GetScenarioByName(ctx, target.ScenarioName)
		if err != nil {
			s.logger.Warn("failed to get scenario for target",
				zap.String("target", target.Name),
				zap.String("scenario", target.ScenarioName),
				zap.Error(err))
		} else if scenario != nil && scenario.Config != nil {
			// Extract defaults from scenario config
			if defaults, ok := scenario.Config["defaults"].(map[string]interface{}); ok {
				if profile, ok := defaults["profile"].(string); ok {
					reqBody.Profile = profile
				}
				if rate, ok := defaults["rate"].(float64); ok {
					rateInt := int(rate)
					reqBody.Rate = &rateInt
				}
				if maxSeconds, ok := defaults["max_seconds"].(float64); ok {
					maxSecondsInt := int(maxSeconds)
					reqBody.MaxSeconds = &maxSecondsInt
				}
				if requestType, ok := defaults["request_type"].(string); ok {
					reqBody.RequestType = requestType
				}
			}
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/targets", s.guidellmURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register target: %w", err)
	}
	defer resp.Body.Close()

	// Ignore "already exists" errors (400) - that's fine for trigger
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("runner returned error: %s (status %d)", string(bodyBytes), resp.StatusCode)
	}

	return nil
}

// runnerTriggerResponse represents the response from the runner's trigger endpoint
type runnerTriggerResponse struct {
	Name    string                 `json:"name"`
	RunID   string                 `json:"run_id"`
	Status  string                 `json:"status"`
	Results *runnerParsedResults   `json:"results,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// runnerParsedResults represents the results from guidellm-runner
type runnerParsedResults struct {
	TotalRequests      int       `json:"TotalRequests"`
	SuccessfulRequests int       `json:"SuccessfulRequests"`
	FailedRequests     int       `json:"FailedRequests"`
	PromptTokens       int       `json:"PromptTokens"`
	OutputTokens       int       `json:"OutputTokens"`
	OutputTokensPerSec float64   `json:"OutputTokensPerSec"`
	RequestsPerSec     float64   `json:"RequestsPerSec"`
	TTFTValues         []float64 `json:"TTFTValues"`
	ITLValues          []float64 `json:"ITLValues"`
	E2EValues          []float64 `json:"E2EValues"`
}

func (s *BenchmarkService) triggerRunOnRunner(ctx context.Context, target *domain.BenchmarkTarget, run *domain.BenchmarkRun) error {
	// First, register the target on the runner (in case it doesn't exist)
	if err := s.registerTargetOnRunner(ctx, target); err != nil {
		return fmt.Errorf("failed to register target on runner: %w", err)
	}

	reqBody := runnerTriggerRequest{
		RunID:           run.ID.String(),
		ConfigOverrides: target.ConfigOverrides,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/targets/%s/trigger", s.guidellmURL, target.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to trigger run: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("runner returned error: %s (status %d)", string(bodyBytes), resp.StatusCode)
	}

	// Parse the response
	var triggerResp runnerTriggerResponse
	if err := json.Unmarshal(bodyBytes, &triggerResp); err != nil {
		s.logger.Warn("failed to parse trigger response", zap.Error(err))
		return nil // Don't fail the request, just log it
	}

	// Update the run with results
	now := time.Now()
	if triggerResp.Status == "completed" && triggerResp.Results != nil {
		results := s.convertRunnerResults(triggerResp.Results)
		statusCompleted := "completed"
		update := &domain.BenchmarkRunUpdate{
			Status:      &statusCompleted,
			StartedAt:   &run.CreatedAt,
			CompletedAt: &now,
			Results:     results,
		}

		// Calculate duration
		duration := int(now.Sub(run.CreatedAt).Seconds())
		update.DurationSeconds = &duration

		if _, err := s.repo.UpdateRun(ctx, run.ID, update); err != nil {
			s.logger.Warn("failed to update run with results",
				zap.String("run_id", run.ID.String()),
				zap.Error(err))
		}

		// Also update the target's last run info
		lastRunStatus := "completed"
		s.repo.UpdateTarget(ctx, target.ID, &domain.BenchmarkTargetUpdate{
			LastRunAt:     &now,
			LastRunStatus: &lastRunStatus,
		})
	} else if triggerResp.Status == "failed" {
		statusFailed := "failed"
		errMsg := triggerResp.Error
		s.repo.UpdateRun(ctx, run.ID, &domain.BenchmarkRunUpdate{
			Status:       &statusFailed,
			ErrorMessage: &errMsg,
			CompletedAt:  &now,
		})

		lastRunStatus := "failed"
		s.repo.UpdateTarget(ctx, target.ID, &domain.BenchmarkTargetUpdate{
			LastRunAt:     &now,
			LastRunStatus: &lastRunStatus,
		})
	}

	return nil
}

// convertRunnerResults converts runner's parsed results to domain BenchmarkResults
func (s *BenchmarkService) convertRunnerResults(r *runnerParsedResults) *domain.BenchmarkResults {
	results := &domain.BenchmarkResults{
		RequestsTotal:     r.TotalRequests,
		RequestsPerSecond: r.RequestsPerSec,
		TokensPerSecond:   r.OutputTokensPerSec,
		ErrorCount:        r.FailedRequests,
		TotalInputTokens:  r.PromptTokens,
		TotalOutputTokens: r.OutputTokens,
	}

	// Calculate error rate
	if r.TotalRequests > 0 {
		results.ErrorRate = float64(r.FailedRequests) / float64(r.TotalRequests)
	}

	// Calculate TTFT percentiles (convert from seconds to milliseconds)
	if len(r.TTFTValues) > 0 {
		results.TTFTP50 = percentile(r.TTFTValues, 50) * 1000
		results.TTFTP90 = percentile(r.TTFTValues, 90) * 1000
		results.TTFTP99 = percentile(r.TTFTValues, 99) * 1000
		results.TTFTAvg = average(r.TTFTValues) * 1000
	}

	// Calculate ITL percentiles
	if len(r.ITLValues) > 0 {
		results.ITLP50 = percentile(r.ITLValues, 50) * 1000
		results.ITLP90 = percentile(r.ITLValues, 90) * 1000
		results.ITLP99 = percentile(r.ITLValues, 99) * 1000
		results.ITLAvg = average(r.ITLValues) * 1000
	}

	// Calculate E2E latency percentiles
	if len(r.E2EValues) > 0 {
		results.LatencyP50 = percentile(r.E2EValues, 50) * 1000
		results.LatencyP90 = percentile(r.E2EValues, 90) * 1000
		results.LatencyP95 = percentile(r.E2EValues, 95) * 1000
		results.LatencyP99 = percentile(r.E2EValues, 99) * 1000
		results.LatencyAvg = average(r.E2EValues) * 1000
		results.LatencyMin = min(r.E2EValues) * 1000
		results.LatencyMax = max(r.E2EValues) * 1000
	}

	return results
}

// Helper functions for statistics

// percentile calculates the pth percentile of a sorted slice
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Make a copy and sort
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sortFloat64s(sorted)

	// Calculate index
	idx := (p / 100) * float64(len(sorted)-1)
	lower := int(idx)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	// Linear interpolation
	weight := idx - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func min(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func max(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// sortFloat64s sorts a slice of float64s in place
func sortFloat64s(a []float64) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

// BenchmarkValidationError represents a validation error in the benchmark service layer
type BenchmarkValidationError struct {
	Message string
}

func (e *BenchmarkValidationError) Error() string {
	return e.Message
}
