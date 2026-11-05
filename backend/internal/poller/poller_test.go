package poller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"lge-dashboard/backend/internal/iec"
)

// fakeIEC serves bodies from a map and counts requests per path.
type fakeIEC struct {
	mu    sync.Mutex
	files map[string]string
	hits  map[string]int
}

func (f *fakeIEC) Fetch(_ context.Context, path string, v iec.Validators) (iec.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hits[path]++
	body, ok := f.files[path]
	if !ok {
		return iec.Result{Path: path, Status: 404, NotFound: true}, nil
	}
	etag := fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
	if v.ETag == etag {
		return iec.Result{Path: path, Status: 304, NotModified: true, Validators: v}, nil
	}
	return iec.Result{Path: path, Status: 200, Body: []byte(body), Validators: iec.Validators{ETag: etag}}, nil
}

func (f *fakeIEC) set(path, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[path] = body
}

func (f *fakeIEC) resetHits() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hits = map[string]int{}
}

type fakeRec struct {
	mu       sync.Mutex
	v        map[string]iec.Validators
	body     map[string]string
	versions map[string]int
}

func (r *fakeRec) Validators(_ context.Context, p string) (iec.Validators, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.v[p], nil
}

func (r *fakeRec) Record(_ context.Context, res iec.Result) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if res.NotModified || res.NotFound {
		return false, nil
	}
	r.v[res.Path] = res.Validators
	if r.body[res.Path] == string(res.Body) {
		return false, nil
	}
	r.body[res.Path] = string(res.Body)
	r.versions[res.Path]++
	return true, nil
}

func (r *fakeRec) Latest(_ context.Context, p string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.body[p]
	if !ok {
		return nil, fmt.Errorf("none")
	}
	return []byte(b), nil
}

var e = iec.Election2026

func nationalBody(counts map[int]int) string {
	var ms []string
	for id, vds := range counts {
		ms = append(ms, fmt.Sprintf(`{"MunicipalityID":%d,"VDsComplete":%d,"WardsComplete":0,"SeatsComplete":0}`, id, vds))
	}
	provinces := []string{fmt.Sprintf(`{"ProvinceID":1,"MunicipalityResults":[%s]}`, strings.Join(ms, ","))}
	for i := 2; i <= 9; i++ {
		provinces = append(provinces, fmt.Sprintf(`{"ProvinceID":%d,"MunicipalityResults":[]}`, i))
	}
	return fmt.Sprintf(`{"ReportDate":"2026-11-05T09:59:21+02:00","ProvinceResults":[%s]}`, strings.Join(provinces, ","))
}

func setup(t *testing.T) (*Poller, *fakeIEC, *fakeRec, time.Time) {
	t.Helper()
	f := &fakeIEC{files: map[string]string{}, hits: map[string]int{}}
	r := &fakeRec{v: map[string]iec.Validators{}, body: map[string]string{}, versions: map[string]int{}}
	munis := []iec.Municipality{{ID: 1001, Province: 1}, {ID: 1002, Province: 1}, {ID: 1003, Province: 1}}
	for _, m := range munis {
		f.set(e.MunicipalityPath(m.ID), fmt.Sprintf(`{"id":%d,"v":0}`, m.ID))
	}
	f.set(e.ProvincePath(1), `{"v":0}`)
	f.set(e.NationalPath(), nationalBody(map[int]int{1001: 1, 1002: 1, 1003: 1}))
	f.set(e.BigNationalPath(), `{"v":0}`)
	p := New(Config{Election: e, Municipality: munis, Sentinel: 1001}, f, r, slog.New(slog.NewTextHandler(io.Discard, nil)))
	now := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)
	p.sweep(context.Background(), "startup")
	p.lastSweep = now
	p.Tick(context.Background(), now) // takes in the first national file
	p.fetchDirty(context.Background())
	f.resetHits()
	return p, f, r, now
}

