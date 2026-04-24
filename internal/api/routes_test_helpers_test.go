package api

import "net/http"

// registerRoutesWithTestFallbacks preserves legacy unauthenticated fallback
// registrations for tests that validate handler behavior without auth wiring.
// Production code must use RegisterRoutesWithDependencies.
func registerRoutesWithTestFallbacks(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /health", healthHandler)
	registerUIAssetRoutes(mux)
	registerUIRoutesWithFallbacksForTest(mux, deps)

	if deps.DeviceStore != nil {
		if deps.UserStore != nil && deps.SessionStore != nil {
			registerDeviceRoutesWithAuth(mux, deps.DeviceStore, deps.UserStore, deps.SessionStore, deps.RateLimiters)
		} else {
			mux.HandleFunc("POST /api/v1/devices", createDeviceHandler(deps.DeviceStore))
			mux.HandleFunc("GET /api/v1/devices", listDevicesHandler(deps.DeviceStore))
			mux.HandleFunc("GET /api/v1/devices/{id}", getDeviceHandler(deps.DeviceStore))
			mux.HandleFunc("POST /api/v1/devices/{id}/rotate-key", rotateDeviceKeyHandler(deps.DeviceStore))
		}
	}
	if deps.DeviceStore != nil && deps.Spool != nil && deps.Buffer != nil {
		registerIngestRoutes(mux, deps)
	}
	if deps.Spool != nil && deps.Buffer != nil {
		registerStatusRoutes(mux, deps)
	}
	if deps.PointStore != nil {
		if deps.UserStore != nil && deps.SessionStore != nil && deps.DeviceStore != nil {
			registerPointRoutes(mux, deps)
			registerExportRoutes(mux, deps)
		} else {
			mux.HandleFunc("GET /api/v1/points", pointsHandler(deps.PointStore, nil))
			mux.HandleFunc("GET /api/v1/points/recent", recentPointsHandler(deps.PointStore, nil))
			mux.HandleFunc("GET /api/v1/exports/geojson", geoJSONExportHandler(deps.PointStore, nil))
			mux.HandleFunc("GET /api/v1/exports/gpx", gpxExportHandler(deps.PointStore, nil))
		}
	}
	if deps.VisitStore != nil {
		if deps.UserStore != nil && deps.SessionStore != nil && deps.DeviceStore != nil {
			registerVisitRoutes(mux, deps)
		} else {
			mux.HandleFunc("POST /api/v1/visits/generate", generateVisitsHandler(deps.VisitStore, nil))
			mux.HandleFunc("GET /api/v1/visits", listVisitsHandler(deps.VisitStore, deps.VisitLabelResolver, nil))
		}
	}
	if deps.UserStore != nil && deps.SessionStore != nil {
		registerLoginRoutes(mux, deps.UserStore, deps.SessionStore, deps.CookieSecurity, deps.RateLimiters)
		registerUserRoutes(mux, deps.UserStore, deps.SessionStore, deps.RateLimiters)
	}
}

func registerUIRoutesWithFallbacksForTest(mux *http.ServeMux, deps Dependencies) {
	if deps.SessionStore != nil && deps.UserStore != nil {
		registerUIRoutes(mux, deps)
		return
	}
	mux.HandleFunc("GET /{$}", statusPageHandler(deps.CookieSecurity))
	mux.HandleFunc("GET /ui/status", statusPageHandler(deps.CookieSecurity))
	mux.HandleFunc("GET /ui/map", mapPageHandler(deps.CookieSecurity))
	mux.HandleFunc("GET /ui/admin/users", adminUsersPageHandler(deps.CookieSecurity))
}
