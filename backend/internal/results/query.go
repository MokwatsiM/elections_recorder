package results

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// AreaSummary is a compact row for lists and maps.
type AreaSummary struct {
	ID         int           `json:"id"`
	Name       string        `json:"name"`
	Code       string        `json:"code,omitempty"`
	ParentID   int           `json:"parentId,omitempty"`
	IsMetro    bool          `json:"isMetro,omitempty"`
	Reported   bool          `json:"reported"`
	LeaderID   int           `json:"leaderId,omitempty"`
	LeaderPct  float64       `json:"leaderPct,omitempty"`
	Valid      int           `json:"valid,omitempty"`
	VDPct      *float64      `json:"vdPct,omitempty"`
	Status     CouncilStatus `json:"status,omitempty"`
	PartyPct   *float64      `json:"partyPct,omitempty"` // when a party was requested
	PartyVotes *int          `json:"partyVotes,omitempty"`
	PartySwing *float64      `json:"partySwing,omitempty"` // requested party, against 2021
	WinnerID   int           `json:"winnerId,omitempty"`
}

func fill(s *AreaSummary, ps []Party, partyID *int) {
	if l := leader(ps); l != nil {
		s.Reported, s.LeaderID, s.LeaderPct = true, l.ID, l.Pct
	}
	for _, p := range ps {
		s.Valid += p.Votes
	}
	if partyID != nil && s.Reported {
		pct, votes := 0.0, 0
		for _, p := range ps {
			if p.ID == *partyID {
				pct, votes = p.Pct, p.Votes
			}
		}
		s.PartyPct, s.PartyVotes = &pct, &votes
	}
}

func fillArea(s *AreaSummary, a *Area, partyID *int) {
	fill(s, a.Parties, partyID)
	s.Valid = a.Votes.Valid
	pct := a.Progress.VDPct()
	s.VDPct = &pct
}

// swing is a party's change against 2021, or nil if that year isn't available.
func swing(pct *float64, past []Support, partyID *int) *float64 {
	if pct == nil || partyID == nil || !hasYear(past, baselineYear) {
		return nil
	}
	base := 0.0
	for _, r := range past {
		if r.Year == baselineYear && r.PartyID == *partyID {
			base = r.Pct
		}
	}
	d := *pct - base
	return &d
}

type NationalView struct {
	Area
	Provinces []AreaSummary `json:"provinces"`
}

func (e *Engine) National(partyID *int) (*NationalView, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.national == nil {
		return nil, false
	}
	v := &NationalView{Area: *e.national, Provinces: e.provinceRows(partyID)}
	return v, true
}

func (e *Engine) provinceRows(partyID *int) []AreaSummary {
	rows := make([]AreaSummary, 0, len(e.provinces))
	for _, a := range e.provinces {
		s := AreaSummary{ID: a.ID, Name: a.Name}
		fillArea(&s, a, partyID)
		s.PartySwing = swing(s.PartyPct, e.refs.Load().Province[a.ID], partyID)
		rows = append(rows, s)
	}
	slices.SortFunc(rows, func(a, b AreaSummary) int { return cmp.Compare(a.ID, b.ID) })
	return rows
}

type ProvinceView struct {
	Area
	Municipalities []AreaSummary `json:"municipalities"`
}

func (e *Engine) Province(id int, partyID *int) (*ProvinceView, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	a, ok := e.provinces[id]
	if !ok {
		return nil, false
	}
	return &ProvinceView{Area: *a, Municipalities: e.municipalityRows(id, partyID)}, true
}

// municipalityRows lists municipalities that have a results file. province 0
// means all of them.
func (e *Engine) municipalityRows(province int, partyID *int) []AreaSummary {
	rows := make([]AreaSummary, 0, len(e.munis))
	for _, m := range e.munis {
		if province != 0 && m.ParentID != province {
			continue
		}
		s := AreaSummary{ID: m.ID, Name: m.Name, Code: m.Code, ParentID: m.ParentID, IsMetro: m.IsMetro}
		fillArea(&s, &m.Area, partyID)
		s.PartySwing = swing(s.PartyPct, e.refs.Load().Munic[m.ID], partyID)
		if m.Council != nil {
			s.Status = m.Council.Status
		}
		rows = append(rows, s)
	}
	slices.SortFunc(rows, func(a, b AreaSummary) int { return strings.Compare(a.Name, b.Name) })
	return rows
}

type MunicipalityView struct {
	Area
	ProvinceName string        `json:"provinceName"`
	Wards        []AreaSummary `json:"wards"`
}

