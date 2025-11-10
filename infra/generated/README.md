# Generated Infrastructure Artifacts

Terraform renders cluster manifests into version-controlled form so GitOps
pipelines can promote them. Files are organized per environment, mirroring the
outputs created under `infra/terraform/environments/<env>/.generated/`.

- `argo/` – ApplicationSet manifests that bootstrap ArgoCD resources.
- `network/` – NetworkPolicy YAML and Linode firewall specs.
- `observability/` – Helm values overrides for kube-prometheus-stack.
- `data-services/` – Endpoint documentation for shared backing services.
- `secrets/` – SealedSecret skeletons (replace placeholders before applying).

Use these files as the source of truth for GitOps repositories. After updating a
Terraform environment, re-run the copy step or `terraform apply` to regenerate
and then commit the refreshed artifacts.
