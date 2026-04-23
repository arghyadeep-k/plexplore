package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"plexplore/internal/store"
)

type fakePointStore struct {
	lastDeviceID     string
	lastLimit        int
	points           []store.RecentPoint
	lastExportFilter store.ExportPointFilter
	lastPointsFilter store.ExportPointFilter
}

func (f *fakePointStore) ListRecentPoints(_ context.Context, deviceID string, limit int) ([]store.RecentPoint, error) {
	f.lastDeviceID = deviceID
	f.lastLimit = limit
	out := make([]store.RecentPoint, len(f.points))
	copy(out, f.points)
	return out, nil
}

func (f *fakePointStore) ListPointsForExport(_ context.Context, filter store.ExportPointFilter) ([]store.RecentPoint, error) {
	f.lastExportFilter = filter
	out := make([]store.RecentPoint, len(f.points))
	copy(out, f.points)
	return out, nil
}

func (f *fakePointStore) ListPoints(_ context.Context, filter store.ExportPointFilter, limit int) ([]store.RecentPoint, error) {
	f.lastPointsFilter = filter
	f.lastLimit = limit
	out := make([]store.RecentPoint, len(f.points))
	copy(out, f.points)
	return out, nil
}

func TestPointsEndpoint_DefaultQuery(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{
				Seq:          1,
				DeviceID:     "phone-main",
				SourceType:   "owntracks",
				TimestampUTC: time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
				Lat:          41.1,
				Lon:          -87.1,
			},
		},
	}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if pointStore.lastLimit != 500 {
		t.Fatalf("expected default limit 500, got %d", pointStore.lastLimit)
	}
	if pointStore.lastPointsFilter.DeviceID != "" || pointStore.lastPointsFilter.FromUTC != nil || pointStore.lastPointsFilter.ToUTC != nil {
		t.Fatalf("expected empty default filter, got %+v", pointStore.lastPointsFilter)
	}
}

func TestPointsEndpoint_RangeFiltering(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points?from=2026-04-22T11:00:00Z&to=2026-04-22T13:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if pointStore.lastPointsFilter.FromUTC == nil || pointStore.lastPointsFilter.ToUTC == nil {
		t.Fatalf("expected from/to filters set, got %+v", pointStore.lastPointsFilter)
	}
}

func TestPointsEndpoint_DeviceFiltering(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points?device_id=phone-main&limit=20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if pointStore.lastPointsFilter.DeviceID != "phone-main" {
		t.Fatalf("expected device filter phone-main, got %+v", pointStore.lastPointsFilter)
	}
	if pointStore.lastLimit != 20 {
		t.Fatalf("expected limit=20, got %d", pointStore.lastLimit)
	}
}

func TestPointsEndpoint_InvalidQueryParams(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{PointStore: pointStore})

	cases := []string{
		"/api/v1/points?from=not-a-time",
		"/api/v1/points?to=not-a-time",
		"/api/v1/points?limit=bad",
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

func TestRecentPointsEndpoint_DefaultLimitAndShape(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{
				Seq:          10,
				DeviceID:     "phone-main",
				SourceType:   "owntracks",
				TimestampUTC: time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
				Lat:          41.1,
				Lon:          -87.1,
			},
		},
	}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points/recent", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if pointStore.lastLimit != 50 {
		t.Fatalf("expected default limit 50, got %d", pointStore.lastLimit)
	}

	var resp recentPointsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(resp.Points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(resp.Points))
	}
	if resp.Points[0].DeviceID != "phone-main" || resp.Points[0].Seq != 10 {
		t.Fatalf("unexpected point payload: %+v", resp.Points[0])
	}
}

func TestRecentPointsEndpoint_DeviceFilterAndLimit(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points/recent?device_id=phone-main&limit=7", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if pointStore.lastDeviceID != "phone-main" {
		t.Fatalf("expected device_id filter phone-main, got %q", pointStore.lastDeviceID)
	}
	if pointStore.lastLimit != 7 {
		t.Fatalf("expected limit=7, got %d", pointStore.lastLimit)
	}
}

func TestRecentPointsEndpoint_InvalidLimit(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points/recent?limit=abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRecentPointsEndpoint_UserSeesOnlyOwnPoints_WhenSessionAuthEnabled(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{Seq: 1, UserID: 10, DeviceID: "u1-phone", SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 41.0, Lon: -87.0},
			{Seq: 2, UserID: 11, DeviceID: "u2-phone", SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 42.0, Lon: -88.0},
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
		PointStore:   pointStore,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points/recent?limit=20", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-u1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp recentPointsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(resp.Points) != 1 || resp.Points[0].DeviceID != "u1-phone" {
		t.Fatalf("expected only user1 points, got %+v", resp.Points)
	}
}

func TestRecentPointsEndpoint_DeviceFilterTrickBlocked_WhenSessionAuthEnabled(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{Seq: 2, UserID: 11, DeviceID: "u2-phone", SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 42.0, Lon: -88.0},
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
		PointStore:   pointStore,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points/recent?device_id=u2-phone&limit=20", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-u1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp recentPointsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(resp.Points) != 0 {
		t.Fatalf("expected zero points for cross-user device filter trick, got %+v", resp.Points)
	}
}

func TestRecentPointsEndpoint_UnauthenticatedDenied_WhenSessionAuthEnabled(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		PointStore:   &fakePointStore{},
		DeviceStore:  &fakeDeviceStore{},
		UserStore:    &fakeUserStore{users: map[int64]store.User{}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points/recent", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPointsEndpoint_UserSeesOnlyOwnPoints_WhenSessionAuthEnabled(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{Seq: 1, UserID: 10, DeviceID: "u1-phone", SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 41.0, Lon: -87.0},
			{Seq: 2, UserID: 11, DeviceID: "u2-phone", SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 42.0, Lon: -88.0},
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
		PointStore:   pointStore,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points?limit=20", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-u1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp recentPointsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(resp.Points) != 1 || resp.Points[0].DeviceID != "u1-phone" {
		t.Fatalf("expected only user1 points, got %+v", resp.Points)
	}
}

func TestPointsEndpoint_DeviceFilterTrickBlocked_WhenSessionAuthEnabled(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{Seq: 2, UserID: 11, DeviceID: "u2-phone", SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 42.0, Lon: -88.0},
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
		PointStore:   pointStore,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points?device_id=u2-phone&limit=20", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-u1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp recentPointsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(resp.Points) != 0 {
		t.Fatalf("expected zero points for cross-user device filter trick, got %+v", resp.Points)
	}
}

func TestPointsEndpoint_UnauthenticatedDenied_WhenSessionAuthEnabled(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		PointStore:   &fakePointStore{},
		DeviceStore:  &fakeDeviceStore{},
		UserStore:    &fakeUserStore{users: map[int64]store.User{}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}
