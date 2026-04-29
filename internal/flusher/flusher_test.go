package flusher

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"plexplore/internal/buffer"
	"plexplore/internal/ingest"
	"plexplore/internal/spool"
)

type fakeBuffer struct {
	mu    sync.Mutex
	queue []ingest.SpoolRecord
}

func (b *fakeBuffer) Enqueue(records []ingest.SpoolRecord) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.queue = append(b.queue, records...)
	return nil
}

func (b *fakeBuffer) DrainBatch(maxPoints int) []ingest.SpoolRecord {
	b.mu.Lock()
	defer b.mu.Unlock()
	if maxPoints <= 0 || len(b.queue) == 0 {
		return nil
	}
	if maxPoints > len(b.queue) {
		maxPoints = len(b.queue)
	}
	out := append([]ingest.SpoolRecord(nil), b.queue[:maxPoints]...)
	b.queue = append([]ingest.SpoolRecord(nil), b.queue[maxPoints:]...)
	return out
}

func (b *fakeBuffer) RequeueFront(records []ingest.SpoolRecord) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	head := append([]ingest.SpoolRecord(nil), records...)
	b.queue = append(head, b.queue...)
	return nil
}

func (b *fakeBuffer) Stats() buffer.Stats {
	b.mu.Lock()
	defer b.mu.Unlock()
	return buffer.Stats{TotalBufferedPoints: len(b.queue)}
}

type fakeStore struct {
	mu      sync.Mutex
	calls   int
	errPlan []error
	seenSeq map[uint64]struct{}
}

func (s *fakeStore) InsertSpoolBatch(records []ingest.SpoolRecord) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if len(s.errPlan) > 0 {
		err := s.errPlan[0]
		s.errPlan = s.errPlan[1:]
		if err != nil {
			return 0, err
		}
	}
	var max uint64
	if s.seenSeq == nil {
		s.seenSeq = make(map[uint64]struct{})
	}
	for _, record := range records {
		s.seenSeq[record.Seq] = struct{}{}
		if record.Seq > max {
			max = record.Seq
		}
	}
	return max, nil
}

type fakeCheckpoint struct {
	mu           sync.Mutex
	advanced     []uint64
	advanceErr   error
	compactCalls int
	compactErr   error
}

func (c *fakeCheckpoint) AdvanceCheckpoint(lastCommittedSeq uint64) (spool.Checkpoint, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.advanceErr != nil {
		return spool.Checkpoint{}, c.advanceErr
	}
	c.advanced = append(c.advanced, lastCommittedSeq)
	return spool.Checkpoint{LastCommittedSeq: lastCommittedSeq}, nil
}

func (c *fakeCheckpoint) CompactCommittedSegments() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.compactCalls++
	if c.compactErr != nil {
		return 0, c.compactErr
	}
	return 1, nil
}

func makeRecords(seqs ...uint64) []ingest.SpoolRecord {
	out := make([]ingest.SpoolRecord, 0, len(seqs))
	for _, seq := range seqs {
		out = append(out, ingest.SpoolRecord{
			Seq:      seq,
			DeviceID: "d1",
			Point: ingest.CanonicalPoint{
				DeviceID:     "d1",
				SourceType:   "owntracks",
				TimestampUTC: time.Date(2026, 4, 21, 20, 0, int(seq), 0, time.UTC),
				Lat:          37.42 + float64(seq)*0.001,
				Lon:          -122.08 - float64(seq)*0.001,
				IngestHash:   "hash",
			},
		})
	}
	return out
}

func TestFlusher_SuccessfulFlush(t *testing.T) {
	buf := &fakeBuffer{queue: makeRecords(1, 2, 3)}
	store := &fakeStore{}
	checkpoint := &fakeCheckpoint{}

	f := New(store, checkpoint, buf, Config{FlushBatchSize: 10, FlushInterval: time.Minute})
	if err := f.FlushNow(); err != nil {
		t.Fatalf("FlushNow failed: %v", err)
	}

	if got := buf.Stats().TotalBufferedPoints; got != 0 {
		t.Fatalf("expected empty buffer, got %d", got)
	}
	if store.calls != 1 {
		t.Fatalf("expected store called once, got %d", store.calls)
	}
	if !slices.Equal(checkpoint.advanced, []uint64{3}) {
		t.Fatalf("expected checkpoint advance [3], got %v", checkpoint.advanced)
	}
	if checkpoint.compactCalls != 1 {
		t.Fatalf("expected one compaction call, got %d", checkpoint.compactCalls)
	}
}

