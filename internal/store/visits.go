package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"plexplore/internal/visits"
)

// Visit is the persisted visit projection.
type Visit struct {
	ID          int64
	DeviceRowID int64
	DeviceID    string
	StartAt     time.Time
	EndAt       time.Time
	CentroidLat float64
	CentroidLon float64
	PointCount  int
}

// RebuildVisitsForDeviceID detects visits from stored points for one stable
// device row id and rewrites that device's visit rows deterministically.
func (s *SQLiteStore) RebuildVisitsForDeviceID(ctx context.Context, deviceID int64, cfg visits.Config) (int, error) {
	return s.RebuildVisitsForDeviceRange(ctx, deviceID, nil, nil, cfg)
}

// RebuildVisitsForDeviceRange detects visits for a bounded device-id/time window
// and rewrites only visit rows whose start_at falls inside the same window.
func (s *SQLiteStore) RebuildVisitsForDeviceRange(ctx context.Context, deviceID int64, fromUTC, toUTC *time.Time, cfg visits.Config) (int, error) {
	if deviceID <= 0 {
		return 0, fmt.Errorf("device_id is required")
	}

	points, err := s.listPointsForVisitDetectionByDeviceID(ctx, deviceID, fromUTC, toUTC)
	if err != nil {
		return 0, fmt.Errorf("list points for visit detection: %w", err)
	}

	detectPoints := make([]visits.Point, 0, len(points))
	for _, p := range points {
		detectPoints = append(detectPoints, visits.Point{
			TimestampUTC: p.TimestampUTC.UTC(),
			Lat:          p.Lat,
			Lon:          p.Lon,
		})
	}
	detected := visits.Detect(detectPoints, cfg)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin visits tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	deleteSQL := `DELETE FROM visits WHERE device_id = ?`
	args := []any{deviceID}
	if fromUTC != nil {
		deleteSQL += ` AND start_at >= ?`
		args = append(args, fromUTC.UTC().Format(time.RFC3339Nano))
	}
	if toUTC != nil {
		deleteSQL += ` AND start_at <= ?`
		args = append(args, toUTC.UTC().Format(time.RFC3339Nano))
	}
	deleteSQL += ";"
	if _, err := tx.ExecContext(ctx, deleteSQL, args...); err != nil {
		return 0, fmt.Errorf("delete existing visits in range: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO visits(device_id, start_at, end_at, centroid_lat, centroid_lon, point_count)
VALUES (?, ?, ?, ?, ?, ?);
`)
	if err != nil {
		return 0, fmt.Errorf("prepare visits insert: %w", err)
	}
	defer stmt.Close()

	for _, visit := range detected {
		if _, err := stmt.ExecContext(
			ctx,
			deviceID,
			visit.StartAt.UTC().Format(time.RFC3339Nano),
			visit.EndAt.UTC().Format(time.RFC3339Nano),
			visit.CentroidLat,
			visit.CentroidLon,
			visit.PointCount,
		); err != nil {
			return 0, fmt.Errorf("insert visit: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit visits tx: %w", err)
	}
	return len(detected), nil
}

func (s *SQLiteStore) listPointsForVisitDetectionByDeviceID(ctx context.Context, deviceID int64, fromUTC, toUTC *time.Time) ([]RecentPoint, error) {
	baseSQL := `
SELECT rp.seq, rp.device_id, rp.source_type, rp.timestamp_utc, rp.lat, rp.lon
FROM raw_points rp
`
	whereParts := []string{"rp.device_id = ?"}
	args := []any{deviceID}

	if fromUTC != nil {
		whereParts = append(whereParts, "rp.timestamp_utc >= ?")
		args = append(args, fromUTC.UTC().Format(time.RFC3339Nano))
	}
	if toUTC != nil {
		whereParts = append(whereParts, "rp.timestamp_utc <= ?")
		args = append(args, toUTC.UTC().Format(time.RFC3339Nano))
	}
	baseSQL += "WHERE " + strings.Join(whereParts, " AND ") + "\n"
	baseSQL += "ORDER BY rp.timestamp_utc ASC, rp.seq ASC;"

	rows, err := s.db.QueryContext(ctx, baseSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("list points for visit detection query: %w", err)
	}
	defer rows.Close()

	out := make([]RecentPoint, 0)
	for rows.Next() {
		var point RecentPoint
		var timestampRaw string
		if err := rows.Scan(
			&point.Seq,
			&point.DeviceID,
			&point.SourceType,
			&timestampRaw,
			&point.Lat,
			&point.Lon,
		); err != nil {
			return nil, fmt.Errorf("scan visit detection point: %w", err)
		}
		parsed, parseErr := parseDBTime(timestampRaw)
		if parseErr != nil {
			return nil, fmt.Errorf("parse visit detection timestamp: %w", parseErr)
		}
		point.TimestampUTC = parsed
		out = append(out, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate visit detection points: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) ListVisits(ctx context.Context, userID int64, deviceID *int64, fromUTC, toUTC *time.Time, limit int) ([]Visit, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("user_id is required")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 5000 {
		limit = 5000
	}

	whereParts := []string{"d.user_id = ?"}
	args := []any{userID}
	if deviceID != nil {
		whereParts = append(whereParts, "v.device_id = ?")
		args = append(args, *deviceID)
	}
	if fromUTC != nil {
		whereParts = append(whereParts, "v.start_at >= ?")
		args = append(args, fromUTC.UTC().Format(time.RFC3339Nano))
	}
	if toUTC != nil {
		whereParts = append(whereParts, "v.start_at <= ?")
		args = append(args, toUTC.UTC().Format(time.RFC3339Nano))
	}

	query := `
SELECT v.id, v.device_id, d.name, v.start_at, v.end_at, v.centroid_lat, v.centroid_lon, v.point_count
FROM visits v
JOIN devices d ON d.id = v.device_id
`
	query += "WHERE " + strings.Join(whereParts, " AND ") + "\n"
	query += "ORDER BY v.start_at ASC, v.id ASC\nLIMIT ?;"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list visits: %w", err)
	}
	defer rows.Close()

	out := make([]Visit, 0)
	for rows.Next() {
		var v Visit
		var startRaw string
		var endRaw string
		if err := rows.Scan(
			&v.ID,
			&v.DeviceRowID,
			&v.DeviceID,
			&startRaw,
			&endRaw,
			&v.CentroidLat,
			&v.CentroidLon,
			&v.PointCount,
		); err != nil {
			return nil, fmt.Errorf("scan visit: %w", err)
		}
		if v.StartAt, err = parseDBTime(startRaw); err != nil {
			return nil, fmt.Errorf("parse visit start_at: %w", err)
		}
		if v.EndAt, err = parseDBTime(endRaw); err != nil {
			return nil, fmt.Errorf("parse visit end_at: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate visits: %w", err)
	}
	return out, nil
}
