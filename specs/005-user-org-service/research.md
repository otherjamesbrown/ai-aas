# Research: User & Organization Service

**Branch**: `005-user-org-service-upgrade`  
**Date**: 2025-11-11  
**Spec**: `/specs/005-user-org-service/spec.md`

## Research Questions & Answers

| Topic | Decision | Rationale | Alternatives Considered |
|-------|----------|-----------|--------------------------|
| Authentication & MFA stack | Adopt `ory/fosite` for OAuth2/OIDC flows with native email/password, TOTP, and WebAuthn second factors | Fosite offers battle-tested OAuth2/MFA primitives, extensible grant handlers, and integrates cleanly with our Go stack; supports future federation needs | Building custom auth flows (high risk), Keycloak (heavier operational footprint), Auth0 (external dependency, higher cost) |
| SSO Federation | Integrate external IdPs via `go-oidc` with SAML bridge through `crewjam/saml` adapter | Allows initial OIDC support and optional SAML for enterprises without maintaining heavy IdP infrastructure; both libraries compatible with fosite | Embedding Keycloak as broker (extra ops), forcing OIDC-only (excludes legacy SAML orgs) |
| Policy / Authorization Engine | Use embedded Open Policy Agent (`opa` SDK) with Rego policies stored in `policies/` bundle, executed via cached policy decision points | OPA gives declarative policy definition, versioned with code, evaluated quickly in-process with bundle updates; meets auditability and drift detection needs | Google Zanzibar-inspired graph service (complex, high build effort), Casbin (less expressive for budget/time constraints) |
| Budget enforcement telemetry | Consume usage signals from billing service via Kafka topic `billing.usage`, aggregate in Redis + Postgres, and evaluate against policy thresholds | Kafka already provisioned (per infra), supports high throughput, allows replay for drift analysis; caching reduces load on Postgres | Polling billing APIs (laggy), direct DB reads of usage service (tight coupling) |
| Declarative reconciliation | Track Git state with `go-git` + webhook triggers, queue reconcile jobs in Kafka topic `identity.reconcile`, process via dedicated worker | Keeps reconciliation logic inside service, allows pause/resume, works offline with cached repo, and leverages existing Kafka cluster for retries/backoff | Outsourcing to ArgoCD/Flux (less control over domain-specific validations), direct GitHub API polling only (limited resilience) |
| Audit & compliance pipeline | Emit structured events to Kafka `audit.identity`, forward to Loki + object storage via shared ingestion pipeline, sign exports with Ed25519 | Kafka ensures ordered, durable logs; Loki integration satisfies observability; signed exports deliver tamper evidence mandated by compliance | Writing directly to S3 (harder to stream/alert), relying on stdout logs (incomplete), proprietary SIEM (cost, vendor lock) |
| Secrets & key storage | Store API key secrets in Hashicorp Vault (managed by infra spec) via Transit engine; cache only hashed fingerprints in Postgres | Vault Transit provides envelope encryption + revocation hooks without storing plaintext; fingerprints allow lookups and audit diffing | Custom AES key management (reinvents wheel), storing plaintext in DB (violates security gate) |

## Outstanding Follow-ups

- Align Vault Transit namespaces and policies with infrastructure team to ensure least-privilege access for service workloads.  
- Confirm Kafka retention policies for `billing.usage`, `identity.reconcile`, and `audit.identity` topics meet 400-day audit requirement or document archival strategy.  
- Define SSO onboarding playbook with security team (metadata exchange, certificate rotation) before GA.  
- Coordinate with shared notification service for invite/alert templates and localization coverage (EN + ES).  
- Evaluate hardware security module (HSM) integration for Ed25519 key storage if compliance review demands hardware-backed signing.

