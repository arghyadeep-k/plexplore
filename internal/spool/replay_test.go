package spool

import (
	"os"
	"testing"
	"time"

	"plexplore/internal/ingest"
)

func testPoint(seqOffset int) ingest.CanonicalPoint {
	return ingest.CanonicalPoint{
		DeviceID:     "device-a",
		SourceType:   "owntracks",
		TimestampUTC: time.Date(2026, 4, 21, 20, 0, seqOffset, 0, time.UTC),
		Lat:          37.42 + float64(seqOffset)*0.001,
		Lon:          -122.08 - float64(seqOffset)*0.001,
	}
}

func TestReplayAfterCheckpoint_AfterSimulatedRestart(t *testing.T) {
	dir := t.TempDir()

	managerA := NewFileSpoolManagerWithOptions(dir, 220, ManagerOptions{
		FSyncMode:          FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})
	_, err := managerA.AppendCanonicalPoints([]ingest.CanonicalPoint{
		testPoint(1),
		testPoint(2),
		testPoint(3),
	})
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if err := managerA.Close(); err != nil {
		t.Fatalf("close managerA failed: %v", err)
	}

	// Simulated restart: new manager instance over the same spool directory.
	managerB := NewFileSpoolManagerWithOptions(dir, 220, ManagerOptions{
		FSyncMode:          FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})
	t.Cleanup(func() { _ = managerB.Close() })

	replayed, err := managerB.ReplayAfterCheckpoint()
	if err != nil {
		t.Fatalf("ReplayAfterCheckpoint failed: %v", err)
	}
	if len(replayed) != 3 {
		t.Fatalf("expected 3 replayed records, got %d", len(replayed))
	}
	if replayed[0].Seq != 1 || replayed[1].Seq != 2 || replayed[2].Seq != 3 {
		t.Fatalf("unexpected replay sequence values: %d %d %d", replayed[0].Seq, replayed[1].Seq, replayed[2].Seq)
	}
}

func TestReplayAfterCheckpoint_PartiallyCommittedSpool(t *testing.T) {
	dir := t.TempDir()
	manager := NewFileSpoolManagerWithOptions(dir, 260, ManagerOptions{
		FSyncMode:          FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})
	t.Cleanup(func() { _ = manager.Close() })

	_, err := manager.AppendCanonicalPoints([]ingest.CanonicalPoint{
		testPoint(1),
		testPoint(2),
		testPoint(3),
		testPoint(4),
		testPoint(5),
	})
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	_, err = manager.AdvanceCheckpoint(3)
	if err != nil {
		t.Fatalf("AdvanceCheckpoint failed: %v", err)
	}

	replayed, err := manager.ReplayAfterCheckpoint()
	if err != nil {
		t.Fatalf("ReplayAfterCheckpoint failed: %v", err)
	}
	if len(replayed) != 2 {
		t.Fatalf("expected 2 replayed records, got %d", len(replayed))
	}
	if replayed[0].Seq != 4 || replayed[1].Seq != 5 {
		t.Fatalf("unexpected replay sequence values after checkpoint: %d %d", replayed[0].Seq, replayed[1].Seq)
	}
}

func TestCheckpointAdvancement_IsMonotonic(t *testing.T) {
	dir := t.TempDir()
	manager := NewFileSpoolManager(dir, 1024)
	t.Cleanup(func() { _ = manager.Close() })

	first, err := manager.AdvanceCheckpoint(7)
	if err != nil {
		t.Fatalf("AdvanceCheckpoint(7) failed: %v", err)
	}
	if first.LastCommittedSeq != 7 {
		t.Fatalf("expected checkpoint=7, got %d", first.LastCommittedSeq)
	}

	second, err := manager.AdvanceCheckpoint(5)
	if err != nil {
		t.Fatalf("AdvanceCheckpoint(5) failed: %v", err)
	}
	if second.LastCommittedSeq != 7 {
		t.Fatalf("expected checkpoint to remain 7, got %d", second.LastCommittedSeq)
	}

	loaded, err := manager.ReadCheckpoint()
	if err != nil {
		t.Fatalf("ReadCheckpoint failed: %v", err)
	}
	if loaded.LastCommittedSeq != 7 {
		t.Fatalf("expected persisted checkpoint=7, got %d", loaded.LastCommittedSeq)
	}

	if _, err := os.Stat(manager.CheckpointPath()); err != nil {
		t.Fatalf("expected checkpoint file to exist, stat failed: %v", err)
	}
}
