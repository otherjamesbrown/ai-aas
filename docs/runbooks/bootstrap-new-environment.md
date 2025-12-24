# Bootstrap New Environment - Runbook

---
last_updated: 2025-12-08
document_type: guide
purpose: End-to-end guide for provisioning a complete Kubernetes environment from scratch
---

## Overview

This runbook provides a complete, step-by-step guide for bootstrapping a new AI-AAS environment from infrastructure creation through application deployment. It covers:

- Infrastructure provisioning (Terraform → Linode LKE cluster)
- Core platform installation (ArgoCD, cert-manager, Istio, Knative, KServe)
- GPU node configuration (optional)
- Platform service deployment
- Verification and troubleshooting

**Use this runbook when**:
- Creating a new environment (development, staging, production)
- Rebuilding an existing environment from scratch
- Setting up a test/demo environment
- Training new operators on the bootstrap process

**Time to complete**: 30-60 minutes (depending on cloud provider provisioning times)

## Prerequisites

### Required Tools

Install these tools before starting:

```bash
# Verify tool installation
terraform version  # Required: >= 1.0
kubectl version --client  # Required: Any recent version
helm version  # Required: >= 3.0
jq --version  # Required: For JSON parsing
argocd version --client  # Recommended: For ArgoCD operations
```

**Installation guides**:
- Terraform: https://developer.hashicorp.com/terraform/install
- kubectl: https://kubernetes.io/docs/tasks/tools/
- Helm: https://helm.sh/docs/intro/install/
- jq: `apt install jq` (Linux) or `brew install jq` (macOS)
- ArgoCD CLI: https://argo-cd.readthedocs.io/en/stable/cli_installation/

### Required Credentials

1. **Linode Personal Access Token**:
   - Create at: https://cloud.linode.com/profile/tokens
   - Required scopes: `linodes`, `lke`, `object-storage`
   - Set as environment variable:
     ```bash
     export LINODE_TOKEN="your-token-here"
     ```
   - OR add to `secrets/env/.env`:
     ```bash
     LINODE_TOKEN=your-token-here
     ```

2. **GitHub Repository Access**:
   - Repository URL: https://github.com/otherjamesbrown/ai-aas
   - Authentication: Personal Access Token (PAT) with `repo` scope
   - Required for ArgoCD git repository integration

3. **Terraform State Storage** (optional but recommended):
   - Local state: Works for single-operator environments
   - Remote state (S3/Linode Object Storage): Recommended for teams
   - Configure in `infra/terraform/environments/<env>/backend.tf`

### Repository Setup

Ensure you have the latest code:

```bash
cd ~/ai-aas  # Or your repository location
git checkout develop  # Or staging/main for respective environments
git pull origin develop
```

## Bootstrap Methods

Choose one of two methods:

### Method 1: Automated (Recommended)

Use the all-in-one provisioning script:

```bash
./scripts/infra/provision-environment.sh <environment>
```

**What it does**:
1. Validates prerequisites (tools, credentials)
2. Runs Terraform to create LKE cluster
3. Saves kubeconfig to `secrets/kubeconfigs/`
4. Installs ArgoCD
5. Applies GPU node labels (if GPU nodes present)
6. Bootstraps ArgoCD applications

