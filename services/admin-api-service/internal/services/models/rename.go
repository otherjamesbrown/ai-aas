// Package models provides the business logic for model management operations.
package models

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrModelDeployed indicates the model has active deployments
var ErrModelDeployed = errors.New("model has active deployments")

// ErrModelNameExists indicates the new name is already in use
var ErrModelNameExists = errors.New("model name already exists")

// RenameModelRequest contains the data for renaming a model
type RenameModelRequest struct {
	NewName      string `json:"new_name"`
	MigrateCache bool   `json:"migrate_cache"`
}

// RenameModelResponse contains the result of a rename operation
type RenameModelResponse struct {
	OldName        string `json:"old_name"`
	NewName        string `json:"new_name"`
	CacheMigrated  bool   `json:"cache_migrated"`
	CacheSizeBytes int64  `json:"cache_size_bytes,omitempty"`
	CacheFileCount int    `json:"cache_file_count,omitempty"`
}

// CredentialGetter retrieves credentials from storage
type CredentialGetter interface {
	GetCredential(ctx context.Context, credType string) (string, error)
}

// RenameModel renames a model in the registry and optionally migrates cached files
func (s *Service) RenameModel(ctx context.Context, oldName string, req RenameModelRequest, credGetter CredentialGetter) (*RenameModelResponse, error) {
	// Validate new name format (KServe naming requirements)
	if err := ValidateKServeName(req.NewName); err != nil {
		return nil, err
	}

	// Start a transaction for atomic database updates
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get the existing model
	var modelID string
	var currentName string
	err = tx.QueryRow(ctx, `
		SELECT id, name FROM model_registry WHERE name = $1
	`, oldName).Scan(&modelID, &currentName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrModelNotFound
		}
		return nil, fmt.Errorf("query model: %w", err)
	}

	// Check if new name already exists
	var existingCount int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM model_registry WHERE name = $1
	`, req.NewName).Scan(&existingCount)
	if err != nil {
		return nil, fmt.Errorf("check name existence: %w", err)
	}
	if existingCount > 0 {
		return nil, ErrModelNameExists
	}

	// Check for active deployments (any status except 'terminated' and 'disabled')
	var activeDeployments int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM model_deployments
		WHERE model_id = $1 AND status NOT IN ('terminated', 'disabled')
	`, modelID).Scan(&activeDeployments)
	if err != nil {
		return nil, fmt.Errorf("check deployments: %w", err)
	}
	if activeDeployments > 0 {
		return nil, fmt.Errorf("%w: found %d active deployment(s)", ErrModelDeployed, activeDeployments)
	}

	// Update the model name in the registry
	_, err = tx.Exec(ctx, `
		UPDATE model_registry SET name = $1 WHERE id = $2
	`, req.NewName, modelID)
	if err != nil {
		return nil, fmt.Errorf("update model name: %w", err)
	}

	response := &RenameModelResponse{
		OldName:       oldName,
		NewName:       req.NewName,
		CacheMigrated: false,
	}

	// If migrate_cache is true, move S3 files
	if req.MigrateCache {
		// Get S3 credentials
		s3AccessKey, err := credGetter.GetCredential(ctx, "s3-access")
		if err != nil {
			return nil, fmt.Errorf("get S3 access key: %w", err)
		}

		s3SecretKey, err := credGetter.GetCredential(ctx, "s3-secret")
		if err != nil {
			return nil, fmt.Errorf("get S3 secret key: %w", err)
		}

		s3Endpoint, err := credGetter.GetCredential(ctx, "s3-endpoint")
		if err != nil {
			return nil, fmt.Errorf("get S3 endpoint: %w", err)
		}

		s3Bucket, err := credGetter.GetCredential(ctx, "s3-bucket")
		if err != nil {
			return nil, fmt.Errorf("get S3 bucket: %w", err)
		}

		s3Region, _ := credGetter.GetCredential(ctx, "s3-region")
		if s3Region == "" {
			s3Region = "us-east-1"
		}

		// Create S3 client
		s3Client, err := s.createS3Client(ctx, s3Endpoint, s3AccessKey, s3SecretKey, s3Bucket, s3Region)
		if err != nil {
			return nil, fmt.Errorf("create S3 client: %w", err)
		}

		// Move files from models/{oldName}/ to models/{newName}/
		oldPrefix := fmt.Sprintf("models/%s", oldName)
		newPrefix := fmt.Sprintf("models/%s", req.NewName)

		totalSize, fileCount, err := s3Client.MovePrefix(ctx, oldPrefix, newPrefix)
		if err != nil {
			// Rollback transaction if S3 migration fails
			return nil, fmt.Errorf("migrate S3 files: %w", err)
		}

		// Update cache entries to reflect new storage path
		if fileCount > 0 {
			_, err = tx.Exec(ctx, `
				UPDATE model_cache
				SET object_storage_path = REPLACE(object_storage_path, $1, $2)
				WHERE model_id = $3
			`, oldPrefix, newPrefix, modelID)
			if err != nil {
				return nil, fmt.Errorf("update cache paths: %w", err)
			}

			response.CacheMigrated = true
			response.CacheSizeBytes = totalSize
			response.CacheFileCount = fileCount
		}
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return response, nil
}

// S3Client defines the interface for S3 operations
type S3Client interface {
	MovePrefix(ctx context.Context, oldPrefix, newPrefix string) (totalSize int64, fileCount int, err error)
}

// S3ClientFactory creates S3 clients with the given credentials
type S3ClientFactory func(ctx context.Context, endpoint, accessKey, secretKey, bucket, region string) (S3Client, error)

// createS3Client creates an S3 client with the given credentials
// This uses a factory function that can be injected for testing
func (s *Service) createS3Client(ctx context.Context, endpoint, accessKey, secretKey, bucket, region string) (S3Client, error) {
	if s.s3ClientFactory == nil {
		return nil, fmt.Errorf("S3 client factory not configured")
	}
	return s.s3ClientFactory(ctx, endpoint, accessKey, secretKey, bucket, region)
}
