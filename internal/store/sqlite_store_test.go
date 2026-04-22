package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"plexplore/internal/ingest"
)

func loadMigrationSQL(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve caller path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	migrationsDir := filepath.Join(root, "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations dir %q: %v", migrationsDir, err)
	}

	sqlFiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".sql") {
			sqlFiles = append(sqlFiles, filepath.Join(migrationsDir, entry.Name()))
		}
	}
	slices.Sort(sqlFiles)

	var builder strings.Builder
	for _, sqlFile := range sqlFiles {
		data, readErr := os.ReadFile(sqlFile)
		if readErr != nil {
			t.Fatalf("read migration SQL %q: %v", sqlFile, readErr)
		}
		builder.Write(data)
		builder.WriteString("\n")
	}
	return builder.String()
}

func openStoreWithSchema(t *testing.T) *SQLiteStore {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "tracker.db")
	store, err := OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := store.db.Exec(loadMigrationSQL(t)); err != nil {
		t.Fatalf("apply test schema failed: %v", err)
	}
	return store
}

func testSpoolRecord(seq uint64, deviceID, hash string, ts time.Time) ingest.SpoolRecord {
	return ingest.SpoolRecord{
		Seq:        seq,
		DeviceID:   deviceID,
		ReceivedAt: ts,
		Point: ingest.CanonicalPoint{
			DeviceID:     deviceID,
			SourceType:   "owntracks",
			TimestampUTC: ts,
			Lat:          37.42 + float64(seq)*0.0001,
			Lon:          -122.08 - float64(seq)*0.0001,
			IngestHash:   hash,
		},
	}
}

func TestSQLiteStore_InsertSpoolBatch_Success(t *testing.T) {
	store := openStoreWithSchema(t)
	base := time.Date(2026, 4, 21, 20, 0, 0, 0, time.UTC)

	records := []ingest.SpoolRecord{
		testSpoolRecord(1, "d1", "hash-1", base),
		testSpoolRecord(2, "d1", "hash-2", base.Add(time.Second)),
		testSpoolRecord(3, "d1", "hash-3", base.Add(2*time.Second)),
	}

	maxSeq, err := store.InsertSpoolBatch(records)
	if err != nil {
		t.Fatalf("InsertSpoolBatch failed: %v", err)
	}
	if maxSeq != 3 {
		t.Fatalf("expected max committed seq 3, got %d", maxSeq)
	}

	var rawCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM raw_points;`).Scan(&rawCount); err != nil {
		t.Fatalf("count raw_points failed: %v", err)
	}
	if rawCount != 3 {
		t.Fatalf("expected 3 raw_points, got %d", rawCount)
	}

	var pointsCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM points;`).Scan(&pointsCount); err != nil {
		t.Fatalf("count points failed: %v", err)
	}
	if pointsCount != 3 {
		t.Fatalf("expected 3 points, got %d", pointsCount)
	}

	var lastSeq int
	var lastSeen string
	if err := store.db.QueryRow(`
SELECT last_seq_received, COALESCE(last_seen_at, '')
FROM devices
WHERE api_key = 'auto:d1';
`).Scan(&lastSeq, &lastSeen); err != nil {
		t.Fatalf("query device status failed: %v", err)
	}
	if lastSeq != 3 {
		t.Fatalf("expected last_seq_received 3, got %d", lastSeq)
	}
	if lastSeen == "" {
		t.Fatal("expected non-empty last_seen_at")
	}
}

func TestSQLiteStore_InsertSpoolBatch_ReplayDuplicates(t *testing.T) {
	store := openStoreWithSchema(t)
	base := time.Date(2026, 4, 21, 20, 0, 0, 0, time.UTC)

	records := []ingest.SpoolRecord{
		testSpoolRecord(1, "d1", "hash-r1", base),
		testSpoolRecord(2, "d1", "hash-r2", base.Add(time.Second)),
		testSpoolRecord(3, "d1", "hash-r3", base.Add(2*time.Second)),
	}

	if _, err := store.InsertSpoolBatch(records); err != nil {
		t.Fatalf("initial insert failed: %v", err)
	}
	maxSeq, err := store.InsertSpoolBatch(records)
	if err != nil {
		t.Fatalf("replay insert failed: %v", err)
	}
	if maxSeq != 3 {
		t.Fatalf("expected max committed seq 3 on replay, got %d", maxSeq)
	}

	var rawCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM raw_points;`).Scan(&rawCount); err != nil {
		t.Fatalf("count raw_points failed: %v", err)
	}
	if rawCount != 3 {
		t.Fatalf("expected deduplicated raw_points count 3, got %d", rawCount)
	}
}

func TestSQLiteStore_InsertSpoolBatch_PartialDuplicates(t *testing.T) {
	store := openStoreWithSchema(t)
	base := time.Date(2026, 4, 21, 20, 0, 0, 0, time.UTC)

	initial := []ingest.SpoolRecord{
		testSpoolRecord(1, "d1", "hash-p1", base),
		testSpoolRecord(2, "d1", "hash-p2", base.Add(time.Second)),
	}
	if _, err := store.InsertSpoolBatch(initial); err != nil {
		t.Fatalf("initial insert failed: %v", err)
	}

	mixed := []ingest.SpoolRecord{
		testSpoolRecord(2, "d1", "hash-p2", base.Add(time.Second)),   // duplicate
		testSpoolRecord(3, "d1", "hash-p3", base.Add(2*time.Second)), // new
		testSpoolRecord(4, "d1", "hash-p4", base.Add(3*time.Second)), // new
	}
	maxSeq, err := store.InsertSpoolBatch(mixed)
	if err != nil {
		t.Fatalf("mixed insert failed: %v", err)
	}
	if maxSeq != 4 {
		t.Fatalf("expected max committed seq 4, got %d", maxSeq)
	}

	var rawCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM raw_points;`).Scan(&rawCount); err != nil {
		t.Fatalf("count raw_points failed: %v", err)
	}
	if rawCount != 4 {
		t.Fatalf("expected 4 total raw_points after partial duplicates, got %d", rawCount)
	}

	var lastSeq int
	if err := store.db.QueryRow(`SELECT last_seq_received FROM devices WHERE api_key = ?;`, "auto:d1").Scan(&lastSeq); err != nil {
		t.Fatalf("query device last_seq_received failed: %v", err)
	}
	if lastSeq != 4 {
		t.Fatalf("expected device last_seq_received 4, got %d", lastSeq)
	}
}

