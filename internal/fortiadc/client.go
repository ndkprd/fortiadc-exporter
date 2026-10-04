// Package fortiadc is a read-only client for the FortiADC REST monitor API,
// authenticated with a REST API administrator key (header APITOKEN).
package fortiadc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const requestTimeout = 10 * time.Second

// Endpoint names, shared with the collector and the self-metrics.
const (
	EndpointInfo      = "info"
	EndpointResources = "resources"
	EndpointDisk      = "disk"
	EndpointLicense   = "license"
	EndpointHA        = "ha"
	EndpointVSList    = "vs_list"
	EndpointVSStatus  = "vs_status"
	EndpointGSLBHosts = "gslb_hosts"
)

// Endpoints lists every data endpoint, in scrape order.
var Endpoints = []string{
	EndpointInfo, EndpointResources, EndpointDisk, EndpointLicense,
	EndpointHA, EndpointVSList, EndpointVSStatus, EndpointGSLBHosts,
}

// Call results reported to Observe.
const (
	ResultOK             = "ok"
	ResultHTTPError      = "http_error"
	ResultTransportError = "transport_error"
	ResultDecodeError    = "decode_error"
)

// Results lists every result value, for metric initialisation.
var Results = []string{ResultOK, ResultHTTPError, ResultTransportError, ResultDecodeError}

// machineVDOM scopes device-wide data (platform, HA), identical in any vdom.
const machineVDOM = "root"

var paths = map[string]string{
	EndpointInfo:      "/api/platform/info",
	EndpointResources: "/api/platform/resources_usage",
	EndpointDisk:      "/api/platform/disk_status",
	EndpointLicense:   "/api/platform/license_info",
	EndpointHA:        "/api/system_ha/status",
	EndpointVSList:    "/api/all_vs_info/vs_list",
	EndpointVSStatus:  "/api/slb/all_vs_status",
	EndpointGSLBHosts: "/api/global_load_balance_host/statistics",
}

// Client talks to one FortiADC appliance.
type Client struct {
	baseURL string
	apiKey  string
	vdom    string
	http    *http.Client

	// Observe, if set, is called once per API call.
	Observe func(endpoint, result string, d time.Duration)
}

// NewClient builds a Client for the appliance at baseURL. vdom scopes virtual
// server and GSLB data; device-wide data always uses vdom "root".
func NewClient(baseURL, apiKey, vdom string, insecureSkipVerify bool) *Client {
	hc := &http.Client{Timeout: requestTimeout}
	if insecureSkipVerify {
		hc.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, vdom: vdom, http: hc}
}

// Info is the device identity and uptime.
type Info struct {
	HostName      string
	Serial        string
	Firmware      string
	UptimeSeconds float64 // 0 if the uptime string could not be parsed
}

// Resources holds CPU, RAM and disk usage as percentages (0-100).
type Resources struct{ CPU, RAM, Disk float64 }

// License is the FortiGuard registration state and per-service expiry.
type License struct {
	Registered bool
	Services   map[string]time.Time // services with a known expiry date
}

// HA is the high-availability state.
type HA struct {
	Mode, State, ConfigSyncState string
	Peers                        int
}

// VSHealth is one virtual server's health from the virtual server list.
type VSHealth struct {
	Name    string
	Healthy bool
}

// VSCounters is one virtual server's counters from the bulk status call.
type VSCounters struct {
	Name           string
	CurrentSession float64
	TotalSessions  float64
	InBytes        float64
	OutBytes       float64
	RequestErrors  float64
	ResponseErrors float64
}

// GSLBHost is one GSLB host; Status is the raw, unmapped FortiADC code.
type GSLBHost struct {
	Name      string
	Status    float64
	Responses float64
}

// GetInfo fetches device identity and uptime.
func (c *Client) GetInfo(ctx context.Context) (*Info, error) {
	var p struct {
		HostName string `json:"hostName"`
		Serial   string `json:"serialNumber"`
		Firmware string `json:"firmwareVersion"`
		Uptime   string `json:"systemUpTime"`
	}
	if err := c.get(ctx, EndpointInfo, machineVDOM, &p); err != nil {
		return nil, err
	}
	return &Info{p.HostName, p.Serial, p.Firmware, parseUptime(p.Uptime)}, nil
}

// GetResources fetches CPU, RAM and disk usage.
func (c *Client) GetResources(ctx context.Context) (*Resources, error) {
	var p struct{ CPU, RAM, Disk float64 }
	if err := c.get(ctx, EndpointResources, machineVDOM, &p); err != nil {
		return nil, err
	}
	return &Resources{p.CPU, p.RAM, p.Disk}, nil
}

// GetDiskError reports whether the device flags a disk error.
func (c *Client) GetDiskError(ctx context.Context) (bool, error) {
	var p struct {
		DiskError string `json:"disk_error"`
	}
	if err := c.get(ctx, EndpointDisk, machineVDOM, &p); err != nil {
		return false, err
	}
	return p.DiskError != "" && p.DiskError != "0", nil
}

