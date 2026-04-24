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
	// APIKey is accepted for backwards compatibility but ignored.
	// Device keys are always server-generated.
	APIKey string `json:"api_key"`
}

type rotateDeviceKeyRequest struct {
	// APIKey is accepted for backwards compatibility but ignored.
	// Device keys are always server-generated.
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

func registerDeviceRoutesWithAuth(mux *http.ServeMux, deviceStore DeviceStore, userStore UserStore, sessionStore SessionStore, rateLimiters RateLimiters) {
	if deviceStore == nil {
		panic("registerDeviceRoutesWithAuth requires non-nil deviceStore")
	}
	if userStore == nil || sessionStore == nil {
		panic("registerDeviceRoutesWithAuth requires non-nil userStore and sessionStore")
	}

	withSensitiveRateLimit := func(scope string, next http.Handler) http.Handler {
		limiter := rateLimiters.AdminSensitive
		if limiter == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limiterKey := rateLimitKeyForRequest(r, rateLimiters.TrustProxyHeaders, "admin:"+scope)
			if allowed, retryAfter := limiter.Allow(limiterKey); !allowed {
				writeRateLimitedJSON(w, retryAfter, "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	withSessionAuth := func(next http.Handler) http.Handler {
		return LoadCurrentUserFromSession(
			sessionStore,
			userStore,
			RequireUserSessionAuth(next),
		)
	}

	mux.Handle("POST /api/v1/devices", withSessionAuth(withSensitiveRateLimit("devices:create", http.HandlerFunc(createDeviceHandler(deviceStore)))))
	mux.Handle("GET /api/v1/devices", withSessionAuth(http.HandlerFunc(listDevicesHandler(deviceStore))))
	mux.Handle("GET /api/v1/devices/{id}", withSessionAuth(http.HandlerFunc(getDeviceHandler(deviceStore))))
	mux.Handle("POST /api/v1/devices/{id}/rotate-key", withSessionAuth(withSensitiveRateLimit("devices:rotate", http.HandlerFunc(rotateDeviceKeyHandler(deviceStore)))))
}

func createDeviceHandler(deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, hasCurrentUser := CurrentUserFromContext(r.Context())

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
		apiKey, err := generateAPIKey()
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to generate api key")
			return
		}

		userID := req.UserID
		if hasCurrentUser {
			userID = currentUser.ID
			requestedUserID := req.UserID
			if requestedUserID > 0 && requestedUserID != currentUser.ID {
				if !currentUser.IsAdmin {
					writeJSONError(w, http.StatusForbidden, "cannot create device for another user")
					return
				}
				userID = requestedUserID
			}
		}

		device, err := deviceStore.CreateDevice(r.Context(), store.CreateDeviceParams{
			UserID:     userID,
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

		writeJSON(w, http.StatusCreated, deviceSecretResponseFromStore(device, apiKey))
	}
}

func listDevicesHandler(deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, hasCurrentUser := CurrentUserFromContext(r.Context())

		devices, err := deviceStore.ListDevices(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		out := make([]devicePublicResponse, 0, len(devices))
		for _, d := range devices {
			if hasCurrentUser && d.UserID != currentUser.ID {
				continue
			}
			out = append(out, devicePublicResponseFromStore(d))
		}
		writeJSON(w, http.StatusOK, listDevicesResponse{Devices: out})
	}
}

func getDeviceHandler(deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, hasCurrentUser := CurrentUserFromContext(r.Context())

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
		if hasCurrentUser && device.UserID != currentUser.ID {
			writeJSONError(w, http.StatusNotFound, "device not found")
			return
		}

		writeJSON(w, http.StatusOK, devicePublicResponseFromStore(device))
	}
}

func rotateDeviceKeyHandler(deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, hasCurrentUser := CurrentUserFromContext(r.Context())

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
		if hasCurrentUser && device.UserID != currentUser.ID {
			writeJSONError(w, http.StatusForbidden, "forbidden")
			return
		}

		var req rotateDeviceKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		_ = req.APIKey

		apiKey, genErr := generateAPIKey()
		if genErr != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to generate api key")
			return
		}

		device, err = deviceStore.RotateDeviceAPIKey(r.Context(), deviceID, apiKey)
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

		writeJSON(w, http.StatusOK, deviceSecretResponseFromStore(device, apiKey))
	}
}

func generateAPIKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func deviceSecretResponseFromStore(device store.Device, plainAPIKey string) deviceSecretResponse {
	preview := strings.TrimSpace(device.APIKeyPreview)
	if preview == "" {
		preview = maskAPIKeyPreview(plainAPIKey)
	}
	return deviceSecretResponse{
		ID:            device.ID,
		UserID:        device.UserID,
		Name:          device.Name,
		SourceType:    device.SourceType,
		APIKey:        plainAPIKey,
		APIKeyPreview: preview,
		CreatedAt:     formatDeviceTime(device.CreatedAt),
		UpdatedAt:     formatDeviceTime(device.UpdatedAt),
		LastSeenAt:    formatDeviceTimePtr(device.LastSeenAt),
	}
}

func devicePublicResponseFromStore(device store.Device) devicePublicResponse {
	preview := strings.TrimSpace(device.APIKeyPreview)
	if preview == "" {
		preview = maskAPIKeyPreview(device.APIKey)
	}
	return devicePublicResponse{
		ID:            device.ID,
		UserID:        device.UserID,
		Name:          device.Name,
		SourceType:    device.SourceType,
		APIKeyPreview: preview,
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
	setCommonSecurityHeaders(w)
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
