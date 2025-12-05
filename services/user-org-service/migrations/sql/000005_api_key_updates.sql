-- Migration: 000005_api_key_updates
-- Description: Add key_id (short identifier) and notes columns to api_keys table
-- The key_id is a shorter, more user-friendly identifier than the UUID api_key_id
-- Notes provides a human-readable description (like GitHub's token name)

-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
  -- Add key_id column if it doesn't exist (12 char alphanumeric identifier)
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                 WHERE table_name = 'api_keys' AND column_name = 'key_id') THEN
    ALTER TABLE api_keys ADD COLUMN key_id TEXT;

    -- Populate existing rows with generated key_ids
    UPDATE api_keys SET key_id = LOWER(SUBSTRING(REPLACE(gen_random_uuid()::text, '-', ''), 1, 12))
    WHERE key_id IS NULL;

    -- Make it NOT NULL and UNIQUE after populating
    ALTER TABLE api_keys ALTER COLUMN key_id SET NOT NULL;
    CREATE UNIQUE INDEX IF NOT EXISTS api_keys_key_id_idx ON api_keys(key_id);
  END IF;

  -- Add notes column if it doesn't exist (human-readable description)
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                 WHERE table_name = 'api_keys' AND column_name = 'notes') THEN
    ALTER TABLE api_keys ADD COLUMN notes TEXT NOT NULL DEFAULT '';
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  -- Remove notes column if it exists
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_name = 'api_keys' AND column_name = 'notes') THEN
    ALTER TABLE api_keys DROP COLUMN notes;
  END IF;

  -- Remove key_id column and index if they exist
  DROP INDEX IF EXISTS api_keys_key_id_idx;
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_name = 'api_keys' AND column_name = 'key_id') THEN
    ALTER TABLE api_keys DROP COLUMN key_id;
  END IF;
END $$;
-- +goose StatementEnd
