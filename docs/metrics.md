# Metric catalog

Prefix `fortiadc_`. `/probe?target=NAME` emits the first block; `/metrics` serves the exporter's own metrics plus Go/process collectors. Every probe is built from const metrics (no timestamps). A failing endpoint omits only its own metrics and sets `fortiadc_scrape_endpoint_up{endpoint}` to 0; `fortiadc_up` is 0 only when **all** endpoints failed.

`vdom` for `platform/*` and HA calls is always `root`. Virtual server and GSLB calls use the target's configured `vdom` (the `vdom` label).

## /probe

| Metric | Type | Unit | Labels | Example PromQL |
|---|---|---|---|---|
| `fortiadc_up` | gauge | bool | – | `fortiadc_up == 0` |
| `fortiadc_scrape_duration_seconds` | gauge | s | – | `fortiadc_scrape_duration_seconds > 20` |
| `fortiadc_scrape_endpoint_up` | gauge | bool | `endpoint` (info, resources, disk, license, ha, vs_list, vs_status, gslb_hosts) | `fortiadc_scrape_endpoint_up == 0` |
| `fortiadc_info` | gauge (=1) | – | `device_name`, `serial`, `firmware` | `count by (firmware) (fortiadc_info)` |
| `fortiadc_uptime_seconds` | gauge | s | – | `fortiadc_uptime_seconds < 600` |
| `fortiadc_cpu_usage_ratio` | gauge | ratio 0–1 | – | `fortiadc_cpu_usage_ratio > 0.85` |
| `fortiadc_memory_usage_ratio` | gauge | ratio 0–1 | – | `fortiadc_memory_usage_ratio > 0.9` |
| `fortiadc_disk_usage_ratio` | gauge | ratio 0–1 | – | `fortiadc_disk_usage_ratio > 0.8` |
| `fortiadc_disk_error` | gauge | bool | – | `fortiadc_disk_error == 1` |
| `fortiadc_ha_info` | gauge (=1) | – | `mode`, `state`, `config_sync_state` | `changes(fortiadc_ha_info[1h]) > 0` |
| `fortiadc_ha_peers` | gauge | count | – | `fortiadc_ha_peers < 1 and on() fortiadc_ha_info{mode!="standalone"}` |
| `fortiadc_license_registered` | gauge | bool | – | `fortiadc_license_registered == 0` |
| `fortiadc_license_expiry_timestamp_seconds` | gauge | unix s | `service` (lower-case FortiGuard keys, e.g. fmwr; "Unknown" expiry omitted) | `(fortiadc_license_expiry_timestamp_seconds - time()) / 86400 < 30` |
| `fortiadc_virtual_server_healthy` | gauge | bool | `vdom`, `virtual_server` | `fortiadc_virtual_server_healthy == 0` |
| `fortiadc_virtual_server_current_sessions` | gauge | count | `vdom`, `virtual_server` | `sum(fortiadc_virtual_server_current_sessions)` |
| `fortiadc_virtual_server_sessions_total` | counter | sessions | `vdom`, `virtual_server` | `rate(fortiadc_virtual_server_sessions_total[5m])` |
| `fortiadc_virtual_server_bytes_total` | counter | bytes | `vdom`, `virtual_server`, `direction` (in, out) | `sum by (virtual_server) (rate(fortiadc_virtual_server_bytes_total[5m]))` |
| `fortiadc_virtual_server_errors_total` | counter | errors | `vdom`, `virtual_server`, `error_type` (request, response) | `rate(fortiadc_virtual_server_errors_total{error_type="response"}[5m])` |
| `fortiadc_gslb_host_status` | gauge | raw device code, unmapped | `vdom`, `gslb_host` | map values in Grafana (seen: 20, 21, 23, 24; meaning undocumented) |
| `fortiadc_gslb_host_responses_total` | counter | responses | `vdom`, `gslb_host` | `rate(fortiadc_gslb_host_responses_total[5m])` |

Series per target ≈ 31 + 6·N_vs + 2·N_gslb (288 measured with 9 virtual servers, 97 GSLB hosts, 12 licence services). `virtual_server` and `gslb_host` are configured object names; soft cap ~200 and ~500 per target. Virtual server counters were all 0 on the test device, so their cumulative semantics are unverified.

## /metrics (exporter's own)

| Metric | Type | Labels |
|---|---|---|
| `fortiadc_exporter_build_info` | gauge (=1) | `version`, `goversion` |
| `fortiadc_exporter_api_requests_total` | counter | `endpoint`, `result` (ok, http_error, transport_error, decode_error) |
| `fortiadc_exporter_api_request_duration_seconds` | histogram | `endpoint` |
| `fortiadc_exporter_probes_total` | counter | `result` (ok, bad_request, unknown_target) |
