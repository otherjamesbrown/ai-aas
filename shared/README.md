# Shared Libraries Overview

This directory contains the polyglot building blocks that power service bootstrapping across the platform.

- `shared/go` – Go modules for configuration loading, observability, data access helpers, standardized error handling, and authorization middleware.
- `shared/ts` – TypeScript packages providing the equivalent functionality for Node.js services.

## Usage

Refer to the quickstart (`specs/004-shared-libraries/quickstart.md`) for environment setup. In short:

```go
// Go example
cfg := config.MustLoad(ctx)
obs := observability.MustInit(ctx, observability.Config{ServiceName: cfg.Service.Name, Endpoint: cfg.Telemetry.Endpoint})
defer obs.Shutdown(ctx)

policyEngine, _ := auth.LoadPolicyFromFile("policies/service-template/policy.json")
router := chi.NewRouter()
router.Use(observability.RequestContextMiddleware)
router.With(auth.Middleware(policyEngine, auth.HeaderExtractor)).Get("/secure/data", handler)
```

```ts
// TypeScript example
const config = loadConfig();
const telemetry = await startTelemetry({ ...config.telemetry, serviceName: config.service.name });
const policy = await PolicyEngine.fromFile('policies/service-template/policy.json');
const app = Fastify();
app.addHook('onRequest', createRequestContextHook());
app.addHook('preHandler', createAuthMiddleware(policy));
```

## Release Process

1. Run `make shared-check` and `scripts/shared/upgrade-verify.sh`.
2. Follow the [shared upgrade checklist](../docs/upgrades/shared-libraries.md).
3. Trigger the GitHub workflow `.github/workflows/shared-libraries-release.yml` with the desired semver tag.

## Testing Matrix

- Unit tests: `shared/go`, `shared/ts`, and `tests/ts/unit`
- Contract tests: `tests/go/contract`, `tests/ts/contract`
- Integration tests: `tests/go/integration`, `tests/ts/integration`
- Sample services: `samples/service-template/go`, `samples/service-template/ts`
- Performance benchmarks: `tests/go/perf`, `tests/ts/perf` (see `docs/perf/shared-libraries.md`)

