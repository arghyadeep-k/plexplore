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
	RebuildVisitsForDeviceRange(rctx context.Context, deviceID string, fromUTC, toUTC *time.Time, cfg visits.Config) (int, error)
	GetVisitGenerationState(rctx context.Context, deviceName string) (store.VisitGenerationState, bool, error)
	UpsertVisitGenerationState(rctx context.Context, deviceName string, lastProcessedSeq uint64) error
	GetMaxPointSeqForDevice(rctx context.Context, deviceName string) (uint64, bool, error)
	GetPointTimestampForDeviceSeq(rctx context.Context, deviceName string, seq uint64) (time.Time, bool, error)
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
	deviceNames := uniqueSortedDeviceNames(devices)
	if len(deviceNames) == 0 {
		return VisitSchedulerRunResult{}, nil
	}
	selected := s.selectDeviceBatch(deviceNames)

	var result VisitSchedulerRunResult
	for _, deviceName := range selected {
		result.ProcessedDevices++
		created, updated, skipped, procErr := s.processDevice(ctx, deviceName)
		if procErr != nil {
			result.Errors++
			log.Printf("visit scheduler device=%s error: %v", deviceName, procErr)
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

func (s *VisitScheduler) processDevice(ctx context.Context, deviceName string) (created int, updated bool, skipped bool, err error) {
	maxSeq, hasPoints, err := s.store.GetMaxPointSeqForDevice(ctx, deviceName)
	if err != nil {
		return 0, false, false, err
	}
	if !hasPoints {
		return 0, false, true, nil
	}

	state, ok, err := s.store.GetVisitGenerationState(ctx, deviceName)
	if err != nil {
		return 0, false, false, err
	}
	if ok && maxSeq <= state.LastProcessedSeq {
		return 0, false, true, nil
	}

	var fromUTC *time.Time
	if ok && state.LastProcessedSeq > 0 {
		ts, found, tsErr := s.store.GetPointTimestampForDeviceSeq(ctx, deviceName, state.LastProcessedSeq)
		if tsErr != nil {
			return 0, false, false, tsErr
		}
		if found {
			start := ts.UTC().Add(-s.cfg.Lookback)
			fromUTC = &start
		}
	}

	created, err = s.store.RebuildVisitsForDeviceRange(ctx, deviceName, fromUTC, nil, s.cfg.DetectConfig)
	if err != nil {
		return 0, false, false, err
	}
	if err := s.store.UpsertVisitGenerationState(ctx, deviceName, maxSeq); err != nil {
		return 0, false, false, err
	}
	return created, true, false, nil
}

func uniqueSortedDeviceNames(devices []store.Device) []string {
	seen := make(map[string]struct{}, len(devices))
	out := make([]string, 0, len(devices))
	for _, d := range devices {
		name := strings.TrimSpace(d.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (s *VisitScheduler) selectDeviceBatch(deviceNames []string) []string {
	if len(deviceNames) <= s.cfg.DeviceBatchSize {
		return deviceNames
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	start := s.roundRobinStart % len(deviceNames)
	batch := make([]string, 0, s.cfg.DeviceBatchSize)
	for i := 0; i < s.cfg.DeviceBatchSize; i++ {
		idx := (start + i) % len(deviceNames)
		batch = append(batch, deviceNames[idx])
	}
	s.roundRobinStart = (start + s.cfg.DeviceBatchSize) % len(deviceNames)
	return batch
}