func TestNationalCounterChangeFetchesOnlyThatMunicipality(t *testing.T) {
	p, f, _, now := setup(t)
	ctx := context.Background()

	f.set(e.MunicipalityPath(1002), `{"id":1002,"v":1}`)
	f.set(e.NationalPath(), nationalBody(map[int]int{1001: 1, 1002: 9, 1003: 1}))
	p.Tick(ctx, now.Add(30*time.Second))

	if f.hits[e.MunicipalityPath(1002)] != 1 {
		t.Errorf("changed municipality fetched %d times, want 1", f.hits[e.MunicipalityPath(1002)])
	}
	if f.hits[e.MunicipalityPath(1003)] != 0 {
		t.Errorf("unchanged municipality fetched %d times, want 0", f.hits[e.MunicipalityPath(1003)])
	}
	if f.hits[e.ProvincePath(1)] != 1 {
		t.Errorf("province fetched %d times, want 1", f.hits[e.ProvincePath(1)])
	}
	if len(p.dirty) != 0 {
		t.Errorf("dirty = %v, want empty", p.dirty)
	}
}

func TestDirtyRetriesUntilFileRefreshesThenGivesUp(t *testing.T) {
	p, f, _, now := setup(t)
	ctx := context.Background()

	// National reports new counts, but the municipality file hasn't refreshed.
	f.set(e.NationalPath(), nationalBody(map[int]int{1001: 1, 1002: 9, 1003: 1}))
	p.Tick(ctx, now.Add(30*time.Second))
	if _, ok := p.dirty[1002]; !ok {
		t.Fatal("municipality should stay dirty while its file is unchanged")
	}

	f.set(e.MunicipalityPath(1002), `{"id":1002,"v":1}`)
	p.Tick(ctx, now.Add(60*time.Second))
	if _, ok := p.dirty[1002]; ok {
		t.Fatal("municipality should be clean once its file changes")
	}

	f.set(e.NationalPath(), nationalBody(map[int]int{1001: 1, 1002: 10, 1003: 1}))
	for i := range p.cfg.MaxDirtyTries {
		p.Tick(ctx, now.Add(time.Duration(90+30*i)*time.Second))
	}
	if _, ok := p.dirty[1002]; ok {
		t.Fatal("should give up after MaxDirtyTries and leave it to the sweep")
	}
}

func TestSentinelTriggersSweepAfterDelayAndGap(t *testing.T) {
	p, f, r, now := setup(t)
	ctx := context.Background()
	other := e.MunicipalityPath(1003)

	f.set(e.MunicipalityPath(1001), `{"id":1001,"v":1}`)
	f.set(other, `{"id":1003,"v":1}`)

	p.Tick(ctx, now.Add(30*time.Second))
	if r.versions[other] != 1 {
		t.Fatalf("other municipality has %d versions before the sweep, want 1", r.versions[other])
	}

	// Past the delay but inside the minimum gap between sweeps: no sweep yet.
	p.Tick(ctx, now.Add(2*time.Minute))
	if r.versions[other] != 1 {
		t.Fatal("swept before the minimum gap elapsed")
	}

	p.Tick(ctx, now.Add(3*time.Minute))
	if r.versions[other] != 2 {
		t.Errorf("other municipality has %d versions after the sweep, want 2", r.versions[other])
	}
	if !p.sweepAfter.IsZero() {
		t.Error("sentinel trigger should be cleared by the sweep")
	}
}

func TestTimedSweepRunsWithoutAnySignal(t *testing.T) {
	p, f, _, now := setup(t)
	p.Tick(context.Background(), now.Add(10*time.Minute))
	if f.hits[e.MunicipalityPath(1003)] != 1 {
		t.Errorf("sweep fetched municipality %d times, want 1", f.hits[e.MunicipalityPath(1003)])
	}
}

func TestBadNationalFileKeepsPreviousCounters(t *testing.T) {
	p, f, _, now := setup(t)
	before := len(p.counters)
	schemaErrors := 0
	p.cfg.OnSchema = func(string, error) { schemaErrors++ }

	f.set(e.NationalPath(), `{"ReportDate":"2026-11-05T10:00:00+02:00","Renamed":[]}`)
	p.Tick(context.Background(), now.Add(30*time.Second))

	if schemaErrors != 1 {
		t.Errorf("schema errors = %d, want 1", schemaErrors)
	}
	if len(p.counters) != before || len(p.dirty) != 0 {
		t.Errorf("counters=%d dirty=%v, want previous state kept", len(p.counters), p.dirty)
	}
}

func TestMissingMunicipalityFileIsNotAnError(t *testing.T) {
	p, f, _, now := setup(t)
	f.mu.Lock()
	delete(f.files, e.MunicipalityPath(1003))
	f.mu.Unlock()
	p.Tick(context.Background(), now.Add(10*time.Minute))
	if !p.LastSuccess().After(now.Add(-time.Hour)) {
		t.Error("expected a recorded success")
	}
}
