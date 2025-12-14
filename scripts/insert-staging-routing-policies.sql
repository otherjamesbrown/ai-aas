-- Insert Routing Policies for Staging Environment
-- This creates routing policies for the three models deployed in staging

-- Policy for mistral-7b-instruct-v03
INSERT INTO routing_policies (
    policy_id,
    organization_id,
    model,
    backends,
    fallback_backends,
    failover_threshold,
    enabled,
    version,
    metadata,
    created_at,
    updated_at,
    created_by,
    updated_by
) VALUES (
    gen_random_uuid(),
    '*',  -- Global policy
    'mistral-7b-instruct-v03',
    '[{"backend_id": "staging-mistral-7b-instruct-v03", "weight": 100}]'::jsonb,
    '[]'::jsonb,
    1,
    true,
    1,
    '{"environment": "staging", "backend_type": "kserve", "created_via": "sql_script"}'::jsonb,
    NOW(),
    NOW(),
    'system-setup',
    'system-setup'
) ON CONFLICT (organization_id, model) DO UPDATE SET
    backends = EXCLUDED.backends,
    enabled = EXCLUDED.enabled,
    updated_at = NOW(),
    version = routing_policies.version + 1;

-- Policy for openai/gpt-oss-20b
INSERT INTO routing_policies (
    policy_id,
    organization_id,
    model,
    backends,
    fallback_backends,
    failover_threshold,
    enabled,
    version,
    metadata,
    created_at,
    updated_at,
    created_by,
    updated_by
) VALUES (
    gen_random_uuid(),
    '*',
    'openai/gpt-oss-20b',
    '[{"backend_id": "staging-openai-gpt-oss-20b", "weight": 100}]'::jsonb,
    '[]'::jsonb,
    1,
    true,
    1,
    '{"environment": "staging", "backend_type": "kserve", "created_via": "sql_script"}'::jsonb,
    NOW(),
    NOW(),
    'system-setup',
    'system-setup'
) ON CONFLICT (organization_id, model) DO UPDATE SET
    backends = EXCLUDED.backends,
    enabled = EXCLUDED.enabled,
    updated_at = NOW(),
    version = routing_policies.version + 1;

-- Policy for unsloth/gpt-oss-20b
INSERT INTO routing_policies (
    policy_id,
    organization_id,
    model,
    backends,
    fallback_backends,
    failover_threshold,
    enabled,
    version,
    metadata,
    created_at,
    updated_at,
    created_by,
    updated_by
) VALUES (
    gen_random_uuid(),
    '*',
    'unsloth/gpt-oss-20b',
    '[{"backend_id": "staging-unsloth-gpt-oss-20b", "weight": 100}]'::jsonb,
    '[]'::jsonb,
    1,
    true,
    1,
    '{"environment": "staging", "backend_type": "kserve", "created_via": "sql_script"}'::jsonb,
    NOW(),
    NOW(),
    'system-setup',
    'system-setup'
) ON CONFLICT (organization_id, model) DO UPDATE SET
    backends = EXCLUDED.backends,
    enabled = EXCLUDED.enabled,
    updated_at = NOW(),
    version = routing_policies.version + 1;

-- Verify inserted policies
SELECT
    policy_id,
    organization_id,
    model,
    backends,
    enabled,
    version,
    created_at
FROM routing_policies
WHERE organization_id = '*'
ORDER BY model;
