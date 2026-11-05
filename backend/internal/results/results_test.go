package results

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

func golden(t *testing.T, name string) []byte {
	t.Helper()
	f, err := os.Open("../iec/testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseRealNationalMatchesTheIEC(t *testing.T) {
	a, err := parseNational(golden(t, "national"))
	if err != nil {
		t.Fatal(err)
	}
	if a.Progress.VDsExpected != 23696 || a.Progress.CouncilsExpected != 213 || a.Progress.SeatsExpected != 8825 {
		t.Errorf("expected counts: %+v", a.Progress)
	}
	// These are the figures the IEC's own dashboard displayed for this file.
	if *a.Tally != (CouncilTally{Hung: 19, Majority: 8, Drawn: 0, Pending: 186}) {
		t.Errorf("tally = %+v", *a.Tally)
	}
	anc := a.Parties[0]
	if anc.ID != 7 || anc.Votes != 1550122 || anc.Pct != 31.85 || anc.Seats != 264 || anc.Largest != 16 || anc.Controlled != 7 {
		t.Errorf("ANC = %+v", anc)
	}
	if a.Votes.Cast != 4943726 || a.Votes.Registered != 29230808 {
		t.Errorf("votes = %+v", a.Votes)
	}
}

func TestParseRealMunicipalityCoversWardsAndVDs(t *testing.T) {
	m, err := parseMunicipality(1001, golden(t, "munic1001"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "Nelson Mandela Bay" || m.Code != "NMA" || !m.IsMetro || m.ParentID != 1 {
		t.Errorf("identity: %+v", m.Area)
	}
	if len(m.Wards) != 60 {
		t.Fatalf("wards = %d", len(m.Wards))
	}
	vds, reported := 0, 0
	for _, w := range m.Wards {
		for _, v := range w.VDs {
			vds++
			if v.Reported {
				reported++
			}
		}
	}
	if vds != 268 {
		t.Errorf("vds = %d, want 268", vds)
	}
	if reported == 0 || reported == vds {
		t.Errorf("reported = %d of %d, expected a partial count", reported, vds)
	}
	if m.Council.Status != CouncilPending || m.Council.MajorityNeeded != 61 {
		t.Errorf("council = %+v", m.Council)
	}
}

func TestLargestPartyIsNotTreatedAsControl(t *testing.T) {
	// Blue Crane Route: the IEC flags the ANC as IsCouncilWinner on 5 of 11 seats.
	r := rawCounters{HungCouncils: 1, SeatsExpected: 11, SeatsComplete: 11, MuncipalitiesComplete: 1}
	c := council(r, []Party{{ID: 7, Seats: 5, Largest: 1, Controlled: 0}})
	if c.Status != CouncilHung || c.ControlParty != 0 || c.LargestParty != 7 {
		t.Errorf("hung council = %+v", c)
	}

	r = rawCounters{MajorityCouncils: 1, SeatsExpected: 13, SeatsComplete: 13}
	c = council(r, []Party{{ID: 7, Seats: 7, Largest: 1, Controlled: 1}})
	if c.Status != CouncilMajority || c.ControlParty != 7 || c.MajorityNeeded != 7 {
		t.Errorf("majority council = %+v", c)
	}

	r = rawCounters{DeadLockDrawnCouncils: 1, SeatsExpected: 10}
	if c = council(r, nil); c.Status != CouncilDrawn {
		t.Errorf("drawn council = %+v", c)
	}
	if c = council(rawCounters{SeatsExpected: 10}, nil); c.Status != CouncilPending {
		t.Errorf("pending council = %+v", c)
	}
}

func TestZeroVoteVDIsNotReported(t *testing.T) {
	m, err := parseMunicipality(1, []byte(`{"MunicipalityID":1,"Municipality":"X - Y","WardResults":[{"WardID":100,"VotingDistrictResults":[
		{"VotingDistrictID":5,"PartyBallotResults":[{"ID":7,"TotalValidVotes":0}]},
		{"VotingDistrictID":6,"PartyBallotResults":[{"ID":7,"TotalValidVotes":9}]}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	vds := m.Wards[0].VDs
	if vds[0].Reported || len(vds[0].Parties) != 0 {
		t.Errorf("empty VD = %+v", vds[0])
	}
	if !vds[1].Reported {
		t.Errorf("VD with votes = %+v", vds[1])
	}
}

func TestParseRejectsChangedStructure(t *testing.T) {
	if _, err := parseNational([]byte(`{"ProvinceResults":[]}`)); err == nil {
		t.Error("national: expected an error")
	}
	if _, err := parseProvince(1, []byte(`{"ProvinceID":2,"Province":"X"}`)); err == nil {
		t.Error("province: expected an error for wrong ID")
	}
	if _, err := parseMunicipality(1, []byte(`{"MunicipalityID":1}`)); err == nil {
		t.Error("municipality: expected an error for missing name")
	}
}

func testMuni(mut func(*Municipality)) *Municipality {
	m := &Municipality{
		Area: Area{Level: LevelMunicipality, ID: 1, ParentID: 2, Name: "Test",
			Progress: Progress{VDs: 50, VDsExpected: 100},
			Council:  &Council{Status: CouncilPending},
			Parties:  []Party{{ID: 7, Name: "A", Votes: 60}, {ID: 52, Name: "B", Votes: 40}}},
		Wards: []Ward{{ID: 10, No: 1}},
	}
	if mut != nil {
		mut(m)
	}
	return m
}

func types(evs []Event) map[string]int {
	out := map[string]int{}
	for _, e := range evs {
		out[e.Type]++
	}
	return out
}

func TestDiffFirstSightingIsOnlyABaseline(t *testing.T) {
	cur := testMuni(func(m *Municipality) { m.Wards[0].WinnerID = 7 })
	if evs := diffMunicipality(nil, cur, time.Now()); len(evs) != 0 {
		t.Errorf("events = %v", types(evs))
	}
}

func TestDiffEmitsDomainEvents(t *testing.T) {
	old := testMuni(nil)
	cur := testMuni(func(m *Municipality) {
		m.Wards[0].WinnerID = 7
		m.Wards[0].Parties = []Party{{ID: 7, Name: "A"}}
		m.Council = &Council{Status: CouncilHung, LargestParty: 7}
		m.Parties = []Party{{ID: 52, Name: "B", Votes: 90, PRSeats: 2, Seats: 2}, {ID: 7, Name: "A", Votes: 80}}
	})
	got := types(diffMunicipality(old, cur, time.Now()))
	for _, want := range []string{EventWardDeclared, EventCouncilChange, EventLeadChanged, EventSeatsChanged} {
		if got[want] != 1 {
			t.Errorf("%s: %d events, all = %v", want, got[want], got)
		}
	}
}

func TestDiffNoChangeProducesNothing(t *testing.T) {
	if evs := diffMunicipality(testMuni(nil), testMuni(nil), time.Now()); len(evs) != 0 {
		t.Errorf("events = %v", types(evs))
	}
}

func TestEarlyLeadChangeIsNotNews(t *testing.T) {
	old := testMuni(func(m *Municipality) { m.Progress.VDs = 1 })
	cur := testMuni(func(m *Municipality) {
		m.Progress.VDs = 2
		m.Parties = []Party{{ID: 52, Name: "B", Votes: 90}, {ID: 7, Name: "A", Votes: 80}}
	})
	if got := types(diffMunicipality(old, cur, time.Now())); got[EventLeadChanged] != 0 {
		t.Errorf("events = %v", got)
	}
}

func TestMilestonesFireOncePerStep(t *testing.T) {
	a := func(vds int) *Area {
		return &Area{Level: LevelNational, Progress: Progress{VDs: vds, VDsExpected: 100}}
	}
	got := diffNational(a(4), a(16), time.Now())
	var pcts []int
	for _, e := range got {
		pcts = append(pcts, e.Data.(Milestone).Percent)
	}
	if len(pcts) != 3 || pcts[0] != 5 || pcts[2] != 15 {
		t.Errorf("milestones = %v", pcts)
	}
	if got := diffNational(a(16), a(17), time.Now()); len(got) != 0 {
		t.Errorf("no step crossed, got %v", types(got))
	}
}

type memSource struct {
	mu       sync.Mutex
	versions []Version
}

func (s *memSource) Hydration(_ context.Context, cursor int64) ([]Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	latest := map[string]Version{}
	for _, v := range s.versions {
		if v.ID <= cursor {
			latest[v.Path] = v
		}
	}
	out := make([]Version, 0, len(latest))
	for _, v := range latest {
		out = append(out, v)
	}
	return out, nil
}

func (s *memSource) After(_ context.Context, cursor int64, limit int) ([]Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Version
	for _, v := range s.versions {
		if v.ID > cursor && len(out) < limit {
			out = append(out, v)
		}
	}
	return out, nil
}

type capturePub struct {
	mu     sync.Mutex
	events []string
	seqs   []int64
}

func (p *capturePub) Publish(seq int64, typ string, _ time.Time, _ []string, _ any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, typ)
	p.seqs = append(p.seqs, seq)
}

func munic(id, seats int, date string) []byte {
	b, _ := json.Marshal(map[string]any{
		"MunicipalityID": id, "ProvinceID": 1, "Municipality": "ABC - Town", "ReportDate": date, "VDExpected": 10, "VDsComplete": 5,
		"PartyBallotResults": []map[string]any{{"ID": 7, "Name": "A", "TotalValidVotes": 10, "TotalSeatsWon": seats}},
		"WardResults":        []map[string]any{{"WardID": 100001, "ReportDate": date, "PartyBallotResults": []map[string]any{{"ID": 7, "Name": "A", "TotalValidVotes": 10, "WardSeatsWon": seats}}}},
	})
	return b
}

func newTestEngine(src Source, sink Sink, pub Publisher) *Engine {
	return NewEngine(src, sink, pub, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestEngineAppliesInOrderSkipsOldAndKeepsDataOnBadFile(t *testing.T) {
	const path = "dashboards/lge/MapsJason/2026/EEID1887Munic5.js"
	src := &memSource{versions: []Version{
		{ID: 1, Path: path, Body: munic(5, 0, "2026-11-05T10:00:00+02:00"), FetchedAt: time.Now()},
		{ID: 2, Path: path, Body: munic(5, 1, "2026-11-05T10:06:00+02:00"), FetchedAt: time.Now()},
		{ID: 3, Path: path, Body: munic(5, 0, "2026-11-05T09:00:00+02:00"), FetchedAt: time.Now()}, // older
		{ID: 4, Path: path, Body: []byte(`{"ReportDate":"2026-11-05T11:00:00+02:00","Renamed":1}`), FetchedAt: time.Now()},
		{ID: 5, Path: "dashboards/lge/MapsJason/2026/EEID1887BigMapsNational.js", Body: []byte(`{}`), FetchedAt: time.Now()},
	}}
	sink, pub := NewMemorySink(), &capturePub{}
	e := newTestEngine(src, sink, pub)
	schema := 0
	e.OnSchema = func(string, error) { schema++ }

	if _, err := e.drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e.cursor != 5 {
		t.Errorf("cursor = %d", e.cursor)
	}
	m, ok := e.Municipality(5, nil)
	if !ok || m.Wards[0].WinnerID != 7 {
		t.Fatalf("municipality = %+v", m)
	}
	if schema != 1 {
		t.Errorf("schema errors = %d", schema)
	}
	feed, _ := sink.Feed(context.Background(), 0, 10)
	if len(feed) != 1 || feed[0].Type != EventWardDeclared {
		t.Errorf("feed = %+v", feed)
	}
	for i := 1; i < len(pub.seqs); i++ {
		if pub.seqs[i] <= pub.seqs[i-1] {
			t.Errorf("sequence not increasing: %v", pub.seqs)
		}
	}
}

type flakySink struct {
	*MemorySink
	fails int
}

func (f *flakySink) Commit(ctx context.Context, c Commit) error {
	if f.fails > 0 {
		f.fails--
		return context.DeadlineExceeded
	}
	return f.MemorySink.Commit(ctx, c)
}

func TestFailedCommitDoesNotLoseEvents(t *testing.T) {
	const path = "dashboards/lge/MapsJason/2026/EEID1887Munic5.js"
	src := &memSource{versions: []Version{
		{ID: 1, Path: path, Body: munic(5, 0, "2026-11-05T10:00:00+02:00"), FetchedAt: time.Now()},
		{ID: 2, Path: path, Body: munic(5, 1, "2026-11-05T10:06:00+02:00"), FetchedAt: time.Now()},
	}}
	sink := &flakySink{MemorySink: NewMemorySink(), fails: 1}
	e := newTestEngine(src, sink, &capturePub{})
	if _, err := e.drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if feed, _ := sink.Feed(context.Background(), 0, 10); len(feed) != 1 {
		t.Errorf("feed = %+v, the ward declaration was lost", feed)
	}
}

func TestHydrationRestoresStateWithoutRepeatingEvents(t *testing.T) {
	const path = "dashboards/lge/MapsJason/2026/EEID1887Munic5.js"
	src := &memSource{versions: []Version{
		{ID: 1, Path: path, Body: munic(5, 0, "2026-11-05T10:00:00+02:00"), FetchedAt: time.Now()},
		{ID: 2, Path: path, Body: munic(5, 1, "2026-11-05T10:06:00+02:00"), FetchedAt: time.Now()},
	}}
	pub := &capturePub{}
	e := newTestEngine(src, &restartSink{MemorySink: NewMemorySink(), cursor: 2, seq: 40}, pub)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- e.Run(ctx) }()
	for !e.Ready() {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	if m, ok := e.Municipality(5, nil); !ok || m.Wards[0].WinnerID != 7 {
		t.Fatalf("state not restored: %+v", m)
	}
	if len(pub.events) != 0 || e.Seq() != 40 {
		t.Errorf("events = %v seq = %d", pub.events, e.Seq())
	}
}

type restartSink struct {
	*MemorySink
	cursor, seq int64
}

func (r *restartSink) LoadCursor(context.Context) (int64, int64, error) { return r.cursor, r.seq, nil }

func TestHistorySwingOnlyWhereComparable(t *testing.T) {
	e := newTestEngine(&memSource{}, NewMemorySink(), &capturePub{})
	e.SetRefs(&Refs{
		Parties:  map[int]PartyInfo{7: {ID: 7, Abbr: "ANC"}},
		National: []Support{{Year: 2021, PartyID: 7, Abbr: "ANC", Pct: 40}, {Year: 2016, PartyID: 7, Abbr: "ANC", Pct: 50}},
		Ward:     map[int]map[int][]Support{1: {10: {{Year: 2016, PartyID: 7, Pct: 30}}}},
	})
	e.national = &Area{Name: "South Africa", Parties: []Party{{ID: 7, Pct: 35, Votes: 1}, {ID: 99, Pct: 5, Votes: 1}}}
	e.munis[1] = testMuni(nil)
	e.wardIndex[10] = &Ward{ID: 10, No: 1, MunicID: 1, Parties: []Party{{ID: 7, Pct: 60, Votes: 1}}}

	v, ok := e.History(LevelNational, 0, 0)
	if !ok || !v.Comparable {
		t.Fatalf("%+v", v)
	}
	if v.Parties[0].PartyID != 7 || *v.Parties[0].Swing != -5 || v.Parties[0].Years[0].Year != 2016 {
		t.Errorf("ANC series = %+v", v.Parties[0])
	}
	if *v.Parties[1].Swing != 5 {
		t.Errorf("new party swing = %v", *v.Parties[1].Swing)
	}

	w, ok := e.History(LevelWard, 10, 1)
	if !ok || w.Comparable || w.Parties[0].Swing != nil || w.Note == "" {
		t.Errorf("a ward with no 2021 equivalent must show no swing: %+v", w)
	}
}

func TestSearchFindsStationsAndPlaces(t *testing.T) {
	e := newTestEngine(&memSource{}, NewMemorySink(), &capturePub{})
	e.SetRefs(&Refs{VDNames: map[int]string{55: "MASAKHE PUBLIC SCHOOL"}})
	m, err := parseMunicipality(1001, golden(t, "munic1001"))
	if err != nil {
		t.Fatal(err)
	}
	m.Wards[0].VDs[0].ID = 55
	e.mu.Lock()
	e.indexMunicipality(m)
	e.mu.Unlock()

	if hits := e.Search("masakhe", 5); len(hits) != 1 || hits[0].Level != LevelVD || hits[0].WardID != m.Wards[0].ID {
		t.Errorf("station hits = %+v", hits)
	}
	if hits := e.Search("nelson", 5); len(hits) == 0 || hits[0].Level != LevelMunicipality {
		t.Errorf("place hits = %+v", hits)
	}
	if hits := e.Search("a", 5); len(hits) != 0 {
		t.Errorf("single letter hits = %d", len(hits))
	}
}

func TestDisplayName(t *testing.T) {
	for in_, want := range map[string]string{
		"NC067 - NAMA KHOI":        "Nama Khoi",
		"NC084 - !KHEIS":           "!Kheis",
		"NMA - Nelson Mandela Bay": "Nelson Mandela Bay",
		"EC101 - Dr. Beyers Naude": "Dr. Beyers Naude",
	} {
		if got := displayName(in_); got != want {
			t.Errorf("%q = %q, want %q", in_, got, want)
		}
	}
}
