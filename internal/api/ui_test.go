package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatusPageServedAtRoot(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
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
	if !strings.Contains(body, "Recent Points") {
		t.Fatalf("expected recent points section in body, got %q", body)
	}
}

func TestStatusPage_DoesNotMatchTypoPath(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/ui/statu", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for typo path, got %d", rec.Code)
	}
}

func TestMapPageServedAtUIMap(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/ui/map", nil)
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
	if !strings.Contains(strings.ToLower(body), "leaflet") {
		t.Fatalf("expected leaflet assets in body, got %q", body)
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
}
