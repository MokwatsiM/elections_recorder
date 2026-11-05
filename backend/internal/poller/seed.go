package poller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"lge-dashboard/backend/internal/iec"
)

// Seed downloads the files that don't change during the count: party metadata,
// boundaries, voting stations, history and past elections. It shares the
// poller's client and rate limit, and skips anything already stored, so it can
// be interrupted and resumed.
func Seed(ctx context.Context, f Fetcher, r Recorder, munis []iec.Municipality, log *slog.Logger) error {
	const mapData = "ElectionMap/MapData/"
	const mapsJason = "dashboards/lge/MapsJason/"

	paths := []string{
		mapsJason + "2016/NewSearchPartyList.js",
		mapsJason + "2016/PartyList.js",
		mapsJason + "2016/ByElections.js",
		mapsJason + "HistoryYears/HistoryYearsNational.js",
		mapData + "vs.js",
		mapData + "2026/nationalProvince.js",
		mapData + "2026/nationalMunic.js",
		mapData + "2026/nationalWard.js",
	}
	for pv := 1; pv <= 9; pv++ {
		paths = append(paths, fmt.Sprintf("%s2026/provinceMunic%d.js", mapData, pv))
	}
	for _, m := range munis {
		paths = append(paths,
			fmt.Sprintf("%s2026/municward%d.js", mapData, m.ID),
			fmt.Sprintf("%s2026/municvd%d.js", mapData, m.ID),
			fmt.Sprintf("%sHistoryYears/M%d.js", mapsJason, m.ID),
		)
	}

	fetchOnce := func(path string) ([]byte, error) {
		if v, err := r.Validators(ctx, path); err != nil {
			return nil, err
		} else if v != (iec.Validators{}) {
			return r.Latest(ctx, path)
		}
		res, err := f.Fetch(ctx, path, iec.Validators{})
		if err != nil {
			return nil, err
		}
		if _, err := r.Record(ctx, res); err != nil {
			return nil, err
		}
		return res.Body, nil
	}

	start := time.Now()
	failed := 0
	run := func(path string) []byte {
		body, err := fetchOnce(path)
		if err != nil && !errors.Is(err, context.Canceled) {
			failed++
			log.Warn("seed: fetch failed", "path", path, "err", err)
		}
		return body
	}

	for _, path := range paths {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		run(path)
	}

	for _, e := range iec.PastElection {
		body := run(e.NationalPath())
		run(e.BigNationalPath())
		for pv := 1; pv <= 9; pv++ {
			run(e.ProvincePath(pv))
		}
		var n struct {
			ProvinceResults []struct {
				MunicipalityResults []struct{ MunicipalityID int }
			}
		}
		if err := json.Unmarshal(body, &n); err != nil {
			log.Warn("seed: national file unreadable", "year", e.Year, "err", err)
			continue
		}
		for _, p := range n.ProvinceResults {
			for _, m := range p.MunicipalityResults {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				run(e.MunicipalityPath(m.MunicipalityID))
			}
		}
		log.Info("seed: election stored", "year", e.Year)
	}

	log.Info("seed finished", "failed", failed, "took", time.Since(start).Round(time.Second))
	if failed > 0 {
		return fmt.Errorf("seed: %d files failed", failed)
	}
	return nil
}
