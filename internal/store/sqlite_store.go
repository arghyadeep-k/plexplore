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

	store := &SQLiteStore{db: db}
	if err := store.backfillDeviceAPIKeyHashes(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
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

	apiKeySentinel, err := generateAPIKeySentinel()
	if err != nil {
		return 0, err
	}
	nowUTC := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.Exec(`
INSERT INTO devices(user_id, name, source_type, api_key, api_key_hash, api_key_preview, last_seq_received, updated_at)
VALUES (?, ?, ?, ?, '', '', 0, ?);
`, userID, deviceName, sourceType, apiKeySentinel, nowUTC); err != nil {
		return 0, fmt.Errorf("ensure device %q: %w", deviceName, err)
	}

	var id int64
	if err := tx.QueryRow(`SELECT id FROM devices WHERE user_id = ? AND name = ? ORDER BY id ASC LIMIT 1;`, userID, deviceName).Scan(&id); err != nil {
		return 0, fmt.Errorf("select device id %q: %w", deviceName, err)
	}
	return id, nil
}

func (s *SQLiteStore) backfillDeviceAPIKeyHashes() error {
	if s == nil || s.db == nil {
		return nil
	}
	hasDevices, err := s.tableExists("devices")
	if err != nil {
		return fmt.Errorf("check devices table: %w", err)
	}
	if !hasDevices {
		return nil
	}
	hasHash, err := s.columnExists("devices", "api_key_hash")
	if err != nil {
		return fmt.Errorf("check devices.api_key_hash: %w", err)
	}
	hasPreview, err := s.columnExists("devices", "api_key_preview")
	if err != nil {
		return fmt.Errorf("check devices.api_key_preview: %w", err)
	}
	if !hasHash || !hasPreview {
		return nil
	}

	rows, err := s.db.Query(`
SELECT id, COALESCE(api_key, ''), COALESCE(api_key_hash, ''), COALESCE(api_key_preview, '')
FROM devices
ORDER BY id ASC;
`)
	if err != nil {
		return fmt.Errorf("load device keys for backfill: %w", err)
	}
	defer rows.Close()

	type pendingUpdate struct {
		id       int64
		sentinel string
		hash     string
		preview  string
	}
	updates := make([]pendingUpdate, 0)
	for rows.Next() {
		var id int64
		var apiKey string
		var apiKeyHash string
		var apiKeyPreview string
		if err := rows.Scan(&id, &apiKey, &apiKeyHash, &apiKeyPreview); err != nil {
			return fmt.Errorf("scan backfill device row: %w", err)
		}
		key := strings.TrimSpace(apiKey)
		hash := strings.TrimSpace(apiKeyHash)
		preview := strings.TrimSpace(apiKeyPreview)

		if hash == "" && key != "" {
			hash = hashDeviceAPIKey(key)
		}
		if preview == "" && key != "" {
			preview = buildAPIKeyPreview(key)
		}
		if hash == "" {
			continue
		}

		sentinel := fmt.Sprintf("hashonly:%d", id)
		if key == sentinel && apiKeyHash == hash && apiKeyPreview == preview {
			continue
		}
		updates = append(updates, pendingUpdate{
			id:       id,
			sentinel: sentinel,
			hash:     hash,
			preview:  preview,
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate backfill device rows: %w", err)
	}
	if len(updates) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin backfill tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.Prepare(`
UPDATE devices
SET api_key = ?, api_key_hash = ?, api_key_preview = ?
WHERE id = ?;
`)
	if err != nil {
		return fmt.Errorf("prepare backfill update: %w", err)
	}
	defer stmt.Close()

	for _, update := range updates {
		if _, err := stmt.Exec(update.sentinel, update.hash, update.preview, update.id); err != nil {
			return fmt.Errorf("backfill device id=%d: %w", update.id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit backfill tx: %w", err)
	}
	return nil
}

func (s *SQLiteStore) tableExists(tableName string) (bool, error) {
	var name string
	err := s.db.QueryRow(`
SELECT name
FROM sqlite_master
WHERE type='table' AND name = ?
LIMIT 1;
`, tableName).Scan(&name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(name) != "", nil
}

func (s *SQLiteStore) columnExists(tableName, columnName string) (bool, error) {
	rows, err := s.db.Query(`PRAGMA table_info(` + tableName + `);`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(columnName)) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
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
