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

func lookupDeviceIDForUser(t *testing.T, s *SQLiteStore, userID int64, name string) int64 {
	t.Helper()
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM devices WHERE user_id = ? AND name = ? ORDER BY id ASC LIMIT 1;`, userID, name).Scan(&id); err != nil {
		t.Fatalf("lookup device id failed: %v", err)
	}
	return id
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

	deviceID := lookupDeviceIDForUser(t, s, 1, "d1")
	created, err := s.RebuildVisitsForDeviceID(context.Background(), deviceID, visits.Config{
		MinDwell:        10 * time.Minute,
		MaxRadiusMeters: 30,
	})
	if err != nil {
		t.Fatalf("RebuildVisitsForDevice failed: %v", err)
	}
	if created != 1 {
		t.Fatalf("expected 1 visit, got %d", created)
	}

	out, err := s.ListVisits(context.Background(), 1, &deviceID, nil, nil, 10)
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

	deviceID := lookupDeviceIDForUser(t, s, 1, "d1")
	created, err := s.RebuildVisitsForDeviceID(context.Background(), deviceID, visits.Config{
		MinDwell:        10 * time.Minute,
		MaxRadiusMeters: 50,
	})
	if err != nil {
		t.Fatalf("RebuildVisitsForDevice failed: %v", err)
	}
	if created != 0 {
		t.Fatalf("expected 0 visits, got %d", created)
	}

	out, err := s.ListVisits(context.Background(), 1, &deviceID, nil, nil, 10)
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

	deviceID := lookupDeviceIDForUser(t, s, 1, "d1")
	created, err := s.RebuildVisitsForDeviceID(context.Background(), deviceID, visits.Config{
		MinDwell:        10 * time.Minute,
		MaxRadiusMeters: 40,
	})
	if err != nil {
		t.Fatalf("RebuildVisitsForDevice failed: %v", err)
	}
	if created != 2 {
		t.Fatalf("expected 2 visits, got %d", created)
	}

	out, err := s.ListVisits(context.Background(), 1, &deviceID, nil, nil, 10)
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
	deviceID := lookupDeviceIDForUser(t, s, 1, "d1")
	created, err := s.RebuildVisitsForDeviceRange(context.Background(), deviceID, &from, &to, visits.Config{
		MinDwell:        10 * time.Minute,
		MaxRadiusMeters: 40,
	})
	if err != nil {
		t.Fatalf("RebuildVisitsForDeviceRange failed: %v", err)
	}
	if created != 1 {
		t.Fatalf("expected 1 bounded visit, got %d", created)
	}

	out, err := s.ListVisits(context.Background(), 1, &deviceID, nil, nil, 10)
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

	deviceID := lookupDeviceIDForUser(t, s, 1, "d1")
	if _, err := s.RebuildVisitsForDeviceID(context.Background(), deviceID, visits.Config{
		MinDwell:        10 * time.Minute,
		MaxRadiusMeters: 40,
	}); err != nil {
		t.Fatalf("RebuildVisitsForDevice failed: %v", err)
	}

	from := base.Add(20 * time.Hour)
	to := base.Add(28 * time.Hour)
	out, err := s.ListVisits(context.Background(), 1, &deviceID, &from, &to, 10)
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

func TestVisitIsolation_SameDeviceNameAcrossUsers(t *testing.T) {
	s := openStoreWithSchema(t)
	base := time.Date(2026, 4, 23, 8, 0, 0, 0, time.UTC)

	if _, err := s.db.Exec(`
INSERT INTO users(id, name, email, password_hash, is_admin, updated_at)
VALUES
    (10, 'u1', 'u1@example.com', 'hash-u1-password', 0, strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    (11, 'u2', 'u2@example.com', 'hash-u2-password', 0, strftime('%Y-%m-%dT%H:%M:%fZ','now'));
`); err != nil {
		t.Fatalf("insert users failed: %v", err)
	}

	if _, err := s.InsertSpoolBatch([]ingest.SpoolRecord{
		visitRecord(100, "phone", "u1-a1", base, 41.00000, -87.00000),
		visitRecord(101, "phone", "u1-a2", base.Add(7*time.Minute), 41.00001, -87.00001),
		visitRecord(102, "phone", "u1-a3", base.Add(15*time.Minute), 41.00002, -87.00000),
	}); err != nil {
		t.Fatalf("insert user1 points failed: %v", err)
	}

	if _, err := s.InsertSpoolBatch([]ingest.SpoolRecord{
		{
			Seq:        200,
			DeviceID:   "phone",
			ReceivedAt: base,
			Point: ingest.CanonicalPoint{
				UserID:       "11",
				DeviceID:     "phone",
				SourceType:   "owntracks",
				TimestampUTC: base,
				Lat:          42.00000,
				Lon:          -88.00000,
				IngestHash:   "u2-a1",
			},
		},
		{
			Seq:        201,
			DeviceID:   "phone",
			ReceivedAt: base.Add(7 * time.Minute),
			Point: ingest.CanonicalPoint{
				UserID:       "11",
				DeviceID:     "phone",
				SourceType:   "owntracks",
				TimestampUTC: base.Add(7 * time.Minute),
				Lat:          42.00001,
				Lon:          -88.00001,
				IngestHash:   "u2-a2",
			},
		},
		{
			Seq:        202,
			DeviceID:   "phone",
			ReceivedAt: base.Add(15 * time.Minute),
			Point: ingest.CanonicalPoint{
				UserID:       "11",
				DeviceID:     "phone",
				SourceType:   "owntracks",
				TimestampUTC: base.Add(15 * time.Minute),
				Lat:          42.00002,
				Lon:          -88.00000,
				IngestHash:   "u2-a3",
			},
		},
	}); err != nil {
		t.Fatalf("insert user2 points failed: %v", err)
	}

	deviceU1 := lookupDeviceIDForUser(t, s, 1, "phone")
	deviceU2 := lookupDeviceIDForUser(t, s, 11, "phone")
	if deviceU1 == deviceU2 {
		t.Fatalf("expected distinct row ids for same-name devices across users, got %d", deviceU1)
	}

	if _, err := s.RebuildVisitsForDeviceID(context.Background(), deviceU1, visits.Config{
		MinDwell:        10 * time.Minute,
		MaxRadiusMeters: 40,
	}); err != nil {
		t.Fatalf("rebuild visits user1 failed: %v", err)
	}
	visitsU1, err := s.ListVisits(context.Background(), 1, &deviceU1, nil, nil, 10)
	if err != nil {
		t.Fatalf("list visits user1 failed: %v", err)
	}
	if len(visitsU1) != 1 {
		t.Fatalf("expected one visit for user1, got %d", len(visitsU1))
	}

	visitsU2Before, err := s.ListVisits(context.Background(), 11, &deviceU2, nil, nil, 10)
	if err != nil {
		t.Fatalf("list visits user2 before generate failed: %v", err)
	}
	if len(visitsU2Before) != 0 {
		t.Fatalf("expected zero visits for user2 before own generation, got %d", len(visitsU2Before))
	}

	if _, err := s.RebuildVisitsForDeviceID(context.Background(), deviceU2, visits.Config{
		MinDwell:        10 * time.Minute,
		MaxRadiusMeters: 40,
	}); err != nil {
		t.Fatalf("rebuild visits user2 failed: %v", err)
	}

	visitsU2After, err := s.ListVisits(context.Background(), 11, &deviceU2, nil, nil, 10)
	if err != nil {
		t.Fatalf("list visits user2 after generate failed: %v", err)
	}
	if len(visitsU2After) != 1 {
		t.Fatalf("expected one visit for user2 after own generation, got %d", len(visitsU2After))
	}

	visitsU1Again, err := s.ListVisits(context.Background(), 1, &deviceU1, nil, nil, 10)
	if err != nil {
		t.Fatalf("list visits user1 after user2 generation failed: %v", err)
	}
	if len(visitsU1Again) != 1 {
		t.Fatalf("expected user1 visits unchanged after user2 generation, got %d", len(visitsU1Again))
	}
}

func TestVisitPlaceCache_UpsertAndRead(t *testing.T) {
	s := openStoreWithSchema(t)
	ctx := context.Background()

	label, ok, err := s.GetVisitPlaceLabel(ctx, "nominatim", "41.1000", "-87.1000")
	if err != nil {
		t.Fatalf("GetVisitPlaceLabel initial failed: %v", err)
	}
	if ok || label != "" {
		t.Fatalf("expected empty initial cache, got ok=%v label=%q", ok, label)
	}

	if err := s.UpsertVisitPlaceLabel(ctx, "nominatim", "41.1000", "-87.1000", "Test Place"); err != nil {
		t.Fatalf("UpsertVisitPlaceLabel failed: %v", err)
	}
	label, ok, err = s.GetVisitPlaceLabel(ctx, "nominatim", "41.1000", "-87.1000")
	if err != nil {
		t.Fatalf("GetVisitPlaceLabel after insert failed: %v", err)
	}
	if !ok || label != "Test Place" {
		t.Fatalf("expected cached Test Place, got ok=%v label=%q", ok, label)
	}

	if err := s.UpsertVisitPlaceLabel(ctx, "nominatim", "41.1000", "-87.1000", "Updated Place"); err != nil {
		t.Fatalf("UpsertVisitPlaceLabel update failed: %v", err)
	}
	label, ok, err = s.GetVisitPlaceLabel(ctx, "nominatim", "41.1000", "-87.1000")
	if err != nil {
		t.Fatalf("GetVisitPlaceLabel after update failed: %v", err)
	}
	if !ok || label != "Updated Place" {
		t.Fatalf("expected cached Updated Place, got ok=%v label=%q", ok, label)
	}
}
