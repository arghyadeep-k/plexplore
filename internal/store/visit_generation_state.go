package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type VisitGenerationState struct {
	DeviceName       string
	LastProcessedSeq uint64
	UpdatedAt        time.Time
}

func (s *SQLiteStore) GetVisitGenerationState(ctx context.Context, deviceName string) (VisitGenerationState, bool, error) {
	name := strings.TrimSpace(deviceName)
	if name == "" {
		return VisitGenerationState{}, false, fmt.Errorf("device_name is required")
	}

	var state VisitGenerationState
	var updatedRaw string
	err := s.db.QueryRowContext(ctx, `
SELECT device_name, last_processed_seq, updated_at
FROM visit_generation_state
WHERE device_name = ?;
`, name).Scan(&state.DeviceName, &state.LastProcessedSeq, &updatedRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return VisitGenerationState{}, false, nil
		}
		return VisitGenerationState{}, false, fmt.Errorf("get visit generation state: %w", err)
	}
	updated, parseErr := parseDBTime(updatedRaw)
	if parseErr != nil {
		return VisitGenerationState{}, false, fmt.Errorf("parse visit generation updated_at: %w", parseErr)
	}
	state.UpdatedAt = updated
	return state, true, nil
}

func (s *SQLiteStore) UpsertVisitGenerationState(ctx context.Context, deviceName string, lastProcessedSeq uint64) error {
	name := strings.TrimSpace(deviceName)
	if name == "" {
		return fmt.Errorf("device_name is required")
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO visit_generation_state(device_name, last_processed_seq, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(device_name) DO UPDATE SET
    last_processed_seq = excluded.last_processed_seq,
    updated_at = excluded.updated_at;
`, name, lastProcessedSeq, updatedAt)
	if err != nil {
		return fmt.Errorf("upsert visit generation state: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetMaxPointSeqForDevice(ctx context.Context, deviceName string) (uint64, bool, error) {
	name := strings.TrimSpace(deviceName)
	if name == "" {
		return 0, false, fmt.Errorf("device_name is required")
	}

	var raw sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
SELECT MAX(rp.seq)
FROM raw_points rp
JOIN devices d ON d.id = rp.device_id
WHERE d.name = ?;
`, name).Scan(&raw)
	if err != nil {
		return 0, false, fmt.Errorf("max point seq for device: %w", err)
	}
	if !raw.Valid || raw.Int64 <= 0 {
		return 0, false, nil
	}
	return uint64(raw.Int64), true, nil
}

func (s *SQLiteStore) GetPointTimestampForDeviceSeq(ctx context.Context, deviceName string, seq uint64) (time.Time, bool, error) {
	name := strings.TrimSpace(deviceName)
	if name == "" {
		return time.Time{}, false, fmt.Errorf("device_name is required")
	}
	if seq == 0 {
		return time.Time{}, false, nil
	}

	var raw string
	err := s.db.QueryRowContext(ctx, `
SELECT rp.timestamp_utc
FROM raw_points rp
JOIN devices d ON d.id = rp.device_id
WHERE d.name = ? AND rp.seq = ?
LIMIT 1;
`, name, seq).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("point timestamp for seq: %w", err)
	}
	parsed, parseErr := parseDBTime(raw)
	if parseErr != nil {
		return time.Time{}, false, fmt.Errorf("parse point timestamp for seq: %w", parseErr)
	}
	return parsed, true, nil
}
