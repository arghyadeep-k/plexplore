package tasks

import (
	"context"
	"sync"
	"testing"
	"time"

	"plexplore/internal/store"
	"plexplore/internal/visits"
)

type schedulerRebuildCall struct {
	deviceID string
	fromUTC  *time.Time
	toUTC    *time.Time
	cfg      visits.Config
}

type fakeVisitSchedulerStore struct {
	mu sync.Mutex

	devices []store.Device

	maxSeqByDevice  map[string]uint64
	tsByDeviceSeq   map[string]map[uint64]time.Time
	stateByDevice   map[string]store.VisitGenerationState
	createdByDevice map[string]int

	rebuildCalls []schedulerRebuildCall

	rebuildStarted chan struct{}
	blockRebuild   chan struct{}
}

func newFakeVisitSchedulerStore() *fakeVisitSchedulerStore {
	return &fakeVisitSchedulerStore{
		maxSeqByDevice:  make(map[string]uint64),
		tsByDeviceSeq:   make(map[string]map[uint64]time.Time),
		stateByDevice:   make(map[string]store.VisitGenerationState),
		createdByDevice: make(map[string]int),
	}
}

func (f *fakeVisitSchedulerStore) ListDevices(_ context.Context) ([]store.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.Device, len(f.devices))
	copy(out, f.devices)
	return out, nil
}

func (f *fakeVisitSchedulerStore) RebuildVisitsForDeviceRange(_ context.Context, deviceID string, fromUTC, toUTC *time.Time, cfg visits.Config) (int, error) {
	f.mu.Lock()
	f.rebuildCalls = append(f.rebuildCalls, schedulerRebuildCall{
		deviceID: deviceID,
		fromUTC:  fromUTC,
		toUTC:    toUTC,
		cfg:      cfg,
	})
	started := f.rebuildStarted
	block := f.blockRebuild
	created := f.createdByDevice[deviceID]
	f.mu.Unlock()

	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if block != nil {
		<-block
	}
	return created, nil
}

func (f *fakeVisitSchedulerStore) GetVisitGenerationState(_ context.Context, deviceName string) (store.VisitGenerationState, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	state, ok := f.stateByDevice[deviceName]
	return state, ok, nil
}

func (f *fakeVisitSchedulerStore) UpsertVisitGenerationState(_ context.Context, deviceName string, lastProcessedSeq uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stateByDevice[deviceName] = store.VisitGenerationState{
		DeviceName:       deviceName,
		LastProcessedSeq: lastProcessedSeq,
		UpdatedAt:        time.Now().UTC(),
	}
	return nil
}

func (f *fakeVisitSchedulerStore) GetMaxPointSeqForDevice(_ context.Context, deviceName string) (uint64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	seq, ok := f.maxSeqByDevice[deviceName]
	if !ok || seq == 0 {
		return 0, false, nil
	}
	return seq, true, nil
}

func (f *fakeVisitSchedulerStore) GetPointTimestampForDeviceSeq(_ context.Context, deviceName string, seq uint64) (time.Time, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	bySeq, ok := f.tsByDeviceSeq[deviceName]
	if !ok {
		return time.Time{}, false, nil
	}
	ts, ok := bySeq[seq]
	if !ok {
		return time.Time{}, false, nil
	}
	return ts, true, nil
}

func (f *fakeVisitSchedulerStore) rebuildCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.rebuildCalls)
}

func (f *fakeVisitSchedulerStore) lastRebuildCall() (schedulerRebuildCall, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.rebuildCalls) == 0 {
		return schedulerRebuildCall{}, false
	}
	return f.rebuildCalls[len(f.rebuildCalls)-1], true
}

