# Centralized tool version manifest.
# Update these values when bumping toolchains; automation must reference them.

GO_VERSION ?= 1.24.6
GO_TOOLCHAIN ?= go1.24.10
GOLANGCI_LINT_VERSION ?= 1.55.2
GOSEC_VERSION ?= 2.19.0
ACT_VERSION ?= 0.2.61
AWS_CLI_VERSION ?= 2.17.0
MINIO_CLIENT_VERSION ?= 2025-01-15T00-00-00Z
TERRAFORM_VERSION ?= 1.6.6
VAULT_VERSION ?= 1.17.0
LINODE_CLI_VERSION ?= 5.45.0
HELM_VERSION ?= 3.14.4
TFSEC_VERSION ?= 1.28.1
TFLINT_VERSION ?= 0.50.0
TERRATEST_VERSION ?= 0.46.0

# Example usage (in Makefile):
#   include configs/tool-versions.mk
#   GO ?= go
#   GOLANGCI_LINT ?= golangci-lint
#   check-go-version:
#       @$(GO) version | grep -q "go${GO_VERSION}"