func TestFlusher_FailedFlushDoesNotAdvanceCheckpoint(t *testing.T) {
	buf := &fakeBuffer{queue: makeRecords(1, 2)}
	store := &fakeStore{errPlan: []error{errors.New("db down")}}
	checkpoint := &fakeCheckpoint{}

	f := New(store, checkpoint, buf, Config{FlushBatchSize: 10, FlushInterval: time.Minute})
	if err := f.FlushNow(); err == nil {
		t.Fatal("expected flush error, got nil")
	}

	if got := buf.Stats().TotalBufferedPoints; got != 2 {
		t.Fatalf("expected records requeued (2), got %d", got)
	}
	if len(checkpoint.advanced) != 0 {
		t.Fatalf("expected no checkpoint advancement, got %v", checkpoint.advanced)
	}
	if checkpoint.compactCalls != 0 {
		t.Fatalf("expected no compaction call, got %d", checkpoint.compactCalls)
	}
}

func TestFlusher_CheckpointFailureDoesNotCompact(t *testing.T) {
	buf := &fakeBuffer{queue: makeRecords(1, 2)}
	store := &fakeStore{}
	checkpoint := &fakeCheckpoint{advanceErr: errors.New("checkpoint write failed")}

	f := New(store, checkpoint, buf, Config{FlushBatchSize: 10, FlushInterval: time.Minute})
	if err := f.FlushNow(); err == nil {
		t.Fatal("expected flush error, got nil")
	}

	if len(checkpoint.advanced) != 0 {
		t.Fatalf("expected no checkpoint advancement, got %v", checkpoint.advanced)
	}
	if checkpoint.compactCalls != 0 {
		t.Fatalf("expected no compaction call, got %d", checkpoint.compactCalls)
	}
	if got := buf.Stats().TotalBufferedPoints; got != 2 {
		t.Fatalf("expected drained batch requeued for checkpoint retry, got %d", got)
	}
	if store.calls != 1 {
		t.Fatalf("expected sqlite insert called once before checkpoint failure, got %d", store.calls)
	}
}

func TestFlusher_RetryBehavior(t *testing.T) {
	buf := &fakeBuffer{queue: makeRecords(1, 2)}
	store := &fakeStore{errPlan: []error{errors.New("transient"), nil}}
	checkpoint := &fakeCheckpoint{}

	f := New(store, checkpoint, buf, Config{FlushBatchSize: 10, FlushInterval: time.Minute})

	if err := f.FlushNow(); err == nil {
		t.Fatal("expected first flush to fail")
	}
	if got := buf.Stats().TotalBufferedPoints; got != 2 {
		t.Fatalf("expected buffer retained after failure, got %d", got)
	}

	if err := f.FlushNow(); err != nil {
		t.Fatalf("expected retry flush to succeed, got %v", err)
	}
	if got := buf.Stats().TotalBufferedPoints; got != 0 {
		t.Fatalf("expected empty buffer after retry success, got %d", got)
	}
	if !slices.Equal(checkpoint.advanced, []uint64{2}) {
		t.Fatalf("expected checkpoint advance [2], got %v", checkpoint.advanced)
	}
	if checkpoint.compactCalls != 1 {
		t.Fatalf("expected one compaction call after successful retry, got %d", checkpoint.compactCalls)
	}
}

func TestFlusher_CheckpointFailureRetryEventuallyAdvancesWithoutDuplicateCommits(t *testing.T) {
	buf := &fakeBuffer{queue: makeRecords(1, 2)}
	store := &fakeStore{}
	checkpoint := &fakeCheckpoint{advanceErr: errors.New("checkpoint write failed")}

	f := New(store, checkpoint, buf, Config{FlushBatchSize: 10, FlushInterval: time.Minute})

	if err := f.FlushNow(); err == nil {
		t.Fatal("expected first flush to fail on checkpoint advancement")
	}
	if got := buf.Stats().TotalBufferedPoints; got != 2 {
		t.Fatalf("expected batch retained for retry after checkpoint failure, got %d", got)
	}
	if len(checkpoint.advanced) != 0 {
		t.Fatalf("expected no checkpoint advancement on first attempt, got %v", checkpoint.advanced)
	}
	if checkpoint.compactCalls != 0 {
		t.Fatalf("expected no compaction on checkpoint failure, got %d", checkpoint.compactCalls)
	}

	checkpoint.mu.Lock()
	checkpoint.advanceErr = nil
	checkpoint.mu.Unlock()

	if err := f.FlushNow(); err != nil {
		t.Fatalf("expected second flush to succeed after checkpoint recovers, got %v", err)
	}
	if got := buf.Stats().TotalBufferedPoints; got != 0 {
		t.Fatalf("expected empty buffer after successful retry, got %d", got)
	}
	if !slices.Equal(checkpoint.advanced, []uint64{2}) {
		t.Fatalf("expected checkpoint advance [2], got %v", checkpoint.advanced)
	}
	if checkpoint.compactCalls != 1 {
		t.Fatalf("expected one compaction call after successful checkpoint retry, got %d", checkpoint.compactCalls)
	}
	if len(store.seenSeq) != 2 {
		t.Fatalf("expected idempotent unique commit set size=2, got %d", len(store.seenSeq))
	}
	if store.calls != 2 {
		t.Fatalf("expected two sqlite insert attempts across retry, got %d", store.calls)
	}
}

