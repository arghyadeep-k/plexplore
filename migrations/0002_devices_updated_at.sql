ALTER TABLE devices ADD COLUMN updated_at TEXT;

UPDATE devices
SET updated_at = COALESCE(
    last_seen_at,
    created_at,
    strftime('%Y-%m-%dT%H:%M:%fZ','now')
)
WHERE updated_at IS NULL OR trim(updated_at) = '';
