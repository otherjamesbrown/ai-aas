package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/repository"
	"go.uber.org/zap"
)

// ModelRegistryService handles model registry business logic
type ModelRegistryService struct {
	repo       *repository.ModelRepository
	policyRepo *repository.PolicyRepository
	logger     *zap.Logger
}

// NewModelRegistryService creates a new model registry service
func NewModelRegistryService(repo *repository.ModelRepository, policyRepo *repository.PolicyRepository, logger *zap.Logger) *ModelRegistryService {
	return &ModelRegistryService{
		repo:       repo,
		policyRepo: policyRepo,
		logger:     logger,
	}
}

// Register registers or updates a model
func (s *ModelRegistryService) Register(ctx context.Context, reg *domain.ModelRegistration) (*domain.Model, bool, error) {
	// Validate input
	if err := s.validateRegistration(reg); err != nil {
		return nil, false, err
	}

	// Upsert the model
	model, created, err := s.repo.Upsert(ctx, reg)
	if err != nil {
		s.logger.Error("failed to register model",
			zap.String("model_name", reg.ModelName),
			zap.String("environment", reg.DeploymentEnvironment),
			zap.Error(err),
		)
		return nil, false, fmt.Errorf("failed to register model: %w", err)
	}

	action := "updated"
	if created {
		action = "created"
	}

	s.logger.Info("model registered",
		zap.String("model_id", model.ModelID.String()),
		zap.String("model_name", model.ModelName),
		zap.String("environment", stringOrEmpty(model.DeploymentEnvironment)),
		zap.String("action", action),
	)

	// Auto-create global routing policy for new models (default behavior)
	// This ensures newly deployed models are immediately accessible to all orgs
	// Always try to create policy, even on updates, in case it was missing
	if s.policyRepo == nil {
		s.logger.Error("policyRepo is nil - cannot auto-create routing policy",
			zap.String("model_name", model.ModelName),
		)
	} else {
		s.logger.Debug("attempting to auto-create routing policy",
			zap.String("model_name", model.ModelName),
			zap.String("model_id", model.ModelID.String()),
			zap.Bool("is_new_model", created),
		)
		if err := s.createDefaultRoutingPolicy(ctx, model); err != nil {
			// Check if the error is due to policy already existing
			// This is expected and acceptable - not an error condition
			if isPolicyAlreadyExistsError(err) {
				s.logger.Debug("routing policy already exists (expected)",
					zap.String("model_name", model.ModelName),
					zap.String("model_id", model.ModelID.String()),
				)
			} else {
				// Unexpected error - log as error but don't fail registration
				// The model is registered, policy can be created manually if needed
				s.logger.Error("failed to auto-create routing policy",
					zap.String("model_name", model.ModelName),
					zap.String("model_id", model.ModelID.String()),
					zap.Error(err),
				)
			}
		} else {
			s.logger.Info("successfully auto-created routing policy",
				zap.String("model_name", model.ModelName),
				zap.String("model_id", model.ModelID.String()),
			)
		}
	}

	return model, created, nil
}

