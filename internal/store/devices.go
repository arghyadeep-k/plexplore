package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrDeviceNotFound = errors.New("device not found")

// Device is the minimal device record used for management and API-key auth.
type Device struct {
	ID            int64
	UserID        int64
	Name          string
	SourceType    string
	APIKey        string
	APIKeyHash    string
	APIKeyPreview string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastSeenAt    *time.Time
}

// CreateDeviceParams contains input fields for manual device registration.
type CreateDeviceParams struct {
	UserID     int64
	Name       string
	SourceType string
	APIKey     string
}

func (s *SQLiteStore) CreateDevice(ctx context.Context, params CreateDeviceParams) (Device, error) {
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return Device{}, fmt.Errorf("device name is required")
	}
	sourceType := strings.TrimSpace(params.SourceType)
	if sourceType == "" {
		return Device{}, fmt.Errorf("source type is required")
	}
	apiKey := strings.TrimSpace(params.APIKey)
	if apiKey == "" {
		return Device{}, fmt.Errorf("api key is required")
	}
	apiKeyHash := hashDeviceAPIKey(apiKey)
	if apiKeyHash == "" {
		return Device{}, fmt.Errorf("api key is required")
	}
	apiKeyPreview := buildAPIKeyPreview(apiKey)
	apiKeySentinel, err := generateAPIKeySentinel()
	if err != nil {
		return Device{}, err
	}

	userID := params.UserID
	if userID <= 0 {
		userID = 1
	}
	if userID == 1 {
		if _, err := s.db.ExecContext(ctx, `
INSERT INTO users(id, name)
VALUES(1, 'default')
ON CONFLICT(id) DO NOTHING;
`); err != nil {
			return Device{}, fmt.Errorf("ensure default user: %w", err)
		}
	}

	nowUTC := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `
INSERT INTO devices(user_id, name, source_type, api_key, api_key_hash, api_key_preview, last_seq_received, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 0, ?);
`, userID, name, sourceType, apiKeySentinel, apiKeyHash, apiKeyPreview, nowUTC)
	if err != nil {
		return Device{}, fmt.Errorf("create device: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return Device{}, fmt.Errorf("device last insert id: %w", err)
	}

	return s.GetDeviceByID(ctx, id)
}

func (s *SQLiteStore) ListDevices(ctx context.Context) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, name, source_type, api_key_hash, api_key_preview, created_at, updated_at, last_seen_at
FROM devices
ORDER BY id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	out := make([]Device, 0)
	for rows.Next() {
		var d Device
		var createdAtRaw string
		var updatedAtRaw string
		var lastSeenAtRaw sql.NullString
		if err := rows.Scan(
			&d.ID,
			&d.UserID,
			&d.Name,
			&d.SourceType,
			&d.APIKeyHash,
			&d.APIKeyPreview,
			&createdAtRaw,
			&updatedAtRaw,
			&lastSeenAtRaw,
		); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		if d.CreatedAt, err = parseDBTime(createdAtRaw); err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		if d.UpdatedAt, err = parseDBTime(updatedAtRaw); err != nil {
			return nil, fmt.Errorf("parse updated_at: %w", err)
		}
		if lastSeenAtRaw.Valid && strings.TrimSpace(lastSeenAtRaw.String) != "" {
			parsed, parseErr := parseDBTime(lastSeenAtRaw.String)
			if parseErr != nil {
				return nil, fmt.Errorf("parse last_seen_at: %w", parseErr)
			}
			d.LastSeenAt = &parsed
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate devices: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) GetDeviceByAPIKey(ctx context.Context, apiKey string) (Device, error) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return Device{}, ErrDeviceNotFound
	}

	var id int64
	apiKeyHash := hashDeviceAPIKey(key)
	if apiKeyHash == "" {
		return Device{}, ErrDeviceNotFound
	}
	err := s.db.QueryRowContext(ctx, `
SELECT id
FROM devices
WHERE api_key_hash = ?;
`, apiKeyHash).Scan(
		&id,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Device{}, ErrDeviceNotFound
		}
		return Device{}, fmt.Errorf("lookup device by api key: %w", err)
	}
	return s.GetDeviceByID(ctx, id)
}

func (s *SQLiteStore) GetDeviceByID(ctx context.Context, id int64) (Device, error) {
	if id <= 0 {
		return Device{}, ErrDeviceNotFound
	}

	var d Device
	var createdAtRaw string
	var updatedAtRaw string
	var lastSeenAtRaw sql.NullString
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, name, source_type, api_key_hash, api_key_preview, created_at, updated_at, last_seen_at
FROM devices
WHERE id = ?;
`, id).Scan(
		&d.ID,
		&d.UserID,
		&d.Name,
		&d.SourceType,
		&d.APIKeyHash,
		&d.APIKeyPreview,
		&createdAtRaw,
		&updatedAtRaw,
		&lastSeenAtRaw,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Device{}, ErrDeviceNotFound
		}
		return Device{}, fmt.Errorf("lookup device by id: %w", err)
	}

	parsedCreatedAt, err := parseDBTime(createdAtRaw)
	if err != nil {
		return Device{}, fmt.Errorf("parse created_at: %w", err)
	}
	parsedUpdatedAt, err := parseDBTime(updatedAtRaw)
	if err != nil {
		return Device{}, fmt.Errorf("parse updated_at: %w", err)
	}
	d.CreatedAt = parsedCreatedAt
	d.UpdatedAt = parsedUpdatedAt

	if lastSeenAtRaw.Valid && strings.TrimSpace(lastSeenAtRaw.String) != "" {
		parsedLastSeenAt, parseErr := parseDBTime(lastSeenAtRaw.String)
		if parseErr != nil {
			return Device{}, fmt.Errorf("parse last_seen_at: %w", parseErr)
		}
		d.LastSeenAt = &parsedLastSeenAt
	}

	return d, nil
}

func (s *SQLiteStore) RotateDeviceAPIKey(ctx context.Context, id int64, newAPIKey string) (Device, error) {
	if id <= 0 {
		return Device{}, ErrDeviceNotFound
	}
	key := strings.TrimSpace(newAPIKey)
	if key == "" {
		return Device{}, fmt.Errorf("new api key is required")
	}
	apiKeyHash := hashDeviceAPIKey(key)
	if apiKeyHash == "" {
		return Device{}, fmt.Errorf("new api key is required")
	}
	apiKeyPreview := buildAPIKeyPreview(key)
	apiKeySentinel, err := generateAPIKeySentinel()
	if err != nil {
		return Device{}, err
	}

	nowUTC := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `
UPDATE devices
SET api_key = ?, api_key_hash = ?, api_key_preview = ?, updated_at = ?
WHERE id = ?;
`, apiKeySentinel, apiKeyHash, apiKeyPreview, nowUTC, id)
	if err != nil {
		return Device{}, fmt.Errorf("rotate device api key: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return Device{}, fmt.Errorf("rotate device api key rows affected: %w", err)
	}
	if rows == 0 {
		return Device{}, ErrDeviceNotFound
	}

	return s.GetDeviceByID(ctx, id)
}

func parseDBTime(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}
