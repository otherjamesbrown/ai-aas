package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// UsageSummary represents aggregated usage data.
type UsageSummary struct {
	OrgID           string    `json:"org_id"`
	Period          string    `json:"period"`
	StartDate       time.Time `json:"start_date"`
	EndDate         time.Time `json:"end_date"`
	TotalTokens     int64     `json:"total_tokens"`
	PromptTokens    int64     `json:"prompt_tokens"`
	CompletionTokens int64    `json:"completion_tokens"`
	TotalRequests   int64     `json:"total_requests"`
	TotalCost       float64   `json:"total_cost"`
	Currency        string    `json:"currency"`
}

// UsageByModel represents usage broken down by model.
type UsageByModel struct {
	ModelID          string  `json:"model_id"`
	ModelName        string  `json:"model_name"`
	TotalTokens      int64   `json:"total_tokens"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalRequests    int64   `json:"total_requests"`
	TotalCost        float64 `json:"total_cost"`
}

// UsageByUser represents usage broken down by user.
type UsageByUser struct {
	UserID           string  `json:"user_id"`
	UserEmail        string  `json:"user_email"`
	UserName         string  `json:"user_name"`
	TotalTokens      int64   `json:"total_tokens"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalRequests    int64   `json:"total_requests"`
	TotalCost        float64 `json:"total_cost"`
}

// UsageReport is the full usage report response.
type UsageReport struct {
	Summary  UsageSummary   `json:"summary"`
	ByModel  []UsageByModel `json:"by_model"`
	ByUser   []UsageByUser  `json:"by_user"`
}

// GetUsageSummary gets the usage summary for the organization.
func (c *Client) GetUsageSummary(ctx context.Context, orgID, period string) (*UsageSummary, error) {
	path := fmt.Sprintf("/v1/orgs/%s/usage/summary", url.PathEscape(orgID))
	if period != "" {
		path += "?period=" + url.QueryEscape(period)
	}

	var result UsageSummary
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetUsageByModel gets usage broken down by model.
func (c *Client) GetUsageByModel(ctx context.Context, orgID, period string) ([]UsageByModel, error) {
	path := fmt.Sprintf("/v1/orgs/%s/usage/by-model", url.PathEscape(orgID))
	if period != "" {
		path += "?period=" + url.QueryEscape(period)
	}

	var result struct {
		Models []UsageByModel `json:"models"`
	}
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return result.Models, nil
}

// GetUsageByUser gets usage broken down by user.
func (c *Client) GetUsageByUser(ctx context.Context, orgID, period string) ([]UsageByUser, error) {
	path := fmt.Sprintf("/v1/orgs/%s/usage/by-user", url.PathEscape(orgID))
	if period != "" {
		path += "?period=" + url.QueryEscape(period)
	}

	var result struct {
		Users []UsageByUser `json:"users"`
	}
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return result.Users, nil
}

// GetUsageReport gets the full usage report.
func (c *Client) GetUsageReport(ctx context.Context, orgID, period string) (*UsageReport, error) {
	path := fmt.Sprintf("/v1/orgs/%s/usage", url.PathEscape(orgID))
	if period != "" {
		path += "?period=" + url.QueryEscape(period)
	}

	var result UsageReport
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
