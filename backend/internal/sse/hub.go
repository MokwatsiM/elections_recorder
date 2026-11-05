// Package sse fans out events to browsers over Server-Sent Events. Each message
// is encoded once and shared, so the cost per viewer stays low.
package sse

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	clientBuffer  = 128
	heartbeat     = 15 * time.Second // Cloudflare closes idle connections at about 100s
	writeDeadline = 10 * time.Second
)

type Config struct {
	RingSize       int
	MaxPerIP       int
	MaxConnections int
	// TrustProxy reads the client IP from Cloudflare's header. Only enable it
	// behind a proxy, or clients could forge it to escape the per-IP limit.
	TrustProxy bool
}

type message struct {
	seq    int64
	topics []string
	frame  []byte
}

type client struct {
	topics map[string]bool
	ch     chan *message
	once   sync.Once
	gone   chan struct{}
}

func (c *client) wants(m *message) bool {
	if m.topics == nil {
		return true
	}
	for _, t := range m.topics {
		if c.topics[t] {
			return true
		}
	}
	return false
}

func (c *client) evict() { c.once.Do(func() { close(c.gone) }) }

type Hub struct {
	cfg Config
	log *slog.Logger

	mu      sync.RWMutex
	clients map[*client]struct{}
	perIP   map[string]int
	ring    []*message
	head    int
	count   int

	active  atomic.Int64
	evicted atomic.Int64
}

func NewHub(cfg Config, log *slog.Logger) *Hub {
	if cfg.RingSize == 0 {
		cfg.RingSize = 8192
	}
	if cfg.MaxPerIP == 0 {
		cfg.MaxPerIP = 20
	}
	if cfg.MaxConnections == 0 {
		cfg.MaxConnections = 50000
	}
	return &Hub{
		cfg: cfg, log: log,
		clients: make(map[*client]struct{}),
		perIP:   make(map[string]int),
		ring:    make([]*message, cfg.RingSize),
	}
}

func (h *Hub) Active() int64  { return h.active.Load() }
func (h *Hub) Evicted() int64 { return h.evicted.Load() }

func encode(seq int64, typ string, payload []byte) []byte {
	var b strings.Builder
	b.Grow(len(payload) + len(typ) + 32)
	if seq > 0 {
		b.WriteString("id: ")
		b.WriteString(strconv.FormatInt(seq, 10))
		b.WriteByte('\n')
	}
	b.WriteString("event: ")
	b.WriteString(typ)
	b.WriteString("\ndata: ")
	b.Write(payload)
	b.WriteString("\n\n")
	return []byte(b.String())
}

// Publish sends an event to matching clients and keeps it for reconnect replay.
func (h *Hub) Publish(seq int64, typ string, at time.Time, topics []string, data any) {
	payload, err := json.Marshal(struct {
		Seq  int64     `json:"seq"`
		Type string    `json:"type"`
		At   time.Time `json:"at"`
		Data any       `json:"data"`
	}{seq, typ, at, data})
	if err != nil {
		h.log.Error("encoding event", "type", typ, "err", err)
		return
	}
	m := &message{seq: seq, topics: topics, frame: encode(seq, typ, payload)}

	h.mu.Lock()
	h.ring[h.head] = m
	h.head = (h.head + 1) % len(h.ring)
	h.count = min(h.count+1, len(h.ring))
	h.mu.Unlock()
	h.broadcast(m)
}

// Broadcast sends a transient message to everyone, without an ID or replay.
func (h *Hub) Broadcast(typ string, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		h.log.Error("encoding broadcast", "type", typ, "err", err)
		return
	}
	h.broadcast(&message{frame: encode(0, typ, payload)})
}

func (h *Hub) broadcast(m *message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if !c.wants(m) {
			continue
		}
		select {
		case c.ch <- m:
		default:
			// Too slow to keep up. It reconnects and catches up by refetching,
			// which stops one bad connection delaying everyone else.
			c.evict()
			h.evicted.Add(1)
		}
	}
}

