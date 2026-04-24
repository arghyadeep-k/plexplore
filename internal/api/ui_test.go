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
	if !strings.Contains(body, "localStorage") || !strings.Contains(body, "prefers-color-scheme") {
		t.Fatalf("expected dark mode persistence/system preference hooks in status page, got %q", body)
	}
}

func TestStatusPage_DoesNotMatchTypoPath(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    &fakeUserStore{users: map[int64]store.User{1: {ID: 1, Email: "u@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{}},
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/statu", nil)
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
	if !strings.Contains(body, "/api/v1/visits") {
		t.Fatalf("expected visits endpoint usage in map page, got %q", body)
	}
	if !strings.Contains(body, "visitLayer") {
		t.Fatalf("expected visit layer rendering in map page script, got %q", body)
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
	if !strings.Contains(body, "localStorage") || !strings.Contains(body, "prefers-color-scheme") {
		t.Fatalf("expected dark mode persistence/system preference hooks in map page, got %q", body)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatalf("expected CSP header on map page")
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
	if !strings.Contains(body, "/api/v1/users") {
		t.Fatalf("expected admin users API usage in body, got %q", body)
	}
	if !strings.Contains(body, "X-CSRF-Token") {
		t.Fatalf("expected csrf header usage in admin users page script, got %q", body)
	}
	if !strings.Contains(body, `id="theme_toggle"`) {
		t.Fatalf("expected theme toggle on users page, got %q", body)
	}
	if !strings.Contains(body, "localStorage") || !strings.Contains(body, "prefers-color-scheme") {
		t.Fatalf("expected dark mode hooks on users page, got %q", body)
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
	if !strings.Contains(body, ">Users</a>") {
		t.Fatalf("expected Users nav label on status page, got %q", body)
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
	if !strings.Contains(body, ">Users</a>") {
		t.Fatalf("expected Users nav label on map page, got %q", body)
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
