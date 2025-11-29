# Troubleshooting: `make check`

## Common Failures

| Symptom | Cause | Fix |
|---------|-------|-----|
| `gofmt` diffs reported | Files not formatted | Run `make fmt` and re-run `make check`. |
| `golangci-lint` errors | Lint rule violations | Read the lint output; adjust code or update `configs/golangci.yml` with justification. |
| `gosec` severity HIGH | Potential security flaw | Address issue or mark as false positive with `#nosec` and justification in code review. |

## Tips

- Ensure local tool versions match `configs/tool-versions.mk`.
- Use `LOG_LEVEL=debug make check` (planned flag) for verbose output.
- To reproduce remote failures locally, run `make ci-local`.

Record persistent or flaky checks in the issue tracker with logs and environment details.

