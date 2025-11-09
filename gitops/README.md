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
