package results

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"lge-dashboard/backend/internal/iec"
)

// Version is one recorded change to a file.
type Version struct {
	ID        int64
	Path      string
	Body      []byte
	FetchedAt time.Time
}

// Source supplies recorded versions.
type Source interface {
	Hydration(ctx context.Context, cursor int64) ([]Version, error)
	After(ctx context.Context, cursor int64, limit int) ([]Version, error)
}

// Snapshot is one point on an area's timeline.
type Snapshot struct {
	Level Level
	ID    int
	At    time.Time
	Area  *Area
}

// Commit is written atomically, so events are never published without being saved.
type Commit struct {
	Cursor    int64
	Seq       int64
	Events    []Event
	Snapshots []Snapshot
}

// Sink stores what the engine produces.
type Sink interface {
	LoadCursor(ctx context.Context) (cursor, seq int64, err error)
	Commit(ctx context.Context, c Commit) error
	Feed(ctx context.Context, before int64, limit int) ([]Event, error)
	Timeline(ctx context.Context, level Level, id int, limit int) ([]TimelinePoint, error)
}

type TimelinePoint struct {
	At       time.Time `json:"at"`
	Progress Progress  `json:"progress"`
	Parties  []Party   `json:"parties"`
}

type Publisher interface {
	Publish(seq int64, eventType string, at time.Time, topics []string, data any)
}

type Engine struct {
	src  Source
	sink Sink
	pub  Publisher
	log  *slog.Logger

	// Pace, when set, is called with each version's gap in replay mode.
	Pace func(ctx context.Context, gap time.Duration) error
	// OnSchema is called when a file has an unexpected structure.
	OnSchema func(path string, err error)

	notify chan struct{}
	cursor int64
	seq    atomic.Int64
	ready  atomic.Bool
	lastAt time.Time

	mu        sync.RWMutex
	national  *Area
	provinces map[int]*Area
	munis     map[int]*Municipality
	wardIndex map[int]*Ward
	vdIndex   map[int]*VD
	seenPaths map[string]time.Time

	refs atomic.Pointer[Refs]
}

func NewEngine(src Source, sink Sink, pub Publisher, log *slog.Logger) *Engine {
	e := &Engine{
		src: src, sink: sink, pub: pub, log: log,
		notify:    make(chan struct{}, 1),
		provinces: make(map[int]*Area),
		munis:     make(map[int]*Municipality),
		wardIndex: make(map[int]*Ward),
		vdIndex:   make(map[int]*VD),
		seenPaths: make(map[string]time.Time),
		OnSchema:  func(string, error) {},
	}
	e.refs.Store(&Refs{})
	return e
}

// Notify wakes the engine after a new version is recorded.
func (e *Engine) Notify() {
	select {
	case e.notify <- struct{}{}:
	default:
	}
}

func (e *Engine) Ready() bool { return e.ready.Load() }
func (e *Engine) Seq() int64  { return e.seq.Load() }

// SetRefs replaces the reference data (party names, stations, history).
func (e *Engine) SetRefs(r *Refs) { e.refs.Store(r) }

// Run hydrates from the last processed versions, then applies new ones in order.
func (e *Engine) Run(ctx context.Context) error {
	cursor, seq, err := e.sink.LoadCursor(ctx)
	if err != nil {
		return fmt.Errorf("load cursor: %w", err)
	}
	e.cursor = cursor
	e.seq.Store(seq)

	if cursor > 0 {
		versions, err := e.src.Hydration(ctx, cursor)
		if err != nil {
			return fmt.Errorf("hydrate: %w", err)
		}
		for _, v := range versions {
			e.apply(v) // events from these were already produced in an earlier run
		}
		e.log.Info("hydrated", "files", len(versions), "cursor", cursor, "seq", seq)
	}
	e.ready.Store(true)

	for {
		n, err := e.drain(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			e.log.Error("processing versions", "err", err)
		}
		if n > 0 && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-e.notify:
		case <-time.After(5 * time.Second):
		}
	}
}

