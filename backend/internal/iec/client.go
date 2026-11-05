// Package iec fetches the IEC's published results files. They are static JSON
// behind Cloudflare, not an API, so the client's job is to be cheap and polite:
// conditional requests, a global rate limit and a shared backoff.
package iec

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	DefaultBaseURL   = "https://results.elections.org.za/"
	DefaultUserAgent = "LGE-Results-Dashboard/0.1 (unofficial results viewer)"

	maxBodyBytes = 512 << 20
	minBackoff   = 30 * time.Second
	maxBackoff   = 10 * time.Minute
)

// ErrInvalidBody means a 200 response was not JSON, for example a Cloudflare
// challenge page. It is treated as an upstream failure and never recorded.
var ErrInvalidBody = errors.New("iec: response body is not valid JSON")

type Config struct {
	BaseURL      string
	UserAgent    string
	RequestsPerS float64
	HTTPClient   *http.Client
}

type Validators struct {
	ETag         string
	LastModified string
}

type Result struct {
	Path        string
	Status      int
	NotModified bool
	NotFound    bool
	Body        []byte // decompressed
	Gzip        []byte // set when upstream sent gzip, reused for storage
	Validators  Validators
}

type Client struct {
	base    string
	ua      string
	http    *http.Client
	limiter *rate.Limiter
	observe func(status string, bytes int)

	mu         sync.Mutex
	failures   int
	pauseUntil time.Time
}

func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}
	if cfg.RequestsPerS <= 0 {
		cfg.RequestsPerS = 4
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 2 * time.Minute}
	}
	return &Client{
		base:    cfg.BaseURL,
		ua:      cfg.UserAgent,
		http:    cfg.HTTPClient,
		limiter: rate.NewLimiter(rate.Limit(cfg.RequestsPerS), 1),
		observe: func(string, int) {},
	}
}

// OnRequest registers a callback used for metrics.
func (c *Client) OnRequest(f func(status string, bytes int)) { c.observe = f }

// Fetch performs one conditional GET. A 404 is a normal outcome: municipalities
// have no file until their first results are captured.
func (c *Client) Fetch(ctx context.Context, path string, v Validators) (Result, error) {
	if err := c.waitForPause(ctx); err != nil {
		return Result{}, err
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return Result{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json, text/javascript, */*")
	req.Header.Set("Accept-Encoding", "gzip")
	if v.ETag != "" {
		req.Header.Set("If-None-Match", v.ETag)
	}
	if v.LastModified != "" {
		req.Header.Set("If-Modified-Since", v.LastModified)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		c.fail(0)
		c.observe("network_error", 0)
		return Result{}, fmt.Errorf("iec: GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	res := Result{
		Path:   path,
		Status: resp.StatusCode,
		Validators: Validators{
			ETag:         resp.Header.Get("ETag"),
			LastModified: resp.Header.Get("Last-Modified"),
		},
	}

	switch {
	case resp.StatusCode == http.StatusNotModified:
		res.NotModified = true
		res.Validators = v
		c.succeed()
		c.observe("304", 0)
		return res, nil
	case resp.StatusCode == http.StatusNotFound:
		res.NotFound = true
		io.Copy(io.Discard, resp.Body)
		c.succeed()
		c.observe("404", 0)
		return res, nil
	case resp.StatusCode != http.StatusOK:
		io.Copy(io.Discard, resp.Body)
		c.fail(parseRetryAfter(resp.Header.Get("Retry-After")))
		c.observe(strconv.Itoa(resp.StatusCode), 0)
		return res, fmt.Errorf("iec: GET %s: status %d", path, resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		c.fail(0)
		c.observe("read_error", 0)
		return res, fmt.Errorf("iec: read %s: %w", path, err)
	}
	if resp.Header.Get("Content-Encoding") == "gzip" {
		res.Gzip = raw
		if res.Body, err = gunzip(raw); err != nil {
			c.fail(0)
			c.observe("read_error", len(raw))
			return res, fmt.Errorf("iec: gunzip %s: %w", path, err)
		}
	} else {
		res.Body = raw
	}
	res.Body = bytes.TrimPrefix(res.Body, []byte("\xef\xbb\xbf"))

	body := bytes.TrimSpace(res.Body)
	if len(body) == 0 {
		// The IEC serves some files empty, and a file could look this way while
		// being rewritten. Keep the previous validators so it's fetched again.
		res.NotFound, res.Body, res.Gzip, res.Validators = true, nil, nil, v
		c.succeed()
		c.observe("empty", 0)
		return res, nil
	}
	if !json.Valid(body) {
		// An HTML page means Cloudflare is blocking us, which affects the whole
		// host. Other bad JSON is a problem with this one file.
		if body[0] == '<' {
			c.fail(0)
		}
		c.observe("invalid_body", len(raw))
		return res, fmt.Errorf("%w: %s", ErrInvalidBody, path)
	}
	c.succeed()
	c.observe("200", len(raw))
	return res, nil
}

// PausedUntil reports when requests resume, or the zero time if not backing off.
func (c *Client) PausedUntil() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().After(c.pauseUntil) {
		return time.Time{}
	}
	return c.pauseUntil
}

func (c *Client) waitForPause(ctx context.Context) error {
	for {
		until := c.PausedUntil()
		if until.IsZero() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Until(until)):
		}
	}
}

func (c *Client) succeed() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures = 0
}

// fail pauses every request, not only the failing one: a block or an outage
// affects the whole host, so retrying other paths would only make it worse.
func (c *Client) fail(retryAfter time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures++
	wait := min(minBackoff<<min(c.failures-1, 8), maxBackoff)
	wait = max(wait+rand.N(wait/4), retryAfter)
	if until := time.Now().Add(wait); until.After(c.pauseUntil) {
		c.pauseUntil = until
	}
}

func parseRetryAfter(s string) time.Duration {
	if secs, err := strconv.Atoi(s); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(s); err == nil {
		return max(time.Until(t), 0)
	}
	return 0
}

func gunzip(b []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, maxBodyBytes))
}
