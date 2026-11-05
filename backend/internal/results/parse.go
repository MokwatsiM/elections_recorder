// Package results turns recorded IEC files into the dashboard's read model and
// the events that describe how it changed.
package results

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"lge-dashboard/backend/internal/iec"
)

type Level string

const (
	LevelNational     Level = "national"
	LevelProvince     Level = "province"
	LevelMunicipality Level = "municipality"
	LevelWard         Level = "ward"
	LevelVD           Level = "vd"
)

// Party is one party's result in an area. TotalValidVotes counts ballots: a
// voter casts a ward and a PR ballot, so Votes is about twice the voters.
type Party struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Votes     int     `json:"votes"`
	Pct       float64 `json:"pct"`
	WardVotes int     `json:"wardVotes,omitempty"`
	PRVotes   int     `json:"prVotes,omitempty"`
	WardSeats int     `json:"wardSeats,omitempty"`
	PRSeats   int     `json:"prSeats,omitempty"`
	Seats     int     `json:"seats,omitempty"`
	// Largest is councils where this party has the most seats, as the IEC's
	// CouncilsWon. It does not mean control: see Controlled.
	Largest    int    `json:"largest,omitempty"`
	Controlled int    `json:"controlled,omitempty"`
	LeadVotes  bool   `json:"leadVotes,omitempty"`
	LeadSeats  bool   `json:"leadSeats,omitempty"`
	Candidate  string `json:"candidate,omitempty"`
}

type Progress struct {
	VDs              int `json:"vds"`
	VDsExpected      int `json:"vdsExpected"`
	Wards            int `json:"wards"`
	WardsExpected    int `json:"wardsExpected"`
	Seats            int `json:"seats"`
	SeatsExpected    int `json:"seatsExpected"`
	Councils         int `json:"councils"`
	CouncilsExpected int `json:"councilsExpected"`
}

// VDPct is calculated from the counts because the IEC's own percentages are
// rounded to whole numbers.
func (p Progress) VDPct() float64 {
	if p.VDsExpected == 0 {
		return 0
	}
	return float64(p.VDs) * 100 / float64(p.VDsExpected)
}

// Votes holds area totals, published only down to municipality. Cast counts
// ballots and each voter casts several, so Cast/Registered is not turnout: it
// exceeded 100% on every finished council checked.
type Votes struct {
	Registered int `json:"registered"`
	Cast       int `json:"cast"`
	Valid      int `json:"valid"`
	Spoilt     int `json:"spoilt"`
	Special    int `json:"special"`
}

type CouncilTally struct {
	Hung     int `json:"hung"`
	Majority int `json:"majority"`
	Drawn    int `json:"drawn"`
	Pending  int `json:"pending"`
}

type CouncilStatus string

const (
	CouncilPending  CouncilStatus = "pending"
	CouncilMajority CouncilStatus = "majority"
	CouncilHung     CouncilStatus = "hung"
	CouncilDrawn    CouncilStatus = "drawn" // a party holds exactly half the seats
)

// Council is a municipality's status, taken only from IEC flags.
type Council struct {
	Status         CouncilStatus `json:"status"`
	ControlParty   int           `json:"controlParty,omitempty"`
	LargestParty   int           `json:"largestParty,omitempty"`
	LargestTied    bool          `json:"largestTied,omitempty"`
	MajorityNeeded int           `json:"majorityNeeded"`
	SeatsAllocated int           `json:"seatsAllocated"`
	SeatsTotal     int           `json:"seatsTotal"`
}

type Area struct {
	Level      Level         `json:"level"`
	ID         int           `json:"id"`
	ParentID   int           `json:"parentId,omitempty"`
	Name       string        `json:"name"`
	Code       string        `json:"code,omitempty"`
	IsMetro    bool          `json:"isMetro,omitempty"`
	ReportDate time.Time     `json:"reportDate"`
	Progress   Progress      `json:"progress"`
	Votes      Votes         `json:"votes"`
	Tally      *CouncilTally `json:"tally,omitempty"`
	Council    *Council      `json:"council,omitempty"`
	Parties    []Party       `json:"parties"`
}

type VD struct {
	ID         int       `json:"id"`
	WardID     int       `json:"wardId"`
	ReportDate time.Time `json:"reportDate"`
	Reported   bool      `json:"reported"`
	Parties    []Party   `json:"parties"`
}

type Ward struct {
	ID         int       `json:"id"`
	No         int       `json:"no"`
	MunicID    int       `json:"municId"`
	ReportDate time.Time `json:"reportDate"`
	Reported   bool      `json:"reported"`
	WinnerID   int       `json:"winnerId,omitempty"` // set once the IEC allocates the ward seat
	Candidate  string    `json:"candidate,omitempty"`
	Parties    []Party   `json:"parties"`
	VDs        []VD      `json:"vds,omitempty"`
}

