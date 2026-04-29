package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"plexplore/internal/buffer"
	"plexplore/internal/flusher"
	"plexplore/internal/ingest"
	"plexplore/internal/spool"
	"plexplore/internal/store"
)

type fakeStatusBuffer struct {
	stats buffer.Stats
}

func (f *fakeStatusBuffer) Enqueue(records []ingest.SpoolRecord) error {
	return nil
}

func (f *fakeStatusBuffer) Stats() buffer.Stats {
	return f.stats
}

type fakeStatusSpool struct {
	segmentCount int
	checkpoint   spool.Checkpoint
}

func (f *fakeStatusSpool) AppendCanonicalPoints(points []ingest.CanonicalPoint) ([]ingest.SpoolRecord, error) {
	return nil, nil
}

func (f *fakeStatusSpool) ReadCheckpoint() (spool.Checkpoint, error) {
	return f.checkpoint, nil
}

func (f *fakeStatusSpool) SegmentCount() (int, error) {
	return f.segmentCount, nil
}

type fakeStatusFlusher struct {
	result flusher.LastFlushResult
	has    bool
}

type fakeStatusVisitScheduler struct {
	status VisitSchedulerStatusSnapshot
}

func (f *fakeStatusVisitScheduler) Status() VisitSchedulerStatusSnapshot {
	return f.status
}

func (f *fakeStatusFlusher) TriggerFlush() {}

func (f *fakeStatusFlusher) LastFlushResult() (flusher.LastFlushResult, bool) {
	return f.result, f.has
}

