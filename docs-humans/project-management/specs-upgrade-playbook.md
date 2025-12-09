# Specs Upgrade Playbook

Goal: bring every feature spec to the same quality bar achieved for `000-project-setup` (clarified spec, populated plan, supporting artifacts, tasks, llms.txt entries).

## Workflow Per Spec

1. **Baseline Review**
   - Read `specs/<feature>/spec.md`.
   - Compare against template (`.specify/templates/spec-template.md`) to identify missing sections or outdated clarifications.
   - Note gaps versus constitution gates (API-first, security, observability, etc.).

2. **Specification Upgrades**
   - Add clarifications (service scope, observability expectations, out-of-scope note).
   - Ensure user stories have clear priorities, independent tests, and Acceptance scenarios covering primary/alternate/exception flows.
   - Expand requirements with measurable NFRs matching the feature domain.
   - Introduce Key Entities, Assumptions, Traceability Matrix.

3. **Quickstart / Support Docs**
   - Mirror the approach in `000-project-setup/quickstart.md`: provide prerequisites, Linode context, remote CI instructions, metrics guidance.
   - Create runbooks/troubleshooting docs if feature needs them.

4. **Plan Generation (`/speckit.plan`)**
   - Run plan command with feature-specific stack/constraints.
   - Verify Constitution Check references active gates (lists compliance, no placeholders).
   - Populate supporting artifacts (`research.md`, `data-model.md`, `contracts/*`, `quickstart.md`).
   - Update agent context via `.specify/scripts/bash/update-agent-context.sh cursor-agent`.

5. **Task Generation (`/speckit.tasks`)**
   - Execute command without extra args.
   - Review `tasks.md` for coverage of all FR/NFRs (benchmarking, rollback, metrics as relevant).
   - Add additional tasks manually if coverage gaps appear.

6. **Cross-Artifact Audit (`/speckit.analyze`)**
   - Run after tasks.
   - Address any findings (coverage gaps, inconsistencies, constitution violations).
   - Re-run until the report shows full coverage with no issues.

7. **Update `llms.txt`**
   - Append links for the feature spec, plan, quickstart, and relevant external documentation (e.g., Linode API endpoints or service-specific APIs).

8. **Documentation Checklist**
   - Ensure supporting docs exist (troubleshooting, metrics policy, service templates) following the patterns created for feature 000.
   - For each new doc or script, run `read_lints` on modified paths.

9. **Repeat for Next Spec**
   - Suggested order (based on dependency hierarchy):
     1. `001-infrastructure`
     2. `002-local-dev-environment`
     3. `003-database-schemas`
     4. `004-shared-libraries`
     5. Service specs (`005` – `009`)
     6. Platform integration specs (`010` – `013`)
   - After each spec, commit or checkpoint before proceeding.

## Tracking Progress

Maintain a simple table (e.g., in `docs/specs-progress.md`) marking completion of:

| Spec | Spec Ready | Plan | Tasks | Analyze Clear | llms.txt Updated | Notes |
|------|------------|------|-------|----------------|------------------|-------|
| 000-project-setup | ✅ | ✅ | ✅ | ✅ | ✅ | baseline |
| 001-infrastructure | ☐ | ☐ | ☐ | ☐ | ☐ | |
| ... | | | | | | |

Use the table to drive work and ensure no feature is left partially upgraded.

## General Tips

- Keep tooling references aligned with `configs/tool-versions.mk`.
- Reuse existing docs/scripts when possible—avoid duplicating content.
- When new automation patterns emerge, update `llms.txt` and relevant playbook sections so future specs inherit improvements.