type Municipality struct {
	Area
	Wards []Ward `json:"wards"`
}

type rawParty struct {
	ID                  int
	Name                string
	TotalValidVotes     int
	PercOfVotes         float64
	WardVotes           int
	PRVotes             int
	WardSeatsWon        int
	PRSeatsWon          int
	TotalSeatsWon       int
	CouncilsWon         int
	MajorityWon         int
	IsCouncilWinner     bool
	IsLeadingPartySeats bool
	IsLeadingPartyVotes bool
	RegisteredVoters    int
	TotalVotesCast      int
	ValidVotesCast      int
	SpoiltVotes         int
	SpecialVotes        int
	CandidateName       string
}

type rawCounters struct {
	ReportDate            string
	PartyBallotResults    []rawParty
	ProvinceExpected      int
	MunicExpected         int
	WardExpected          int
	VDExpected            int
	SeatsExpected         int
	MuncipalitiesComplete int
	WardsComplete         int
	VDsComplete           int
	SeatsComplete         int
	HungCouncils          int
	DeadLockDrawnCouncils int
	MajorityCouncils      int
	TotalDrawnCouncils    int
}

type rawNational struct {
	rawCounters
	ElectoralEventID int
	ProvinceResults  []json.RawMessage
}

type rawProvince struct {
	rawCounters
	ProvinceID int
	Province   string
}

type rawVD struct {
	VotingDistrictID   int
	ReportDate         string
	PartyBallotResults []rawParty
}

type rawWard struct {
	WardID                int
	ReportDate            string
	VotingDistrictResults []rawVD
	PartyBallotResults    []rawParty
}

type rawMunicipality struct {
	rawCounters
	ProvinceID     int
	MunicipalityID int
	Municipality   string
	WardResults    []rawWard
}

func parseNational(body []byte) (*Area, error) {
	var r rawNational
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("%w: %v", iec.ErrSchema, err)
	}
	if len(r.ProvinceResults) != 9 || r.VDExpected == 0 {
		return nil, fmt.Errorf("%w: national provinces=%d vds=%d", iec.ErrSchema, len(r.ProvinceResults), r.VDExpected)
	}
	a := newArea(LevelNational, 0, "South Africa", r.rawCounters)
	a.Tally = tally(r.rawCounters)
	return a, nil
}

func parseProvince(id int, body []byte) (*Area, error) {
	var r rawProvince
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("%w: %v", iec.ErrSchema, err)
	}
	if r.ProvinceID != id || r.Province == "" {
		return nil, fmt.Errorf("%w: province file %d has id=%d name=%q", iec.ErrSchema, id, r.ProvinceID, r.Province)
	}
	a := newArea(LevelProvince, id, r.Province, r.rawCounters)
	a.Tally = tally(r.rawCounters)
	return a, nil
}

func parseMunicipality(id int, body []byte) (*Municipality, error) {
	var r rawMunicipality
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("%w: %v", iec.ErrSchema, err)
	}
	if r.MunicipalityID != id || r.Municipality == "" {
		return nil, fmt.Errorf("%w: municipality file %d has id=%d name=%q", iec.ErrSchema, id, r.MunicipalityID, r.Municipality)
	}
	a := newArea(LevelMunicipality, id, displayName(r.Municipality), r.rawCounters)
	a.ParentID = r.ProvinceID
	a.Code = municipalityCode(r.Municipality)
	a.IsMetro = metroCodes[a.Code]
	a.Council = council(r.rawCounters, a.Parties)

	m := &Municipality{Area: *a, Wards: make([]Ward, 0, len(r.WardResults))}
	for _, rw := range r.WardResults {
		w := Ward{
			ID: rw.WardID, No: rw.WardID % 1000, MunicID: id,
			ReportDate: reportTime(rw.ReportDate),
			Parties:    parties(rw.PartyBallotResults),
			VDs:        make([]VD, 0, len(rw.VotingDistrictResults)),
		}
		w.Reported = len(w.Parties) > 0
		for _, p := range w.Parties {
			if p.WardSeats > 0 {
				w.WinnerID, w.Candidate = p.ID, p.Candidate
				break
			}
		}
		for _, rv := range rw.VotingDistrictResults {
			v := VD{ID: rv.VotingDistrictID, WardID: rw.WardID, ReportDate: reportTime(rv.ReportDate), Parties: parties(rv.PartyBallotResults)}
			v.Reported = len(v.Parties) > 0
			w.VDs = append(w.VDs, v)
		}
		m.Wards = append(m.Wards, w)
	}
	return m, nil
}

