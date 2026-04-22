package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"plexplore/internal/store"
)

type fakeDeviceStore struct {
	nextID      int64
	devices     []store.Device
	lookupError error
}

func (f *fakeDeviceStore) CreateDevice(_ context.Context, params store.CreateDeviceParams) (store.Device, error) {
	f.nextID++
	d := store.Device{
		ID:         f.nextID,
		UserID:     params.UserID,
		Name:       params.Name,
		SourceType: params.SourceType,
		APIKey:     params.APIKey,
	}
	if d.UserID == 0 {
		d.UserID = 1
	}
	f.devices = append(f.devices, d)
	return d, nil
}

func (f *fakeDeviceStore) ListDevices(_ context.Context) ([]store.Device, error) {
	out := make([]store.Device, len(f.devices))
	copy(out, f.devices)
	return out, nil
}

func (f *fakeDeviceStore) GetDeviceByAPIKey(_ context.Context, apiKey string) (store.Device, error) {
	if f.lookupError != nil {
		return store.Device{}, f.lookupError
	}
	for _, d := range f.devices {
		if d.APIKey == apiKey {
			return d, nil
		}
	}
	return store.Device{}, store.ErrDeviceNotFound
}

func TestDevicesAPI_CreateAndList(t *testing.T) {
	ds := &fakeDeviceStore{}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{DeviceStore: ds})

	body := []byte(`{"name":"phone","source_type":"owntracks","api_key":"abc123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}

	var resp listDevicesResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal list response failed: %v", err)
	}
	if len(resp.Devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(resp.Devices))
	}
	if resp.Devices[0].APIKey != "abc123" {
		t.Fatalf("expected api key abc123, got %q", resp.Devices[0].APIKey)
	}
}

func TestRequireDeviceAPIKeyAuth(t *testing.T) {
	ds := &fakeDeviceStore{
		devices: []store.Device{
			{ID: 1, UserID: 1, Name: "phone", SourceType: "owntracks", APIKey: "k1"},
		},
	}

	protected := RequireDeviceAPIKeyAuth(ds, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		device, ok := DeviceFromContext(r.Context())
		if !ok {
			http.Error(w, "missing device in context", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_id": device.ID,
		})
	}))

	noKeyReq := httptest.NewRequest(http.MethodGet, "/api/v1/ingest/test", nil)
	noKeyRec := httptest.NewRecorder()
	protected.ServeHTTP(noKeyRec, noKeyReq)
	if noKeyRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing key, got %d", noKeyRec.Code)
	}

	validReq := httptest.NewRequest(http.MethodGet, "/api/v1/ingest/test", nil)
	validReq.Header.Set("X-API-Key", "k1")
	validRec := httptest.NewRecorder()
	protected.ServeHTTP(validRec, validReq)
	if validRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid key, got %d body=%s", validRec.Code, validRec.Body.String())
	}
}

func TestRequireDeviceAPIKeyAuth_DeviceLookupError(t *testing.T) {
	ds := &fakeDeviceStore{lookupError: errors.New("db down")}
	protected := RequireDeviceAPIKeyAuth(ds, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ingest/test", nil)
	req.Header.Set("X-API-Key", "k1")
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on lookup error, got %d", rec.Code)
	}
}
