# Shared Go Unit Test Harness

This module houses unit tests for the shared Go libraries. Phase 2 provisions the structure so later tasks can layer real test cases without additional scaffolding.

- Tests import packages from `github.com/ai-aas/shared-go/...`.
- Run with `go test ./...` (invoked automatically through `make shared-go-test`).
- Keep helper fixtures local to this directory to avoid coupling with service-specific tests.

