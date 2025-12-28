package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// AuditEvent represents an audit log event.
type AuditEvent struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Action      string    `json:"action"`
	ResourceType string   `json:"resource_type"`
	ResourceID  string    `json:"resource_id"`
	ActorType   string    `json:"actor_type"`
	ActorID     string    `json:"actor_id"`
	ActorEmail  string    `json:"actor_email"`
	IPAddress   string    `json:"ip_address,omitempty"`
	UserAgent   string    `json:"user_agent,omitempty"`
	Details     string    `json:"details,omitempty"`
	Status      string    `json:"status"`
}

// AuditEventsResponse is the response from listing audit events.
type AuditEventsResponse struct {
	Events     []AuditEvent `json:"events"`
	TotalCount int          `json:"total_count"`
	HasMore    bool         `json:"has_more"`
}

// AuditFilters contains filters for audit log queries.
type AuditFilters struct {
	Action       string
	ResourceType string
	ActorID      string
	StartDate    string
	EndDate      string
	Limit        int
}

// ListAuditEvents lists audit events for the organization.
func (c *Client) ListAuditEvents(ctx context.Context, orgID string, filters *AuditFilters) (*AuditEventsResponse, error) {
	path := fmt.Sprintf("/v1/orgs/%s/audit-logs", url.PathEscape(orgID))

	params := url.Values{}
	if filters != nil {
		if filters.Action != "" {
			params.Set("action", filters.Action)
		}
		if filters.ResourceType != "" {
			params.Set("resource_type", filters.ResourceType)
		}
		if filters.ActorID != "" {
			params.Set("actor_id", filters.ActorID)
		}
		if filters.StartDate != "" {
			params.Set("start_date", filters.StartDate)
		}
		if filters.EndDate != "" {
			params.Set("end_date", filters.EndDate)
		}
		if filters.Limit > 0 {
			params.Set("limit", fmt.Sprintf("%d", filters.Limit))
		}
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var result AuditEventsResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
