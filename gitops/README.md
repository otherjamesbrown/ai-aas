# GitOps Repository Structure

Infrastructure manifests promoted from Terraform live here. ArgoCD (or an
alternative GitOps controller) should sync from these directories to manage the
live clusters.

```
gitops/
  clusters/
    development/
      apps/        # ArgoCD ApplicationSets, chart values, policies, secrets
      projects/    # ArgoCD Project definitions
    production/
      apps/
      projects/
  templates/       # Reusable skeletons (optional)
```

Promotion flow:
1. Run Terraform (`terraform apply -var environment=<env>`).
2. Copy `infra/generated/<env>/` files into `gitops/clusters/<env>/apps/` using
   `scripts/gitops/sync.sh infra/generated/<env> gitops/clusters/<env>/apps`.
3. Commit changes to this directory and push to the GitOps repo watched by
   ArgoCD.
4. Monitor ArgoCD sync status to confirm resources converge.

Secrets (e.g., kubeconfigs, bootstrap secrets) should be stored as SealedSecrets
or equivalent before committing.

## Managing Platform Bootstrap Secrets

- Source secret values locally (we keep `GRAFANA_ADMIN` and `REGISTRY_TOKEN` in
  `.env`, which is git-ignored).
- For development:
  ```bash
  export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
  kubectl create secret generic platform-bootstrap \
    --namespace secrets \
    --from-env-file <(grep -E '^(GRAFANA_ADMIN|REGISTRY_TOKEN)=' .env) \
    --dry-run=client -o yaml \
  | kubeseal --controller-name sealed-secrets-controller \
      --controller-namespace kube-system --format yaml \
  > gitops/clusters/development/apps/secrets/bootstrap-sealedsecret.yaml
  ```
- Repeat for production (switch `KUBECONFIG` and output path to
  `production/apps/secrets/`).
- Commit the sealed manifests; the plaintext secret is never written to disk
  beyond the pipe.
- After updating secrets, run `argocd app sync secrets-<env> --grpc-web` to
  propagate changes.
