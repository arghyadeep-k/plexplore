package store

import (
	"context"
	"errors"
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
	if loaded.CreatedAt.IsZero() || loaded.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps on loaded device, got %+v", loaded)
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
	if rotated.APIKey != "new-key" {
		t.Fatalf("expected new-key after rotation, got %q", rotated.APIKey)
	}

	if _, err := s.GetDeviceByAPIKey(ctx, "old-key"); !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("expected old key lookup to fail, got %v", err)
	}
	if _, err := s.GetDeviceByAPIKey(ctx, "new-key"); err != nil {
		t.Fatalf("expected new key lookup to succeed, got %v", err)
	}
}
