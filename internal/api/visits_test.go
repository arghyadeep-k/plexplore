package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"plexplore/internal/store"
	"plexplore/internal/visits"
)

type fakeVisitStore struct {
	lastUserID   int64
	lastDeviceID *int64
	lastFromUTC  *time.Time
	lastToUTC    *time.Time
	lastConfig   visits.Config
	lastLimit    int
	created      int
	list         []store.Visit
	deviceOwners map[int64]int64
}

type fakeVisitLabelResolver struct {
	enabled       bool
	maxLookups    int
	calls         int
	providerCalls int
	label         string
}

func (f *fakeVisitLabelResolver) Enabled() bool { return f.enabled }

func (f *fakeVisitLabelResolver) MaxProviderLookupsPerRequest() int { return f.maxLookups }

func (f *fakeVisitLabelResolver) ResolveVisitLabel(_ context.Context, _, _ float64, allowProvider bool) (string, bool, error) {
	f.calls++
	if !allowProvider {
		return "", false, nil
	}
	f.providerCalls++
	return f.label, true, nil
}

func (f *fakeVisitStore) RebuildVisitsForDeviceRange(_ context.Context, deviceID int64, fromUTC, toUTC *time.Time, cfg visits.Config) (int, error) {
	f.lastDeviceID = &deviceID
	f.lastFromUTC = fromUTC
	f.lastToUTC = toUTC
	f.lastConfig = cfg
	return f.created, nil
}

func (f *fakeVisitStore) ListVisits(_ context.Context, userID int64, deviceID *int64, fromUTC, toUTC *time.Time, limit int) ([]store.Visit, error) {
	f.lastUserID = userID
	f.lastDeviceID = deviceID
	f.lastFromUTC = fromUTC
	f.lastToUTC = toUTC
	f.lastLimit = limit
	out := make([]store.Visit, 0, len(f.list))
	for _, item := range f.list {
		if len(f.deviceOwners) > 0 {
			if ownerID, ok := f.deviceOwners[item.DeviceRowID]; !ok || ownerID != userID {
				continue
			}
		}
		if deviceID != nil && item.DeviceRowID != *deviceID {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func TestGenerateVisitsEndpoint_DeviceAndRange(t *testing.T) {
	vs := &fakeVisitStore{created: 2}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{VisitStore: vs})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/visits/generate?device_id=1&from=2026-04-20T00:00:00Z&to=2026-04-22T00:00:00Z&min_dwell=10m&max_radius_m=25", nil)
	addCSRF(req, testCSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if vs.lastDeviceID == nil || *vs.lastDeviceID != 1 {
		t.Fatalf("expected device_id 1, got %+v", vs.lastDeviceID)
	}
	if vs.lastFromUTC == nil || vs.lastToUTC == nil {
		t.Fatalf("expected explicit from/to passed to visit generation, got from=%v to=%v", vs.lastFromUTC, vs.lastToUTC)
	}
	if vs.lastConfig.MinDwell != 10*time.Minute || vs.lastConfig.MaxRadiusMeters != 25 {
		t.Fatalf("unexpected visit config: %+v", vs.lastConfig)
	}

	var resp generateVisitsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if !resp.OK || resp.CreatedVisits != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestGenerateVisitsEndpoint_InvalidParams(t *testing.T) {
	vs := &fakeVisitStore{}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{VisitStore: vs})

	cases := []string{
		"/api/v1/visits/generate",
		"/api/v1/visits/generate?device_id=bad&from=bad",
		"/api/v1/visits/generate?device_id=1&from=2026-04-22T00:00:00Z&to=2026-04-20T00:00:00Z",
		"/api/v1/visits/generate?device_id=1&min_dwell=bad",
		"/api/v1/visits/generate?device_id=1&max_radius_m=bad",
	}
	for _, path := range cases {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		addCSRF(req, testCSRFToken)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %q, got %d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestGenerateVisitsEndpoint_RequiresValidCSRF(t *testing.T) {
	vs := &fakeVisitStore{created: 1}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{VisitStore: vs})

	reqMissing := httptest.NewRequest(http.MethodPost, "/api/v1/visits/generate?device_id=1", nil)
	recMissing := httptest.NewRecorder()
	mux.ServeHTTP(recMissing, reqMissing)
	if recMissing.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without csrf token, got %d body=%s", recMissing.Code, recMissing.Body.String())
	}

	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/visits/generate?device_id=1", nil)
	reqInvalid.Header.Set("X-CSRF-Token", "csrf-header")
	reqInvalid.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-cookie"})
	recInvalid := httptest.NewRecorder()
	mux.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusForbidden {
		t.Fatalf("expected 403 with mismatched csrf token, got %d body=%s", recInvalid.Code, recInvalid.Body.String())
	}

	reqValid := httptest.NewRequest(http.MethodPost, "/api/v1/visits/generate?device_id=1", nil)
	addCSRF(reqValid, testCSRFToken)
	recValid := httptest.NewRecorder()
	mux.ServeHTTP(recValid, reqValid)
	if recValid.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid csrf token, got %d body=%s", recValid.Code, recValid.Body.String())
	}
}

func TestListVisitsEndpoint(t *testing.T) {
	vs := &fakeVisitStore{
		list: []store.Visit{
			{
				ID:          1,
				DeviceRowID: 1,
				DeviceID:    "phone-main",
				StartAt:     time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC),
				EndAt:       time.Date(2026, 4, 22, 10, 20, 0, 0, time.UTC),
				CentroidLat: 41.1,
				CentroidLon: -87.1,
				PointCount:  5,
			},
		},
	}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{VisitStore: vs})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/visits?device_id=1&from=2026-04-22T00:00:00Z&to=2026-04-23T00:00:00Z&limit=7", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if vs.lastDeviceID == nil || *vs.lastDeviceID != 1 || vs.lastLimit != 7 {
		t.Fatalf("unexpected visit list call: device=%+v limit=%d", vs.lastDeviceID, vs.lastLimit)
	}
	if vs.lastFromUTC == nil || vs.lastToUTC == nil {
		t.Fatalf("expected from/to filters passed to store, got from=%v to=%v", vs.lastFromUTC, vs.lastToUTC)
	}
}

