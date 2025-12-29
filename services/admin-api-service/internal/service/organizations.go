package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/repository"
	"go.uber.org/zap"
)

// OrganizationService handles organization business logic
type OrganizationService struct {
	repo   *repository.OrganizationRepository
	logger *zap.Logger
}

// NewOrganizationService creates a new organization service
func NewOrganizationService(repo *repository.OrganizationRepository, logger *zap.Logger) *OrganizationService {
	return &OrganizationService{
		repo:   repo,
		logger: logger,
	}
}

// Create creates a new organization
func (s *OrganizationService) Create(ctx context.Context, create *domain.OrganizationCreate) (*domain.Organization, error) {
	// Check for duplicate slug
	existing, err := s.repo.GetBySlug(ctx, create.Slug)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing organization: %w", err)
	}
	if existing != nil {
		return nil, &ConflictError{Message: fmt.Sprintf("organization with slug '%s' already exists", create.Slug)}
	}

	org, err := s.repo.Create(ctx, create)
	if err != nil {
		s.logger.Error("failed to create organization", zap.String("slug", create.Slug), zap.Error(err))
		return nil, err
	}

	s.logger.Info("organization created",
		zap.String("organization_id", org.OrganizationID.String()),
		zap.String("slug", org.Slug),
	)

	return org, nil
}

// Get retrieves an organization by ID
func (s *OrganizationService) Get(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves organizations with pagination
func (s *OrganizationService) List(ctx context.Context, params domain.OrganizationListParams) (*domain.OrganizationListResponse, error) {
	return s.repo.List(ctx, params)
}

// Update updates specific fields of an organization
func (s *OrganizationService) Update(ctx context.Context, id uuid.UUID, update *domain.OrganizationUpdate) (*domain.Organization, error) {
	// Validate plan tier if provided
	if update.PlanTier != nil && !domain.IsValidPlanTier(*update.PlanTier) {
		return nil, fmt.Errorf("invalid plan_tier: %s", *update.PlanTier)
	}

	// Validate status if provided
	if update.Status != nil && !domain.IsValidOrgStatus(*update.Status) {
		return nil, fmt.Errorf("invalid status: %s", *update.Status)
	}

	org, err := s.repo.Update(ctx, id, update)
	if err != nil {
		return nil, err
	}

	if org != nil {
		s.logger.Info("organization updated",
			zap.String("organization_id", org.OrganizationID.String()),
			zap.String("slug", org.Slug),
		)
	}

	return org, nil
}

// ConflictError represents a conflict error
type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string {
	return e.Message
}