func (e *Engine) Municipality(id int, partyID *int) (*MunicipalityView, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	m, ok := e.munis[id]
	if !ok {
		return nil, false
	}
	v := &MunicipalityView{Area: m.Area, Wards: wardRows(m, partyID)}
	if p, ok := e.provinces[m.ParentID]; ok {
		v.ProvinceName = p.Name
	}
	return v, true
}

func wardRows(m *Municipality, partyID *int) []AreaSummary {
	rows := make([]AreaSummary, 0, len(m.Wards))
	for i := range m.Wards {
		w := &m.Wards[i]
		s := AreaSummary{ID: w.ID, Name: fmt.Sprintf("Ward %d", w.No), ParentID: m.ID, WinnerID: w.WinnerID}
		fill(&s, w.Parties, partyID)
		reported := 0
		for j := range w.VDs {
			if w.VDs[j].Reported {
				reported++
			}
		}
		if n := len(w.VDs); n > 0 {
			pct := float64(reported) * 100 / float64(n)
			s.VDPct = &pct
		}
		rows = append(rows, s)
	}
	return rows
}

type WardView struct {
	ID           int           `json:"id"`
	No           int           `json:"no"`
	Municipality AreaRef       `json:"municipality"`
	ProvinceName string        `json:"provinceName"`
	ReportDate   string        `json:"reportDate"`
	Reported     bool          `json:"reported"`
	WinnerID     int           `json:"winnerId,omitempty"`
	Candidate    string        `json:"candidate,omitempty"`
	Parties      []Party       `json:"parties"`
	VDs          []AreaSummary `json:"vds"`
}

func (e *Engine) Ward(municID, wardID int, partyID *int) (*WardView, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	m, ok := e.munis[municID]
	w, wok := e.wardIndex[wardID]
	if !ok || !wok || w.MunicID != municID {
		return nil, false
	}
	v := &WardView{
		ID: w.ID, No: w.No, Municipality: refOf(&m.Area), ReportDate: w.ReportDate.Format("2006-01-02T15:04:05Z07:00"),
		Reported: w.Reported, WinnerID: w.WinnerID, Candidate: w.Candidate, Parties: w.Parties,
		VDs: e.vdRows(w, partyID),
	}
	if p, ok := e.provinces[m.ParentID]; ok {
		v.ProvinceName = p.Name
	}
	return v, true
}

func (e *Engine) vdRows(w *Ward, partyID *int) []AreaSummary {
	names := e.refs.Load().VDNames
	rows := make([]AreaSummary, 0, len(w.VDs))
	for i := range w.VDs {
		v := &w.VDs[i]
		s := AreaSummary{ID: v.ID, Name: vdName(names, v.ID), ParentID: w.ID}
		fill(&s, v.Parties, partyID)
		rows = append(rows, s)
	}
	return rows
}

func vdName(names map[int]string, id int) string {
	if n := names[id]; n != "" {
		return n
	}
	return "Voting district " + strconv.Itoa(id)
}

type VDView struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	WardID       int     `json:"wardId"`
	WardNo       int     `json:"wardNo"`
	Municipality AreaRef `json:"municipality"`
	ProvinceName string  `json:"provinceName"`
	ReportDate   string  `json:"reportDate"`
	Reported     bool    `json:"reported"`
	Parties      []Party `json:"parties"`
}

func (e *Engine) VD(municID, vdID int) (*VDView, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	m, ok := e.munis[municID]
	v, vok := e.vdIndex[vdID]
	if !ok || !vok {
		return nil, false
	}
	w, wok := e.wardIndex[v.WardID]
	if !wok || w.MunicID != municID {
		return nil, false
	}
	out := &VDView{
		ID: v.ID, Name: vdName(e.refs.Load().VDNames, v.ID), WardID: v.WardID, WardNo: w.No,
		Municipality: refOf(&m.Area), ReportDate: v.ReportDate.Format("2006-01-02T15:04:05Z07:00"),
		Reported: v.Reported, Parties: v.Parties,
	}
	if p, ok := e.provinces[m.ParentID]; ok {
		out.ProvinceName = p.Name
	}
	return out, true
}

type CouncilRow struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Code     string  `json:"code"`
	Province int     `json:"province"`
	IsMetro  bool    `json:"isMetro"`
	Council  Council `json:"council"`
	VDPct    float64 `json:"vdPct"`
	Parties  []Party `json:"parties"`
}

