# Fixtures

Masked captures from a FortiADC-200D v7.2.1 (`scripts/capture-fixtures.sh`, auth: `APITOKEN` header).
Names/IPs/serials are masked, lists cut to 3 entries. Re-run the script to refresh; never commit real values.

Findings (2026-10-04):
- `slb/all_vs_status_last_2` is 404 in every vdom; `slb/all_vs_status` is the bulk per-VS counter source (list, `null` placeholders must be skipped, counters were all 0 so cumulative semantics are unverified).
- `all_vs_info/vs_list` `config.availability` is `HEALTHY` or `DEAD`; its history arrays are empty.
- `platform/license_info` has **no** `licenseStatus`; `fortiguard.registration` is `"Registered"`, every other dict entry is a service with `expire` (`YYYY-MM-DD` or `Unknown`) and `expiry` (int, 0 when Unknown).
- `systemUpTime` looks like `"715d, 23h, 6m, 5s"`.
- `global_load_balance_host/statistics`: `hoststatus` is an undocumented int (20/21/23/24 seen), exported raw. `total-response` is an int counter.
- `global_load_balance_servers` payload contains a `password` field: deliberately not captured.
- `system_ha/status.peers` is empty on a standalone unit; `mode`/`local_state` = `standalone`/`Standalone`.
