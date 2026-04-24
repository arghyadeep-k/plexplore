CREATE TABLE IF NOT EXISTS visit_generation_state (
    device_name TEXT PRIMARY KEY,
    last_processed_seq INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX IF NOT EXISTS idx_visit_generation_state_updated_at
ON visit_generation_state(updated_at);
