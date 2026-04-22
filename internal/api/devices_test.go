package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
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

func (f *fakeDeviceStore) GetDeviceByID(_ context.Context, id int64) (store.Device, error) {
	for _, d := range f.devices {
		if d.ID == id {
			return d, nil
		}
	}
	return store.Device{}, store.ErrDeviceNotFound
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

func (f *fakeDeviceStore) RotateDeviceAPIKey(_ context.Context, id int64, newAPIKey string) (store.Device, error) {
	for i := range f.devices {
		if f.devices[i].ID == id {
			f.devices[i].APIKey = newAPIKey
			f.devices[i].UpdatedAt = time.Now().UTC()
			return f.devices[i], nil
		}
	}
	return store.Device{}, store.ErrDeviceNotFound
}

func TestDevicesAPI_CreateReturnsFullKeyAndListMasksKey(t *testing.T) {
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
	var created deviceSecretResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response failed: %v", err)
	}
	if created.APIKey != "abc123" {
		t.Fatalf("expected full api key in create response, got %q", created.APIKey)
	}
	if created.APIKeyPreview == "" {
		t.Fatalf("expected api key preview in create response, got %+v", created)
	}
	if created.CreatedAt == "" || created.UpdatedAt == "" {
		t.Fatalf("expected created_at/updated_at in create response, got %+v", created)
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
	if resp.Devices[0].APIKeyPreview == "" {
		t.Fatalf("expected masked api key preview, got %+v", resp.Devices[0])
	}
	if resp.Devices[0].APIKeyPreview == "abc123" {
		t.Fatalf("expected list response key to be masked, got %q", resp.Devices[0].APIKeyPreview)
	}
}

func TestDevicesAPI_GetMasksKey(t *testing.T) {
	ds := &fakeDeviceStore{}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{DeviceStore: ds})

	body := []byte(`{"name":"phone","source_type":"owntracks","api_key":"abc123"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/devices", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices/1", nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}

	var resp devicePublicResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal get response failed: %v", err)
	}
	if resp.APIKeyPreview == "" || resp.APIKeyPreview == "abc123" {
		t.Fatalf("expected masked preview for get response, got %+v", resp)
	}
}

func TestDevicesAPI_RotateKeyInvalidatesOldKeyAndReturnsNewKey(t *testing.T) {
	ds := &fakeDeviceStore{}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{DeviceStore: ds})

	createBody := []byte(`{"name":"phone","source_type":"owntracks","api_key":"old-key"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/devices", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	rotateBody := []byte(`{"api_key":"new-key"}`)
	rotateReq := httptest.NewRequest(http.MethodPost, "/api/v1/devices/1/rotate-key", bytes.NewReader(rotateBody))
	rotateReq.Header.Set("Content-Type", "application/json")
	rotateRec := httptest.NewRecorder()
	mux.ServeHTTP(rotateRec, rotateReq)
	if rotateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rotateRec.Code, rotateRec.Body.String())
	}

	var rotated deviceSecretResponse
	if err := json.Unmarshal(rotateRec.Body.Bytes(), &rotated); err != nil {
		t.Fatalf("unmarshal rotate response failed: %v", err)
	}
	if rotated.APIKey != "new-key" {
		t.Fatalf("expected rotated full key new-key, got %q", rotated.APIKey)
	}

	if _, err := ds.GetDeviceByAPIKey(context.Background(), "old-key"); !errors.Is(err, store.ErrDeviceNotFound) {
		t.Fatalf("expected old key to be invalidated, got err=%v", err)
	}
	loaded, err := ds.GetDeviceByAPIKey(context.Background(), "new-key")
	if err != nil {
		t.Fatalf("expected new key lookup success, got %v", err)
	}
	if loaded.ID != 1 {
		t.Fatalf("expected rotated key to map to device id=1, got %d", loaded.ID)
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
