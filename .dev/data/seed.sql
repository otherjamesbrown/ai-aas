-- Sample data seeding for local development stack
-- This script creates minimal test data for development and testing purposes.
-- Executed automatically on `make reset` or manually via `make seed-data`.

-- Note: This is a simplified seed for the local dev stack.
-- For full operational/analytics seeds, see db/seeds/operational/ and db/seeds/analytics/

BEGIN;

-- Ensure extensions are available
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Sample organization (if organizations table exists)
DO $$
DECLARE
  org_exists BOOLEAN;
BEGIN
  -- Check if organizations table exists
  SELECT EXISTS (
    SELECT FROM information_schema.tables 
    WHERE table_schema = 'public' 
    AND table_name = 'organizations'
  ) INTO org_exists;

  IF org_exists THEN
    -- Insert sample organization if it doesn't exist
    INSERT INTO organizations (organization_id, slug, display_name, plan_tier, budget_limit_tokens, created_at, updated_at)
    VALUES (
      '00000000-0000-0000-0000-000000000001',
      'dev-org',
      'Development Organization',
      'development',
      1000000,
      NOW(),
      NOW()
    )
    ON CONFLICT (organization_id) DO NOTHING;
  END IF;
END $$;

-- Sample user (if users table exists)
DO $$
DECLARE
  users_exists BOOLEAN;
BEGIN
  SELECT EXISTS (
    SELECT FROM information_schema.tables 
    WHERE table_schema = 'public' 
    AND table_name = 'users'
  ) INTO users_exists;

  IF users_exists THEN
    -- Note: Password should be hashed in production, this is for dev only
    -- Hash: bcrypt('dev-password') - $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
    INSERT INTO users (user_id, organization_id, email, role, password_hash, created_at, updated_at)
    VALUES (
      '00000000-0000-0000-0000-000000000001',
      '00000000-0000-0000-0000-000000000001',
      'dev@example.com',
      'admin',
      '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
      NOW(),
      NOW()
    )
    ON CONFLICT (user_id) DO NOTHING;
  END IF;
END $$;

-- Sample API key (if api_keys table exists)
DO $$
DECLARE
  keys_exists BOOLEAN;
BEGIN
  SELECT EXISTS (
    SELECT FROM information_schema.tables 
    WHERE table_schema = 'public' 
    AND table_name = 'api_keys'
  ) INTO keys_exists;

  IF keys_exists THEN
    INSERT INTO api_keys (api_key_id, organization_id, key_name, hashed_secret, created_at, expires_at)
    VALUES (
      '00000000-0000-0000-0000-000000000001',
      '00000000-0000-0000-0000-000000000001',
      'dev-api-key',
      'dev-hashed-secret-placeholder',
      NOW(),
      NOW() + INTERVAL '1 year'
    )
    ON CONFLICT (api_key_id) DO NOTHING;
  END IF;
END $$;

COMMIT;

-- Print seed completion message
DO $$
BEGIN
  RAISE NOTICE 'Sample data seeded successfully';
  RAISE NOTICE 'Organization: dev-org';
  RAISE NOTICE 'User: dev@example.com (password: dev-password)';
END $$;

