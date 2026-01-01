package postgres

import (
	"time"

	"github.com/google/uuid"
)

type Org struct {
	ID                    uuid.UUID
	Slug                  string
	Name                  string
	Status                string
	BillingOwnerUserID    *uuid.UUID
	BudgetPolicyID        *uuid.UUID
	DeclarativeMode       string
	DeclarativeRepoURL    *string
	DeclarativeBranch     *string
	DeclarativeLastCommit *string
	MFARequiredRoles      []string
	Metadata              map[string]any
	Version               int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             *time.Time
}

type CreateOrgParams struct {
	ID                    uuid.UUID
	Slug                  string
	Name                  string
	Status                string
	BillingOwnerUserID    *uuid.UUID
	BudgetPolicyID        *uuid.UUID
	DeclarativeMode       string
	DeclarativeRepoURL    *string
	DeclarativeBranch     *string
	DeclarativeLastCommit *string
	MFARequiredRoles      []string
	Metadata              map[string]any
}

type UpdateOrgParams struct {
	ID                    uuid.UUID
	Version               int64
	Name                  string
	Status                string
	BillingOwnerUserID    *uuid.UUID
	BudgetPolicyID        *uuid.UUID
	DeclarativeMode       string
	DeclarativeRepoURL    *string
	DeclarativeBranch     *string
	DeclarativeLastCommit *string
	MFARequiredRoles      []string
	Metadata              map[string]any
}

type User struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	Email          string
	DisplayName    string
	PasswordHash   string
	Status         string
	MFAEnrolled    bool
	MFAMethods     []string
	MFASecret      *string
	LastLoginAt    *time.Time
	LockoutUntil   *time.Time
	RecoveryTokens []string
	ExternalIDP    *string
	Metadata       map[string]any
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type CreateUserParams struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	Email          string
	DisplayName    string
	PasswordHash   string
	Status         string
	MFAEnrolled    bool
	MFAMethods     []string
	MFASecret      *string
	LastLoginAt    *time.Time
	LockoutUntil   *time.Time
	RecoveryTokens []string
	ExternalIDP    *string
	Metadata       map[string]any
}

type UpdateUserStatusParams struct {
	OrgID        uuid.UUID
	ID           uuid.UUID
	Version      int64
	Status       string
	LockoutUntil *time.Time
}

type UpdateUserProfileParams struct {
	OrgID       uuid.UUID
	ID          uuid.UUID
	Version     int64
	DisplayName string
	MFAEnrolled bool
	MFAMethods  []string
	MFASecret   *string
	Metadata    map[string]any
}

type UpdateUserPasswordHashParams struct {
	OrgID        uuid.UUID
	ID           uuid.UUID
	Version      int64
	PasswordHash string
}

type ServiceAccount struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	Name           string
	Description    *string
	Status         string
	Metadata       map[string]any
	LastRotationAt *time.Time
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type CreateServiceAccountParams struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	Name           string
	Description    *string
	Status         string
	Metadata       map[string]any
	LastRotationAt *time.Time
}

type UpdateServiceAccountParams struct {
	ID             uuid.UUID
	Version        int64
	Description    *string
	Status         string
	Metadata       map[string]any
	LastRotationAt *time.Time
}

type PrincipalType string

const (
	PrincipalTypeUser           PrincipalType = "user"
	PrincipalTypeServiceAccount PrincipalType = "service_account"
)

