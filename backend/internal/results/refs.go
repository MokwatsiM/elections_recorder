package results

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// PartyInfo is IEC party metadata.
type PartyInfo struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Abbr  string `json:"abbr"`
	Color string `json:"color"`
}

// Support is a party's share of the vote in a past election.
type Support struct {
	Year    int     `json:"year"`
	PartyID int     `json:"partyId"`
	Abbr    string  `json:"abbr"`
	Pct     float64 `json:"pct"`
}

// Refs is reference data that doesn't change during the count.
type Refs struct {
	Parties  map[int]PartyInfo
	VDNames  map[int]string
	National []Support
	Province map[int][]Support
	Munic    map[int][]Support
	Ward     map[int]map[int][]Support // municipality → ward → support
}

func parsePartyList(body []byte) (map[int]PartyInfo, error) {
	var raw []struct {
		Key           int    `json:"key"`
		PartyFullName string `json:"PartyFullName"`
		Value         string `json:"value"`
		SWebColour    string `json:"sWebColour"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("party list: %w", err)
	}
	out := make(map[int]PartyInfo, len(raw))
	for _, r := range raw {
		out[r.Key] = PartyInfo{ID: r.Key, Name: r.PartyFullName, Abbr: r.Value, Color: r.SWebColour}
	}
	return out, nil
}

func parseStations(body []byte) (map[int]string, error) {
	var raw []struct {
		VDNumber string `json:"vdnumber"`
		Name     string `json:"name"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("stations: %w", err)
	}
	out := make(map[int]string, len(raw))
	for _, r := range raw {
		if id, err := strconv.Atoi(r.VDNumber); err == nil {
			out[id] = r.Name
		}
	}
	return out, nil
}

type rawSupport struct {
	LYear              int     `json:"lYear"`
	LProvinceID        int     `json:"lProvinceId"`
	LMunicipalityID    int     `json:"lMunicipalityId"`
	LPartyID           int     `json:"lPartyId"`
	LWardID            string  `json:"lWardId"`
	SPartyAbbr         string  `json:"sPartyAbbr"`
	IPercentageSupport float64 `json:"iPercentageSupport"`
}

func (r rawSupport) support() Support {
	return Support{Year: r.LYear, PartyID: r.LPartyID, Abbr: r.SPartyAbbr, Pct: r.IPercentageSupport}
}

func applyHistoryNational(refs *Refs, body []byte) error {
	var raw struct {
		National, Provinces, Municipalities []rawSupport
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("history: %w", err)
	}
	refs.Province = map[int][]Support{}
	refs.Munic = map[int][]Support{}
	for _, r := range raw.National {
		refs.National = append(refs.National, r.support())
	}
	for _, r := range raw.Provinces {
		refs.Province[r.LProvinceID] = append(refs.Province[r.LProvinceID], r.support())
	}
	for _, r := range raw.Municipalities {
		refs.Munic[r.LMunicipalityID] = append(refs.Munic[r.LMunicipalityID], r.support())
	}
	return nil
}

func applyHistoryMunic(refs *Refs, id int, body []byte) error {
	var raw struct{ Wards []rawSupport }
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("ward history %d: %w", id, err)
	}
	wards := map[int][]Support{}
	for _, r := range raw.Wards {
		if w, err := strconv.Atoi(r.LWardID); err == nil {
			wards[w] = append(wards[w], r.support())
		}
	}
	if refs.Ward == nil {
		refs.Ward = map[int]map[int][]Support{}
	}
	refs.Ward[id] = wards
	return nil
}

// LoadRefs builds reference data with get, which returns a stored file's body or
// an error if it isn't available. Whatever is missing stays empty.
func LoadRefs(get func(path string) ([]byte, error), munis []int) *Refs {
	refs := &Refs{Parties: map[int]PartyInfo{}, VDNames: map[int]string{}}
	const mapsJason = "dashboards/lge/MapsJason/"

	if b, err := get(mapsJason + "2016/NewSearchPartyList.js"); err == nil {
		if p, err := parsePartyList(b); err == nil {
			refs.Parties = p
		}
	}
	if b, err := get("ElectionMap/MapData/vs.js"); err == nil {
		if v, err := parseStations(b); err == nil {
			refs.VDNames = v
		}
	}
	if b, err := get(mapsJason + "HistoryYears/HistoryYearsNational.js"); err == nil {
		_ = applyHistoryNational(refs, b)
	}
	for _, id := range munis {
		if b, err := get(fmt.Sprintf("%sHistoryYears/M%d.js", mapsJason, id)); err == nil {
			_ = applyHistoryMunic(refs, id, b)
		}
	}
	return refs
}
