package domain

import (
	"time"

	"github.com/google/uuid"
)

// Organization represents an organization entity
type Organization struct {
	OrganizationID    uuid.UUID `json:"organization_id"`
	Slug              string    `json:"slug"`
	DisplayName       string    `json:"display_name"`
	PlanTier          string    `json:"plan_tier"`
	Status            string    `json:"status"`
	BudgetLimitTokens *int64    `json:"budget_limit_tokens,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// OrganizationCreate represents a request to create an organization
type OrganizationCreate struct {
	Slug              string `json:"slug"`
	DisplayName       string `json:"display_name"`
	PlanTier          string `json:"plan_tier,omitempty"`
	BudgetLimitTokens *int64 `json:"budget_limit_tokens,omitempty"`
}

// OrganizationUpdate represents a partial organization update
type OrganizationUpdate struct {
	DisplayName       *string `json:"display_name,omitempty"`
	PlanTier          *string `json:"plan_tier,omitempty"`
	Status            *string `json:"status,omitempty"`
	BudgetLimitTokens *int64  `json:"budget_limit_tokens,omitempty"`
}

// OrganizationListParams represents query parameters for listing organizations
type OrganizationListParams struct {
	Limit  int
	Offset int
}

// OrganizationListResponse represents a paginated list of organizations
type OrganizationListResponse struct {
	Organizations []Organization `json:"organizations"`
	Pagination    Pagination     `json:"pagination"`
}

// Valid plan tiers - must match DB CHECK constraint
var ValidPlanTiers = []string{"starter", "growth", "enterprise"}

// Valid organization statuses - must match DB CHECK constraint
var ValidOrgStatuses = []string{"active", "suspended", "closed"}

// IsValidPlanTier checks if a plan tier is valid
func IsValidPlanTier(tier string) bool {
	for _, t := range ValidPlanTiers {
		if t == tier {
			return true
		}
	}
	return false
}

// IsValidOrgStatus checks if an organization status is valid
func IsValidOrgStatus(status string) bool {
	for _, s := range ValidOrgStatuses {
		if s == status {
			return true
		}
	}
	return false
}
