// Package handler serves the exporter's HTTP endpoints: a static landing page
// at "/", a per-target probe at ProbePath, and the exporter's own metrics at
// MetricsPath.
package handler

import "net/http"

// Fixed HTTP paths (blackbox_exporter convention): ProbePath scrapes the
// target named by the "target" query parameter, MetricsPath exposes the
// exporter's own metrics.
const (
	ProbePath   = "/probe"
	MetricsPath = "/metrics"
)

const indexPage = `<!DOCTYPE html>
<html>
<head><title>FortiADC Exporter</title></head>
<body>
<h1>FortiADC Exporter</h1>
<p><a href="/metrics">Metrics</a> (exporter's own)</p>
<p>Probe a target: <code>/probe?target=NAME</code>, where NAME is a key under <code>fortiadc:</code> in config.yml.</p>
</body>
</html>
`

// Index serves the static landing page at "/".
func Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexPage))
}
