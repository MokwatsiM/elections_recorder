package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgtype"

	"lge-dashboard/backend/internal/results"
	"lge-dashboard/backend/internal/store/db"
)

const resultsPathLike = "dashboards/lge/MapsJason/2026/%"

// Hydration returns the latest version of each results file up to cursor.
func (s *Store) Hydration(ctx context.Context, cursor int64) ([]results.Version, error) {
	rows, err := s.q.HydrationVersions(ctx, db.HydrationVersionsParams{ID: cursor, Path: resultsPathLike})
	if err != nil {
		return nil, err
	}
	out := make([]results.Version, 0, len(rows))
	for _, r := range rows {
		body, err := decompress(r.Gz)
		if err != nil {
			return nil, fmt.Errorf("version %d: %w", r.ID, err)
		}
		out = append(out, results.Version{ID: r.ID, Path: r.Path, Body: body, FetchedAt: r.FetchedAt.Time})
	}
	return out, nil
}

// After returns results file versions recorded after cursor, oldest first.
func (s *Store) After(ctx context.Context, cursor int64, limit int) ([]results.Version, error) {
	rows, err := s.q.VersionsAfter(ctx, db.VersionsAfterParams{ID: cursor, Path: resultsPathLike, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]results.Version, 0, len(rows))
	for _, r := range rows {
		body, err := decompress(r.Gz)
		if err != nil {
			return nil, fmt.Errorf("version %d: %w", r.ID, err)
		}
		out = append(out, results.Version{ID: r.ID, Path: r.Path, Body: body, FetchedAt: r.FetchedAt.Time})
	}
	return out, nil
}

func (s *Store) LoadCursor(ctx context.Context) (int64, int64, error) {
	st, err := s.q.LoadEngineState(ctx)
	return st.Cursor, st.Seq, err
}

func ts(c results.Snapshot) pgtype.Timestamptz { return pgtype.Timestamptz{Time: c.At, Valid: true} }

func (s *Store) Commit(ctx context.Context, c results.Commit) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	for _, e := range c.Events {
		if !e.Persisted() {
			continue
		}
		data, err := json.Marshal(e.Data)
		if err != nil {
			return err
		}
		if err := q.InsertEvent(ctx, db.InsertEventParams{
			Seq: e.Seq, Type: e.Type, At: pgtype.Timestamptz{Time: e.At, Valid: true}, Topics: e.Topics, Data: data,
		}); err != nil {
			return err
		}
	}
	for _, snap := range c.Snapshots {
		data, err := json.Marshal(results.TimelinePoint{At: snap.At, Progress: snap.Area.Progress, Parties: snap.Area.Parties})
		if err != nil {
			return err
		}
		if err := q.InsertSnapshot(ctx, db.InsertSnapshotParams{Level: string(snap.Level), AreaID: int32(snap.ID), At: ts(snap), Data: data}); err != nil {
			return err
		}
	}
	if err := q.SaveEngineState(ctx, db.SaveEngineStateParams{Cursor: c.Cursor, Seq: c.Seq}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Feed(ctx context.Context, before int64, limit int) ([]results.Event, error) {
	if before <= 0 {
		before = math.MaxInt64
	}
	rows, err := s.q.FeedBefore(ctx, db.FeedBeforeParams{Seq: before, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]results.Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, results.Event{Seq: r.Seq, Type: r.Type, At: r.At.Time, Data: json.RawMessage(r.Data)})
	}
	return out, nil
}

func (s *Store) Timeline(ctx context.Context, level results.Level, id, limit int) ([]results.TimelinePoint, error) {
	rows, err := s.q.Timeline(ctx, db.TimelineParams{Level: string(level), AreaID: int32(id), Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]results.TimelinePoint, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- { // oldest first for charting
		var p results.TimelinePoint
		if err := json.Unmarshal(rows[i].Data, &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
