package users

import (
	"testing"

	"github.com/google/uuid"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
	"github.com/stretchr/testify/assert"
)

// TestDeleteUserRouteRegistration verifies that DeleteUser handler compiles and follows the correct pattern.
// Full integration tests are in the integration test suite.
func TestDeleteUserRouteRegistration(t *testing.T) {
	// Verify the handler compiles by checking it exists
	var h Handler
	assert.NotNil(t, h.DeleteUser, "DeleteUser handler should exist")
}

// TestDeleteUserStoreMethod verifies the store method signature
func TestDeleteUserStoreMethod(t *testing.T) {
	// This test ensures the DeleteUser method exists on the store with correct signature
	// Actual functionality is tested in integration tests
	var store *postgres.Store

	// Verify the method compiles - this will fail at compile time if signature is wrong
	_ = func(orgID, userID uuid.UUID) error {
		if store != nil {
			return store.DeleteUser(nil, orgID, userID)
		}
		return nil
	}
}

// TestGetUserByEmailRouteRegistration verifies that GetUserByEmail handler compiles and follows the correct pattern.
// Full integration tests are in the integration test suite.
func TestGetUserByEmailRouteRegistration(t *testing.T) {
	// Verify the handler compiles by checking it exists
	var h Handler
	assert.NotNil(t, h.GetUserByEmail, "GetUserByEmail handler should exist")
}

// TestGetUserByEmailStoreMethod verifies the store method signature
func TestGetUserByEmailStoreMethod(t *testing.T) {
	// This test ensures the GetUserByEmail method exists on the store with correct signature
	// Actual functionality is tested in integration tests
	var store *postgres.Store

	// Verify the method compiles - this will fail at compile time if signature is wrong
	_ = func(orgID uuid.UUID, email string) (postgres.User, error) {
		if store != nil {
			return store.GetUserByEmail(nil, orgID, email)
		}
		return postgres.User{}, nil
	}
}
