# Quickstart: Web Portal

**Branch**: `008-web-portal`  
**Date**: 2025-11-11  
**Audience**: Frontend engineers, UX designers, support tooling stakeholders

---

## 1. Prerequisites

1. **Platform context**
   - Portal authenticates against the Identity service (OAuth2/OIDC) and consumes Organization, Billing, and Audit REST APIs exposed through the API Router.
   - Production and staging deployments run on Akamai Linode Kubernetes Engine (LKE) behind the shared Nginx Ingress; CDN caching and WAF rules are already defined in `gitops/`.
2. **Tooling**
   ```bash
   # macOS/Linux/WSL2
   brew install node@20 pnpm             # or use `asdf` with configs/tool-versions.mk
   corepack enable                       # ensures pnpm shim is available

   # verify versions
   node -v   # v20.x
   pnpm -v   # 9.x
   ```
   - Install Playwright browsers once globally:
     ```bash
     pnpm exec playwright install --with-deps
     ```
3. **Access**
   - API credentials: ensure you have client ID/secret for OAuth2 PKCE apps (stored in 1Password) and access to staging billing/identity sandboxes.
   - Telemetry: request API token for the OpenTelemetry collector endpoint (used to verify trace export).

## 2. One-Time Setup

```bash
git clone git@github.com:otherjamesbrown/ai-aas.git
cd ai-aas
pnpm install --filter @ai-aas/portal...    # installs portal + shared/ts deps
pnpm run -r build --filter @ai-aas/shared  # ensure shared design system is built
```

Expected outcome:
- `node_modules/` hydrated for `shared/ts`, `tests/ts/*`, and new `web/portal` workspace.
- `shared/ts/dist/` generated for component reuse.
- `.env.example` copied to `web/portal/.env.local` (script emitted in setup task).

## 3. Daily Workflow

### Start the Dev Server
```bash
cd web/portal
pnpm dev
```
- Serves at `https://localhost:5173` with HTTPS (self-signed cert). Accept certificate in browser once.
- Uses mock adapters if `VITE_API_PROXY` unset; set to staging base URL for integrated testing.

### Run Unit & Integration Tests
```bash
pnpm test              # Vitest (watch mode)
pnpm test:coverage     # Enforces ≥80% line coverage
pnpm lint              # ESLint + TypeScript strict
```

### Execute E2E + Accessibility Checks
```bash
pnpm e2e               # Playwright smoke suite (P1 flows)
pnpm e2e:axe           # Playwright + axe-core accessibility assertions
```
- Fails if WCAG 2.2 AA violations detected; reference `tests/ts/e2e/README.md` for baselines.

### Sync Telemetry Locally
```bash
pnpm exec otel-user login         # bootstrap script to store collector token
pnpm run trace:dev                # runs synthetic trace emission against collector sandbox
```
- Exposes local status badge at http://localhost:5173/status when dev server running.

### Validate Performance Budgets
```bash
pnpm run build
pnpm exec lighthouse http://localhost:4173 --view --preset=portal
```
- Budgets: TTI < 3.0s, Speed Index < 3.5s, CLS < 0.1. Upload report to `/docs/metrics/report.md` when updating dependencies.

## 4. Deployment Notes

- Build artifact produced via `pnpm run build` → `dist/` folder; published to GHCR-backed CDN image (`portal-nginx`).
- Helm chart coordinates asset digest and environment variables via `gitops/templates/portal-values.yaml`.
- `make deploy PORTAL_ENV=staging` triggers ArgoCD sync; ensure PR updates `gitops/clusters/<env>/portal.yaml`.

## 5. Observability & Alerting

- OpenTelemetry exporters configured in `web/portal/src/providers/telemetry.ts`.
- Metrics (Web Vitals, API latency) forwarded to Prometheus via OTLP HTTP.
- Synthetic monitoring: add route definitions to `dashboards/web-portal/README.md`; ensure status badge on landing page hooks into `/status`.
- Audit events accessible under `Usage → Audit` tab; cross-check with `analytics/` dashboards weekly.

## 6. Troubleshooting

| Scenario | Resolution |
|----------|------------|
| OAuth redirect loop on localhost | Set `VITE_AUTH_REDIRECT=http://localhost:5173/auth/callback` and register in staging identity app. |
| 403 shown for admin pages when testing support role | Confirm test account role assignment via Admin CLI or staging portal; RBAC caching clears after 30s or on hard refresh. |
| Usage dashboard empty with data available | Verify billing API reachable; inspect browser devtools for `source=degraded`. Retry once; otherwise follow runbook `docs/runbooks/portal-usage.md` (to be authored). |
| Playwright e2e flake on charts | Increase GPU memory on CI runners via `PWDEBUG=1` to inspect; ensure chart rendering waits for `data-testid="usage-chart-ready"`. |
| Accessibility audit flags shadcn component | Update tokens in `shared/ts` and run `pnpm lint:styles`; most issues resolved by adjusting Tailwind theme. |

---

**Next Steps**: After completing setup, run `/speckit.tasks @008-web-portal` to view the implementation task list and ensure constitution gates remain satisfied through delivery. Update `llms.txt` with portal artifacts once tasks complete.

