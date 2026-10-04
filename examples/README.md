# Examples

| Path | What |
|---|---|
| `docker-compose/` | exporter + Prometheus + Grafana stack with a provisioned dashboard |
| `kubernetes/` | exporter manifests and a Prometheus Operator `ServiceMonitor` |
| `grafana/dashboards/fortiadc-overview.json` | importable dashboard (needs a Prometheus datasource; variables `PROMETHEUS_DS`, `JOB`, `instance`) |

Docker Compose: `cp ../config.yml.example docker-compose/config.yml`, edit it and `prometheus.yml`, then `docker compose -f docker-compose/docker-compose.yml up -d --build`.

Kubernetes: edit `kubernetes/fortiadc-exporter.yaml` (image, target, key), then `kubectl apply -f kubernetes/`. The `ServiceMonitor` needs the Prometheus Operator CRDs.

The dashboard only shows probe metrics (job of the `/probe` scrape). The exporter's own `fortiadc_exporter_*` metrics live under a separate job.
