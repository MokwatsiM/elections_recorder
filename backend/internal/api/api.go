// Package api serves result snapshots. Responses are cached in memory and at the
// edge, so traffic spikes never reach the engine or the IEC.
package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"lge-dashboard/backend/internal/results"
)

const maxCachedResponses = 5000

// Status reports upstream health to viewers, who should know if figures are old.
type Status struct {
	Ready         bool      `json:"ready"`
	Replay        bool      `json:"replay,omitempty"`
	Seq           int64     `json:"seq"`
	LastIECAnswer time.Time `json:"lastIecAnswer"`
	LastChange    time.Time `json:"lastChange"`
	PausedUntil   time.Time `json:"pausedUntil"`
	Stale         bool      `json:"stale"`
}

type Server struct {
	engine *results.Engine
	sink   results.Sink
	status func(ctx context.Context) Status
	stream http.Handler
	log    *slog.Logger

	mu      sync.RWMutex
	cacheAt int64
	cache   map[string]*cached
}

type cached struct {
	raw, gz []byte
	etag    string
}

func New(e *results.Engine, sink results.Sink, status func(context.Context) Status, stream http.Handler, log *slog.Logger) *Server {
	return &Server{engine: e, sink: sink, status: status, stream: stream, log: log, cache: map[string]*cached{}}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	v1 := "GET /api/v1"
	mux.Handle(v1+"/stream", s.stream)
	mux.HandleFunc(v1+"/status", s.handleStatus)
	mux.HandleFunc(v1+"/national", s.snapshot(func(r *http.Request) (any, int) {
		if v, ok := s.engine.National(partyParam(r)); ok {
			return v, 200
		}
		return nil, 404
	}))
	mux.HandleFunc(v1+"/provinces/{id}", s.snapshot(func(r *http.Request) (any, int) {
		if v, ok := s.engine.Province(intPath(r, "id"), partyParam(r)); ok {
			return v, 200
		}
		return nil, 404
	}))
	mux.HandleFunc(v1+"/municipalities/{id}", s.snapshot(func(r *http.Request) (any, int) {
		if v, ok := s.engine.Municipality(intPath(r, "id"), partyParam(r)); ok {
			return v, 200
		}
		return nil, 404
	}))
	mux.HandleFunc(v1+"/municipalities/{id}/wards/{wardId}", s.snapshot(func(r *http.Request) (any, int) {
		if v, ok := s.engine.Ward(intPath(r, "id"), intPath(r, "wardId"), partyParam(r)); ok {
			return v, 200
		}
		return nil, 404
	}))
	mux.HandleFunc(v1+"/municipalities/{id}/vds/{vd}", s.snapshot(func(r *http.Request) (any, int) {
		if v, ok := s.engine.VD(intPath(r, "id"), intPath(r, "vd")); ok {
			return v, 200
		}
		return nil, 404
	}))
	mux.HandleFunc(v1+"/councils", s.snapshot(func(r *http.Request) (any, int) {
		q := r.URL.Query()
		return s.engine.Councils(q.Get("status"), q.Get("metro") == "true"), 200
	}))
	mux.HandleFunc(v1+"/map/{level}", s.snapshot(func(r *http.Request) (any, int) {
		rows, err := s.engine.MapRows(results.Level(r.PathValue("level")), intQuery(r, "parent"), partyParam(r))
		if err != nil {
			return map[string]string{"error": err.Error()}, 400
		}
		return rows, 200
	}))
	mux.HandleFunc(v1+"/history/{level}/{id}", s.snapshot(func(r *http.Request) (any, int) {
		if v, ok := s.engine.History(results.Level(r.PathValue("level")), intPath(r, "id"), intQuery(r, "municipality")); ok {
			return v, 200
		}
		return nil, 404
	}))
	mux.HandleFunc(v1+"/timeline/{level}/{id}", s.snapshot(func(r *http.Request) (any, int) {
		level := results.Level(r.PathValue("level"))
		if level != results.LevelNational && level != results.LevelProvince && level != results.LevelMunicipality {
			return map[string]string{"error": "timelines run from national to municipality"}, 400
		}
		pts, err := s.sink.Timeline(r.Context(), level, intPath(r, "id"), 500)
		if err != nil {
			s.log.Error("timeline", "err", err)
			return map[string]string{"error": "unavailable"}, 500
		}
		return pts, 200
	}))
	mux.HandleFunc(v1+"/feed", s.snapshot(func(r *http.Request) (any, int) {
		limit := min(max(intQuery(r, "limit"), 1), 100)
		if r.URL.Query().Get("limit") == "" {
			limit = 50
		}
		evs, err := s.sink.Feed(r.Context(), int64(intQuery(r, "before")), limit)
		if err != nil {
			s.log.Error("feed", "err", err)
			return map[string]string{"error": "unavailable"}, 500
		}
		return evs, 200
	}))
	mux.HandleFunc(v1+"/search", s.snapshot(func(r *http.Request) (any, int) {
		q := r.URL.Query().Get("q")
		if len(q) > 80 {
			q = q[:80]
		}
		return s.engine.Search(q, 20), 200
	}))
	mux.HandleFunc(v1+"/parties", s.snapshot(func(*http.Request) (any, int) { return s.engine.PartyList(), 200 }))
	return mux
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	st := s.status(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(st)
}

// snapshot wraps a read with caching keyed on the engine's sequence number, so
// any change invalidates it and identical requests between changes cost nothing.
func (s *Server) snapshot(load func(*http.Request) (any, int)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.engine.Ready() {
			w.Header().Set("Retry-After", "5")
			http.Error(w, `{"error":"starting up"}`, http.StatusServiceUnavailable)
			return
		}
		seq := s.engine.Seq()
		key := r.URL.Path + "?" + r.URL.RawQuery
		c := s.lookup(seq, key)
		if c == nil {
			v, status := load(r)
			if status != http.StatusOK {
				writeError(w, v, status)
				return
			}
			var err error
			if c, err = build(v); err != nil {
				s.log.Error("encoding response", "path", key, "err", err)
				http.Error(w, `{"error":"unavailable"}`, http.StatusInternalServerError)
				return
			}
			s.store(seq, key, c)
		}

		h := w.Header()
		h.Set("Content-Type", "application/json")
		h.Set("Cache-Control", "public, max-age=5, s-maxage=10, stale-while-revalidate=30")
		h.Set("ETag", c.etag)
		h.Add("Vary", "Accept-Encoding")
		if r.Header.Get("If-None-Match") == c.etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			h.Set("Content-Encoding", "gzip")
			w.Write(c.gz)
			return
		}
		w.Write(c.raw)
	}
}

func writeError(w http.ResponseWriter, v any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=5, s-maxage=10")
	w.WriteHeader(status)
	if v == nil {
		v = map[string]string{"error": "no results for this area yet"}
	}
	json.NewEncoder(w).Encode(v)
}

func (s *Server) lookup(seq int64, key string) *cached {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cacheAt != seq {
		return nil
	}
	return s.cache[key]
}

func (s *Server) store(seq int64, key string, c *cached) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cacheAt != seq || len(s.cache) >= maxCachedResponses {
		s.cache, s.cacheAt = map[string]*cached{}, seq
	}
	s.cache[key] = c
}

func build(v any) (*cached, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(raw); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	return &cached{raw: raw, gz: buf.Bytes(), etag: `"` + hex.EncodeToString(sum[:12]) + `"`}, nil
}

func partyParam(r *http.Request) *int {
	v := r.URL.Query().Get("party")
	if v == "" {
		return nil
	}
	id, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &id
}

func intPath(r *http.Request, name string) int {
	id, _ := strconv.Atoi(r.PathValue(name))
	return id
}

func intQuery(r *http.Request, name string) int {
	id, _ := strconv.Atoi(r.URL.Query().Get(name))
	return id
}