func TestListVisitsEndpoint_InvalidParams(t *testing.T) {
	vs := &fakeVisitStore{}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{VisitStore: vs})

	cases := []string{
		"/api/v1/visits?from=bad",
		"/api/v1/visits?to=bad",
		"/api/v1/visits?from=2026-04-22T00:00:00Z&to=2026-04-21T00:00:00Z",
		"/api/v1/visits?limit=0",
		"/api/v1/visits?device_id=bad",
	}
	for _, path := range cases {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %q, got %d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestListVisitsEndpoint_WithVisitLabelResolver(t *testing.T) {
	vs := &fakeVisitStore{
		list: []store.Visit{
			{
				ID:          1,
				DeviceRowID: 1,
				DeviceID:    "phone-main",
				StartAt:     time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC),
				EndAt:       time.Date(2026, 4, 22, 10, 20, 0, 0, time.UTC),
				CentroidLat: 41.1,
				CentroidLon: -87.1,
				PointCount:  5,
			},
			{
				ID:          2,
				DeviceRowID: 1,
				DeviceID:    "phone-main",
				StartAt:     time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
				EndAt:       time.Date(2026, 4, 22, 12, 30, 0, 0, time.UTC),
				CentroidLat: 41.2,
				CentroidLon: -87.2,
				PointCount:  6,
			},
		},
	}
	resolver := &fakeVisitLabelResolver{
		enabled:    true,
		maxLookups: 1,
		label:      "Cached Place",
	}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{
		VisitStore:         vs,
		VisitLabelResolver: resolver,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/visits?device_id=1&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if resolver.calls != 2 {
		t.Fatalf("expected resolver called for each visit, got %d", resolver.calls)
	}
	if resolver.providerCalls != 1 {
		t.Fatalf("expected provider budget of 1 call, got %d", resolver.providerCalls)
	}

	var resp listVisitsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(resp.Visits) != 2 {
		t.Fatalf("expected 2 visits, got %d", len(resp.Visits))
	}
	if resp.Visits[0].PlaceLabel != "Cached Place" {
		t.Fatalf("expected first visit place label, got %q", resp.Visits[0].PlaceLabel)
	}
	if resp.Visits[1].PlaceLabel != "" {
		t.Fatalf("expected second visit without label after budget exhaustion, got %q", resp.Visits[1].PlaceLabel)
	}
}

func TestListVisitsEndpoint_UserSeesOnlyOwnVisits_WhenSessionAuthEnabled(t *testing.T) {
	vs := &fakeVisitStore{
		list: []store.Visit{
			{
				ID:          1,
				DeviceRowID: 1,
				DeviceID:    "u1-phone",
				StartAt:     time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC),
				EndAt:       time.Date(2026, 4, 22, 10, 20, 0, 0, time.UTC),
				CentroidLat: 41.1,
				CentroidLon: -87.1,
				PointCount:  5,
			},
			{
				ID:          2,
				DeviceRowID: 2,
				DeviceID:    "u2-phone",
				StartAt:     time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
				EndAt:       time.Date(2026, 4, 22, 12, 30, 0, 0, time.UTC),
				CentroidLat: 42.2,
				CentroidLon: -88.2,
				PointCount:  6,
			},
		},
		deviceOwners: map[int64]int64{
			1: 10,
			2: 11,
		},
	}
	deviceStore := &fakeDeviceStore{
		devices: []store.Device{
			{ID: 1, UserID: 10, Name: "u1-phone", SourceType: "owntracks", APIKey: "k1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
			{ID: 2, UserID: 11, Name: "u2-phone", SourceType: "owntracks", APIKey: "k2", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{
		VisitStore:   vs,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/visits?limit=10", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-u1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp listVisitsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(resp.Visits) != 1 || resp.Visits[0].DeviceID != 1 || resp.Visits[0].DeviceName != "u1-phone" {
		t.Fatalf("expected only user1 visits, got %+v", resp.Visits)
	}
}

func TestGenerateVisitsEndpoint_CrossUserDeviceDenied_WhenSessionAuthEnabled(t *testing.T) {
	vs := &fakeVisitStore{created: 2}
	deviceStore := &fakeDeviceStore{
		devices: []store.Device{
			{ID: 1, UserID: 10, Name: "u1-phone", SourceType: "owntracks", APIKey: "k1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
			{ID: 2, UserID: 11, Name: "u2-phone", SourceType: "owntracks", APIKey: "k2", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{
		VisitStore:   vs,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/visits/generate?device_id=2", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-u1"})
	addCSRF(req, testCSRFToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	if vs.lastDeviceID != nil {
		t.Fatalf("expected store not called for denied device, got %+v", vs.lastDeviceID)
	}
}

func TestGenerateVisitsEndpoint_SameDeviceNameAcrossUsers_IsScopedByDeviceRowID(t *testing.T) {
	vs := &fakeVisitStore{created: 2}
	deviceStore := &fakeDeviceStore{
		devices: []store.Device{
			{ID: 100, UserID: 10, Name: "phone", SourceType: "owntracks", APIKey: "k1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
			{ID: 200, UserID: 11, Name: "phone", SourceType: "owntracks", APIKey: "k2", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{
		VisitStore:   vs,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	reqDenied := httptest.NewRequest(http.MethodPost, "/api/v1/visits/generate?device_id=200", nil)
	reqDenied.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-u1"})
	addCSRF(reqDenied, testCSRFToken)
	recDenied := httptest.NewRecorder()
	mux.ServeHTTP(recDenied, reqDenied)
	if recDenied.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-user same-name device, got %d body=%s", recDenied.Code, recDenied.Body.String())
	}

	reqAllowed := httptest.NewRequest(http.MethodPost, "/api/v1/visits/generate?device_id=100", nil)
	reqAllowed.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-u1"})
	addCSRF(reqAllowed, testCSRFToken)
	recAllowed := httptest.NewRecorder()
	mux.ServeHTTP(recAllowed, reqAllowed)
	if recAllowed.Code != http.StatusOK {
		t.Fatalf("expected 200 for own same-name device, got %d body=%s", recAllowed.Code, recAllowed.Body.String())
	}
	if vs.lastDeviceID == nil || *vs.lastDeviceID != 100 {
		t.Fatalf("expected visit generation for user-owned row id 100, got %+v", vs.lastDeviceID)
	}
}

func TestListVisitsEndpoint_SameDeviceNameAcrossUsers_IsScopedBySessionUser(t *testing.T) {
	vs := &fakeVisitStore{
		list: []store.Visit{
			{
				ID:          1,
				DeviceRowID: 100,
				DeviceID:    "phone",
				StartAt:     time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC),
				EndAt:       time.Date(2026, 4, 22, 10, 20, 0, 0, time.UTC),
				CentroidLat: 41.1,
				CentroidLon: -87.1,
				PointCount:  5,
			},
			{
				ID:          2,
				DeviceRowID: 200,
				DeviceID:    "phone",
				StartAt:     time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
				EndAt:       time.Date(2026, 4, 22, 12, 30, 0, 0, time.UTC),
				CentroidLat: 42.2,
				CentroidLon: -88.2,
				PointCount:  6,
			},
		},
		deviceOwners: map[int64]int64{
			100: 10,
			200: 11,
		},
	}
	deviceStore := &fakeDeviceStore{
		devices: []store.Device{
			{ID: 100, UserID: 10, Name: "phone", SourceType: "owntracks", APIKey: "k1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
			{ID: 200, UserID: 11, Name: "phone", SourceType: "owntracks", APIKey: "k2", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{
		VisitStore:   vs,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/visits?limit=10", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-u1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp listVisitsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(resp.Visits) != 1 || resp.Visits[0].DeviceID != 100 || resp.Visits[0].DeviceName != "phone" {
		t.Fatalf("expected only user1 same-name device visits, got %+v", resp.Visits)
	}
}

func TestVisitsEndpoints_UnauthenticatedDenied_WhenSessionAuthEnabled(t *testing.T) {
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{
		VisitStore:   &fakeVisitStore{},
		DeviceStore:  &fakeDeviceStore{},
		UserStore:    &fakeUserStore{users: map[int64]store.User{}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{}},
	})

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/visits", nil)
	recList := httptest.NewRecorder()
	mux.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for list, got %d body=%s", recList.Code, recList.Body.String())
	}

	reqGenerate := httptest.NewRequest(http.MethodPost, "/api/v1/visits/generate?device_id=1", nil)
	recGenerate := httptest.NewRecorder()
	mux.ServeHTTP(recGenerate, reqGenerate)
	if recGenerate.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for generate, got %d body=%s", recGenerate.Code, recGenerate.Body.String())
	}
}
