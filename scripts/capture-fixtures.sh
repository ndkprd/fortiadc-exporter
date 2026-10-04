#!/bin/sh
# Capture masked FortiADC monitor responses into internal/fortiadc/testdata.
# Needs FADC_URL, FADC_API_KEY (source .envrc). Read-only GETs only.
set -eu
out=internal/fortiadc/testdata
mkdir -p "$out"

fetch() { # name path
	curl -sk -m 30 -H "APITOKEN: $FADC_API_KEY" -H 'Accept: application/json' \
		"$FADC_URL/api/$2" | python3 scripts/mask.py >"$out/$1.json"
}

fetch info 'platform/info?vdom=root'
fetch resources_usage 'platform/resources_usage?vdom=root'
fetch disk_status 'platform/disk_status?vdom=root'
fetch license_info 'platform/license_info?vdom=root'
fetch ha_status 'system_ha/status?vdom=root'
fetch vs_list "all_vs_info/vs_list?vdom=${FADC_VDOM:-root}"
fetch vs_status "slb/all_vs_status?vdom=${FADC_VDOM:-root}"
fetch gslb_hosts "global_load_balance_host/statistics?vdom=${FADC_VDOM:-root}"
ls -l "$out"