func newArea(level Level, id int, name string, r rawCounters) *Area {
	a := &Area{
		Level: level, ID: id, Name: name,
		ReportDate: reportTime(r.ReportDate),
		Parties:    parties(r.PartyBallotResults),
		Progress: Progress{
			VDs: r.VDsComplete, VDsExpected: r.VDExpected,
			Wards: r.WardsComplete, WardsExpected: r.WardExpected,
			Seats: r.SeatsComplete, SeatsExpected: r.SeatsExpected,
			Councils: r.MuncipalitiesComplete, CouncilsExpected: r.MunicExpected,
		},
	}
	// The IEC repeats area totals on every party row.
	if len(r.PartyBallotResults) > 0 {
		p := r.PartyBallotResults[0]
		a.Votes = Votes{p.RegisteredVoters, p.TotalVotesCast, p.ValidVotesCast, p.SpoiltVotes, p.SpecialVotes}
	}
	return a
}

// parties keeps only those with votes or seats, leaving out the many zero rows
// so all 23,696 voting districts fit comfortably in memory.
func parties(raw []rawParty) []Party {
	out := make([]Party, 0, 8)
	for _, r := range raw {
		if r.TotalValidVotes == 0 && r.TotalSeatsWon == 0 {
			continue
		}
		candidate := strings.TrimSpace(r.CandidateName)
		if candidate == "-" {
			candidate = ""
		}
		out = append(out, Party{
			ID: r.ID, Name: r.Name, Votes: r.TotalValidVotes, Pct: r.PercOfVotes,
			WardVotes: r.WardVotes, PRVotes: r.PRVotes,
			WardSeats: r.WardSeatsWon, PRSeats: r.PRSeatsWon, Seats: r.TotalSeatsWon,
			Largest: r.CouncilsWon, Controlled: r.MajorityWon,
			LeadVotes: r.IsLeadingPartyVotes, LeadSeats: r.IsLeadingPartySeats,
			Candidate: candidate,
		})
	}
	slices.SortStableFunc(out, func(a, b Party) int { return b.Votes - a.Votes })
	return out
}

func tally(r rawCounters) *CouncilTally {
	return &CouncilTally{
		Hung: r.HungCouncils, Majority: r.MajorityCouncils, Drawn: r.DeadLockDrawnCouncils,
		Pending: r.MunicExpected - r.HungCouncils - r.MajorityCouncils - r.DeadLockDrawnCouncils,
	}
}

func council(r rawCounters, ps []Party) *Council {
	c := &Council{
		Status:         CouncilPending,
		MajorityNeeded: r.SeatsExpected/2 + 1,
		SeatsAllocated: r.SeatsComplete,
		SeatsTotal:     r.SeatsExpected,
		LargestTied:    r.TotalDrawnCouncils > 0,
	}
	switch {
	case r.DeadLockDrawnCouncils > 0:
		c.Status = CouncilDrawn
	case r.MajorityCouncils > 0:
		c.Status = CouncilMajority
	case r.HungCouncils > 0:
		c.Status = CouncilHung
	}
	for _, p := range ps {
		if p.Controlled > 0 {
			c.ControlParty = p.ID
		}
		if p.Largest > 0 {
			c.LargestParty = p.ID
		}
	}
	return c
}

var metroCodes = map[string]bool{"BUF": true, "CPT": true, "EKU": true, "ETH": true, "JHB": true, "MAN": true, "NMA": true, "TSH": true}

func municipalityCode(name string) string {
	code, _, _ := strings.Cut(name, " - ")
	return strings.TrimSpace(code)
}

func displayName(name string) string {
	if _, after, ok := strings.Cut(name, " - "); ok {
		name = strings.TrimSpace(after)
	}
	// Some IEC names are all capitals: "NAMA KHOI" reads as "Nama Khoi".
	if name != strings.ToUpper(name) {
		return name
	}
	words := strings.Fields(strings.ToLower(name))
	for i, w := range words {
		r := []rune(w)
		for j := range r {
			if unicode.IsLetter(r[j]) { // skips the click in names like !Kheis
				r[j] = unicode.ToUpper(r[j])
				break
			}
		}
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

func reportTime(s string) time.Time {
	t, _ := iec.ParseTime(s)
	return t
}

var (
	nationalPathRe = regexp.MustCompile(`/EEID\d+LiteMapsNational\.js$`)
	provincePathRe = regexp.MustCompile(`/EEID\d+Province(\d+)\.js$`)
	municPathRe    = regexp.MustCompile(`/EEID\d+Munic(\d+)\.js$`)
)

func classify(path string) (Level, int, bool) {
	if nationalPathRe.MatchString(path) {
		return LevelNational, 0, true
	}
	if m := provincePathRe.FindStringSubmatch(path); m != nil {
		id, _ := strconv.Atoi(m[1])
		return LevelProvince, id, true
	}
	if m := municPathRe.FindStringSubmatch(path); m != nil {
		id, _ := strconv.Atoi(m[1])
		return LevelMunicipality, id, true
	}
	return "", 0, false
}
