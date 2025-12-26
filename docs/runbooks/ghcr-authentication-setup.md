# GHCR Authentication Setup - Runbook

---
last_updated: 2025-12-14
document_type: guide
purpose: Setup GitHub Container Registry (GHCR) authentication for pulling private container images
---

## Overview

This runbook provides instructions for configuring GitHub Container Registry (GHCR) authentication in Kubernetes clusters. All AI-AAS service images are stored in GHCR as private images and require authentication to pull.

**Use this runbook when**:
- Bootstrapping a new environment
- Deploying a service that fails with `ImagePullBackOff` or `ErrImagePull`
- Adding a new namespace that needs to pull GHCR images
- Rotating GHCR credentials

**Time to complete**: 5-10 minutes per namespace

## Prerequisites

### Required Credentials

You need a GitHub Personal Access Token (PAT) with the following scopes:
- `read:packages` - Read container images from GHCR
- `write:packages` - (Optional) Push images during CI/CD

**Creating a PAT**:
1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Click "Generate new token (classic)"
3. Select scopes: `read:packages` (and `write:packages` if needed for CI/CD)
4. Copy the token - you won't be able to see it again!
5. Store securely in `secrets/env/.env` as `GITHUB_TOKEN=ghp_xxx`

**Required tools**:
```bash
kubectl version --client  # Any recent version
```

### Repository Information

- **Registry**: `ghcr.io`
- **Organization**: `otherjamesbrown`
- **Repository**: `ai-aas`
- **Image naming pattern**: `ghcr.io/otherjamesbrown/ai-aas/<service-name>:<tag>`

**Example images**:
- `ghcr.io/otherjamesbrown/ai-aas/api-router-service:dev`
- `ghcr.io/otherjamesbrown/ai-aas/admin-api-service:staging`
- `ghcr.io/otherjamesbrown/ai-aas/analytics-service:main`

## Creating GHCR Pull Secret

### Step 1: Set Environment Variables

Set your GitHub credentials:

```bash
# GitHub username (organization owner)
export GITHUB_USERNAME="otherjamesbrown"

# GitHub Personal Access Token (from secrets/env/.env)
export GITHUB_TOKEN="ghp_xxx"  # Replace with actual token

# Target namespace (where the service will be deployed)
export NAMESPACE="development"  # Or staging, analytics-service, etc.

# Environment name (for kubeconfig selection)
export ENVIRONMENT="development"  # Or staging, production
```

**Common namespaces**:
- `development` - Development environment services
- `staging` - Staging environment services
- `admin-api-service` - Admin API service
- `analytics-service` - Analytics service
- `user-org-service` - User/Organization service
- `ai-model-system` - AI model operator

### Step 2: Create Kubernetes Secret

Use `kubectl create secret` to generate a properly-formatted Docker config secret:

```bash
kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml \
  create secret docker-registry ghcr-pull-secret \
  --docker-server=ghcr.io \
  --docker-username=${GITHUB_USERNAME} \
  --docker-password=${GITHUB_TOKEN} \
  --namespace=${NAMESPACE}
```

**Expected output**:
```
secret/ghcr-pull-secret created
```

### Step 3: Verify Secret Creation

Verify the secret exists and has the correct format:

```bash
kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml \
  get secret ghcr-pull-secret \
  --namespace=${NAMESPACE} \
  -o jsonpath='{.type}'
```

**Expected output**:
```
kubernetes.io/dockerconfigjson
```

**Verify the secret contains authentication data** (without exposing it):

```bash
kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml \
  get secret ghcr-pull-secret \
  --namespace=${NAMESPACE} \
  -o jsonpath='{.data.\.dockerconfigjson}' | wc -c
```

Should return a number greater than 100 (indicating the secret has data).

## Configuring Services to Use Pull Secret

### Helm Chart Configuration

Add `imagePullSecrets` to the service's Helm values file:

**File**: `services/<service-name>/deployments/helm/<service-name>/values-<environment>.yaml`

```yaml
image:
  repository: ghcr.io/otherjamesbrown/ai-aas/<service-name>
  tag: <environment>  # dev, staging, main
  pullPolicy: Always
  pullSecrets:
    - ghcr-pull-secret  # Reference the secret name
```

**Example** - `services/api-router-service/deployments/helm/api-router-service/values-development.yaml`:

```yaml
image:
  repository: ghcr.io/otherjamesbrown/ai-aas/api-router-service
  tag: dev
  pullPolicy: Always
  pullSecrets:
    - ghcr-pull-secret
```

### Helm Template Configuration

