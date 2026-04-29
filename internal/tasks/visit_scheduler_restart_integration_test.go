package tasks

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"plexplore/internal/ingest"
	"plexplore/internal/visits"
)

func TestVisitSchedulerRestart_PersistsWatermarkAndAvoidsDuplicateVisits(t *testing.T) {
	baseDir := t.TempDir()
	spoolDir := filepath.Join(baseDir, "spool")
	dbPath := filepath.Join(baseDir, "tracker.db")
	if err := applyTestSchema(dbPath); err != nil {
		t.Fatalf("apply test schema: %v", err)
	}

	rt1 := openIntegrationRuntime(t, spoolDir, dbPath, 1024*1024, 64, nil)

	userAID := createUser(t, rt1.sqliteStore, "user-a@example.com")
	userBID := createUser(t, rt1.sqliteStore, "user-b@example.com")
	deviceA := createDeviceForUser(t, rt1.sqliteStore, userAID, "phone", "owntracks", "k-user-a")
	deviceB := createDeviceForUser(t, rt1.sqliteStore, userBID, "phone", "owntracks", "k-user-b")

	nextSeq := uint64(1)
	nextSeq = appendDevicePoints(t, rt1, nextSeq, userAID, deviceA.Name, []schedulerPoint{
		{at: time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC), lat: 41.10000, lon: -87.10000},
		{at: time.Date(2026, 4, 28, 0, 10, 0, 0, time.UTC), lat: 41.10002, lon: -87.09998},
		{at: time.Date(2026, 4, 28, 0, 20, 0, 0, time.UTC), lat: 41.10001, lon: -87.09999},
	})
	nextSeq = appendDevicePoints(t, rt1, nextSeq, userBID, deviceB.Name, []schedulerPoint{
		{at: time.Date(2026, 4, 28, 1, 0, 0, 0, time.UTC), lat: 42.20000, lon: -88.20000},
		{at: time.Date(2026, 4, 28, 1, 10, 0, 0, time.UTC), lat: 42.20001, lon: -88.19999},
		{at: time.Date(2026, 4, 28, 1, 20, 0, 0, time.UTC), lat: 42.20002, lon: -88.19998},
	})

	scheduler1 := NewVisitScheduler(rt1.sqliteStore, VisitSchedulerConfig{
		Enabled:         true,
		Interval:        time.Hour,
		DeviceBatchSize: 10,
		Lookback:        15 * time.Minute,
		DetectConfig: visits.Config{
			MinDwell:        15 * time.Minute,
			MaxRadiusMeters: 40,
		},
	})
	firstRun, err := scheduler1.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("first scheduler run failed: %v", err)
	}
	if firstRun.UpdatedDevices != 2 {
		t.Fatalf("expected first run to update both devices, got %+v", firstRun)
	}

	assertVisitCountByDevice(t, dbPath, deviceA.ID, 1)
	assertVisitCountByDevice(t, dbPath, deviceB.ID, 1)
	assertWatermarkSeq(t, rt1, deviceA.ID, 3)
	assertWatermarkSeq(t, rt1, deviceB.ID, 6)

	rt1.close()

	rt2 := openIntegrationRuntime(t, spoolDir, dbPath, 1024*1024, 64, nil)
	t.Cleanup(func() { rt2.close() })

	scheduler2 := NewVisitScheduler(rt2.sqliteStore, VisitSchedulerConfig{
		Enabled:         true,
		Interval:        time.Hour,
		DeviceBatchSize: 10,
		Lookback:        15 * time.Minute,
		DetectConfig: visits.Config{
			MinDwell:        15 * time.Minute,
			MaxRadiusMeters: 40,
		},
	})
	secondRun, err := scheduler2.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("second scheduler run after restart failed: %v", err)
	}
	if secondRun.UpdatedDevices != 0 || secondRun.SkippedDevices != 2 {
		t.Fatalf("expected no-op run after restart with no new points, got %+v", secondRun)
	}

	assertVisitCountByDevice(t, dbPath, deviceA.ID, 1)
	assertVisitCountByDevice(t, dbPath, deviceB.ID, 1)
	assertWatermarkSeq(t, rt2, deviceA.ID, 3)
	assertWatermarkSeq(t, rt2, deviceB.ID, 6)

	nextSeq = appendDevicePoints(t, rt2, nextSeq, userAID, deviceA.Name, []schedulerPoint{
		{at: time.Date(2026, 4, 28, 2, 0, 0, 0, time.UTC), lat: 41.30000, lon: -87.30000},
		{at: time.Date(2026, 4, 28, 2, 10, 0, 0, time.UTC), lat: 41.30001, lon: -87.29999},
		{at: time.Date(2026, 4, 28, 2, 20, 0, 0, time.UTC), lat: 41.30002, lon: -87.29998},
	})

	thirdRun, err := scheduler2.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("third scheduler run with new points failed: %v", err)
	}
	if thirdRun.UpdatedDevices != 1 {
		t.Fatalf("expected only user A device to update, got %+v", thirdRun)
	}

	assertVisitCountByDevice(t, dbPath, deviceA.ID, 2)
	assertVisitCountByDevice(t, dbPath, deviceB.ID, 1)
	assertWatermarkSeq(t, rt2, deviceA.ID, 9)
	assertWatermarkSeq(t, rt2, deviceB.ID, 6)
}