func (e *Engine) drain(ctx context.Context) (int, error) {
	versions, err := e.src.After(ctx, e.cursor, 25)
	if err != nil {
		return 0, err
	}
	for _, v := range versions {
		if e.Pace != nil && !e.lastAt.IsZero() {
			if err := e.Pace(ctx, v.FetchedAt.Sub(e.lastAt)); err != nil {
				return 0, err
			}
		}
		e.lastAt = v.FetchedAt

		events, snaps := e.apply(v)
		for i := range events {
			events[i].Seq = e.seq.Add(1)
		}
		commit := Commit{Cursor: v.ID, Seq: e.seq.Load(), Events: events, Snapshots: snaps}
		if err := e.commitWithRetry(ctx, commit); err != nil {
			return 0, err
		}
		e.cursor = v.ID
		for _, ev := range events {
			e.pub.Publish(ev.Seq, ev.Type, ev.At, ev.Topics, ev.Data)
		}
	}
	return len(versions), nil
}

// commitWithRetry keeps trying because state is already updated in memory.
// Applying the version a second time would compare it with itself and lose its
// events, so the only safe way forward is to get this commit saved.
func (e *Engine) commitWithRetry(ctx context.Context, c Commit) error {
	for wait := time.Second; ; wait = min(wait*2, time.Minute) {
		err := e.sink.Commit(ctx, c)
		if err == nil {
			return nil
		}
		e.log.Error("commit failed, retrying", "cursor", c.Cursor, "err", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

// apply updates state from one file and returns what changed. Bad or out-of-order
// files are skipped, so the current figures stay on screen.
func (e *Engine) apply(v Version) ([]Event, []Snapshot) {
	level, id, ok := classify(v.Path)
	if !ok {
		return nil, nil
	}
	if rd, ok := iec.ReportDate(v.Body); ok {
		e.mu.RLock()
		last := e.seenPaths[v.Path]
		e.mu.RUnlock()
		if rd.Before(last) {
			e.log.Info("skipping older file", "path", v.Path, "reportDate", rd, "have", last)
			return nil, nil
		}
		e.mu.Lock()
		e.seenPaths[v.Path] = rd
		e.mu.Unlock()
	}

	var events []Event
	var area *Area
	switch level {
	case LevelNational:
		a, err := parseNational(v.Body)
		if err != nil {
			e.schemaError(v.Path, err)
			return nil, nil
		}
		e.mu.Lock()
		events = diffNational(e.national, a, v.FetchedAt)
		e.national = a
		e.mu.Unlock()
		area = a
	case LevelProvince:
		a, err := parseProvince(id, v.Body)
		if err != nil {
			e.schemaError(v.Path, err)
			return nil, nil
		}
		e.mu.Lock()
		e.provinces[id] = a
		e.mu.Unlock()
		area = a
	case LevelMunicipality:
		m, err := parseMunicipality(id, v.Body)
		if err != nil {
			e.schemaError(v.Path, err)
			return nil, nil
		}
		e.mu.Lock()
		events = diffMunicipality(e.munis[id], m, v.FetchedAt)
		e.indexMunicipality(m)
		e.mu.Unlock()
		area = &m.Area
	}

	events = append(events, Event{Type: EventAreaUpdated, At: v.FetchedAt, Topics: topicsFor(area), Data: summarize(area, 6)})
	return events, []Snapshot{{Level: area.Level, ID: area.ID, At: v.FetchedAt, Area: summarize(area, 12)}}
}

func (e *Engine) indexMunicipality(m *Municipality) {
	e.munis[m.ID] = m
	for i := range m.Wards {
		w := &m.Wards[i]
		e.wardIndex[w.ID] = w
		for j := range w.VDs {
			e.vdIndex[w.VDs[j].ID] = &w.VDs[j]
		}
	}
}

func (e *Engine) schemaError(path string, err error) {
	e.log.Error("file has an unexpected structure, keeping previous data", "path", path, "err", err)
	e.OnSchema(path, err)
}

func sortBySeats(ps []Party) {
	slices.SortStableFunc(ps, func(a, b Party) int { return b.Seats - a.Seats })
}
