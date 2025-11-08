# hello-service

# hello-service

Generated via `make service-new NAME=hello-service`.

## Local Development

```bash
make build
make test
make check
```

## Usage

Build and run the CLI to produce a greeting:

```bash
go run ./cmd/hello-service            # => Hello, there!
go run ./cmd/hello-service Taylor     # => Hello, Taylor!
```

The greeting logic lives in `pkg/hello` and is covered by unit tests.

For customization options see `docs/services/customizing.md`.
