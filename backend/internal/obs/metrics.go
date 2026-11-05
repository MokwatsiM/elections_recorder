// Package obs holds the service's Prometheus metrics.
package obs

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	IECRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "iec_requests_total", Help: "Requests to the IEC by outcome.",
	}, []string{"status"})
	IECBytes = promauto.NewCounter(prometheus.CounterOpts{
		Name: "iec_bytes_total", Help: "Bytes downloaded from the IEC.",
	})
	IECSchemaErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "iec_schema_errors_total", Help: "Files whose structure didn't match expectations.",
	})
	RecorderVersions = promauto.NewCounter(prometheus.CounterOpts{
		Name: "recorder_versions_total", Help: "File versions recorded.",
	})
	PollerSweeps = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "poller_sweeps_total", Help: "Full sweeps by trigger.",
	}, []string{"trigger"})
)