// since returns messages after seq. ok is false if the buffer no longer reaches
// back that far, so the client must refetch.
func (h *Hub) since(seq int64, c *client) ([]*message, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.count == 0 {
		return nil, true
	}
	start := (h.head - h.count + len(h.ring)) % len(h.ring)
	oldest := h.ring[start].seq
	if seq < oldest-1 {
		return nil, false
	}
	var out []*message
	for i := 0; i < h.count; i++ {
		m := h.ring[(start+i)%len(h.ring)]
		if m.seq > seq && c.wants(m) {
			out = append(out, m)
		}
	}
	return out, true
}

func (h *Hub) clientIP(r *http.Request) string {
	if h.cfg.TrustProxy {
		if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
			return ip
		}
		if ip, _, ok := strings.Cut(r.Header.Get("X-Forwarded-For"), ","); ok || ip != "" {
			return strings.TrimSpace(ip)
		}
	}
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}
	return r.RemoteAddr
}

func (h *Hub) register(c *client, ip string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.clients) >= h.cfg.MaxConnections || h.perIP[ip] >= h.cfg.MaxPerIP {
		return false
	}
	h.clients[c] = struct{}{}
	h.perIP[ip]++
	h.active.Store(int64(len(h.clients)))
	return true
}

func (h *Hub) unregister(c *client, ip string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	if h.perIP[ip]--; h.perIP[ip] <= 0 {
		delete(h.perIP, ip)
	}
	h.active.Store(int64(len(h.clients)))
}

// ServeHTTP handles GET /stream?topics=national,province:1,muni:1001,feed.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	topics := map[string]bool{}
	for _, t := range strings.Split(r.URL.Query().Get("topics"), ",") {
		if t = strings.TrimSpace(t); t != "" && len(topics) < 32 {
			topics[t] = true
		}
	}
	if len(topics) == 0 {
		topics["national"] = true
	}

	c := &client{topics: topics, ch: make(chan *message, clientBuffer), gone: make(chan struct{})}
	ip := h.clientIP(r)
	if !h.register(c, ip) {
		w.Header().Set("Retry-After", "30")
		http.Error(w, "too many connections", http.StatusTooManyRequests)
		return
	}
	defer h.unregister(c, ip)

	rc := http.NewResponseController(w)
	hdr := w.Header()
	hdr.Set("Content-Type", "text/event-stream")
	hdr.Set("Cache-Control", "no-cache, no-transform")
	hdr.Set("Connection", "keep-alive")
	hdr.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	write := func(b []byte) error {
		rc.SetWriteDeadline(time.Now().Add(writeDeadline))
		if _, err := w.Write(b); err != nil {
			return err
		}
		return rc.Flush()
	}

	if err := write([]byte("retry: 3000\n\n")); err != nil {
		return
	}

	// Register first, then replay, so nothing falls between the two. A client may
	// see an event twice, so it ignores any sequence number it has already applied.
	if last := lastEventID(r); last > 0 {
		replay, ok := h.since(last, c)
		if !ok {
			if err := write(encode(0, "resync", []byte(`{"reason":"too far behind"}`))); err != nil {
				return
			}
		}
		for _, m := range replay {
			if err := write(m.frame); err != nil {
				return
			}
		}
	}

	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-c.gone:
			write(encode(0, "resync", []byte(`{"reason":"slow connection"}`)))
			return
		case m := <-c.ch:
			if err := write(m.frame); err != nil {
				return
			}
		case <-ticker.C:
			if err := write([]byte(": ping\n\n")); err != nil {
				return
			}
		}
	}
}

func lastEventID(r *http.Request) int64 {
	v := r.Header.Get("Last-Event-ID")
	if v == "" {
		v = r.URL.Query().Get("lastEventId")
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// String is used in logs.
func (h *Hub) String() string {
	return fmt.Sprintf("sse.Hub{active=%d evicted=%d}", h.Active(), h.Evicted())
}