func TestStatusEndpoint_ReturnsOperationalState(t *testing.T) {
	lastSuccessAt := time.Date(2026, 4, 21, 22, 9, 30, 0, time.UTC)
	lastAttemptAt := time.Date(2026, 4, 21, 22, 10, 0, 0, time.UTC)

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		Buffer: &fakeStatusBuffer{
			stats: buffer.Stats{
				TotalBufferedPoints: 11,
				TotalBufferedBytes:  2048,
				OldestBufferedAge:   17 * time.Second,
			},
		},
		Spool: &fakeStatusSpool{
			segmentCount: 3,
			checkpoint:   spool.Checkpoint{LastCommittedSeq: 44},
		},
		Flusher: &fakeStatusFlusher{
			has: true,
			result: flusher.LastFlushResult{
				AtUTC:            lastAttemptAt,
				LastSuccessAtUTC: lastSuccessAt,
				Success:          true,
			},
		},
		VisitScheduler: &fakeStatusVisitScheduler{
			status: VisitSchedulerStatusSnapshot{
				Enabled:          true,
				Running:          false,
				LastRunStartUTC:  time.Date(2026, 4, 21, 22, 0, 0, 0, time.UTC),
				LastRunFinishUTC: time.Date(2026, 4, 21, 22, 0, 2, 0, time.UTC),
				LastSuccessUTC:   time.Date(2026, 4, 21, 22, 0, 2, 0, time.UTC),
				LastRunProcessed: 2,
				LastRunUpdated:   1,
				LastRunCreated:   3,
				WatermarkDevices: 2,
				WatermarkMinSeq:  10,
				WatermarkMaxSeq:  20,
				LagSeconds:       12,
			},
		},
		SpoolDir:   "/tmp/plexplore-spool",
		SQLitePath: "/tmp/plexplore.db",
		UserStore:  &fakeUserStore{users: map[int64]store.User{1: {ID: 1, Email: "u@example.com"}}},
		SessionStore: &fakeSessionStore{
			sessionByToken: map[string]store.Session{
				"token-1": {Token: "token-1", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp operationalStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal status response failed: %v", err)
	}
	if resp.BufferPoints != 11 || resp.BufferBytes != 2048 {
		t.Fatalf("unexpected buffer status: %+v", resp)
	}
	if resp.SpoolSegmentCount != 3 || resp.CheckpointSeq != 44 {
		t.Fatalf("unexpected spool status: %+v", resp)
	}
	if resp.ServiceHealth != "ok" {
		t.Fatalf("expected service health ok, got %q", resp.ServiceHealth)
	}
	if resp.SpoolDirPath != "/tmp/plexplore-spool" || resp.SQLiteDBPath != "/tmp/plexplore.db" {
		t.Fatalf("unexpected path fields: %+v", resp)
	}
	if resp.LastFlush == nil || !resp.LastFlush.Success {
		t.Fatalf("expected successful last flush, got %+v", resp.LastFlush)
	}
	if resp.LastFlushAttemptAtUTC == "" || resp.LastFlushSuccessAtUTC == "" {
		t.Fatalf("expected flush timing fields, got %+v", resp)
	}
	if resp.VisitScheduler == nil || !resp.VisitScheduler.Enabled {
		t.Fatalf("expected visit scheduler status in response, got %+v", resp.VisitScheduler)
	}
	if resp.VisitScheduler.LastRun.ProcessedDevices != 2 || resp.VisitScheduler.WatermarkSummary.DevicesWithWatermark != 2 {
		t.Fatalf("unexpected visit scheduler payload: %+v", resp.VisitScheduler)
	}
}

func TestStatusEndpoint_IncludesLastFlushError(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		Buffer: &fakeStatusBuffer{
			stats: buffer.Stats{
				TotalBufferedPoints: 1,
				TotalBufferedBytes:  64,
			},
		},
		Spool: &fakeStatusSpool{
			segmentCount: 1,
			checkpoint:   spool.Checkpoint{LastCommittedSeq: 3},
		},
		Flusher: &fakeStatusFlusher{
			has: true,
			result: flusher.LastFlushResult{
				AtUTC:   time.Date(2026, 4, 21, 23, 0, 0, 0, time.UTC),
				Success: false,
				Error:   "sqlite busy",
			},
		},
		UserStore: &fakeUserStore{users: map[int64]store.User{1: {ID: 1, Email: "u@example.com"}}},
		SessionStore: &fakeSessionStore{
			sessionByToken: map[string]store.Session{
				"token-1": {Token: "token-1", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp operationalStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal status response failed: %v", err)
	}
	if resp.LastFlushError != "sqlite busy" {
		t.Fatalf("expected last flush error sqlite busy, got %+v", resp)
	}
	if resp.LastFlushSuccessAtUTC != "" {
		t.Fatalf("expected empty last flush success time, got %+v", resp)
	}
}

func TestStatusEndpoint_UnauthenticatedDenied_WhenSessionDepsProvided(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		Buffer: &fakeStatusBuffer{
			stats: buffer.Stats{
				TotalBufferedPoints: 2,
				TotalBufferedBytes:  128,
			},
		},
		Spool: &fakeStatusSpool{
			segmentCount: 2,
			checkpoint:   spool.Checkpoint{LastCommittedSeq: 9},
		},
		UserStore:    &fakeUserStore{users: map[int64]store.User{1: {ID: 1, Email: "u@example.com"}}},
		SessionStore: &fakeSessionStore{sessionByToken: map[string]store.Session{}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated /api/v1/status, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStatusEndpoint_PublicAliasRouteIsSafe(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		Buffer: &fakeStatusBuffer{
			stats: buffer.Stats{
				TotalBufferedPoints: 2,
				TotalBufferedBytes:  128,
			},
		},
		Spool: &fakeStatusSpool{
			segmentCount: 2,
			checkpoint:   spool.Checkpoint{LastCommittedSeq: 9},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /status alias, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() == "" {
		t.Fatalf("expected public status body")
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal /status response failed: %v", err)
	}
	if _, ok := resp["spool_dir_path"]; ok {
		t.Fatalf("did not expect sensitive field spool_dir_path in /status: %+v", resp)
	}
	if _, ok := resp["sqlite_db_path"]; ok {
		t.Fatalf("did not expect sensitive field sqlite_db_path in /status: %+v", resp)
	}
	if _, ok := resp["checkpoint_seq"]; ok {
		t.Fatalf("did not expect sensitive field checkpoint_seq in /status: %+v", resp)
	}
	if _, ok := resp["visit_scheduler"]; ok {
		t.Fatalf("did not expect visit_scheduler in public /status: %+v", resp)
	}
	if got := resp["service_health"]; got != "ok" {
		t.Fatalf("expected service_health=ok, got %+v", got)
	}
}

func TestHealthEndpoint_RemainsPublic(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /health, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff header on /health, got %q", got)
	}
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("did not expect in-app HSTS on local HTTP /health, got %q", got)
	}
}
