package sse

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newHub(cfg Config) *Hub { return NewHub(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))) }

type conn struct {
	lines chan string
	close func()
}

func connect(t *testing.T, url, lastID string) *conn {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	c := &conn{lines: make(chan string, 256), close: func() { cancel(); resp.Body.Close() }}
	go func() {
		s := bufio.NewScanner(resp.Body)
		for s.Scan() {
			c.lines <- s.Text()
		}
		close(c.lines)
	}()
	t.Cleanup(c.close)
	return c
}

func (c *conn) waitFor(t *testing.T, want string) {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case l, ok := <-c.lines:
			if !ok {
				t.Fatalf("stream closed waiting for %q", want)
			}
			if strings.Contains(l, want) {
				return
			}
		case <-timeout:
			t.Fatalf("timed out waiting for %q", want)
		}
	}
}

func (c *conn) noLine(t *testing.T, unwanted string) {
	t.Helper()
	timeout := time.After(150 * time.Millisecond)
	for {
		select {
		case l, ok := <-c.lines:
			if !ok {
				return
			}
			if strings.Contains(l, unwanted) {
				t.Fatalf("received unexpected %q", l)
			}
		case <-timeout:
			return
		}
	}
}

func waitClients(t *testing.T, h *Hub, n int64) {
	t.Helper()
	for range 300 {
		if h.Active() == n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("active clients = %d, want %d", h.Active(), n)
}

func TestTopicFiltering(t *testing.T) {
	h := newHub(Config{})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := connect(t, srv.URL+"?topics=muni:1", "")
	waitClients(t, h, 1)

	h.Publish(1, "other", time.Now(), []string{"muni:2"}, map[string]int{"v": 1})
	c.noLine(t, "event: other")
	h.Publish(2, "mine", time.Now(), []string{"muni:1"}, map[string]int{"v": 2})
	c.waitFor(t, "id: 2")
	c.waitFor(t, "event: mine")
}

func TestReconnectReplaysMissedEvents(t *testing.T) {
	h := newHub(Config{})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	for i := int64(1); i <= 5; i++ {
		h.Publish(i, "e", time.Now(), []string{"national"}, map[string]int64{"n": i})
	}
	c := connect(t, srv.URL+"?topics=national", "3")
	c.waitFor(t, "id: 4")
	c.waitFor(t, "id: 5")
}

func TestReconnectTooFarBehindGetsResync(t *testing.T) {
	h := newHub(Config{RingSize: 4})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	for i := int64(1); i <= 10; i++ {
		h.Publish(i, "e", time.Now(), []string{"national"}, nil)
	}
	c := connect(t, srv.URL+"?topics=national", "2")
	c.waitFor(t, "event: resync")
}

func TestSlowClientIsEvictedWithoutBlockingOthers(t *testing.T) {
	h := newHub(Config{})
	slow := &client{topics: map[string]bool{"national": true}, ch: make(chan *message, 1), gone: make(chan struct{})}
	if !h.register(slow, "1.1.1.1") {
		t.Fatal("register")
	}
	done := make(chan struct{})
	go func() {
		for i := int64(1); i <= 10; i++ {
			h.Publish(i, "e", time.Now(), []string{"national"}, nil)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publishing blocked on a slow client")
	}
	select {
	case <-slow.gone:
	default:
		t.Fatal("slow client not evicted")
	}
	if h.Evicted() == 0 {
		t.Error("eviction not counted")
	}
}

func TestPerIPLimit(t *testing.T) {
	h := newHub(Config{MaxPerIP: 1})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	connect(t, srv.URL, "")
	waitClients(t, h, 1)
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestForgedProxyHeaderIgnoredUnlessTrusted(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "9.9.9.9:1234"
	r.Header.Set("CF-Connecting-IP", "1.2.3.4")
	if ip := newHub(Config{}).clientIP(r); ip != "9.9.9.9" {
		t.Errorf("untrusted ip = %s", ip)
	}
	if ip := newHub(Config{TrustProxy: true}).clientIP(r); ip != "1.2.3.4" {
		t.Errorf("trusted ip = %s", ip)
	}
}

func TestDisconnectFreesTheSlot(t *testing.T) {
	h := newHub(Config{})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := connect(t, srv.URL, "")
	waitClients(t, h, 1)
	c.close()
	waitClients(t, h, 0)
}
