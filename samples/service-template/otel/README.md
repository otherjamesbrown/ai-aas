# Local Observability Stack

Use this Docker Compose file to run an OpenTelemetry Collector locally for the sample services.

```bash
docker compose up
```

The collector listens on:

- gRPC OTLP: `localhost:4317`
- HTTP OTLP: `localhost:4318`

Logs, metrics, and traces are exported to the collector's stdout by default.

