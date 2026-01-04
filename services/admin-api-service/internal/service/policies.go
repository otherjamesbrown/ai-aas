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
	repo        *repository.PolicyRepository
	modelRepo   *repository.ModelRepository
	logger      *zap.Logger
	environment string // Environment name for validation (local allows localhost)
}

// NewPolicyService creates a new policy service
func NewPolicyService(repo *repository.PolicyRepository, modelRepo *repository.ModelRepository, environment string, logger *zap.Logger) *PolicyService {
	return &PolicyService{
		repo:        repo,
		modelRepo:   modelRepo,
		logger:      logger,
		environment: environment,
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

		// Validate backend URL if Endpoint is provided
		if b.Endpoint != nil && *b.Endpoint != "" {
			if err := s.validateBackendURL(*b.Endpoint); err != nil {
				return fmt.Errorf("invalid backend endpoint for %s: %w", b.BackendID, err)
			}
		}
	}

	return nil
}

// validateBackendURL validates a backend URL according to environment rules
func (s *PolicyService) validateBackendURL(urlStr string) error {
	// Only enforce restrictions in non-local environments
	if s.environment == "local" {
		return nil // Allow any URL in local environment
	}

	// Parse the URL
	// Note: We don't use net/url here to keep validation simple and avoid import
	// Just check for localhost patterns in the string

	// Check for localhost patterns
	if containsLocalhost(urlStr) {
		return fmt.Errorf("localhost URLs not allowed in '%s' environment. Use Kubernetes DNS format: http://<service>.<namespace>.svc.cluster.local:<port>", s.environment)
	}

	// Warn on raw IP addresses (log warning but don't reject)
	if containsRawIP(urlStr) {
		s.logger.Warn("backend URL uses raw IP address - prefer Kubernetes DNS names",
			zap.String("url", urlStr),
			zap.String("suggestion", "Use format: http://<service>.<namespace>.svc.cluster.local:<port>"),
		)
	}

	return nil
}

// containsLocalhost checks if a URL contains localhost patterns
func containsLocalhost(urlStr string) bool {
	// Check for common localhost patterns
	localhostPatterns := []string{
		"localhost",
		"127.0.0.1",
		"::1",
	}

	for _, pattern := range localhostPatterns {
		if urlContains(urlStr, pattern) {
			return true
		}
	}

	return false
}

// containsRawIP checks if a URL appears to contain a raw IP address
func containsRawIP(urlStr string) bool {
	// Simple heuristic: contains :// followed by digits and dots
	// This is not perfect but catches common cases like http://10.0.0.1:8080

	// Skip if it's a Kubernetes DNS name (contains .svc.cluster.local)
	if urlContains(urlStr, ".svc.cluster.local") {
		return false
	}

	// Check for IPv4-like patterns after ://
	afterScheme := urlStr
	if idx := urlIndexOf(urlStr, "://"); idx != -1 {
		afterScheme = urlStr[idx+3:]
	}

	// Extract hostname (before : or / or end)
	hostname := afterScheme
	for i, c := range hostname {
		if c == ':' || c == '/' {
			hostname = hostname[:i]
			break
		}
	}

	// Check if it looks like an IPv4 address
	// IPv4 pattern: starts with digit, contains 3 dots, only digits and dots, length < 16
	if len(hostname) == 0 || !urlIsDigit(hostname[0]) {
		return false
	}

	// Count dots and check for non-digit/non-dot characters
	dotCount := 0
	for i := 0; i < len(hostname); i++ {
		c := hostname[i]
		if c == '.' {
			dotCount++
		} else if !urlIsDigit(c) {
			// Contains a letter or other char - likely a domain name
			return false
		}
	}

	// IPv4 has exactly 3 dots (though we'll be lenient and accept 1-3)
	return dotCount >= 1 && dotCount <= 3 && len(hostname) <= 15
}

// Helper functions for URL validation (prefix with url to avoid package conflicts)
func urlContains(s, substr string) bool {
	return urlIndexOf(s, substr) != -1
}

func urlIndexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func urlIsDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
