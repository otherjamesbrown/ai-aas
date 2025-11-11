-- Sample analytics seed data for local/testing environments
-- Assumes operational seed has created demo-lab entities and API keys.

INSERT INTO usage_events (occurred_at, organization_id, api_key_id, model_id, tokens_consumed, latency_ms, status, error_code, region, cost_usd)
SELECT NOW() - INTERVAL '15 minutes', org.organization_id, key.api_key_id, model.model_id,
       1200, 320, 'success', NULL, 'us-east-1', 0.30
FROM organizations org
JOIN api_keys key ON key.organization_id = org.organization_id AND key.name = 'demo-lab-internal'
JOIN model_registry_entries model ON model.organization_id = org.organization_id AND model.model_name = 'gpt-lite' AND model.revision = 1
WHERE org.slug = 'demo-lab'
ON CONFLICT DO NOTHING;

INSERT INTO usage_events (occurred_at, organization_id, api_key_id, model_id, tokens_consumed, latency_ms, status, error_code, region, cost_usd)
SELECT NOW() - INTERVAL '45 minutes', org.organization_id, key.api_key_id, model.model_id,
       800, 210, 'error', 'timeout', 'us-east-1', 0.00
FROM organizations org
JOIN api_keys key ON key.organization_id = org.organization_id AND key.name = 'demo-lab-internal'
JOIN model_registry_entries model ON model.organization_id = org.organization_id AND model.model_name = 'gpt-lite' AND model.revision = 1
WHERE org.slug = 'demo-lab'
ON CONFLICT DO NOTHING;
