package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *SQLiteStore) GetVisitPlaceLabel(ctx context.Context, provider, latKey, lonKey string) (string, bool, error) {
	p := strings.TrimSpace(provider)
	lat := strings.TrimSpace(latKey)
	lon := strings.TrimSpace(lonKey)
	if p == "" || lat == "" || lon == "" {
		return "", false, nil
	}

	var label string
	err := s.db.QueryRowContext(ctx, `
SELECT label
FROM visit_place_cache
WHERE provider = ? AND lat_key = ? AND lon_key = ?
LIMIT 1;
`, p, lat, lon).Scan(&label)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("query visit place cache: %w", err)
	}
	return label, true, nil
}

func (s *SQLiteStore) UpsertVisitPlaceLabel(ctx context.Context, provider, latKey, lonKey, label string) error {
	p := strings.TrimSpace(provider)
	lat := strings.TrimSpace(latKey)
	lon := strings.TrimSpace(lonKey)
	value := strings.TrimSpace(label)
	if p == "" || lat == "" || lon == "" || value == "" {
		return nil
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO visit_place_cache(provider, lat_key, lon_key, label, updated_at)
VALUES(?, ?, ?, ?, ?)
ON CONFLICT(provider, lat_key, lon_key)
DO UPDATE SET
    label = excluded.label,
    updated_at = excluded.updated_at;
`, p, lat, lon, value, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("upsert visit place cache: %w", err)
	}
	return nil
}
