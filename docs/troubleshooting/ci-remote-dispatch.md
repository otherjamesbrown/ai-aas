# Issue Report: workflow_dispatch runs not created on feature branch

## Summary
- Dispatching `workflow_dispatch` for `CI Remote` on branch `000-project-setup` returns success in CLI/UI, but no new `workflow_dispatch` run is created in Actions. Only prior `push` events appear.

## Repository Context
- Owner/Repo: `otherjamesbrown/ai-aas`
- Default branch: `main`
- Feature branch: `000-project-setup` (exists on remote; contains workflows and all sources)
- Workflows:
  - `.github/workflows/ci.yml`
  - `.github/workflows/ci-remote.yml` (the one we dispatch)
  - `.github/workflows/reusable/build.yml`

## Authentication & Permissions
- `gh auth status`: logged in; token scopes include `repo`, `workflow` (and others).
- Repo Settings → Actions → General:
  - Workflow permissions changed from “Read repository contents and packages” to “Read and write permissions”.
  - Actions enabled (able to run `push` workflows).

## Reproduction (CLI)

```
# Dispatch (feature branch), various forms tried:
gh workflow run ci-remote.yml --ref 000-project-setup \
  -f ref=000-project-setup \
  -f service=world-service \
  -f notes="smoke test"

# Also tried JSON via stdin:
printf '{"service":"world-service","notes":"smoke test"}' | \
  gh workflow run ci-remote.yml --ref 000-project-setup --json

# List runs (filtered to workflow_dispatch on the branch)
gh run list --workflow ci-remote.yml --event workflow_dispatch --branch 000-project-setup -L 1
# => no runs found

# Only prior push events show up:
gh run list --workflow="ci-remote.yml"
# Example output:
# completed  failure  Initial automation scaffolding  .github/workflows/ci-remote.yml  000-project-setup  push  19195507836  ...
```

## Reproduction (UI)
- Actions → “CI Remote” → “Run workflow”
  - Branch: `000-project-setup`
  - Inputs: `service=world-service`, `notes="smoke test"`, `ref=000-project-setup` (also tried “current”)
- Result: UI confirms, but no new `workflow_dispatch` run appears.

## Verification Checks

```
# Verify branch exists and contains workflow:
git branch -r
# origin/000-project-setup is present

gh api repos/otherjamesbrown/ai-aas/git/trees/000-project-setup?recursive=1 | \
  jq -r '.tree[].path' | grep '^.github/workflows/ci-remote.yml$'
# Path exists in remote tree

# Verify workflows endpoint:
gh api repos/otherjamesbrown/ai-aas/actions/workflows/ci-remote.yml
```

## Observations
- `gh workflow run` prints success: “✓ Created workflow_dispatch event for ci-remote.yml at 000-project-setup”
- Listing `workflow_dispatch` runs on the feature branch returns nothing, repeatedly.
- Listing recent runs shows only prior `push` events (on both `main` and `000-project-setup`).

## What’s Been Tried
- Ensured scripts executable; replaced `readarray`; corrected `gh workflow run` flags (avoid mixing `--json` and `--field`), added polling for run ID.
- Switched the repo’s workflow permissions to Read & Write.
- UI manual dispatch with explicit branch/ref and inputs.

## Hypotheses
1. Repository/organization Actions policy blocks `workflow_dispatch` on non-default branches (or branch rules require approvals).
2. Workflow-level or environment-level restrictions prevent dispatch runs (e.g., required approvals, branch policies, environment protection).
3. The dispatcher is correct, but GitHub is defaulting to `main` or silently rejecting the run on `000-project-setup` due to policy.

## Suggested Next Steps
1. Confirm Actions policy at repo/organization:
   - Repo Settings → Actions → General:
     - Allow all actions and reusable workflows.
     - Workflow permissions: Read and write (done).
     - Disable “Require approval for all outside collaborators” temporarily.
     - Check any branch/tag restrictions for workflows.
   - If applicable, org-level policies that restrict non-default branch runs.

2. Sanity check with a trivial workflow:
   - Add `.github/workflows/ping.yml`:
     ```yaml
     name: Ping
     on: { workflow_dispatch: {} }
     jobs:
       echo:
         runs-on: ubuntu-latest
         steps:
           - run: echo "ping"
     ```
   - Commit/push on `000-project-setup` and try:
     ```
     gh workflow run ping.yml --ref 000-project-setup
     gh run list --workflow ping.yml --event workflow_dispatch --branch 000-project-setup -L 1
     ```
   - If this fails to create a run, it’s definitely policy/permissions.

3. Try default branch:
   ```
   gh workflow run ci-remote.yml --ref main \
     -f ref=main -f service=world-service -f notes="test on main"
   gh run list --workflow ci-remote.yml --event workflow_dispatch --branch main -L 1
   ```
   - If `main` succeeds but the feature branch doesn’t, policy is branch-scoped—adjust settings or dispatch on `main`.

4. Extend CLI polling (if needed):
   - Sometimes the run manifests after several seconds. Poll `gh run list` for `workflow_dispatch` on the branch for ~1–2 minutes.

5. Review audit logs (if available) for denied dispatch events.

## Goal
Enable `workflow_dispatch` of `.github/workflows/ci-remote.yml` on branch `000-project-setup` so `make ci-remote SERVICE=world-service` works reliably.*** End Patch*** }】 ваканс ***!

