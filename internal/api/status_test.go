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

func (f *fakeStatusFlusher) TriggerFlush() {}

func (f *fakeStatusFlusher) LastFlushResult() (flusher.LastFlushResult, bool) {
	return f.result, f.has
}

func TestStatusEndpoint_ReturnsOperationalState(t *testing.T) {
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
				AtUTC:   time.Date(2026, 4, 21, 22, 10, 0, 0, time.UTC),
				Success: true,
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
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
	if resp.LastFlush == nil || !resp.LastFlush.Success {
		t.Fatalf("expected successful last flush, got %+v", resp.LastFlush)
	}
}
