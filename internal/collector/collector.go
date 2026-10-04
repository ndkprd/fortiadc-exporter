// Package collector implements a per-scrape prometheus.Collector that reads a
// FortiADC appliance's monitor API and emits const metrics.
package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"gitlab.com/endekasoft/fortiadc-exporter/internal/fortiadc"
)

// API is the FortiADC data the collector needs. *fortiadc.Client satisfies it.
type API interface {
	GetInfo(ctx context.Context) (*fortiadc.Info, error)
	GetResources(ctx context.Context) (*fortiadc.Resources, error)
	GetDiskError(ctx context.Context) (bool, error)
	GetLicense(ctx context.Context) (*fortiadc.License, error)
	GetHA(ctx context.Context) (*fortiadc.HA, error)
	GetVSList(ctx context.Context) ([]fortiadc.VSHealth, error)
	GetVSStatus(ctx context.Context) ([]fortiadc.VSCounters, error)
	GetGSLBHosts(ctx context.Context) ([]fortiadc.GSLBHost, error)
}

func desc(name, help string, labels ...string) *prometheus.Desc {
	return prometheus.NewDesc("fortiadc_"+name, help, labels, nil)
}

// Descriptors are defined once (PM-LIF-02).
var (
	upDesc         = desc("up", "Whether at least one FortiADC API call of this scrape succeeded (1) or all failed (0).")
	durationDesc   = desc("scrape_duration_seconds", "Duration of the whole FortiADC scrape in seconds.")
	endpointUpDesc = desc("scrape_endpoint_up", "Whether the last call to a FortiADC API endpoint succeeded (1) or failed (0).", "endpoint")

	infoDesc      = desc("info", "FortiADC identity; value is always 1.", "device_name", "serial", "firmware")
	uptimeDesc    = desc("uptime_seconds", "Time since the FortiADC booted, in seconds.")
	cpuDesc       = desc("cpu_usage_ratio", "FortiADC CPU usage as a ratio from 0 to 1.")
	memoryDesc    = desc("memory_usage_ratio", "FortiADC memory usage as a ratio from 0 to 1.")
	diskUsageDesc = desc("disk_usage_ratio", "FortiADC disk usage as a ratio from 0 to 1.")
	diskErrorDesc = desc("disk_error", "Whether the FortiADC reports a disk error (1) or not (0).")

	haInfoDesc  = desc("ha_info", "FortiADC high-availability state; value is always 1.", "mode", "state", "config_sync_state")
	haPeersDesc = desc("ha_peers", "Number of HA peers the FortiADC currently sees.")

	registeredDesc = desc("license_registered", "Whether the FortiADC is registered with FortiGuard (1) or not (0).")
	licenseExpDesc = desc("license_expiry_timestamp_seconds", "Unix time when a FortiGuard service licence expires; services with unknown expiry are omitted.", "service")

	vsHealthyDesc  = desc("virtual_server_healthy", "Whether a virtual server is HEALTHY (1) or not (0).", "vdom", "virtual_server")
	vsSessionsDesc = desc("virtual_server_current_sessions", "Current sessions on a virtual server.", "vdom", "virtual_server")
	vsTotalDesc    = desc("virtual_server_sessions_total", "Sessions handled by a virtual server since the counter was last reset on the device.", "vdom", "virtual_server")
	vsBytesDesc    = desc("virtual_server_bytes_total", "Bytes handled by a virtual server by direction (in or out), since the device counter was last reset.", "vdom", "virtual_server", "direction")
	vsErrorsDesc   = desc("virtual_server_errors_total", "Errors on a virtual server by type (request or response), since the device counter was last reset.", "vdom", "virtual_server", "error_type")

	gslbStatusDesc = desc("gslb_host_status", "Raw FortiADC GSLB host status code, unmapped.", "vdom", "gslb_host")
	gslbRespDesc   = desc("gslb_host_responses_total", "DNS responses served for a GSLB host since the device counter was last reset.", "vdom", "gslb_host")
)

var allDescs = []*prometheus.Desc{
	upDesc, durationDesc, endpointUpDesc, infoDesc, uptimeDesc, cpuDesc, memoryDesc, diskUsageDesc,
	diskErrorDesc, haInfoDesc, haPeersDesc, registeredDesc, licenseExpDesc, vsHealthyDesc,
	vsSessionsDesc, vsTotalDesc, vsBytesDesc, vsErrorsDesc, gslbStatusDesc, gslbRespDesc,
}

