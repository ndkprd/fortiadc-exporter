package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRegisterInitialisesAndObserves(t *testing.T) {
	Register("test")

	// every known combination exists at 0 before first use
	if n, _ := testutil.GatherAndCount(Registry, "fortiadc_exporter_api_requests_total"); n != 8*4 {
		t.Errorf("api_requests series = %d, want 32", n)
	}
	if n, _ := testutil.GatherAndCount(Registry, "fortiadc_exporter_probes_total"); n != 3 {
		t.Errorf("probes series = %d, want 3", n)
	}

	ObserveAPI("info", "ok", 120*time.Millisecond)
	CountProbe(ProbeOK)
	if got := testutil.ToFloat64(apiRequests.WithLabelValues("info", "ok")); got != 1 {
		t.Errorf("info/ok = %v, want 1", got)
	}
	if got := testutil.ToFloat64(probes.WithLabelValues(ProbeOK)); got != 1 {
		t.Errorf("probes ok = %v, want 1", got)
	}

	problems, err := testutil.GatherAndLint(Registry)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Errorf("%s: %s", p.Metric, p.Text)
	}
}