func TestFlusher_LastFlushResultRecorded(t *testing.T) {
	buf := &fakeBuffer{queue: makeRecords(1)}
	store := &fakeStore{}
	checkpoint := &fakeCheckpoint{}

	f := New(store, checkpoint, buf, Config{FlushBatchSize: 10, FlushInterval: time.Minute})
	if err := f.FlushNow(); err != nil {
		t.Fatalf("FlushNow failed: %v", err)
	}

	result, ok := f.LastFlushResult()
	if !ok {
		t.Fatal("expected last flush result to be recorded")
	}
	if !result.Success {
		t.Fatalf("expected successful result, got %+v", result)
	}
	if result.AtUTC.IsZero() {
		t.Fatalf("expected non-zero flush timestamp, got %+v", result)
	}
}

func TestFlusher_LastFlushResultRecordsCheckpointFailure(t *testing.T) {
	buf := &fakeBuffer{queue: makeRecords(1, 2)}
	store := &fakeStore{}
	checkpoint := &fakeCheckpoint{advanceErr: errors.New("checkpoint write failed")}

	f := New(store, checkpoint, buf, Config{FlushBatchSize: 10, FlushInterval: time.Minute})
	if err := f.FlushNow(); err == nil {
		t.Fatal("expected checkpoint failure")
	}

	result, ok := f.LastFlushResult()
	if !ok {
		t.Fatal("expected last flush result to be recorded")
	}
	if result.Success {
		t.Fatalf("expected failure result, got %+v", result)
	}
	if result.Error == "" || result.Error != "checkpoint write failed" {
		t.Fatalf("expected checkpoint error in last flush result, got %+v", result)
	}
}

func TestFlusher_CompactionFailureDoesNotInvalidateCommit(t *testing.T) {
	buf := &fakeBuffer{queue: makeRecords(1, 2, 3)}
	store := &fakeStore{}
	checkpoint := &fakeCheckpoint{compactErr: errors.New("compact failed")}

	f := New(store, checkpoint, buf, Config{FlushBatchSize: 10, FlushInterval: time.Minute})
	if err := f.FlushNow(); err != nil {
		t.Fatalf("expected flush success despite compaction failure, got %v", err)
	}

	if got := buf.Stats().TotalBufferedPoints; got != 0 {
		t.Fatalf("expected empty buffer, got %d", got)
	}
	if !slices.Equal(checkpoint.advanced, []uint64{3}) {
		t.Fatalf("expected checkpoint advance [3], got %v", checkpoint.advanced)
	}
	if checkpoint.compactCalls != 1 {
		t.Fatalf("expected one compaction attempt, got %d", checkpoint.compactCalls)
	}
}

func TestFlusher_CheckpointOnlyBatchAdvancesWithoutStoreWrite(t *testing.T) {
	r1 := makeRecords(1)[0]
	r2 := makeRecords(2)[0]
	r1.CheckpointOnly = true
	r2.CheckpointOnly = true

	buf := &fakeBuffer{queue: []ingest.SpoolRecord{r1, r2}}
	store := &fakeStore{}
	checkpoint := &fakeCheckpoint{}

	f := New(store, checkpoint, buf, Config{FlushBatchSize: 10, FlushInterval: time.Minute})
	if err := f.FlushNow(); err != nil {
		t.Fatalf("expected checkpoint-only flush success, got %v", err)
	}

	if store.calls != 0 {
		t.Fatalf("expected no store writes for checkpoint-only batch, got %d", store.calls)
	}
	if !slices.Equal(checkpoint.advanced, []uint64{2}) {
		t.Fatalf("expected checkpoint advance [2], got %v", checkpoint.advanced)
	}
}
