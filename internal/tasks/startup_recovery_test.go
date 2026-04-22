package tasks

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"plexplore/internal/buffer"
	"plexplore/internal/flusher"
	"plexplore/internal/ingest"
	"plexplore/internal/spool"
	"plexplore/internal/store"
)

func migrationSQL(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve caller path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	sqlPath := filepath.Join(root, "migrations", "0001_init_schema.sql")

	data, err := os.ReadFile(sqlPath)
	if err != nil {
		t.Fatalf("read migration SQL %q: %v", sqlPath, err)
	}
	return string(data)
}

func applySchema(t *testing.T, dbPath string) {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db %q: %v", dbPath, err)
	}
	defer db.Close()

	if _, err := db.Exec(migrationSQL(t)); err != nil {
		t.Fatalf("apply schema failed: %v", err)
	}
}

func countRawPoints(t *testing.T, dbPath string) int {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db %q: %v", dbPath, err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM raw_points;`).Scan(&count); err != nil {
		t.Fatalf("count raw_points failed: %v", err)
	}
	return count
}

func makePoint(seq int) ingest.CanonicalPoint {
	return ingest.CanonicalPoint{
		DeviceID:     "recovery-device",
		SourceType:   "owntracks",
		TimestampUTC: time.Date(2026, 4, 21, 22, 0, seq, 0, time.UTC),
		Lat:          37.42 + float64(seq)*0.001,
		Lon:          -122.08 - float64(seq)*0.001,
	}
}

func TestRecoverFromSpool_RestartAfterAppendBeforeCommit(t *testing.T) {
	baseDir := t.TempDir()
	spoolDir := filepath.Join(baseDir, "spool")
	dbPath := filepath.Join(baseDir, "tracker.db")

	spoolA := spool.NewFileSpoolManagerWithOptions(spoolDir, 1024*1024, spool.ManagerOptions{
		FSyncMode:          spool.FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})
	if _, err := spoolA.AppendCanonicalPoints([]ingest.CanonicalPoint{
		makePoint(1),
		makePoint(2),
	}); err != nil {
		t.Fatalf("append canonical points failed: %v", err)
	}
	if err := spoolA.Close(); err != nil {
		t.Fatalf("close spoolA failed: %v", err)
	}

	applySchema(t, dbPath)

	spoolB := spool.NewFileSpoolManagerWithOptions(spoolDir, 1024*1024, spool.ManagerOptions{
		FSyncMode:          spool.FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})
	t.Cleanup(func() { _ = spoolB.Close() })

	sqlStore, err := store.OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore failed: %v", err)
	}
	t.Cleanup(func() { _ = sqlStore.Close() })

	buf := buffer.NewManager(32, 128*1024)
	batchFlusher := flusher.New(sqlStore, spoolB, buf, flusher.Config{
		FlushInterval:  time.Minute,
		FlushBatchSize: 32,
	})

	result, err := RecoverFromSpool(spoolB, buf, batchFlusher, RecoveryConfig{EnqueueBatchSize: 8})
	if err != nil {
		t.Fatalf("RecoverFromSpool failed: %v", err)
	}
	if result.Replayed != 2 {
		t.Fatalf("expected replayed=2, got %d", result.Replayed)
	}
	if result.Enqueued != 2 {
		t.Fatalf("expected enqueued=2, got %d", result.Enqueued)
	}

	if got := countRawPoints(t, dbPath); got != 2 {
		t.Fatalf("expected 2 raw_points after recovery flush, got %d", got)
	}

	checkpoint, err := spoolB.ReadCheckpoint()
	if err != nil {
		t.Fatalf("ReadCheckpoint failed: %v", err)
	}
	if checkpoint.LastCommittedSeq != 2 {
		t.Fatalf("expected checkpoint seq=2, got %d", checkpoint.LastCommittedSeq)
	}
}

func TestRecoverFromSpool_RestartAfterCommitAndCheckpoint(t *testing.T) {
	baseDir := t.TempDir()
	spoolDir := filepath.Join(baseDir, "spool")
	dbPath := filepath.Join(baseDir, "tracker.db")

	applySchema(t, dbPath)

	spoolA := spool.NewFileSpoolManagerWithOptions(spoolDir, 1024*1024, spool.ManagerOptions{
		FSyncMode:          spool.FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})

	sqlStoreA, err := store.OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore A failed: %v", err)
	}

	appended, err := spoolA.AppendCanonicalPoints([]ingest.CanonicalPoint{
		makePoint(1),
		makePoint(2),
	})
	if err != nil {
		t.Fatalf("append canonical points failed: %v", err)
	}

	bufA := buffer.NewManager(32, 128*1024)
	if err := bufA.Enqueue(appended); err != nil {
		t.Fatalf("enqueue appended records failed: %v", err)
	}
	flusherA := flusher.New(sqlStoreA, spoolA, bufA, flusher.Config{
		FlushInterval:  time.Minute,
		FlushBatchSize: 32,
	})
	if err := flusherA.FlushNow(); err != nil {
		t.Fatalf("initial FlushNow failed: %v", err)
	}

	if err := sqlStoreA.Close(); err != nil {
		t.Fatalf("close sqlStoreA failed: %v", err)
	}
	if err := spoolA.Close(); err != nil {
		t.Fatalf("close spoolA failed: %v", err)
	}

	spoolB := spool.NewFileSpoolManagerWithOptions(spoolDir, 1024*1024, spool.ManagerOptions{
		FSyncMode:          spool.FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})
	t.Cleanup(func() { _ = spoolB.Close() })

	sqlStoreB, err := store.OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore B failed: %v", err)
	}
	t.Cleanup(func() { _ = sqlStoreB.Close() })

	bufB := buffer.NewManager(32, 128*1024)
	flusherB := flusher.New(sqlStoreB, spoolB, bufB, flusher.Config{
		FlushInterval:  time.Minute,
		FlushBatchSize: 32,
	})

	result, err := RecoverFromSpool(spoolB, bufB, flusherB, RecoveryConfig{EnqueueBatchSize: 8})
	if err != nil {
		t.Fatalf("RecoverFromSpool failed: %v", err)
	}
	if result.Replayed != 0 {
		t.Fatalf("expected replayed=0 after committed restart, got %d", result.Replayed)
	}

	if got := countRawPoints(t, dbPath); got != 2 {
		t.Fatalf("expected raw_points to remain 2, got %d", got)
	}

	checkpoint, err := spoolB.ReadCheckpoint()
	if err != nil {
		t.Fatalf("ReadCheckpoint failed: %v", err)
	}
	if checkpoint.LastCommittedSeq != 2 {
		t.Fatalf("expected checkpoint seq=2, got %d", checkpoint.LastCommittedSeq)
	}
}
