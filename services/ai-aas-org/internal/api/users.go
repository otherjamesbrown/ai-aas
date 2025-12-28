package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// User represents a user in the organization.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	OrgID     string    `json:"org_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateUserRequest is the request to create a new user.
type CreateUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role,omitempty"`
}

// CreateUserResponse is the response from creating a user.
type CreateUserResponse struct {
	User         User   `json:"user"`
	TemporaryKey string `json:"temporary_key,omitempty"`
}

// ListUsersResponse is the response from listing users.
type ListUsersResponse struct {
	Users      []User `json:"users"`
	TotalCount int    `json:"total_count"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}

// ListUsers lists all users in the organization.
func (c *Client) ListUsers(ctx context.Context, orgID string, page, pageSize int) (*ListUsersResponse, error) {
	path := fmt.Sprintf("/v1/orgs/%s/users", url.PathEscape(orgID))
	if page > 0 || pageSize > 0 {
		params := url.Values{}
		if page > 0 {
			params.Set("page", fmt.Sprintf("%d", page))
		}
		if pageSize > 0 {
			params.Set("page_size", fmt.Sprintf("%d", pageSize))
		}
		path += "?" + params.Encode()
	}

	var result ListUsersResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetUser gets a user by ID.
func (c *Client) GetUser(ctx context.Context, orgID, userID string) (*User, error) {
	path := fmt.Sprintf("/v1/orgs/%s/users/%s", url.PathEscape(orgID), url.PathEscape(userID))

	var result User
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetUserByEmail gets a user by email.
func (c *Client) GetUserByEmail(ctx context.Context, orgID, email string) (*User, error) {
	path := fmt.Sprintf("/v1/orgs/%s/users/by-email/%s", url.PathEscape(orgID), url.PathEscape(email))

	var result User
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateUser creates a new user in the organization.
func (c *Client) CreateUser(ctx context.Context, orgID string, req *CreateUserRequest) (*CreateUserResponse, error) {
	path := fmt.Sprintf("/v1/orgs/%s/users", url.PathEscape(orgID))

	var result CreateUserResponse
	if err := c.doRequest(ctx, http.MethodPost, path, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateUserRequest is the request to update a user.
type UpdateUserRequest struct {
	Name   string `json:"name,omitempty"`
	Role   string `json:"role,omitempty"`
	Status string `json:"status,omitempty"`
}

// UpdateUser updates a user.
func (c *Client) UpdateUser(ctx context.Context, orgID, userID string, req *UpdateUserRequest) (*User, error) {
	path := fmt.Sprintf("/v1/orgs/%s/users/%s", url.PathEscape(orgID), url.PathEscape(userID))

	var result User
	if err := c.doRequest(ctx, http.MethodPatch, path, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteUser deletes a user from the organization.
func (c *Client) DeleteUser(ctx context.Context, orgID, userID string) error {
	path := fmt.Sprintf("/v1/orgs/%s/users/%s", url.PathEscape(orgID), url.PathEscape(userID))
	return c.doRequest(ctx, http.MethodDelete, path, nil, nil)
}
