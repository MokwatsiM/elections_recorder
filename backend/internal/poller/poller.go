// Package poller decides which IEC files to fetch and when.
//
// The IEC regenerates its files about every 6 minutes, in order: municipalities,
// then provinces, then about 4 minutes later the national file. So three things
// trigger fetches:
//
//   - the sentinel: one municipality file that signals a new cycle has started,
//     which starts a sweep and picks up results before the national file shows them
//   - the national file: its per-municipality counters reveal anything still missed
//   - a timed sweep: the safety net if both signals are ever misleading
package poller

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"lge-dashboard/backend/internal/iec"
)

type Fetcher interface {
	Fetch(ctx context.Context, path string, v iec.Validators) (iec.Result, error)
}

type Recorder interface {
	Validators(ctx context.Context, path string) (iec.Validators, error)
	Record(ctx context.Context, res iec.Result) (bool, error)
	Latest(ctx context.Context, path string) ([]byte, error)
}

type Config struct {
	Election     iec.Election
	Municipality []iec.Municipality
	Sentinel     int

	Tick          time.Duration // how often the national file and sentinel are checked
	SweepEvery    time.Duration // safety-net sweep
	MinSweepGap   time.Duration // never sweep more often than this
	SweepDelay    time.Duration // wait after the sentinel for the batch to finish
	MaxDirtyTries int           // give up on a municipality until the next sweep
	Concurrency   int

	OnChange func(path string)            // called after a new version is recorded
	OnSchema func(path string, err error) // called when a file has an unexpected structure
	OnCycle  func(kind string)            // called when a sweep runs
}

func (c *Config) defaults() {
	if c.Tick == 0 {
		c.Tick = 30 * time.Second
	}
	if c.SweepEvery == 0 {
		c.SweepEvery = 10 * time.Minute
	}
	if c.MinSweepGap == 0 {
		c.MinSweepGap = 3 * time.Minute
	}
	if c.SweepDelay == 0 {
		c.SweepDelay = 45 * time.Second
	}
	if c.MaxDirtyTries == 0 {
		c.MaxDirtyTries = 5
	}
	if c.Concurrency == 0 {
		c.Concurrency = 4
	}
	if c.OnChange == nil {
		c.OnChange = func(string) {}
	}
	if c.OnSchema == nil {
		c.OnSchema = func(string, error) {}
	}
	if c.OnCycle == nil {
		c.OnCycle = func(string) {}
	}
}

type Poller struct {
	cfg Config
	f   Fetcher
	r   Recorder
	log *slog.Logger

	province map[int]int // municipality → province
	counters map[int]iec.Counters
	dirty    map[int]int // municipality → attempts so far

	lastSweep   time.Time
	sweepAfter  time.Time // a sentinel-triggered sweep is due at this time
	mu          sync.Mutex
	lastSuccess time.Time
}

func New(cfg Config, f Fetcher, r Recorder, log *slog.Logger) *Poller {
	cfg.defaults()
	p := &Poller{
		cfg: cfg, f: f, r: r, log: log,
		province: make(map[int]int, len(cfg.Municipality)),
		dirty:    make(map[int]int),
	}
	for _, m := range cfg.Municipality {
		p.province[m.ID] = m.Province
	}
	return p
}

// LastSuccess reports when the IEC last answered, used for health checks.
func (p *Poller) LastSuccess() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastSuccess
}