// Councils lists municipalities that have results, largest seat count first
// within each group. Empty filters match everything.
func (e *Engine) Councils(status string, metroOnly bool) []CouncilRow {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rows := make([]CouncilRow, 0, len(e.munis))
	for _, m := range e.munis {
		if m.Council == nil || (status != "" && string(m.Council.Status) != status) || (metroOnly && !m.IsMetro) {
			continue
		}
		rows = append(rows, CouncilRow{
			ID: m.ID, Name: m.Name, Code: m.Code, Province: m.ParentID, IsMetro: m.IsMetro,
			Council: *m.Council, VDPct: m.Progress.VDPct(), Parties: seatOrVoteLeaders(m.Parties, 6),
		})
	}
	slices.SortFunc(rows, func(a, b CouncilRow) int {
		if a.IsMetro != b.IsMetro {
			if a.IsMetro {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return rows
}

func seatOrVoteLeaders(ps []Party, n int) []Party {
	out := slices.Clone(ps)
	slices.SortStableFunc(out, func(a, b Party) int {
		if a.Seats != b.Seats {
			return b.Seats - a.Seats
		}
		return b.Votes - a.Votes
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// MapRows returns one row per area for colouring a map.
func (e *Engine) MapRows(level Level, parent int, partyID *int) ([]AreaSummary, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	switch level {
	case LevelProvince:
		return e.provinceRows(partyID), nil
	case LevelMunicipality:
		return e.municipalityRows(parent, partyID), nil
	case LevelWard:
		var rows []AreaSummary
		for _, m := range e.munis {
			if parent == 0 || m.ID == parent {
				rows = append(rows, wardRows(m, partyID)...)
			}
		}
		return rows, nil
	case LevelVD:
		m, ok := e.munis[parent]
		if !ok {
			return nil, fmt.Errorf("municipality %d has no results", parent)
		}
		var rows []AreaSummary
		for i := range m.Wards {
			rows = append(rows, e.vdRows(&m.Wards[i], partyID)...)
		}
		return rows, nil
	}
	return nil, fmt.Errorf("unknown map level %q", level)
}

type SearchHit struct {
	Level   Level  `json:"level"`
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Context string `json:"context,omitempty"`
	MunicID int    `json:"municId,omitempty"`
	WardID  int    `json:"wardId,omitempty"`
}

// Search matches places, wards and voting stations. The data is small enough
// for a scan: about 29,000 rows, and results are cached at the edge.
func (e *Engine) Search(q string, limit int) []SearchHit {
	q = strings.ToLower(strings.TrimSpace(q))
	if len(q) < 2 {
		return []SearchHit{}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	names := e.refs.Load().VDNames

	var prefix, contains []SearchHit
	add := func(h SearchHit, text string) {
		switch i := strings.Index(text, q); {
		case i == 0:
			prefix = append(prefix, h)
		case i > 0:
			contains = append(contains, h)
		}
	}
	for _, p := range e.provinces {
		add(SearchHit{Level: LevelProvince, ID: p.ID, Name: p.Name}, strings.ToLower(p.Name))
	}
	munis := make([]*Municipality, 0, len(e.munis))
	for _, m := range e.munis {
		munis = append(munis, m)
	}
	slices.SortFunc(munis, func(a, b *Municipality) int { return cmp.Compare(a.ID, b.ID) })
	for _, m := range munis {
		add(SearchHit{Level: LevelMunicipality, ID: m.ID, Name: m.Name, Context: m.Code, MunicID: m.ID}, strings.ToLower(m.Name+" "+m.Code))
	}
	for _, m := range munis {
		for i := range m.Wards {
			w := &m.Wards[i]
			label := fmt.Sprintf("ward %d %s %d", w.No, strings.ToLower(m.Name), w.ID)
			add(SearchHit{Level: LevelWard, ID: w.ID, Name: fmt.Sprintf("Ward %d", w.No), Context: m.Name, MunicID: m.ID, WardID: w.ID}, label)
			for j := range w.VDs {
				v := &w.VDs[j]
				name := vdName(names, v.ID)
				add(SearchHit{Level: LevelVD, ID: v.ID, Name: name, Context: fmt.Sprintf("Ward %d, %s", w.No, m.Name), MunicID: m.ID, WardID: w.ID},
					strings.ToLower(name)+" "+strconv.Itoa(v.ID))
			}
		}
		if len(prefix) >= limit*4 && len(contains) >= limit {
			break
		}
	}
	rank := func(l Level) int {
		return slices.Index([]Level{LevelProvince, LevelMunicipality, LevelWard, LevelVD}, l)
	}
	byLevel := func(a, b SearchHit) int { return rank(a.Level) - rank(b.Level) }
	slices.SortStableFunc(prefix, byLevel)
	slices.SortStableFunc(contains, byLevel)
	out := append(prefix, contains...)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// PartySeries is one party's support through past elections and today.
type PartySeries struct {
	PartyID int       `json:"partyId"`
	Abbr    string    `json:"abbr"`
	Years   []YearPct `json:"years"`
	Current *float64  `json:"current"` // nil if the party hasn't reported here
	Swing   *float64  `json:"swing"`   // current minus 2021
}

type YearPct struct {
	Year int     `json:"year"`
	Pct  float64 `json:"pct"`
}

type HistoryView struct {
	Level Level  `json:"level"`
	ID    int    `json:"id"`
	Name  string `json:"name"`
	// Comparable is false where boundaries changed, so swing can mislead.
	Comparable bool          `json:"comparable"`
	Note       string        `json:"note,omitempty"`
	Parties    []PartySeries `json:"parties"`
}

const baselineYear = 2021

func (e *Engine) History(level Level, id, municID int) (*HistoryView, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	refs := e.refs.Load()
	v := &HistoryView{Level: level, ID: id}
	var past []Support
	var current []Party
	switch level {
	case LevelNational:
		if e.national == nil {
			return nil, false
		}
		v.Name, past, current, v.Comparable = e.national.Name, refs.National, e.national.Parties, true
	case LevelProvince:
		a, ok := e.provinces[id]
		if !ok {
			return nil, false
		}
		v.Name, past, current, v.Comparable = a.Name, refs.Province[id], a.Parties, true
	case LevelMunicipality:
		m, ok := e.munis[id]
		if !ok {
			return nil, false
		}
		v.Name, past, current = m.Name, refs.Munic[id], m.Parties
		v.Comparable = hasYear(past, baselineYear)
		v.Note = "Municipal boundaries can change between elections, so comparisons are indicative."
	case LevelWard:
		w, ok := e.wardIndex[id]
		if !ok || w.MunicID != municID {
			return nil, false
		}
		v.Name, past, current = fmt.Sprintf("Ward %d", w.No), refs.Ward[municID][id], w.Parties
		v.Comparable = hasYear(past, baselineYear)
		if v.Comparable {
			v.Note = "This ward ID also existed in 2021, but its boundary may have been redrawn. Treat the swing as indicative."
		} else {
			v.Note = "This ward has no 2021 equivalent, so no swing is shown."
		}
	default:
		return nil, false
	}
	v.Parties = buildSeries(past, current, v.Comparable, refs.Parties)
	return v, true
}

func hasYear(s []Support, y int) bool {
	return slices.ContainsFunc(s, func(r Support) bool { return r.Year == y })
}

func buildSeries(past []Support, current []Party, comparable bool, info map[int]PartyInfo) []PartySeries {
	byParty := map[int]*PartySeries{}
	get := func(id int, abbr string) *PartySeries {
		s, ok := byParty[id]
		if !ok {
			if abbr == "" {
				abbr = info[id].Abbr
			}
			s = &PartySeries{PartyID: id, Abbr: abbr, Years: []YearPct{}}
			byParty[id] = s
		}
		return s
	}
	for _, r := range past {
		s := get(r.PartyID, r.Abbr)
		s.Years = append(s.Years, YearPct{r.Year, r.Pct})
	}
	for _, p := range current {
		s := get(p.ID, "")
		pct := p.Pct
		s.Current = &pct
	}
	out := make([]PartySeries, 0, len(byParty))
	for _, s := range byParty {
		slices.SortFunc(s.Years, func(a, b YearPct) int { return a.Year - b.Year })
		if comparable && s.Current != nil {
			base := 0.0
			for _, y := range s.Years {
				if y.Year == baselineYear {
					base = y.Pct
				}
			}
			swing := *s.Current - base
			s.Swing = &swing
		}
		out = append(out, *s)
	}
	score := func(s PartySeries) float64 {
		if s.Current != nil {
			return 1000 + *s.Current
		}
		for _, y := range s.Years {
			if y.Year == baselineYear {
				return y.Pct
			}
		}
		return -1
	}
	slices.SortFunc(out, func(a, b PartySeries) int { return cmp.Compare(score(b), score(a)) })
	return out
}

// PartyList returns metadata for parties with results in this election.
func (e *Engine) PartyList() []PartyInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	info := e.refs.Load().Parties
	seen := map[int]struct{}{}
	add := func(ps []Party) {
		for _, p := range ps {
			seen[p.ID] = struct{}{}
		}
	}
	if e.national != nil {
		add(e.national.Parties)
	}
	for _, a := range e.provinces {
		add(a.Parties)
	}
	for _, m := range e.munis {
		add(m.Parties)
	}
	out := make([]PartyInfo, 0, len(seen))
	for id := range seen {
		if p, ok := info[id]; ok {
			out = append(out, p)
		} else {
			out = append(out, PartyInfo{ID: id, Abbr: strconv.Itoa(id)})
		}
	}
	slices.SortFunc(out, func(a, b PartyInfo) int { return cmp.Compare(a.ID, b.ID) })
	return out
}
