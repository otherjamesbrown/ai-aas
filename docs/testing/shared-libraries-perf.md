# Shared Libraries Performance Benchmarks

Last updated: 2025-11-11

## Overview

The shared libraries ship with Go and TypeScript benchmark suites capturing the steady-state overhead introduced by request context middleware, telemetry bootstrap, and configuration loading. These suites run in CI (`Shared Library Benchmarks` job) and publish artifacts to help track regressions over time.

## How to Run Locally

```bash
# Go suite
go test ./tests/go/perf/... -bench=. -benchmem

# TypeScript suite
npm ci --prefix tests/ts/perf
npm run bench --prefix tests/ts/perf
```

Run on a quiet system and repeat if you observe high variance.

## Current Baselines (Apple M1 Max)

| Language | Benchmark                         | Mean Latency | Allocations | Notes |
|----------|-----------------------------------|--------------|-------------|-------|
| Go       | `BenchmarkRequestContextMiddleware` | ~1.46 µs     | 25 allocs   | HTTP handler with context injector |
| Go       | `BenchmarkTelemetryInit`            | ~33 µs       | 135 allocs  | OTLP HTTP exporter bootstrap + shutdown |
| Go       | `BenchmarkConfigLoad`               | ~0.20 µs     | 1 alloc     | Environment parsing only |
| TypeScript | `request context hook`            | ~0.40 µs     | N/A         | Fastify-compatible hook |
| TypeScript | `telemetry init/graceful shutdown` | ~1.5 µs      | N/A         | OTLP gRPC exporter mock |
| TypeScript | `config load`                      | ~2.0 µs      | N/A         | Uses `dotenv` parser |

These values establish the expected order of magnitude. Investigate changes exceeding ~5% without an accompanying rationale.

## CI Outputs

- Workflow: `.github/workflows/shared-libraries-ci.yml` (`Shared Library Benchmarks` job).
- Artifacts: `shared-library-benchmarks` (contains `go-benchmarks.txt` and `benchmarks.json`).
- The TypeScript suite additionally emits `benchmarks.json` in Vitest’s format for historical comparison.

For diffing runs, download the artifact and compare against previous baselines in this document.

## Next Steps

- Consider automating regression checks by parsing the benchmark artifacts and failing builds when thresholds exceed agreed limits.
- Extend coverage with additional scenarios (e.g., database probe execution, degraded telemetry fallback) as needed.

