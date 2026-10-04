// Package metrics holds the exporter's own metrics, served on /metrics.
package metrics

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"gitlab.com/endekasoft/fortiadc-exporter/internal/fortiadc"
)

// Probe results.
const (
	ProbeOK            = "ok"
	ProbeBadRequest    = "bad_request"
	ProbeUnknownTarget = "unknown_target"
)

// Registry is the exporter's own registry (Go runtime, process, and below).
var Registry = prometheus.NewRegistry()

var (
	apiRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "fortiadc_exporter_api_requests_total",
		Help: "FortiADC API calls made by the exporter, by endpoint and result.",
	}, []string{"endpoint", "result"})
	apiDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "fortiadc_exporter_api_request_duration_seconds",
		Help:    "Duration of FortiADC API calls made by the exporter, in seconds.",
		Buckets: []float64{.05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"endpoint"})
	probes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "fortiadc_exporter_probes_total",
		Help: "Probe requests handled by the exporter, by result.",
	}, []string{"result"})
)

// Register wires the metrics into Registry and initialises every known label
// combination to 0. version is the exporter build version.
func Register(version string) {
	buildInfo := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "fortiadc_exporter_build_info",
		Help:        "Exporter build information; value is always 1.",
		ConstLabels: prometheus.Labels{"version": version, "goversion": runtime.Version()},
	})
	buildInfo.Set(1)
	Registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		buildInfo, apiRequests, apiDuration, probes)

	for _, ep := range fortiadc.Endpoints {
		apiDuration.WithLabelValues(ep)
		for _, r := range fortiadc.Results {
			apiRequests.WithLabelValues(ep, r)
		}
	}
	for _, r := range []string{ProbeOK, ProbeBadRequest, ProbeUnknownTarget} {
		probes.WithLabelValues(r)
	}
}

// ObserveAPI records one FortiADC API call; it matches fortiadc.Client.Observe.
func ObserveAPI(endpoint, result string, d time.Duration) {
	apiRequests.WithLabelValues(endpoint, result).Inc()
	apiDuration.WithLabelValues(endpoint).Observe(d.Seconds())
}

// CountProbe records one handled probe request.
func CountProbe(result string) { probes.WithLabelValues(result).Inc() }
