package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"plexplore/internal/store"
)

type contextKey string

const authenticatedDeviceKey contextKey = "authenticated_device"

// RequireDeviceAPIKeyAuth authenticates requests using device API keys.
// Intended for ingest endpoints once they are added.
func RequireDeviceAPIKeyAuth(deviceStore DeviceStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := extractAPIKey(r)
		if apiKey == "" {
			writeJSONError(w, http.StatusUnauthorized, "missing API key")
			return
		}

		device, err := deviceStore.GetDeviceByAPIKey(r.Context(), apiKey)
		if err != nil {
			if errors.Is(err, store.ErrDeviceNotFound) {
				writeJSONError(w, http.StatusUnauthorized, "invalid API key")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "device lookup failed")
			return
		}

		ctx := context.WithValue(r.Context(), authenticatedDeviceKey, device)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func DeviceFromContext(ctx context.Context) (store.Device, bool) {
	device, ok := ctx.Value(authenticatedDeviceKey).(store.Device)
	return device, ok
}

func extractAPIKey(r *http.Request) string {
	apiKey := strings.TrimSpace(r.Header.Get("X-API-Key"))
	if apiKey != "" {
		return apiKey
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	const bearerPrefix = "Bearer "
	if strings.HasPrefix(authHeader, bearerPrefix) {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
	}
	return ""
}
