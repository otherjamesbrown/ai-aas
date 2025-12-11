// Package models provides HTTP handlers for model management operations.
package models

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
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

// RenameModel renames a model in the registry
func (a *ServiceAdapter) RenameModel(name string, req RenameModelRequest) (*RenameModelResponse, error) {
	svcReq := svcModels.RenameModelRequest{
		NewName:      req.NewName,
		MigrateCache: req.MigrateCache,
	}

	// Use the service as a credential getter (it implements the interface)
	result, err := a.svc.RenameModel(a.ctx, name, svcReq, a.svc)
	if err != nil {
		// Translate service-layer errors to handler-layer errors
		if errors.Is(err, svcModels.ErrModelNotFound) {
			return nil, ErrModelNotFound
		}
		if errors.Is(err, svcModels.ErrModelDeployed) {
			return nil, ErrModelDeployed
		}
		if errors.Is(err, svcModels.ErrModelNameExists) {
			return nil, ErrModelNameExists
		}
		// Translate validation errors
		if errors.Is(err, svcModels.ErrNameEmpty) || errors.Is(err, svcModels.ErrNameTooLong) || errors.Is(err, svcModels.ErrInvalidKServeName) {
			return nil, ErrInvalidModelName
		}
		return nil, err
	}

	return &RenameModelResponse{
		OldName:        result.OldName,
		NewName:        result.NewName,
		CacheMigrated:  result.CacheMigrated,
		CacheSizeBytes: result.CacheSizeBytes,
		CacheFileCount: result.CacheFileCount,
	}, nil
}

// GetModelCache returns cache entries for a model
func (a *ServiceAdapter) GetModelCache(name string) ([]CacheEntry, error) {
	svcEntries, err := a.svc.GetModelCache(a.ctx, name)
	if err != nil {
		return nil, err
	}

	entries := make([]CacheEntry, len(svcEntries))
	for i, e := range svcEntries {
		entries[i] = convertCacheEntry(e)
	}
	return entries, nil
}

// PullModel starts a model pull operation
func (a *ServiceAdapter) PullModel(name string, opts PullOptions) (*PullJob, error) {
	svcJob, err := a.svc.CreatePullJob(a.ctx, svcModels.CreatePullJobRequest{
		ModelName: name,
		Revision:  opts.Revision,
	})
	if err != nil {
		return nil, err
	}

	return convertPullJob(svcJob, name), nil
}

// ListPullJobs returns all pull jobs for a model
func (a *ServiceAdapter) ListPullJobs(modelName string) ([]PullJob, error) {
	svcJobs, err := a.svc.ListPullJobs(a.ctx, modelName)
	if err != nil {
		return nil, err
	}

	jobs := make([]PullJob, len(svcJobs))
	for i, j := range svcJobs {
		jobs[i] = *convertPullJob(&j, modelName)
	}
	return jobs, nil
}

// GetPullJob returns a specific pull job by ID, scoped to the specified model
func (a *ServiceAdapter) GetPullJob(modelName, jobID string) (*PullJob, error) {
	id, err := uuid.Parse(jobID)
	if err != nil {
		return nil, fmt.Errorf("invalid job ID: %w", err)
	}

	svcJob, err := a.svc.GetPullJob(a.ctx, modelName, id)
	if err != nil {
		return nil, err
	}

	return convertPullJob(svcJob, modelName), nil
}

// CancelPullJob cancels a running pull job
func (a *ServiceAdapter) CancelPullJob(jobID string) error {
	id, err := uuid.Parse(jobID)
	if err != nil {
		return fmt.Errorf("invalid job ID: %w", err)
	}

	return a.svc.CancelPullJob(a.ctx, id)
}

