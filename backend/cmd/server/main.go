package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"lge-dashboard/backend/internal/api"
	"lge-dashboard/backend/internal/iec"
	"lge-dashboard/backend/internal/obs"
	"lge-dashboard/backend/internal/poller"
	"lge-dashboard/backend/internal/results"
	"lge-dashboard/backend/internal/sse"
	"lge-dashboard/backend/internal/store"
)

// staleAfter is well past the IEC's six-minute cycle, so an ordinary gap between
// batches doesn't alarm viewers.
const staleAfter = 15 * time.Minute

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, log); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	st, err := store.Open(ctx, env("DATABASE_URL", "postgres://lge:lge@localhost:5432/lge?sslmode=disable"))
	if err != nil {
		return err
	}
	defer st.Close()

	maxPerIP, _ := strconv.Atoi(env("SSE_MAX_PER_IP", "20"))
	hub := sse.NewHub(sse.Config{TrustProxy: env("TRUST_PROXY", "false") == "true", MaxPerIP: maxPerIP}, log)
	promauto.NewGaugeFunc(prometheus.GaugeOpts{Name: "sse_clients", Help: "Connected SSE clients."},
		func() float64 { return float64(hub.Active()) })
	promauto.NewCounterFunc(prometheus.CounterOpts{Name: "sse_evicted_total", Help: "Clients dropped for being too slow."},
		func() float64 { return float64(hub.Evicted()) })

	munis := iec.Municipalities()
	munisIDs := make([]int, len(munis))
	for i, m := range munis {
		munisIDs[i] = m.ID
	}

	replaySpeed, _ := strconv.ParseFloat(env("REPLAY_SPEED", "0"), 64)
	replay := replaySpeed > 0

	var sink results.Sink = st
	if replay {
		sink = results.NewMemorySink()
	}
	engine := results.NewEngine(st, sink, hub, log)
	engine.OnSchema = func(string, error) { obs.IECSchemaErrors.Inc() }

	seedDone := make(chan struct{})
	go loadRefs(ctx, st, engine, munisIDs, seedDone, log)

	var client *iec.Client
	var pollr *poller.Poller
	if replay {
		log.Info("replay mode: reading recorded versions, not contacting the IEC", "speed", replaySpeed)
		engine.Pace = func(ctx context.Context, gap time.Duration) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(min(time.Duration(float64(gap)/replaySpeed), 5*time.Second)):
				return nil
			}
		}
	} else {
		rps, _ := strconv.ParseFloat(env("IEC_RPS", "4"), 64)
		client = iec.NewClient(iec.Config{
			BaseURL:      env("IEC_BASE_URL", iec.DefaultBaseURL),
			UserAgent:    env("IEC_USER_AGENT", iec.DefaultUserAgent),
			RequestsPerS: rps,
		})
		client.OnRequest(func(status string, n int) {
			obs.IECRequests.WithLabelValues(status).Inc()
			obs.IECBytes.Add(float64(n))
		})
		pollr = poller.New(poller.Config{
			Election:     iec.Election2026,
			Municipality: munis,
			Sentinel:     1001,
			OnChange: func(string) {
				obs.RecorderVersions.Inc()
				engine.Notify()
			},
			OnSchema: func(string, error) { obs.IECSchemaErrors.Inc() },
			OnCycle:  func(kind string) { obs.PollerSweeps.WithLabelValues(kind).Inc() },
		}, client, st, log)
	}

	status := func(ctx context.Context) api.Status {
		s := api.Status{Ready: engine.Ready(), Replay: replay, Seq: engine.Seq()}
		if replay {
			return s
		}
		if stats, err := st.Stats(ctx); err == nil {
			s.LastChange = stats.LastChange
		}
		s.LastIECAnswer = pollr.LastSuccess()
		s.PausedUntil = client.PausedUntil()
		s.Stale = !s.PausedUntil.IsZero() || time.Since(s.LastIECAnswer) > staleAfter
		return s
	}

	srv := api.New(engine, sink, status, hub, log)
	go serve(ctx, env("LISTEN_ADDR", ":8080"), srv.Handler(), engine, log)

	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				hub.Broadcast("status", status(sctx))
				cancel()
			}
		}
	}()

	if replay {
		close(seedDone)
		return engine.Run(ctx)
	}

	lockCtx, release, err := st.HoldPollerLock(ctx)
	if err != nil {
		return err
	}
	defer release()
	log.Info("poller lock held, starting")

	go func() {
		if err := engine.Run(lockCtx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("engine stopped", "err", err)
		}
	}()
	if env("SEED_ON_START", "true") == "true" {
		go func() {
			defer close(seedDone)
			if err := poller.Seed(lockCtx, client, st, munis, log); err != nil && !errors.Is(err, context.Canceled) {
				log.Warn("seed incomplete, restart to retry the missing files", "err", err)
			}
		}()
	} else {
		close(seedDone)
	}
	return pollr.Run(lockCtx)
}

// loadRefs refreshes reference data as the seed downloads it, then loads it a
// final time. Files the IEC never published simply stay empty.
func loadRefs(ctx context.Context, st *store.Store, e *results.Engine, munis []int, seedDone <-chan struct{}, log *slog.Logger) {
	get := func(path string) ([]byte, error) { return st.Latest(ctx, path) }
	load := func() *results.Refs {
		refs := results.LoadRefs(get, munis)
		e.SetRefs(refs)
		return refs
	}
	load()
	for {
		select {
		case <-ctx.Done():
			return
		case <-seedDone:
			refs := load()
			log.Info("reference data loaded", "parties", len(refs.Parties), "stations", len(refs.VDNames), "munic_history", len(refs.Ward))
			return
		case <-time.After(time.Minute):
			load()
		}
	}
}

func serve(ctx context.Context, addr string, apiHandler http.Handler, e *results.Engine, log *slog.Logger) {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !e.Ready() {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ready\n"))
	})
	mux.Handle("/api/", apiHandler)
	// ServeFile handles the Range requests PMTiles reads individual tiles with.
	tiles := env("TILES_FILE", "/data/tiles/sa.pmtiles")
	mux.HandleFunc("GET /tiles/sa.pmtiles", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFile(w, r, tiles)
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 2 * time.Minute}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("http server", "err", err)
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
