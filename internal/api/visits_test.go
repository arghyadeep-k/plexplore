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
	lastDeviceID string
	lastFromUTC  *time.Time
	lastToUTC    *time.Time
	lastConfig   visits.Config
	lastLimit    int
	created      int
	list         []store.Visit
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

func (f *fakeVisitStore) RebuildVisitsForDeviceRange(_ context.Context, deviceID string, fromUTC, toUTC *time.Time, cfg visits.Config) (int, error) {
	f.lastDeviceID = deviceID
	f.lastFromUTC = fromUTC
	f.lastToUTC = toUTC
	f.lastConfig = cfg
	return f.created, nil
}

func (f *fakeVisitStore) ListVisits(_ context.Context, deviceID string, fromUTC, toUTC *time.Time, limit int) ([]store.Visit, error) {
	f.lastDeviceID = deviceID
	f.lastFromUTC = fromUTC
	f.lastToUTC = toUTC
	f.lastLimit = limit
	out := make([]store.Visit, len(f.list))
	copy(out, f.list)
	return out, nil
}

func TestGenerateVisitsEndpoint_DeviceAndRange(t *testing.T) {
	vs := &fakeVisitStore{created: 2}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{VisitStore: vs})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/visits/generate?device_id=phone-main&from=2026-04-20T00:00:00Z&to=2026-04-22T00:00:00Z&min_dwell=10m&max_radius_m=25", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if vs.lastDeviceID != "phone-main" {
		t.Fatalf("expected device_id phone-main, got %q", vs.lastDeviceID)
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
	RegisterRoutesWithDependencies(mux, Dependencies{VisitStore: vs})

	cases := []string{
		"/api/v1/visits/generate",
		"/api/v1/visits/generate?device_id=d1&from=bad",
		"/api/v1/visits/generate?device_id=d1&from=2026-04-22T00:00:00Z&to=2026-04-20T00:00:00Z",
		"/api/v1/visits/generate?device_id=d1&min_dwell=bad",
		"/api/v1/visits/generate?device_id=d1&max_radius_m=bad",
	}
	for _, path := range cases {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %q, got %d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestListVisitsEndpoint(t *testing.T) {
	vs := &fakeVisitStore{
		list: []store.Visit{
			{
				ID:          1,
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
	RegisterRoutesWithDependencies(mux, Dependencies{VisitStore: vs})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/visits?device_id=phone-main&from=2026-04-22T00:00:00Z&to=2026-04-23T00:00:00Z&limit=7", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if vs.lastDeviceID != "phone-main" || vs.lastLimit != 7 {
		t.Fatalf("unexpected visit list call: device=%q limit=%d", vs.lastDeviceID, vs.lastLimit)
	}
	if vs.lastFromUTC == nil || vs.lastToUTC == nil {
		t.Fatalf("expected from/to filters passed to store, got from=%v to=%v", vs.lastFromUTC, vs.lastToUTC)
	}
}

func TestListVisitsEndpoint_InvalidParams(t *testing.T) {
	vs := &fakeVisitStore{}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{VisitStore: vs})

	cases := []string{
		"/api/v1/visits?from=bad",
		"/api/v1/visits?to=bad",
		"/api/v1/visits?from=2026-04-22T00:00:00Z&to=2026-04-21T00:00:00Z",
		"/api/v1/visits?limit=0",
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
				DeviceID:    "phone-main",
				StartAt:     time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC),
				EndAt:       time.Date(2026, 4, 22, 10, 20, 0, 0, time.UTC),
				CentroidLat: 41.1,
				CentroidLon: -87.1,
				PointCount:  5,
			},
			{
				ID:          2,
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
	RegisterRoutesWithDependencies(mux, Dependencies{
		VisitStore:         vs,
		VisitLabelResolver: resolver,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/visits?device_id=phone-main&limit=10", nil)
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
				DeviceID:    "u1-phone",
				StartAt:     time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC),
				EndAt:       time.Date(2026, 4, 22, 10, 20, 0, 0, time.UTC),
				CentroidLat: 41.1,
				CentroidLon: -87.1,
				PointCount:  5,
			},
			{
				ID:          2,
				DeviceID:    "u2-phone",
				StartAt:     time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
				EndAt:       time.Date(2026, 4, 22, 12, 30, 0, 0, time.UTC),
				CentroidLat: 42.2,
				CentroidLon: -88.2,
				PointCount:  6,
			},
		},
	}
	deviceStore := &fakeDeviceStore{
		devices: []store.Device{
			{ID: 1, UserID: 10, Name: "u1-phone", SourceType: "owntracks", APIKey: "k1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
			{ID: 2, UserID: 11, Name: "u2-phone", SourceType: "owntracks", APIKey: "k2", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
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
	if len(resp.Visits) != 1 || resp.Visits[0].DeviceID != "u1-phone" {
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
	RegisterRoutesWithDependencies(mux, Dependencies{
		VisitStore:   vs,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/visits/generate?device_id=u2-phone", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-u1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	if vs.lastDeviceID != "" {
		t.Fatalf("expected store not called for denied device, got %q", vs.lastDeviceID)
	}
}

func TestVisitsEndpoints_UnauthenticatedDenied_WhenSessionAuthEnabled(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
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

	reqGenerate := httptest.NewRequest(http.MethodPost, "/api/v1/visits/generate?device_id=u1-phone", nil)
	recGenerate := httptest.NewRecorder()
	mux.ServeHTTP(recGenerate, reqGenerate)
	if recGenerate.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for generate, got %d body=%s", recGenerate.Code, recGenerate.Body.String())
	}
}
