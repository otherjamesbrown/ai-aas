# Services Overview

Services in the AI-AAS platform share a common automation layer delivered by the root `Makefile` and `templates/service.mk`.

## Creating a New Service

```bash
make service-new NAME=billing-service
```

This command:

1. Copies the `_template/` scaffold into `services/billing-service`.
2. Replaces placeholders (Makefile, README, Go module, main.go).
3. Registers the module in `go.work`.

Review the generated README and customize service-specific notes.

## Standard Targets

Each service inherits the following targets from `templates/service.mk`:

- `make build` — Compile binaries under `bin/`.
- `make test` — Run unit tests.
- `make fmt` — Apply formatting to Go files.
- `make lint` — Execute `golangci-lint`.
- `make security` — Execute `gosec`.
- `make check` — Run fmt + lint + security + tests.
- `make clean` — Remove build artifacts and caches.

Service maintainers can extend targets by appending rules below the `include ../../templates/service.mk` line.

## Directory Guidelines

- Keep commands and binaries under `cmd/` and `bin/`.
- Place reusable packages in `pkg/`.
- Store configuration and sample data under `config/`.
- Update service-specific docs next to the code or reference shared docs.

For additional customization patterns, see `docs/services/customizing.md`.

