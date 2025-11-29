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

// EnableDeploymentWithHistory enables a deployment and records the action
func (s *Service) EnableDeploymentWithHistory(ctx context.Context, modelName, environment, enabledBy, reason string) error {
	// Get the deployment first
	deployment, err := s.GetDeployment(ctx, modelName, environment)
	if err != nil {
		return err
	}

	// Enable the deployment
	if err := s.EnableDeployment(ctx, modelName, environment, enabledBy); err != nil {
		return err
	}

	// Record the state change
	_, err = s.RecordStateChange(ctx, RecordStateChangeRequest{
		DeploymentID: deployment.ID,
		Action:       StateActionEnabled,
		PerformedBy:  enabledBy,
		Reason:       reason,
	})

	return err
}

// DisableDeploymentWithHistory disables a deployment and records the action
func (s *Service) DisableDeploymentWithHistory(ctx context.Context, modelName, environment, disabledBy, reason string) error {
	// Get the deployment first
	deployment, err := s.GetDeployment(ctx, modelName, environment)
	if err != nil {
		return err
	}

	// Disable the deployment
	if err := s.DisableDeployment(ctx, modelName, environment, disabledBy); err != nil {
		return err
	}

	// Record the state change
	_, err = s.RecordStateChange(ctx, RecordStateChangeRequest{
		DeploymentID: deployment.ID,
		Action:       StateActionDisabled,
		PerformedBy:  disabledBy,
		Reason:       reason,
	})

	return err
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
