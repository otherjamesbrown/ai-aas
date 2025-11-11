# Go Sample Service Template

This minimal service demonstrates how shared Go libraries will be integrated. Phase 2 provisions the skeleton so later tasks can focus on wiring configuration, observability, data access, and error handling.

## Commands

- `go run ./cmd/service-template` – run the HTTP server (defaults to `:8080`)
- `make run` – convenience wrapper (defined in the local Makefile)

## Endpoints

- `GET /healthz` – returns a simple JSON body indicating service health

Future phases will replace the placeholders with integrations from `shared/go/...`.