// GetLicense fetches registration state and per-service expiry dates.
func (c *Client) GetLicense(ctx context.Context) (*License, error) {
	var p struct {
		FortiGuard map[string]json.RawMessage `json:"fortiguard"`
	}
	if err := c.get(ctx, EndpointLicense, machineVDOM, &p); err != nil {
		return nil, err
	}
	lic := &License{Services: map[string]time.Time{}}
	for name, raw := range p.FortiGuard {
		if name == "registration" {
			var s string
			_ = json.Unmarshal(raw, &s)
			lic.Registered = s == "Registered"
			continue
		}
		var svc struct {
			Expire string `json:"expire"`
		}
		if json.Unmarshal(raw, &svc) != nil {
			continue // e.g. loginId, a plain string
		}
		if t, err := time.Parse("2006-01-02", svc.Expire); err == nil { // skips "Unknown"
			lic.Services[strings.ToLower(name)] = t
		}
	}
	return lic, nil
}

// GetHA fetches the HA state.
func (c *Client) GetHA(ctx context.Context) (*HA, error) {
	var p struct {
		Mode  string            `json:"mode"`
		State string            `json:"local_state"`
		Sync  string            `json:"cfg_sync_state"`
		Peers []json.RawMessage `json:"peers"`
	}
	if err := c.get(ctx, EndpointHA, machineVDOM, &p); err != nil {
		return nil, err
	}
	return &HA{p.Mode, p.State, p.Sync, len(p.Peers)}, nil
}

// GetVSList fetches virtual server health in the configured vdom.
func (c *Client) GetVSList(ctx context.Context) ([]VSHealth, error) {
	var p []struct {
		Config struct {
			Name         string `json:"mkey"`
			Availability string `json:"availability"`
		} `json:"config"`
	}
	if err := c.get(ctx, EndpointVSList, c.vdom, &p); err != nil {
		return nil, err
	}
	out := make([]VSHealth, len(p))
	for i, v := range p {
		out[i] = VSHealth{v.Config.Name, v.Config.Availability == "HEALTHY"}
	}
	return out, nil
}

// GetVSStatus fetches per-virtual-server counters in the configured vdom.
func (c *Client) GetVSStatus(ctx context.Context) ([]VSCounters, error) {
	var p []*struct {
		Name           string  `json:"mkey"`
		CurrentSession float64 `json:"current_sessions"`
		TotalSessions  float64 `json:"total_sessions"`
		InBytes        float64 `json:"in_bytes"`
		OutBytes       float64 `json:"out_bytes"`
		RequestErrors  float64 `json:"request_errors"`
		ResponseErrors float64 `json:"response_errors"`
	}
	if err := c.get(ctx, EndpointVSStatus, c.vdom, &p); err != nil {
		return nil, err
	}
	var out []VSCounters
	for _, v := range p {
		if v == nil || v.Name == "" { // the device pads the list with nulls
			continue
		}
		out = append(out, VSCounters(*v))
	}
	return out, nil
}

// GetGSLBHosts fetches GSLB host statistics in the configured vdom.
func (c *Client) GetGSLBHosts(ctx context.Context) ([]GSLBHost, error) {
	var p []struct {
		Name      string  `json:"mkey"`
		Status    float64 `json:"hoststatus"`
		Responses float64 `json:"total-response"`
	}
	if err := c.get(ctx, EndpointGSLBHosts, c.vdom, &p); err != nil {
		return nil, err
	}
	out := make([]GSLBHost, 0, len(p))
	for _, h := range p {
		if h.Name != "" {
			out = append(out, GSLBHost(h))
		}
	}
	return out, nil
}

// get performs GET endpoint?vdom=..., unwraps {"payload": ...} into out and
// reports the call to Observe.
func (c *Client) get(ctx context.Context, endpoint, vdom string, out any) error {
	start := time.Now()
	result, err := c.do(ctx, endpoint, vdom, out)
	if c.Observe != nil {
		c.Observe(endpoint, result, time.Since(start))
	}
	return err
}

func (c *Client) do(ctx context.Context, endpoint, vdom string, out any) (string, error) {
	u := c.baseURL + paths[endpoint] + "?vdom=" + url.QueryEscape(vdom)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return ResultTransportError, fmt.Errorf("fortiadc: build request %s: %w", endpoint, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("APITOKEN", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return ResultTransportError, fmt.Errorf("fortiadc: request %s: %w", endpoint, redactKey(err, c.apiKey))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ResultHTTPError, fmt.Errorf("fortiadc: %s: status %d: %s", endpoint, resp.StatusCode, snippet(resp.Body))
	}
	var env struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return ResultDecodeError, fmt.Errorf("fortiadc: decode %s: %w", endpoint, err)
	}
	if err := json.Unmarshal(env.Payload, out); err != nil {
		return ResultDecodeError, fmt.Errorf("fortiadc: decode %s payload: %w", endpoint, err)
	}
	return ResultOK, nil
}

var uptimeRe = regexp.MustCompile(`(\d+)\s*([dhms])`)

// parseUptime converts "715d, 23h, 6m, 5s" to seconds; 0 if nothing matches.
func parseUptime(s string) float64 {
	unit := map[string]float64{"d": 86400, "h": 3600, "m": 60, "s": 1}
	var total float64
	for _, m := range uptimeRe.FindAllStringSubmatch(s, -1) {
		n, _ := strconv.ParseFloat(m[1], 64)
		total += n * unit[m[2]]
	}
	return total
}

// snippet returns a bounded prefix of r for error messages.
func snippet(r io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(r, 512))
	return strings.TrimSpace(string(b))
}

// redactKey keeps the API key out of logged transport errors.
func redactKey(err error, key string) error {
	return errors.New(strings.ReplaceAll(err.Error(), key, "<redacted>"))
}
