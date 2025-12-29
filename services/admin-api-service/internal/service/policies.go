package service

import (
	"context"
	"fmt"

	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/repository"
	"go.uber.org/zap"
)

// PolicyService handles routing policy business logic
type PolicyService struct {
	repo      *repository.PolicyRepository
	modelRepo *repository.ModelRepository
	logger    *zap.Logger
}

// NewPolicyService creates a new policy service
func NewPolicyService(repo *repository.PolicyRepository, modelRepo *repository.ModelRepository, logger *zap.Logger) *PolicyService {
	return &PolicyService{
		repo:      repo,
		modelRepo: modelRepo,
		logger:    logger,
	}
}

// Create creates a new routing policy
func (s *PolicyService) Create(ctx context.Context, create *domain.PolicyCreate, createdBy string) (*domain.RoutingPolicy, error) {
	// Validate backends
	if err := s.validateBackends(ctx, create.Backends); err != nil {
		return nil, err
	}

	policy, err := s.repo.Create(ctx, create, createdBy)
	if err != nil {
		s.logger.Error("failed to create policy", zap.String("model", create.Model), zap.Error(err))
		return nil, err
	}

	s.logger.Info("policy created",
		zap.String("policy_id", policy.PolicyID),
		zap.String("model", policy.Model),
	)

	return policy, nil
}

// Get retrieves a policy by ID
func (s *PolicyService) Get(ctx context.Context, id string) (*domain.RoutingPolicy, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves policies with filtering and pagination
func (s *PolicyService) List(ctx context.Context, params domain.PolicyListParams) (*domain.PolicyListResponse, error) {
	return s.repo.List(ctx, params)
}

// Update updates a policy
func (s *PolicyService) Update(ctx context.Context, id string, update *domain.PolicyUpdate, updatedBy string) (*domain.RoutingPolicy, error) {
	// Validate backends if provided
	if update.Backends != nil {
		if err := s.validateBackends(ctx, update.Backends); err != nil {
			return nil, err
		}
	}

	// Validate failover threshold
	if update.FailoverThreshold != nil && (*update.FailoverThreshold < 1 || *update.FailoverThreshold > 10) {
		return nil, fmt.Errorf("failover_threshold must be between 1 and 10")
	}

	policy, err := s.repo.Update(ctx, id, update, updatedBy)
	if err != nil {
		return nil, err
	}

	if policy != nil {
		s.logger.Info("policy updated",
			zap.String("policy_id", policy.PolicyID),
			zap.Int("version", policy.Version),
		)
	}

	return policy, nil
}

// Delete soft-deletes a policy
func (s *PolicyService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.logger.Info("policy deleted", zap.String("policy_id", id))
	return nil
}

// Activate enables a policy
func (s *PolicyService) Activate(ctx context.Context, id, updatedBy string) (*domain.PolicyActivationResponse, error) {
	policy, err := s.repo.SetEnabled(ctx, id, true, updatedBy)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, nil
	}

	s.logger.Info("policy activated", zap.String("policy_id", id))

	return &domain.PolicyActivationResponse{
		PolicyID:  policy.PolicyID,
		Enabled:   policy.Enabled,
		UpdatedAt: policy.UpdatedAt,
		Message:   "Policy activated successfully",
	}, nil
}

// Deactivate disables a policy
func (s *PolicyService) Deactivate(ctx context.Context, id, updatedBy string) (*domain.PolicyActivationResponse, error) {
	policy, err := s.repo.SetEnabled(ctx, id, false, updatedBy)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, nil
	}

	s.logger.Info("policy deactivated", zap.String("policy_id", id))

	return &domain.PolicyActivationResponse{
		PolicyID:  policy.PolicyID,
		Enabled:   policy.Enabled,
		UpdatedAt: policy.UpdatedAt,
		Message:   "Policy deactivated. Traffic will fail until policy is reactivated.",
	}, nil
}

// Sync retrieves policies for api-router sync
func (s *PolicyService) Sync(ctx context.Context, sinceVersion int, environment string) (*domain.PolicySyncResponse, error) {
	return s.repo.GetForSync(ctx, sinceVersion, environment)
}

// Validate validates a policy configuration without creating it
func (s *PolicyService) Validate(ctx context.Context, create *domain.PolicyCreate) *domain.PolicyValidationResponse {
	response := &domain.PolicyValidationResponse{
		Valid:    true,
		Errors:   []domain.ValidationError{},
		Warnings: []string{},
	}

	// Validate model name
	if create.Model == "" {
		response.Valid = false
		response.Errors = append(response.Errors, domain.ValidationError{
			Field:   "model",
			Message: "model is required",
		})
	}

	// Validate backends
	if len(create.Backends) == 0 {
		response.Valid = false
		response.Errors = append(response.Errors, domain.ValidationError{
			Field:   "backends",
			Message: "at least one backend is required",
		})
	} else if len(create.Backends) > 10 {
		response.Valid = false
		response.Errors = append(response.Errors, domain.ValidationError{
			Field:   "backends",
			Message: "maximum 10 backends allowed",
		})
	}

	// Validate weights sum to 100
	if !domain.ValidateBackendWeights(create.Backends) {
		response.Valid = false
		response.Errors = append(response.Errors, domain.ValidationError{
			Field:   "backends",
			Message: "backend weights must sum to 100",
		})
	}

	// Validate failover threshold
	if create.FailoverThreshold != 0 && (create.FailoverThreshold < 1 || create.FailoverThreshold > 10) {
		response.Valid = false
		response.Errors = append(response.Errors, domain.ValidationError{
			Field:   "failover_threshold",
			Message: "failover_threshold must be between 1 and 10",
		})
	}

	return response
}

func (s *PolicyService) validateBackends(ctx context.Context, backends []domain.Backend) error {
	if len(backends) == 0 {
		return fmt.Errorf("at least one backend is required")
	}

	if len(backends) > 10 {
		return fmt.Errorf("maximum 10 backends allowed")
	}

	if !domain.ValidateBackendWeights(backends) {
		return fmt.Errorf("backend weights must sum to 100")
	}

	// Verify each backend exists in model registry
	// Note: In production, this would check against actual model registry
	for _, b := range backends {
		if b.BackendID == "" {
			return fmt.Errorf("backend_id is required")
		}
		if b.Weight < 1 || b.Weight > 100 {
			return fmt.Errorf("backend weight must be between 1 and 100")
		}
	}

	return nil
}
