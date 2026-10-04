package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitlab.com/endekasoft/fortiadc-exporter/internal/fortiadc"
)

// down is a collector.API whose every call fails.
type down struct{}

var errDown = errors.New("down")

func (down) GetInfo(context.Context) (*fortiadc.Info, error)           { return nil, errDown }
func (down) GetResources(context.Context) (*fortiadc.Resources, error) { return nil, errDown }
func (down) GetDiskError(context.Context) (bool, error)                { return false, errDown }
func (down) GetLicense(context.Context) (*fortiadc.License, error)     { return nil, errDown }
func (down) GetHA(context.Context) (*fortiadc.HA, error)               { return nil, errDown }
func (down) GetVSList(context.Context) ([]fortiadc.VSHealth, error)    { return nil, errDown }
func (down) GetVSStatus(context.Context) ([]fortiadc.VSCounters, error) {
	return nil, errDown
}
func (down) GetGSLBHosts(context.Context) ([]fortiadc.GSLBHost, error) { return nil, errDown }

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestProbe(t *testing.T) {
	p := NewProbe(map[string]*Target{"adc": {API: down{}, VDOM: "root"}})

	if rec := get(p, "/probe"); rec.Code != http.StatusBadRequest {
		t.Errorf("missing target: %d", rec.Code)
	}
	if rec := get(p, "/probe?target=nope"); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown target: %d", rec.Code)
	}
	rec := get(p, "/probe?target=adc")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "fortiadc_up 0") {
		t.Errorf("probe: %d %s", rec.Code, rec.Body)
	}
}

func TestIndex(t *testing.T) {
	rec := get(http.HandlerFunc(Index), "/")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `href="/metrics"`) {
		t.Errorf("index: %d %s", rec.Code, rec.Body)
	}
	if rec := get(http.HandlerFunc(Index), "/other"); rec.Code != http.StatusNotFound {
		t.Errorf("other path: %d", rec.Code)
	}
}

func TestScrapeTimeout(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if got := scrapeTimeout(req); got != defaultScrapeTimeout {
		t.Errorf("default = %v", got)
	}
	req.Header.Set("X-Prometheus-Scrape-Timeout-Seconds", "10")
	if got := scrapeTimeout(req); got != 10*time.Second-timeoutMargin {
		t.Errorf("header = %v", got)
	}
}
