# Terraform Infrastructure

This directory contains Terraform configuration for provisioning Akamai Linode infrastructure across the development, staging, production, and system environments.

- `backend.hcl` – Remote state configuration targeting Linode Object Storage.
- `environments/` – Per-environment stacks that compose Terraform modules for the platform.
- `modules/` – Reusable modules (cluster, network, observability, secrets, data services).
- `Makefile` – Developer entry points for `plan`, `apply`, `destroy`, `drift`, and `state` commands.
- `state-policy.json` – Bucket policy enforcing least-privilege access to Terraform state.

Use the Makefile targets instead of invoking `terraform` directly to ensure linting, security scans, and approvals run consistently.
