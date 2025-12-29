package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
)

// BenchmarkRepository handles benchmark database operations
type BenchmarkRepository struct {
	db *DB
}

// NewBenchmarkRepository creates a new benchmark repository
func NewBenchmarkRepository(db *DB) *BenchmarkRepository {
	return &BenchmarkRepository{db: db}
}

// ============================================================================
// Scenario Operations
// ============================================================================

// UpsertScenario creates or updates a benchmark scenario
func (r *BenchmarkRepository) UpsertScenario(ctx context.Context, upsert *domain.BenchmarkScenarioUpsert) (*domain.BenchmarkScenario, error) {
	configJSON, err := json.Marshal(upsert.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		INSERT INTO benchmark_scenarios (name, description, version, config, synced_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (name) DO UPDATE SET
			description = EXCLUDED.description,
			version = EXCLUDED.version,
			config = EXCLUDED.config,
			synced_at = NOW()
		RETURNING name, description, version, config, synced_at
	`

	var scenario domain.BenchmarkScenario
	var configBytes []byte

	err = r.db.Pool().QueryRow(ctx, query,
		upsert.Name, upsert.Description, upsert.Version, configJSON,
	).Scan(
		&scenario.Name, &scenario.Description, &scenario.Version,
		&configBytes, &scenario.SyncedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to upsert scenario: %w", err)
	}

	if err := json.Unmarshal(configBytes, &scenario.Config); err != nil {
		scenario.Config = make(map[string]interface{})
	}

	return &scenario, nil
}

// GetScenarioByName retrieves a scenario by name
func (r *BenchmarkRepository) GetScenarioByName(ctx context.Context, name string) (*domain.BenchmarkScenario, error) {
	query := `
		SELECT name, description, version, config, synced_at
		FROM benchmark_scenarios
		WHERE name = $1
	`

	var scenario domain.BenchmarkScenario
	var configBytes []byte

	err := r.db.Pool().QueryRow(ctx, query, name).Scan(
		&scenario.Name, &scenario.Description, &scenario.Version,
		&configBytes, &scenario.SyncedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get scenario: %w", err)
	}

	if err := json.Unmarshal(configBytes, &scenario.Config); err != nil {
		scenario.Config = make(map[string]interface{})
	}

	return &scenario, nil
}

// ListScenarios retrieves scenarios with pagination
func (r *BenchmarkRepository) ListScenarios(ctx context.Context, params domain.BenchmarkScenarioListParams) (*domain.BenchmarkScenarioListResponse, error) {
	// Count total
	var total int
	if err := r.db.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM benchmark_scenarios").Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count scenarios: %w", err)
	}

	// Fetch page
	limit := params.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := `
		SELECT name, description, version, config, synced_at
		FROM benchmark_scenarios
		ORDER BY name ASC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Pool().Query(ctx, query, limit, params.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list scenarios: %w", err)
	}
	defer rows.Close()

	scenarios := []domain.BenchmarkScenario{}
	for rows.Next() {
		var scenario domain.BenchmarkScenario
		var configBytes []byte

		if err := rows.Scan(
			&scenario.Name, &scenario.Description, &scenario.Version,
			&configBytes, &scenario.SyncedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan scenario: %w", err)
		}

		if err := json.Unmarshal(configBytes, &scenario.Config); err != nil {
			scenario.Config = make(map[string]interface{})
		}

		scenarios = append(scenarios, scenario)
	}

	response := &domain.BenchmarkScenarioListResponse{
		Scenarios: scenarios,
		Pagination: domain.Pagination{
			Total:  total,
			Limit:  limit,
			Offset: params.Offset,
		},
	}

	if params.Offset+len(scenarios) < total {
		nextOffset := params.Offset + limit
		response.Pagination.NextOffset = &nextOffset
	}

	return response, nil
}

// DeleteScenario removes a scenario by name
func (r *BenchmarkRepository) DeleteScenario(ctx context.Context, name string) error {
	query := `DELETE FROM benchmark_scenarios WHERE name = $1`
	_, err := r.db.Pool().Exec(ctx, query, name)
	return err
}

// ============================================================================
// Target Operations
// ============================================================================

// CreateTarget creates a new benchmark target
func (r *BenchmarkRepository) CreateTarget(ctx context.Context, create *domain.BenchmarkTargetCreate) (*domain.BenchmarkTarget, error) {
	configOverridesJSON, err := json.Marshal(create.ConfigOverrides)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config overrides: %w", err)
	}

	// Note: scheduleEnabled is always false on creation.
	// It becomes true when StartTarget is called.

	target := &domain.BenchmarkTarget{
		ID:              uuid.New(),
		Name:            create.Name,
		DisplayName:     create.DisplayName,
		ModelName:       create.ModelName,
		ScenarioName:    create.ScenarioName,
		Environment:     create.Environment,
		EndpointURL:     create.EndpointURL,
		OrgID:           create.OrgID,
		Status:          "paused", // Start as paused, becomes active when started
		IntervalSeconds: create.IntervalSeconds,
		ScheduleEnabled: false, // Disabled until explicitly started
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	query := `
		INSERT INTO benchmark_targets (
			id, name, display_name, model_name, scenario_name, environment,
			endpoint_url, org_id, status, interval_seconds, schedule_enabled,
			config_overrides, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = r.db.Pool().Exec(ctx, query,
		target.ID, target.Name, target.DisplayName, target.ModelName,
		target.ScenarioName, target.Environment, target.EndpointURL,
		target.OrgID, target.Status, target.IntervalSeconds,
		target.ScheduleEnabled, configOverridesJSON, target.CreatedAt, target.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create target: %w", err)
	}

	target.ConfigOverrides = create.ConfigOverrides
	return target, nil
}

// GetTargetByID retrieves a target by ID
func (r *BenchmarkRepository) GetTargetByID(ctx context.Context, id uuid.UUID) (*domain.BenchmarkTarget, error) {
	query := `
		SELECT id, name, display_name, model_name, scenario_name, environment,
			endpoint_url, org_id, status, interval_seconds, schedule_enabled,
			config_overrides, last_run_at, last_run_status, created_at, updated_at
		FROM benchmark_targets
		WHERE id = $1
	`

	var target domain.BenchmarkTarget
	var configOverridesBytes []byte

	err := r.db.Pool().QueryRow(ctx, query, id).Scan(
		&target.ID, &target.Name, &target.DisplayName, &target.ModelName,
		&target.ScenarioName, &target.Environment, &target.EndpointURL,
		&target.OrgID, &target.Status, &target.IntervalSeconds,
		&target.ScheduleEnabled, &configOverridesBytes, &target.LastRunAt,
		&target.LastRunStatus, &target.CreatedAt, &target.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get target: %w", err)
	}

	if err := json.Unmarshal(configOverridesBytes, &target.ConfigOverrides); err != nil {
		target.ConfigOverrides = make(map[string]interface{})
	}

	return &target, nil
}

// GetTargetByNameAndEnv retrieves a target by name and environment
func (r *BenchmarkRepository) GetTargetByNameAndEnv(ctx context.Context, name, environment string) (*domain.BenchmarkTarget, error) {
	query := `
		SELECT id, name, display_name, model_name, scenario_name, environment,
			endpoint_url, org_id, status, interval_seconds, schedule_enabled,
			config_overrides, last_run_at, last_run_status, created_at, updated_at
		FROM benchmark_targets
		WHERE name = $1 AND environment = $2
	`

	var target domain.BenchmarkTarget
	var configOverridesBytes []byte

	err := r.db.Pool().QueryRow(ctx, query, name, environment).Scan(
		&target.ID, &target.Name, &target.DisplayName, &target.ModelName,
		&target.ScenarioName, &target.Environment, &target.EndpointURL,
		&target.OrgID, &target.Status, &target.IntervalSeconds,
		&target.ScheduleEnabled, &configOverridesBytes, &target.LastRunAt,
		&target.LastRunStatus, &target.CreatedAt, &target.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get target: %w", err)
	}

	if err := json.Unmarshal(configOverridesBytes, &target.ConfigOverrides); err != nil {
		target.ConfigOverrides = make(map[string]interface{})
	}

	return &target, nil
}

// ListTargets retrieves targets with filtering and pagination
func (r *BenchmarkRepository) ListTargets(ctx context.Context, params domain.BenchmarkTargetListParams) (*domain.BenchmarkTargetListResponse, error) {
	// Build query
	baseQuery := `FROM benchmark_targets WHERE 1=1`
	args := []interface{}{}
	argNum := 1

	if params.Environment != "" {
		baseQuery += fmt.Sprintf(" AND environment = $%d", argNum)
		args = append(args, params.Environment)
		argNum++
	}

	if params.ModelName != "" {
		baseQuery += fmt.Sprintf(" AND model_name = $%d", argNum)
		args = append(args, params.ModelName)
		argNum++
	}

	if params.ScenarioName != "" {
		baseQuery += fmt.Sprintf(" AND scenario_name = $%d", argNum)
		args = append(args, params.ScenarioName)
		argNum++
	}

	if params.Status != "" {
		baseQuery += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, params.Status)
		argNum++
	}

	if params.OrgID != nil {
		baseQuery += fmt.Sprintf(" AND org_id = $%d", argNum)
		args = append(args, *params.OrgID)
		argNum++
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := r.db.Pool().QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count targets: %w", err)
	}

	// Fetch page
	limit := params.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	selectQuery := `
		SELECT id, name, display_name, model_name, scenario_name, environment,
			endpoint_url, org_id, status, interval_seconds, schedule_enabled,
			config_overrides, last_run_at, last_run_status, created_at, updated_at
	` + baseQuery + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argNum, argNum+1)

	args = append(args, limit, params.Offset)

	rows, err := r.db.Pool().Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list targets: %w", err)
	}
	defer rows.Close()

	targets := []domain.BenchmarkTarget{}
	for rows.Next() {
		var target domain.BenchmarkTarget
		var configOverridesBytes []byte

		if err := rows.Scan(
			&target.ID, &target.Name, &target.DisplayName, &target.ModelName,
			&target.ScenarioName, &target.Environment, &target.EndpointURL,
			&target.OrgID, &target.Status, &target.IntervalSeconds,
			&target.ScheduleEnabled, &configOverridesBytes, &target.LastRunAt,
			&target.LastRunStatus, &target.CreatedAt, &target.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan target: %w", err)
		}

		if err := json.Unmarshal(configOverridesBytes, &target.ConfigOverrides); err != nil {
			target.ConfigOverrides = make(map[string]interface{})
		}

		targets = append(targets, target)
	}

	response := &domain.BenchmarkTargetListResponse{
		Targets: targets,
		Pagination: domain.Pagination{
			Total:  total,
			Limit:  limit,
			Offset: params.Offset,
		},
	}

	if params.Offset+len(targets) < total {
		nextOffset := params.Offset + limit
		response.Pagination.NextOffset = &nextOffset
	}

	return response, nil
}

// UpdateTarget updates specific fields of a target
func (r *BenchmarkRepository) UpdateTarget(ctx context.Context, id uuid.UUID, update *domain.BenchmarkTargetUpdate) (*domain.BenchmarkTarget, error) {
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argNum := 1

	if update.DisplayName != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_name = $%d", argNum))
		args = append(args, *update.DisplayName)
		argNum++
	}
	if update.ModelName != nil {
		setClauses = append(setClauses, fmt.Sprintf("model_name = $%d", argNum))
		args = append(args, *update.ModelName)
		argNum++
	}
	if update.ScenarioName != nil {
		setClauses = append(setClauses, fmt.Sprintf("scenario_name = $%d", argNum))
		args = append(args, *update.ScenarioName)
		argNum++
	}
	if update.EndpointURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("endpoint_url = $%d", argNum))
		args = append(args, *update.EndpointURL)
		argNum++
	}
	if update.OrgID != nil {
		setClauses = append(setClauses, fmt.Sprintf("org_id = $%d", argNum))
		args = append(args, *update.OrgID)
		argNum++
	}
	if update.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argNum))
		args = append(args, *update.Status)
		argNum++
	}
	if update.IntervalSeconds != nil {
		setClauses = append(setClauses, fmt.Sprintf("interval_seconds = $%d", argNum))
		args = append(args, *update.IntervalSeconds)
		argNum++
	}
	if update.ScheduleEnabled != nil {
		setClauses = append(setClauses, fmt.Sprintf("schedule_enabled = $%d", argNum))
		args = append(args, *update.ScheduleEnabled)
		argNum++
	}
	if update.ConfigOverrides != nil {
		configOverridesJSON, _ := json.Marshal(update.ConfigOverrides)
		setClauses = append(setClauses, fmt.Sprintf("config_overrides = $%d", argNum))
		args = append(args, configOverridesJSON)
		argNum++
	}
	if update.LastRunAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("last_run_at = $%d", argNum))
		args = append(args, *update.LastRunAt)
		argNum++
	}
	if update.LastRunStatus != nil {
		setClauses = append(setClauses, fmt.Sprintf("last_run_status = $%d", argNum))
		args = append(args, *update.LastRunStatus)
		argNum++
	}

	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE benchmark_targets SET %s
		WHERE id = $%d
		RETURNING id, name, display_name, model_name, scenario_name, environment,
			endpoint_url, org_id, status, interval_seconds, schedule_enabled,
			config_overrides, last_run_at, last_run_status, created_at, updated_at
	`, joinStrings(setClauses, ", "), argNum)

	var target domain.BenchmarkTarget
	var configOverridesBytes []byte

	err := r.db.Pool().QueryRow(ctx, query, args...).Scan(
		&target.ID, &target.Name, &target.DisplayName, &target.ModelName,
		&target.ScenarioName, &target.Environment, &target.EndpointURL,
		&target.OrgID, &target.Status, &target.IntervalSeconds,
		&target.ScheduleEnabled, &configOverridesBytes, &target.LastRunAt,
		&target.LastRunStatus, &target.CreatedAt, &target.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update target: %w", err)
	}

	if err := json.Unmarshal(configOverridesBytes, &target.ConfigOverrides); err != nil {
		target.ConfigOverrides = make(map[string]interface{})
	}

	return &target, nil
}

// DeleteTarget removes a target by ID
func (r *BenchmarkRepository) DeleteTarget(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM benchmark_targets WHERE id = $1`
	_, err := r.db.Pool().Exec(ctx, query, id)
	return err
}

// ============================================================================
// Run Operations
// ============================================================================

// CreateRun creates a new benchmark run
func (r *BenchmarkRepository) CreateRun(ctx context.Context, create *domain.BenchmarkRunCreate) (*domain.BenchmarkRun, error) {
	triggeredBy := create.TriggeredBy
	if triggeredBy == "" {
		triggeredBy = "schedule"
	}

	run := &domain.BenchmarkRun{
		ID:              uuid.New(),
		TargetID:        create.TargetID,
		ScenarioName:    create.ScenarioName,
		Status:          "pending",
		TriggeredBy:     triggeredBy,
		TriggeredByUser: create.TriggeredByUser,
		CreatedAt:       time.Now().UTC(),
	}

	query := `
		INSERT INTO benchmark_runs (
			id, target_id, scenario_name, status, triggered_by,
			triggered_by_user, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.Pool().Exec(ctx, query,
		run.ID, run.TargetID, run.ScenarioName, run.Status,
		run.TriggeredBy, run.TriggeredByUser, run.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create run: %w", err)
	}

	return run, nil
}

// GetRunByID retrieves a run by ID
func (r *BenchmarkRepository) GetRunByID(ctx context.Context, id uuid.UUID) (*domain.BenchmarkRun, error) {
	query := `
		SELECT id, target_id, scenario_name, status, started_at, completed_at,
			duration_seconds, results, error_message, triggered_by,
			triggered_by_user, created_at
		FROM benchmark_runs
		WHERE id = $1
	`

	var run domain.BenchmarkRun
	var resultsBytes []byte

	err := r.db.Pool().QueryRow(ctx, query, id).Scan(
		&run.ID, &run.TargetID, &run.ScenarioName, &run.Status,
		&run.StartedAt, &run.CompletedAt, &run.DurationSeconds,
		&resultsBytes, &run.ErrorMessage, &run.TriggeredBy,
		&run.TriggeredByUser, &run.CreatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}

	if resultsBytes != nil {
		var results domain.BenchmarkResults
		if err := json.Unmarshal(resultsBytes, &results); err == nil {
			run.Results = &results
		}
	}

	return &run, nil
}

// ListRuns retrieves runs with filtering and pagination
func (r *BenchmarkRepository) ListRuns(ctx context.Context, params domain.BenchmarkRunListParams) (*domain.BenchmarkRunListResponse, error) {
	// Build query
	baseQuery := `FROM benchmark_runs br WHERE 1=1`
	args := []interface{}{}
	argNum := 1

	// Filter by org_id via target join
	if params.OrgID != nil {
		baseQuery += fmt.Sprintf(" AND br.target_id IN (SELECT id FROM benchmark_targets WHERE org_id = $%d)", argNum)
		args = append(args, *params.OrgID)
		argNum++
	}

	if params.TargetID != nil {
		baseQuery += fmt.Sprintf(" AND br.target_id = $%d", argNum)
		args = append(args, *params.TargetID)
		argNum++
	}

	if params.ScenarioName != "" {
		baseQuery += fmt.Sprintf(" AND br.scenario_name = $%d", argNum)
		args = append(args, params.ScenarioName)
		argNum++
	}

	if params.Status != "" {
		baseQuery += fmt.Sprintf(" AND br.status = $%d", argNum)
		args = append(args, params.Status)
		argNum++
	}

	if params.TriggeredBy != "" {
		baseQuery += fmt.Sprintf(" AND br.triggered_by = $%d", argNum)
		args = append(args, params.TriggeredBy)
		argNum++
	}

	if params.StartedAfter != nil {
		baseQuery += fmt.Sprintf(" AND br.started_at >= $%d", argNum)
		args = append(args, *params.StartedAfter)
		argNum++
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := r.db.Pool().QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count runs: %w", err)
	}

	// Fetch page
	limit := params.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	selectQuery := `
		SELECT br.id, br.target_id, br.scenario_name, br.status, br.started_at, br.completed_at,
			br.duration_seconds, br.results, br.error_message, br.triggered_by,
			br.triggered_by_user, br.created_at
	` + baseQuery + fmt.Sprintf(" ORDER BY br.created_at DESC LIMIT $%d OFFSET $%d", argNum, argNum+1)

	args = append(args, limit, params.Offset)

	rows, err := r.db.Pool().Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}
	defer rows.Close()

	runs := []domain.BenchmarkRun{}
	for rows.Next() {
		var run domain.BenchmarkRun
		var resultsBytes []byte

		if err := rows.Scan(
			&run.ID, &run.TargetID, &run.ScenarioName, &run.Status,
			&run.StartedAt, &run.CompletedAt, &run.DurationSeconds,
			&resultsBytes, &run.ErrorMessage, &run.TriggeredBy,
			&run.TriggeredByUser, &run.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}

		if resultsBytes != nil {
			var results domain.BenchmarkResults
			if err := json.Unmarshal(resultsBytes, &results); err == nil {
				run.Results = &results
			}
		}

		runs = append(runs, run)
	}

	response := &domain.BenchmarkRunListResponse{
		Runs: runs,
		Pagination: domain.Pagination{
			Total:  total,
			Limit:  limit,
			Offset: params.Offset,
		},
	}

	if params.Offset+len(runs) < total {
		nextOffset := params.Offset + limit
		response.Pagination.NextOffset = &nextOffset
	}

	return response, nil
}

// UpdateRun updates specific fields of a run
func (r *BenchmarkRepository) UpdateRun(ctx context.Context, id uuid.UUID, update *domain.BenchmarkRunUpdate) (*domain.BenchmarkRun, error) {
	setClauses := []string{}
	args := []interface{}{}
	argNum := 1

	if update.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argNum))
		args = append(args, *update.Status)
		argNum++
	}
	if update.StartedAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("started_at = $%d", argNum))
		args = append(args, *update.StartedAt)
		argNum++
	}
	if update.CompletedAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("completed_at = $%d", argNum))
		args = append(args, *update.CompletedAt)
		argNum++
	}
	if update.DurationSeconds != nil {
		setClauses = append(setClauses, fmt.Sprintf("duration_seconds = $%d", argNum))
		args = append(args, *update.DurationSeconds)
		argNum++
	}
	if update.Results != nil {
		resultsJSON, _ := json.Marshal(update.Results)
		setClauses = append(setClauses, fmt.Sprintf("results = $%d", argNum))
		args = append(args, resultsJSON)
		argNum++
	}
	if update.ErrorMessage != nil {
		setClauses = append(setClauses, fmt.Sprintf("error_message = $%d", argNum))
		args = append(args, *update.ErrorMessage)
		argNum++
	}

	if len(setClauses) == 0 {
		// No updates, just fetch
		return r.GetRunByID(ctx, id)
	}

	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE benchmark_runs SET %s
		WHERE id = $%d
		RETURNING id, target_id, scenario_name, status, started_at, completed_at,
			duration_seconds, results, error_message, triggered_by,
			triggered_by_user, created_at
	`, joinStrings(setClauses, ", "), argNum)

	var run domain.BenchmarkRun
	var resultsBytes []byte

	err := r.db.Pool().QueryRow(ctx, query, args...).Scan(
		&run.ID, &run.TargetID, &run.ScenarioName, &run.Status,
		&run.StartedAt, &run.CompletedAt, &run.DurationSeconds,
		&resultsBytes, &run.ErrorMessage, &run.TriggeredBy,
		&run.TriggeredByUser, &run.CreatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update run: %w", err)
	}

	if resultsBytes != nil {
		var results domain.BenchmarkResults
		if err := json.Unmarshal(resultsBytes, &results); err == nil {
			run.Results = &results
		}
	}

	return &run, nil
}

// DeleteRun removes a run by ID
func (r *BenchmarkRepository) DeleteRun(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM benchmark_runs WHERE id = $1`
	_, err := r.db.Pool().Exec(ctx, query, id)
	return err
}

