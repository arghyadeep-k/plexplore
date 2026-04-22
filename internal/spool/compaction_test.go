package spool

import (
	"os"
	"slices"
	"testing"
	"time"

	"plexplore/internal/ingest"
)

func listSegmentStarts(t *testing.T, dir string) []uint64 {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	starts := make([]uint64, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		startSeq, err := ParseSegmentStartSeq(entry.Name())
		if err == nil {
			starts = append(starts, startSeq)
		}
	}
	slices.Sort(starts)
	return starts
}

func makeCompactionPoint(offset int) ingest.CanonicalPoint {
	return ingest.CanonicalPoint{
		DeviceID:     "device-a",
		SourceType:   "owntracks",
		TimestampUTC: time.Date(2026, 4, 21, 20, 0, offset, 0, time.UTC),
		Lat:          37.4 + float64(offset)*0.001,
		Lon:          -122.0 - float64(offset)*0.001,
		RawPayload:   []byte(`{"payload":"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}`),
	}
}

func TestCompactCommittedSegments_RemovesFullyCommittedSegments(t *testing.T) {
	dir := t.TempDir()

	manager := NewFileSpoolManagerWithOptions(dir, 220, ManagerOptions{
		FSyncMode:          FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})

	_, err := manager.AppendCanonicalPoints([]ingest.CanonicalPoint{
		makeCompactionPoint(1),
		makeCompactionPoint(2),
		makeCompactionPoint(3),
		makeCompactionPoint(4),
		makeCompactionPoint(5),
		makeCompactionPoint(6),
	})
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Re-open with no active writable segment so old committed segments can be compacted.
	manager = NewFileSpoolManagerWithOptions(dir, 220, ManagerOptions{
		FSyncMode:          FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})
	t.Cleanup(func() { _ = manager.Close() })

	before := listSegmentStarts(t, dir)
	if len(before) < 2 {
		t.Fatalf("expected multiple segments before compaction, got %v", before)
	}

	lastStart := before[len(before)-1]
	_, err = manager.AdvanceCheckpoint(lastStart - 1)
	if err != nil {
		t.Fatalf("AdvanceCheckpoint failed: %v", err)
	}

	deleted, err := manager.CompactCommittedSegments()
	if err != nil {
		t.Fatalf("CompactCommittedSegments failed: %v", err)
	}
	if deleted < 1 {
		t.Fatalf("expected at least one deleted segment, got %d", deleted)
	}

	after := listSegmentStarts(t, dir)
	if len(after) != 1 || after[0] != lastStart {
		t.Fatalf("expected only newest segment (%d) to remain, got %v", lastStart, after)
	}
}

func TestCompactCommittedSegments_DoesNotDeleteActiveWritableSegment(t *testing.T) {
	dir := t.TempDir()

	manager := NewFileSpoolManagerWithOptions(dir, 220, ManagerOptions{
		FSyncMode:          FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})
	t.Cleanup(func() { _ = manager.Close() })

	appended, err := manager.AppendCanonicalPoints([]ingest.CanonicalPoint{
		makeCompactionPoint(1),
		makeCompactionPoint(2),
		makeCompactionPoint(3),
		makeCompactionPoint(4),
	})
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if len(appended) == 0 {
		t.Fatal("expected appended records")
	}

	startsBefore := listSegmentStarts(t, dir)
	if len(startsBefore) < 2 {
		t.Fatalf("expected rollover with multiple segments, got %v", startsBefore)
	}

	activeStart := manager.activeSegmentStart
	if activeStart == 0 {
		t.Fatal("expected active writable segment to be set")
	}

	lastSeq := appended[len(appended)-1].Seq
	_, err = manager.AdvanceCheckpoint(lastSeq)
	if err != nil {
		t.Fatalf("AdvanceCheckpoint failed: %v", err)
	}

	_, err = manager.CompactCommittedSegments()
	if err != nil {
		t.Fatalf("CompactCommittedSegments failed: %v", err)
	}

	if _, err := os.Stat(manager.SegmentPath(activeStart)); err != nil {
		t.Fatalf("expected active writable segment to remain, stat failed: %v", err)
	}
}
