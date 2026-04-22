package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"plexplore/internal/store"
)

type createDeviceRequest struct {
	UserID     int64  `json:"user_id"`
	Name       string `json:"name"`
	SourceType string `json:"source_type"`
	APIKey     string `json:"api_key"`
}

type deviceResponse struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Name       string `json:"name"`
	SourceType string `json:"source_type"`
	APIKey     string `json:"api_key"`
}

type listDevicesResponse struct {
	Devices []deviceResponse `json:"devices"`
}

func registerDeviceRoutes(mux *http.ServeMux, deviceStore DeviceStore) {
	mux.HandleFunc("POST /api/v1/devices", createDeviceHandler(deviceStore))
	mux.HandleFunc("GET /api/v1/devices", listDevicesHandler(deviceStore))
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

		writeJSON(w, http.StatusCreated, deviceResponseFromStore(device))
	}
}

func listDevicesHandler(deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		devices, err := deviceStore.ListDevices(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		out := make([]deviceResponse, 0, len(devices))
		for _, d := range devices {
			out = append(out, deviceResponseFromStore(d))
		}
		writeJSON(w, http.StatusOK, listDevicesResponse{Devices: out})
	}
}

func generateAPIKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func deviceResponseFromStore(device store.Device) deviceResponse {
	return deviceResponse{
		ID:         device.ID,
		UserID:     device.UserID,
		Name:       device.Name,
		SourceType: device.SourceType,
		APIKey:     device.APIKey,
	}
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
