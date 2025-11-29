# Platform Backlog

A scratch list of tasks, ideas, and improvements to tackle. Add items here so we don't forget!

---

## High Priority

### Infrastructure - DNS & TLS Setup

**Domain:** `otherjamesbrown.com` (GoDaddy)

- [x] **Set up DNS for development cluster** (2024-11-29)
  - Domain: `*.dev.otherjamesbrown.com` → `172.232.58.222`
  - Created wildcard A record in GoDaddy DNS
  - Services available at: `api.dev.otherjamesbrown.com`, `portal.dev.otherjamesbrown.com`, etc.

- [ ] **Set up DNS for production cluster** (when ready)
  - Domain: `*.otherjamesbrown.com` or `*.prod.otherjamesbrown.com`
  - Point to production LoadBalancer IP

- [x] **Configure valid TLS certificates** (2024-11-29)
  - Implemented Let's Encrypt with cert-manager (free, auto-renewal)
  - Created letsencrypt-prod and letsencrypt-staging ClusterIssuers
  - All ingresses configured for automatic certificate provisioning

- [x] **Update all ingress hostnames** (2024-11-29)
  - Replaced `*.172.232.58.222.nip.io` with `*.dev.otherjamesbrown.com`
  - Updated CLI default config
  - Updated docs/platform/environment-access.md

- [x] **Centralize domain configuration** (2024-11-29 - PR #47)
  - Created `configs/environments/development.yaml` and `production.yaml` as canonical domain source
  - Updated all Helm charts to use `global.domain.base` templating
  - Updated ingress templates to auto-compute hosts from global domain
  - CLI uses `AI_AAS_BASE_DOMAIN` environment variable
  - GitHub workflows use repository variables

### Model Deployment
- [ ] **Deploy vLLM model to development cluster**
  - Register model in model registry
  - Configure vLLM deployment via Knative/Istio
  - Verify inference endpoint via API Router
  - Test with `ai-aas-cli inference` commands

### Pipeline & CI/CD
- [ ] Enable GitHub branch protection on `main` (require PR review, status checks)
- [ ] Add Slack/PagerDuty notifications for workflow failures
- [ ] Add dependency vulnerability scanning (Dependabot/Snyk)

---

## Medium Priority

### Web Portal / UI
- [ ] **Add service health dashboard to Web Portal**
  - Display platform health status similar to CLI `ai-aas-cli status` output
  - Show all services: Admin API, API Router, User-Org, Analytics, Grafana, ArgoCD
  - Real-time status indicators (healthy/slow/unhealthy)
  - Latency display for each service

### CLI Improvements
- [ ] Consider raising "slow" threshold from 1s to 2s for external health checks
- [ ] Add `ai-aas-cli config show` command to display current configuration

### Observability
- [ ] Add post-sync health validation hooks in ArgoCD
- [ ] Implement SLO monitoring (error rate, latency percentiles)
- [ ] Add code coverage enforcement to CI

### Services
- [ ] Complete production deployment for all services (api-router, analytics, admin-api missing)
- [ ] Add rollback testing for service deployments

---

## Low Priority / Future Ideas

### Platform
- [ ] Implement app-of-apps pattern for production (like development)
- [ ] Add changelog generation for service releases
- [ ] Consider external secret management (Vault, External Secrets Operator)

### Developer Experience
- [ ] Add local development with Tilt or Skaffold
- [ ] Create developer onboarding documentation

---

## Completed

- [x] Fix CLI extractBaseDomain to handle multiple service prefixes (2024-11-29)
- [x] Sync develop branch with main (2024-11-29)
- [x] Fix production RBAC - replace wildcards with explicit whitelist (2024-11-29)
- [x] Add post-deployment health check workflow (2024-11-29)
- [x] Add branch sync check workflow (2024-11-29)
- [x] Add circuit breakers and parallel health checks to API Router
- [x] Add self-healing (HPA, replicas, startup probes) to API Router

---

## Notes

- Using real DNS now (`*.dev.otherjamesbrown.com`) - much faster than nip.io
- Internal API Router responds in ~34 microseconds, external latency depends on network path
- Production RBAC was too permissive with wildcards - now fixed
- All TLS certificates managed by cert-manager with Let's Encrypt (auto-renewal)
