package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"plexplore/internal/buffer"
	"plexplore/internal/spool"
	"plexplore/internal/store"
)

func expectPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic, got none")
		}
	}()
	fn()
}

func TestRuntimeRouter_NoUnauthFallbackProtectedRoutes(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		DeviceStore: &fakeDeviceStore{},
		PointStore:  &fakePointStore{},
		VisitStore:  &fakeVisitStore{},
	})

	cases := []string{
		"/ui/status",
		"/ui/map",
		"/ui/admin/users",
		"/api/v1/devices",
		"/api/v1/points",
		"/api/v1/exports/geojson",
		"/api/v1/visits",
	}
	for _, path := range cases {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for unregistered protected fallback route %q, got %d body=%s", path, rec.Code, rec.Body.String())
		}
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthRec := httptest.NewRecorder()
	mux.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("expected /health to remain public, got %d body=%s", healthRec.Code, healthRec.Body.String())
	}
}

func TestRouteHelpers_FailClosed_WhenAuthDepsMissing(t *testing.T) {
	expectPanic(t, func() {
		registerDeviceRoutesWithAuth(http.NewServeMux(), &fakeDeviceStore{}, nil, nil, RateLimiters{})
	})
	expectPanic(t, func() {
		registerPointRoutes(http.NewServeMux(), Dependencies{PointStore: &fakePointStore{}})
	})
	expectPanic(t, func() {
		registerExportRoutes(http.NewServeMux(), Dependencies{PointStore: &fakePointStore{}})
	})
	expectPanic(t, func() {
		registerVisitRoutes(http.NewServeMux(), Dependencies{VisitStore: &fakeVisitStore{}})
	})
	expectPanic(t, func() {
		registerUIRoutes(http.NewServeMux(), Dependencies{})
	})
}

func TestRuntimeRouter_ProtectedRoutesDenyAnonymous(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		DeviceStore: &fakeDeviceStore{
			devices: []store.Device{{ID: 1, UserID: 1, Name: "phone-main", SourceType: "owntracks", APIKey: "k1"}},
		},
		PointStore: &fakePointStore{},
		VisitStore: &fakeVisitStore{},
		Spool: &fakeStatusSpool{
			segmentCount: 1,
			checkpoint:   spool.Checkpoint{LastCommittedSeq: 1},
		},
		Buffer: &fakeStatusBuffer{
			stats: buffer.Stats{},
		},
		UserStore: &fakeUserStore{
			users: map[int64]store.User{
				1: {ID: 1, Email: "u@example.com", IsAdmin: false},
			},
		},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{}},
	})

	htmlProtected := []string{"/", "/ui/status", "/ui/map", "/ui/admin/users"}
	for _, path := range htmlProtected {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("expected 303 for anonymous protected html route %q, got %d body=%s", path, rec.Code, rec.Body.String())
		}
		if !strings.HasPrefix(rec.Header().Get("Location"), "/login") {
			t.Fatalf("expected login redirect for %q, got %q", path, rec.Header().Get("Location"))
		}
	}

	apiProtected := []string{
		"/api/v1/status",
		"/api/v1/devices",
		"/api/v1/points",
		"/api/v1/exports/geojson",
		"/api/v1/visits",
		"/api/v1/users",
	}
	for _, path := range apiProtected {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for anonymous protected api route %q, got %d body=%s", path, rec.Code, rec.Body.String())
		}
	}

	loginReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	loginRec := httptest.NewRecorder()
	mux.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected /login to remain public, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/status", nil)
	statusRec := httptest.NewRecorder()
	mux.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected /status to remain public-safe, got %d body=%s", statusRec.Code, statusRec.Body.String())
	}
}

func TestRuntimeRouter_StatusAliasProtectionMatchesCanonical(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore: &fakeUserStore{
			users: map[int64]store.User{
				1: {ID: 1, Email: "u@example.com"},
			},
		},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{}},
	})

	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	recRoot := httptest.NewRecorder()
	mux.ServeHTTP(recRoot, reqRoot)
	if recRoot.Code != http.StatusSeeOther {
		t.Fatalf("expected / to require auth, got %d body=%s", recRoot.Code, recRoot.Body.String())
	}

	reqAlias := httptest.NewRequest(http.MethodGet, "/ui/status", nil)
	recAlias := httptest.NewRecorder()
	mux.ServeHTTP(recAlias, reqAlias)
	if recAlias.Code != http.StatusSeeOther {
		t.Fatalf("expected /ui/status to require auth, got %d body=%s", recAlias.Code, recAlias.Body.String())
	}

	if !strings.HasPrefix(recRoot.Header().Get("Location"), "/login") || !strings.HasPrefix(recAlias.Header().Get("Location"), "/login") {
		t.Fatalf("expected both canonical and alias to redirect to /login, got root=%q alias=%q", recRoot.Header().Get("Location"), recAlias.Header().Get("Location"))
	}
}

func TestRuntimeRouter_AdminRoutesRejectNonAdmin(t *testing.T) {
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

	uiReq := httptest.NewRequest(http.MethodGet, "/ui/admin/users", nil)
	uiReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-user"})
	uiRec := httptest.NewRecorder()
	mux.ServeHTTP(uiRec, uiReq)
	if uiRec.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin forbidden for /ui/admin/users, got %d body=%s", uiRec.Code, uiRec.Body.String())
	}

	apiReq := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	apiReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-user"})
	apiRec := httptest.NewRecorder()
	mux.ServeHTTP(apiRec, apiReq)
	if apiRec.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin forbidden for /api/v1/users, got %d body=%s", apiRec.Code, apiRec.Body.String())
	}
}
