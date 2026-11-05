package results

import (
	"fmt"
	"time"
)

const (
	EventAreaUpdated   = "area.updated"
	EventWardDeclared  = "ward.declared"
	EventCouncilChange = "council.decided"
	EventLeadChanged   = "lead.changed"
	EventSeatsChanged  = "seats.changed"
	EventMilestone     = "progress.milestone"
)

// Event describes a change. Sequence numbers increase across all event types.
type Event struct {
	Seq    int64     `json:"seq"`
	Type   string    `json:"type"`
	At     time.Time `json:"at"`
	Topics []string  `json:"-"`
	Data   any       `json:"data"`
}

// Persisted reports whether an event belongs in the feed. Area updates are only
// a cue to refresh, so they are kept in the replay buffer and not stored.
func (e Event) Persisted() bool { return e.Type != EventAreaUpdated }

type AreaRef struct {
	Level    Level  `json:"level"`
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ParentID int    `json:"parentId,omitempty"`
	IsMetro  bool   `json:"isMetro,omitempty"`
}

type WardDeclared struct {
	Area      AreaRef `json:"area"`
	WardID    int     `json:"wardId"`
	WardNo    int     `json:"wardNo"`
	PartyID   int     `json:"partyId"`
	PartyName string  `json:"partyName"`
	Candidate string  `json:"candidate,omitempty"`
}

type CouncilChange struct {
	Area    AreaRef       `json:"area"`
	From    CouncilStatus `json:"from"`
	To      CouncilStatus `json:"to"`
	Control int           `json:"controlParty,omitempty"`
	Largest int           `json:"largestParty,omitempty"`
	Tied    bool          `json:"largestTied,omitempty"`
	Parties []Party       `json:"parties"`
	Seats   int           `json:"seatsTotal"`
}

type LeadChange struct {
	Area     AreaRef `json:"area"`
	FromID   int     `json:"fromId"`
	FromName string  `json:"fromName"`
	ToID     int     `json:"toId"`
	ToName   string  `json:"toName"`
	Reported float64 `json:"reportedPct"`
}

type SeatDelta struct {
	PartyID int `json:"partyId"`
	Seats   int `json:"seats"`
	Delta   int `json:"delta"`
}

type SeatsChange struct {
	Area   AreaRef     `json:"area"`
	Deltas []SeatDelta `json:"deltas"`
}

type Milestone struct {
	Percent  int      `json:"percent"`
	Progress Progress `json:"progress"`
}

// minLeadReportedPct stops a lead change in the first handful of districts,
// which is expected and not news, from reaching the feed.
const minLeadReportedPct = 5

// topicsFor keeps each stream small: municipality updates don't reach national
// viewers, who would otherwise receive all 213 every cycle on mobile data.
func topicsFor(a *Area) []string {
	switch a.Level {
	case LevelNational:
		return []string{"national"}
	case LevelProvince:
		return []string{fmt.Sprintf("province:%d", a.ID)}
	default:
		return []string{fmt.Sprintf("province:%d", a.ParentID), fmt.Sprintf("muni:%d", a.ID)}
	}
}

func refOf(a *Area) AreaRef {
	return AreaRef{Level: a.Level, ID: a.ID, Name: a.Name, ParentID: a.ParentID, IsMetro: a.IsMetro}
}

func summarize(a *Area, maxParties int) *Area {
	s := *a
	if len(s.Parties) > maxParties {
		s.Parties = s.Parties[:maxParties]
	}
	return &s
}

// diffNational finds milestones: each time national capture passes a 5% step.
func diffNational(old, cur *Area, at time.Time) []Event {
	if old == nil {
		return nil
	}
	var out []Event
	from, to := int(old.Progress.VDPct())/5, int(cur.Progress.VDPct())/5
	for step := from + 1; step <= to; step++ {
		out = append(out, Event{Type: EventMilestone, At: at, Topics: []string{"feed", "national"},
			Data: Milestone{Percent: step * 5, Progress: cur.Progress}})
	}
	return append(out, leadEvent(old, cur, at)...)
}