type APIKey struct {
	ID            uuid.UUID
	KeyID         string // Short, unique identifier (12 chars) - user-friendly lookup key
	OrgID         uuid.UUID
	PrincipalType PrincipalType
	PrincipalID   uuid.UUID
	Notes         string // Human-readable description (like GitHub token name)
	Fingerprint   string
	Status        string
	Scopes        []string
	IssuedAt      time.Time
	RevokedAt     *time.Time
	ExpiresAt     *time.Time
	LastUsedAt    *time.Time
	Annotations   map[string]any
	Version       int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

type CreateAPIKeyParams struct {
	ID            uuid.UUID
	KeyID         string // Short, unique identifier (12 chars)
	OrgID         uuid.UUID
	PrincipalType PrincipalType
	PrincipalID   uuid.UUID
	Notes         string // Human-readable description
	Fingerprint   string
	Status        string
	Scopes        []string
	ExpiresAt     *time.Time
	Annotations   map[string]any
}

type RevokeAPIKeyParams struct {
	ID        uuid.UUID
	Version   int64
	Status    string
	RevokedAt time.Time
}

type Session struct {
	ID               uuid.UUID
	OrgID            uuid.UUID
	UserID           uuid.UUID
	RefreshTokenHash string
	IPAddress        *string
	UserAgent        *string
	MFAVerifiedAt    *time.Time
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

type CreateSessionParams struct {
	ID               uuid.UUID
	OrgID            uuid.UUID
	UserID           uuid.UUID
	RefreshTokenHash string
	IPAddress        *string
	UserAgent        *string
	MFAVerifiedAt    *time.Time
	ExpiresAt        time.Time
}

type RevokeSessionParams struct {
	ID      uuid.UUID
	Version int64
	Time    time.Time
}

// UserModelAccess represents a user's model access configuration within an org
type UserModelAccess struct {
	ID         uuid.UUID
	OrgID      uuid.UUID
	UserID     uuid.UUID
	AccessMode string // "restricted" or "auto_grant"
	Version    int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UserModelGrant represents an explicit model grant for a user
type UserModelGrant struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	UserID    uuid.UUID
	ModelName string
	GrantedAt time.Time
	GrantedBy *uuid.UUID
	ExpiresAt *time.Time
}

// CreateModelGrantParams holds parameters for creating a model grant
type CreateModelGrantParams struct {
	OrgID     uuid.UUID
	UserID    uuid.UUID
	ModelName string
	GrantedBy *uuid.UUID
	ExpiresAt *time.Time
}

// UserModelAccessInfo combines access mode and grants for API response
type UserModelAccessInfo struct {
	AccessMode    string
	GrantedModels []UserModelGrant
}

// BootstrapKey represents a bootstrap key for org admin onboarding.
type BootstrapKey struct {
	ID          uuid.UUID
	KeyID       string // Short human-readable key ID (e.g., "bsk_abc123")
	OrgID       uuid.UUID
	OrgName     string // Populated from join
	Fingerprint string // SHA-256 hash of the token
	Status      string // active, revoked, redeemed, expired
	Notes       string
	ExpiresAt   time.Time
	RedeemedAt  *time.Time
	RedeemedBy  string // User ID or email who redeemed
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateBootstrapKeyParams holds parameters for creating a bootstrap key.
type CreateBootstrapKeyParams struct {
	OrgID       uuid.UUID
	Fingerprint string
	Notes       string
	ExpiresAt   time.Time
}

// AuditLog represents an audit event stored in the database.
type AuditLog struct {
	EventID     uuid.UUID
	OrgID       uuid.UUID
	ActorID     uuid.UUID
	ActorType   string
	ActorEmail  string
	TargetID    *uuid.UUID
	TargetType  *string
	Action      string
	Resource    *string
	PolicyID    *uuid.UUID
	IPAddress   *string
	UserAgent   *string
	Metadata    map[string]any
	Hash        string
	Signature   string
	DeliveredAt *time.Time
	CreatedAt   time.Time
}

// AuditLogFilters contains filters for querying audit logs.
type AuditLogFilters struct {
	OrgID        uuid.UUID
	Action       *string
	ResourceType *string
	ActorID      *string
	StartDate    *time.Time
	EndDate      *time.Time
	Limit        int
	Offset       int
}

// Well-known UUID for the system "No Token Rate-Limit" policy.
var NoTokenRateLimitPolicyID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// TokenRateLimitPolicy represents a token rate-limit policy.
type TokenRateLimitPolicy struct {
	ID          uuid.UUID
	OrgID       *uuid.UUID // nil for system built-in policies
	Name        string
	Description *string
	Limit1h     *int64 // nil = unlimited
	Limit24h    *int64 // nil = unlimited
	Limit7d     *int64 // nil = unlimited
	IsBuiltin   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateTokenPolicyParams holds parameters for creating a token policy.
type CreateTokenPolicyParams struct {
	OrgID       uuid.UUID
	Name        string
	Description *string
	Limit1h     *int64
	Limit24h    *int64
	Limit7d     *int64
}

// UpdateTokenPolicyParams holds parameters for updating a token policy.
type UpdateTokenPolicyParams struct {
	ID          uuid.UUID
	Name        *string
	Description *string
	Limit1h     *int64
	Limit24h    *int64
	Limit7d     *int64
	// ClearLimit1h, ClearLimit24h, ClearLimit7d allow setting limits to NULL
	ClearLimit1h  bool
	ClearLimit24h bool
	ClearLimit7d  bool
}

// EffectiveTokenPolicy represents a user's effective policy with source info.
type EffectiveTokenPolicy struct {
	Policy TokenRateLimitPolicy
	Source string // "override" or "inherited"
}

// WindowType represents a token usage window type.
type WindowType string

const (
	WindowType1h  WindowType = "1h"
	WindowType24h WindowType = "24h"
	WindowType7d  WindowType = "7d"
)

// TokenUsageWindow represents token usage in a rolling window.
type TokenUsageWindow struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	WindowType  WindowType
	WindowStart time.Time
	TokensUsed  int64
	UpdatedAt   time.Time
}

// TokenUsageStatus represents current usage for a window with policy limits.
type TokenUsageStatus struct {
	Window     WindowType
	Limit      int64
	Used       int64
	Remaining  int64
	Percentage float64
	ResetsAt   time.Time
}
