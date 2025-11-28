// Package models provides HTTP handlers for model management operations.
package models

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	svcModels "github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/services/models"
)

// ServiceAdapter adapts the models.Service to the handler's Service interface
type ServiceAdapter struct {
	svc *svcModels.Service
	ctx context.Context
}

// NewServiceAdapter creates a new service adapter
func NewServiceAdapter(svc *svcModels.Service) *ServiceAdapter {
	return &ServiceAdapter{
		svc: svc,
		ctx: context.Background(),
	}
}

// WithContext sets the context for operations
func (a *ServiceAdapter) WithContext(ctx context.Context) *ServiceAdapter {
	return &ServiceAdapter{
		svc: a.svc,
		ctx: ctx,
	}
}

// ListModels returns models matching the given options
func (a *ServiceAdapter) ListModels(opts ListModelsOptions) ([]Model, error) {
	svcOpts := svcModels.ListModelsOptions{
		Cached:      opts.Cached,
		Deployed:    opts.Deployed,
		Orphaned:    opts.Orphaned,
		Environment: opts.Environment,
	}

	svcModels, err := a.svc.ListModels(a.ctx, svcOpts)
	if err != nil {
		return nil, err
	}

	models := make([]Model, len(svcModels))
	for i, m := range svcModels {
		models[i] = convertModel(m)
	}
	return models, nil
}

// GetModel returns a model by name
func (a *ServiceAdapter) GetModel(name string) (*Model, error) {
	m, err := a.svc.GetModel(a.ctx, name)
	if err != nil {
		return nil, err
	}

	model := convertModel(*m)
	return &model, nil
}

// AddModel registers a new model
func (a *ServiceAdapter) AddModel(req AddModelRequest) (*Model, error) {
	svcReq := svcModels.AddModelRequest{
		Name:          req.Name,
		HFModelID:     req.HFModelID,
		RequiresAuth:  req.RequiresAuth,
		IsGated:       req.LicenseType != "",
		LicenseType:   req.LicenseType,
		AcceptLicense: req.AcceptLicense,
		GPUMemoryGB:   req.GPUMemoryGB,
		CPUMemoryGB:   req.CPUMemoryGB,
	}

	m, err := a.svc.AddModel(a.ctx, svcReq)
	if err != nil {
		return nil, err
	}

	model := convertModel(*m)
	return &model, nil
}

// RemoveModel deletes a model from the registry
func (a *ServiceAdapter) RemoveModel(name string, force bool) error {
	return a.svc.RemoveModel(a.ctx, name, force)
}

// GetModelCache returns cache entries for a model
func (a *ServiceAdapter) GetModelCache(name string) ([]CacheEntry, error) {
	// TODO: Implement when cache service is ready
	return []CacheEntry{}, nil
}

// PullModel starts a model pull operation
func (a *ServiceAdapter) PullModel(name string, opts PullOptions) (*PullJob, error) {
	// TODO: Implement when pull service is ready
	return &PullJob{
		ID:        "placeholder",
		ModelName: name,
		Status:    "pending",
	}, nil
}

// VerifyCache verifies the cache integrity for a model
func (a *ServiceAdapter) VerifyCache(name string, version string) (*VerifyResult, error) {
	// TODO: Implement when cache service is ready
	return &VerifyResult{
		Valid:        true,
		FilesChecked: 0,
	}, nil
}

// ListCredentials returns all stored credential types
func (a *ServiceAdapter) ListCredentials() ([]Credential, error) {
	creds, err := a.svc.ListCredentials(a.ctx)
	if err != nil {
		return nil, err
	}

	result := make([]Credential, len(creds))
	for i, c := range creds {
		result[i] = Credential{
			Type:      c.Type,
			Masked:    c.Masked,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		}
	}
	return result, nil
}

// SetCredential stores an encrypted credential
func (a *ServiceAdapter) SetCredential(credType string, value string, metadata map[string]interface{}) error {
	return a.svc.SetCredential(a.ctx, credType, value, metadata)
}

// TestCredential tests a credential
func (a *ServiceAdapter) TestCredential(credType string) (*CredentialTestResult, error) {
	// Get the credential and test it based on type
	_, err := a.svc.GetCredential(a.ctx, credType)
	if err != nil {
		return &CredentialTestResult{
			Valid:   false,
			Message: "Credential not found or invalid",
		}, nil
	}

	// TODO: Implement actual credential testing per type (HF API, S3, etc.)
	return &CredentialTestResult{
		Valid:   true,
		Message: "Credential is configured",
	}, nil
}

// DeleteCredential removes a credential
func (a *ServiceAdapter) DeleteCredential(credType string) error {
	return a.svc.DeleteCredential(a.ctx, credType)
}

// Helper functions

func convertModel(m svcModels.Model) Model {
	model := Model{
		ID:           m.ID.String(),
		Name:         m.Name,
		HFModelID:    m.HFModelID,
		HFRevision:   m.HFRevision,
		RequiresAuth: m.RequiresAuth,
		IsGated:      m.IsGated,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}

	if m.LicenseType != nil {
		model.LicenseType = *m.LicenseType
	}
	if m.LicenseURL != nil {
		model.LicenseURL = *m.LicenseURL
	}
	if m.LicenseAcceptedAt != nil {
		model.LicenseAcceptedAt = m.LicenseAcceptedAt
	}
	if m.LicenseAcceptedBy != nil {
		model.LicenseAcceptedBy = *m.LicenseAcceptedBy
	}
	if m.RecommendedGPUMemoryGB != nil {
		model.RecommendedGPUMemoryGB = int(*m.RecommendedGPUMemoryGB)
	}
	if m.RecommendedCPUMemoryGB != nil {
		model.RecommendedCPUMemoryGB = int(*m.RecommendedCPUMemoryGB)
	}
	if m.PinnedVersion != nil {
		model.PinnedVersion = *m.PinnedVersion
	}

	return model
}

// NoOpEncryptor is a placeholder encryptor for development
type NoOpEncryptor struct{}

// Encrypt returns the plaintext (no encryption)
func (e *NoOpEncryptor) Encrypt(plaintext string) (string, error) {
	return plaintext, nil
}

// Decrypt returns the ciphertext as-is (no decryption)
func (e *NoOpEncryptor) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}

// CreateModelsService creates a models service with the given pool
func CreateModelsService(pool *pgxpool.Pool) *svcModels.Service {
	return svcModels.NewService(pool, &NoOpEncryptor{})
}
