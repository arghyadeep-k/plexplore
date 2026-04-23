package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// RecentPoint is a compact debugging projection of stored location points.
type RecentPoint struct {
	Seq          uint64
	UserID       int64
	DeviceID     string
	SourceType   string
	TimestampUTC time.Time
	Lat          float64
	Lon          float64
}

// ExportPointFilter controls optional filtering for export endpoints.
type ExportPointFilter struct {
	DeviceID string
	FromUTC  *time.Time
	ToUTC    *time.Time
}

func (s *SQLiteStore) ListRecentPoints(ctx context.Context, deviceID string, limit int) ([]RecentPoint, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	device := strings.TrimSpace(deviceID)
	baseSQL := `
SELECT rp.seq, rp.user_id, d.name, rp.source_type, rp.timestamp_utc, rp.lat, rp.lon
FROM raw_points rp
JOIN devices d ON d.id = rp.device_id
`

	args := make([]any, 0, 2)
	if device != "" {
		baseSQL += "WHERE d.name = ?\n"
		args = append(args, device)
	}
	baseSQL += "ORDER BY rp.timestamp_utc DESC, rp.seq DESC\nLIMIT ?;"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, baseSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("list recent points: %w", err)
	}
	defer rows.Close()

	out := make([]RecentPoint, 0)
	for rows.Next() {
		var point RecentPoint
		var timestampRaw string
		if err := rows.Scan(
			&point.Seq,
			&point.UserID,
			&point.DeviceID,
			&point.SourceType,
			&timestampRaw,
			&point.Lat,
			&point.Lon,
		); err != nil {
			return nil, fmt.Errorf("scan recent point: %w", err)
		}
		parsed, parseErr := parseDBTime(timestampRaw)
		if parseErr != nil {
			return nil, fmt.Errorf("parse recent point timestamp: %w", parseErr)
		}
		point.TimestampUTC = parsed
		out = append(out, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent points: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) ListPoints(ctx context.Context, filter ExportPointFilter, limit int) ([]RecentPoint, error) {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}

	baseSQL := `
SELECT rp.seq, rp.user_id, d.name, rp.source_type, rp.timestamp_utc, rp.lat, rp.lon
FROM raw_points rp
JOIN devices d ON d.id = rp.device_id
`

	whereParts := make([]string, 0, 3)
	args := make([]any, 0, 4)

	device := strings.TrimSpace(filter.DeviceID)
	if device != "" {
		whereParts = append(whereParts, "d.name = ?")
		args = append(args, device)
	}
	if filter.FromUTC != nil {
		whereParts = append(whereParts, "rp.timestamp_utc >= ?")
		args = append(args, filter.FromUTC.UTC().Format(time.RFC3339Nano))
	}
	if filter.ToUTC != nil {
		whereParts = append(whereParts, "rp.timestamp_utc <= ?")
		args = append(args, filter.ToUTC.UTC().Format(time.RFC3339Nano))
	}

	if len(whereParts) > 0 {
		baseSQL += "WHERE " + strings.Join(whereParts, " AND ") + "\n"
	}
	baseSQL += "ORDER BY rp.timestamp_utc ASC, rp.seq ASC\nLIMIT ?;"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, baseSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("list points: %w", err)
	}
	defer rows.Close()

	out := make([]RecentPoint, 0)
	for rows.Next() {
		var point RecentPoint
		var timestampRaw string
		if err := rows.Scan(
			&point.Seq,
			&point.UserID,
			&point.DeviceID,
			&point.SourceType,
			&timestampRaw,
			&point.Lat,
			&point.Lon,
		); err != nil {
			return nil, fmt.Errorf("scan point: %w", err)
		}
		parsed, parseErr := parseDBTime(timestampRaw)
		if parseErr != nil {
			return nil, fmt.Errorf("parse point timestamp: %w", parseErr)
		}
		point.TimestampUTC = parsed
		out = append(out, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate points: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) ListPointsForExport(ctx context.Context, filter ExportPointFilter) ([]RecentPoint, error) {
	baseSQL := `
SELECT rp.seq, rp.user_id, d.name, rp.source_type, rp.timestamp_utc, rp.lat, rp.lon
FROM raw_points rp
JOIN devices d ON d.id = rp.device_id
`

	whereParts := make([]string, 0, 3)
	args := make([]any, 0, 3)

	device := strings.TrimSpace(filter.DeviceID)
	if device != "" {
		whereParts = append(whereParts, "d.name = ?")
		args = append(args, device)
	}
	if filter.FromUTC != nil {
		whereParts = append(whereParts, "rp.timestamp_utc >= ?")
		args = append(args, filter.FromUTC.UTC().Format(time.RFC3339Nano))
	}
	if filter.ToUTC != nil {
		whereParts = append(whereParts, "rp.timestamp_utc <= ?")
		args = append(args, filter.ToUTC.UTC().Format(time.RFC3339Nano))
	}

	if len(whereParts) > 0 {
		baseSQL += "WHERE " + strings.Join(whereParts, " AND ") + "\n"
	}
	baseSQL += "ORDER BY rp.timestamp_utc ASC, rp.seq ASC;"

	rows, err := s.db.QueryContext(ctx, baseSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("list export points: %w", err)
	}
	defer rows.Close()

	out := make([]RecentPoint, 0)
	for rows.Next() {
		var point RecentPoint
		var timestampRaw string
		if err := rows.Scan(
			&point.Seq,
			&point.UserID,
			&point.DeviceID,
			&point.SourceType,
			&timestampRaw,
			&point.Lat,
			&point.Lon,
		); err != nil {
			return nil, fmt.Errorf("scan export point: %w", err)
		}
		parsed, parseErr := parseDBTime(timestampRaw)
		if parseErr != nil {
			return nil, fmt.Errorf("parse export point timestamp: %w", parseErr)
		}
		point.TimestampUTC = parsed
		out = append(out, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate export points: %w", err)
	}
	return out, nil
}
