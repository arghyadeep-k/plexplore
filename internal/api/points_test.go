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
