package buffer

import (
	"errors"
	"testing"
	"time"

	"plexplore/internal/ingest"
)

func makeRecord(seq uint64, deviceID string, payloadSize int) ingest.SpoolRecord {
	raw := make([]byte, payloadSize)
	for i := range raw {
		raw[i] = 'x'
	}

	return ingest.SpoolRecord{
		Seq:        seq,
		DeviceID:   deviceID,
		ReceivedAt: time.Date(2026, 4, 21, 20, 0, int(seq), 0, time.UTC),
		Point: ingest.CanonicalPoint{
			DeviceID:     deviceID,
			SourceType:   "owntracks",
			TimestampUTC: time.Date(2026, 4, 21, 20, 0, int(seq), 0, time.UTC),
			Lat:          37.42 + float64(seq)*0.001,
			Lon:          -122.08 - float64(seq)*0.001,
			RawPayload:   raw,
			IngestHash:   "hash",
		},
	}
}

func TestManager_EnqueueDrainAndStats(t *testing.T) {
	manager := NewManager(10, 1024*1024)
	baseNow := time.Date(2026, 4, 21, 20, 0, 0, 0, time.UTC)
	manager.nowFn = func() time.Time { return baseNow }

	r1 := makeRecord(1, "d1", 20)
	r2 := makeRecord(2, "d2", 20)
	if err := manager.Enqueue([]ingest.SpoolRecord{r1, r2}); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	stats := manager.Stats()
	if stats.TotalBufferedPoints != 2 {
		t.Fatalf("expected 2 buffered points, got %d", stats.TotalBufferedPoints)
	}
	if stats.TotalBufferedBytes <= 0 {
		t.Fatalf("expected positive buffered bytes, got %d", stats.TotalBufferedBytes)
	}
	if stats.OldestBufferedAge != 0 {
		t.Fatalf("expected zero age at enqueue time, got %v", stats.OldestBufferedAge)
	}

	manager.nowFn = func() time.Time { return baseNow.Add(5 * time.Second) }
	stats = manager.Stats()
	if stats.OldestBufferedAge != 5*time.Second {
		t.Fatalf("expected oldest age 5s, got %v", stats.OldestBufferedAge)
	}

	drained := manager.DrainBatch(1)
	if len(drained) != 1 || drained[0].Seq != 1 {
		t.Fatalf("expected to drain first record seq=1, got %+v", drained)
	}

	stats = manager.Stats()
	if stats.TotalBufferedPoints != 1 {
		t.Fatalf("expected 1 buffered point after drain, got %d", stats.TotalBufferedPoints)
	}

	drained = manager.DrainBatch(10)
	if len(drained) != 1 || drained[0].Seq != 2 {
		t.Fatalf("expected to drain second record seq=2, got %+v", drained)
	}

	stats = manager.Stats()
	if stats.TotalBufferedPoints != 0 || stats.TotalBufferedBytes != 0 || stats.OldestBufferedAge != 0 {
		t.Fatalf("expected empty buffer stats, got %+v", stats)
	}
}

func TestManager_EnqueueRespectsMaxPoints(t *testing.T) {
	manager := NewManager(2, 1024*1024)
	r1 := makeRecord(1, "d1", 10)
	r2 := makeRecord(2, "d1", 10)
	r3 := makeRecord(3, "d1", 10)

	if err := manager.Enqueue([]ingest.SpoolRecord{r1, r2}); err != nil {
		t.Fatalf("initial Enqueue failed: %v", err)
	}
	err := manager.Enqueue([]ingest.SpoolRecord{r3})
	if !errors.Is(err, ErrMaxPointsExceeded) {
		t.Fatalf("expected ErrMaxPointsExceeded, got %v", err)
	}

	stats := manager.Stats()
	if stats.TotalBufferedPoints != 2 {
		t.Fatalf("expected buffered points to remain 2, got %d", stats.TotalBufferedPoints)
	}
}

func TestManager_EnqueueRespectsMaxBytes(t *testing.T) {
	manager := NewManager(10, 200)
	r1 := makeRecord(1, "d1", 120)
	r2 := makeRecord(2, "d1", 120)

	err := manager.Enqueue([]ingest.SpoolRecord{r1, r2})
	if !errors.Is(err, ErrMaxBytesExceeded) {
		t.Fatalf("expected ErrMaxBytesExceeded, got %v", err)
	}

	stats := manager.Stats()
	if stats.TotalBufferedPoints != 0 || stats.TotalBufferedBytes != 0 {
		t.Fatalf("expected empty buffer after rejected enqueue, got %+v", stats)
	}
}

func TestManager_DedupeSuppressesNearDuplicateFromSameDevice(t *testing.T) {
	baseTS := time.Date(2026, 4, 21, 20, 0, 0, 0, time.UTC)
	manager := NewManagerWithDedupe(10, 1024*1024, 2*time.Second, 10.0)

	r1 := makeRecord(1, "d1", 10)
	r1.Point.TimestampUTC = baseTS
	r1.Point.Lat = 37.421999
	r1.Point.Lon = -122.084000

	r2 := makeRecord(2, "d1", 10)
	r2.Point.TimestampUTC = baseTS.Add(1 * time.Second)
	r2.Point.Lat = 37.4219995
	r2.Point.Lon = -122.0840004

	if err := manager.Enqueue([]ingest.SpoolRecord{r1, r2}); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	drained := manager.DrainBatch(10)
	if len(drained) != 1 {
		t.Fatalf("expected near-duplicate suppression to keep 1 record, got %d", len(drained))
	}
	if drained[0].Seq != r1.Seq {
		t.Fatalf("expected first record to be retained, got seq=%d", drained[0].Seq)
	}
}

func TestManager_DedupeRetainsRealMovement(t *testing.T) {
	baseTS := time.Date(2026, 4, 21, 20, 0, 0, 0, time.UTC)
	manager := NewManagerWithDedupe(10, 1024*1024, 2*time.Second, 10.0)

	r1 := makeRecord(1, "d1", 10)
	r1.Point.TimestampUTC = baseTS
	r1.Point.Lat = 37.421999
	r1.Point.Lon = -122.084000

	// About 111m latitude shift, still within time threshold.
	r2 := makeRecord(2, "d1", 10)
	r2.Point.TimestampUTC = baseTS.Add(1 * time.Second)
	r2.Point.Lat = 37.422999
	r2.Point.Lon = -122.084000

	if err := manager.Enqueue([]ingest.SpoolRecord{r1, r2}); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	drained := manager.DrainBatch(10)
	if len(drained) != 2 {
		t.Fatalf("expected real movement to retain both records, got %d", len(drained))
	}
	if drained[0].Seq != r1.Seq || drained[1].Seq != r2.Seq {
		t.Fatalf("unexpected drained order/sequences: %+v", drained)
	}
}
