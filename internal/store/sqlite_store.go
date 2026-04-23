package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"plexplore/internal/ingest"
)

const storePiPragmasSQL = `
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA wal_autocheckpoint = 1000;
PRAGMA busy_timeout = 5000;
PRAGMA cache_size = -4096;
PRAGMA temp_store = MEMORY;
`

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	if _, err := db.Exec(storePiPragmasSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply sqlite pragmas: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// InsertSpoolBatch inserts spool records in one transaction and returns the
// highest sequence number successfully committed.
//
// Idempotency is enforced by raw_points.ingest_hash uniqueness using
// INSERT ... ON CONFLICT(ingest_hash) DO NOTHING.
func (s *SQLiteStore) InsertSpoolBatch(records []ingest.SpoolRecord) (uint64, error) {
	if len(records) == 0 {
		return 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	defaultUserID, err := ensureDefaultUser(tx)
	if err != nil {
		return 0, err
	}

	rawStmt, err := tx.Prepare(`
INSERT INTO raw_points(
    seq, user_id, device_id, source_type, timestamp_utc, lat, lon,
    altitude, accuracy, speed, heading, battery, motion_type,
    raw_payload_json, ingest_hash, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(ingest_hash) DO NOTHING;
`)
	if err != nil {
		return 0, fmt.Errorf("prepare raw_points insert: %w", err)
	}
	defer rawStmt.Close()

	pointStmt, err := tx.Prepare(`
INSERT INTO points(raw_point_id, user_id, device_id, timestamp_utc, lat, lon, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(raw_point_id) DO NOTHING;
`)
	if err != nil {
		return 0, fmt.Errorf("prepare points insert: %w", err)
	}
	defer pointStmt.Close()

	deviceUpdateStmt, err := tx.Prepare(`
UPDATE devices
SET
    last_seen_at = ?,
    updated_at = ?,
    last_seq_received = CASE
        WHEN last_seq_received < ? THEN ?
        ELSE last_seq_received
    END
WHERE id = ?;
`)
	if err != nil {
		return 0, fmt.Errorf("prepare device update: %w", err)
	}
	defer deviceUpdateStmt.Close()

	maxSeqCommitted := uint64(0)
	deviceLastSeen := make(map[int64]time.Time)
	deviceLastSeq := make(map[int64]uint64)

	for _, record := range records {
		userID := defaultUserID
		if parsedUserID, ok := parsePointUserID(record.Point.UserID); ok {
			userID = parsedUserID
		}

		deviceName := normalizedDeviceName(record.DeviceID)
		sourceType := normalizedSourceType(record.Point.SourceType)
		deviceID, err := ensureDevice(tx, userID, deviceName, sourceType)
		if err != nil {
			return 0, err
		}

		ts := normalizedTimestamp(record.Point.TimestampUTC, record.ReceivedAt)
		createdAt := time.Now().UTC().Format(time.RFC3339Nano)
		payload := nullableRawPayload(record.Point.RawPayload)

		res, err := rawStmt.Exec(
			record.Seq,
			userID,
			deviceID,
			sourceType,
			ts,
			record.Point.Lat,
			record.Point.Lon,
			record.Point.Altitude,
			record.Point.Accuracy,
			record.Point.Speed,
			record.Point.Heading,
			record.Point.Battery,
			record.Point.MotionType,
			payload,
			record.Point.IngestHash,
			createdAt,
		)
		if err != nil {
			return 0, fmt.Errorf("insert raw_point seq=%d: %w", record.Seq, err)
		}

		rows, err := res.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("rows affected raw_point seq=%d: %w", record.Seq, err)
		}

		if rows > 0 {
			rawPointID, err := res.LastInsertId()
			if err != nil {
				return 0, fmt.Errorf("last insert id raw_point seq=%d: %w", record.Seq, err)
			}

			if _, err := pointStmt.Exec(
				rawPointID,
				userID,
				deviceID,
				ts,
				record.Point.Lat,
				record.Point.Lon,
				createdAt,
			); err != nil {
				return 0, fmt.Errorf("insert point seq=%d: %w", record.Seq, err)
			}
		}

		if record.Seq > maxSeqCommitted {
			maxSeqCommitted = record.Seq
		}
		if tsTime, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			if existing, ok := deviceLastSeen[deviceID]; !ok || tsTime.After(existing) {
				deviceLastSeen[deviceID] = tsTime
			}
		}
		if record.Seq > deviceLastSeq[deviceID] {
			deviceLastSeq[deviceID] = record.Seq
		}
	}

	for deviceID, seq := range deviceLastSeq {
		seenAt := deviceLastSeen[deviceID].UTC().Format(time.RFC3339Nano)
		if _, err := deviceUpdateStmt.Exec(seenAt, seenAt, seq, seq, deviceID); err != nil {
			return 0, fmt.Errorf("update device state id=%d: %w", deviceID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return maxSeqCommitted, nil
}

func ensureDefaultUser(tx *sql.Tx) (int64, error) {
	if _, err := tx.Exec(`
INSERT INTO users(id, name)
VALUES(1, 'default')
ON CONFLICT(id) DO NOTHING;
`); err != nil {
		return 0, fmt.Errorf("ensure default user: %w", err)
	}
	return 1, nil
}

func parsePointUserID(raw string) (int64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func ensureDevice(tx *sql.Tx, userID int64, deviceName, sourceType string) (int64, error) {
	var existingID int64
	err := tx.QueryRow(`
SELECT id
FROM devices
WHERE user_id = ? AND name = ?
ORDER BY id ASC
LIMIT 1;
`, userID, deviceName).Scan(&existingID)
	if err == nil {
		return existingID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("lookup device %q for user %d: %w", deviceName, userID, err)
	}

	apiKey := fmt.Sprintf("auto:%d:%s", userID, deviceName)
	nowUTC := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.Exec(`
INSERT INTO devices(user_id, name, source_type, api_key, last_seq_received, updated_at)
VALUES (?, ?, ?, ?, 0, ?)
ON CONFLICT(api_key) DO NOTHING;
`, userID, deviceName, sourceType, apiKey, nowUTC); err != nil {
		return 0, fmt.Errorf("ensure device %q: %w", deviceName, err)
	}

	var id int64
	if err := tx.QueryRow(`SELECT id FROM devices WHERE user_id = ? AND name = ? ORDER BY id ASC LIMIT 1;`, userID, deviceName).Scan(&id); err != nil {
		return 0, fmt.Errorf("select device id %q: %w", deviceName, err)
	}
	return id, nil
}

func normalizedDeviceName(deviceID string) string {
	value := strings.TrimSpace(deviceID)
	if value == "" {
		return "unknown-device"
	}
	return value
}

func normalizedSourceType(sourceType string) string {
	value := strings.TrimSpace(sourceType)
	if value == "" {
		return "unknown"
	}
	return value
}

func normalizedTimestamp(ts time.Time, fallback time.Time) string {
	if !ts.IsZero() {
		return ts.UTC().Format(time.RFC3339Nano)
	}
	if !fallback.IsZero() {
		return fallback.UTC().Format(time.RFC3339Nano)
	}
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func nullableRawPayload(payload []byte) *string {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
