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
	OrgID         uuid.UUID
	PrincipalType PrincipalType
	PrincipalID   uuid.UUID
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
	OrgID         uuid.UUID
	PrincipalType PrincipalType
	PrincipalID   uuid.UUID
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