// VerifyCache verifies the cache integrity for a model
func (a *ServiceAdapter) VerifyCache(name string, version string) (*VerifyResult, error) {
	svcResult, err := a.svc.VerifyCache(a.ctx, name, version)
	if err != nil {
		return nil, err
	}

	return &VerifyResult{
		Valid:         svcResult.Valid,
		FilesChecked:  svcResult.FilesChecked,
		FilesMissing:  svcResult.FilesMissing,
		FilesCorrupt:  svcResult.FilesCorrupt,
		ChecksumMatch: svcResult.ChecksumMatch,
		VerifiedAt:    svcResult.VerifiedAt,
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

// Deployment operations

// ListDeployments returns deployments matching the given options
func (a *ServiceAdapter) ListDeployments(opts ListDeploymentsOptions) ([]Deployment, error) {
	svcOpts := svcModels.ListDeploymentsOptions{
		Environment: opts.Environment,
		ModelName:   opts.ModelName,
		Status:      opts.Status,
		Enabled:     opts.Enabled,
	}

	svcDeployments, err := a.svc.ListDeployments(a.ctx, svcOpts)
	if err != nil {
		return nil, err
	}

	deployments := make([]Deployment, len(svcDeployments))
	for i, d := range svcDeployments {
		deployments[i] = convertDeployment(d)
	}
	return deployments, nil
}

// GetDeployment returns a deployment by model name and environment
func (a *ServiceAdapter) GetDeployment(modelName, environment string) (*Deployment, error) {
	d, err := a.svc.GetDeployment(a.ctx, modelName, environment)
	if err != nil {
		return nil, err
	}

	deployment := convertDeployment(*d)
	return &deployment, nil
}

// CreateDeployment creates a new deployment
func (a *ServiceAdapter) CreateDeployment(req CreateDeploymentRequest) (*Deployment, error) {
	svcReq := svcModels.CreateDeploymentRequest{
		ModelName:   req.ModelName,
		Environment: req.Environment,
		Namespace:   req.Namespace,
		GPUCount:    req.GPUCount,
		MemoryGB:    req.MemoryGB,
		Replicas:    req.Replicas,
	}

	d, err := a.svc.CreateDeployment(a.ctx, svcReq)
	if err != nil {
		return nil, err
	}

	deployment := convertDeployment(*d)
	return &deployment, nil
}

// EnableDeployment enables a deployment
func (a *ServiceAdapter) EnableDeployment(modelName, environment, enabledBy string) error {
	return a.svc.EnableDeployment(a.ctx, modelName, environment, enabledBy)
}

// DisableDeployment disables a deployment
func (a *ServiceAdapter) DisableDeployment(modelName, environment, disabledBy string) error {
	return a.svc.DisableDeployment(a.ctx, modelName, environment, disabledBy)
}

// ScaleDeployment scales a deployment
func (a *ServiceAdapter) ScaleDeployment(modelName, environment string, req ScaleDeploymentRequest) error {
	svcReq := svcModels.ScaleDeploymentRequest{
		ReplicasDesired: req.Replicas,
		GPUCount:        req.GPUCount,
		MemoryGB:        req.MemoryGB,
	}
	return a.svc.ScaleDeployment(a.ctx, modelName, environment, svcReq)
}

// DeleteDeployment terminates a deployment
func (a *ServiceAdapter) DeleteDeployment(modelName, environment string) error {
	return a.svc.DeleteDeployment(a.ctx, modelName, environment)
}

// Validation operations

// ValidateModel runs validation checks for a model
func (a *ServiceAdapter) ValidateModel(modelName string, req ValidateModelRequest) ([]ValidationResult, error) {
	svcReq := svcModels.ValidateModelRequest{
		ModelName:   modelName,
		Environment: req.Environment,
		Layers:      req.Layers,
	}

	svcResults, err := a.svc.ValidateModel(a.ctx, svcReq)
	if err != nil {
		return nil, err
	}

	results := make([]ValidationResult, len(svcResults))
	for i, r := range svcResults {
		results[i] = convertValidationResult(r)
	}
	return results, nil
}

// GetValidationHistory returns recent validation results for a model
func (a *ServiceAdapter) GetValidationHistory(modelName string, limit int) ([]ValidationResult, error) {
	svcResults, err := a.svc.GetValidationHistory(a.ctx, modelName, limit)
	if err != nil {
		return nil, err
	}

	results := make([]ValidationResult, len(svcResults))
	for i, r := range svcResults {
		results[i] = convertValidationResult(r)
	}
	return results, nil
}

// State history operations

// EnableDeploymentWithHistory enables a deployment and records the action
func (a *ServiceAdapter) EnableDeploymentWithHistory(modelName, environment, enabledBy, reason string) error {
	return a.svc.EnableDeploymentWithHistory(a.ctx, modelName, environment, enabledBy, reason)
}

// DisableDeploymentWithHistory disables a deployment and records the action
func (a *ServiceAdapter) DisableDeploymentWithHistory(modelName, environment, disabledBy, reason string) error {
	return a.svc.DisableDeploymentWithHistory(a.ctx, modelName, environment, disabledBy, reason)
}

// GetStateHistory returns state history for a deployment
func (a *ServiceAdapter) GetStateHistory(modelName, environment string, limit int) ([]StateHistoryEntry, error) {
	svcEntries, err := a.svc.GetStateHistory(a.ctx, modelName, environment, limit)
	if err != nil {
		return nil, err
	}

	entries := make([]StateHistoryEntry, len(svcEntries))
	for i, e := range svcEntries {
		entries[i] = convertStateHistoryEntry(e)
	}
	return entries, nil
}

// GetAllStateHistory returns recent state changes across all deployments
func (a *ServiceAdapter) GetAllStateHistory(limit int) ([]StateHistoryEntry, error) {
	svcEntries, err := a.svc.GetAllStateHistory(a.ctx, limit)
	if err != nil {
		return nil, err
	}

	entries := make([]StateHistoryEntry, len(svcEntries))
	for i, e := range svcEntries {
		entries[i] = convertStateHistoryEntryWithModel(e)
	}
	return entries, nil
}

// Alias operations

// ListAliases returns all aliases
func (a *ServiceAdapter) ListAliases() ([]Alias, error) {
	svcAliases, err := a.svc.ListAliases(a.ctx)
	if err != nil {
		return nil, err
	}

	aliases := make([]Alias, len(svcAliases))
	for i, al := range svcAliases {
		aliases[i] = convertAlias(al)
	}
	return aliases, nil
}

// GetAlias returns an alias by name
func (a *ServiceAdapter) GetAlias(aliasName string) (*Alias, error) {
	al, err := a.svc.GetAlias(a.ctx, aliasName)
	if err != nil {
		return nil, err
	}

	alias := convertAlias(*al)
	return &alias, nil
}

// ResolveAlias resolves an alias to its target model name
func (a *ServiceAdapter) ResolveAlias(name string) (string, error) {
	return a.svc.ResolveAlias(a.ctx, name)
}

// CreateAlias creates a new alias
func (a *ServiceAdapter) CreateAlias(req CreateAliasRequest) (*Alias, error) {
	svcReq := svcModels.CreateAliasRequest{
		AliasName:   req.AliasName,
		ModelName:   req.ModelName,
		Description: req.Description,
	}

	al, err := a.svc.CreateAlias(a.ctx, svcReq)
	if err != nil {
		return nil, err
	}

	alias := convertAlias(*al)
	return &alias, nil
}

// UpdateAlias updates an alias
func (a *ServiceAdapter) UpdateAlias(aliasName string, req UpdateAliasRequest) (*Alias, error) {
	svcReq := svcModels.UpdateAliasRequest{
		ModelName:   req.ModelName,
		Description: req.Description,
	}

	al, err := a.svc.UpdateAlias(a.ctx, aliasName, svcReq)
	if err != nil {
		return nil, err
	}

	alias := convertAlias(*al)
	return &alias, nil
}

// DeleteAlias removes an alias
func (a *ServiceAdapter) DeleteAlias(aliasName string) error {
	return a.svc.DeleteAlias(a.ctx, aliasName)
}

// GetAliasesForModel returns all aliases pointing to a model
func (a *ServiceAdapter) GetAliasesForModel(modelName string) ([]Alias, error) {
	svcAliases, err := a.svc.GetAliasesForModel(a.ctx, modelName)
	if err != nil {
		return nil, err
	}

	aliases := make([]Alias, len(svcAliases))
	for i, al := range svcAliases {
		aliases[i] = convertAlias(al)
	}
	return aliases, nil
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
	if m.CacheStatus != nil {
		model.CacheStatus = *m.CacheStatus
	}
	if m.DeploymentStatus != nil {
		model.DeploymentStatus = *m.DeploymentStatus
	}

	return model
}

func convertCacheEntry(e svcModels.CacheEntry) CacheEntry {
	entry := CacheEntry{
		ID:                e.ID.String(),
		ModelID:           e.ModelID.String(),
		Version:           e.Version,
		HFRevision:        e.HFRevision,
		ObjectStoragePath: e.ObjectStoragePath,
		Status:            e.Status,
		CachedAt:          e.CachedAt,
		VerifiedAt:        e.VerifiedAt,
	}

	if e.SizeBytes != nil {
		entry.SizeBytes = *e.SizeBytes
	}
	if e.FileCount != nil {
		entry.FileCount = *e.FileCount
	}
	if e.ChecksumSHA256 != nil {
		entry.ChecksumSHA256 = *e.ChecksumSHA256
	}

	return entry
}

func convertPullJob(j *svcModels.PullJob, modelName string) *PullJob {
	job := &PullJob{
		ID:             j.ID.String(),
		ModelName:      modelName,
		Status:         j.Status,
		Progress:       j.Progress,
		BytesCompleted: j.BytesCompleted,
		StartedAt:      j.StartedAt,
		CompletedAt:    j.CompletedAt,
	}

	if j.BytesTotal != nil {
		job.BytesTotal = *j.BytesTotal
	}
	if j.ErrorMessage != nil {
		job.Error = *j.ErrorMessage
	}

	return job
}

func convertDeployment(d svcModels.Deployment) Deployment {
	deployment := Deployment{
		ID:              d.ID.String(),
		ModelID:         d.ModelID.String(),
		Environment:     d.Environment,
		Namespace:       d.Namespace,
		Enabled:         d.Enabled,
		Status:          d.Status,
		ReplicasDesired: d.ReplicasDesired,
		ReplicasReady:   d.ReplicasReady,
		GPUCount:        d.GPUCount,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}

	if d.CacheID != nil {
		deployment.CacheID = d.CacheID.String()
	}
	if d.InferenceServiceName != nil {
		deployment.InferenceServiceName = *d.InferenceServiceName
	}
	if d.Endpoint != nil {
		deployment.Endpoint = *d.Endpoint
	}
	if d.MemoryGB != nil {
		deployment.MemoryGB = *d.MemoryGB
	}
	if d.LastHealthCheckAt != nil {
		deployment.LastHealthCheckAt = d.LastHealthCheckAt
	}
	if d.LastHealthStatus != nil {
		deployment.LastHealthStatus = *d.LastHealthStatus
	}
	if d.LastEnabledAt != nil {
		deployment.LastEnabledAt = d.LastEnabledAt
	}
	if d.LastEnabledBy != nil {
		deployment.LastEnabledBy = *d.LastEnabledBy
	}
	if d.LastDisabledAt != nil {
		deployment.LastDisabledAt = d.LastDisabledAt
	}
	if d.LastDisabledBy != nil {
		deployment.LastDisabledBy = *d.LastDisabledBy
	}

	return deployment
}

func convertValidationResult(r svcModels.ValidationResult) ValidationResult {
	return ValidationResult{
		ID:             r.ID.String(),
		ModelID:        r.ModelID.String(),
		Environment:    r.Environment,
		ValidationType: r.ValidationType,
		CheckName:      r.CheckName,
		Status:         r.Status,
		Message:        r.Message,
		Remediation:    r.Remediation,
		ValidatedAt:    r.ValidatedAt,
	}
}

func convertStateHistoryEntry(e svcModels.StateHistoryEntry) StateHistoryEntry {
	return StateHistoryEntry{
		ID:           e.ID.String(),
		DeploymentID: e.DeploymentID.String(),
		Action:       e.Action,
		PerformedBy:  e.PerformedBy,
		Reason:       e.Reason,
		ScheduledAt:  e.ScheduledAt,
		ExecutedAt:   e.ExecutedAt,
	}
}

func convertStateHistoryEntryWithModel(e svcModels.StateHistoryWithModel) StateHistoryEntry {
	return StateHistoryEntry{
		ID:           e.ID.String(),
		DeploymentID: e.DeploymentID.String(),
		ModelName:    e.ModelName,
		Environment:  e.Environment,
		Action:       e.Action,
		PerformedBy:  e.PerformedBy,
		Reason:       e.Reason,
		ScheduledAt:  e.ScheduledAt,
		ExecutedAt:   e.ExecutedAt,
	}
}

func convertAlias(a svcModels.Alias) Alias {
	return Alias{
		ID:          a.ID.String(),
		AliasName:   a.AliasName,
		ModelID:     a.ModelID.String(),
		ModelName:   a.ModelName,
		Description: a.Description,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
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
