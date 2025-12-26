// Package models provides the business logic for model management operations.
package models

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateDeployment_AutoRegistersModel verifies that CreateDeployment
// automatically registers a model in the registry if it doesn't exist
func TestCreateDeployment_AutoRegistersModel(t *testing.T) {
	// This test requires a database connection - skip if DB_URL not set
	dbURL := getTestDBURL(t)
	if dbURL == "" {
		t.Skip("Skipping test - DB_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "failed to connect to test database")
	defer pool.Close()

	// Create service with test database
	encryptor := &noopEncryptor{}
	svc := NewService(pool, encryptor)

	// Clean up test data
	testModelName := "test-auto-register-model-" + uuid.New().String()[:8]
	defer func() {
		_ = svc.RemoveModel(ctx, testModelName, true)
	}()

	// Verify model doesn't exist yet
	_, err = svc.GetModel(ctx, testModelName)
	assert.ErrorIs(t, err, ErrModelNotFound, "model should not exist before test")

	// Create deployment - should auto-register the model
	req := CreateDeploymentRequest{
		ModelName:   testModelName,
		Environment: "development",
		Namespace:   "system",
		GPUCount:    1,
		MemoryGB:    16,
		Replicas:    1,
	}

	deployment, err := svc.CreateDeployment(ctx, req)
	require.NoError(t, err, "CreateDeployment should auto-register model")
	require.NotNil(t, deployment, "deployment should be created")

	// Verify model was auto-registered
	model, err := svc.GetModel(ctx, testModelName)
	require.NoError(t, err, "model should now exist in registry")
	assert.Equal(t, testModelName, model.Name, "model name should match")
	assert.Equal(t, testModelName, model.HFModelID, "HF model ID should default to model name")

	// Verify deployment was created
	assert.Equal(t, testModelName, deployment.ModelID.String(), "deployment should reference model ID")
	assert.Equal(t, "development", deployment.Environment, "deployment environment should match")
	assert.Equal(t, "system", deployment.Namespace, "deployment namespace should match")
}

// TestCreateDeployment_ExistingModel verifies that CreateDeployment
// works correctly when the model already exists in the registry
func TestCreateDeployment_ExistingModel(t *testing.T) {
	// This test requires a database connection - skip if DB_URL not set
	dbURL := getTestDBURL(t)
	if dbURL == "" {
		t.Skip("Skipping test - DB_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "failed to connect to test database")
	defer pool.Close()

	// Create service with test database
	encryptor := &noopEncryptor{}
	svc := NewService(pool, encryptor)

	// Create a test model first
	testModelName := "test-existing-model-" + uuid.New().String()[:8]
	defer func() {
		_ = svc.RemoveModel(ctx, testModelName, true)
	}()

	addReq := AddModelRequest{
		Name:        testModelName,
		HFModelID:   "test/model-id",
		GPUMemoryGB: 24,
		CPUMemoryGB: 32,
	}
	model, err := svc.AddModel(ctx, addReq)
	require.NoError(t, err, "failed to create test model")

	// Create deployment for existing model
	deployReq := CreateDeploymentRequest{
		ModelName:   testModelName,
		Environment: "development",
		Namespace:   "system",
		GPUCount:    1,
		MemoryGB:    16,
		Replicas:    1,
	}

	deployment, err := svc.CreateDeployment(ctx, deployReq)
	require.NoError(t, err, "CreateDeployment should work with existing model")
	require.NotNil(t, deployment, "deployment should be created")

	// Verify deployment references the existing model
	assert.Equal(t, model.ID, deployment.ModelID, "deployment should reference existing model ID")
	assert.Equal(t, "development", deployment.Environment, "deployment environment should match")

	// Verify model metadata wasn't changed
	updatedModel, err := svc.GetModel(ctx, testModelName)
	require.NoError(t, err, "model should still exist")
	assert.Equal(t, "test/model-id", updatedModel.HFModelID, "HF model ID should not be changed")
	assert.Equal(t, int32(24), *updatedModel.RecommendedGPUMemoryGB, "GPU memory should not be changed")
}

// getTestDBURL returns the database URL for testing, or empty string if not configured
func getTestDBURL(t *testing.T) string {
	// In CI/CD or test environments, this should be set
	// For local testing, developers need to provide a test database
	// For now, skip tests if not set
	return "" // TODO: Add support for test database URL via env var
}

// noopEncryptor is a test encryptor that doesn't actually encrypt
type noopEncryptor struct{}

func (e *noopEncryptor) Encrypt(plaintext string) (string, error) {
	return plaintext, nil
}

func (e *noopEncryptor) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}
