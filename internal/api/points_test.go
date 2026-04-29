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
	lastDeviceID     *int64
	lastLimit        int
	points           []store.RecentPoint
	lastExportFilter store.ExportPointFilter
	lastPointsFilter store.ExportPointFilter
	streamCalled     bool
	streamErr        error
	streamCallCount  int
}

func (f *fakePointStore) ListRecentPoints(_ context.Context, deviceID *int64, limit int) ([]store.RecentPoint, error) {
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

func (f *fakePointStore) StreamPointsForExport(_ context.Context, filter store.ExportPointFilter, limit int, fn func(store.RecentPoint) error) (int, error) {
	f.streamCallCount++
	if f.streamErr != nil {
		return 0, f.streamErr
	}
	f.streamCalled = true
	f.lastExportFilter = filter
	f.lastLimit = limit
	count := 0
	for _, p := range f.points {
		if filter.UserID > 0 && p.UserID != filter.UserID {
			continue
		}
		if filter.DeviceRowID != nil && p.DeviceID != *filter.DeviceRowID {
			continue
		}
		if filter.FromUTC != nil && p.TimestampUTC.Before(*filter.FromUTC) {
			continue
		}
		if filter.ToUTC != nil && p.TimestampUTC.After(*filter.ToUTC) {
			continue
		}
		if filter.AfterSeq > 0 && p.Seq <= filter.AfterSeq {
			continue
		}
		if count >= limit {
			break
		}
		if err := fn(p); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (f *fakePointStore) ListPoints(_ context.Context, filter store.ExportPointFilter, limit int) ([]store.RecentPoint, error) {
	f.lastPointsFilter = filter
	f.lastLimit = limit
	out := make([]store.RecentPoint, 0, len(f.points))
	for _, p := range f.points {
		if filter.UserID > 0 && p.UserID != filter.UserID {
			continue
		}
		if filter.DeviceRowID != nil && p.DeviceID != *filter.DeviceRowID {
			continue
		}
		if filter.FromUTC != nil && p.TimestampUTC.Before(*filter.FromUTC) {
			continue
		}
		if filter.ToUTC != nil && p.TimestampUTC.After(*filter.ToUTC) {
			continue
		}
		if filter.AfterSeq > 0 && p.Seq <= filter.AfterSeq {
			continue
		}
		out = append(out, p)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func TestPointsEndpoint_DefaultQuery(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{
				Seq:          1,
				DeviceID:     1,
				SourceType:   "owntracks",
				TimestampUTC: time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
				Lat:          41.1,
				Lon:          -87.1,
			},
		},
	}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if pointStore.lastLimit != defaultPointsLimit+1 {
		t.Fatalf("expected query limit 501 (default+1 for pagination check), got %d", pointStore.lastLimit)
	}
	if pointStore.lastPointsFilter.DeviceRowID != nil || pointStore.lastPointsFilter.FromUTC != nil || pointStore.lastPointsFilter.ToUTC != nil {
		t.Fatalf("expected empty default filter, got %+v", pointStore.lastPointsFilter)
	}
}

func TestPointsEndpoint_RangeFiltering(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

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
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points?device_id=123&limit=20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if pointStore.lastPointsFilter.DeviceRowID == nil || *pointStore.lastPointsFilter.DeviceRowID != 123 {
		t.Fatalf("expected device filter 123, got %+v", pointStore.lastPointsFilter)
	}
	if pointStore.lastLimit != 21 {
		t.Fatalf("expected query limit=21 (limit+1), got %d", pointStore.lastLimit)
	}
}

func TestPointsEndpoint_InvalidQueryParams(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

	cases := []string{
		"/api/v1/points?from=not-a-time",
		"/api/v1/points?to=not-a-time",
		"/api/v1/points?limit=bad",
		"/api/v1/points?cursor=bad",
		"/api/v1/points?simplify=true&max_points=bad",
		"/api/v1/points?device_id=phone-main",
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

func TestPointsEndpoint_LimitCapApplied(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points?limit=999999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if pointStore.lastLimit != maxPointsLimit+1 {
		t.Fatalf("expected capped query limit %d, got %d", maxPointsLimit+1, pointStore.lastLimit)
	}
}

func TestPointsEndpoint_PaginationCursor(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{Seq: 1, UserID: 1, DeviceID: 1, SourceType: "owntracks", TimestampUTC: time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC), Lat: 1, Lon: 1},
			{Seq: 2, UserID: 1, DeviceID: 1, SourceType: "owntracks", TimestampUTC: time.Date(2026, 4, 22, 12, 1, 0, 0, time.UTC), Lat: 2, Lon: 2},
			{Seq: 3, UserID: 1, DeviceID: 1, SourceType: "owntracks", TimestampUTC: time.Date(2026, 4, 22, 12, 2, 0, 0, time.UTC), Lat: 3, Lon: 3},
		},
	}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{
		PointStore:   pointStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{1: {ID: 1, Email: "u@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"tok": {Token: "tok", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points?limit=2", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var first pointsPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("unmarshal first page failed: %v", err)
	}
	if len(first.Points) != 2 || first.NextCursor == nil || *first.NextCursor != 2 {
		t.Fatalf("unexpected first page %+v", first)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/points?limit=2&cursor=2", nil)
	req2.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok"})
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec2.Code, rec2.Body.String())
	}
	var second pointsPageResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &second); err != nil {
		t.Fatalf("unmarshal second page failed: %v", err)
	}
	if len(second.Points) != 1 || second.Points[0].Seq != 3 {
		t.Fatalf("unexpected second page %+v", second)
	}
	if second.NextCursor != nil {
		t.Fatalf("expected no next cursor on last page, got %+v", second.NextCursor)
	}
}

func TestPointsEndpoint_SimplifyReducesLargeResponse(t *testing.T) {
	points := make([]store.RecentPoint, 0, 3000)
	base := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	for i := 1; i <= 3000; i++ {
		points = append(points, store.RecentPoint{
			Seq:          uint64(i),
			UserID:       1,
			DeviceID:     1,
			SourceType:   "owntracks",
			TimestampUTC: base.Add(time.Duration(i) * time.Second),
			Lat:          40.0 + float64(i)*0.0001,
			Lon:          -87.0 - float64(i)*0.0001,
		})
	}

	pointStore := &fakePointStore{points: points}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{
		PointStore:   pointStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{1: {ID: 1, Email: "u@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"tok": {Token: "tok", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points?simplify=true&limit=999999&max_points=200", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp pointsPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal simplify response failed: %v", err)
	}
	if len(resp.Points) != 200 {
		t.Fatalf("expected 200 sampled points, got %d", len(resp.Points))
	}
	if !resp.Sampled || resp.SampledFrom == 0 {
		t.Fatalf("expected sampled metadata, got %+v", resp)
	}
	if pointStore.lastLimit != maxSimplifiedPointsLimit+1 {
		t.Fatalf("expected simplified capped query limit %d, got %d", maxSimplifiedPointsLimit+1, pointStore.lastLimit)
	}
}

func TestRecentPointsEndpoint_DefaultLimitAndShape(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{
				Seq:          10,
				DeviceID:     10,
				SourceType:   "owntracks",
				TimestampUTC: time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
				Lat:          41.1,
				Lon:          -87.1,
			},
		},
	}

	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

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
	if resp.Points[0].DeviceID != 10 || resp.Points[0].Seq != 10 {
		t.Fatalf("unexpected point payload: %+v", resp.Points[0])
	}
}

func TestRecentPointsEndpoint_DeviceFilterAndLimit(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points/recent?device_id=7&limit=7", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if pointStore.lastDeviceID == nil || *pointStore.lastDeviceID != 7 {
		t.Fatalf("expected device_id filter 7, got %+v", pointStore.lastDeviceID)
	}
	if pointStore.lastLimit != 7 {
		t.Fatalf("expected limit=7, got %d", pointStore.lastLimit)
	}
}

func TestRecentPointsEndpoint_InvalidLimit(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points/recent?limit=abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRecentPointsEndpoint_InvalidDeviceID(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points/recent?device_id=phone-main", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPointsEndpoint_SameNameDevices_FilteredByNumericDeviceID(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{Seq: 1, UserID: 10, DeviceID: 101, SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 41.0, Lon: -87.0},
			{Seq: 2, UserID: 10, DeviceID: 102, SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 42.0, Lon: -88.0},
		},
	}
	deviceStore := &fakeDeviceStore{
		devices: []store.Device{
			{ID: 101, UserID: 10, Name: "phone-main", SourceType: "owntracks", APIKey: "k1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
			{ID: 102, UserID: 10, Name: "phone-main", SourceType: "owntracks", APIKey: "k2", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{
		PointStore:   pointStore,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points?device_id=101&limit=20", nil)
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
	if len(resp.Points) != 1 || resp.Points[0].DeviceID != 101 {
		t.Fatalf("expected only device_id=101, got %+v", resp.Points)
	}
}

func TestRecentPointsEndpoint_UserSeesOnlyOwnPoints_WhenSessionAuthEnabled(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{Seq: 1, UserID: 10, DeviceID: 1, SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 41.0, Lon: -87.0},
			{Seq: 2, UserID: 11, DeviceID: 2, SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 42.0, Lon: -88.0},
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
	if len(resp.Points) != 1 || resp.Points[0].DeviceID != 1 {
		t.Fatalf("expected only user1 points, got %+v", resp.Points)
	}
}

func TestRecentPointsEndpoint_DeviceFilterTrickBlocked_WhenSessionAuthEnabled(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{Seq: 2, UserID: 11, DeviceID: 2, SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 42.0, Lon: -88.0},
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
		PointStore:   pointStore,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points/recent?device_id=2&limit=20", nil)
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
	registerRoutesWithTestFallbacks(mux, Dependencies{
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
			{Seq: 1, UserID: 10, DeviceID: 1, SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 41.0, Lon: -87.0},
			{Seq: 2, UserID: 11, DeviceID: 2, SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 42.0, Lon: -88.0},
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
	if len(resp.Points) != 1 || resp.Points[0].DeviceID != 1 {
		t.Fatalf("expected only user1 points, got %+v", resp.Points)
	}
}

func TestPointsEndpoint_DeviceFilterTrickBlocked_WhenSessionAuthEnabled(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{Seq: 2, UserID: 11, DeviceID: 2, SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 42.0, Lon: -88.0},
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
		PointStore:   pointStore,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/points?device_id=2&limit=20", nil)
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
	registerRoutesWithTestFallbacks(mux, Dependencies{
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
