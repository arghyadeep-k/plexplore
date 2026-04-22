package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrDeviceNotFound = errors.New("device not found")

// Device is the minimal device record used for management and API-key auth.
type Device struct {
	ID         int64
	UserID     int64
	Name       string
	SourceType string
	APIKey     string
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

	res, err := s.db.ExecContext(ctx, `
INSERT INTO devices(user_id, name, source_type, api_key, last_seq_received)
VALUES (?, ?, ?, ?, 0);
`, userID, name, sourceType, apiKey)
	if err != nil {
		return Device{}, fmt.Errorf("create device: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return Device{}, fmt.Errorf("device last insert id: %w", err)
	}

	return Device{
		ID:         id,
		UserID:     userID,
		Name:       name,
		SourceType: sourceType,
		APIKey:     apiKey,
	}, nil
}

func (s *SQLiteStore) ListDevices(ctx context.Context) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, name, source_type, api_key
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
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.SourceType, &d.APIKey); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
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

	var d Device
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, name, source_type, api_key
FROM devices
WHERE api_key = ?;
`, key).Scan(&d.ID, &d.UserID, &d.Name, &d.SourceType, &d.APIKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Device{}, ErrDeviceNotFound
		}
		return Device{}, fmt.Errorf("lookup device by api key: %w", err)
	}
	return d, nil
}
