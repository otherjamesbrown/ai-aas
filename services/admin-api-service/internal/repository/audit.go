package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
)

// AuditRepository handles audit log database operations
type AuditRepository struct {
	db *DB
}

// NewAuditRepository creates a new audit repository
func NewAuditRepository(db *DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// Create creates a new audit log entry
func (r *AuditRepository) Create(ctx context.Context, create *domain.AuditLogCreate) error {
	var changesJSON []byte
	if create.Changes != nil {
		var err error
		changesJSON, err = json.Marshal(create.Changes)
		if err != nil {
			changesJSON = nil
		}
	}

	var clientIP *net.IP
	if create.ClientIP != "" {
		ip := net.ParseIP(create.ClientIP)
		if ip != nil {
			clientIP = &ip
		}
	}

	var requestID *uuid.UUID
	if create.RequestID != "" {
		if id, err := uuid.Parse(create.RequestID); err == nil {
			requestID = &id
		}
	}

	var userAgent *string
	if create.UserAgent != "" {
		userAgent = &create.UserAgent
	}

	query := `
		INSERT INTO admin_api_audit_logs (
			timestamp, actor, action, resource_type, resource_id,
			outcome, changes, error_detail, client_ip, user_agent, request_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.Pool().Exec(ctx, query,
		time.Now().UTC(), create.Actor, create.Action, create.ResourceType,
		create.ResourceID, create.Outcome, changesJSON, create.ErrorDetail,
		clientIP, userAgent, requestID,
	)

	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

// List retrieves audit logs with filtering and pagination
func (r *AuditRepository) List(ctx context.Context, params domain.AuditLogListParams) (*domain.AuditLogListResponse, error) {
	baseQuery := `FROM admin_api_audit_logs WHERE 1=1`
	args := []interface{}{}
	argNum := 1

	if params.Actor != "" {
		baseQuery += fmt.Sprintf(" AND actor = $%d", argNum)
		args = append(args, params.Actor)
		argNum++
	}

	if params.ResourceType != "" {
		baseQuery += fmt.Sprintf(" AND resource_type = $%d", argNum)
		args = append(args, params.ResourceType)
		argNum++
	}

	if params.ResourceID != "" {
		baseQuery += fmt.Sprintf(" AND resource_id = $%d", argNum)
		args = append(args, params.ResourceID)
		argNum++
	}

	if params.From != nil {
		baseQuery += fmt.Sprintf(" AND timestamp >= $%d", argNum)
		args = append(args, *params.From)
		argNum++
	}

	if params.To != nil {
		baseQuery += fmt.Sprintf(" AND timestamp <= $%d", argNum)
		args = append(args, *params.To)
		argNum++
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := r.db.Pool().QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Fetch page
	limit := params.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	selectQuery := `
		SELECT id, timestamp, actor, action, resource_type, resource_id,
			outcome, changes, error_detail, client_ip, user_agent, request_id
	` + baseQuery + fmt.Sprintf(" ORDER BY timestamp DESC LIMIT $%d OFFSET $%d", argNum, argNum+1)

	args = append(args, limit, params.Offset)

	rows, err := r.db.Pool().Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()

	logs := []domain.AuditLog{}
	for rows.Next() {
		var log domain.AuditLog
		var changesJSON []byte

		if err := rows.Scan(
			&log.ID, &log.Timestamp, &log.Actor, &log.Action, &log.ResourceType,
			&log.ResourceID, &log.Outcome, &changesJSON, &log.ErrorDetail,
			&log.ClientIP, &log.UserAgent, &log.RequestID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}

		if changesJSON != nil {
			json.Unmarshal(changesJSON, &log.Changes)
		}

		logs = append(logs, log)
	}

	response := &domain.AuditLogListResponse{
		AuditLogs: logs,
		Pagination: domain.Pagination{
			Total:  total,
			Limit:  limit,
			Offset: params.Offset,
		},
	}

	if params.Offset+len(logs) < total {
		nextOffset := params.Offset + limit
		response.Pagination.NextOffset = &nextOffset
	}

	return response, nil
}
