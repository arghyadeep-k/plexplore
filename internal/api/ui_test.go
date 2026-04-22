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