// Collector scrapes one FortiADC. Build a new one per probe.
type Collector struct {
	ctx  context.Context
	api  API
	vdom string
}

// New returns a Collector that calls api with ctx and labels vdom-scoped
// metrics with vdom.
func New(ctx context.Context, api API, vdom string) *Collector {
	return &Collector{ctx: ctx, api: api, vdom: vdom}
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range allDescs {
		ch <- d
	}
}

// Collect implements prometheus.Collector. Endpoints are called sequentially;
// a failing endpoint omits only its own metrics.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	gauge := func(d *prometheus.Desc, v float64, lv ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, lv...)
	}
	counter := func(d *prometheus.Desc, v float64, lv ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, v, lv...)
	}

	steps := []struct {
		endpoint string
		run      func() error
	}{
		{fortiadc.EndpointInfo, func() error {
			v, err := c.api.GetInfo(c.ctx)
			if err != nil {
				return err
			}
			gauge(infoDesc, 1, v.HostName, v.Serial, v.Firmware)
			if v.UptimeSeconds > 0 {
				gauge(uptimeDesc, v.UptimeSeconds)
			}
			return nil
		}},
		{fortiadc.EndpointResources, func() error {
			v, err := c.api.GetResources(c.ctx)
			if err != nil {
				return err
			}
			gauge(cpuDesc, v.CPU/100)
			gauge(memoryDesc, v.RAM/100)
			gauge(diskUsageDesc, v.Disk/100)
			return nil
		}},
		{fortiadc.EndpointDisk, func() error {
			bad, err := c.api.GetDiskError(c.ctx)
			if err != nil {
				return err
			}
			gauge(diskErrorDesc, b2f(bad))
			return nil
		}},
		{fortiadc.EndpointLicense, func() error {
			v, err := c.api.GetLicense(c.ctx)
			if err != nil {
				return err
			}
			gauge(registeredDesc, b2f(v.Registered))
			for svc, exp := range v.Services {
				gauge(licenseExpDesc, float64(exp.Unix()), svc)
			}
			return nil
		}},
		{fortiadc.EndpointHA, func() error {
			v, err := c.api.GetHA(c.ctx)
			if err != nil {
				return err
			}
			gauge(haInfoDesc, 1, v.Mode, v.State, v.ConfigSyncState)
			gauge(haPeersDesc, float64(v.Peers))
			return nil
		}},
		{fortiadc.EndpointVSList, func() error {
			v, err := c.api.GetVSList(c.ctx)
			if err != nil {
				return err
			}
			for _, vs := range v {
				gauge(vsHealthyDesc, b2f(vs.Healthy), c.vdom, vs.Name)
			}
			return nil
		}},
		{fortiadc.EndpointVSStatus, func() error {
			v, err := c.api.GetVSStatus(c.ctx)
			if err != nil {
				return err
			}
			for _, vs := range v {
				gauge(vsSessionsDesc, vs.CurrentSession, c.vdom, vs.Name)
				counter(vsTotalDesc, vs.TotalSessions, c.vdom, vs.Name)
				counter(vsBytesDesc, vs.InBytes, c.vdom, vs.Name, "in")
				counter(vsBytesDesc, vs.OutBytes, c.vdom, vs.Name, "out")
				counter(vsErrorsDesc, vs.RequestErrors, c.vdom, vs.Name, "request")
				counter(vsErrorsDesc, vs.ResponseErrors, c.vdom, vs.Name, "response")
			}
			return nil
		}},
		{fortiadc.EndpointGSLBHosts, func() error {
			v, err := c.api.GetGSLBHosts(c.ctx)
			if err != nil {
				return err
			}
			for _, h := range v {
				gauge(gslbStatusDesc, h.Status, c.vdom, h.Name)
				counter(gslbRespDesc, h.Responses, c.vdom, h.Name)
			}
			return nil
		}},
	}

	succeeded := 0
	for _, s := range steps {
		err := s.run()
		if err != nil {
			log.Error().Str("event", "fortiadc_endpoint_failed").Str("endpoint", s.endpoint).Err(err).Msg("FortiADC API call failed")
		} else {
			succeeded++
		}
		gauge(endpointUpDesc, b2f(err == nil), s.endpoint)
	}
	gauge(upDesc, b2f(succeeded > 0))
	gauge(durationDesc, time.Since(start).Seconds())
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
