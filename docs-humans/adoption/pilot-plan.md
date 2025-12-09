# Shared Libraries Pilot Adoption Plan

Last updated: 2025-11-11

## Goals

- Validate shared libraries in two production-bound services.
- Measure implementation effort and boilerplate reduction (target ≥30%).
- Capture operational feedback (telemetry, auth, data access) ahead of general availability.

## Candidate Services

| Service | Owner Team | Rationale | Target Window |
|---------|------------|-----------|---------------|
| `billing-api` | Finance Platform | Heavy database + telemetry requirements; already using Go | Sprint 48 |
| `content-ingest` | Media Ingest | Node-based, needs standardized auth & tracing | Sprint 49 |

## Engagement Timeline

| Week | Milestone | Artifact |
|------|-----------|----------|
| W0 | Kickoff with service owners | Meeting notes |
| W1 | Align on scope & success criteria | Checklist in this doc |
| W2 | Feature branch integration start | link to service PR |
| W3 | Testing + performance review | Benchmark diffs |
| W4 | Production rollout (canary) | Change ticket |
| W5 | Retrospective & results | Update `pilot-results.md` |

## Responsibilities

- **Shared Libraries Team**
  - Provide implementation support (pairing sessions).
  - Maintain upgrade scripts (`scripts/shared/upgrade-verify.sh`).
  - Track coverage/benchmarks triggered in CI.

- **Pilot Teams**
  - Own service-specific changes and deployments.
  - Report telemetry drift, auth policy gaps, and integration blockers.
  - Participate in retro and share metrics.

- **SRE/Observability**
  - Validate dashboards and alerts in target clusters.
  - Ensure OTLP collectors scaled for added traffic.

## Success Criteria

1. Both services deploy shared libraries to production within planned window without severity-1 incidents.
2. Upgrade verification scripts (`make shared-check`, `upgrade-verify.sh`) run clean in pilot CI pipelines.
3. Boilerplate reduction ≥30% (measure lines removed vs added).
4. Telemetry dashboards show consistent request identifiers and exporter failure counters remain <1 per hour.
5. Runbooks updated with pilot-specific lessons (see `docs/runbooks/shared-libraries.md`).

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| OTLP collector saturation | Missing traces/metrics | Pre-scale collector; monitor `shared_telemetry_export_failures_total`. |
| Policy bundle drift | Authz failures | Freeze bundle versions before rollout; integrate policy sync checks. |
| Migration fatigue | Delayed adoption | Time-box integration; offer office hours. |
| Benchmark regressions | Performance surprises | Compare CI artifacts; flag >5% deviations. |

## Tracking

- Use Jira epic **SHAREDLIB-PI-1** for pilot tasks.
- Link pilot PRs and incidents in the epic.
- Update metrics summary in `docs/adoption/pilot-results.md`.

## Next Steps

1. Confirm pilot service selection with engineering managers.
2. Schedule kickoff sessions (30 minutes each).
3. Prepare example integration PRs referencing quickstart and perf docs.
4. Align on rollout gates (smoke tests, perf checks, observability sign-off).

