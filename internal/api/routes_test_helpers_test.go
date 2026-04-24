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
		registerDeviceRoutesWithAuth(mux, deps.DeviceStore, deps.UserStore, deps.SessionStore, deps.RateLimiters)
	}
	if deps.DeviceStore != nil && deps.Spool != nil && deps.Buffer != nil {
		registerIngestRoutes(mux, deps)
	}
	if deps.Spool != nil && deps.Buffer != nil {
		registerStatusRoutes(mux, deps)
	}
	if deps.PointStore != nil {
		registerPointRoutes(mux, deps)
		registerExportRoutes(mux, deps)
	}
	if deps.VisitStore != nil {
		registerVisitRoutes(mux, deps)
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
