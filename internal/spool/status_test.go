package spool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSegmentCount_CountsOnlySegmentFiles(t *testing.T) {
	dir := t.TempDir()
	manager := NewFileSpoolManager(dir, 1024)
	t.Cleanup(func() { _ = manager.Close() })

	if err := os.WriteFile(filepath.Join(dir, SegmentFileName(1)), []byte{}, 0o644); err != nil {
		t.Fatalf("write segment 1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, SegmentFileName(2)), []byte{}, 0o644); err != nil {
		t.Fatalf("write segment 2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "checkpoint.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write checkpoint: %v", err)
	}

	count, err := manager.SegmentCount()
	if err != nil {
		t.Fatalf("SegmentCount failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 segments, got %d", count)
	}
}
