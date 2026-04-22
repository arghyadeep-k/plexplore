-- Initial schema for tracker service.
-- Keep types and columns simple; evolve via additive migrations.

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL DEFAULT '',
    email TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    source_type TEXT NOT NULL,
    api_key TEXT NOT NULL UNIQUE,
    last_seen_at TEXT,
    last_seq_received INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS raw_points (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seq INTEGER NOT NULL UNIQUE,
    user_id INTEGER NOT NULL,
    device_id INTEGER NOT NULL,
    source_type TEXT NOT NULL,
    timestamp_utc TEXT NOT NULL,
    lat REAL NOT NULL,
    lon REAL NOT NULL,
    altitude REAL,
    accuracy REAL,
    speed REAL,
    heading REAL,
    battery REAL,
    motion_type TEXT,
    raw_payload_json TEXT,
    ingest_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

-- Optional lightweight derived table for query-friendly access patterns.
CREATE TABLE IF NOT EXISTS points (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    raw_point_id INTEGER NOT NULL UNIQUE,
    user_id INTEGER NOT NULL,
    device_id INTEGER NOT NULL,
    timestamp_utc TEXT NOT NULL,
    lat REAL NOT NULL,
    lon REAL NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    FOREIGN KEY (raw_point_id) REFERENCES raw_points(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_devices_user_id ON devices(user_id);
CREATE INDEX IF NOT EXISTS idx_devices_source_type ON devices(source_type);

CREATE INDEX IF NOT EXISTS idx_raw_points_device_time ON raw_points(device_id, timestamp_utc);
CREATE INDEX IF NOT EXISTS idx_raw_points_user_time ON raw_points(user_id, timestamp_utc);
CREATE INDEX IF NOT EXISTS idx_raw_points_created_at ON raw_points(created_at);

CREATE INDEX IF NOT EXISTS idx_points_device_time ON points(device_id, timestamp_utc);
CREATE INDEX IF NOT EXISTS idx_points_user_time ON points(user_id, timestamp_utc);
