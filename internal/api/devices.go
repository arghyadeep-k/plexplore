package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"plexplore/internal/store"
)

type createDeviceRequest struct {
	UserID     int64  `json:"user_id"`
	Name       string `json:"name"`
	SourceType string `json:"source_type"`
	APIKey     string `json:"api_key"`
}

type rotateDeviceKeyRequest struct {
	APIKey string `json:"api_key"`
}

type deviceSecretResponse struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"user_id"`
	Name          string `json:"name"`
	SourceType    string `json:"source_type"`
	APIKey        string `json:"api_key"`
	APIKeyPreview string `json:"api_key_preview"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
	LastSeenAt    string `json:"last_seen_at,omitempty"`
}

type devicePublicResponse struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"user_id"`
	Name          string `json:"name"`
	SourceType    string `json:"source_type"`
	APIKeyPreview string `json:"api_key_preview"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
	LastSeenAt    string `json:"last_seen_at,omitempty"`
}

type listDevicesResponse struct {
	Devices []devicePublicResponse `json:"devices"`
}

func registerDeviceRoutes(mux *http.ServeMux, deviceStore DeviceStore) {
	mux.HandleFunc("POST /api/v1/devices", createDeviceHandler(deviceStore))
	mux.HandleFunc("GET /api/v1/devices", listDevicesHandler(deviceStore))
	mux.HandleFunc("GET /api/v1/devices/{id}", getDeviceHandler(deviceStore))
	mux.HandleFunc("POST /api/v1/devices/{id}/rotate-key", rotateDeviceKeyHandler(deviceStore))
}

func createDeviceHandler(deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createDeviceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		name := strings.TrimSpace(req.Name)
		sourceType := strings.TrimSpace(req.SourceType)
		if name == "" || sourceType == "" {
			writeJSONError(w, http.StatusBadRequest, "name and source_type are required")
			return
		}
		apiKey := strings.TrimSpace(req.APIKey)
		if apiKey == "" {
			generated, err := generateAPIKey()
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "failed to generate api key")
				return
			}
			apiKey = generated
		}

		device, err := deviceStore.CreateDevice(r.Context(), store.CreateDeviceParams{
			UserID:     req.UserID,
			Name:       name,
			SourceType: sourceType,
			APIKey:     apiKey,
		})
		if err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				status = http.StatusConflict
			}
			writeJSONError(w, status, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, deviceSecretResponseFromStore(device))
	}
}

func listDevicesHandler(deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		devices, err := deviceStore.ListDevices(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		out := make([]devicePublicResponse, 0, len(devices))
		for _, d := range devices {
			out = append(out, devicePublicResponseFromStore(d))
		}
		writeJSON(w, http.StatusOK, listDevicesResponse{Devices: out})
	}
}

func getDeviceHandler(deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceID, err := parseDeviceIDPath(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid device id")
			return
		}

		device, err := deviceStore.GetDeviceByID(r.Context(), deviceID)
		if err != nil {
			if errors.Is(err, store.ErrDeviceNotFound) {
				writeJSONError(w, http.StatusNotFound, "device not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, devicePublicResponseFromStore(device))
	}
}

func rotateDeviceKeyHandler(deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceID, err := parseDeviceIDPath(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid device id")
			return
		}

		var req rotateDeviceKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		apiKey := strings.TrimSpace(req.APIKey)
		if apiKey == "" {
			generated, genErr := generateAPIKey()
			if genErr != nil {
				writeJSONError(w, http.StatusInternalServerError, "failed to generate api key")
				return
			}
			apiKey = generated
		}

		device, err := deviceStore.RotateDeviceAPIKey(r.Context(), deviceID, apiKey)
		if err != nil {
			if errors.Is(err, store.ErrDeviceNotFound) {
				writeJSONError(w, http.StatusNotFound, "device not found")
				return
			}
			status := http.StatusInternalServerError
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				status = http.StatusConflict
			}
			writeJSONError(w, status, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, deviceSecretResponseFromStore(device))
	}
}

func generateAPIKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func deviceSecretResponseFromStore(device store.Device) deviceSecretResponse {
	return deviceSecretResponse{
		ID:            device.ID,
		UserID:        device.UserID,
		Name:          device.Name,
		SourceType:    device.SourceType,
		APIKey:        device.APIKey,
		APIKeyPreview: maskAPIKeyPreview(device.APIKey),
		CreatedAt:     formatDeviceTime(device.CreatedAt),
		UpdatedAt:     formatDeviceTime(device.UpdatedAt),
		LastSeenAt:    formatDeviceTimePtr(device.LastSeenAt),
	}
}

func devicePublicResponseFromStore(device store.Device) devicePublicResponse {
	return devicePublicResponse{
		ID:            device.ID,
		UserID:        device.UserID,
		Name:          device.Name,
		SourceType:    device.SourceType,
		APIKeyPreview: maskAPIKeyPreview(device.APIKey),
		CreatedAt:     formatDeviceTime(device.CreatedAt),
		UpdatedAt:     formatDeviceTime(device.UpdatedAt),
		LastSeenAt:    formatDeviceTimePtr(device.LastSeenAt),
	}
}

func parseDeviceIDPath(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(r.PathValue("id"))
	if raw == "" {
		return 0, strconv.ErrSyntax
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}

func maskAPIKeyPreview(apiKey string) string {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return ""
	}
	if len(key) <= 6 {
		return key[:1] + "..." + key[len(key)-1:]
	}
	prefix := key[:4]
	suffix := key[len(key)-4:]
	return prefix + "..." + suffix
}

func formatDeviceTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func formatDeviceTimePtr(ts *time.Time) string {
	if ts == nil || ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

func IsDeviceAuthError(err error) bool {
	return errors.Is(err, store.ErrDeviceNotFound)
}
