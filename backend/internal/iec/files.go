package iec

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// Municipality comes from the IEC's 2026 boundary file. It lists all 213
// councils, whereas the national results file omits any without results yet.
type Municipality struct {
	ID       int    `json:"id"`
	Province int    `json:"province"`
	Code     string `json:"code"`
	Name     string `json:"name"`
}

//go:embed municipalities.json
var municipalitiesJSON []byte

func Municipalities() []Municipality {
	var ms []Municipality
	if err := json.Unmarshal(municipalitiesJSON, &ms); err != nil {
		panic(fmt.Sprintf("iec: embedded municipalities.json: %v", err))
	}
	return ms
}

// Election identifies one electoral event's results folder.
type Election struct {
	Year int
	EEID int
}

var (
	Election2026 = Election{2026, 1887}
	PastElection = []Election{{2021, 1091}, {2016, 402}, {2011, 197}, {2006, 95}, {2000, 2}}
)

const resultsDir = "dashboards/lge/MapsJason/"

func (e Election) file(name string) string {
	return fmt.Sprintf("%s%d/EEID%d%s.js", resultsDir, e.Year, e.EEID, name)
}

func (e Election) NationalPath() string    { return e.file("LiteMapsNational") }
func (e Election) BigNationalPath() string { return e.file("BigMapsNational") }
func (e Election) ProvincePath(id int) string {
	return e.file(fmt.Sprintf("Province%d", id))
}
func (e Election) MunicipalityPath(id int) string {
	return e.file(fmt.Sprintf("Munic%d", id))
}

// National is the part of the national file needed to detect what changed.
type National struct {
	ElectoralEventID int
	ReportDate       string
	ProvinceResults  []struct {
		ProvinceID          int
		MunicipalityResults []struct {
			MunicipalityID int
			Counters
		}
	}
}

// Counters advance whenever a municipality captures new results.
type Counters struct {
	VDsComplete   int
	WardsComplete int
	SeatsComplete int
}

var ErrSchema = fmt.Errorf("iec: unexpected file structure")

// ParseNational rejects a file that no longer has the expected structure, so a
// change upstream raises an alert rather than quietly clearing our data.
func ParseNational(body []byte) (National, error) {
	var n National
	if err := json.Unmarshal(body, &n); err != nil {
		return n, fmt.Errorf("%w: %v", ErrSchema, err)
	}
	if n.ReportDate == "" || len(n.ProvinceResults) != 9 {
		return n, fmt.Errorf("%w: reportDate=%q provinces=%d", ErrSchema, n.ReportDate, len(n.ProvinceResults))
	}
	return n, nil
}

// Counters returns each municipality's counters, keyed by municipality ID.
func (n National) Counters() map[int]Counters {
	out := make(map[int]Counters)
	for _, p := range n.ProvinceResults {
		for _, m := range p.MunicipalityResults {
			out[m.MunicipalityID] = m.Counters
		}
	}
	return out
}

var (
	reportDateRe = regexp.MustCompile(`"ReportDate":"([^"]*)"`)
)

// StripReportDates removes the generation timestamps. The IEC rewrites files
// every cycle, so hashing what remains tells us whether the results changed.
func StripReportDates(body []byte) []byte {
	return reportDateRe.ReplaceAll(body, nil)
}

// ReportDate returns the file's first (top-level) report date.
func ReportDate(body []byte) (time.Time, bool) {
	m := reportDateRe.FindSubmatch(body)
	if m == nil {
		return time.Time{}, false
	}
	return ParseTime(string(m[1]))
}

// ParseTime parses IEC timestamps, which carry an offset and 7 fractional digits.
func ParseTime(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, s)
	return t, err == nil
}
