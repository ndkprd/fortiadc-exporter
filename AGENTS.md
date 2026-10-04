# AGENTS.md

Go 1.26 Prometheus exporter for FortiADC.

## Layout
- `cmd/fortiadc-exporter` wiring and flags.
- `internal/config` YAML targets (`fortiadc:` map; `api_key` or `api_key_env`).
- `internal/fortiadc` read-only client, `APITOKEN` header; endpoint names are constants. Fixtures in `testdata/` (masked).
- `internal/collector` per-scrape collector over the `API` interface; descriptors defined once at package level.
- `internal/handler` `/`, `/probe` (per-target mutex, honours `X-Prometheus-Scrape-Timeout-Seconds`).
- `internal/metrics` exporter's own metrics.

## Rules
- Follow Prometheus naming conventions (base units, `_total` counters, bounded labels). A new metric needs a row in `docs/metrics.md` and a test. Const metrics per scrape, no sample timestamps.
- Machine-level calls use `vdom=root`; virtual server and GSLB calls use the configured vdom.
- GET only. Never log or fixture the API key; GSLB `servers` payloads contain a password field, don't capture them.
- Dockerfile: multi-stage, `debian:trixie-slim` runtime.
- Verify: `go vet ./... && go test ./...`, then scrape a live target through `promtool check metrics`.
