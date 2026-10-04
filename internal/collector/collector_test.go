package collector

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"gitlab.com/endekasoft/fortiadc-exporter/internal/fortiadc"
)

type fake struct{ failResources, failAll bool }

func (f fake) err(fail bool) error {
	if fail || f.failAll {
		return errors.New("boom")
	}
	return nil
}

func (f fake) GetInfo(context.Context) (*fortiadc.Info, error) {
	return &fortiadc.Info{HostName: "adc", Serial: "SN1", Firmware: "v7", UptimeSeconds: 100}, f.err(false)
}
func (f fake) GetResources(context.Context) (*fortiadc.Resources, error) {
	return &fortiadc.Resources{CPU: 2, RAM: 71, Disk: 1}, f.err(f.failResources)
}
func (f fake) GetDiskError(context.Context) (bool, error) { return true, f.err(false) }
func (f fake) GetLicense(context.Context) (*fortiadc.License, error) {
	return &fortiadc.License{Registered: true, Services: map[string]time.Time{"fmwr": time.Unix(1672704000, 0)}}, f.err(false)
}
func (f fake) GetHA(context.Context) (*fortiadc.HA, error) {
	return &fortiadc.HA{Mode: "standalone", State: "Standalone", ConfigSyncState: "N/A"}, f.err(false)
}
func (f fake) GetVSList(context.Context) ([]fortiadc.VSHealth, error) {
	return []fortiadc.VSHealth{{Name: "vs1", Healthy: true}, {Name: "vs2"}}, f.err(false)
}
func (f fake) GetVSStatus(context.Context) ([]fortiadc.VSCounters, error) {
	return []fortiadc.VSCounters{{Name: "vs1", CurrentSession: 5, TotalSessions: 9, InBytes: 10, OutBytes: 20, RequestErrors: 1, ResponseErrors: 2}}, f.err(false)
}
func (f fake) GetGSLBHosts(context.Context) ([]fortiadc.GSLBHost, error) {
	return []fortiadc.GSLBHost{{Name: "h1", Status: 21, Responses: 553}}, f.err(false)
}

func registry(f fake) *prometheus.Registry {
	r := prometheus.NewRegistry()
	r.MustRegister(New(context.Background(), f, "root"))
	return r
}

func TestMetricValues(t *testing.T) {
	want := `
# HELP fortiadc_cpu_usage_ratio FortiADC CPU usage as a ratio from 0 to 1.
# TYPE fortiadc_cpu_usage_ratio gauge
fortiadc_cpu_usage_ratio 0.02
# HELP fortiadc_virtual_server_bytes_total Bytes handled by a virtual server by direction (in or out), since the device counter was last reset.
# TYPE fortiadc_virtual_server_bytes_total counter
fortiadc_virtual_server_bytes_total{direction="in",vdom="root",virtual_server="vs1"} 10
fortiadc_virtual_server_bytes_total{direction="out",vdom="root",virtual_server="vs1"} 20
# HELP fortiadc_virtual_server_healthy Whether a virtual server is HEALTHY (1) or not (0).
# TYPE fortiadc_virtual_server_healthy gauge
fortiadc_virtual_server_healthy{vdom="root",virtual_server="vs1"} 1
fortiadc_virtual_server_healthy{vdom="root",virtual_server="vs2"} 0
# HELP fortiadc_gslb_host_status Raw FortiADC GSLB host status code, unmapped.
# TYPE fortiadc_gslb_host_status gauge
fortiadc_gslb_host_status{gslb_host="h1",vdom="root"} 21
# HELP fortiadc_license_expiry_timestamp_seconds Unix time when a FortiGuard service licence expires; services with unknown expiry are omitted.
# TYPE fortiadc_license_expiry_timestamp_seconds gauge
fortiadc_license_expiry_timestamp_seconds{service="fmwr"} 1.6727040e+09
`
	err := testutil.GatherAndCompare(registry(fake{}), strings.NewReader(want),
		"fortiadc_cpu_usage_ratio", "fortiadc_virtual_server_bytes_total", "fortiadc_virtual_server_healthy",
		"fortiadc_gslb_host_status", "fortiadc_license_expiry_timestamp_seconds")
	if err != nil {
		t.Error(err)
	}
}

func TestPartialFailure(t *testing.T) {
	want := `
# HELP fortiadc_scrape_endpoint_up Whether the last call to a FortiADC API endpoint succeeded (1) or failed (0).
# TYPE fortiadc_scrape_endpoint_up gauge
fortiadc_scrape_endpoint_up{endpoint="disk"} 1
fortiadc_scrape_endpoint_up{endpoint="gslb_hosts"} 1
fortiadc_scrape_endpoint_up{endpoint="ha"} 1
fortiadc_scrape_endpoint_up{endpoint="info"} 1
fortiadc_scrape_endpoint_up{endpoint="license"} 1
fortiadc_scrape_endpoint_up{endpoint="resources"} 0
fortiadc_scrape_endpoint_up{endpoint="vs_list"} 1
fortiadc_scrape_endpoint_up{endpoint="vs_status"} 1
# HELP fortiadc_up Whether at least one FortiADC API call of this scrape succeeded (1) or all failed (0).
# TYPE fortiadc_up gauge
fortiadc_up 1
`
	r := registry(fake{failResources: true})
	if err := testutil.GatherAndCompare(r, strings.NewReader(want), "fortiadc_scrape_endpoint_up", "fortiadc_up"); err != nil {
		t.Error(err)
	}
	if n, _ := testutil.GatherAndCount(r, "fortiadc_cpu_usage_ratio"); n != 0 {
		t.Errorf("failed endpoint must omit its metrics, got %d cpu series", n)
	}
	if n, _ := testutil.GatherAndCount(r, "fortiadc_info"); n != 1 {
		t.Errorf("other endpoints must still emit, got %d info series", n)
	}
}

func TestAllFailed(t *testing.T) {
	want := `
# HELP fortiadc_up Whether at least one FortiADC API call of this scrape succeeded (1) or all failed (0).
# TYPE fortiadc_up gauge
fortiadc_up 0
`
	if err := testutil.GatherAndCompare(registry(fake{failAll: true}), strings.NewReader(want), "fortiadc_up"); err != nil {
		t.Error(err)
	}
}

func TestLint(t *testing.T) {
	problems, err := testutil.GatherAndLint(registry(fake{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Errorf("%s: %s", p.Metric, p.Text)
	}
}
