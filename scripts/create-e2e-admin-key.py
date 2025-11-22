#!/usr/bin/env python3
"""
Generate an admin API key for e2e tests and output SQL to insert it into the database.
"""
import secrets
import hashlib
import base64
from datetime import datetime, timedelta
import uuid

# Generate a random 32-byte secret
secret_bytes = secrets.token_bytes(32)
secret = base64.urlsafe_b64encode(secret_bytes).decode('utf-8').rstrip('=')

# Compute SHA-256 fingerprint of the secret
fingerprint_hash = hashlib.sha256(secret.encode('utf-8')).digest()
fingerprint = base64.urlsafe_b64encode(fingerprint_hash).decode('utf-8').rstrip('=')

# Generate UUIDs for org, user, and API key
org_id = str(uuid.uuid4())
user_id = str(uuid.uuid4())
api_key_id = str(uuid.uuid4())

# Set expiration to 1 year from now
expires_at = datetime.utcnow() + timedelta(days=365)

# Create password hash for user (bcrypt hash of "e2e-admin-password")
# This is the bcrypt hash of "e2e-admin-password" with cost 10
password_hash = "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi"

print(f"""
-- ========================================
-- E2E Admin Setup for AI-AAS Dev Cluster
-- ========================================
-- Generated: {datetime.utcnow().isoformat()}Z
--
-- SAVE THIS INFORMATION SECURELY:
--
-- Organization: e2e-admin-org (slug)
-- Email: e2e-admin@ai-aas.dev
-- Password: e2e-admin-password
-- API Key Secret: {secret}
--
-- ========================================

BEGIN;

-- Create E2E Admin Organization
INSERT INTO orgs (org_id, slug, name, status, declarative_mode, mfa_required_roles, metadata, created_at, updated_at)
VALUES (
    '{org_id}',
    'e2e-admin-org',
    'E2E Test Admin Organization',
    'active',
    'disabled',
    '[]'::jsonb,
    '{{}}'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (slug) DO UPDATE
SET status = 'active';

-- Create E2E Admin User
INSERT INTO users (user_id, org_id, email, display_name, password_hash, status, mfa_enrolled, mfa_methods, recovery_tokens, metadata, created_at, updated_at)
VALUES (
    '{user_id}',
    '{org_id}',
    'e2e-admin@ai-aas.dev',
    'E2E Admin User',
    '{password_hash}',
    'active',
    false,
    '[]'::jsonb,
    '[]'::jsonb,
    '{{}}'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (org_id, email) DO UPDATE
SET status = 'active', password_hash = '{password_hash}';

-- Create E2E Admin API Key
INSERT INTO api_keys (api_key_id, org_id, principal_type, principal_id, fingerprint, status, scopes, issued_at, expires_at, annotations, created_at, updated_at)
VALUES (
    '{api_key_id}',
    '{org_id}',
    'user',
    '{user_id}',
    '{fingerprint}',
    'active',
    '["*"]'::jsonb,
    NOW(),
    NOW() + INTERVAL '365 days',
    '{{"purpose": "e2e-tests", "created_by": "script"}}'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (org_id, fingerprint) DO UPDATE
SET status = 'active', expires_at = NOW() + INTERVAL '365 days';

COMMIT;

-- ========================================
-- Verification Queries
-- ========================================
-- Run these to verify the setup:
--
-- SELECT * FROM orgs WHERE slug = 'e2e-admin-org';
-- SELECT * FROM users WHERE email = 'e2e-admin@ai-aas.dev';
-- SELECT * FROM api_keys WHERE principal_id = '{user_id}';
""")

# Also print the API key secret separately for easy copying
print("\n" + "="*60)
print("E2E ADMIN API KEY (Save this securely!):")
print("="*60)
print(f"API Key Secret: {secret}")
print(f"Org ID: {org_id}")
print(f"User ID: {user_id}")
print(f"API Key ID: {api_key_id}")
print("="*60)