// createDefaultRoutingPolicy creates a global routing policy for the model
func (s *ModelRegistryService) createDefaultRoutingPolicy(ctx context.Context, model *domain.Model) error {
	// Build the backend ID from model name (matches deployment naming convention)
	backendID := model.ModelName

	endpoint := ""
	if model.DeploymentEndpoint != nil {
		endpoint = *model.DeploymentEndpoint
	}

	s.logger.Debug("building policy create request",
		zap.String("model_name", model.ModelName),
		zap.String("backend_id", backendID),
		zap.String("endpoint", endpoint),
	)

	policyCreate := &domain.PolicyCreate{
		OrganizationID: "*", // Global policy - applies to all organizations
		Model:          model.ModelName,
		Backends: []domain.Backend{
			{
				BackendID: backendID,
				Weight:    100,
			},
		},
		FailoverThreshold: 3,
		Metadata: map[string]interface{}{
			"auto_created":  true,
			"model_id":      model.ModelID.String(),
			"endpoint":      endpoint,
			"namespace":     stringOrEmpty(model.DeploymentNamespace),
			"environment":   stringOrEmpty(model.DeploymentEnvironment),
			"created_reason": "auto-created on model registration",
		},
	}

	s.logger.Debug("calling policyRepo.Create",
		zap.String("model", model.ModelName),
		zap.String("org_id", "*"),
	)

	policy, err := s.policyRepo.Create(ctx, policyCreate, "system")
	if err != nil {
		s.logger.Error("policyRepo.Create returned error",
			zap.String("model_name", model.ModelName),
			zap.String("backend_id", backendID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to create routing policy: %w", err)
	}

	s.logger.Info("auto-created global routing policy",
		zap.String("policy_id", policy.PolicyID),
		zap.String("model", model.ModelName),
		zap.String("backend_id", backendID),
	)

	return nil
}

// Get retrieves a model by name and environment
func (s *ModelRegistryService) Get(ctx context.Context, name, environment string) (*domain.Model, error) {
	if name == "" {
		return nil, fmt.Errorf("model name is required")
	}
	if environment == "" {
		return nil, fmt.Errorf("environment is required")
	}

	return s.repo.GetByNameAndEnv(ctx, name, environment)
}

// List retrieves models with filtering and pagination
func (s *ModelRegistryService) List(ctx context.Context, params domain.ModelListParams) (*domain.ModelListResponse, error) {
	// Validate environment if provided
	if params.Environment != "" && !domain.IsValidEnvironment(params.Environment) {
		return nil, fmt.Errorf("invalid environment: %s", params.Environment)
	}

	// Validate status if provided
	if params.Status != "" && !domain.IsValidStatus(params.Status) {
		return nil, fmt.Errorf("invalid status: %s", params.Status)
	}

	return s.repo.List(ctx, params)
}

// Update updates specific fields of a model
func (s *ModelRegistryService) Update(ctx context.Context, name, environment string, update *domain.ModelUpdate) (*domain.Model, error) {
	if name == "" {
		return nil, fmt.Errorf("model name is required")
	}
	if environment == "" {
		return nil, fmt.Errorf("environment is required")
	}

	// Validate status if provided
	if update.DeploymentStatus != nil && !domain.IsValidStatus(*update.DeploymentStatus) {
		return nil, fmt.Errorf("invalid status: %s", *update.DeploymentStatus)
	}

	model, err := s.repo.Update(ctx, name, environment, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update model: %w", err)
	}

	if model != nil {
		s.logger.Info("model updated",
			zap.String("model_id", model.ModelID.String()),
			zap.String("model_name", model.ModelName),
			zap.String("environment", stringOrEmpty(model.DeploymentEnvironment)),
		)
	}

	return model, nil
}

// Delete removes a model
func (s *ModelRegistryService) Delete(ctx context.Context, name, environment string) error {
	if name == "" {
		return fmt.Errorf("model name is required")
	}
	if environment == "" {
		return fmt.Errorf("environment is required")
	}

	if err := s.repo.Delete(ctx, name, environment); err != nil {
		return fmt.Errorf("failed to delete model: %w", err)
	}

	s.logger.Info("model deleted",
		zap.String("model_name", name),
		zap.String("environment", environment),
	)

	return nil
}

func (s *ModelRegistryService) validateRegistration(reg *domain.ModelRegistration) error {
	if reg.ModelName == "" {
		return fmt.Errorf("model_name is required")
	}
	if len(reg.ModelName) > 255 {
		return fmt.Errorf("model_name must be 255 characters or less")
	}
	if reg.DeploymentEndpoint == "" {
		return fmt.Errorf("deployment_endpoint is required")
	}
	if reg.DeploymentEnvironment == "" {
		return fmt.Errorf("deployment_environment is required")
	}
	if !domain.IsValidEnvironment(reg.DeploymentEnvironment) {
		return fmt.Errorf("invalid deployment_environment: %s", reg.DeploymentEnvironment)
	}
	if reg.DeploymentNamespace == "" {
		return fmt.Errorf("deployment_namespace is required")
	}
	if reg.DeploymentStatus != "" && !domain.IsValidStatus(reg.DeploymentStatus) {
		return fmt.Errorf("invalid deployment_status: %s", reg.DeploymentStatus)
	}
	return nil
}

// isPolicyAlreadyExistsError checks if the error is due to unique constraint violation
// This indicates the policy already exists, which is expected behavior
func isPolicyAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// Check for PostgreSQL unique constraint violation
	return strings.Contains(errStr, "unique") ||
		strings.Contains(errStr, "duplicate") ||
		strings.Contains(errStr, "idx_routing_policies_unique_org_model")
}

// stringOrEmpty returns the dereferenced string or empty string if nil
func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