func TestVisitSchedulerRunOnce_IncrementalProgress(t *testing.T) {
	st := newFakeVisitSchedulerStore()
	st.devices = []store.Device{{ID: 1, Name: "phone-main"}}
	st.maxSeqByDevice["phone-main"] = 10
	st.tsByDeviceSeq["phone-main"] = map[uint64]time.Time{
		10: time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC),
	}
	st.createdByDevice["phone-main"] = 2

	s := NewVisitScheduler(st, VisitSchedulerConfig{
		Enabled:         true,
		Interval:        time.Minute,
		DeviceBatchSize: 10,
		Lookback:        time.Hour,
		DetectConfig: visits.Config{
			MinDwell:        20 * time.Minute,
			MaxRadiusMeters: 40,
		},
	})

	result1, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("first RunOnce failed: %v", err)
	}
	if result1.ProcessedDevices != 1 || result1.UpdatedDevices != 1 || result1.CreatedVisits != 2 {
		t.Fatalf("unexpected first run result: %+v", result1)
	}
	if got := st.rebuildCallCount(); got != 1 {
		t.Fatalf("expected one rebuild call after first run, got %d", got)
	}
	state, ok, err := st.GetVisitGenerationState(context.Background(), "phone-main")
	if err != nil {
		t.Fatalf("GetVisitGenerationState failed: %v", err)
	}
	if !ok || state.LastProcessedSeq != 10 {
		t.Fatalf("expected watermark seq=10 after first run, got ok=%t state=%+v", ok, state)
	}

	result2, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("second RunOnce failed: %v", err)
	}
	if result2.ProcessedDevices != 1 || result2.SkippedDevices != 1 || result2.UpdatedDevices != 0 || result2.CreatedVisits != 0 {
		t.Fatalf("unexpected second run result: %+v", result2)
	}
	if got := st.rebuildCallCount(); got != 1 {
		t.Fatalf("expected no additional rebuild call when no new points, got %d calls", got)
	}

	st.maxSeqByDevice["phone-main"] = 12
	st.tsByDeviceSeq["phone-main"][10] = time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	st.createdByDevice["phone-main"] = 1

	result3, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("third RunOnce failed: %v", err)
	}
	if result3.UpdatedDevices != 1 || result3.CreatedVisits != 1 {
		t.Fatalf("unexpected third run result: %+v", result3)
	}
	if got := st.rebuildCallCount(); got != 2 {
		t.Fatalf("expected second rebuild call after new points, got %d calls", got)
	}
	lastCall, ok := st.lastRebuildCall()
	if !ok {
		t.Fatalf("expected last rebuild call")
	}
	if lastCall.fromUTC == nil {
		t.Fatalf("expected incremental run to pass non-nil fromUTC lookback")
	}
	expectedFrom := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC).Add(-time.Hour)
	if !lastCall.fromUTC.Equal(expectedFrom) {
		t.Fatalf("expected fromUTC=%s, got %s", expectedFrom.Format(time.RFC3339), lastCall.fromUTC.Format(time.RFC3339))
	}
}

func TestVisitSchedulerRunOnce_NoOverlapConcurrentRuns(t *testing.T) {
	st := newFakeVisitSchedulerStore()
	st.devices = []store.Device{{ID: 1, Name: "phone-main"}}
	st.maxSeqByDevice["phone-main"] = 3
	st.createdByDevice["phone-main"] = 1
	st.rebuildStarted = make(chan struct{}, 1)
	st.blockRebuild = make(chan struct{})

	s := NewVisitScheduler(st, VisitSchedulerConfig{Enabled: true, Interval: time.Minute})

	errCh := make(chan error, 1)
	go func() {
		_, err := s.RunOnce(context.Background())
		errCh <- err
	}()

	select {
	case <-st.rebuildStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for first rebuild to start")
	}

	result2, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("second RunOnce failed: %v", err)
	}
	if result2.ProcessedDevices != 0 || result2.UpdatedDevices != 0 || result2.CreatedVisits != 0 {
		t.Fatalf("expected overlapping run to be skipped, got %+v", result2)
	}

	close(st.blockRebuild)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("first RunOnce failed: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for first RunOnce to finish")
	}

	if got := st.rebuildCallCount(); got != 1 {
		t.Fatalf("expected exactly one rebuild call with overlap prevention, got %d", got)
	}
}

func TestVisitSchedulerStart_TriggersBackgroundRun(t *testing.T) {
	st := newFakeVisitSchedulerStore()
	st.devices = []store.Device{{ID: 1, Name: "phone-main"}}
	st.maxSeqByDevice["phone-main"] = 1
	st.createdByDevice["phone-main"] = 1

	s := NewVisitScheduler(st, VisitSchedulerConfig{
		Enabled:  true,
		Interval: 25 * time.Millisecond,
	})

	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(parent)
	defer s.Stop(context.Background())

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if st.rebuildCallCount() >= 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected background scheduler to trigger at least one rebuild call, got %d", st.rebuildCallCount())
}
