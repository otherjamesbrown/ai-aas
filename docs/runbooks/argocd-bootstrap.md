# ArgoCD Bootstrap Runbook

## Overview
Use this runbook to install ArgoCD in a target cluster and register the
`gitops/` directory so ApplicationSets manage infrastructure resources for the
environment. Repeat for each cluster (development, production) using its
kubeconfig context and secrets.

## Prerequisites
- Helm 3.14+
- kubectl
- ArgoCD CLI (`brew install argocd` on macOS)
- Kubeconfig for the target cluster (see `docs/platform/infrastructure-overview.md`)
- Repository credentials (personal access token with read access)

## Steps

1. **Authenticate to the cluster**
   ```bash
   export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
   kubectl config use-context lke531921-ctx
   ```

2. **Run bootstrap script**
   ```bash
   ./scripts/gitops/bootstrap_argocd.sh development lke531921-ctx
   ```
   Environment name must match `gitops/clusters/<env>`.

3. **Retrieve initial admin password (first install only)**
   ```bash
   kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 --decode
   ```

4. **Port-forward the ArgoCD UI (optional)**
   ```bash
   kubectl -n argocd port-forward svc/argocd-server 8080:80
   open http://localhost:8080
   ```

5. **Register the GitOps repository**
   ```bash
   argocd login localhost:8080 --username admin --password <password> --insecure
   argocd repo add https://github.com/otherjamesbrown/ai-aas.git \
     --username <github-username> --password <github-token>
   ```

6. **Verify ApplicationSet sync**
   ```bash
   argocd app list
   argocd app sync platform-development-infrastructure
   argocd app get platform-development-infrastructure
   ```
   Ensure status is `Synced` and `Healthy`.

7. **Repeat for production**
   ```bash
   export KUBECONFIG=~/kubeconfigs/kubeconfig-production.yaml
   ./scripts/gitops/bootstrap_argocd.sh production lke531922-ctx
   ```

## Troubleshooting
- Helm install failures: check for network connectivity or conflicting
  resources. `helm uninstall argocd -n argocd` before retrying if necessary.
- Repo access denied: ensure GitHub PAT has `repo` scope and ArgoCD instance
  egress is permitted.
- Application stuck progressing: inspect events via `kubectl -n argocd get events`
  and check ArgoCD logs (`kubectl -n argocd logs deploy/argocd-application-controller`).
- Redis secret missing: rerun the bootstrap script; it auto-creates `argocd-redis`
  if absent. Delete failing pods after secret creation (`kubectl -n argocd delete pod <pod>`).

## Post-Bootstrap Tasks
- Configure SSO or external auth for ArgoCD server.
- Switch the service type to LoadBalancer or ingress by editing
  `gitops/templates/argocd-values.yaml` and re-running the bootstrap script.
- Set up ArgoCD notifications/webhooks if desired.
