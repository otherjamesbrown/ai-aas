package domain

import (
	"net"
	"time"

	"github.com/google/uuid"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID           int64      `json:"id"`
	Timestamp    time.Time  `json:"timestamp"`
	Actor        string     `json:"actor"`
	Action       string     `json:"action"`
	ResourceType string     `json:"resource_type"`
	ResourceID   string     `json:"resource_id"`
	Outcome      string     `json:"outcome"`
	Changes      any        `json:"changes,omitempty"`
	ErrorDetail  *string    `json:"error_detail,omitempty"`
	ClientIP     *net.IP    `json:"client_ip,omitempty"`
	UserAgent    *string    `json:"user_agent,omitempty"`
	RequestID    *uuid.UUID `json:"request_id,omitempty"`
}

// AuditLogCreate represents data for creating an audit log entry
type AuditLogCreate struct {
	Actor        string
	Action       string
	ResourceType string
	ResourceID   string
	Outcome      string
	Changes      any
	ErrorDetail  *string
	ClientIP     string
	UserAgent    string
	RequestID    string
}

// AuditLogListParams represents query parameters for listing audit logs
type AuditLogListParams struct {
	Actor        string
	ResourceType string
	ResourceID   string
	From         *time.Time
	To           *time.Time
	Limit        int
	Offset       int
}

// AuditLogListResponse represents a paginated list of audit logs
type AuditLogListResponse struct {
	AuditLogs  []AuditLog `json:"audit_logs"`
	Pagination Pagination `json:"pagination"`
}

// Audit actions
const (
	ActionModelCreate      = "model.create"
	ActionModelUpdate      = "model.update"
	ActionModelDelete      = "model.delete"
	ActionOrgCreate        = "organization.create"
	ActionOrgUpdate        = "organization.update"
	ActionPolicyCreate     = "routing_policy.create"
	ActionPolicyUpdate     = "routing_policy.update"
	ActionPolicyDelete     = "routing_policy.delete"
	ActionPolicyActivate   = "routing_policy.activate"
	ActionPolicyDeactivate = "routing_policy.deactivate"
)

// Audit outcomes
const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
)

// Resource types
const (
	ResourceTypeModel        = "model"
	ResourceTypeOrganization = "organization"
	ResourceTypePolicy       = "routing_policy"
)
