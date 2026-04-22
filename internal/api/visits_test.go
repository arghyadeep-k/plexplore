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
