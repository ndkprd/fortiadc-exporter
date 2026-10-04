package fortiadc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testKey = "test-key"

// newServer serves the masked fixtures, enforcing the APITOKEN header and
// recording the vdom each path was queried with.
func newServer(t *testing.T) (*Client, map[string]string) {
	t.Helper()
	vdoms := map[string]string{}
	byPath := map[string]string{}
	for ep, p := range paths {
		byPath[p] = map[string]string{
			EndpointInfo: "info", EndpointResources: "resources_usage", EndpointDisk: "disk_status",
			EndpointLicense: "license_info", EndpointHA: "ha_status", EndpointVSList: "vs_list",
			EndpointVSStatus: "vs_status", EndpointGSLBHosts: "gslb_hosts",
		}[ep]
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("APITOKEN") != testKey {
			http.Error(w, `{"code":401,"message":"Token parse error."}`, http.StatusUnauthorized)
			return
		}
		name, ok := byPath[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		vdoms[r.URL.Path] = r.URL.Query().Get("vdom")
		b, err := os.ReadFile(filepath.Join("testdata", name+".json"))
		if err != nil {
			t.Error(err)
		}
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, testKey, "development", false), vdoms
}

func TestGetters(t *testing.T) {
	c, vdoms := newServer(t)
	ctx := context.Background()

	info, err := c.GetInfo(ctx)
	if err != nil || info.Firmware != "FortiADC-200D v7.2.1,build0220,230414 (GA)" || info.UptimeSeconds != 715*86400+23*3600+6*60+5 {
		t.Errorf("info = %+v, %v", info, err)
	}
	if r, err := c.GetResources(ctx); err != nil || *r != (Resources{CPU: 2, RAM: 71, Disk: 1}) {
		t.Errorf("resources = %+v, %v", r, err)
	}
	if bad, err := c.GetDiskError(ctx); err != nil || bad {
		t.Errorf("disk error = %v, %v", bad, err)
	}
	lic, err := c.GetLicense(ctx)
	if err != nil || !lic.Registered || lic.Services["fmwr"] != time.Date(2023, 1, 3, 0, 0, 0, 0, time.UTC) {
		t.Errorf("license = %+v, %v", lic, err)
	}
	if _, ok := lic.Services["furl"]; ok { // expire "Unknown" is skipped
		t.Error("service with Unknown expiry must be skipped")
	}
	if _, ok := lic.Services["loginid"]; ok {
		t.Error("loginId is not a service")
	}
	if ha, err := c.GetHA(ctx); err != nil || *ha != (HA{Mode: "standalone", State: "Standalone", ConfigSyncState: "N/A"}) {
		t.Errorf("ha = %+v, %v", ha, err)
	}
	if l, err := c.GetVSList(ctx); err != nil || len(l) != 3 || !l[0].Healthy {
		t.Errorf("vs list = %+v, %v", l, err)
	}
	if s, err := c.GetVSStatus(ctx); err != nil || len(s) != 3 || s[0].CurrentSession != 5 { // null padding skipped
		t.Errorf("vs status = %+v, %v", s, err)
	}
	if h, err := c.GetGSLBHosts(ctx); err != nil || len(h) != 3 || h[0].Status != 21 || h[0].Responses != 553 {
		t.Errorf("gslb = %+v, %v", h, err)
	}

	// machine-level data uses root, vdom-scoped data uses the configured vdom
	for ep, want := range map[string]string{
		EndpointInfo: "root", EndpointHA: "root", EndpointVSList: "development", EndpointGSLBHosts: "development",
	} {
		if got := vdoms[paths[ep]]; got != want {
			t.Errorf("%s vdom = %q, want %q", ep, got, want)
		}
	}
}

func TestErrorsAndObserve(t *testing.T) {
	c, _ := newServer(t)
	var got []string
	c.Observe = func(ep, result string, _ time.Duration) { got = append(got, ep+":"+result) }

	bad := NewClient(c.baseURL, "wrong", "root", false)
	bad.Observe = c.Observe
	if _, err := bad.GetInfo(context.Background()); err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "wrong") {
		t.Errorf("want 401 error without key, got %v", err)
	}
	if _, err := c.GetInfo(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"info:http_error", "info:ok"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("observed %v, want %v", got, want)
	}

	down := NewClient("http://127.0.0.1:1", testKey, "root", false)
	if _, err := down.GetInfo(context.Background()); err == nil {
		t.Error("want transport error")
	}
}

func TestDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("not json")) }))
	defer srv.Close()
	var result string
	c := NewClient(srv.URL, testKey, "root", false)
	c.Observe = func(_, r string, _ time.Duration) { result = r }
	if _, err := c.GetResources(context.Background()); err == nil || result != ResultDecodeError {
		t.Errorf("err=%v result=%q", err, result)
	}
}

func TestParseUptime(t *testing.T) {
	for in, want := range map[string]float64{
		"10d,22h,53m,36s":   10*86400 + 22*3600 + 53*60 + 36,
		"715d, 23h, 6m, 5s": 715*86400 + 23*3600 + 6*60 + 5,
		"garbage":           0,
		"":                  0,
	} {
		if got := parseUptime(in); got != want {
			t.Errorf("parseUptime(%q) = %v, want %v", in, got, want)
		}
	}
}
