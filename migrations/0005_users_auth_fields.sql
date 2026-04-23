ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';

UPDATE users
SET
    email = COALESCE(email, ''),
    updated_at = CASE
        WHEN COALESCE(updated_at, '') = '' THEN COALESCE(created_at, strftime('%Y-%m-%dT%H:%M:%fZ','now'))
        ELSE updated_at
    END
WHERE email IS NULL OR COALESCE(updated_at, '') = '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique_nonempty
ON users(email)
WHERE email <> '';
