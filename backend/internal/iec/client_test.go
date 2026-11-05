package iec

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(Config{BaseURL: srv.URL + "/", RequestsPerS: 1000, HTTPClient: srv.Client()})
}

func TestConditionalRequestReturnsNotModified(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `W/"a"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `W/"a"`)
		w.Write([]byte(`{"ok":true}`))
	})
	ctx := context.Background()
	res, err := c.Fetch(ctx, "f.js", Validators{})
	if err != nil || res.NotModified || res.Validators.ETag != `W/"a"` {
		t.Fatalf("first fetch: %+v, %v", res, err)
	}
	res, err = c.Fetch(ctx, "f.js", res.Validators)
	if err != nil || !res.NotModified || res.Validators.ETag != `W/"a"` {
		t.Fatalf("second fetch: %+v, %v", res, err)
	}
}

func TestGzipBodyIsDecompressedAndBOMRemoved(t *testing.T) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write([]byte("\xef\xbb\xbf{\"a\":1}"))
	w.Close()
	c := testClient(t, func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Encoding", "gzip")
		rw.Write(buf.Bytes())
	})
	res, err := c.Fetch(context.Background(), "f.js", Validators{})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Body) != `{"a":1}` || !bytes.Equal(res.Gzip, buf.Bytes()) {
		t.Fatalf("body=%q gzip=%d bytes", res.Body, len(res.Gzip))
	}
}

func TestNotFoundIsNotAFailure(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(404) })
	res, err := c.Fetch(context.Background(), "f.js", Validators{})
	if err != nil || !res.NotFound {
		t.Fatalf("%+v, %v", res, err)
	}
	if !c.PausedUntil().IsZero() {
		t.Fatal("a 404 must not trigger backoff")
	}
}

func TestChallengePageIsRejectedAndPauses(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("<html>Just a moment...</html>"))
	})
	_, err := c.Fetch(context.Background(), "f.js", Validators{})
	if !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("err = %v, want ErrInvalidBody", err)
	}
	if c.PausedUntil().IsZero() {
		t.Fatal("expected a pause")
	}
}

func TestEmptyFileIsSkippedWithoutPausingOrLosingValidators(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("ETag", "new") })
	old := Validators{ETag: "old"}
	res, err := c.Fetch(context.Background(), "f.js", old)
	if err != nil || !res.NotFound || res.Validators != old {
		t.Fatalf("%+v, %v", res, err)
	}
	if !c.PausedUntil().IsZero() {
		t.Fatal("an empty file must not pause polling")
	}
}

func TestMalformedJSONFailsOnlyThatFile(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(`{"truncated":`)) })
	if _, err := c.Fetch(context.Background(), "f.js", Validators{}); !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("err = %v", err)
	}
	if !c.PausedUntil().IsZero() {
		t.Fatal("one bad file must not pause polling")
	}
}

func TestErrorsBackOffExponentiallyAndHonourRetryAfter(t *testing.T) {
	c := NewClient(Config{})
	c.fail(0)
	first := time.Until(c.pauseUntil)
	if first < minBackoff-time.Second || first > minBackoff*5/4+time.Second {
		t.Fatalf("first backoff = %v", first)
	}
	c.fail(0)
	if second := time.Until(c.pauseUntil); second < 2*minBackoff-time.Second {
		t.Fatalf("second backoff = %v", second)
	}
	for range 20 {
		c.fail(0)
	}
	if capped := time.Until(c.pauseUntil); capped > maxBackoff*5/4+time.Second {
		t.Fatalf("backoff not capped: %v", capped)
	}
	c2 := NewClient(Config{})
	c2.fail(time.Hour)
	if time.Until(c2.pauseUntil) < 59*time.Minute {
		t.Fatal("Retry-After ignored")
	}
}

func TestPauseBlocksRequestsUntilContextEnds(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { calls++; w.Write([]byte(`{}`)) })
	c.fail(0)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := c.Fetch(ctx, "f.js", Validators{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	if calls != 0 {
		t.Fatal("request sent while paused")
	}
}

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	f, err := os.Open("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseRealNationalFile(t *testing.T) {
	n, err := ParseNational(readGolden(t, "national"))
	if err != nil {
		t.Fatal(err)
	}
	counters := n.Counters()
	if len(counters) != 200 {
		t.Errorf("municipalities = %d, want 200", len(counters))
	}
	if got := counters[1001]; got != (Counters{VDsComplete: 77, WardsComplete: 1, SeatsComplete: 1}) {
		t.Errorf("Nelson Mandela Bay counters = %+v", got)
	}
}

func TestParseNationalRejectsChangedStructure(t *testing.T) {
	for _, body := range []string{`{}`, `{"ReportDate":"x","ProvinceResults":[]}`, `[]`} {
		if _, err := ParseNational([]byte(body)); !errors.Is(err, ErrSchema) {
			t.Errorf("%s: err = %v, want ErrSchema", body, err)
		}
	}
}

func TestStripReportDatesIgnoresOnlyTimestamps(t *testing.T) {
	a := `{"ReportDate":"2026-11-05T09:56:15.01+02:00","V":1}`
	b := `{"ReportDate":"2026-11-05T10:02:15.01+02:00","V":1}`
	c := `{"ReportDate":"2026-11-05T10:02:15.01+02:00","V":2}`
	if !bytes.Equal(StripReportDates([]byte(a)), StripReportDates([]byte(b))) {
		t.Error("files differing only by date should match")
	}
	if bytes.Equal(StripReportDates([]byte(a)), StripReportDates([]byte(c))) {
		t.Error("files with different results must not match")
	}
}

func TestReportDateParsesRealFormat(t *testing.T) {
	got, ok := ReportDate(readGolden(t, "national"))
	if !ok || got.Year() != 2026 || got.Month() != time.November {
		t.Fatalf("got %v, %v", got, ok)
	}
}

func TestEmbeddedMunicipalities(t *testing.T) {
	ms := Municipalities()
	if len(ms) != 213 {
		t.Fatalf("got %d municipalities", len(ms))
	}
	if !strings.Contains(Election2026.MunicipalityPath(1001), "2026/EEID1887Munic1001.js") {
		t.Error(Election2026.MunicipalityPath(1001))
	}
}
