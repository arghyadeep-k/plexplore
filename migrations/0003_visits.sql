CREATE TABLE IF NOT EXISTS visits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER NOT NULL,
    start_at TEXT NOT NULL,
    end_at TEXT NOT NULL,
    centroid_lat REAL NOT NULL,
    centroid_lon REAL NOT NULL,
    point_count INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_visits_device_start
ON visits(device_id, start_at);
