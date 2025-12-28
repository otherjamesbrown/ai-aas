package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Model represents an AI model available to the organization.
type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Description string `json:"description"`
	Status      string `json:"status"`
	ContextSize int    `json:"context_size"`
}

// UserModelAccess represents a user's access to a model.
type UserModelAccess struct {
	UserID    string `json:"user_id"`
	ModelID   string `json:"model_id"`
	ModelName string `json:"model_name"`
	GrantedAt string `json:"granted_at"`
	GrantedBy string `json:"granted_by"`
}

// ListModelsResponse is the response from listing models.
type ListModelsResponse struct {
	Models []Model `json:"models"`
}

// ListModels lists all models available to the organization.
func (c *Client) ListModels(ctx context.Context, orgID string) (*ListModelsResponse, error) {
	path := fmt.Sprintf("/v1/orgs/%s/models", url.PathEscape(orgID))

	var result ListModelsResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetModel gets a model by ID.
func (c *Client) GetModel(ctx context.Context, orgID, modelID string) (*Model, error) {
	path := fmt.Sprintf("/v1/orgs/%s/models/%s", url.PathEscape(orgID), url.PathEscape(modelID))

	var result Model
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListUserModelsResponse is the response from listing user model access.
type ListUserModelsResponse struct {
	Models []UserModelAccess `json:"models"`
}

// ListUserModels lists models a user has access to.
func (c *Client) ListUserModels(ctx context.Context, orgID, userID string) (*ListUserModelsResponse, error) {
	path := fmt.Sprintf("/v1/orgs/%s/users/%s/models", url.PathEscape(orgID), url.PathEscape(userID))

	var result ListUserModelsResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GrantModelAccessRequest is the request to grant model access.
type GrantModelAccessRequest struct {
	ModelID string `json:"model_id"`
}

// GrantModelAccess grants a user access to a model.
func (c *Client) GrantModelAccess(ctx context.Context, orgID, userID, modelID string) error {
	path := fmt.Sprintf("/v1/orgs/%s/users/%s/models", url.PathEscape(orgID), url.PathEscape(userID))
	req := GrantModelAccessRequest{ModelID: modelID}
	return c.doRequest(ctx, http.MethodPost, path, req, nil)
}

// RevokeModelAccess revokes a user's access to a model.
func (c *Client) RevokeModelAccess(ctx context.Context, orgID, userID, modelID string) error {
	path := fmt.Sprintf("/v1/orgs/%s/users/%s/models/%s", url.PathEscape(orgID), url.PathEscape(userID), url.PathEscape(modelID))
	return c.doRequest(ctx, http.MethodDelete, path, nil, nil)
}

// GrantAllModelsAccess grants a user access to all available models.
func (c *Client) GrantAllModelsAccess(ctx context.Context, orgID, userID string) error {
	path := fmt.Sprintf("/v1/orgs/%s/users/%s/models/all", url.PathEscape(orgID), url.PathEscape(userID))
	return c.doRequest(ctx, http.MethodPost, path, nil, nil)
}
