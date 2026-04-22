CREATE TABLE IF NOT EXISTS visit_place_cache (
    provider TEXT NOT NULL,
    lat_key TEXT NOT NULL,
    lon_key TEXT NOT NULL,
    label TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY(provider, lat_key, lon_key)
);

CREATE INDEX IF NOT EXISTS idx_visit_place_cache_updated_at
ON visit_place_cache(updated_at);
