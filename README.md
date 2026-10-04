# fortiadc-exporter

Multi-target Prometheus exporter for Fortinet FortiADC. It reads each appliance's REST monitor API on every scrape and exposes system, HA, licence, virtual server and GSLB metrics. Read-only; stateless.

> **Disclaimer:** this is an independent, community project. It is not affiliated with, endorsed by, or sponsored by Fortinet, Inc. FortiADC and Fortinet are trademarks of Fortinet, Inc.

## Run

```sh
cp config.yml.example config.yml        # edit targets
docker compose up -d --build            # or: go run ./cmd/fortiadc-exporter -config config.yml
curl 'localhost:9634/probe?target=adc-01.example.com'
```

| Path | What |
|---|---|
| `/` | landing page |
| `/probe?target=NAME` | scrape one FortiADC (`NAME` = key under `fortiadc:`) |
| `/metrics` | the exporter's own metrics |

Keep the listener internal; there is no authentication on these endpoints.

## FortiADC setup

Create a **REST API administrator** (System → Administrator) with a read-only profile and use its API key. The exporter sends it as the `APITOKEN` header on every call. No login/logout, no session.

Set the key in `config.yml` (`api_key`) or via an environment variable named by `api_key_env`, which overrides it. Keep `config.yml` mode 0600 and out of git.

## Configuration

```yaml
fortiadc:
  adc-01.example.com:
    url: "https://10.0.0.10"
    api_key: "changeme"
    vdom: "root"                 # scopes virtual servers and GSLB; machine data always uses root
    insecure_skip_verify: true
listen_address: ":9634"
```

Prometheus scrape config: see `prometheus.yml.example`. Metric catalog: `docs/metrics.md`.

## Notes

- Tested on FortiADC-200D v7.2.1. `slb/all_vs_status_last_2` (listed in old docs) is 404 there; the exporter uses `slb/all_vs_status`.
- GSLB host status is exported as the raw device code; map it in Grafana.
- Fixtures in `internal/fortiadc/testdata` are masked captures; refresh with `scripts/capture-fixtures.sh` (needs `FADC_URL`, `FADC_API_KEY`).
- Develop: `go test ./...`. Lint a scrape: `curl .../probe?target=x | promtool check metrics`.
