package spool

// SegmentCount returns the number of segment files currently present.
func (m *FileSpoolManager) SegmentCount() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	starts, err := m.listSegmentStartsLocked()
	if err != nil {
		return 0, err
	}
	return len(starts), nil
}
