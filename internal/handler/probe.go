package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.com/endekasoft/fortiadc-exporter/internal/collector"
	"gitlab.com/endekasoft/fortiadc-exporter/internal/metrics"
)

const (
	defaultScrapeTimeout = 30 * time.Second
	timeoutMargin        = 500 * time.Millisecond
)

// Target is a configured FortiADC appliance: its API client and the vdom that
// scopes its virtual server and GSLB metrics.
type Target struct {
	API  collector.API
	VDOM string
	mu   sync.Mutex // serialises probes so retries cannot pile up on the device
}

// Probe serves ProbePath?target=NAME by scraping only that appliance.
type Probe struct{ targets map[string]*Target }

// NewProbe builds a Probe over the configured targets.
func NewProbe(targets map[string]*Target) *Probe { return &Probe{targets: targets} }

// ServeHTTP implements http.Handler.
func (p *Probe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("target")
	if name == "" {
		metrics.CountProbe(metrics.ProbeBadRequest)
		http.Error(w, "target parameter is required", http.StatusBadRequest)
		return
	}
	t, ok := p.targets[name]
	if !ok {
		metrics.CountProbe(metrics.ProbeUnknownTarget)
		http.Error(w, fmt.Sprintf("unknown target %q", name), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), scrapeTimeout(r))
	defer cancel()

	t.mu.Lock()
	defer t.mu.Unlock()

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector.New(ctx, t.API, t.VDOM))
	metrics.CountProbe(metrics.ProbeOK)
	promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

// scrapeTimeout honours Prometheus' X-Prometheus-Scrape-Timeout-Seconds header,
// minus a small margin so the response still reaches the scraper.
func scrapeTimeout(r *http.Request) time.Duration {
	s, err := strconv.ParseFloat(r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"), 64)
	if err != nil || s <= 0 {
		return defaultScrapeTimeout
	}
	if d := time.Duration(s*float64(time.Second)) - timeoutMargin; d > 0 {
		return d
	}
	return time.Duration(s * float64(time.Second))
}
