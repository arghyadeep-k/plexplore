package buffer

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"plexplore/internal/ingest"
)

var (
	ErrMaxPointsExceeded = errors.New("buffer max points exceeded")
	ErrMaxBytesExceeded  = errors.New("buffer max bytes exceeded")
)

const (
	DefaultDedupeMaxTimeDelta  = 2 * time.Second
	DefaultDedupeMaxDistanceM  = 10.0
	earthRadiusMeters          = 6371000.0
)

// Stats contains lightweight runtime buffer metrics.
type Stats struct {
	TotalBufferedPoints int
	TotalBufferedBytes  int
	OldestBufferedAge   time.Duration
}

type entry struct {
	record     ingest.SpoolRecord
	sizeBytes  int
	enqueuedAt time.Time
}

type dedupeState struct {
	timestampUTC time.Time
	lat          float64
	lon          float64
}

// Manager is a FIFO in-memory queue for spool records with hard limits.
// It is safe for concurrent callers through internal mutex protection.
type Manager struct {
	mu sync.Mutex

	maxPoints int
	maxBytes  int

	totalPoints int
	totalBytes  int

	queue        []entry
	deviceCounts map[string]int
	lastByDevice map[string]dedupeState
	dedupeTime   time.Duration
	dedupeDistM  float64
	nowFn        func() time.Time
}

func NewManager(maxPoints, maxBytes int) *Manager {
	return NewManagerWithDedupe(maxPoints, maxBytes, DefaultDedupeMaxTimeDelta, DefaultDedupeMaxDistanceM)
}

// NewManagerWithDedupe creates a manager with configurable near-duplicate
// suppression thresholds for the same device.
func NewManagerWithDedupe(maxPoints, maxBytes int, maxTimeDelta time.Duration, maxDistanceMeters float64) *Manager {
	if maxPoints <= 0 {
		maxPoints = 1
	}
	if maxBytes <= 0 {
		maxBytes = 1
	}
	if maxTimeDelta < 0 {
		maxTimeDelta = 0
	}
	if maxDistanceMeters < 0 {
		maxDistanceMeters = 0
	}

	return &Manager{
		maxPoints:    maxPoints,
		maxBytes:     maxBytes,
		queue:        make([]entry, 0),
		deviceCounts: make(map[string]int),
		lastByDevice: make(map[string]dedupeState),
		dedupeTime:   maxTimeDelta,
		dedupeDistM:  maxDistanceMeters,
		nowFn:        time.Now,
	}
}

// Enqueue buffers records in RAM. This call is all-or-nothing: if limits would
// be exceeded, no records from this call are buffered.
func (m *Manager) Enqueue(records []ingest.SpoolRecord) error {
	if len(records) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	filtered, nextDedupeState := m.filterNearDuplicates(records)
	if len(filtered) == 0 {
		return nil
	}

	incomingBytes := 0
	for _, record := range filtered {
		size, err := estimateRecordBytes(record)
		if err != nil {
			return fmt.Errorf("estimate record bytes: %w", err)
		}
		incomingBytes += size
	}

	if m.totalPoints+len(filtered) > m.maxPoints {
		return ErrMaxPointsExceeded
	}
	if m.totalBytes+incomingBytes > m.maxBytes {
		return ErrMaxBytesExceeded
	}

	now := m.nowFn().UTC()
	for _, record := range filtered {
		size, _ := estimateRecordBytes(record)
		m.queue = append(m.queue, entry{
			record:     record,
			sizeBytes:  size,
			enqueuedAt: now,
		})
		m.totalPoints++
		m.totalBytes += size
		m.deviceCounts[record.DeviceID]++
	}
	for deviceID, state := range nextDedupeState {
		m.lastByDevice[deviceID] = state
	}

	return nil
}

