package api

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"plexplore/internal/store"
)

func TestGeoJSONExport_ValidStructure(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{
				Seq:          5,
				DeviceID:     "phone-main",
				SourceType:   "owntracks",
				TimestampUTC: time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
				Lat:          41.25,
				Lon:          -87.75,
			},
		},
	}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exports/geojson?device_id=phone-main&from=2026-04-22T11:00:00Z&to=2026-04-22T13:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if pointStore.lastExportFilter.DeviceID != "phone-main" {
		t.Fatalf("expected device filter phone-main, got %+v", pointStore.lastExportFilter)
	}
	if pointStore.lastExportFilter.FromUTC == nil || pointStore.lastExportFilter.ToUTC == nil {
		t.Fatalf("expected from/to filters set, got %+v", pointStore.lastExportFilter)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal geojson response failed: %v", err)
	}
	if payload["type"] != "FeatureCollection" {
		t.Fatalf("expected FeatureCollection, got %+v", payload)
	}

	features, ok := payload["features"].([]interface{})
	if !ok || len(features) != 1 {
		t.Fatalf("expected one feature, got %+v", payload["features"])
	}
	feature, ok := features[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected feature object, got %+v", features[0])
	}
	if feature["type"] != "Feature" {
		t.Fatalf("expected feature type Feature, got %+v", feature)
	}
	geometry, ok := feature["geometry"].(map[string]interface{})
	if !ok || geometry["type"] != "Point" {
		t.Fatalf("expected point geometry, got %+v", feature["geometry"])
	}
	coords, ok := geometry["coordinates"].([]interface{})
	if !ok || len(coords) != 2 {
		t.Fatalf("expected coordinate pair, got %+v", geometry["coordinates"])
	}
}

func TestGeoJSONExport_InvalidTimestampQuery(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exports/geojson?from=bad-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGPXExport_ValidStructureAndContent(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{
				Seq:          11,
				DeviceID:     "phone-main",
				SourceType:   "owntracks",
				TimestampUTC: time.Date(2026, 4, 22, 12, 30, 0, 0, time.UTC),
				Lat:          41.2,
				Lon:          -87.2,
			},
			{
				Seq:          12,
				DeviceID:     "phone-main",
				SourceType:   "owntracks",
				TimestampUTC: time.Date(2026, 4, 22, 12, 31, 0, 0, time.UTC),
				Lat:          41.21,
				Lon:          -87.21,
			},
		},
	}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exports/gpx?device_id=phone-main", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/gpx+xml") {
		t.Fatalf("expected GPX content type, got %q", rec.Header().Get("Content-Type"))
	}

	body := rec.Body.String()
	if !strings.Contains(body, "<gpx") || !strings.Contains(body, "<trkpt") {
		t.Fatalf("expected gpx/trkpt elements in body: %s", body)
	}

	var parsed gpxDocument
	if err := xml.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("xml unmarshal failed: %v", err)
	}
	if parsed.XMLName.Local != "gpx" {
		t.Fatalf("expected gpx root, got %+v", parsed.XMLName)
	}
	if len(parsed.Track.Segment.Points) != 2 {
		t.Fatalf("expected 2 track points, got %d", len(parsed.Track.Segment.Points))
	}
	if parsed.Track.Segment.Points[0].Lat != 41.2 || parsed.Track.Segment.Points[0].Lon != -87.2 {
		t.Fatalf("unexpected first track point %+v", parsed.Track.Segment.Points[0])
	}
}

func TestGPXExport_InvalidTimestampQuery(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exports/gpx?to=bad-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
