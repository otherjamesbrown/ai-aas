// Command seed-test-users creates test users and organizations for development/testing.
//
// This utility creates:
// - System Admin user (no org, stored in metadata)
// - Acme Ltd organization with admin and manager users
// - JoeBlogs Ltd organization with admin and manager users
//
// Roles are stored in user metadata as {"roles": ["role_name"]}
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
	force := flag.Bool("force", false, "Force re-seed even if org/user exists")
	flag.Parse()

	cfg := config.MustLoad()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL must be set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	store, err := postgres.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("create store: %v", err)
	}
	defer store.Close()

	// System Admin (no org, but we need to create a system org for it)
	fmt.Println("Creating System Admin...")
	systemOrgID, err := seedOrg(ctx, store, "system", "System", *force)
	if err != nil {
		log.Fatalf("seed system org: %v", err)
	}
	sysAdminID, err := seedUserWithRole(ctx, store, systemOrgID, "sys-admin@example.com", "SysAdmin2024!SecurePass", "System Administrator", []string{"system_admin"}, *force)
	if err != nil {
		log.Fatalf("seed system admin: %v", err)
	}
	fmt.Printf("  ✓ System Admin: sys-admin@example.com (ID: %s)\n", sysAdminID)

	// Acme Ltd Organization
	fmt.Println("\nCreating Acme Ltd organization...")
	acmeOrgID, err := seedOrg(ctx, store, "acme-ltd", "Acme Ltd", *force)
	if err != nil {
		log.Fatalf("seed acme org: %v", err)
	}
	fmt.Printf("  ✓ Organization: Acme Ltd (ID: %s)\n", acmeOrgID)

	// Acme Admin
	acmeAdminID, err := seedUserWithRole(ctx, store, acmeOrgID, "admin@example-acme.com", "AcmeAdmin2024!Secure", "Acme Admin", []string{"admin"}, *force)
	if err != nil {
		log.Fatalf("seed acme admin: %v", err)
	}
	fmt.Printf("  ✓ Admin: admin@example-acme.com (ID: %s)\n", acmeAdminID)

	// Acme Manager
	acmeManagerID, err := seedUserWithRole(ctx, store, acmeOrgID, "manager@example-acme.com", "AcmeManager2024!Secure", "Acme Manager", []string{"manager"}, *force)
	if err != nil {
		log.Fatalf("seed acme manager: %v", err)
	}
	fmt.Printf("  ✓ Manager: manager@example-acme.com (ID: %s)\n", acmeManagerID)

	// JoeBlogs Ltd Organization
	fmt.Println("\nCreating JoeBlogs Ltd organization...")
	joeblogsOrgID, err := seedOrg(ctx, store, "joeblogs-ltd", "JoeBlogs Ltd", *force)
	if err != nil {
		log.Fatalf("seed joeblogs org: %v", err)
	}
	fmt.Printf("  ✓ Organization: JoeBlogs Ltd (ID: %s)\n", joeblogsOrgID)

	// JoeBlogs Admin
	joeblogsAdminID, err := seedUserWithRole(ctx, store, joeblogsOrgID, "admin@example-joeblogs.com", "JoeBlogsAdmin2024!Secure", "JoeBlogs Admin", []string{"admin"}, *force)
	if err != nil {
		log.Fatalf("seed joeblogs admin: %v", err)
	}
	fmt.Printf("  ✓ Admin: admin@example-joeblogs.com (ID: %s)\n", joeblogsAdminID)

	// JoeBlogs Manager
	joeblogsManagerID, err := seedUserWithRole(ctx, store, joeblogsOrgID, "manager@example-joeblogs.com", "JoeBlogsManager2024!Secure", "JoeBlogs Manager", []string{"manager"}, *force)
	if err != nil {
		log.Fatalf("seed joeblogs manager: %v", err)
	}
	fmt.Printf("  ✓ Manager: manager@example-joeblogs.com (ID: %s)\n", joeblogsManagerID)

	fmt.Println("\n✓ All test users seeded successfully!")
	fmt.Println("\nSee seeded-users.md for credentials.")
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

func seedUserWithRole(ctx context.Context, store *postgres.Store, orgID uuid.UUID, email, password, displayName string, roles []string, force bool) (uuid.UUID, error) {
	// Check if user exists
	existing, err := store.GetUserByEmail(ctx, orgID, email)
	if err == nil && !force {
		// Update roles if needed
		if existing.Metadata == nil {
			existing.Metadata = make(map[string]any)
		}
		existing.Metadata["roles"] = roles
		// TODO: Update user metadata if needed
		return existing.ID, nil
	}

	passwordHash, err := security.HashPassword(password)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hash password: %w", err)
	}

	userID := uuid.New()
	metadata := map[string]any{
		"roles": roles,
	}

	params := postgres.CreateUserParams{
		ID:             userID,
		OrgID:          orgID,
		Email:          email,
		DisplayName:    displayName,
		PasswordHash:   passwordHash,
		Status:         "active",
		MFAEnrolled:    false,
		MFAMethods:     []string{},
		RecoveryTokens: []string{},
		Metadata:       metadata,
	}

	user, err := store.CreateUser(ctx, params)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}
	return user.ID, nil
}