**Skip to**: [Verify Deployment](#verify-deployment) section after completion

### Method 2: Manual (Detailed)

Follow the step-by-step process below for full control and understanding.

---

## Step-by-Step Bootstrap Process

### Phase 1: Infrastructure Provisioning (Terraform)

#### Step 1.1: Configure Terraform Variables

Navigate to the environment directory:

```bash
cd infra/terraform/environments/<environment>
# Example: cd infra/terraform/environments/staging
```

**Review `terraform.tfvars`**:

```hcl
# Example terraform.tfvars
environment = "staging"
cluster_label = "ai-aas-staging"
k8s_version = "1.32"
region = "fr-par"  # Paris, France

# Node pools
baseline_node_pool = {
  instance_type = "g6-standard-8"  # 8 vCPU, 16GB RAM
  node_count    = 3
}

gpu_node_pool = {
  instance_type = "g2-gpu-rtx4000a1-m"  # RTX4000, 20GB VRAM, 32GB RAM
  node_count    = 2
}
```

**Key configuration decisions**:
- **Baseline nodes**: For platform services (Admin API, routers, ArgoCD, etc.)
- **GPU nodes**: For model inference (vLLM deployments)
- **Region**: Choose nearest to your users
- **K8s version**: Match existing environments or use latest stable

**Reference**: See available Linode instance types at https://www.linode.com/pricing/

#### Step 1.2: Initialize Terraform

```bash
terraform init
```

**Expected output**:
```
Initializing provider plugins...
- Finding linode/linode versions matching "~> 2.0"...
- Installing linode/linode v2.x.x...
Terraform has been successfully initialized!
```

**Troubleshooting**:
- `terraform init` fails: Check network connectivity, provider versions
- Lock file conflicts: Delete `.terraform.lock.hcl` and retry

#### Step 1.3: Plan Infrastructure Changes

```bash
terraform plan -var-file=terraform.tfvars -out=tfplan
```

**Review the plan carefully**:
- Verify node pool sizes match your intent
- Check estimated costs (Terraform shows Linode pricing)
- Confirm region and k8s_version are correct

**Save the plan**: The `-out=tfplan` flag saves the plan for exact application

#### Step 1.4: Apply Infrastructure

```bash
terraform apply tfplan
```

**What happens**:
1. Linode LKE cluster is created (2-5 minutes)
2. Node pools are provisioned (3-10 minutes depending on size)
3. Kubernetes API becomes available
4. Terraform outputs kubeconfig and cluster info

**Expected output**:
```
Apply complete! Resources: 3 added, 0 changed, 0 destroyed.

Outputs:
cluster_id = "531921"
cluster_status = "ready"
kubeconfig = <sensitive>
api_endpoints = ["https://xxx-xxx-xxx-xxx.linodelke.net:443"]
```

**Troubleshooting**:
- Quota exceeded: Contact Linode support to increase limits
- Timeout: Linode provisioning can be slow; wait and check Linode UI
- Apply failed mid-way: Run `terraform apply` again (Terraform is idempotent)

#### Step 1.5: Save Kubeconfig

**Automated (recommended)**:
```bash
# Terraform output method
terraform output -raw kubeconfig > ../../../../secrets/kubeconfigs/kubeconfig-staging.yaml
chmod 600 ../../../../secrets/kubeconfigs/kubeconfig-staging.yaml
```

**Manual (fallback)**:
```bash
# Get cluster ID from Terraform output
CLUSTER_ID=$(terraform output -raw cluster_id)

# Download kubeconfig from Linode API
curl -H "Authorization: Bearer $LINODE_TOKEN" \
  "https://api.linode.com/v4/lke/clusters/${CLUSTER_ID}/kubeconfig" | \
  jq -r '.kubeconfig' | base64 -d > ../../../../secrets/kubeconfigs/kubeconfig-staging.yaml

chmod 600 ../../../../secrets/kubeconfigs/kubeconfig-staging.yaml
```

**Set kubeconfig for subsequent steps**:
```bash
export KUBECONFIG=~/ai-aas/secrets/kubeconfigs/kubeconfig-staging.yaml
```

**Verify cluster access**:
```bash
kubectl cluster-info
kubectl get nodes
```

**Expected output**:
```
NAME                        STATUS   ROLES    AGE   VERSION
lke531921-baseline-xxx1     Ready    <none>   3m    v1.32.0
lke531921-baseline-xxx2     Ready    <none>   3m    v1.32.0
lke531921-gpu-xxx1          Ready    <none>   3m    v1.32.0
```

---

### Phase 2: Core Platform Installation

#### Step 2.1: Install ArgoCD

**Option A: Using bootstrap script** (Recommended):

```bash
./scripts/gitops/bootstrap_argocd.sh staging
```

**Option B: Manual Helm installation**:

```bash
# Add ArgoCD Helm repository
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update

# Install ArgoCD
helm upgrade --install argocd argo/argo-cd \
  --namespace argocd \
  --create-namespace \
  -f gitops/templates/argocd-values.yaml \
  --wait --timeout=5m
```

**Wait for ArgoCD to be ready**:
```bash
kubectl wait --for=condition=available --timeout=300s \
  deployment/argocd-server -n argocd
```

**Retrieve initial admin password**:
```bash
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d
echo  # Newline for readability
```

**Save this password** - you'll need it for ArgoCD UI access.

**Access ArgoCD UI** (optional):
```bash
kubectl port-forward svc/argocd-server -n argocd 8080:443
# Open: https://localhost:8080
# Username: admin
# Password: (from previous step)
```

**Troubleshooting**:
- Pods stuck in Pending: Check node resources with `kubectl describe nodes`
- Redis connection errors: Create redis secret manually (bootstrap script does this automatically)
- Webhook errors: Ensure cert-manager is not already installed (it will be installed next)

#### Step 2.2: Apply ArgoCD Projects

ArgoCD Projects define RBAC boundaries for applications.

```bash
kubectl apply -f gitops/clusters/staging/projects/
```

**Verify project creation**:
```bash
kubectl get appprojects -n argocd
```

**Expected output**:
```
NAME               AGE
platform-staging   10s
```

**What this does**:
- Creates `platform-staging` AppProject
- Defines allowed destination namespaces
- Defines allowed source repositories (GitHub repo)
- Sets cluster resource whitelists (RBAC, networking)

**Troubleshooting**:
- Project creation fails: Check YAML syntax in `gitops/clusters/staging/projects/platform-project.yaml`
- Permission denied: Ensure ArgoCD is fully initialized

#### Step 2.3: Bootstrap Infrastructure Applications

Infrastructure applications include cert-manager, Istio, Knative, KServe, and GPU operator.

```bash
kubectl apply -f gitops/clusters/staging/apps/ --recursive
```

**What gets deployed**:
- cert-manager (TLS certificate management)
- Istio (service mesh)
- Knative Serving (serverless platform)
- KServe (model serving framework)
- GPU Operator (NVIDIA driver/device plugin) - if GPU nodes present
- Monitoring stack (if defined)

**Watch ArgoCD sync the applications**:
```bash
# List all applications
kubectl get applications -n argocd

# Watch sync status (requires argocd CLI)
watch -n 2 'kubectl get applications -n argocd -o wide'
```

**Expected progression**:
1. Applications initially show `OutOfSync` status
2. ArgoCD syncs them automatically (if auto-sync enabled)
3. Applications transition to `Synced` and `Healthy`

**Initial sync order** (ArgoCD handles dependencies):
1. cert-manager (base dependency for TLS)
2. Istio (service mesh)
3. Knative Serving (depends on Istio)
4. KServe (depends on Knative, cert-manager)
5. GPU Operator (independent, can sync in parallel)

**Troubleshooting**:
- Applications stuck OutOfSync: Manual sync with `kubectl patch application <app> -n argocd -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{}}}' --type merge`
- Application degraded: Check application status with `kubectl describe application <app> -n argocd`
- Dependency issues: Some apps require others to be healthy first (e.g., KServe needs Knative)

#### Step 2.4: Register Git Repository with ArgoCD

ArgoCD needs credentials to pull from the private GitHub repository.

**Option A: Using ArgoCD CLI**:

```bash
# Login to ArgoCD
argocd login localhost:8080 --username admin --password <password> --insecure

# Add repository
argocd repo add https://github.com/otherjamesbrown/ai-aas.git \
  --username <github-username> \
  --password <github-pat-token>
```

**Option B: Using kubectl**:

```bash
kubectl create secret generic ai-aas-repo \
  --namespace argocd \
  --from-literal=url=https://github.com/otherjamesbrown/ai-aas.git \
  --from-literal=username=<github-username> \
  --from-literal=password=<github-pat-token> \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl label secret ai-aas-repo -n argocd \
  argocd.argoproj.io/secret-type=repository
```

**Verify repository connection**:
```bash
argocd repo list
# OR
kubectl get secrets -n argocd -l argocd.argoproj.io/secret-type=repository
```

**Troubleshooting**:
- Connection failed: Verify GitHub PAT has `repo` scope
- Private repo access: Ensure token has access to repository
- ArgoCD can't fetch: Check network egress from cluster

---

### Phase 3: GPU Configuration (Optional)

Skip this phase if your environment has no GPU nodes.

#### Step 3.1: Verify GPU Nodes

```bash
# List all nodes with GPU instance types
kubectl get nodes -o json | jq -r '.items[] | select(.metadata.labels."node.kubernetes.io/instance-type" | contains("gpu")) | .metadata.name'
```

**Expected output**:
```
lke531921-gpu-xxx1
lke531921-gpu-xxx2
```

#### Step 3.2: Apply GPU Node Labels

Labels enable workload scheduling to specific GPU types:

```bash
./scripts/infra/apply-gpu-node-labels.sh
```

**What this does**:
- Adds `node-type=gpu` label
- Adds `ai-aas.io/gpu-class=rtx4000-medium` (or appropriate class)
- Adds `nvidia.com/gpu.product=NVIDIA-RTX-4000-Ada`
- Applies taint `gpu-workload=true:NoSchedule` (prevents non-GPU workloads)

**Verify labels**:
```bash
kubectl get nodes -l node-type=gpu --show-labels
```

**Expected output**:
```
NAME                     LABELS
lke531921-gpu-xxx1       node-type=gpu,ai-aas.io/gpu-class=rtx4000-medium,...
```

#### Step 3.3: Verify GPU Operator

The GPU operator should have been installed via ArgoCD in Phase 2.

```bash
# Check GPU operator pods
kubectl get pods -n gpu-operator

# Verify GPU capacity is reported
kubectl get nodes -l node-type=gpu -o json | jq -r '.items[] | "\(.metadata.name): \(.status.allocatable."nvidia.com/gpu" // "0") GPUs"'
```

**Expected output**:
```
lke531921-gpu-xxx1: 1 GPUs
lke531921-gpu-xxx2: 1 GPUs
```

**Troubleshooting**:
- GPU operator not running: Check ArgoCD application `gpu-operator-staging`
- GPU capacity shows 0: Check nvidia-device-plugin daemonset logs
- Driver installation failing: Check nvidia-driver-daemonset logs

**Reference**: See [infrastructure-overview.md](../platform/infrastructure-overview.md) for GPU architecture

---

### Phase 4: Platform Service Deployment

#### Step 4.1: Verify Infrastructure Applications

Before deploying platform services, ensure infrastructure is healthy:

```bash
# Check all applications
kubectl get applications -n argocd -o wide

# Check for unhealthy apps
kubectl get applications -n argocd -o json | \
  jq -r '.items[] | select(.status.health.status != "Healthy") | "\(.metadata.name): \(.status.health.status)"'
```

**Expected state**: All infrastructure apps should be `Healthy` and `Synced`.

**If any apps are unhealthy**:
1. Check application details: `kubectl describe application <app> -n argocd`
2. Check pod status: `kubectl get pods -n <namespace>`
3. Review logs: `kubectl logs -n <namespace> <pod>`
4. Refer to [argocd-testing-guide.md](../platform/argocd-testing-guide.md)

#### Step 4.2: Create Platform Secrets

Platform services require secrets that are NOT stored in git.

**Database credentials**:
```bash
# Create database secret (if not using external DB)
kubectl create secret generic database-credentials \
  --namespace=admin-api-service \
  --from-literal=DATABASE_URL="postgresql://user:pass@host:5432/admin_api_db" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic database-credentials \
  --namespace=user-org-service \
  --from-literal=DATABASE_URL="postgresql://user:pass@host:5432/user_org_db" \
  --dry-run=client -o yaml | kubectl apply -f -
```

**API keys and service credentials**:
```bash
# Master admin API key
kubectl create secret generic admin-api-secrets \
  --namespace=admin-api-service \
  --from-literal=MASTER_ADMIN_API_KEY="$(openssl rand -hex 32)" \
  --from-literal=JWT_SECRET="$(openssl rand -hex 32)" \
  --dry-run=client -o yaml | kubectl apply -f -
```

**External service endpoints** (LoadBalancer IPs, etc.):
```bash
# Create ConfigMap for external endpoints (not committed to git)
kubectl create configmap external-endpoints \
  --namespace=development \
  --from-literal=LOADBALANCER_IP="<ip-from-linode>" \
  --dry-run=client -o yaml | kubectl apply -f -
```

**Reference**: See [environment-access.md](../platform/environment-access.md) for credential patterns

#### Step 4.3: Deploy Platform Services

Platform services are deployed via ArgoCD and should already be syncing.

**Verify platform service applications**:
```bash
kubectl get applications -n argocd | grep -E "admin-api|user-org|api-router|analytics|web-portal"
```

**Expected applications**:
- `admin-api-service-staging`
- `user-org-service-staging`
- `api-router-service-staging`
- `analytics-service-staging` (if deployed)
- `web-portal-staging` (if deployed)

**Monitor service deployment**:
```bash
# Watch pods in development/staging namespace
watch -n 2 'kubectl get pods -n staging'

# Check service health
kubectl get pods -n staging -l app=admin-api-service
kubectl get pods -n staging -l app=user-org-service
```

**Wait for all services to be Running**:
```bash
# All pods should eventually be Running and Ready
kubectl get pods -n staging
```

**Troubleshooting**:
- Pods stuck in Pending: Check resource requests vs node capacity
- Pods in CrashLoopBackOff: Check logs with `kubectl logs -n staging <pod>`
- ImagePullBackOff: Verify image names and registry access
- Database connection errors: Verify database secrets and connectivity

---

### Phase 5: Verification

Follow the comprehensive verification checklist:

**Quick verification script**:
```bash
#!/bin/bash
# Save as scripts/infra/verify-environment.sh

echo "=== Cluster Nodes ==="
kubectl get nodes

echo -e "\n=== ArgoCD Applications ==="
kubectl get applications -n argocd -o wide

echo -e "\n=== Failed Pods ==="
kubectl get pods -A | grep -v -E "Running|Completed" || echo "None"

echo -e "\n=== Platform Services ==="
kubectl get pods -n staging

echo -e "\n=== GPU Nodes (if applicable) ==="
kubectl get nodes -l node-type=gpu --show-labels || echo "No GPU nodes"

echo -e "\n=== Ingress Status ==="
kubectl get ingress -A

echo -e "\n=== Certificate Status ==="
kubectl get certificates -A
```

**Run verification**:
```bash
chmod +x scripts/infra/verify-environment.sh
./scripts/infra/verify-environment.sh
```

**Comprehensive verification**: See [new-environment-checklist.md](../platform/new-environment-checklist.md) for detailed verification procedures (30+ checkpoints).

**Key verification points**:
1. All nodes are Ready
2. All ArgoCD applications are Healthy and Synced
3. No pods in CrashLoopBackOff or Error state
4. Platform services respond to health checks
5. GPU capacity is detected (if GPU nodes present)
6. Certificates are Ready (if cert-manager configured)
7. Ingress has LoadBalancer IP assigned

---

## Post-Bootstrap Configuration

### Configure DNS

1. **Get LoadBalancer IP**:
   ```bash
   kubectl get svc -n istio-system istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
   ```

2. **Update DNS records**:
   - Create A records pointing to LoadBalancer IP:
     - `*.staging.ai-aas.com` → LoadBalancer IP
     - `api.staging.ai-aas.com` → LoadBalancer IP
     - `argocd.staging.ai-aas.com` → LoadBalancer IP

3. **Update ArgoCD ingress** to use custom domain

**Reference**: See [endpoints-and-urls.md](../platform/endpoints-and-urls.md)

### Configure TLS Certificates

**Self-signed certificates** (development/testing):
```bash
./scripts/infra/generate-self-signed-certs.sh staging
./scripts/infra/create-tls-secrets.sh staging
```

**Let's Encrypt** (staging/production):
1. Ensure cert-manager is installed (Phase 2)
2. Configure ClusterIssuer for Let's Encrypt
3. Certificate resources are created automatically by Ingress annotations

**Reference**: See [tls-ssl-setup.md](../platform/tls-ssl-setup.md)

### Configure Observability (Optional but Recommended)

Deploy monitoring stack:

1. **Check if monitoring apps exist**:
   ```bash
   kubectl get applications -n argocd | grep -E "prometheus|grafana|loki"
   ```

2. **If not deployed**, add monitoring applications to `gitops/clusters/staging/apps/`

3. **Access Grafana**:
   ```bash
   kubectl port-forward -n observability svc/grafana 3000:80
   # Open: http://localhost:3000
   ```

**Reference**: See [observability-guide.md](../platform/observability-guide.md)

### Update Documentation

After successful bootstrap:

1. **Update [environment-access.md](../platform/environment-access.md)**:
   - Add kubeconfig location
   - Add ArgoCD URL and credentials
   - Add database connection strings
   - Add API endpoint URLs

2. **Update [endpoints-and-urls.md](../platform/endpoints-and-urls.md)**:
   - Add LoadBalancer IP
   - Add ingress URLs
   - Add service endpoints

3. **Update this runbook** if you encountered any issues or discovered improvements

---

## Rollback Procedures

### Rollback ArgoCD Application

If an application deployment fails:

```bash
# Rollback to previous version
argocd app rollback <app-name> <revision-number>

# Example: Rollback to revision 5
argocd app rollback admin-api-service-staging 5

# Or find revision history first
argocd app history admin-api-service-staging
```

### Rollback Infrastructure (Terraform)

If cluster creation needs to be undone:

```bash
cd infra/terraform/environments/staging

# Destroy infrastructure
terraform destroy -var-file=terraform.tfvars

# This will:
# - Delete LKE cluster
# - Delete all node pools
# - Remove Linode resources
```

**WARNING**: This is destructive and cannot be undone. All cluster data will be lost.

### Partial Rollback

To remove only platform services but keep infrastructure:

```bash
# Delete ArgoCD applications
kubectl delete applications -n argocd -l environment=staging

# Or delete specific applications
kubectl delete application admin-api-service-staging -n argocd
```

---

## Troubleshooting Common Issues

### Issue: Terraform Apply Fails with Quota Error

**Symptom**:
```
Error: Error creating a Linode LKE Cluster: [409] Your account must be validated before you can create an LKE cluster
```

**Resolution**:
1. Contact Linode support to validate your account
2. Check current quota: `linode-cli account view`
3. Request quota increase if needed

### Issue: ArgoCD Applications Stuck OutOfSync

**Symptom**: Applications show `OutOfSync` status and don't auto-sync

**Resolution**:
```bash
# Check application sync policy
kubectl get application <app> -n argocd -o yaml | grep -A 10 syncPolicy

# Manual sync
argocd app sync <app-name>

# Or enable auto-sync
kubectl patch application <app> -n argocd --type merge -p '{"spec":{"syncPolicy":{"automated":{"prune":true,"selfHeal":true}}}}'
```

### Issue: GPU Operator Pods CrashLooping

**Symptom**: nvidia-driver-daemonset or nvidia-device-plugin pods in CrashLoopBackOff

**Resolution**:
```bash
# Check driver installation logs
kubectl logs -n gpu-operator daemonset/nvidia-driver-daemonset

# Common issues:
# 1. Kernel version mismatch - update nodes
# 2. Missing kernel headers - install on nodes
# 3. Wrong GPU operator version - check compatibility matrix

# Verify GPU operator tolerations match node taints
kubectl get daemonset -n gpu-operator nvidia-driver-daemonset -o yaml | grep -A 5 tolerations
```

### Issue: cert-manager Not Issuing Certificates

**Symptom**: Certificate resources stuck in `Ready=False` state

**Resolution**:
```bash
# Check certificate status
kubectl describe certificate <cert-name> -n <namespace>

# Check cert-manager logs
kubectl logs -n cert-manager deployment/cert-manager

# Check challenges (for ACME/Let's Encrypt)
kubectl get challenges -A

# Common issues:
# 1. ClusterIssuer not configured
# 2. DNS not resolving for domain validation
# 3. Webhook not reachable
```

**Reference**: See [certificate-architecture.md](../platform/certificate-architecture.md)

### Issue: Platform Services Can't Connect to Database

**Symptom**: Pods in CrashLoopBackOff with database connection errors

**Resolution**:
```bash
# Check database secret exists
kubectl get secret database-credentials -n admin-api-service

# Verify connection string format
kubectl get secret database-credentials -n admin-api-service -o jsonpath='{.data.DATABASE_URL}' | base64 -d
# Should be: postgresql://user:pass@host:5432/dbname

# Test database connectivity from cluster
kubectl run -it --rm pg-test --image=postgres:15 --restart=Never -- \
  psql "postgresql://user:pass@host:5432/dbname" -c "SELECT version();"

# Check network policies
kubectl get networkpolicies -n admin-api-service
```

### Issue: Ingress Not Getting LoadBalancer IP

**Symptom**: Ingress shows `ADDRESS: <pending>` indefinitely

**Resolution**:
```bash
# Check ingress controller service
kubectl get svc -n istio-system istio-ingressgateway

# If LoadBalancer is pending:
# 1. Check Linode NodeBalancer quota
# 2. Check cloud-controller-manager logs
# 3. Verify service type is LoadBalancer

# Fallback: Use NodePort
kubectl patch svc istio-ingressgateway -n istio-system \
  -p '{"spec":{"type":"NodePort"}}'

# Then access via any node IP + NodePort
```

---

## Integration with Existing Scripts

This runbook integrates with several automation scripts:

| Script | Purpose | When to Use |
|--------|---------|-------------|
| `scripts/infra/provision-environment.sh` | All-in-one bootstrap | Automated bootstrap (Method 1) |
| `scripts/gitops/bootstrap_argocd.sh` | ArgoCD installation only | Manual bootstrap (Phase 2) |
| `scripts/infra/apply-gpu-node-labels.sh` | GPU node configuration | After Terraform (Phase 3) |
| `scripts/infra/setup-kubeconfigs.sh` | Kubeconfig management | Managing multiple clusters |
| `scripts/infra/generate-self-signed-certs.sh` | TLS certificate generation | Development/testing environments |
| `scripts/infra/teardown-environment.sh` | Environment cleanup | Destroying environments |

**Recommended workflow**: Use automated script for initial bootstrap, then manual steps for troubleshooting or learning.

---

## Environment-Specific Considerations

### Development Environment

**Characteristics**:
- Self-signed certificates
- Lower resource allocations
- ArgoCD auto-sync enabled
- Uses `develop` branch for ArgoCD targetRevision

**Special configuration**:
```bash
# Services accessible at: *.dev.otherjamesbrown.com

# Smaller node pools to reduce costs
baseline_node_pool = {
  instance_type = "g6-standard-4"  # 4 vCPU, 8GB RAM
  node_count    = 2
}

# Optional: No GPU nodes for pure development
gpu_node_pool = {
  node_count = 0
}
```

### Staging Environment

**Characteristics**:
- Production-like configuration
- Uses `staging` branch for ArgoCD targetRevision
- Let's Encrypt staging or self-signed certificates
- Full GPU pool for testing

**Special configuration**:
```bash
# Production-sized nodes
baseline_node_pool = {
  instance_type = "g6-standard-8"
  node_count    = 3
}

gpu_node_pool = {
  instance_type = "g2-gpu-rtx4000a1-m"
  node_count    = 2
}

# Use real domain or *.staging.ai-aas.com
```

### Production Environment

**Characteristics**:
- High availability (3+ baseline nodes)
- Uses `main` branch for ArgoCD targetRevision
- Let's Encrypt production certificates
- ArgoCD auto-sync DISABLED (manual approval required)
- Resource quotas and limits enforced
- Network policies strictly enforced

**Special configuration**:
```bash
# High availability node pools
baseline_node_pool = {
  instance_type = "g6-standard-8"
  node_count    = 5
}

gpu_node_pool = {
  instance_type = "g2-gpu-rtx4000a1-l"  # Larger GPU nodes
  node_count    = 4
}

# Production domain
# Use real domain: *.ai-aas.com
```

**Additional production requirements**:
- Backup strategy for databases
- Monitoring and alerting configured
- Incident response procedures documented
- On-call rotation established

---

## Related Documentation

| Document | Purpose |
|----------|---------|
| [agent-infra-ops-manager.md](../platform/agent-infra-ops-manager.md) | Document map and navigation |
| [new-environment-checklist.md](../platform/new-environment-checklist.md) | Comprehensive verification (30+ checks) |
| [infrastructure-overview.md](../platform/infrastructure-overview.md) | Architecture overview |
| [environment-access.md](../platform/environment-access.md) | Credentials and access |
| [endpoints-and-urls.md](../platform/endpoints-and-urls.md) | Service endpoints |
| [ci-cd-pipeline.md](../platform/ci-cd-pipeline.md) | CI/CD and deployment workflow |
| [argocd-testing-guide.md](../platform/argocd-testing-guide.md) | ArgoCD troubleshooting |
| [certificate-architecture.md](../platform/certificate-architecture.md) | Certificate management |
| [tls-ssl-setup.md](../platform/tls-ssl-setup.md) | TLS configuration |
| [observability-guide.md](../platform/observability-guide.md) | Monitoring and logging |
| [linode-setup.md](./linode-setup.md) | Linode-specific setup |
| [argocd-bootstrap.md](./argocd-bootstrap.md) | ArgoCD installation details |
| [deploy-to-environments.md](./deploy-to-environments.md) | Deployment workflow |

---

## Maintenance

**This runbook should be updated when**:
- New infrastructure components are added
- Bootstrap automation scripts change
- Terraform modules are refactored
- New environment types are introduced
- Common issues are discovered and resolved

**Last comprehensive update**: 2025-12-08

---

## Feedback and Improvements

If you encounter issues not covered in this runbook or have suggestions for improvements:

1. Create a beads issue:
   ```bash
   bd create "Bootstrap runbook: <description>" --type task --priority 2
   ```

2. Update this runbook with the resolution

3. Share knowledge with the team in Slack or team meetings

**Remember**: This runbook is a living document. Keep it accurate and up-to-date.
