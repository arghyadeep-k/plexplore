package spool

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"plexplore/internal/ingest"
)

func readAllSegmentRecords(path string) ([]ingest.SpoolRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var records []ingest.SpoolRecord
	start := 0
	for i, b := range data {
		if b != '\n' {
			continue
		}
		line := data[start : i+1]
		start = i + 1
		record, err := DeserializeRecord(line)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if start != len(data) {
		return nil, io.ErrUnexpectedEOF
	}
	return records, nil
}

func TestAppendCanonicalPoints_AppendsAndReturnsRecords(t *testing.T) {
	dir := t.TempDir()
	manager := NewFileSpoolManagerWithOptions(dir, 1024*1024, ManagerOptions{
		FSyncMode:          FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})
	t.Cleanup(func() { _ = manager.Close() })

	points := []ingest.CanonicalPoint{
		{
			DeviceID:     "device-a",
			SourceType:   "owntracks",
			TimestampUTC: time.Date(2026, 4, 21, 20, 0, 0, 0, time.UTC),
			Lat:          37.42,
			Lon:          -122.08,
		},
	}

	appended, err := manager.AppendCanonicalPoints(points)
	if err != nil {
		t.Fatalf("AppendCanonicalPoints returned error: %v", err)
	}
	if len(appended) != 1 {
		t.Fatalf("expected 1 appended record, got %d", len(appended))
	}
	if appended[0].Seq != 1 {
		t.Fatalf("expected seq=1, got %d", appended[0].Seq)
	}
	if appended[0].Point.IngestHash == "" {
		t.Fatal("expected ingest hash to be assigned")
	}

	records, err := readAllSegmentRecords(manager.SegmentPath(1))
	if err != nil {
		t.Fatalf("readAllSegmentRecords returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record on disk, got %d", len(records))
	}
	if records[0].Seq != 1 {
		t.Fatalf("expected on-disk seq=1, got %d", records[0].Seq)
	}
}

func TestAppendCanonicalPoints_SequenceMonotonicityAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	manager := NewFileSpoolManagerWithOptions(dir, 1024*1024, ManagerOptions{
		FSyncMode:          FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})
	t.Cleanup(func() { _ = manager.Close() })

	batchA, err := manager.AppendCanonicalPoints([]ingest.CanonicalPoint{
		{DeviceID: "d1", SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 1, Lon: 2},
		{DeviceID: "d1", SourceType: "owntracks", TimestampUTC: time.Now().UTC(), Lat: 3, Lon: 4},
	})
	if err != nil {
		t.Fatalf("append batch A failed: %v", err)
	}
	batchB, err := manager.AppendCanonicalPoints([]ingest.CanonicalPoint{
		{DeviceID: "d1", SourceType: "overland", TimestampUTC: time.Now().UTC(), Lat: 5, Lon: 6},
	})
	if err != nil {
		t.Fatalf("append batch B failed: %v", err)
	}

	var got []uint64
	for _, record := range batchA {
		got = append(got, record.Seq)
	}
	for _, record := range batchB {
		got = append(got, record.Seq)
	}
	want := []uint64{1, 2, 3}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected sequence values: got %v want %v", got, want)
	}
}

func TestAppendCanonicalPoints_SegmentRollover(t *testing.T) {
	dir := t.TempDir()
	manager := NewFileSpoolManagerWithOptions(dir, 220, ManagerOptions{
		FSyncMode:          FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})
	t.Cleanup(func() { _ = manager.Close() })

	points := []ingest.CanonicalPoint{
		{
			DeviceID:     "device-a",
			SourceType:   "owntracks",
			TimestampUTC: time.Date(2026, 4, 21, 20, 0, 0, 0, time.UTC),
			Lat:          37.42,
			Lon:          -122.08,
			RawPayload:   []byte(`{"payload":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`),
		},
		{
			DeviceID:     "device-a",
			SourceType:   "owntracks",
			TimestampUTC: time.Date(2026, 4, 21, 20, 0, 1, 0, time.UTC),
			Lat:          37.43,
			Lon:          -122.09,
			RawPayload:   []byte(`{"payload":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`),
		},
		{
			DeviceID:     "device-a",
			SourceType:   "owntracks",
			TimestampUTC: time.Date(2026, 4, 21, 20, 0, 2, 0, time.UTC),
			Lat:          37.44,
			Lon:          -122.10,
			RawPayload:   []byte(`{"payload":"cccccccccccccccccccccccccccccccccccccccccccccccccccc"}`),
		},
	}

	_, err := manager.AppendCanonicalPoints(points)
	if err != nil {
		t.Fatalf("AppendCanonicalPoints returned error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	var segmentStarts []uint64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		startSeq, err := ParseSegmentStartSeq(entry.Name())
		if err == nil {
			segmentStarts = append(segmentStarts, startSeq)
		}
	}
	slices.Sort(segmentStarts)

	if len(segmentStarts) < 2 {
		t.Fatalf("expected rollover to create at least 2 segments, got starts=%v", segmentStarts)
	}

	firstSegmentPath := filepath.Join(dir, SegmentFileName(segmentStarts[0]))
	firstRecords, err := readAllSegmentRecords(firstSegmentPath)
	if err != nil {
		t.Fatalf("read first segment records failed: %v", err)
	}
	if len(firstRecords) == 0 {
		t.Fatal("expected first segment to contain records")
	}
	if firstRecords[0].Seq != segmentStarts[0] {
		t.Fatalf("expected first record seq to match segment start: seq=%d start=%d", firstRecords[0].Seq, segmentStarts[0])
	}
}