// Run polls until ctx ends. It opens with a sweep, so results published while
// the process was down are captured straight away.
func (p *Poller) Run(ctx context.Context) error {
	p.fetchLogged(ctx, p.cfg.Election.NationalPath())
	p.loadCounters(ctx)
	p.sweep(ctx, "startup")

	t := time.NewTicker(p.cfg.Tick)
	defer t.Stop()
	for {
		p.Tick(ctx, time.Now())
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

// Tick runs one polling round.
func (p *Poller) Tick(ctx context.Context, now time.Time) {
	if changed, err := p.fetch(ctx, p.cfg.Election.MunicipalityPath(p.cfg.Sentinel)); err == nil && changed && p.sweepAfter.IsZero() {
		p.sweepAfter = now.Add(p.cfg.SweepDelay)
	}

	if changed, body, err := p.fetchBody(ctx, p.cfg.Election.NationalPath()); err == nil && changed {
		p.applyNational(body)
		p.fetchLogged(ctx, p.cfg.Election.BigNationalPath())
	}

	sinceSweep := now.Sub(p.lastSweep)
	switch {
	case sinceSweep >= p.cfg.SweepEvery:
		p.sweep(ctx, "timer")
	case !p.sweepAfter.IsZero() && !now.Before(p.sweepAfter) && sinceSweep >= p.cfg.MinSweepGap:
		p.sweep(ctx, "sentinel")
	default:
		p.fetchDirty(ctx)
	}
}

func (p *Poller) loadCounters(ctx context.Context) {
	body, err := p.r.Latest(ctx, p.cfg.Election.NationalPath())
	if err != nil {
		return // nothing recorded yet
	}
	if n, err := iec.ParseNational(body); err == nil {
		p.counters = n.Counters()
	}
}

func (p *Poller) applyNational(body []byte) {
	n, err := iec.ParseNational(body)
	if err != nil {
		p.log.Error("national file has an unexpected structure", "err", err)
		p.cfg.OnSchema(p.cfg.Election.NationalPath(), err)
		return
	}
	next := n.Counters()
	if p.counters == nil { // first sighting is a baseline; the startup sweep covered it
		p.counters = next
		return
	}
	for id, c := range next {
		if old, ok := p.counters[id]; !ok || old != c {
			if _, already := p.dirty[id]; !already {
				p.dirty[id] = 0
			}
		}
	}
	p.counters = next
}

// fetchDirty fetches municipalities the national file says have new results,
// along with their provinces. The file may not have refreshed behind Cloudflare
// yet, so an unchanged response stays dirty and is retried on later ticks.
func (p *Poller) fetchDirty(ctx context.Context) {
	if len(p.dirty) == 0 {
		return
	}
	ids := make([]int, 0, len(p.dirty))
	provinces := map[int]struct{}{}
	for id := range p.dirty {
		ids = append(ids, id)
		if pv, ok := p.province[id]; ok {
			provinces[pv] = struct{}{}
		}
	}

	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(p.cfg.Concurrency)
	for _, id := range ids {
		g.Go(func() error {
			changed, err := p.fetch(gctx, p.cfg.Election.MunicipalityPath(id))
			mu.Lock()
			defer mu.Unlock()
			p.dirty[id]++
			if (err == nil && changed) || p.dirty[id] >= p.cfg.MaxDirtyTries {
				delete(p.dirty, id)
			}
			return nil
		})
	}
	for pv := range provinces {
		g.Go(func() error {
			p.fetchLogged(gctx, p.cfg.Election.ProvincePath(pv))
			return nil
		})
	}
	g.Wait()
}

func (p *Poller) sweep(ctx context.Context, kind string) {
	start := time.Now()
	p.lastSweep = start
	p.sweepAfter = time.Time{}
	clear(p.dirty)
	p.cfg.OnCycle(kind)

	paths := make([]string, 0, len(p.cfg.Municipality)+9)
	for pv := 1; pv <= 9; pv++ {
		paths = append(paths, p.cfg.Election.ProvincePath(pv))
	}
	for _, m := range p.cfg.Municipality {
		paths = append(paths, p.cfg.Election.MunicipalityPath(m.ID))
	}

	var mu sync.Mutex
	changed := 0
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(p.cfg.Concurrency)
	for _, path := range paths {
		g.Go(func() error {
			if c, err := p.fetch(gctx, path); err == nil && c {
				mu.Lock()
				changed++
				mu.Unlock()
			}
			return nil
		})
	}
	g.Wait()
	p.log.Info("sweep finished", "trigger", kind, "files", len(paths), "changed", changed, "took", time.Since(start).Round(time.Second))
}

func (p *Poller) fetchLogged(ctx context.Context, path string) {
	if _, err := p.fetch(ctx, path); err != nil && !errors.Is(err, context.Canceled) {
		p.log.Warn("fetch failed", "path", path, "err", err)
	}
}

func (p *Poller) fetch(ctx context.Context, path string) (bool, error) {
	changed, _, err := p.fetchBody(ctx, path)
	return changed, err
}

func (p *Poller) fetchBody(ctx context.Context, path string) (bool, []byte, error) {
	v, err := p.r.Validators(ctx, path)
	if err != nil {
		p.log.Error("reading validators", "path", path, "err", err)
		return false, nil, err
	}
	res, err := p.f.Fetch(ctx, path, v)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			p.log.Warn("fetch failed", "path", path, "err", err)
		}
		return false, nil, err
	}
	p.mu.Lock()
	p.lastSuccess = time.Now()
	p.mu.Unlock()

	changed, err := p.r.Record(ctx, res)
	if err != nil {
		// Validators weren't saved, so the next round fetches this file again.
		p.log.Error("recording failed", "path", path, "err", err)
		return false, nil, err
	}
	if changed {
		p.cfg.OnChange(path)
	}
	return changed, res.Body, nil
}
