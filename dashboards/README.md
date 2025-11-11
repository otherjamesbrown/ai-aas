# Shared Libraries Dashboards

This directory stores observability assets that accompany the shared libraries feature.

- `grafana/`:
  - `shared-libraries.json` – overview dashboard covering HTTP volume/latency, exporter failures, and database probe health with Prometheus templating.
- `alerts/`:
  - `shared-libraries.yaml` – PrometheusRule definitions for error rate, latency, exporter failures, and database probe degradation, suitable for Alertmanager.

Import the dashboard into Grafana via the JSON definition and apply the alert rules to your Prometheus stack to enable shared libraries SLO monitoring.