// DrainBatch removes up to maxPoints records in FIFO order.
func (m *Manager) DrainBatch(maxPoints int) []ingest.SpoolRecord {
	if maxPoints <= 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.queue) == 0 {
		return nil
	}

	count := maxPoints
	if count > len(m.queue) {
		count = len(m.queue)
	}

	drained := make([]ingest.SpoolRecord, 0, count)
	for i := 0; i < count; i++ {
		item := m.queue[i]
		drained = append(drained, item.record)
		m.totalPoints--
		m.totalBytes -= item.sizeBytes

		deviceID := item.record.DeviceID
		m.deviceCounts[deviceID]--
		if m.deviceCounts[deviceID] <= 0 {
			delete(m.deviceCounts, deviceID)
		}
	}

	m.queue = append([]entry(nil), m.queue[count:]...)
	return drained
}

// RequeueFront prepends records back to the in-memory queue in original order.
// This is intended for transient downstream failures (for example DB failure
// after a drain), so already-drained records are not lost.
func (m *Manager) RequeueFront(records []ingest.SpoolRecord) error {
	if len(records) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	prefix := make([]entry, 0, len(records))
	now := m.nowFn().UTC()
	for _, record := range records {
		size, err := estimateRecordBytes(record)
		if err != nil {
			return fmt.Errorf("estimate record bytes: %w", err)
		}
		prefix = append(prefix, entry{
			record:     record,
			sizeBytes:  size,
			enqueuedAt: now,
		})
		m.totalPoints++
		m.totalBytes += size
		m.deviceCounts[record.DeviceID]++
	}

	m.queue = append(prefix, m.queue...)
	return nil
}

func (m *Manager) Stats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()

	var oldestAge time.Duration
	if len(m.queue) > 0 {
		oldestAge = m.nowFn().UTC().Sub(m.queue[0].enqueuedAt)
		if oldestAge < 0 {
			oldestAge = 0
		}
	}

	return Stats{
		TotalBufferedPoints: m.totalPoints,
		TotalBufferedBytes:  m.totalBytes,
		OldestBufferedAge:   oldestAge,
	}
}

func estimateRecordBytes(record ingest.SpoolRecord) (int, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return 0, err
	}
	return len(data) + 1, nil // +1 approximates newline in NDJSON spool format.
}

func (m *Manager) filterNearDuplicates(records []ingest.SpoolRecord) ([]ingest.SpoolRecord, map[string]dedupeState) {
	if m.dedupeTime == 0 && m.dedupeDistM == 0 {
		return records, map[string]dedupeState{}
	}

	candidateState := make(map[string]dedupeState, len(m.lastByDevice))
	for deviceID, state := range m.lastByDevice {
		candidateState[deviceID] = state
	}

	updatedState := make(map[string]dedupeState)
	out := make([]ingest.SpoolRecord, 0, len(records))
	for _, record := range records {
		if isNearDuplicate(record, candidateState, m.dedupeTime, m.dedupeDistM) {
			continue
		}
		state := dedupeState{
			timestampUTC: record.Point.TimestampUTC.UTC(),
			lat:          record.Point.Lat,
			lon:          record.Point.Lon,
		}
		candidateState[record.DeviceID] = state
		updatedState[record.DeviceID] = state
		out = append(out, record)
	}
	return out, updatedState
}

func isNearDuplicate(record ingest.SpoolRecord, stateByDevice map[string]dedupeState, maxTimeDelta time.Duration, maxDistanceMeters float64) bool {
	prev, ok := stateByDevice[record.DeviceID]
	if !ok {
		return false
	}

	ts := record.Point.TimestampUTC.UTC()
	if ts.IsZero() || prev.timestampUTC.IsZero() {
		return false
	}

	timeDiff := ts.Sub(prev.timestampUTC)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	if timeDiff > maxTimeDelta {
		return false
	}

	distance := haversineMeters(prev.lat, prev.lon, record.Point.Lat, record.Point.Lon)
	return distance <= maxDistanceMeters
}

func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}
