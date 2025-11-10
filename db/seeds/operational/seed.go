package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/ai_aas?sslmode=disable"
		log.Printf("DB_URL not provided, defaulting to %s", dsn)
	}

	if _, err := emailHashKey(); err != nil {
		log.Fatalf("email hash key: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	orgID, err := upsertOrganization(ctx, tx)
	if err != nil {
		log.Fatalf("seed organization: %v", err)
	}

	userID, err := upsertUser(ctx, tx, orgID)
	if err != nil {
		log.Fatalf("seed user: %v", err)
	}

	apiKeyID, err := upsertAPIKey(ctx, tx, orgID)
	if err != nil {
		log.Fatalf("seed api key: %v", err)
	}

	modelID, err := upsertModel(ctx, tx, orgID)
	if err != nil {
		log.Fatalf("seed model: %v", err)
	}

	if err := createAuditLog(ctx, tx, orgID, userID); err != nil {
		log.Fatalf("seed audit log: %v", err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}

	log.Printf("Seed completed: organization=%s user=%s api_key=%s model=%s", orgID, userID, apiKeyID, modelID)
}

func upsertOrganization(ctx context.Context, tx *sql.Tx) (uuid.UUID, error) {
	const stmt = `
INSERT INTO organizations (slug, display_name, plan_tier, budget_limit_tokens, status)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (slug) DO UPDATE
SET display_name = EXCLUDED.display_name,
    plan_tier = EXCLUDED.plan_tier,
    budget_limit_tokens = EXCLUDED.budget_limit_tokens,
    status = EXCLUDED.status
RETURNING organization_id`

	var orgID uuid.UUID
	err := tx.QueryRowContext(ctx, stmt,
		"demo-lab",
		"Demo Lab",
		"starter",
		1_000_000,
		"active",
	).Scan(&orgID)
	if err != nil {
		return uuid.Nil, err
	}
	return orgID, nil
}

func upsertUser(ctx context.Context, tx *sql.Tx, orgID uuid.UUID) (uuid.UUID, error) {
	email := "owner@example.com"
	encrypted := encryptEmail(email)
	emailHash := hashEmail(email)

	const stmt = `
INSERT INTO users (organization_id, email, email_hash, role, status)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (organization_id, email_hash) DO UPDATE
SET role = EXCLUDED.role,
    status = EXCLUDED.status
RETURNING user_id`

	var userID uuid.UUID
	err := tx.QueryRowContext(ctx, stmt,
		orgID,
		encrypted,
		emailHash,
		"owner",
		"active",
	).Scan(&userID)
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

func upsertAPIKey(ctx context.Context, tx *sql.Tx, orgID uuid.UUID) (uuid.UUID, error) {
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte("demo-lab-internal-secret"), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, fmt.Errorf("bcrypt secret: %w", err)
	}

	const stmt = `
INSERT INTO api_keys (organization_id, name, hashed_secret, scopes, status)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (organization_id, name) DO UPDATE
SET hashed_secret = EXCLUDED.hashed_secret,
    scopes = EXCLUDED.scopes,
    status = EXCLUDED.status
RETURNING api_key_id`

	var apiKeyID uuid.UUID
	err = tx.QueryRowContext(ctx, stmt,
		orgID,
		"demo-lab-internal",
		hashedSecret,
		[]string{"usage:read", "usage:write"},
		"active",
	).Scan(&apiKeyID)
	if err != nil {
		return uuid.Nil, err
	}
	return apiKeyID, nil
}

func upsertModel(ctx context.Context, tx *sql.Tx, orgID uuid.UUID) (uuid.UUID, error) {
	const stmt = `
INSERT INTO model_registry_entries (organization_id, model_name, revision, deployment_target, cost_per_1k_tokens, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (organization_id, model_name, revision) DO UPDATE
SET deployment_target = EXCLUDED.deployment_target,
    cost_per_1k_tokens = EXCLUDED.cost_per_1k_tokens,
    metadata = EXCLUDED.metadata
RETURNING model_id`

	var modelID uuid.UUID
	err := tx.QueryRowContext(ctx, stmt,
		orgID,
		"gpt-lite",
		1,
		"managed",
		0.25,
		`{"provider":"internal","max_tokens":16000}`,
	).Scan(&modelID)
	if err != nil {
		return uuid.Nil, err
	}
	return modelID, nil
}

func createAuditLog(ctx context.Context, tx *sql.Tx, orgID, userID uuid.UUID) error {
	const stmt = `
INSERT INTO audit_log_entries (actor_type, actor_id, action, target, metadata)
VALUES ($1, $2, $3, $4, $5)`

	_, err := tx.ExecContext(ctx, stmt,
		"user",
		userID.String(),
		"seed_data_initialized",
		fmt.Sprintf("organizations/%s", orgID),
		`{"source":"db/seeds/operational"}`,
	)
	return err
}

func encryptEmail(email string) string {
	hash := hashEmail(email)
	return "enc:" + hash
}

func hashEmail(email string) string {
	key, err := emailHashKey()
	if err != nil {
		log.Fatalf("email hash key: %v", err)
	}

	mac := hmac.New(sha256.New, key)
	if _, err := mac.Write([]byte(strings.ToLower(email))); err != nil {
		log.Fatalf("hash email write: %v", err)
	}
	sum := mac.Sum(nil)
	return hex.EncodeToString(sum)
}

var (
	emailHashKeyOnce sync.Once
	emailHashKeyVal  []byte
	emailHashKeyErr  error
)

func emailHashKey() ([]byte, error) {
	emailHashKeyOnce.Do(func() {
		key := strings.TrimSpace(os.Getenv("MIGRATION_EMAIL_HASH_KEY"))
		if key == "" {
			key = strings.TrimSpace(os.Getenv("EMAIL_HASH_KEY"))
		}
		if key == "" {
			emailHashKeyErr = fmt.Errorf("MIGRATION_EMAIL_HASH_KEY (or EMAIL_HASH_KEY) must be set")
			return
		}
		emailHashKeyVal = []byte(key)
	})
	return emailHashKeyVal, emailHashKeyErr
}
