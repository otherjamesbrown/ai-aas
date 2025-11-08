# world-service

# world-service

Generated via `make service-new NAME=world-service`.

## Local Development

```bash
make build
make test
make check
```

## Usage

```bash
WORLD_SERVICE_ADDR=":8090" go run ./cmd/world-service &
curl http://localhost:8090/world
```

Example response:

```json
{
  "message": "Hello, world-service!",
  "timestamp": "2025-11-08T12:34:56Z"
}
```

Stop the server with `Ctrl+C`. For customization, see `docs/services/customizing.md`.
