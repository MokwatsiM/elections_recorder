package results

import (
	"context"
	"slices"
	"sync"
)

// MemorySink keeps output in memory, so replay mode writes nothing to Postgres
// and can't disturb a live recorder sharing the same database.
type MemorySink struct {
	mu        sync.RWMutex
	events    []Event
	snapshots map[Level]map[int][]TimelinePoint
}

func NewMemorySink() *MemorySink {
	return &MemorySink{snapshots: map[Level]map[int][]TimelinePoint{}}
}

func (m *MemorySink) LoadCursor(context.Context) (int64, int64, error) { return 0, 0, nil }

func (m *MemorySink) Commit(_ context.Context, c Commit) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range c.Events {
		if e.Persisted() {
			m.events = append(m.events, e)
		}
	}
	for _, s := range c.Snapshots {
		if m.snapshots[s.Level] == nil {
			m.snapshots[s.Level] = map[int][]TimelinePoint{}
		}
		m.snapshots[s.Level][s.ID] = append(m.snapshots[s.Level][s.ID], TimelinePoint{At: s.At, Progress: s.Area.Progress, Parties: s.Area.Parties})
	}
	return nil
}

func (m *MemorySink) Feed(_ context.Context, before int64, limit int) ([]Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Event, 0, limit)
	for _, e := range slices.Backward(m.events) {
		if before > 0 && e.Seq >= before {
			continue
		}
		out = append(out, e)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (m *MemorySink) Timeline(_ context.Context, level Level, id, limit int) ([]TimelinePoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pts := m.snapshots[level][id]
	if len(pts) > limit {
		pts = pts[len(pts)-limit:]
	}
	return slices.Clone(pts), nil
}
