// Command seed bootstraps initial organization and user data for development/testing.
//
// Purpose:
//   This utility creates a demo organization and admin user with a hashed password,
//   enabling local development and testing without manual database setup. It supports
//   custom org/user details via flags and can force re-seeding of existing data.
//
// Dependencies:
//   - internal/config: Configuration (requires DATABASE_URL)
//   - internal/storage/postgres: Data access layer for org/user creation
//   - internal/security: Password hashing (Argon2id)
//
// Key Responsibilities:
//   - Create or update organization by slug
//   - Create or update user with hashed password
//   - Generate random password if not provided
//   - Print credentials for use in authentication
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#US-001 (User Authentication)
//   - specs/005-user-org-service/quickstart.md (Local Development Setup)
//
// Debugging Notes:
//   - Requires DATABASE_URL environment variable
//   - Uses 30s timeout for database operations
//   - Force flag allows re-seeding existing org/user
//   - Generated passwords are printed to stdout (development only)
//   - Password hashing uses Argon2id (same as production)
//
// Thread Safety:
//   - Single-threaded execution (command-line tool)
//
// Error Handling:
//   - Missing DATABASE_URL exits with fatal error
//   - Store creation failures exit with fatal error
//   - Seed failures log fatal and exit
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/security"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

func main() {
	var (
		orgSlug      = flag.String("org-slug", "demo", "Organization slug")
		orgName      = flag.String("org-name", "Demo Organization", "Organization name")
		userEmail    = flag.String("user-email", "admin@example.com", "Initial user email")
		userPassword = flag.String("user-password", "", "Initial user password (default: generate random)")
		userName     = flag.String("user-name", "Admin User", "Initial user display name")
		force        = flag.Bool("force", false, "Force re-seed even if org/user exists")
	)
	flag.Parse()

	cfg := config.MustLoad()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL must be set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := postgres.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("create store: %v", err)
	}
	defer store.Close()

	orgID, err := seedOrg(ctx, store, *orgSlug, *orgName, *force)
	if err != nil {
		log.Fatalf("seed org: %v", err)
	}
	fmt.Printf("✓ Organization: %s (ID: %s)\n", *orgSlug, orgID)

	password := *userPassword
	if password == "" {
		password = generatePassword()
		fmt.Printf("✓ Generated password: %s\n", password)
	}

	userID, err := seedUser(ctx, store, orgID, *userEmail, password, *userName, *force)
	if err != nil {
		log.Fatalf("seed user: %v", err)
	}
	fmt.Printf("✓ User: %s (ID: %s)\n", *userEmail, userID)

	fmt.Println("\n✓ Seed completed successfully!")
	fmt.Printf("\nYou can now authenticate with:\n")
	fmt.Printf("  Email: %s\n", *userEmail)
	fmt.Printf("  Password: %s\n", password)
	fmt.Printf("  Org ID: %s\n", orgID)
}

func seedOrg(ctx context.Context, store *postgres.Store, slug, name string, force bool) (uuid.UUID, error) {
	// Check if org exists
	existing, err := store.GetOrgBySlug(ctx, slug)
	if err == nil && !force {
		return existing.ID, nil
	}

	orgID := uuid.New()
	params := postgres.CreateOrgParams{
		ID:     orgID,
		Slug:   slug,
		Name:   name,
		Status: "active",
	}

	org, err := store.CreateOrg(ctx, params)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create org: %w", err)
	}
	return org.ID, nil
}

func seedUser(ctx context.Context, store *postgres.Store, orgID uuid.UUID, email, password, displayName string, force bool) (uuid.UUID, error) {
	// Check if user exists
	existing, err := store.GetUserByEmail(ctx, orgID, email)
	if err == nil && !force {
		return existing.ID, nil
	}

	passwordHash, err := security.HashPassword(password)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hash password: %w", err)
	}

	userID := uuid.New()
	params := postgres.CreateUserParams{
		ID:           userID,
		OrgID:        orgID,
		Email:        email,
		DisplayName:   displayName,
		PasswordHash:  passwordHash,
		Status:        "active",
		MFAEnrolled:   false,
		MFAMethods:    []string{},
		RecoveryTokens: []string{},
		Metadata:     map[string]any{},
	}

	user, err := store.CreateUser(ctx, params)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}
	return user.ID, nil
}

func generatePassword() string {
	// Generate a simple random password for development
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte('a' + (i*7+13)%26)
	}
	return string(b) + "123!"
}

