# Contract Generation

This package provides tools for validating and generating Go code from OpenAPI specifications.

## Prerequisites

### Required Tools

1. **oapi-codegen** - For generating Go types from OpenAPI specs
   ```bash
   go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest
   ```

2. **spectral** (optional) - For comprehensive OpenAPI validation
   ```bash
   npm install -g @stoplight/spectral-cli
   ```

## Usage

### Using Makefile Targets

The easiest way to generate contracts:

```bash
# Validate and generate contracts
make contracts

# Validate only
make contracts-validate

# Generate only (requires oapi-codegen)
make contracts-generate
```

### Using the CLI Tool Directly

```bash
# Validate OpenAPI spec
go run ./cmd/contracts -validate

# Generate Go types
go run ./cmd/contracts -generate

# Custom spec path
go run ./cmd/contracts -validate -spec path/to/spec.yaml

# Custom output path
go run ./cmd/contracts -generate -output ./generated/types.go -package mytypes
```

## Generated Code

The generated code includes:

- **Type Definitions**: Go structs for all OpenAPI schemas
- **Request/Response Types**: Types for API request and response bodies
- **Validation**: Type-safe structs with JSON tags

Generated code is written to `pkg/contracts/generated.go` by default.

## OpenAPI Specification

The OpenAPI specification is located at:
```
specs/006-api-router-service/contracts/api-router.openapi.yaml
```

## Validation

The validation process:

1. Checks if the spec file exists
2. If `spectral` is available, runs comprehensive linting
3. Otherwise, performs basic YAML/JSON validation

For best results, install `spectral` for comprehensive OpenAPI validation.

## Examples

### Generate Types for a Custom Spec

```bash
go run ./cmd/contracts -generate \
  -spec /path/to/custom-spec.yaml \
  -output ./custom/generated.go \
  -package custom
```

### Validate Before Generation

```bash
# Validate first
make contracts-validate

# If validation passes, generate
make contracts-generate
```

## Integration with CI/CD

The contract generation can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions step
- name: Generate contracts
  run: |
    go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest
    make contracts
```

## Troubleshooting

### oapi-codegen not found

Install it:
```bash
go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest
```

### OpenAPI spec not found

The tool auto-detects the spec path. If it fails, specify it explicitly:
```bash
go run ./cmd/contracts -validate -spec /absolute/path/to/spec.yaml
```

### Validation fails

1. Check that the spec file is valid YAML
2. Install `spectral` for better error messages:
   ```bash
   npm install -g @stoplight/spectral-cli
   ```

