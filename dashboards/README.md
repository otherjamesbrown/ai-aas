# Dashboards and Alerts

This directory stores observability assets for platform services.

## Shared Libraries

- `grafana/shared-libraries.json` – overview dashboard covering HTTP volume/latency, exporter failures, and database probe health with Prometheus templating.
- `alerts/shared-libraries.yaml` – PrometheusRule definitions for error rate, latency, exporter failures, and database probe degradation, suitable for Alertmanager.

## Analytics Service

- `grafana/analytics-usage.json` – usage and spend dashboard showing request counts, token usage, cost estimates, and freshness indicators. Uses PostgreSQL datasource to query rollup tables directly.
- `alerts/analytics-service.yaml` – PrometheusRule definitions for service availability, freshness lag, deduplication failures, and rollup worker health.

## Usage

Import dashboards into Grafana via the JSON definitions and apply alert rules to your Prometheus stack to enable SLO monitoring for each service.

