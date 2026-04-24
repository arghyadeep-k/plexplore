package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSQLiteStore_CreateAndLookupDeviceByAPIKey(t *testing.T) {
	s := openStoreWithSchema(t)

	created, err := s.CreateDevice(context.Background(), CreateDeviceParams{
		UserID:     1,
		Name:       "phone-main",
		SourceType: "owntracks",
		APIKey:     "test-key-1",
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected non-zero device id")
	}

	loaded, err := s.GetDeviceByAPIKey(context.Background(), "test-key-1")
	if err != nil {
		t.Fatalf("GetDeviceByAPIKey failed: %v", err)
	}
	if loaded.ID != created.ID {
		t.Fatalf("expected same device id, got %d vs %d", loaded.ID, created.ID)
	}
	if loaded.Name != "phone-main" {
		t.Fatalf("expected device name phone-main, got %q", loaded.Name)
	}
	if loaded.APIKey != "" {
		t.Fatalf("expected no plaintext api key on loaded device, got %q", loaded.APIKey)
	}
	if strings.TrimSpace(loaded.APIKeyHash) == "" {
		t.Fatalf("expected api key hash to be present, got %+v", loaded)
	}
	if strings.TrimSpace(loaded.APIKeyPreview) == "" {
		t.Fatalf("expected api key preview to be present, got %+v", loaded)
	}
	if loaded.CreatedAt.IsZero() || loaded.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps on loaded device, got %+v", loaded)
	}

	var storedRaw, storedHash, storedPreview string
	if err := s.db.QueryRow(`
SELECT COALESCE(api_key, ''), COALESCE(api_key_hash, ''), COALESCE(api_key_preview, '')
FROM devices
WHERE id = ?;
`, loaded.ID).Scan(&storedRaw, &storedHash, &storedPreview); err != nil {
		t.Fatalf("query stored device key fields failed: %v", err)
	}
	if strings.TrimSpace(storedHash) == "" {
		t.Fatalf("expected stored hash to be non-empty")
	}
	if strings.TrimSpace(storedPreview) == "" {
		t.Fatalf("expected stored preview to be non-empty")
	}
	if strings.TrimSpace(storedRaw) == "test-key-1" {
		t.Fatalf("expected plaintext api key not to be stored at rest")
	}
}

func TestSQLiteStore_ListDevices(t *testing.T) {
	s := openStoreWithSchema(t)
	ctx := context.Background()

	_, err := s.CreateDevice(ctx, CreateDeviceParams{
		Name:       "d1",
		SourceType: "owntracks",
		APIKey:     "k1",
	})
	if err != nil {
		t.Fatalf("CreateDevice d1 failed: %v", err)
	}
	_, err = s.CreateDevice(ctx, CreateDeviceParams{
		Name:       "d2",
		SourceType: "overland",
		APIKey:     "k2",
	})
	if err != nil {
		t.Fatalf("CreateDevice d2 failed: %v", err)
	}

	devices, err := s.ListDevices(ctx)
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
}

func TestSQLiteStore_GetDeviceByAPIKey_NotFound(t *testing.T) {
	s := openStoreWithSchema(t)

	_, err := s.GetDeviceByAPIKey(context.Background(), "missing")
	if !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestSQLiteStore_GetDeviceByID_AndRotateAPIKey(t *testing.T) {
	s := openStoreWithSchema(t)
	ctx := context.Background()

	created, err := s.CreateDevice(ctx, CreateDeviceParams{
		Name:       "d1",
		SourceType: "owntracks",
		APIKey:     "old-key",
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	loaded, err := s.GetDeviceByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetDeviceByID failed: %v", err)
	}
	if loaded.ID != created.ID {
		t.Fatalf("expected loaded id %d, got %d", created.ID, loaded.ID)
	}

	rotated, err := s.RotateDeviceAPIKey(ctx, created.ID, "new-key")
	if err != nil {
		t.Fatalf("RotateDeviceAPIKey failed: %v", err)
	}
	if rotated.APIKey != "" {
		t.Fatalf("expected rotated device to hide plaintext api key, got %q", rotated.APIKey)
	}
	if strings.TrimSpace(rotated.APIKeyHash) == "" {
		t.Fatalf("expected rotated api key hash to be present, got %+v", rotated)
	}

	if _, err := s.GetDeviceByAPIKey(ctx, "old-key"); !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("expected old key lookup to fail, got %v", err)
	}
	if _, err := s.GetDeviceByAPIKey(ctx, "new-key"); err != nil {
		t.Fatalf("expected new key lookup to succeed, got %v", err)
	}

	var storedRaw string
	if err := s.db.QueryRow(`SELECT COALESCE(api_key, '') FROM devices WHERE id = ?;`, created.ID).Scan(&storedRaw); err != nil {
		t.Fatalf("query rotated device api_key field failed: %v", err)
	}
	if strings.TrimSpace(storedRaw) == "new-key" || strings.TrimSpace(storedRaw) == "old-key" {
		t.Fatalf("expected no plaintext key persisted after rotation, got %q", storedRaw)
	}
}

func TestSQLiteStore_BackfillPlaintextDeviceKeyToHash(t *testing.T) {
	s := openStoreWithSchema(t)

	if _, err := s.db.Exec(`
INSERT INTO users(id, name)
VALUES(11, 'u11')
ON CONFLICT(id) DO NOTHING;
`); err != nil {
		t.Fatalf("ensure user failed: %v", err)
	}
	if _, err := s.db.Exec(`
INSERT INTO devices(user_id, name, source_type, api_key, api_key_hash, api_key_preview, last_seq_received, updated_at)
VALUES(11, 'legacy-device', 'owntracks', 'legacy-plain-key', '', '', 0, strftime('%Y-%m-%dT%H:%M:%fZ','now'));
`); err != nil {
		t.Fatalf("insert legacy plaintext device failed: %v", err)
	}

	if err := s.backfillDeviceAPIKeyHashes(); err != nil {
		t.Fatalf("backfillDeviceAPIKeyHashes failed: %v", err)
	}

	loaded, err := s.GetDeviceByAPIKey(context.Background(), "legacy-plain-key")
	if err != nil {
		t.Fatalf("GetDeviceByAPIKey failed after backfill: %v", err)
	}
	if loaded.Name != "legacy-device" {
		t.Fatalf("expected legacy-device, got %q", loaded.Name)
	}

	var raw, hash, preview string
	if err := s.db.QueryRow(`
SELECT COALESCE(api_key, ''), COALESCE(api_key_hash, ''), COALESCE(api_key_preview, '')
FROM devices
WHERE id = ?;
`, loaded.ID).Scan(&raw, &hash, &preview); err != nil {
		t.Fatalf("query backfilled device row failed: %v", err)
	}
	if strings.TrimSpace(hash) == "" {
		t.Fatalf("expected hash to be set after backfill")
	}
	if strings.TrimSpace(preview) == "" {
		t.Fatalf("expected preview to be set after backfill")
	}
	if strings.TrimSpace(raw) == "legacy-plain-key" {
		t.Fatalf("expected plaintext key to be removed during backfill")
	}
}
