package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"plexplore/internal/buffer"
	"plexplore/internal/ingest"
	"plexplore/internal/spool"
	"plexplore/internal/store"
)

type fakeSpoolAppender struct {
	nextSeq      uint64
	points       []ingest.CanonicalPoint
	segmentCount int
	checkpoint   spool.Checkpoint
}

func (f *fakeSpoolAppender) AppendCanonicalPoints(points []ingest.CanonicalPoint) ([]ingest.SpoolRecord, error) {
	f.points = append(f.points, points...)
	out := make([]ingest.SpoolRecord, 0, len(points))
	for _, point := range points {
		f.nextSeq++
		out = append(out, ingest.SpoolRecord{
			Seq:        f.nextSeq,
			DeviceID:   point.DeviceID,
			ReceivedAt: time.Now().UTC(),
			Point:      point,
		})
	}
	return out, nil
}

func (f *fakeSpoolAppender) ReadCheckpoint() (spool.Checkpoint, error) {
	return f.checkpoint, nil
}

func (f *fakeSpoolAppender) SegmentCount() (int, error) {
	return f.segmentCount, nil
}

type fakeRecordBuffer struct {
	enqueued []ingest.SpoolRecord
}

func (f *fakeRecordBuffer) Enqueue(records []ingest.SpoolRecord) error {
	f.enqueued = append(f.enqueued, records...)
	return nil
}

func (f *fakeRecordBuffer) Stats() buffer.Stats {
	return buffer.Stats{
		TotalBufferedPoints: len(f.enqueued),
	}
}

func TestIngestOwnTracks_ValidRequest(t *testing.T) {
	ds := &fakeDeviceStore{
		devices: []store.Device{
			{ID: 1, UserID: 1, Name: "phone-main", SourceType: "owntracks", APIKey: "k1"},
		},
	}
	sp := &fakeSpoolAppender{}
	buf := &fakeRecordBuffer{}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		DeviceStore: ds,
		Spool:       sp,
		Buffer:      buf,
	})

	body := []byte(`{"_type":"location","lat":37.42,"lon":-122.08,"tst":1713744000,"tid":"zz","topic":"owntracks/u/dev"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/owntracks", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "k1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	if len(sp.points) != 1 {
		t.Fatalf("expected 1 spooled point, got %d", len(sp.points))
	}
	if sp.points[0].DeviceID != "phone-main" {
		t.Fatalf("expected authenticated device name, got %q", sp.points[0].DeviceID)
	}
	if sp.points[0].IngestHash == "" {
		t.Fatal("expected ingest hash to be set")
	}

	var resp ingestSuccessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !resp.OK || resp.Accepted != 1 || resp.Spooled != 1 || resp.Enqueued != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestIngestOverland_ValidRequest(t *testing.T) {
	ds := &fakeDeviceStore{
		devices: []store.Device{
			{ID: 2, UserID: 1, Name: "ios-main", SourceType: "overland", APIKey: "k2"},
		},
	}
	sp := &fakeSpoolAppender{}
	buf := &fakeRecordBuffer{}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		DeviceStore: ds,
		Spool:       sp,
		Buffer:      buf,
	})

	body := []byte(`{"device_id":"phone-01","locations":[{"coordinates":[-122.08,37.42],"timestamp":"2026-04-21T20:00:00Z"},{"coordinates":[-122.081,37.421],"timestamp":"2026-04-21T20:01:00Z"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/overland/batches", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "k2")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(sp.points) != 2 {
		t.Fatalf("expected 2 spooled points, got %d", len(sp.points))
	}
	for i := range sp.points {
		if sp.points[i].DeviceID != "ios-main" {
			t.Fatalf("point %d expected device ios-main, got %q", i, sp.points[i].DeviceID)
		}
	}
}

func TestIngestOwnTracks_InvalidPayload(t *testing.T) {
	ds := &fakeDeviceStore{
		devices: []store.Device{
			{ID: 1, UserID: 1, Name: "phone-main", SourceType: "owntracks", APIKey: "k1"},
		},
	}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		DeviceStore: ds,
		Spool:       &fakeSpoolAppender{},
		Buffer:      &fakeRecordBuffer{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/owntracks", bytes.NewReader([]byte(`{"_type":"location","lat":1}`)))
	req.Header.Set("X-API-Key", "k1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestIngestOwnTracks_BadAPIKey(t *testing.T) {
	ds := &fakeDeviceStore{
		devices: []store.Device{
			{ID: 1, UserID: 1, Name: "phone-main", SourceType: "owntracks", APIKey: "k1"},
		},
	}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		DeviceStore: ds,
		Spool:       &fakeSpoolAppender{},
		Buffer:      &fakeRecordBuffer{},
	})

	body := []byte(`{"_type":"location","lat":37.42,"lon":-122.08,"tst":1713744000}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/owntracks", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "bad-key")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}
