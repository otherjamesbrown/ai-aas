ALTER TABLE routing_policies DROP CONSTRAINT IF EXISTS check_backend_type;
ALTER TABLE routing_policies DROP COLUMN IF EXISTS tokenizer;
ALTER TABLE routing_policies DROP COLUMN IF EXISTS backend_type;