func TestSQLiteStore_InsertSpoolBatch_MultipleDevices(t *testing.T) {
	store := openStoreWithSchema(t)
	base := time.Date(2026, 4, 21, 20, 0, 0, 0, time.UTC)

	records := []ingest.SpoolRecord{
		testSpoolRecord(1, "d1", "hash-md-1", base),
		testSpoolRecord(2, "d2", "hash-md-2", base.Add(time.Second)),
		testSpoolRecord(3, "d2", "hash-md-3", base.Add(2*time.Second)),
	}
	if _, err := store.InsertSpoolBatch(records); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	for device, want := range map[string]int{"d1": 1, "d2": 3} {
		var got int
		if err := store.db.QueryRow(`SELECT last_seq_received FROM devices WHERE api_key = ?;`, fmt.Sprintf("auto:%s", device)).Scan(&got); err != nil {
			t.Fatalf("query device %s failed: %v", device, err)
		}
		if got != want {
			t.Fatalf("device %s: expected last_seq_received %d, got %d", device, want, got)
		}
	}
}

func TestSQLiteStore_ListRecentPoints(t *testing.T) {
	s := openStoreWithSchema(t)
	base := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

	records := []ingest.SpoolRecord{
		testSpoolRecord(1, "d1", "hash-rp-1", base),
		testSpoolRecord(2, "d1", "hash-rp-2", base.Add(30*time.Second)),
		testSpoolRecord(3, "d2", "hash-rp-3", base.Add(60*time.Second)),
	}
	if _, err := s.InsertSpoolBatch(records); err != nil {
		t.Fatalf("InsertSpoolBatch failed: %v", err)
	}

	all, err := s.ListRecentPoints(context.Background(), "", 2)
	if err != nil {
		t.Fatalf("ListRecentPoints all failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 recent points, got %d", len(all))
	}
	if all[0].Seq != 3 || all[1].Seq != 2 {
		t.Fatalf("unexpected recent ordering: %+v", all)
	}

	filtered, err := s.ListRecentPoints(context.Background(), "d1", 10)
	if err != nil {
		t.Fatalf("ListRecentPoints filtered failed: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 points for d1, got %d", len(filtered))
	}
	for _, p := range filtered {
		if p.DeviceID != "d1" {
			t.Fatalf("expected only device d1 points, got %+v", filtered)
		}
	}
}

func TestSQLiteStore_ListPointsForExport_WithFilters(t *testing.T) {
	s := openStoreWithSchema(t)
	base := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

	records := []ingest.SpoolRecord{
		testSpoolRecord(1, "d1", "hash-exp-1", base),
		testSpoolRecord(2, "d1", "hash-exp-2", base.Add(30*time.Minute)),
		testSpoolRecord(3, "d2", "hash-exp-3", base.Add(60*time.Minute)),
	}
	if _, err := s.InsertSpoolBatch(records); err != nil {
		t.Fatalf("InsertSpoolBatch failed: %v", err)
	}

	from := base.Add(10 * time.Minute)
	to := base.Add(45 * time.Minute)
	points, err := s.ListPointsForExport(context.Background(), ExportPointFilter{
		DeviceID: "d1",
		FromUTC:  &from,
		ToUTC:    &to,
	})
	if err != nil {
		t.Fatalf("ListPointsForExport failed: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 filtered export point, got %d", len(points))
	}
	if points[0].Seq != 2 || points[0].DeviceID != "d1" {
		t.Fatalf("unexpected filtered export point: %+v", points[0])
	}
}
