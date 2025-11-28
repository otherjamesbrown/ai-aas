package service

import (
	"context"

	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/metrics"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/repository"
	"go.uber.org/zap"
)

// AuditService handles audit logging business logic
type AuditService struct {
	repo   *repository.AuditRepository
	logger *zap.Logger
}

// NewAuditService creates a new audit service
func NewAuditService(repo *repository.AuditRepository, logger *zap.Logger) *AuditService {
	return &AuditService{
		repo:   repo,
		logger: logger,
	}
}

// Log creates an audit log entry
func (s *AuditService) Log(ctx context.Context, create *domain.AuditLogCreate) {
	if err := s.repo.Create(ctx, create); err != nil {
		s.logger.Error("failed to create audit log",
			zap.String("action", create.Action),
			zap.String("resource_type", create.ResourceType),
			zap.String("resource_id", create.ResourceID),
			zap.Error(err),
		)
		// Increment failure metric
		metrics.AuditLogFailures.Inc()
	}
}

// List retrieves audit logs with filtering and pagination
func (s *AuditService) List(ctx context.Context, params domain.AuditLogListParams) (*domain.AuditLogListResponse, error) {
	return s.repo.List(ctx, params)
}