// diffMunicipality compares two versions of a municipality. With no earlier
// version there's nothing to compare, so the first sighting is only a baseline.
func diffMunicipality(old, cur *Municipality, at time.Time) []Event {
	if old == nil {
		return nil
	}
	var out []Event
	ref := refOf(&cur.Area)
	topics := append([]string{"feed"}, topicsFor(&cur.Area)...)

	prevWinner := make(map[int]int, len(old.Wards))
	for _, w := range old.Wards {
		prevWinner[w.ID] = w.WinnerID
	}
	for _, w := range cur.Wards {
		if w.WinnerID == 0 || prevWinner[w.ID] == w.WinnerID {
			continue
		}
		name := ""
		for _, p := range w.Parties {
			if p.ID == w.WinnerID {
				name = p.Name
			}
		}
		out = append(out, Event{Type: EventWardDeclared, At: at, Topics: topics, Data: WardDeclared{
			Area: ref, WardID: w.ID, WardNo: w.No, PartyID: w.WinnerID, PartyName: name, Candidate: w.Candidate,
		}})
	}

	if old.Council != nil && cur.Council != nil && old.Council.Status != cur.Council.Status {
		out = append(out, Event{Type: EventCouncilChange, At: at, Topics: topics, Data: CouncilChange{
			Area: ref, From: old.Council.Status, To: cur.Council.Status,
			Control: cur.Council.ControlParty, Largest: cur.Council.LargestParty, Tied: cur.Council.LargestTied,
			Parties: seatLeaders(cur.Parties, 4), Seats: cur.Council.SeatsTotal,
		}})
	}

	out = append(out, leadEvent(&old.Area, &cur.Area, at)...)

	oldPR := make(map[int]Party, len(old.Parties))
	for _, p := range old.Parties {
		oldPR[p.ID] = p
	}
	var deltas []SeatDelta
	prChanged := false
	for _, p := range cur.Parties {
		o := oldPR[p.ID]
		if p.PRSeats != o.PRSeats {
			prChanged = true
		}
		if p.Seats != o.Seats {
			deltas = append(deltas, SeatDelta{PartyID: p.ID, Seats: p.Seats, Delta: p.Seats - o.Seats})
		}
	}
	// Ward seats are already reported one by one, so only proportional allocation
	// (which lands in a batch once a council is complete) gets its own event.
	if prChanged && len(deltas) > 0 {
		out = append(out, Event{Type: EventSeatsChanged, At: at, Topics: topics, Data: SeatsChange{Area: ref, Deltas: deltas}})
	}
	return out
}

func leadEvent(old, cur *Area, at time.Time) []Event {
	if cur.Progress.VDPct() < minLeadReportedPct {
		return nil
	}
	from, to := leader(old.Parties), leader(cur.Parties)
	if from == nil || to == nil || from.ID == to.ID {
		return nil
	}
	return []Event{{Type: EventLeadChanged, At: at, Topics: append([]string{"feed"}, topicsFor(cur)...), Data: LeadChange{
		Area: refOf(cur), FromID: from.ID, FromName: from.Name, ToID: to.ID, ToName: to.Name, Reported: cur.Progress.VDPct(),
	}}}
}

// leader returns the party with the most votes, or nil if nothing has reported.
func leader(ps []Party) *Party {
	var best *Party
	for i := range ps {
		if ps[i].Votes > 0 && (best == nil || ps[i].Votes > best.Votes) {
			best = &ps[i]
		}
	}
	return best
}

func seatLeaders(ps []Party, n int) []Party {
	out := make([]Party, 0, n)
	for _, p := range ps {
		if p.Seats > 0 {
			out = append(out, p)
		}
	}
	sortBySeats(out)
	if len(out) > n {
		out = out[:n]
	}
	return out
}
