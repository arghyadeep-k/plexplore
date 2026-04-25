package tasks

import (
	"context"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"plexplore/internal/store"
	"plexplore/internal/visits"
)

type VisitSchedulerStore interface {
	ListDevices(rctx context.Context) ([]store.Device, error)
	RebuildVisitsForDeviceRange(rctx context.Context, deviceID int64, fromUTC, toUTC *time.Time, cfg visits.Config) (int, error)
	GetVisitGenerationState(rctx context.Context, deviceID int64) (store.VisitGenerationState, bool, error)
	UpsertVisitGenerationState(rctx context.Context, deviceID int64, lastProcessedSeq uint64) error
	GetMaxPointSeqForDevice(rctx context.Context, deviceID int64) (uint64, bool, error)
	GetPointTimestampForDeviceSeq(rctx context.Context, deviceID int64, seq uint64) (time.Time, bool, error)
}

type VisitSchedulerConfig struct {
	Enabled         bool
	Interval        time.Duration
	DeviceBatchSize int
	Lookback        time.Duration
	DetectConfig    visits.Config
}

type VisitScheduler struct {
	store VisitSchedulerStore
	cfg   VisitSchedulerConfig

	started atomic.Bool
	running atomic.Bool

	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu              sync.Mutex
	roundRobinStart int
}

type VisitSchedulerRunResult struct {
	ProcessedDevices int
	SkippedDevices   int
	UpdatedDevices   int
	CreatedVisits    int
	Errors           int
}

func NewVisitScheduler(store VisitSchedulerStore, cfg VisitSchedulerConfig) *VisitScheduler {
	if cfg.Interval <= 0 {
		cfg.Interval = 15 * time.Minute
	}
	if cfg.DeviceBatchSize <= 0 {
		cfg.DeviceBatchSize = 10
	}
	if cfg.Lookback <= 0 {
		cfg.Lookback = 2 * time.Hour
	}
	if cfg.DetectConfig.MinDwell <= 0 {
		cfg.DetectConfig.MinDwell = 15 * time.Minute
	}
	if cfg.DetectConfig.MaxRadiusMeters <= 0 {
		cfg.DetectConfig.MaxRadiusMeters = 35
	}
	return &VisitScheduler{
		store: store,
		cfg:   cfg,
	}
}

func (s *VisitScheduler) Start(parent context.Context) {
	if s == nil || s.store == nil || !s.cfg.Enabled {
		return
	}
	if !s.started.CompareAndSwap(false, true) {
		return
	}

	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.safeRunOnce(ctx)
		ticker := time.NewTicker(s.cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.safeRunOnce(ctx)
			}
		}
	}()
}

func (s *VisitScheduler) Stop(ctx context.Context) {
	if s == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.wg.Wait()
	}()
	select {
	case <-ctx.Done():
	case <-done:
	}
}

func (s *VisitScheduler) RunOnce(ctx context.Context) (VisitSchedulerRunResult, error) {
	return s.runOnce(ctx)
}

func (s *VisitScheduler) safeRunOnce(ctx context.Context) {
	result, err := s.runOnce(ctx)
	if err != nil {
		log.Printf("visit scheduler run failed: %v", err)
		return
	}
	log.Printf(
		"visit scheduler run complete: processed=%d skipped=%d updated=%d created=%d errors=%d",
		result.ProcessedDevices,
		result.SkippedDevices,
		result.UpdatedDevices,
		result.CreatedVisits,
		result.Errors,
	)
}

func (s *VisitScheduler) runOnce(ctx context.Context) (VisitSchedulerRunResult, error) {
	if s == nil || s.store == nil {
		return VisitSchedulerRunResult{}, nil
	}
	if !s.running.CompareAndSwap(false, true) {
		return VisitSchedulerRunResult{}, nil
	}
	defer s.running.Store(false)

	devices, err := s.store.ListDevices(ctx)
	if err != nil {
		return VisitSchedulerRunResult{}, err
	}
	deviceBatch := s.selectDeviceBatch(sortedUniqueDevicesByID(devices))
	if len(deviceBatch) == 0 {
		return VisitSchedulerRunResult{}, nil
	}

	var result VisitSchedulerRunResult
	for _, device := range deviceBatch {
		result.ProcessedDevices++
		created, updated, skipped, procErr := s.processDevice(ctx, device)
		if procErr != nil {
			result.Errors++
			log.Printf("visit scheduler device_id=%d name=%q error: %v", device.ID, device.Name, procErr)
			continue
		}
		if skipped {
			result.SkippedDevices++
			continue
		}
		if updated {
			result.UpdatedDevices++
		}
		result.CreatedVisits += created
	}
	return result, nil
}

func (s *VisitScheduler) processDevice(ctx context.Context, device store.Device) (created int, updated bool, skipped bool, err error) {
	if device.ID <= 0 {
		return 0, false, true, nil
	}

	maxSeq, hasPoints, err := s.store.GetMaxPointSeqForDevice(ctx, device.ID)
	if err != nil {
		return 0, false, false, err
	}
	if !hasPoints {
		return 0, false, true, nil
	}

	state, ok, err := s.store.GetVisitGenerationState(ctx, device.ID)
	if err != nil {
		return 0, false, false, err
	}
	if ok && maxSeq <= state.LastProcessedSeq {
		return 0, false, true, nil
	}

	var fromUTC *time.Time
	if ok && state.LastProcessedSeq > 0 {
		ts, found, tsErr := s.store.GetPointTimestampForDeviceSeq(ctx, device.ID, state.LastProcessedSeq)
		if tsErr != nil {
			return 0, false, false, tsErr
		}
		if found {
			start := ts.UTC().Add(-s.cfg.Lookback)
			fromUTC = &start
		}
	}

	created, err = s.store.RebuildVisitsForDeviceRange(ctx, device.ID, fromUTC, nil, s.cfg.DetectConfig)
	if err != nil {
		return 0, false, false, err
	}
	if err := s.store.UpsertVisitGenerationState(ctx, device.ID, maxSeq); err != nil {
		return 0, false, false, err
	}
	return created, true, false, nil
}

func sortedUniqueDevicesByID(devices []store.Device) []store.Device {
	seen := make(map[int64]struct{}, len(devices))
	out := make([]store.Device, 0, len(devices))
	for _, d := range devices {
		if d.ID <= 0 {
			continue
		}
		if _, ok := seen[d.ID]; ok {
			continue
		}
		seen[d.ID] = struct{}{}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID == out[j].ID {
			return strings.TrimSpace(out[i].Name) < strings.TrimSpace(out[j].Name)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *VisitScheduler) selectDeviceBatch(devices []store.Device) []store.Device {
	if len(devices) <= s.cfg.DeviceBatchSize {
		return devices
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	start := s.roundRobinStart % len(devices)
	batch := make([]store.Device, 0, s.cfg.DeviceBatchSize)
	for i := 0; i < s.cfg.DeviceBatchSize; i++ {
		idx := (start + i) % len(devices)
		batch = append(batch, devices[idx])
	}
	s.roundRobinStart = (start + s.cfg.DeviceBatchSize) % len(devices)
	return batch
}
