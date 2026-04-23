package tasks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"plexplore/internal/api"
	"plexplore/internal/buffer"
	"plexplore/internal/flusher"
	"plexplore/internal/ingest"
	"plexplore/internal/spool"
	"plexplore/internal/store"
)

type integrationEnv struct {
	t             *testing.T
	spoolDir      string
	dbPath        string
	segmentMax    int
	flushBatch    int
	spoolManager  *spool.FileSpoolManager
	sqliteStore   *store.SQLiteStore
	bufferManager *buffer.Manager
	batchFlusher  *flusher.Flusher
	mux           *http.ServeMux
}

type integrationRuntime struct {
	spoolManager  *spool.FileSpoolManager
	sqliteStore   *store.SQLiteStore
	bufferManager *buffer.Manager
	batchFlusher  *flusher.Flusher
}

type failOnceStore struct {
	inner             *store.SQLiteStore
	remainingFailures int
}

func (s *failOnceStore) InsertSpoolBatch(records []ingest.SpoolRecord) (uint64, error) {
	if s.remainingFailures > 0 {
		s.remainingFailures--
		return 0, errors.New("injected sqlite failure")
	}
	return s.inner.InsertSpoolBatch(records)
}

func TestIntegration_OwnTracksIngestToSQLite(t *testing.T) {
	env := newIntegrationEnv(t, integrationOptions{})

	createDevice(t, env.sqliteStore, "phone-main", "owntracks", "k-owntracks")

	resp := env.postJSON("/api/v1/owntracks", "k-owntracks", ownTracksPayload(41.12345, -87.54321, 1713777600))
	if resp.Code != http.StatusOK {
		t.Fatalf("owntracks ingest expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}

	replayedBeforeFlush, err := env.spoolManager.ReplayAfterCheckpoint()
	if err != nil {
		t.Fatalf("ReplayAfterCheckpoint before flush failed: %v", err)
	}
	if len(replayedBeforeFlush) != 1 || replayedBeforeFlush[0].Seq != 1 {
		t.Fatalf("expected one replayable record seq=1 before flush, got %+v", replayedBeforeFlush)
	}

	if got := env.bufferManager.Stats().TotalBufferedPoints; got != 1 {
		t.Fatalf("expected buffer points=1 before flush, got %d", got)
	}

	if err := env.batchFlusher.FlushNow(); err != nil {
		t.Fatalf("FlushNow failed: %v", err)
	}

	if got := countTableRows(t, env.dbPath, "raw_points"); got != 1 {
		t.Fatalf("expected raw_points=1 after flush, got %d", got)
	}
	if got := countTableRows(t, env.dbPath, "points"); got != 1 {
		t.Fatalf("expected points=1 after flush, got %d", got)
	}
	if got := readCheckpointSeq(t, env.spoolManager); got != 1 {
		t.Fatalf("expected checkpoint=1 after flush, got %d", got)
	}
	if got := env.bufferManager.Stats().TotalBufferedPoints; got != 0 {
		t.Fatalf("expected empty buffer after flush, got %d", got)
	}
}

func TestIntegration_OverlandIngestToSQLite(t *testing.T) {
	env := newIntegrationEnv(t, integrationOptions{})

	createDevice(t, env.sqliteStore, "ios-main", "overland", "k-overland")

	overlandBody := `{"device_id":"phone-01","locations":[{"coordinates":[-87.10001,41.90001],"timestamp":"2026-04-22T12:00:00Z","horizontal_accuracy":6.5}]}`
	resp := env.postJSON("/api/v1/overland/batches", "k-overland", overlandBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("overland ingest expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}

	if got := env.bufferManager.Stats().TotalBufferedPoints; got != 1 {
		t.Fatalf("expected buffer points=1 before flush, got %d", got)
	}
	if err := env.batchFlusher.FlushNow(); err != nil {
		t.Fatalf("FlushNow failed: %v", err)
	}

	if got := countTableRows(t, env.dbPath, "raw_points"); got != 1 {
		t.Fatalf("expected raw_points=1 after flush, got %d", got)
	}
	if source := queryString(t, env.dbPath, `SELECT source_type FROM raw_points WHERE seq = 1;`); source != "overland" {
		t.Fatalf("expected source_type=overland, got %q", source)
	}
	if got := readCheckpointSeq(t, env.spoolManager); got != 1 {
		t.Fatalf("expected checkpoint=1 after flush, got %d", got)
	}
}

func TestIntegration_DuplicateIngestDoesNotDuplicateSQLiteRows(t *testing.T) {
	env := newIntegrationEnv(t, integrationOptions{})

	createDevice(t, env.sqliteStore, "dup-device", "owntracks", "k-dup")
	payload := ownTracksPayload(41.00001, -87.00001, 1713777600)

	first := env.postJSON("/api/v1/owntracks", "k-dup", payload)
	if first.Code != http.StatusOK {
		t.Fatalf("first ingest expected 200, got %d body=%s", first.Code, first.Body.String())
	}
	if err := env.batchFlusher.FlushNow(); err != nil {
		t.Fatalf("first FlushNow failed: %v", err)
	}

	second := env.postJSON("/api/v1/owntracks", "k-dup", payload)
	if second.Code != http.StatusOK {
		t.Fatalf("second ingest expected 200, got %d body=%s", second.Code, second.Body.String())
	}
	if err := env.batchFlusher.FlushNow(); err != nil {
		t.Fatalf("second FlushNow failed: %v", err)
	}

	if got := countTableRows(t, env.dbPath, "raw_points"); got != 1 {
		t.Fatalf("expected deduplicated raw_points=1, got %d", got)
	}
	if got := countTableRows(t, env.dbPath, "points"); got != 1 {
		t.Fatalf("expected deduplicated points=1, got %d", got)
	}
	if got := readCheckpointSeq(t, env.spoolManager); got != 2 {
		t.Fatalf("expected checkpoint to advance through duplicate seq=2, got %d", got)
	}

	replayedAfterSecondFlush, err := env.spoolManager.ReplayAfterCheckpoint()
	if err != nil {
		t.Fatalf("ReplayAfterCheckpoint failed: %v", err)
	}
	if len(replayedAfterSecondFlush) != 0 {
		t.Fatalf("expected no replay-pending duplicates after normal flush, got %+v", replayedAfterSecondFlush)
	}
}

func TestIntegration_DuplicateIngestCheckpointProgressPersistsAcrossRestart(t *testing.T) {
	env := newIntegrationEnv(t, integrationOptions{})

	createDevice(t, env.sqliteStore, "dup-restart", "owntracks", "k-dup-restart")
	payload := ownTracksPayload(41.00001, -87.00001, 1713777600)

	first := env.postJSON("/api/v1/owntracks", "k-dup-restart", payload)
	if first.Code != http.StatusOK {
		t.Fatalf("first ingest expected 200, got %d body=%s", first.Code, first.Body.String())
	}
	second := env.postJSON("/api/v1/owntracks", "k-dup-restart", payload)
	if second.Code != http.StatusOK {
		t.Fatalf("second ingest expected 200, got %d body=%s", second.Code, second.Body.String())
	}
	if err := env.batchFlusher.FlushNow(); err != nil {
		t.Fatalf("FlushNow failed: %v", err)
	}

	if got := countTableRows(t, env.dbPath, "raw_points"); got != 1 {
		t.Fatalf("expected deduplicated raw_points=1 before restart, got %d", got)
	}
	if got := readCheckpointSeq(t, env.spoolManager); got != 2 {
		t.Fatalf("expected checkpoint=2 before restart, got %d", got)
	}

	env.close()

	restarted := openIntegrationRuntime(t, env.spoolDir, env.dbPath, env.segmentMax, env.flushBatch, nil)
	t.Cleanup(func() { restarted.close() })

	result, err := RecoverFromSpool(
		restarted.spoolManager,
		restarted.bufferManager,
		restarted.batchFlusher,
		RecoveryConfig{EnqueueBatchSize: 8},
	)
	if err != nil {
		t.Fatalf("RecoverFromSpool failed: %v", err)
	}
	if result.Replayed != 0 || result.Enqueued != 0 {
		t.Fatalf("expected no replay after checkpoint progressed through duplicate, got replayed=%d enqueued=%d", result.Replayed, result.Enqueued)
	}
	if got := countTableRows(t, env.dbPath, "raw_points"); got != 1 {
		t.Fatalf("expected deduplicated raw_points=1 after restart recovery, got %d", got)
	}
	if got := readCheckpointSeq(t, restarted.spoolManager); got != 2 {
		t.Fatalf("expected checkpoint=2 after restart recovery, got %d", got)
	}
}

func TestIntegration_CheckpointAdvancesOnlyAfterSuccessfulSQLiteCommit(t *testing.T) {
	env := newIntegrationEnv(t, integrationOptions{})
	createDevice(t, env.sqliteStore, "checkpoint-device", "owntracks", "k-check")

	resp := env.postJSON("/api/v1/owntracks", "k-check", ownTracksPayload(41.2, -87.2, 1713777600))
	if resp.Code != http.StatusOK {
		t.Fatalf("ingest expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}

	injectedStore := &failOnceStore{
		inner:             env.sqliteStore,
		remainingFailures: 1,
	}
	failingFlusher := flusher.New(injectedStore, env.spoolManager, env.bufferManager, flusher.Config{
		FlushInterval:  time.Hour,
		FlushBatchSize: env.flushBatch,
	})

	if err := failingFlusher.FlushNow(); err == nil {
		t.Fatal("expected first FlushNow to fail with injected sqlite failure")
	}
	if got := countTableRows(t, env.dbPath, "raw_points"); got != 0 {
		t.Fatalf("expected raw_points=0 after failed flush, got %d", got)
	}
	if got := readCheckpointSeq(t, env.spoolManager); got != 0 {
		t.Fatalf("expected checkpoint to remain 0 after failed flush, got %d", got)
	}
	if got := env.bufferManager.Stats().TotalBufferedPoints; got != 1 {
		t.Fatalf("expected drained batch to be requeued (buffer points=1), got %d", got)
	}

	if err := failingFlusher.FlushNow(); err != nil {
		t.Fatalf("expected second FlushNow to succeed, got %v", err)
	}
	if got := countTableRows(t, env.dbPath, "raw_points"); got != 1 {
		t.Fatalf("expected raw_points=1 after successful retry, got %d", got)
	}
	if got := readCheckpointSeq(t, env.spoolManager); got != 1 {
		t.Fatalf("expected checkpoint=1 after successful commit, got %d", got)
	}
}

func TestIntegration_StartupRecoveryReplaysAfterCrashBeforeFlush(t *testing.T) {
	env := newIntegrationEnv(t, integrationOptions{})
	createDevice(t, env.sqliteStore, "recover-device", "owntracks", "k-recover")

	resp := env.postJSON("/api/v1/owntracks", "k-recover", ownTracksPayload(41.4, -87.4, 1713777600))
	if resp.Code != http.StatusOK {
		t.Fatalf("ingest expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}
	if got := countTableRows(t, env.dbPath, "raw_points"); got != 0 {
		t.Fatalf("expected no rows before crash/recovery flush, got %d", got)
	}
	if got := readCheckpointSeq(t, env.spoolManager); got != 0 {
		t.Fatalf("expected checkpoint=0 before crash/recovery, got %d", got)
	}

	env.close()

	restarted := openIntegrationRuntime(t, env.spoolDir, env.dbPath, env.segmentMax, env.flushBatch, nil)
	t.Cleanup(func() { restarted.close() })

	result, err := RecoverFromSpool(
		restarted.spoolManager,
		restarted.bufferManager,
		restarted.batchFlusher,
		RecoveryConfig{EnqueueBatchSize: 8},
	)
	if err != nil {
		t.Fatalf("RecoverFromSpool failed: %v", err)
	}
	if result.Replayed != 1 || result.Enqueued != 1 {
		t.Fatalf("expected replayed=1 enqueued=1, got replayed=%d enqueued=%d", result.Replayed, result.Enqueued)
	}
	if got := countTableRows(t, env.dbPath, "raw_points"); got != 1 {
		t.Fatalf("expected raw_points=1 after recovery flush, got %d", got)
	}
	if got := readCheckpointSeq(t, restarted.spoolManager); got != 1 {
		t.Fatalf("expected checkpoint=1 after recovery flush, got %d", got)
	}
}

func TestIntegration_SpoolSegmentRolloverReplaysCorrectly(t *testing.T) {
	env := newIntegrationEnv(t, integrationOptions{segmentMaxBytes: 180})
	createDevice(t, env.sqliteStore, "rollover-device", "owntracks", "k-roll")

	baseTST := int64(1713777600)
	for i := 0; i < 6; i++ {
		payload := ownTracksPayload(
			41.50000+float64(i)*0.01000,
			-87.50000-float64(i)*0.01000,
			baseTST+int64(i*60),
		)
		resp := env.postJSON("/api/v1/owntracks", "k-roll", payload)
		if resp.Code != http.StatusOK {
			t.Fatalf("ingest %d expected 200, got %d body=%s", i, resp.Code, resp.Body.String())
		}
	}

	segmentsBefore, err := env.spoolManager.SegmentCount()
	if err != nil {
		t.Fatalf("SegmentCount failed: %v", err)
	}
	if segmentsBefore < 2 {
		t.Fatalf("expected segment rollover before recovery, got %d segment(s)", segmentsBefore)
	}

	env.close()

	restarted := openIntegrationRuntime(t, env.spoolDir, env.dbPath, env.segmentMax, env.flushBatch, nil)
	t.Cleanup(func() { restarted.close() })

	result, err := RecoverFromSpool(
		restarted.spoolManager,
		restarted.bufferManager,
		restarted.batchFlusher,
		RecoveryConfig{EnqueueBatchSize: 2},
	)
	if err != nil {
		t.Fatalf("RecoverFromSpool failed: %v", err)
	}
	if result.Replayed != 6 || result.Enqueued != 6 {
		t.Fatalf("expected replayed=6 enqueued=6, got replayed=%d enqueued=%d", result.Replayed, result.Enqueued)
	}
	if got := countTableRows(t, env.dbPath, "raw_points"); got != 6 {
		t.Fatalf("expected raw_points=6 after rollover recovery, got %d", got)
	}
	if got := readCheckpointSeq(t, restarted.spoolManager); got != 6 {
		t.Fatalf("expected checkpoint=6 after rollover recovery, got %d", got)
	}
}

func TestIntegration_DeviceAPIKeyIngestPersistsUnderCorrectOwnerAndDevice(t *testing.T) {
	env := newIntegrationEnv(t, integrationOptions{})

	user2ID := createUser(t, env.sqliteStore, "user2@example.com")
	user3ID := createUser(t, env.sqliteStore, "user3@example.com")
	device2 := createDeviceForUser(t, env.sqliteStore, user2ID, "shared-phone", "owntracks", "k-u2")
	device3 := createDeviceForUser(t, env.sqliteStore, user3ID, "shared-phone", "owntracks", "k-u3")

	respU2 := env.postJSON("/api/v1/owntracks", "k-u2", ownTracksPayload(41.61000, -87.61000, 1713777600))
	if respU2.Code != http.StatusOK {
		t.Fatalf("u2 ingest expected 200, got %d body=%s", respU2.Code, respU2.Body.String())
	}
	respU3 := env.postJSON("/api/v1/owntracks", "k-u3", ownTracksPayload(41.62000, -87.62000, 1713777660))
	if respU3.Code != http.StatusOK {
		t.Fatalf("u3 ingest expected 200, got %d body=%s", respU3.Code, respU3.Body.String())
	}
	if err := env.batchFlusher.FlushNow(); err != nil {
		t.Fatalf("FlushNow failed: %v", err)
	}

	if got := queryInt(t, env.dbPath, `SELECT COUNT(*) FROM raw_points WHERE device_id = ?;`, device2.ID); got != 1 {
		t.Fatalf("expected one raw point for user2 device, got %d", got)
	}
	if got := queryInt(t, env.dbPath, `SELECT COUNT(*) FROM raw_points WHERE device_id = ?;`, device3.ID); got != 1 {
		t.Fatalf("expected one raw point for user3 device, got %d", got)
	}
	if got := queryInt(t, env.dbPath, `SELECT COUNT(*) FROM raw_points WHERE user_id != ? AND device_id = ?;`, user2ID, device2.ID); got != 0 {
		t.Fatalf("expected no cross-user contamination for user2 device, got %d", got)
	}
	if got := queryInt(t, env.dbPath, `SELECT COUNT(*) FROM raw_points WHERE user_id != ? AND device_id = ?;`, user3ID, device3.ID); got != 0 {
		t.Fatalf("expected no cross-user contamination for user3 device, got %d", got)
	}
	if got := queryInt(t, env.dbPath, `SELECT COUNT(*) FROM raw_points rp JOIN devices d ON rp.device_id = d.id WHERE rp.user_id != d.user_id;`); got != 0 {
		t.Fatalf("expected raw_points.user_id to match devices.user_id for all ingested rows, got mismatches=%d", got)
	}
	if got := queryInt(t, env.dbPath, `SELECT COUNT(*) FROM devices;`); got != 2 {
		t.Fatalf("expected exactly two managed devices (no fallback auto devices), got %d", got)
	}
}

func TestIntegration_InvalidDeviceAPIKeyRejected_NoDataPersisted(t *testing.T) {
	env := newIntegrationEnv(t, integrationOptions{})

	createDevice(t, env.sqliteStore, "phone-main", "owntracks", "k-valid")

	resp := env.postJSON("/api/v1/owntracks", "k-invalid", ownTracksPayload(41.70000, -87.70000, 1713777600))
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("invalid API key ingest expected 401, got %d body=%s", resp.Code, resp.Body.String())
	}
	if got := env.bufferManager.Stats().TotalBufferedPoints; got != 0 {
		t.Fatalf("expected no buffered points for invalid API key, got %d", got)
	}
	if got := countTableRows(t, env.dbPath, "raw_points"); got != 0 {
		t.Fatalf("expected raw_points=0 after invalid API key request, got %d", got)
	}
	if got := countTableRows(t, env.dbPath, "points"); got != 0 {
		t.Fatalf("expected points=0 after invalid API key request, got %d", got)
	}
}

type integrationOptions struct {
	segmentMaxBytes int
	flushBatchSize  int
	flusherStore    flusher.Store
}

func newIntegrationEnv(t *testing.T, opts integrationOptions) *integrationEnv {
	t.Helper()

	baseDir := t.TempDir()
	spoolDir := filepath.Join(baseDir, "spool")
	dbPath := filepath.Join(baseDir, "tracker.db")

	if err := applyTestSchema(dbPath); err != nil {
		t.Fatalf("apply test schema: %v", err)
	}

	if opts.segmentMaxBytes <= 0 {
		opts.segmentMaxBytes = 1024 * 1024
	}
	if opts.flushBatchSize <= 0 {
		opts.flushBatchSize = 64
	}

	runtime := openIntegrationRuntime(t, spoolDir, dbPath, opts.segmentMaxBytes, opts.flushBatchSize, opts.flusherStore)
	env := &integrationEnv{
		t:             t,
		spoolDir:      spoolDir,
		dbPath:        dbPath,
		segmentMax:    opts.segmentMaxBytes,
		flushBatch:    opts.flushBatchSize,
		spoolManager:  runtime.spoolManager,
		sqliteStore:   runtime.sqliteStore,
		bufferManager: runtime.bufferManager,
		batchFlusher:  runtime.batchFlusher,
	}

	mux := http.NewServeMux()
	api.RegisterRoutesWithDependencies(mux, api.Dependencies{
		DeviceStore: env.sqliteStore,
		Spool:       env.spoolManager,
		Buffer:      env.bufferManager,
		Flusher:     env.batchFlusher,
	})
	env.mux = mux

	t.Cleanup(func() { env.close() })
	return env
}

func openIntegrationRuntime(
	t *testing.T,
	spoolDir string,
	dbPath string,
	segmentMaxBytes int,
	flushBatchSize int,
	flusherStore flusher.Store,
) *integrationRuntime {
	t.Helper()

	spoolManager := spool.NewFileSpoolManagerWithOptions(spoolDir, segmentMaxBytes, spool.ManagerOptions{
		FSyncMode:          spool.FSyncModeLowWear,
		FSyncInterval:      time.Second,
		FSyncByteThreshold: 1024,
	})

	sqliteStore, err := store.OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore failed: %v", err)
	}
	bufferManager := buffer.NewManager(256, 512*1024)

	storeForFlusher := flusherStore
	if storeForFlusher == nil {
		storeForFlusher = sqliteStore
	}
	batchFlusher := flusher.New(storeForFlusher, spoolManager, bufferManager, flusher.Config{
		FlushInterval:  time.Hour,
		FlushBatchSize: flushBatchSize,
	})

	return &integrationRuntime{
		spoolManager:  spoolManager,
		sqliteStore:   sqliteStore,
		bufferManager: bufferManager,
		batchFlusher:  batchFlusher,
	}
}

func (r *integrationRuntime) close() {
	if r == nil {
		return
	}
	if r.spoolManager != nil {
		_ = r.spoolManager.Close()
	}
	if r.sqliteStore != nil {
		_ = r.sqliteStore.Close()
	}
}

func (e *integrationEnv) close() {
	if e == nil {
		return
	}
	if e.spoolManager != nil {
		_ = e.spoolManager.Close()
		e.spoolManager = nil
	}
	if e.sqliteStore != nil {
		_ = e.sqliteStore.Close()
		e.sqliteStore = nil
	}
}

func (e *integrationEnv) postJSON(path, apiKey, body string) *httptest.ResponseRecorder {
	e.t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func createDevice(t *testing.T, deviceStore *store.SQLiteStore, name, sourceType, apiKey string) {
	t.Helper()

	_, err := deviceStore.CreateDevice(context.Background(), store.CreateDeviceParams{
		Name:       name,
		SourceType: sourceType,
		APIKey:     apiKey,
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
}

func createDeviceForUser(t *testing.T, deviceStore *store.SQLiteStore, userID int64, name, sourceType, apiKey string) store.Device {
	t.Helper()

	device, err := deviceStore.CreateDevice(context.Background(), store.CreateDeviceParams{
		UserID:     userID,
		Name:       name,
		SourceType: sourceType,
		APIKey:     apiKey,
	})
	if err != nil {
		t.Fatalf("CreateDevice for user %d failed: %v", userID, err)
	}
	return device
}

func createUser(t *testing.T, userStore *store.SQLiteStore, email string) int64 {
	t.Helper()

	user, err := userStore.CreateUser(context.Background(), store.CreateUserParams{
		Name:         email,
		Email:        email,
		PasswordHash: "hash-for-tests",
		IsAdmin:      false,
	})
	if err != nil {
		t.Fatalf("CreateUser %q failed: %v", email, err)
	}
	return user.ID
}

func ownTracksPayload(lat, lon float64, tst int64) string {
	return fmt.Sprintf(`{"_type":"location","lat":%.5f,"lon":%.5f,"tst":%d}`, lat, lon, tst)
}

func readCheckpointSeq(t *testing.T, spoolManager *spool.FileSpoolManager) uint64 {
	t.Helper()

	checkpoint, err := spoolManager.ReadCheckpoint()
	if err != nil {
		t.Fatalf("ReadCheckpoint failed: %v", err)
	}
	return checkpoint.LastCommittedSeq
}

func countTableRows(t *testing.T, dbPath, table string) int {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db %q failed: %v", dbPath, err)
	}
	defer db.Close()

	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s;", table)
	if err := db.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("count rows from %s failed: %v", table, err)
	}
	return count
}

func queryString(t *testing.T, dbPath, query string) string {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db %q failed: %v", dbPath, err)
	}
	defer db.Close()

	var value string
	if err := db.QueryRow(query).Scan(&value); err != nil {
		t.Fatalf("query string failed: %v", err)
	}
	return value
}

func queryInt(t *testing.T, dbPath, query string, args ...interface{}) int {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db %q failed: %v", dbPath, err)
	}
	defer db.Close()

	var value int
	if err := db.QueryRow(query, args...).Scan(&value); err != nil {
		t.Fatalf("query int failed: %v", err)
	}
	return value
}

func applyTestSchema(dbPath string) error {
	migration, err := readMigrationSQL()
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite db: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(migration); err != nil {
		return fmt.Errorf("exec migration SQL: %w", err)
	}
	return nil
}

func readMigrationSQL() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("resolve caller path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	migrationsDir := filepath.Join(root, "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return "", fmt.Errorf("read migrations dir %q: %w", migrationsDir, err)
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
			return "", fmt.Errorf("read migration SQL %q: %w", sqlFile, readErr)
		}
		builder.Write(data)
		builder.WriteString("\n")
	}
	return builder.String(), nil
}
