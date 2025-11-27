# Customizing Service Automation

Every service Makefile should include the shared template:

```make
include ../../templates/service.mk
```

## Common Overrides

| Variable | Purpose | Example |
|----------|---------|---------|
| `SERVICE_NAME` | Human-readable service identifier | `SERVICE_NAME := billing-service` |
| `SERVICE_BUILD_FLAGS` | Extra flags passed to `go build` | `SERVICE_BUILD_FLAGS := -tags prod` |
| `SERVICE_TEST_FLAGS` | Extra flags passed to `go test` | `SERVICE_TEST_FLAGS := -run TestBilling` |
| `SERVICE_PRE_BUILD` / `SERVICE_POST_BUILD` | Commands executed before/after build | `SERVICE_PRE_BUILD = @echo "Preparing assets"` |
| `SERVICE_PRE_TEST` / `SERVICE_POST_TEST` | Commands executed before/after tests | `SERVICE_POST_TEST = @echo "Tests complete"` |

Define overrides **above** the `include` line. Avoid redefining shared targets; instead, hook into template-provided variables.

Refer to `services/_template/Makefile` for full extension points.

