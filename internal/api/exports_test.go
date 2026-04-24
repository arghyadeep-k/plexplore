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
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exports/geojson?device_id=phone-main&from=2026-04-22T11:00:00Z&to=2026-04-22T13:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/geo+json") {
		t.Fatalf("expected geojson content type, got %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "plexplore-export.geojson") {
		t.Fatalf("expected download filename header, got %q", rec.Header().Get("Content-Disposition"))
	}
	if pointStore.lastExportFilter.DeviceID != "phone-main" {
		t.Fatalf("expected device filter phone-main, got %+v", pointStore.lastExportFilter)
	}
	if !pointStore.streamCalled {
		t.Fatalf("expected streamed export path to be used")
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
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

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
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exports/gpx?device_id=phone-main", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/gpx+xml") {
		t.Fatalf("expected GPX content type, got %q", rec.Header().Get("Content-Type"))
	}
	if !pointStore.streamCalled {
		t.Fatalf("expected streamed export path to be used")
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "plexplore-export.gpx") {
		t.Fatalf("expected GPX download filename header, got %q", rec.Header().Get("Content-Disposition"))
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

func TestExportEndpoints_LimitCapApplied(t *testing.T) {
	pointStore := &fakePointStore{
		points: []store.RecentPoint{
			{Seq: 1, DeviceID: "phone-main", SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 1, Lon: 1},
		},
	}

	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

	reqGeo := httptest.NewRequest(http.MethodGet, "/api/v1/exports/geojson?limit=999999", nil)
	recGeo := httptest.NewRecorder()
	mux.ServeHTTP(recGeo, reqGeo)
	if recGeo.Code != http.StatusOK {
		t.Fatalf("expected geojson 200, got %d body=%s", recGeo.Code, recGeo.Body.String())
	}
	if pointStore.lastLimit != maxExportLimit {
		t.Fatalf("expected geojson cap %d, got %d", maxExportLimit, pointStore.lastLimit)
	}

	reqGPX := httptest.NewRequest(http.MethodGet, "/api/v1/exports/gpx?limit=999999", nil)
	recGPX := httptest.NewRecorder()
	mux.ServeHTTP(recGPX, reqGPX)
	if recGPX.Code != http.StatusOK {
		t.Fatalf("expected gpx 200, got %d body=%s", recGPX.Code, recGPX.Body.String())
	}
	if pointStore.lastLimit != maxExportLimit {
		t.Fatalf("expected gpx cap %d, got %d", maxExportLimit, pointStore.lastLimit)
	}
}

func TestGPXExport_InvalidTimestampQuery(t *testing.T) {
	pointStore := &fakePointStore{}
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{PointStore: pointStore})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exports/gpx?to=bad-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGeoJSONExport_UserSeesOnlyOwnPoints_WhenSessionAuthEnabled(t *testing.T) {
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
	registerRoutesWithTestFallbacks(mux, Dependencies{
		PointStore:   pointStore,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exports/geojson", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-u1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload geoJSONFeatureCollection
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal geojson failed: %v", err)
	}
	if len(payload.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(payload.Features))
	}
	if payload.Features[0].Properties["device_id"] != "u1-phone" {
		t.Fatalf("expected u1-phone only, got %+v", payload.Features[0].Properties)
	}
}

func TestGeoJSONExport_UnauthenticatedDenied_WhenSessionAuthEnabled(t *testing.T) {
	mux := http.NewServeMux()
	registerRoutesWithTestFallbacks(mux, Dependencies{
		PointStore:   &fakePointStore{},
		DeviceStore:  &fakeDeviceStore{},
		UserStore:    &fakeUserStore{users: map[int64]store.User{}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exports/geojson", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGPXExport_DeviceFilterTrickBlocked_WhenSessionAuthEnabled(t *testing.T) {
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
	registerRoutesWithTestFallbacks(mux, Dependencies{
		PointStore:   pointStore,
		DeviceStore:  deviceStore,
		UserStore:    &fakeUserStore{users: map[int64]store.User{10: {ID: 10, Email: "u1@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{"token-u1": {Token: "token-u1", UserID: 10, ExpiresAt: time.Now().UTC().Add(time.Hour)}}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exports/gpx?device_id=u2-phone", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-u1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var parsed gpxDocument
	if err := xml.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("xml unmarshal failed: %v", err)
	}
	if len(parsed.Track.Segment.Points) != 0 {
		t.Fatalf("expected 0 points for cross-user filter trick, got %d", len(parsed.Track.Segment.Points))
	}
}