Ensure your Helm chart's `templates/deployment.yaml` uses the `imagePullSecrets` value:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "<chart>.fullname" . }}
spec:
  template:
    spec:
      {{- if .Values.image.pullSecrets }}
      imagePullSecrets:
        {{- range .Values.image.pullSecrets }}
        - name: {{ . }}
        {{- end }}
      {{- end }}
      containers:
        - name: {{ .Chart.Name }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
```

**Verification**:

Test the Helm template renders correctly:

```bash
helm template services/<service-name>/deployments/helm/<service-name>/ \
  -f services/<service-name>/deployments/helm/<service-name>/values-<environment>.yaml \
  | grep -A 5 imagePullSecrets
```

**Expected output**:
```yaml
      imagePullSecrets:
        - name: ghcr-pull-secret
```

## Testing Image Pull

After creating the secret and updating the Helm chart, test the deployment:

### Step 1: Apply Configuration via GitOps

Follow the GitOps workflow (do NOT use `kubectl apply` directly):

```bash
# 1. Commit changes
git add services/<service-name>/deployments/helm/<service-name>/values-<environment>.yaml
git commit -m "fix(helm): Add GHCR pull secret to <service-name> [aas-xxx]"

# 2. Push to repository
git push origin <branch>  # develop, staging, or main

# 3. Sync ArgoCD application
argocd app sync <service-name>-<environment>
```

### Step 2: Verify Pod Status

Check that pods start successfully:

```bash
kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml \
  get pods -n ${NAMESPACE} -l app=<service-name>
```

**Successful output**:
```
NAME                              READY   STATUS    RESTARTS   AGE
<service-name>-xxx-xxx            1/1     Running   0          30s
```

**Failed output** (if secret is missing or invalid):
```
NAME                              READY   STATUS             RESTARTS   AGE
<service-name>-xxx-xxx            0/1     ImagePullBackOff   0          30s
```

### Step 3: Verify Image Pull Events

If the pod status shows `ImagePullBackOff` or `ErrImagePull`, check events:

```bash
kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml \
  get events -n ${NAMESPACE} \
  --sort-by='.lastTimestamp' \
  --field-selector involvedObject.kind=Pod
```

**Look for**:
- `Successfully pulled image` - Secret is working correctly
- `Failed to pull image` with `unauthorized` - Secret is missing or credentials are invalid
- `Failed to pull image` with `not found` - Image name/tag is incorrect

## Multi-Namespace Setup

To configure GHCR authentication across all service namespaces, run this script:

```bash
#!/bin/bash
# Script: setup-ghcr-secrets-all-namespaces.sh

set -e

# Configuration
ENVIRONMENT=${1:-development}
GITHUB_USERNAME="otherjamesbrown"
GITHUB_TOKEN="${GITHUB_TOKEN:-$(grep GITHUB_TOKEN secrets/env/.env | cut -d'=' -f2)}"

if [ -z "$GITHUB_TOKEN" ]; then
  echo "ERROR: GITHUB_TOKEN not set. Export it or add to secrets/env/.env"
  exit 1
fi

# Define namespaces that need GHCR access
NAMESPACES=(
  "development"
  "staging"
  "admin-api-service"
  "analytics-service"
  "user-org-service"
  "ai-model-system"
)

KUBECONFIG="/home/dev/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml"

echo "Creating ghcr-pull-secret in ${ENVIRONMENT} cluster..."

for ns in "${NAMESPACES[@]}"; do
  echo "Processing namespace: $ns"

  # Check if namespace exists
  if ! kubectl --kubeconfig=$KUBECONFIG get namespace $ns &>/dev/null; then
    echo "  Namespace $ns does not exist - skipping"
    continue
  fi

  # Check if secret already exists
  if kubectl --kubeconfig=$KUBECONFIG get secret ghcr-pull-secret -n $ns &>/dev/null; then
    echo "  Secret already exists - skipping"
    continue
  fi

  # Create secret
  kubectl --kubeconfig=$KUBECONFIG \
    create secret docker-registry ghcr-pull-secret \
    --docker-server=ghcr.io \
    --docker-username=$GITHUB_USERNAME \
    --docker-password=$GITHUB_TOKEN \
    --namespace=$ns

  echo "  ✓ Created ghcr-pull-secret in $ns"
done

echo "Done!"
```

**Usage**:

```bash
chmod +x setup-ghcr-secrets-all-namespaces.sh
./setup-ghcr-secrets-all-namespaces.sh development
./setup-ghcr-secrets-all-namespaces.sh staging
```

## Troubleshooting

### Issue: ImagePullBackOff

**Symptoms**:
```bash
kubectl get pods -n <namespace>
NAME                       READY   STATUS             RESTARTS   AGE
service-xxx                0/1     ImagePullBackOff   0          2m
```

**Diagnosis**:

Check pod events:
```bash
kubectl describe pod <pod-name> -n <namespace> | grep -A 10 Events
```

**Common causes and fixes**:

| Error Message | Cause | Fix |
|---------------|-------|-----|
| `Failed to pull image: unauthorized` | Secret missing or invalid credentials | Recreate secret with valid GitHub PAT |
| `Failed to pull image: not found` | Incorrect image name or tag | Check `image.repository` and `image.tag` in values file |
| `Error: imagePullSecrets not found` | Secret name mismatch | Verify secret name is `ghcr-pull-secret` |
| `Error: secret not found in namespace` | Secret created in wrong namespace | Create secret in the pod's namespace |

### Issue: Credentials Expired

GitHub PATs can expire. If pulls suddenly fail:

1. **Generate new PAT** (see Prerequisites section)
2. **Delete old secret**:
   ```bash
   kubectl delete secret ghcr-pull-secret -n <namespace>
   ```
3. **Create new secret** with updated token (see Step 2)
4. **Restart pods** to pull with new credentials:
   ```bash
   kubectl rollout restart deployment/<deployment-name> -n <namespace>
   ```

### Issue: Secret Exists but Not Used

**Symptoms**: Secret exists, but pods still fail with `unauthorized`

**Diagnosis**:

Check if the deployment references the secret:
```bash
kubectl get deployment <deployment-name> -n <namespace> -o yaml | grep -A 5 imagePullSecrets
```

**Fix**:

Add `pullSecrets` to Helm values file and redeploy (see "Configuring Services to Use Pull Secret").

### Issue: Cross-Namespace Secret Access

**Problem**: Kubernetes secrets are namespace-scoped. You cannot reference a secret from namespace A in a pod in namespace B.

**Solution**: Create the `ghcr-pull-secret` in EVERY namespace that needs to pull GHCR images.

Use the multi-namespace script above for bulk creation.

## Security Best Practices

### Credential Rotation

Rotate GitHub PATs regularly (recommended: every 90 days):

1. Generate new PAT
2. Update secret in all namespaces
3. Revoke old PAT in GitHub settings
4. Update `secrets/env/.env` with new token

### Minimal Permissions

Use PATs with minimal required scopes:
- **Services**: `read:packages` only
- **CI/CD**: `read:packages` + `write:packages`

### Secret Storage

**DO**:
- Store PAT in `secrets/env/.env` (gitignored)
- Use environment variables for automation
- Document secret creation in runbooks

**DON'T**:
- Commit PATs to git repository
- Share PATs in chat/email
- Use personal PATs for production (use organization tokens if available)

## Integration with Bootstrap Process

When bootstrapping a new environment, add GHCR authentication to the provisioning workflow:

**File**: `scripts/infra/provision-environment.sh`

Add after cluster creation and before service deployment:

```bash
# Create GHCR pull secrets in all namespaces
echo "Creating GHCR pull secrets..."
NAMESPACES=("development" "staging" "admin-api-service" "analytics-service" "user-org-service" "ai-model-system")

for ns in "${NAMESPACES[@]}"; do
  kubectl create namespace $ns --dry-run=client -o yaml | kubectl apply -f -
  kubectl create secret docker-registry ghcr-pull-secret \
    --docker-server=ghcr.io \
    --docker-username=${GITHUB_USERNAME} \
    --docker-password=${GITHUB_TOKEN} \
    --namespace=$ns \
    --dry-run=client -o yaml | kubectl apply -f -
done
```

## Verification Checklist

Before closing this runbook, verify:

- [ ] Secret exists in target namespace: `kubectl get secret ghcr-pull-secret -n <namespace>`
- [ ] Secret has correct type: `kubernetes.io/dockerconfigjson`
- [ ] Helm values file includes `pullSecrets: [ghcr-pull-secret]`
- [ ] Helm template renders `imagePullSecrets` section
- [ ] ArgoCD application synced successfully
- [ ] Pods are running (not `ImagePullBackOff`)
- [ ] Pod events show "Successfully pulled image"

## Related Documentation

| Document | Purpose |
|----------|---------|
| [bootstrap-new-environment.md](bootstrap-new-environment.md) | Full environment bootstrap guide |
| [deploy-to-environments.md](deploy-to-environments.md) | GitOps deployment workflow |
| [environment-access.md](../platform/environment-access.md) | Credentials and access information |
| [github-actions-guide.md](../platform/github-actions-guide.md) | CI/CD pipeline configuration |

## References

- GitHub Container Registry Documentation: https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry
- Kubernetes Image Pull Secrets: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
- GitHub PAT Scopes: https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens#personal-access-tokens-classic
