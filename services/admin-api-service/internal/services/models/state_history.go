// Package models provides the business logic for model management operations.
package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// StateHistoryEntry represents an audit entry for state changes
type StateHistoryEntry struct {
	ID           uuid.UUID
	DeploymentID uuid.UUID
	Action       string
	PerformedBy  string
	Reason       string
	ScheduledAt  *time.Time
	ExecutedAt   time.Time
}

// State action constants
const (
	StateActionEnabled    = "enabled"
	StateActionDisabled   = "disabled"
	StateActionSwappedOut = "swapped_out"
	StateActionSwappedIn  = "swapped_in"
)

// RecordStateChangeRequest contains data for recording a state change
type RecordStateChangeRequest struct {
	DeploymentID uuid.UUID
	Action       string
	PerformedBy  string
	Reason       string
	ScheduledAt  *time.Time
}

// RecordStateChange records a state change in the audit history
func (s *Service) RecordStateChange(ctx context.Context, req RecordStateChangeRequest) (*StateHistoryEntry, error) {
	query := `
		INSERT INTO model_state_history (deployment_id, action, performed_by, reason, scheduled_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, executed_at
	`

	entry := StateHistoryEntry{
		DeploymentID: req.DeploymentID,
		Action:       req.Action,
		PerformedBy:  req.PerformedBy,
		Reason:       req.Reason,
		ScheduledAt:  req.ScheduledAt,
	}

	err := s.pool.QueryRow(ctx, query,
		entry.DeploymentID, entry.Action, entry.PerformedBy, entry.Reason, entry.ScheduledAt,
	).Scan(&entry.ID, &entry.ExecutedAt)

	if err != nil {
		return nil, fmt.Errorf("record state change: %w", err)
	}

	return &entry, nil
}

// GetStateHistory returns state history for a deployment
func (s *Service) GetStateHistory(ctx context.Context, modelName, environment string, limit int) ([]StateHistoryEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT h.id, h.deployment_id, h.action, h.performed_by, h.reason, h.scheduled_at, h.executed_at
		FROM model_state_history h
		JOIN model_deployments d ON h.deployment_id = d.id
		JOIN model_registry r ON d.model_id = r.id
		WHERE r.name = $1 AND d.environment = $2
		ORDER BY h.executed_at DESC
		LIMIT $3
	`

	rows, err := s.pool.Query(ctx, query, modelName, environment, limit)
	if err != nil {
		return nil, fmt.Errorf("query state history: %w", err)
	}
	defer rows.Close()

	var entries []StateHistoryEntry
	for rows.Next() {
		var e StateHistoryEntry
		var performedBy, reason *string
		err := rows.Scan(
			&e.ID, &e.DeploymentID, &e.Action, &performedBy, &reason,
			&e.ScheduledAt, &e.ExecutedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan state history: %w", err)
		}
		if performedBy != nil {
			e.PerformedBy = *performedBy
		}
		if reason != nil {
			e.Reason = *reason
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}

// EnableDeploymentWithHistory enables a deployment and records the action atomically
func (s *Service) EnableDeploymentWithHistory(ctx context.Context, modelName, environment, enabledBy, reason string) error {
	// Use a transaction to ensure atomicity
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback is a no-op if tx is committed

	// Get the deployment first
	var deploymentID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT d.id FROM model_deployments d
		JOIN model_registry r ON d.model_id = r.id
		WHERE r.name = $1 AND d.environment = $2
	`, modelName, environment).Scan(&deploymentID)
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	// Enable the deployment
	_, err = tx.Exec(ctx, `
		UPDATE model_deployments
		SET enabled = true, enabled_by = $1, enabled_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`, enabledBy, deploymentID)
	if err != nil {
		return fmt.Errorf("enable deployment: %w", err)
	}

	// Record the state change
	_, err = tx.Exec(ctx, `
		INSERT INTO model_state_history (deployment_id, action, performed_by, reason)
		VALUES ($1, $2, $3, $4)
	`, deploymentID, StateActionEnabled, enabledBy, reason)
	if err != nil {
		return fmt.Errorf("record state change: %w", err)
	}

	return tx.Commit(ctx)
}

// DisableDeploymentWithHistory disables a deployment and records the action atomically
func (s *Service) DisableDeploymentWithHistory(ctx context.Context, modelName, environment, disabledBy, reason string) error {
	// Use a transaction to ensure atomicity
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback is a no-op if tx is committed

	// Get the deployment first
	var deploymentID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT d.id FROM model_deployments d
		JOIN model_registry r ON d.model_id = r.id
		WHERE r.name = $1 AND d.environment = $2
	`, modelName, environment).Scan(&deploymentID)
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	// Disable the deployment
	_, err = tx.Exec(ctx, `
		UPDATE model_deployments
		SET enabled = false, disabled_by = $1, disabled_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`, disabledBy, deploymentID)
	if err != nil {
		return fmt.Errorf("disable deployment: %w", err)
	}

	// Record the state change
	_, err = tx.Exec(ctx, `
		INSERT INTO model_state_history (deployment_id, action, performed_by, reason)
		VALUES ($1, $2, $3, $4)
	`, deploymentID, StateActionDisabled, disabledBy, reason)
	if err != nil {
		return fmt.Errorf("record state change: %w", err)
	}

	return tx.Commit(ctx)
}

// GetAllStateHistory returns recent state changes across all deployments
func (s *Service) GetAllStateHistory(ctx context.Context, limit int) ([]StateHistoryWithModel, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT h.id, h.deployment_id, h.action, h.performed_by, h.reason, h.scheduled_at, h.executed_at,
		       r.name as model_name, d.environment
		FROM model_state_history h
		JOIN model_deployments d ON h.deployment_id = d.id
		JOIN model_registry r ON d.model_id = r.id
		ORDER BY h.executed_at DESC
		LIMIT $1
	`

	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query all state history: %w", err)
	}
	defer rows.Close()

	var entries []StateHistoryWithModel
	for rows.Next() {
		var e StateHistoryWithModel
		var performedBy, reason *string
		err := rows.Scan(
			&e.ID, &e.DeploymentID, &e.Action, &performedBy, &reason,
			&e.ScheduledAt, &e.ExecutedAt, &e.ModelName, &e.Environment,
		)
		if err != nil {
			return nil, fmt.Errorf("scan state history: %w", err)
		}
		if performedBy != nil {
			e.PerformedBy = *performedBy
		}
		if reason != nil {
			e.Reason = *reason
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}

// StateHistoryWithModel extends StateHistoryEntry with model info
type StateHistoryWithModel struct {
	StateHistoryEntry
	ModelName   string
	Environment string
}
