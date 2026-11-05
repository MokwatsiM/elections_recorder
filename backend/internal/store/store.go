// Package store persists captured IEC files and the state derived from them.
package store

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"lge-dashboard/backend/internal/iec"
	"lge-dashboard/backend/internal/store/db"
	"lge-dashboard/backend/migrations"
)

const pollerLockKey = 1887_2026

type Store struct {
	Pool *pgxpool.Pool
	q    *db.Queries
}

func Open(ctx context.Context, url string) (*Store, error) {
	if err := migrate(ctx, url); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	return &Store{Pool: pool, q: db.New(pool)}, nil
}

func (s *Store) Close() { s.Pool.Close() }

func migrate(ctx context.Context, url string) error {
	conn, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer conn.Close()
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.UpContext(ctx, conn, ".")
}

// Validators returns the stored ETag and Last-Modified for a path.
func (s *Store) Validators(ctx context.Context, path string) (iec.Validators, error) {
	st, err := s.q.GetFetchState(ctx, path)
	if errors.Is(err, pgx.ErrNoRows) {
		return iec.Validators{}, nil
	}
	if err != nil {
		return iec.Validators{}, err
	}
	return iec.Validators{ETag: st.Etag, LastModified: st.LastModified}, nil
}

// Record saves a successful fetch. It adds a new version only when the content
// differs apart from report dates, and reports whether that happened.
func (s *Store) Record(ctx context.Context, res iec.Result) (bool, error) {
	if res.NotModified || res.NotFound {
		return false, s.q.UpsertUnchanged(ctx, db.UpsertUnchangedParams{
			Path: res.Path, Etag: res.Validators.ETag, LastModified: res.Validators.LastModified, LastStatus: int32(res.Status),
		})
	}

	sum := sha256.Sum256(iec.StripReportDates(res.Body))
	st, err := s.q.GetFetchState(ctx, res.Path)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if err == nil && bytes.Equal(st.ContentSha, sum[:]) {
		return false, s.q.UpsertUnchanged(ctx, db.UpsertUnchangedParams{
			Path: res.Path, Etag: res.Validators.ETag, LastModified: res.Validators.LastModified, LastStatus: int32(res.Status),
		})
	}

	gz := res.Gzip
	if gz == nil {
		if gz, err = compress(res.Body); err != nil {
			return false, err
		}
	}
	var reportDate pgtype.Timestamptz
	if t, ok := iec.ReportDate(res.Body); ok {
		reportDate = pgtype.Timestamptz{Time: t, Valid: true}
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.InsertBlob(ctx, db.InsertBlobParams{ContentSha: sum[:], Gz: gz, RawSize: int32(len(res.Body))}); err != nil {
		return false, err
	}
	if _, err := q.InsertVersion(ctx, db.InsertVersionParams{
		Path: res.Path, ContentSha: sum[:], ReportDate: reportDate, UpstreamModified: res.Validators.LastModified,
	}); err != nil {
		return false, err
	}
	if err := q.UpsertChanged(ctx, db.UpsertChangedParams{
		Path: res.Path, Etag: res.Validators.ETag, LastModified: res.Validators.LastModified, ContentSha: sum[:], LastStatus: int32(res.Status),
	}); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// Latest returns the most recent stored body for a path.
func (s *Store) Latest(ctx context.Context, path string) ([]byte, error) {
	gz, err := s.q.LatestBody(ctx, path)
	if err != nil {
		return nil, err
	}
	return decompress(gz)
}

type Stats struct {
	Versions, Paths, StoredBytes int64
	LastChange                   time.Time
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	r, err := s.q.RecorderStats(ctx)
	if err != nil {
		return Stats{}, err
	}
	return Stats{r.Versions, r.Paths, r.StoredBytes, r.LastChange.Time}, nil
}

// HoldPollerLock blocks until this process is the only active poller, then
// returns a context that is cancelled if the lock's connection is lost.
func (s *Store) HoldPollerLock(ctx context.Context) (context.Context, func(), error) {
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return nil, nil, err
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", int64(pollerLockKey)); err != nil {
		conn.Release()
		return nil, nil, err
	}
	lockCtx, cancel := context.WithCancel(ctx)
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-lockCtx.Done():
				return
			case <-t.C:
				if err := conn.Ping(lockCtx); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	release := func() {
		cancel()
		conn.Conn().Close(context.Background())
		conn.Release()
	}
	return lockCtx, release, nil
}

func compress(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(b); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decompress(gz []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	// Stored gzip is the IEC's original, which can begin with a byte-order mark.
	return bytes.TrimPrefix(body, []byte("\xef\xbb\xbf")), nil
}
