// Command loadtest holds many SSE connections open while other clients load
// snapshots, then reports delivery times. Stock k6 has no SSE support, so this
// keeps the test free of custom k6 builds. Each client's IP limit is raised by
// running the target with SSE_MAX_PER_IP set high.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	base := flag.String("base", "http://localhost:8080", "backend URL")
	conns := flag.Int("conns", 2000, "SSE connections to hold")
	readers := flag.Int("readers", 50, "concurrent snapshot readers")
	dur := flag.Duration("for", time.Minute, "how long to run")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *dur)
	defer cancel()

	transport := &http.Transport{
		MaxIdleConnsPerHost: 1000,
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
	}
	client := &http.Client{Transport: transport}

	var connected, failed, frames atomic.Int64
	var wg sync.WaitGroup
	for i := range *conns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Duration(i) * time.Millisecond) // ramp up
			req, _ := http.NewRequestWithContext(ctx, "GET", *base+"/api/v1/stream?topics=national,feed", nil)
			resp, err := client.Do(req)
			if err != nil || resp.StatusCode != 200 {
				if ctx.Err() == nil {
					failed.Add(1)
				}
				return
			}
			defer resp.Body.Close()
			connected.Add(1)
			s := bufio.NewScanner(resp.Body)
			for s.Scan() {
				if strings.HasPrefix(s.Text(), "data:") {
					frames.Add(1)
				}
			}
		}()
	}

	paths := []string{"/national", "/councils", "/map/municipality", "/municipalities/1001", "/feed", "/search?q=cape"}
	var mu sync.Mutex
	var latencies []time.Duration
	var errs atomic.Int64
	for r := range *readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := r; ctx.Err() == nil; n++ {
				start := time.Now()
				req, _ := http.NewRequestWithContext(ctx, "GET", *base+"/api/v1"+paths[n%len(paths)], nil)
				resp, err := client.Do(req)
				if err != nil || resp.StatusCode != 200 {
					if ctx.Err() == nil {
						errs.Add(1)
					}
				} else {
					resp.Body.Close()
					mu.Lock()
					latencies = append(latencies, time.Since(start))
					mu.Unlock()
				}
				time.Sleep(100 * time.Millisecond)
			}
		}()
	}

	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
		case <-t.C:
			fmt.Printf("connected=%d failed=%d frames=%d snapshots=%d\n", connected.Load(), failed.Load(), frames.Load(), len(latencies))
		}
	}
	wg.Wait()
	slices.Sort(latencies)
	at := func(p float64) time.Duration {
		if len(latencies) == 0 {
			return 0
		}
		return latencies[int(float64(len(latencies)-1)*p)]
	}
	fmt.Printf("SSE: %d connected, %d failed, %d events received\nSnapshots: %d ok, %d errors, p50=%v p95=%v p99=%v\n",
		connected.Load(), failed.Load(), frames.Load(), len(latencies), errs.Load(), at(.5), at(.95), at(.99))
}
