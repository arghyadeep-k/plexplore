package store

import (
	"context"
	"testing"
	"time"

	"plexplore/internal/ingest"
	"plexplore/internal/visits"
)

func visitRecord(seq uint64, deviceID, hash string, ts time.Time, lat, lon float64) ingest.SpoolRecord {
	return ingest.SpoolRecord{
		Seq:        seq,
		DeviceID:   deviceID,
		ReceivedAt: ts,
		Point: ingest.CanonicalPoint{
			DeviceID:     deviceID,
			SourceType:   "owntracks",
			TimestampUTC: ts,
			Lat:          lat,
			Lon:          lon,
			IngestHash:   hash,
		},
	}
}

func TestVisitDetection_StationaryPointsBecomeVisit(t *testing.T) {
	s := openStoreWithSchema(t)
	base := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

	records := []ingest.SpoolRecord{
		visitRecord(1, "d1", "visit-a1", base, 41.00000, -87.00000),
		visitRecord(2, "d1", "visit-a2", base.Add(5*time.Minute), 41.00003, -87.00002),
		visitRecord(3, "d1", "visit-a3", base.Add(15*time.Minute), 41.00002, -87.00001),
	}
	if _, err := s.InsertSpoolBatch(records); err != nil {
		t.Fatalf("InsertSpoolBatch failed: %v", err)
	}

	created, err := s.RebuildVisitsForDevice(context.Background(), "d1", visits.Config{
		MinDwell:        10 * time.Minute,
		MaxRadiusMeters: 30,
	})
	if err != nil {
		t.Fatalf("RebuildVisitsForDevice failed: %v", err)
	}
	if created != 1 {
		t.Fatalf("expected 1 visit, got %d", created)
	}

	out, err := s.ListVisits(context.Background(), "d1", nil, nil, 10)
	if err != nil {
		t.Fatalf("ListVisits failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 persisted visit, got %d", len(out))
	}
	if out[0].PointCount != 3 {
		t.Fatalf("expected point_count=3, got %d", out[0].PointCount)
	}
}

func TestVisitDetection_MovingPointsDoNotBecomeVisit(t *testing.T) {
	s := openStoreWithSchema(t)
	base := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

	records := []ingest.SpoolRecord{
		visitRecord(1, "d1", "move-a1", base, 41.00000, -87.00000),
		visitRecord(2, "d1", "move-a2", base.Add(7*time.Minute), 41.00300, -87.00300),
		visitRecord(3, "d1", "move-a3", base.Add(15*time.Minute), 41.00600, -87.00600),
	}
	if _, err := s.InsertSpoolBatch(records); err != nil {
		t.Fatalf("InsertSpoolBatch failed: %v", err)
	}

	created, err := s.RebuildVisitsForDevice(context.Background(), "d1", visits.Config{
		MinDwell:        10 * time.Minute,
		MaxRadiusMeters: 50,
	})
	if err != nil {
		t.Fatalf("RebuildVisitsForDevice failed: %v", err)
	}
	if created != 0 {
		t.Fatalf("expected 0 visits, got %d", created)
	}

	out, err := s.ListVisits(context.Background(), "d1", nil, nil, 10)
	if err != nil {
		t.Fatalf("ListVisits failed: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected no visits, got %+v", out)
	}
}

func TestVisitDetection_TwoVisitsNotMerged(t *testing.T) {
	s := openStoreWithSchema(t)
	base := time.Date(2026, 4, 22, 8, 0, 0, 0, time.UTC)

	records := []ingest.SpoolRecord{
		visitRecord(1, "d1", "merge-a1", base, 41.10000, -87.10000),
		visitRecord(2, "d1", "merge-a2", base.Add(6*time.Minute), 41.10002, -87.10001),
		visitRecord(3, "d1", "merge-a3", base.Add(12*time.Minute), 41.10001, -87.10000),
		// Move far away between visits.
		visitRecord(4, "d1", "merge-b1", base.Add(25*time.Minute), 41.20000, -87.20000),
		visitRecord(5, "d1", "merge-b2", base.Add(32*time.Minute), 41.20002, -87.20001),
		visitRecord(6, "d1", "merge-b3", base.Add(40*time.Minute), 41.20001, -87.20000),
	}
	if _, err := s.InsertSpoolBatch(records); err != nil {
		t.Fatalf("InsertSpoolBatch failed: %v", err)
	}

	created, err := s.RebuildVisitsForDevice(context.Background(), "d1", visits.Config{
		MinDwell:        10 * time.Minute,
		MaxRadiusMeters: 40,
	})
	if err != nil {
		t.Fatalf("RebuildVisitsForDevice failed: %v", err)
	}
	if created != 2 {
		t.Fatalf("expected 2 visits, got %d", created)
	}

	out, err := s.ListVisits(context.Background(), "d1", nil, nil, 10)
	if err != nil {
		t.Fatalf("ListVisits failed: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 persisted visits, got %d", len(out))
	}
	if !out[0].EndAt.Before(out[1].StartAt) {
		t.Fatalf("expected separated visits, got %+v", out)
	}
}

