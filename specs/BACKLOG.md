# Platform Backlog

A scratch list of tasks, ideas, and improvements to tackle. Add items here so we don't forget!

---

## High Priority

### Infrastructure - DNS & TLS Setup

**Domain:** `otherjamesbrown.com` (GoDaddy)

- [ ] **Set up DNS for development cluster**
  - Domain: `*.dev.otherjamesbrown.com` → `172.232.58.222`
  - Create wildcard A record in GoDaddy DNS
  - Services will be: `api.dev.otherjamesbrown.com`, `portal.dev.otherjamesbrown.com`, etc.

- [ ] **Set up DNS for production cluster** (when ready)
  - Domain: `*.otherjamesbrown.com` or `*.prod.otherjamesbrown.com`
  - Point to production LoadBalancer IP

- [ ] **Configure valid TLS certificates**
  - Option A: Let's Encrypt with cert-manager (free, auto-renewal)
  - Option B: Purchase wildcard cert from GoDaddy
  - Update all ingress resources to use new certs

- [ ] **Update all ingress hostnames**
  - Replace `*.172.232.58.222.nip.io` with `*.dev.otherjamesbrown.com`
  - Update CLI default config
  - Update docs/environment-access.md

### Pipeline & CI/CD
- [ ] Enable GitHub branch protection on `main` (require PR review, status checks)
- [ ] Add Slack/PagerDuty notifications for workflow failures
- [ ] Add dependency vulnerability scanning (Dependabot/Snyk)

---

## Medium Priority

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

- nip.io DNS is slow (3+ seconds sometimes) - root cause of "slow" API health checks
- Internal API Router responds in ~34 microseconds, slowness is external network path
- Production RBAC was too permissive with wildcards - now fixed
