// Package huggingface provides a client for interacting with the HuggingFace Hub API.
package huggingface

import (
	"context"
	"fmt"
	"strings"
)

// LicenseInfo contains license information for a model
type LicenseInfo struct {
	IsGated       bool   // Model requires license acceptance
	LicenseType   string // License identifier (e.g., "llama3", "apache-2.0")
	LicenseURL    string // URL to view/accept license
	RequiresAuth  bool   // Model requires authentication to access
	CanAccess     bool   // Current user has access (if authenticated)
	AcceptanceURL string // URL to accept the license on HuggingFace
}

// GetLicenseInfo retrieves license information for a model
func (c *Client) GetLicenseInfo(ctx context.Context, modelID string) (*LicenseInfo, error) {
	info, err := c.GetModel(ctx, modelID)
	if err != nil {
		return nil, err
	}

	license := &LicenseInfo{
		IsGated:      info.Gated,
		RequiresAuth: info.Private || info.Gated,
	}

	// Extract license type from card data
	if info.CardData.License != nil {
		switch v := info.CardData.License.(type) {
		case string:
			license.LicenseType = v
		case []interface{}:
			// Multiple licenses - take first
			if len(v) > 0 {
				if s, ok := v[0].(string); ok {
					license.LicenseType = s
				}
			}
		}
	}

	// Build license URLs
	if license.LicenseType != "" {
		license.LicenseURL = fmt.Sprintf("https://huggingface.co/%s/blob/main/LICENSE", modelID)
	}

	if info.CardData.LicenseLink != "" {
		license.LicenseURL = info.CardData.LicenseLink
	}

	if license.IsGated {
		license.AcceptanceURL = fmt.Sprintf("https://huggingface.co/%s", modelID)
	}

	// Check if user can access (if token provided)
	if c.token != "" {
		// If we got here without ErrForbidden, user has access
		license.CanAccess = true
	}

	return license, nil
}

// CheckAccess verifies if the current user can access a model
func (c *Client) CheckAccess(ctx context.Context, modelID string) (bool, error) {
	_, err := c.GetModel(ctx, modelID)
	if err == ErrForbidden || err == ErrUnauthorized {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// IsGatedModel checks if a model is gated (requires license acceptance)
func (c *Client) IsGatedModel(ctx context.Context, modelID string) (bool, error) {
	info, err := c.GetModel(ctx, modelID)
	if err != nil {
		return false, err
	}
	return info.Gated, nil
}

// RequiresAuth checks if a model requires authentication
func (c *Client) RequiresAuth(ctx context.Context, modelID string) (bool, error) {
	info, err := c.GetModel(ctx, modelID)
	if err != nil {
		return false, err
	}
	return info.Private || info.Gated, nil
}

// CommonLicenses maps common license identifiers to their full names
var CommonLicenses = map[string]string{
	"apache-2.0":                "Apache License 2.0",
	"mit":                       "MIT License",
	"gpl-3.0":                   "GNU General Public License v3.0",
	"llama2":                    "Meta Llama 2 Community License",
	"llama3":                    "Meta Llama 3 Community License",
	"llama3.1":                  "Meta Llama 3.1 Community License",
	"llama3.2":                  "Meta Llama 3.2 Community License",
	"cc-by-4.0":                 "Creative Commons Attribution 4.0",
	"cc-by-nc-4.0":              "Creative Commons Attribution Non-Commercial 4.0",
	"cc-by-sa-4.0":              "Creative Commons Attribution Share-Alike 4.0",
	"cc-by-nc-nd-4.0":           "Creative Commons Attribution Non-Commercial No-Derivatives 4.0",
	"openrail":                  "OpenRAIL License",
	"openrail++":                "OpenRAIL++ License",
	"bigscience-bloom-rail-1.0": "BigScience BLOOM RAIL 1.0",
	"bigcode-openrail-m":        "BigCode OpenRAIL-M",
	"gemma":                     "Gemma Terms of Use",
	"qwen":                      "Tongyi Qianwen License",
}

// GetLicenseFullName returns the full name for a license identifier
func GetLicenseFullName(licenseID string) string {
	if name, ok := CommonLicenses[strings.ToLower(licenseID)]; ok {
		return name
	}
	return licenseID
}

// IsRestrictiveLicense checks if a license is considered restrictive
func IsRestrictiveLicense(licenseID string) bool {
	restrictive := []string{
		"llama2", "llama3", "llama3.1", "llama3.2",
		"gemma", "qwen",
		"cc-by-nc", "cc-by-nc-nd",
	}

	lower := strings.ToLower(licenseID)
	for _, r := range restrictive {
		if strings.Contains(lower, r) {
			return true
		}
	}
	return false
}