func TestVisitDetection_RangeBoundedGeneration(t *testing.T) {
	s := openStoreWithSchema(t)
	base := time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)

	records := []ingest.SpoolRecord{
		// Visit outside target range.
		visitRecord(1, "d1", "rng-a1", base, 41.10000, -87.10000),
		visitRecord(2, "d1", "rng-a2", base.Add(6*time.Minute), 41.10002, -87.10001),
		visitRecord(3, "d1", "rng-a3", base.Add(12*time.Minute), 41.10001, -87.10000),
		// Visit inside target range.
		visitRecord(4, "d1", "rng-b1", base.Add(24*time.Hour), 41.20000, -87.20000),
		visitRecord(5, "d1", "rng-b2", base.Add(24*time.Hour+6*time.Minute), 41.20002, -87.20001),
		visitRecord(6, "d1", "rng-b3", base.Add(24*time.Hour+12*time.Minute), 41.20001, -87.20000),
	}
	if _, err := s.InsertSpoolBatch(records); err != nil {
		t.Fatalf("InsertSpoolBatch failed: %v", err)
	}

	from := base.Add(24 * time.Hour)
	to := base.Add(24*time.Hour + 1*time.Hour)
	created, err := s.RebuildVisitsForDeviceRange(context.Background(), "d1", &from, &to, visits.Config{
		MinDwell:        10 * time.Minute,
		MaxRadiusMeters: 40,
	})
	if err != nil {
		t.Fatalf("RebuildVisitsForDeviceRange failed: %v", err)
	}
	if created != 1 {
		t.Fatalf("expected 1 bounded visit, got %d", created)
	}

	out, err := s.ListVisits(context.Background(), "d1", nil, nil, 10)
	if err != nil {
		t.Fatalf("ListVisits failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected only bounded-range visit persisted, got %d", len(out))
	}
	if out[0].StartAt.Before(from) || out[0].StartAt.After(to) {
		t.Fatalf("expected visit start within bounded range, got %v", out[0].StartAt)
	}
}

func TestListVisits_FilterByTimeRange(t *testing.T) {
	s := openStoreWithSchema(t)
	base := time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)

	records := []ingest.SpoolRecord{
		visitRecord(1, "d1", "rng-list-a1", base, 41.10000, -87.10000),
		visitRecord(2, "d1", "rng-list-a2", base.Add(6*time.Minute), 41.10002, -87.10001),
		visitRecord(3, "d1", "rng-list-a3", base.Add(12*time.Minute), 41.10001, -87.10000),
		visitRecord(4, "d1", "rng-list-b1", base.Add(24*time.Hour), 41.20000, -87.20000),
		visitRecord(5, "d1", "rng-list-b2", base.Add(24*time.Hour+6*time.Minute), 41.20002, -87.20001),
		visitRecord(6, "d1", "rng-list-b3", base.Add(24*time.Hour+12*time.Minute), 41.20001, -87.20000),
	}
	if _, err := s.InsertSpoolBatch(records); err != nil {
		t.Fatalf("InsertSpoolBatch failed: %v", err)
	}

	if _, err := s.RebuildVisitsForDevice(context.Background(), "d1", visits.Config{
		MinDwell:        10 * time.Minute,
		MaxRadiusMeters: 40,
	}); err != nil {
		t.Fatalf("RebuildVisitsForDevice failed: %v", err)
	}

	from := base.Add(20 * time.Hour)
	to := base.Add(28 * time.Hour)
	out, err := s.ListVisits(context.Background(), "d1", &from, &to, 10)
	if err != nil {
		t.Fatalf("ListVisits failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 visit in time range, got %d", len(out))
	}
	if out[0].StartAt.Before(from) || out[0].StartAt.After(to) {
		t.Fatalf("expected visit start within range, got %v", out[0].StartAt)
	}
}
