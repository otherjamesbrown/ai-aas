# Centralized tool version manifest.
# Update these values when bumping toolchains; automation must reference them.

GO_VERSION ?= 1.21.6
GO_TOOLCHAIN ?= go1.21.6
GOLANGCI_LINT_VERSION ?= 1.55.2
GOSEC_VERSION ?= 2.19.0
ACT_VERSION ?= 0.2.61
AWS_CLI_VERSION ?= 2.17.0
MINIO_CLIENT_VERSION ?= 2025-01-15T00-00-00Z

# Example usage (in Makefile):
#   include configs/tool-versions.mk
#   GO ?= go
#   GOLANGCI_LINT ?= golangci-lint
#   check-go-version:
#       @$(GO) version | grep -q "go${GO_VERSION}"

