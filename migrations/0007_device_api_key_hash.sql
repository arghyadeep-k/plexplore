ALTER TABLE devices ADD COLUMN api_key_hash TEXT;
ALTER TABLE devices ADD COLUMN api_key_preview TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_api_key_hash
ON devices(api_key_hash)
WHERE api_key_hash IS NOT NULL AND trim(api_key_hash) <> '';
