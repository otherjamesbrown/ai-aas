# Customizing Service Makefiles

---
last_updated: 2025-12-08
document_type: guide
---

## Overview

Every service Makefile should include the shared template for consistent build automation.

## Basic Setup

```make
SERVICE_NAME := my-service
SERVICE_VERSION := 1.0.0

include ../../templates/service.mk
```

## Common Overrides

Define overrides **above** the `include` line:

| Variable | Purpose | Example |
|----------|---------|---------|
| `SERVICE_NAME` | Human-readable service identifier | `SERVICE_NAME := billing-service` |
| `SERVICE_VERSION` | Service version | `SERVICE_VERSION := 1.2.3` |
| `SERVICE_BUILD_FLAGS` | Extra flags passed to `go build` | `SERVICE_BUILD_FLAGS := -tags prod` |
| `SERVICE_TEST_FLAGS` | Extra flags passed to `go test` | `SERVICE_TEST_FLAGS := -run TestBilling` |

## Lifecycle Hooks

Hook into build/test lifecycle without redefining targets:

| Variable | When Executed | Example |
|----------|---------------|---------|
| `SERVICE_PRE_BUILD` | Before build starts | `SERVICE_PRE_BUILD = @echo "Generating code..."` |
| `SERVICE_POST_BUILD` | After build completes | `SERVICE_POST_BUILD = @echo "Build complete"` |
| `SERVICE_PRE_TEST` | Before tests run | `SERVICE_PRE_TEST = @echo "Setting up test DB..."` |
| `SERVICE_POST_TEST` | After tests complete | `SERVICE_POST_TEST = @echo "Cleaning up..."` |

## Example: Service with Code Generation

```make
SERVICE_NAME := api-router-service
SERVICE_VERSION := 2.1.0

# Generate OpenAPI client before build
SERVICE_PRE_BUILD = @go generate ./...

# Run integration tests with specific tags
SERVICE_TEST_FLAGS := -tags=integration

include ../../templates/service.mk
```

## Example: Service with Custom Build Flags

```make
SERVICE_NAME := analytics-service
SERVICE_VERSION := 1.0.0

# Production build with optimizations
SERVICE_BUILD_FLAGS := -ldflags="-s -w" -tags=prod

include ../../templates/service.mk
```

## Available Targets

The shared template provides these targets:

| Target | Description |
|--------|-------------|
| `make build` | Build the service binary |
| `make test` | Run unit tests |
| `make check` | Run lint + test |
| `make lint` | Run golangci-lint |
| `make clean` | Remove build artifacts |
| `make ci-remote` | Trigger remote CI |

## Best Practices

1. **Don't redefine shared targets** - Use hooks instead
2. **Keep overrides above include** - Variables must be set before template loads
3. **Use standard variable names** - Ensures compatibility with CI

## Reference

- Template source: `templates/service.mk`
- Example service: `services/_template/`
- [service-checklist.md](service-checklist.md) - Full service creation checklist
