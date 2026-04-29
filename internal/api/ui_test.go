package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"plexplore/internal/store"
	"time"
)

func TestStatusPageServedAtRoot(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore: &fakeUserStore{
			users: map[int64]store.User{
				1: {ID: 1, Email: "u@example.com"},
			},
		},
		SessionStore: &fakeSessionStore{
			sessionByToken: map[string]store.Session{
				"tok-1": {Token: "tok-1", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected html content type, got %q", contentType)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Plexplore Status") {
		t.Fatalf("expected status page title in body, got %q", body)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatalf("expected CSP header on status page")
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected X-Frame-Options DENY, got %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options nosniff, got %q", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got == "" {
		t.Fatalf("expected Referrer-Policy header")
	}
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("did not expect in-app HSTS on local HTTP status page, got %q", got)
	}
	if !strings.Contains(body, "Recent Points") {
		t.Fatalf("expected recent points section in body, got %q", body)
	}
	if !strings.Contains(body, `id="scheduler_state"`) || !strings.Contains(body, `id="scheduler_meta"`) {
		t.Fatalf("expected scheduler status section in status page, got %q", body)
	}
	if !strings.Contains(body, `id="theme_toggle"`) {
		t.Fatalf("expected theme toggle in status page, got %q", body)
	}
	if !strings.Contains(body, `id="session_user"`) {
		t.Fatalf("expected session user indicator in status page, got %q", body)
	}
	if !strings.Contains(body, `action="/logout"`) {
		t.Fatalf("expected logout form in status page, got %q", body)
	}
	if !strings.Contains(body, `name="csrf_token"`) {
		t.Fatalf("expected csrf token field in status page logout form, got %q", body)
	}
	if !strings.Contains(body, `id="status_to_map_link"`) || !strings.Contains(body, `href="/ui/map"`) {
		t.Fatalf("expected map navigation link on status page, got %q", body)
	}
	if !strings.Contains(body, `/ui/assets/app/common.js`) || !strings.Contains(body, `/ui/assets/app/status.js`) {
		t.Fatalf("expected external status/common scripts in status page, got %q", body)
	}
	if strings.Contains(body, "<style>") || strings.Contains(body, "<script>") {
		t.Fatalf("expected no inline style/script tags in status page, got %q", body)
	}
	if got := rec.Header().Get("Content-Security-Policy"); strings.Contains(got, "unsafe-inline") {
		t.Fatalf("expected CSP without unsafe-inline, got %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "img-src 'self' data:") {
		t.Fatalf("expected restrictive img-src on status page, got %q", got)
	}
}

func TestStatusPage_DoesNotMatchTypoPath(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    &fakeUserStore{users: map[int64]store.User{1: {ID: 1, Email: "u@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{}},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/devic", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for typo path, got %d", rec.Code)
	}
}

func TestMapPageServedAtUIMap(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore: &fakeUserStore{
			users: map[int64]store.User{
				1: {ID: 1, Email: "u@example.com"},
			},
		},
		SessionStore: &fakeSessionStore{
			sessionByToken: map[string]store.Session{
				"tok-1": {Token: "tok-1", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/map", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected html content type, got %q", contentType)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Plexplore Map") {
		t.Fatalf("expected map page title in body, got %q", body)
	}
	if !strings.Contains(body, `id="map"`) {
		t.Fatalf("expected map container in body, got %q", body)
	}
	if !strings.Contains(body, `id="device_select"`) {
		t.Fatalf("expected device select in body, got %q", body)
	}
	if !strings.Contains(body, `id="sampling_note"`) {
		t.Fatalf("expected sampling note element in body, got %q", body)
	}
	if !strings.Contains(body, `id="date_from"`) || !strings.Contains(body, `id="date_to"`) {
		t.Fatalf("expected date range inputs in body, got %q", body)
	}
	if !strings.Contains(body, "Refresh") {
		t.Fatalf("expected refresh button label in body, got %q", body)
	}
	if !strings.Contains(strings.ToLower(body), "/ui/assets/leaflet/leaflet.css") || !strings.Contains(strings.ToLower(body), "/ui/assets/leaflet/leaflet.js") {
		t.Fatalf("expected self-hosted leaflet assets in body, got %q", body)
	}
	if strings.Contains(strings.ToLower(body), "unpkg.com/leaflet") {
		t.Fatalf("did not expect CDN leaflet URL in body, got %q", body)
	}
	if !strings.Contains(body, `data-tile-mode="none"`) {
		t.Fatalf("expected default privacy-preserving tile mode none, got %q", body)
	}
	if strings.Contains(strings.ToLower(body), "tile.openstreetmap.org") {
		t.Fatalf("did not expect default map page to call external OSM tiles, got %q", body)
	}
	if !strings.Contains(body, `/ui/assets/app/common.js`) || !strings.Contains(body, `/ui/assets/app/map.js`) {
		t.Fatalf("expected external map/common scripts in map page, got %q", body)
	}
	if strings.Contains(body, "<style>") || strings.Contains(body, "<script>") {
		t.Fatalf("expected no inline style/script tags in map page, got %q", body)
	}
	if !strings.Contains(body, "Visits Summary") {
		t.Fatalf("expected visits summary section in map page, got %q", body)
	}
	if !strings.Contains(body, `id="visits_body"`) {
		t.Fatalf("expected visits summary table body in map page, got %q", body)
	}
	if !strings.Contains(body, `id="theme_toggle"`) {
		t.Fatalf("expected theme toggle in map page, got %q", body)
	}
	if !strings.Contains(body, `id="session_user"`) {
		t.Fatalf("expected session user indicator in map page, got %q", body)
	}
	if !strings.Contains(body, `action="/logout"`) {
		t.Fatalf("expected logout form in map page, got %q", body)
	}
	if !strings.Contains(body, `name="csrf_token"`) {
		t.Fatalf("expected csrf token field in map page logout form, got %q", body)
	}
	if !strings.Contains(body, `id="map_to_status_link"`) || !strings.Contains(body, `href="/ui/status"`) {
		t.Fatalf("expected status navigation link on map page, got %q", body)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatalf("expected CSP header on map page")
	}
	if got := rec.Header().Get("Content-Security-Policy"); strings.Contains(got, "unsafe-inline") {
		t.Fatalf("expected CSP without unsafe-inline, got %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "img-src 'self' data:") {
		t.Fatalf("expected restrictive img-src for default map mode none, got %q", got)
	}
}

func TestMapPage_UsesConfiguredExternalTileProvider(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore: &fakeUserStore{
			users: map[int64]store.User{
				1: {ID: 1, Email: "u@example.com"},
			},
		},
		SessionStore: &fakeSessionStore{
			sessionByToken: map[string]store.Session{
				"tok-1": {Token: "tok-1", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
			},
		},
		MapTiles: MapTileConfig{
			Mode:        "custom",
			URLTemplate: "http://tiles.local/{z}/{x}/{y}.png",
			Attribution: "local tile server",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/map", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-tile-mode="custom"`) {
		t.Fatalf("expected custom tile mode in map page, got %q", body)
	}
	if !strings.Contains(body, `data-tile-url-template="http://tiles.local/{z}/{x}/{y}.png"`) {
		t.Fatalf("expected custom tile template in map page, got %q", body)
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "img-src 'self' data: http://tiles.local") {
		t.Fatalf("expected custom tile origin in CSP, got %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); hasImgSrcWildcardScheme(got) {
		t.Fatalf("expected no broad http/https wildcard in CSP, got %q", got)
	}
}

func TestMapPage_CSPIncludesOSMOriginsWhenModeOSM(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore: &fakeUserStore{
			users: map[int64]store.User{
				1: {ID: 1, Email: "u@example.com"},
			},
		},
		SessionStore: &fakeSessionStore{
			sessionByToken: map[string]store.Session{
				"tok-1": {Token: "tok-1", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
			},
		},
		MapTiles: MapTileConfig{
			Mode:        "osm",
			URLTemplate: "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/map", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{
		"https://tile.openstreetmap.org",
		"https://a.tile.openstreetmap.org",
		"https://b.tile.openstreetmap.org",
		"https://c.tile.openstreetmap.org",
	} {
		if !strings.Contains(csp, want) {
			t.Fatalf("expected OSM origin %q in CSP, got %q", want, csp)
		}
	}
}

func TestUIAssets_LeafletServedLocally(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/ui/assets/leaflet/leaflet.css", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for local leaflet css, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), ".leaflet-container") {
		t.Fatalf("expected leaflet css content, got %q", rec.Body.String())
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff header on local asset, got %q", got)
	}
}

func TestUIAssets_LeafletIconServedLocally(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/ui/assets/leaflet/images/marker-icon.png", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for local leaflet icon, got %d body=%s", rec.Code, rec.Body.String())
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "image/png") {
		t.Fatalf("expected image/png content type for marker icon, got %q", contentType)
	}
}

func TestUIAssets_MapScriptContainsEscapedPopupFields(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/ui/assets/app/map.js", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for map js asset, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "escapeHTML(p.timestamp_utc || \"\")") || !strings.Contains(body, "escapeHTML(p.device_id || \"\")") {
		t.Fatalf("expected escaped popup values in map js, got %q", body)
	}
	if !strings.Contains(body, "/api/v1/visits") || !strings.Contains(body, "/api/v1/points?") {
		t.Fatalf("expected map js to query points and visits endpoints, got %q", body)
	}
	if !strings.Contains(body, `params.set("simplify", simplify ? "true" : "false")`) || !strings.Contains(body, `params.set("max_points"`) {
		t.Fatalf("expected map js to request simplification controls, got %q", body)
	}
	if !strings.Contains(body, "renderClusteredMarkers") {
		t.Fatalf("expected map js clustering helper, got %q", body)
	}
}

func TestUIRoutesRequireSession_WhenSessionDepsProvided(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    &fakeUserStore{users: map[int64]store.User{1: {ID: 1, Email: "u@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{}},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect for anonymous ui request, got %d", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/login?next=%2Fui%2Fstatus" {
		t.Fatalf("expected redirect location with next to /login?next=..., got %q", got)
	}
}

func TestUIRoutesAllowSession_WhenValidSessionCookiePresent(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore: &fakeUserStore{
			users: map[int64]store.User{
				1: {ID: 1, Email: "u@example.com"},
			},
		},
		SessionStore: &fakeSessionStore{
			sessionByToken: map[string]store.Session{
				"tok-1": {Token: "tok-1", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/map", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid session, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "u@example.com") {
		t.Fatalf("expected current user email in ui page, got %q", body)
	}
	if !strings.Contains(body, `action="/logout"`) {
		t.Fatalf("expected logout control in ui page, got %q", body)
	}
	if strings.Contains(body, `id="admin_users_link"`) {
		t.Fatalf("did not expect admin link for non-admin user, got %q", body)
	}
}

func TestAdminUsersPageServedForAdminSession(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore: &fakeUserStore{
			users: map[int64]store.User{
				1: {ID: 1, Email: "admin@example.com", IsAdmin: true},
			},
		},
		SessionStore: &fakeSessionStore{
			sessionByToken: map[string]store.Session{
				"tok-admin": {Token: "tok-admin", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-admin"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin users page, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<title>Plexplore Users</title>") || !strings.Contains(body, "<h1>Users</h1>") {
		t.Fatalf("expected users page title/heading in body, got %q", body)
	}
	if !strings.Contains(body, `/ui/assets/app/common.js`) || !strings.Contains(body, `/ui/assets/app/users.js`) {
		t.Fatalf("expected external users/common scripts in admin users page, got %q", body)
	}
	if !strings.Contains(body, `id="theme_toggle"`) {
		t.Fatalf("expected theme toggle on users page, got %q", body)
	}
	if strings.Contains(body, "<style>") || strings.Contains(body, "<script>") {
		t.Fatalf("expected no inline style/script tags on users page, got %q", body)
	}
	if !strings.Contains(body, "admin@example.com") {
		t.Fatalf("expected current admin email in body, got %q", body)
	}
}

func TestStatusPage_AdminLinkStillRendersForAdminSession(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore: &fakeUserStore{
			users: map[int64]store.User{
				1: {ID: 1, Email: "admin@example.com", IsAdmin: true},
			},
		},
		SessionStore: &fakeSessionStore{
			sessionByToken: map[string]store.Session{
				"tok-admin": {Token: "tok-admin", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/status", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-admin"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin status page, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="admin_users_link"`) || !strings.Contains(body, `href="/ui/admin/users"`) {
		t.Fatalf("expected existing admin users nav link on status page, got %q", body)
	}
	if !strings.Contains(body, `id="admin_devices_link"`) || !strings.Contains(body, `href="/ui/admin/devices"`) {
		t.Fatalf("expected devices nav link on status page, got %q", body)
	}
	if !strings.Contains(body, ">Users</a>") {
		t.Fatalf("expected Users nav label on status page, got %q", body)
	}
	if !strings.Contains(body, ">Devices</a>") {
		t.Fatalf("expected Devices nav label on status page, got %q", body)
	}
}

func TestMapPage_AdminLinkLabelIsUsersForAdminSession(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore: &fakeUserStore{
			users: map[int64]store.User{
				1: {ID: 1, Email: "admin@example.com", IsAdmin: true},
			},
		},
		SessionStore: &fakeSessionStore{
			sessionByToken: map[string]store.Session{
				"tok-admin": {Token: "tok-admin", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/map", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-admin"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin map page, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="admin_users_link"`) || !strings.Contains(body, `href="/ui/admin/users"`) {
		t.Fatalf("expected users nav link on map page, got %q", body)
	}
	if !strings.Contains(body, `id="admin_devices_link"`) || !strings.Contains(body, `href="/ui/admin/devices"`) {
		t.Fatalf("expected devices nav link on map page, got %q", body)
	}
	if !strings.Contains(body, ">Users</a>") {
		t.Fatalf("expected Users nav label on map page, got %q", body)
	}
	if !strings.Contains(body, ">Devices</a>") {
		t.Fatalf("expected Devices nav label on map page, got %q", body)
	}
}

func TestAdminUsersPageDeniedForNonAdminSession(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore: &fakeUserStore{
			users: map[int64]store.User{
				2: {ID: 2, Email: "user@example.com", IsAdmin: false},
			},
		},
		SessionStore: &fakeSessionStore{
			sessionByToken: map[string]store.Session{
				"tok-user": {Token: "tok-user", UserID: 2, ExpiresAt: time.Now().UTC().Add(time.Hour)},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-user"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin on admin users page, got %d", rec.Code)
	}
}

func TestAdminDevicesPageServedForAdminSession(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore: &fakeUserStore{
			users: map[int64]store.User{
				1: {ID: 1, Email: "admin@example.com", IsAdmin: true},
			},
		},
		SessionStore: &fakeSessionStore{
			sessionByToken: map[string]store.Session{
				"tok-admin": {Token: "tok-admin", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/admin/devices", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-admin"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin devices page, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<title>Plexplore Devices</title>") || !strings.Contains(body, "<h1>Devices</h1>") {
		t.Fatalf("expected devices page title/heading in body, got %q", body)
	}
	if !strings.Contains(body, `/ui/assets/app/common.js`) || !strings.Contains(body, `/ui/assets/app/devices.js`) {
		t.Fatalf("expected external devices/common scripts in admin devices page, got %q", body)
	}
	if !strings.Contains(body, `id="admin_users_link"`) || !strings.Contains(body, `href="/ui/admin/users"`) {
		t.Fatalf("expected users nav link on devices page, got %q", body)
	}
	if !strings.Contains(body, `id="create_device_btn"`) || !strings.Contains(body, `id="generate_visits_btn"`) {
		t.Fatalf("expected create and generate controls in devices page, got %q", body)
	}
	if !strings.Contains(body, `id="theme_toggle"`) {
		t.Fatalf("expected theme toggle on devices page, got %q", body)
	}
}

func TestAdminDevicesPageDeniedForNonAdminSession(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore: &fakeUserStore{
			users: map[int64]store.User{
				2: {ID: 2, Email: "user@example.com", IsAdmin: false},
			},
		},
		SessionStore: &fakeSessionStore{
			sessionByToken: map[string]store.Session{
				"tok-user": {Token: "tok-user", UserID: 2, ExpiresAt: time.Now().UTC().Add(time.Hour)},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/admin/devices", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-user"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin on admin devices page, got %d", rec.Code)
	}
}

func TestUIAssets_DevicesScriptContainsDeviceAndVisitWorkflows(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/ui/assets/app/devices.js", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for devices js asset, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/api/v1/devices") || !strings.Contains(body, "/rotate-key") {
		t.Fatalf("expected device list/rotate workflow in devices js, got %q", body)
	}
	if !strings.Contains(body, "/api/v1/visits/generate") {
		t.Fatalf("expected visit generation workflow in devices js, got %q", body)
	}
	if !strings.Contains(body, "X-CSRF-Token") {
		t.Fatalf("expected csrf header usage in devices js, got %q", body)
	}
}
