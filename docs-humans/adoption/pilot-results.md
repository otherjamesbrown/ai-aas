# Shared Libraries Pilot Results

Last updated: 2025-11-11

This document captures measurements from the pilot services adopting the shared libraries. Update the tables below as each pilot progresses.

## Measurement Methodology

1. Record a baseline commit for the service **before** integrating shared libraries.
2. After adoption, compare against the baseline using either of the following approaches:
   ```bash
   # Option 1: Git diff stats (all files)
   git diff --stat <baseline_commit> HEAD

   # Option 2: LOC delta for service source only
   git diff <baseline_commit> HEAD -- path/to/service | diffstat -s

   # Option 3: cloc snapshot (requires cloc)
   cloc --csv --out=cloc-before.csv path/to/service --not-match-d='(node_modules|vendor)'
   cloc --csv --out=cloc-after.csv path/to/service --not-match-d='(node_modules|vendor)'
   ```
3. Capture lines removed and added for the service-specific bootstrap code (controllers, config wiring, middleware). Exclude generated artifacts and vendored code.
4. Compute boilerplate reduction:
   ```
   boilerplate_reduction = (baseline_loc - post_adoption_loc) / baseline_loc
   ```
5. Note any shared code additions (e.g., policy definitions) that offset savings.

## Summary Table

Populate once measurements are available.

| Service | Rollout Date | Baseline LOC | Post-Adoption LOC | Boilerplate Reduction | Performance Impact | Notes |
|---------|--------------|--------------|-------------------|-----------------------|--------------------|-------|
| billing-api | _TBD_ | _Pending_ | _Pending_ | _Pending_ | _Pending_ |  |
| content-ingest | _TBD_ | _Pending_ | _Pending_ | _Pending_ | _Pending_ |  |

## Observations

- Compare benchmark artifacts (`shared-library-benchmarks`) in CI before and after adoption.
- Capture telemetry dashboard screenshots highlighting request ID propagation and exporter failure metrics.
- Record any on-call or incident feedback within one week of rollout.

