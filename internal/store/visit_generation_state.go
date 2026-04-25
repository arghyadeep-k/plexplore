package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type VisitGenerationState struct {
	DeviceID         int64
	LastProcessedSeq uint64
	UpdatedAt        time.Time
}

func (s *SQLiteStore) GetVisitGenerationState(ctx context.Context, deviceID int64) (VisitGenerationState, bool, error) {
	if deviceID <= 0 {
		return VisitGenerationState{}, false, fmt.Errorf("device_id is required")
	}

	var state VisitGenerationState
	var updatedRaw string
	err := s.db.QueryRowContext(ctx, `
SELECT device_id, last_processed_seq, updated_at
FROM visit_generation_state
WHERE device_id = ?;
`, deviceID).Scan(&state.DeviceID, &state.LastProcessedSeq, &updatedRaw)
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

func (s *SQLiteStore) UpsertVisitGenerationState(ctx context.Context, deviceID int64, lastProcessedSeq uint64) error {
	if deviceID <= 0 {
		return fmt.Errorf("device_id is required")
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO visit_generation_state(device_id, last_processed_seq, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(device_id) DO UPDATE SET
    last_processed_seq = excluded.last_processed_seq,
    updated_at = excluded.updated_at;
`, deviceID, lastProcessedSeq, updatedAt)
	if err != nil {
		return fmt.Errorf("upsert visit generation state: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetMaxPointSeqForDevice(ctx context.Context, deviceID int64) (uint64, bool, error) {
	if deviceID <= 0 {
		return 0, false, fmt.Errorf("device_id is required")
	}

	var raw sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
SELECT MAX(seq)
FROM raw_points
WHERE device_id = ?;
`, deviceID).Scan(&raw)
	if err != nil {
		return 0, false, fmt.Errorf("max point seq for device: %w", err)
	}
	if !raw.Valid || raw.Int64 <= 0 {
		return 0, false, nil
	}
	return uint64(raw.Int64), true, nil
}

func (s *SQLiteStore) GetPointTimestampForDeviceSeq(ctx context.Context, deviceID int64, seq uint64) (time.Time, bool, error) {
	if deviceID <= 0 {
		return time.Time{}, false, fmt.Errorf("device_id is required")
	}
	if seq == 0 {
		return time.Time{}, false, nil
	}

	var raw string
	err := s.db.QueryRowContext(ctx, `
SELECT timestamp_utc
FROM raw_points
WHERE device_id = ? AND seq = ?
LIMIT 1;
`, deviceID, seq).Scan(&raw)
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