// GetLatestRunForTarget retrieves the most recent run for a target
func (r *BenchmarkRepository) GetLatestRunForTarget(ctx context.Context, targetID uuid.UUID) (*domain.BenchmarkRun, error) {
	query := `
		SELECT id, target_id, scenario_name, status, started_at, completed_at,
			duration_seconds, results, error_message, triggered_by,
			triggered_by_user, created_at
		FROM benchmark_runs
		WHERE target_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var run domain.BenchmarkRun
	var resultsBytes []byte

	err := r.db.Pool().QueryRow(ctx, query, targetID).Scan(
		&run.ID, &run.TargetID, &run.ScenarioName, &run.Status,
		&run.StartedAt, &run.CompletedAt, &run.DurationSeconds,
		&resultsBytes, &run.ErrorMessage, &run.TriggeredBy,
		&run.TriggeredByUser, &run.CreatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest run: %w", err)
	}

	if resultsBytes != nil {
		var results domain.BenchmarkResults
		if err := json.Unmarshal(resultsBytes, &results); err == nil {
			run.Results = &results
		}
	}

	return &run, nil
}

// GetRunsForTargetSince retrieves runs for a target since a given time
func (r *BenchmarkRepository) GetRunsForTargetSince(ctx context.Context, targetID uuid.UUID, since time.Time, limit int) ([]domain.BenchmarkRun, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := `
		SELECT id, target_id, scenario_name, status, started_at, completed_at,
			duration_seconds, results, error_message, triggered_by,
			triggered_by_user, created_at
		FROM benchmark_runs
		WHERE target_id = $1 AND created_at >= $2
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := r.db.Pool().Query(ctx, query, targetID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get runs: %w", err)
	}
	defer rows.Close()

	runs := []domain.BenchmarkRun{}
	for rows.Next() {
		var run domain.BenchmarkRun
		var resultsBytes []byte

		if err := rows.Scan(
			&run.ID, &run.TargetID, &run.ScenarioName, &run.Status,
			&run.StartedAt, &run.CompletedAt, &run.DurationSeconds,
			&resultsBytes, &run.ErrorMessage, &run.TriggeredBy,
			&run.TriggeredByUser, &run.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}

		if resultsBytes != nil {
			var results domain.BenchmarkResults
			if err := json.Unmarshal(resultsBytes, &results); err == nil {
				run.Results = &results
			}
		}

		runs = append(runs, run)
	}

	return runs, nil
}
