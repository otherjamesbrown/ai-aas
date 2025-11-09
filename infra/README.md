# Infrastructure Directory

This directory contains infrastructure-as-code assets and supporting tooling for provisioned environments.

- `terraform/` – Terraform modules, environment stacks, backend configuration, and automation entry points.
- `helm/` – Helm charts and values overlays applied via ArgoCD.
- `argo/` – ArgoCD Application and ApplicationSet manifests that reconcile infrastructure components.
- `tests/` – Infrastructure test suites (Terratest, synthetic probes, performance harnesses).
- `secrets/` – Templates and bundles for syncing sensitive configuration into clusters.

See `docs/platform/infrastructure-overview.md` for the high-level architecture and ownership model.
