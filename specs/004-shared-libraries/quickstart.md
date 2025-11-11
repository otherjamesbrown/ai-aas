# Quickstart: Shared Libraries & Conventions

## Prerequisites
- macOS 14+/Linux with Git, Go 1.21+, Node.js 20 LTS, and Docker (for optional collectors).
- Access to internal package registries (`GOPRIVATE=github.com/ai-aas/*`, `npm config set @ai-aas:registry`).
- Credentials for Vault or secrets provider (if using secure config backends).
- Kubectl context pointed at development cluster plus telemetry collector endpoint.

## 1. Bootstrap Workspace
1. Clone the repository and switch to the feature branch:
   ```bash
   git checkout 004-development
   ```
2. Ensure shared tooling is installed:
   ```bash
   make bootstrap-check
   ```
3. Export environment for private modules:
   ```bash
   export GOPRIVATE="github.com/ai-aas/*"
   npm config set @ai-aas:registry https://npm.ai-aas.dev
   ```

## 2. Install Shared Libraries
### Go Service
```bash
go get github.com/ai-aas/shared-go/config@v0.1.0
go get github.com/ai-aas/shared-go/dataaccess@v0.1.0
go get github.com/ai-aas/shared-go/observability@v0.1.0
go get github.com/ai-aas/shared-go/errors@v0.1.0
```
### TypeScript Service
```bash
npm install @ai-aas/shared@0.1.0
```

## 3. Wire Configuration
1. Copy the sample `.env.example` from `samples/service-template`.
2. Populate required fields:
   - `SERVICE_NAME`
   - `OTEL_EXPORTER_OTLP_ENDPOINT`
   - `VAULT_ADDR` (if using secrets backend)
3. For secure deployment, configure Vault paths via `CONFIG_VAULT_PATH`.

## 4. Integrate Data Access Helpers
1. Import shared helpers:
   - Go: `db, err := dataaccess.OpenSQL(ctx, "postgres", cfg.Database)`
   - TypeScript: `const { pool, probe } = initDataAccess(config.database)`
2. Register standardized health checks:
   - Go: `registry.Register("database", dataaccess.SQLProbe(db))`
   - TypeScript: `registry.register('database', probe)`
3. Configure graceful shutdown:
   - Go: `defer db.Close()`
   - TypeScript: `process.on('SIGTERM', () => pool.end())`

## 5. Initialize Observability
1. Add the bootstrap snippet (Go example):
   ```go
   cfg := config.MustLoad(context.Background())
   obs := observability.MustInit(context.Background(), observability.Config{
     ServiceName: cfg.Service.Name,
     Endpoint:    cfg.Telemetry.Endpoint,
     Protocol:    cfg.Telemetry.Protocol,
     Headers:     cfg.Telemetry.Headers,
     Insecure:    cfg.Telemetry.Insecure,
   })
   defer obs.Shutdown(context.Background())
   ```
2. For TypeScript:
   ```ts
   const cfg = await loadConfig();
   const telemetry = await startTelemetry({
     ...cfg.telemetry,
     serviceName: cfg.service.name,
   });
   process.on('SIGTERM', () => telemetry.shutdown());
   ```
3. Run the collector locally for end-to-end testing:
   ```bash
   docker compose -f samples/service-template/otel/docker-compose.yml up
   ```

## 6. Enable Authorization Middleware
1. Fetch policy bundle metadata:
   ```bash
   make policies-sync
   ```
2. Reference the bundle in service bootstrap:
   - Go: `auth.NewMiddleware(policy.WithBundle("service-template", version))`
   - TypeScript: `createAuthMiddleware({ bundleId: 'service-template', version })`
3. Ensure inbound requests include `X-Actor-Subject` and `X-Actor-Roles` headers so middleware can evaluate policies.

## 7. Verify Telemetry & Health
1. Start the sample service:
   ```bash
   make run-sample SERVICE=service-template
   ```
2. Hit health endpoint:
   ```bash
   curl http://localhost:8080/healthz
   ```
3. Inspect logs, metrics, and traces via Grafana dashboard JSON in `dashboards/grafana/quickstart.json`.

## 8. Run Tests & Lints
```bash
make test-shared          # Go + TS unit tests
make contract-tests       # Contract compatibility suites
make sample-e2e           # End-to-end tests against service template
```

## 9. Upgrade Workflow
1. Review `docs/upgrades/shared-libraries.md` checklist.
2. Execute compatibility smoke tests:
   ```bash
   make upgrade-verify OLD_VERSION=0.1.0 NEW_VERSION=0.2.0
   ```
3. Monitor `alerts/shared-libraries.yaml` for triggered warnings post-upgrade.

## Troubleshooting
- **Telemetry missing**: Verify OTEL endpoint reachable and `OTEL_EXPORTER_OTLP_HEADERS` configured for auth tokens.
- **Policy load failure**: Ensure bundle version exists in `policies/bundles/index.json`; run `make policies-sync` to refresh.
- **Config validation errors**: Run `make config-dump` to inspect resolved configuration and validate against schema.

## Next Steps
- Customize dashboards per service KPIs.
- Integrate release automation hooks to publish new versions through CI pipelines.
- Log decisions and migration notes in `docs/runbooks/shared-libraries.md`.

