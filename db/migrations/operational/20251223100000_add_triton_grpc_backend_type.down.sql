ALTER TABLE routing_policies DROP CONSTRAINT IF EXISTS check_backend_type;
ALTER TABLE routing_policies
    ADD CONSTRAINT check_backend_type CHECK (backend_type IN ('openai', 'triton'));
