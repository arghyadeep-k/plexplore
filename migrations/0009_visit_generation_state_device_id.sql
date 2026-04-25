CREATE TABLE IF NOT EXISTS visit_generation_state_new (
    device_id INTEGER PRIMARY KEY,
    last_processed_seq INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
);

INSERT OR IGNORE INTO visit_generation_state_new(device_id, last_processed_seq, updated_at)
SELECT d.id, s.last_processed_seq, s.updated_at
FROM visit_generation_state s
JOIN devices d ON d.name = s.device_name;

DROP TABLE IF EXISTS visit_generation_state;
ALTER TABLE visit_generation_state_new RENAME TO visit_generation_state;

CREATE INDEX IF NOT EXISTS idx_visit_generation_state_updated_at
ON visit_generation_state(updated_at);
