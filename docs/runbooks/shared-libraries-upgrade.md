# Shared Libraries Upgrade Checklist

This checklist captures the minimum verification required before tagging a new release of the shared Go and TypeScript libraries.

## Preconditions

- Working tree is clean and synced with `main`.
- Release notes have been drafted for the version bump.
- All schema and contract files under `specs/004-shared-libraries/contracts/` are up to date.

## Verification Steps

1. **Run automated checks**
   ```bash
   make shared-check
   scripts/shared/upgrade-verify.sh
   ```
   These commands enforce Go and TypeScript coverage gates, execute contract suites, and rebuild the sample services.
2. **Validate sample services**
   - Start the Go sample service and run `samples/service-template/scripts/smoke.sh`.
   - Start the TypeScript sample service (`npm run dev --prefix samples/service-template/ts`) and run the same smoke script against its port.
3. **Review integration coverage**
   - Confirm `tests/go/integration` and `tests/ts/integration` include new scenarios where applicable.
4. **Update documentation**
   - Ensure `shared/go/README.md`, `shared/ts/README.md`, and this document reflect any new features or breaking changes.
5. **Bump versions**
   - Update `shared/ts/package.json` and any consuming manifests with the new semver.
   - Tag and push the release using `.github/workflows/shared-libraries-release.yml`.

## Release Notes Template

- Summary of changes
- Migration/upgrade steps
- New configuration defaults
- Links to relevant schemas, quickstart, and troubleshooting guides

## Post-Release

- Notify service teams consuming the libraries.
- Monitor CI jobs (`shared-libraries-ci.yml`) for downstream regressions.

