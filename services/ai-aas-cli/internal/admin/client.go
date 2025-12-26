// Package admin provides shared client utilities for admin commands.
package admin

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/deploymentregistry"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
)

// newRegistryClient creates a deployment registry client from config.
func newRegistryClient(cfg *config.Config) *deploymentregistry.Client {
	endpoint := cfg.AdminAPIEndpoint
	if endpoint == "" {
		endpoint = cfg.APIEndpoint
	}

	opts := []api.ClientOption{}
	if cfg.TLSInsecure {
		opts = append(opts, api.WithInsecureSkipVerify())
	}
	if cfg.Timeout > 0 {
		opts = append(opts, api.WithTimeout(time.Duration(cfg.Timeout)*time.Second))
	}

	apiClient := api.NewClient(endpoint, cfg.APIKey, opts...)
	return deploymentregistry.NewClient(apiClient)
}

// isNotFoundError checks if an error indicates a not found condition using typed error checking.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// listAllModels fetches all models with pagination.
func listAllModels(ctx context.Context, client *deploymentregistry.Client, environment, status string) ([]deploymentregistry.Model, error) {
	const pageSize = 100
	var allModels []deploymentregistry.Model
	offset := 0

	for {
		resp, err := client.List(ctx, deploymentregistry.ListParams{
			Environment: environment,
			Status:      status,
			Limit:       pageSize,
			Offset:      offset,
		})
		if err != nil {
			return nil, err
		}

		allModels = append(allModels, resp.Models...)

		// Check if we've fetched all models
		if resp.Pagination.NextOffset == nil || len(resp.Models) < pageSize {
			break
		}

		offset = *resp.Pagination.NextOffset
	}

	return allModels, nil
}