type schedulerPoint struct {
	at  time.Time
	lat float64
	lon float64
}

func appendDevicePoints(
	t *testing.T,
	rt *integrationRuntime,
	startSeq uint64,
	userID int64,
	deviceName string,
	points []schedulerPoint,
) uint64 {
	t.Helper()

	records := make([]ingest.SpoolRecord, 0, len(points))
	seq := startSeq
	userIDRaw := strconv.FormatInt(userID, 10)
	for _, p := range points {
		point := ingest.CanonicalPoint{
			UserID:       userIDRaw,
			DeviceID:     deviceName,
			SourceType:   "owntracks",
			TimestampUTC: p.at.UTC(),
			Lat:          p.lat,
			Lon:          p.lon,
		}
		point.IngestHash = ingest.GenerateDeterministicIngestHash(point)
		records = append(records, ingest.SpoolRecord{
			Seq:        seq,
			DeviceID:   deviceName,
			ReceivedAt: p.at.UTC(),
			Point:      point,
		})
		seq++
	}
	if _, err := rt.sqliteStore.InsertSpoolBatch(records); err != nil {
		t.Fatalf("InsertSpoolBatch failed: %v", err)
	}
	return seq
}

func assertVisitCountByDevice(t *testing.T, dbPath string, deviceID int64, expected int) {
	t.Helper()
	got := queryInt(t, dbPath, `SELECT COUNT(*) FROM visits WHERE device_id = ?;`, deviceID)
	if got != expected {
		t.Fatalf("visit count mismatch for device_id=%d: got=%d expected=%d", deviceID, got, expected)
	}
}

func assertWatermarkSeq(t *testing.T, rt *integrationRuntime, deviceID int64, expected uint64) {
	t.Helper()
	state, ok, err := rt.sqliteStore.GetVisitGenerationState(context.Background(), deviceID)
	if err != nil {
		t.Fatalf("GetVisitGenerationState device_id=%d failed: %v", deviceID, err)
	}
	if !ok {
		t.Fatalf("expected visit generation state for device_id=%d", deviceID)
	}
	if state.LastProcessedSeq != expected {
		t.Fatalf(
			"unexpected watermark for device_id=%d: got=%d expected=%d",
			deviceID,
			state.LastProcessedSeq,
			expected,
		)
	}
	maxSeq, hasPoints, err := rt.sqliteStore.GetMaxPointSeqForDevice(context.Background(), deviceID)
	if err != nil {
		t.Fatalf("GetMaxPointSeqForDevice device_id=%d failed: %v", deviceID, err)
	}
	if hasPoints && maxSeq < state.LastProcessedSeq {
		t.Fatalf(
			"watermark exceeds max seq for device_id=%d: watermark=%d max_seq=%d",
			deviceID,
			state.LastProcessedSeq,
			maxSeq,
		)
	}
}
