# New Environment Checklist

---
last_updated: 2025-12-08
document_type: guide
purpose: Comprehensive checklist for verifying all required components in a Kubernetes environment
---

## Purpose

This checklist ensures that all required infrastructure and platform components are properly deployed and functioning in a new or existing Kubernetes environment. Use this when:

- Provisioning a new environment from scratch
- Auditing an existing environment
- Debugging missing components
- Verifying post-migration state

## Quick Reference

| Component Category | Critical | Verification Time |
|-------------------|----------|-------------------|
| Core Infrastructure | Yes | ~5 minutes |
| Certificate Management | Yes | ~2 minutes |
| Service Mesh & Serving | Yes (GPU clusters) | ~5 minutes |
| ArgoCD & GitOps | Yes | ~3 minutes |
| Platform Services | Yes | ~5 minutes |
| Observability | Recommended | ~3 minutes |
| GPU Support | Environment-specific | ~5 minutes |

**Total verification time: ~20-30 minutes**

## Prerequisites

Before starting this checklist:

1. **Cluster Access**: Ensure you have kubeconfig for the target environment
   ```bash
   export KUBECONFIG=~/kubeconfigs/kubeconfig-<environment>.yaml
   kubectl cluster-info
   ```

2. **Required Tools**:
   - `kubectl` (Kubernetes CLI)
   - `argocd` (ArgoCD CLI) - optional but recommended
   - `helm` (Helm CLI)

3. **Documentation Reference**: Keep [environment-access.md](environment-access.md) open for credentials

## Infrastructure Components

### 1. Cluster Nodes

**Purpose**: Verify cluster has required node pools

**Verification**:
```bash
# Check all nodes are Ready
kubectl get nodes

# Verify node pools exist
kubectl get nodes --show-labels | grep -E 'node-type|instance-type'
```

**Expected State**:
- [ ] All nodes show `Ready` status
- [ ] Baseline nodes present (g6-standard-8 or similar)
- [ ] GPU nodes present if required (g1-gpu-rtx6000 or similar)
- [ ] Nodes have appropriate labels (node-type, instance-type)

**Troubleshooting**:
- If nodes are NotReady: Check node logs with `kubectl describe node <node-name>`
- If node pools missing: Check Terraform configuration in `infra/terraform/environments/<env>/`

**Related Docs**: [infrastructure-overview.md](infrastructure-overview.md)

---

### 2. Namespaces

**Purpose**: Verify required namespaces exist

**Verification**:
```bash
kubectl get namespaces
```

**Expected State**:
- [ ] `argocd` - ArgoCD control plane
- [ ] `cert-manager` - Certificate management
- [ ] `istio-system` - Service mesh
- [ ] `knative-serving` - Knative serving
- [ ] `kserve` - KServe model serving
- [ ] `gpu-operator` - GPU operator (if GPU nodes present)
- [ ] `development` or `staging` - Platform services namespace
- [ ] `admin-api-service` - Admin API namespace
- [ ] `user-org-service` - User/org service namespace
- [ ] `observability` - Monitoring stack (if deployed)

**Troubleshooting**:
- Missing namespaces are automatically created by ArgoCD if `CreateNamespace=true` is set
- Check ArgoCD Applications: `kubectl get applications -n argocd`

---

### 3. Storage Classes

**Purpose**: Verify persistent storage is available

**Verification**:
```bash
kubectl get storageclasses
```

**Expected State**:
- [ ] At least one StorageClass exists
- [ ] One StorageClass is marked as `(default)`
- [ ] StorageClass supports dynamic provisioning

**Common Storage Classes**:
- `linode-block-storage-retain` (Linode LKE)
- `linode-block-storage` (Linode LKE)

**Troubleshooting**:
- If no default: `kubectl patch storageclass <name> -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'`

---

### 4. Network Policies

**Purpose**: Verify network security policies are in place

**Verification**:
```bash
kubectl get networkpolicies -A
```

**Expected State**:
- [ ] NetworkPolicies exist in key namespaces
- [ ] Default-deny policy present (recommended)

**Source Files**:
- `gitops/clusters/<env>/apps/network/`
- Deployed via infrastructure-appset

**Troubleshooting**:
- If missing: Check ArgoCD application `network-<environment>`
- Check sync status: `kubectl get application network-<environment> -n argocd -o yaml`

---

## Certificate Management

### 5. cert-manager

**Purpose**: TLS certificate automation

**Verification**:
```bash
# Check cert-manager pods
kubectl get pods -n cert-manager

# Check cert-manager CRDs
kubectl get crd | grep cert-manager

# Check ClusterIssuers
kubectl get clusterissuers -A
```

**Expected State**:
- [ ] cert-manager deployment running (3/3 pods ready)
- [ ] cert-manager webhook running (1/1 pods ready)
- [ ] cert-manager cainjector running (1/1 pods ready)
- [ ] CRDs installed: `certificates.cert-manager.io`, `issuers.cert-manager.io`, etc.
- [ ] ClusterIssuers configured (letsencrypt-prod, letsencrypt-staging, or selfsigned)

**ArgoCD Application**: `cert-manager-<environment>` or via infrastructure chart

**Troubleshooting**:
- Webhook issues: Check webhook service and certificate
- Check logs: `kubectl logs -n cert-manager deployment/cert-manager`
- Verify webhook connectivity: `kubectl get validatingwebhookconfigurations`

**Related Docs**: [certificate-architecture.md](certificate-architecture.md), [tls-ssl-setup.md](tls-ssl-setup.md)

---

### 6. Certificates

**Purpose**: Verify certificates are issued and valid

**Verification**:
```bash
# Check all certificates
kubectl get certificates -A

# Check certificate status
kubectl describe certificate <cert-name> -n <namespace>
```

**Expected State**:
- [ ] All certificates show `Ready=True`
- [ ] No recent IssueFailed events
- [ ] Certificates valid for required domains

**Common Certificates**:
- Ingress TLS certificates
- KServe webhook certificates
- Knative serving certificates

**Troubleshooting**:
- If certificate stuck: Check cert-manager logs and ClusterIssuer status
- Check certificate challenges: `kubectl get challenges -A`
- For self-signed certs: Check `infra/secrets/certs/`

---

## Service Mesh & Serving Platform

### 7. Istio

**Purpose**: Service mesh for advanced networking

**Verification**:
```bash
# Check Istio components
kubectl get pods -n istio-system

# Check Istio CRDs
kubectl get crd | grep istio

# Verify Istio is injecting sidecars (if auto-injection enabled)
kubectl get namespace -L istio-injection
```

**Expected State**:
- [ ] istiod deployment running (1/1 pods ready)
- [ ] istio-ingressgateway service present (LoadBalancer or NodePort)
- [ ] Istio CRDs installed: `virtualservices.networking.istio.io`, etc.

**ArgoCD Application**: `istio-<environment>`

**Source Files**: `infra/k8s/istio/` or Helm chart

**Troubleshooting**:
- Check istiod logs: `kubectl logs -n istio-system deployment/istiod`
- Verify ingress gateway: `kubectl get svc -n istio-system istio-ingressgateway`
- Check for LoadBalancer IP: May take 1-2 minutes to provision

**Related Docs**: [infrastructure-overview.md](infrastructure-overview.md)

---

### 8. Knative Serving

**Purpose**: Serverless container platform (required for KServe)

**Verification**:
```bash
# Check Knative Serving components
kubectl get pods -n knative-serving

# Check Knative CRDs
kubectl get crd | grep knative

# Check Knative configuration
kubectl get cm config-domain -n knative-serving -o yaml
```

**Expected State**:
- [ ] activator deployment running
- [ ] autoscaler deployment running
- [ ] controller deployment running
- [ ] webhook deployment running
- [ ] Knative CRDs installed: `services.serving.knative.dev`, etc.
- [ ] Domain configuration matches environment

**ArgoCD Application**: `knative-serving-<environment>`

**Source Files**: `infra/k8s/knative-serving/`

**Troubleshooting**:
- Webhook issues: Check webhook certificates and service
- Check controller logs: `kubectl logs -n knative-serving deployment/controller`
- Verify net-istio integration: `kubectl get deployment net-istio-controller -n knative-serving`

**Related Docs**: [infrastructure-overview.md](infrastructure-overview.md)

---

### 9. KServe

**Purpose**: Model serving framework (built on Knative)

**Verification**:
```bash
# Check KServe controller
kubectl get pods -n kserve

# Check KServe CRDs
kubectl get crd | grep kserve

# Check InferenceServices (if any deployed)
kubectl get inferenceservices -A
```

**Expected State**:
- [ ] kserve-controller-manager running (1/1 pods ready)
- [ ] KServe CRDs installed: `inferenceservices.serving.kserve.io`, etc.
- [ ] Webhook configured and accessible

**ArgoCD Application**: `kserve-<environment>`

**Source Files**: `infra/k8s/kserve/install/`

**Troubleshooting**:
- Webhook TLS issues: Check certificate in kserve namespace
- Check controller logs: `kubectl logs -n kserve deployment/kserve-controller-manager`
- Verify Knative dependency: Knative must be healthy first

**Related Docs**: [infrastructure-overview.md](infrastructure-overview.md)

---

## ArgoCD & GitOps

### 10. ArgoCD Installation

**Purpose**: GitOps continuous delivery

**Verification**:
```bash
# Check ArgoCD components
kubectl get pods -n argocd

# Check ArgoCD ingress/service
kubectl get ingress -n argocd
kubectl get svc argocd-server -n argocd

# Test ArgoCD API (if ingress configured)
curl -k https://argocd.<environment>.otherjamesbrown.com/healthz
```

**Expected State**:
- [ ] argocd-server running (1/1 pods ready)
- [ ] argocd-repo-server running (1/1 pods ready)
- [ ] argocd-application-controller running (1/1 pods ready)
- [ ] argocd-redis running (1/1 pods ready)
- [ ] Ingress configured (if external access required)

**Installation**: Via `scripts/infra/provision-environment.sh` or manual install

**Credentials**:
```bash
# Get initial admin password
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d
```

**Troubleshooting**:
- Check ArgoCD logs: `kubectl logs -n argocd deployment/argocd-server`
- Verify git repository access from cluster
- Check RBAC permissions

**Related Docs**: [environment-access.md](environment-access.md), [ci-cd-pipeline.md](ci-cd-pipeline.md)

---

### 11. ArgoCD Projects

**Purpose**: RBAC boundaries for applications

**Verification**:
```bash
# Check AppProjects
kubectl get appprojects -n argocd

# Describe project
kubectl describe appproject platform-<environment> -n argocd
```

**Expected State**:
- [ ] `platform-<environment>` project exists
- [ ] Project allows required source repositories
- [ ] Project allows required destination namespaces
- [ ] Project allows required cluster resources

**Source Files**: `gitops/clusters/<env>/projects/platform-project.yaml`

**Troubleshooting**:
- If applications show "permission denied": Check AppProject's `destinations` and `sourceRepos`
- Verify project exists before creating applications

**Related Docs**: [ci-cd-pipeline.md](ci-cd-pipeline.md)

---

### 12. ArgoCD Applications

**Purpose**: GitOps application definitions

**Verification**:
```bash
# List all applications
kubectl get applications -n argocd

# Check application health
kubectl get applications -n argocd -o wide

# Detailed application status
argocd app list  # Requires argocd CLI and login
```

**Expected State**:
- [ ] Core infrastructure apps present: cert-manager, istio, knative-serving, kserve
- [ ] Platform service apps present: admin-api, user-org-service, api-router
- [ ] All applications show Healthy status
- [ ] All applications show Synced status
- [ ] Applications reference correct targetRevision (`develop`, `staging`, or `main`)

**Source Files**: `gitops/clusters/<env>/apps/`

**Key Applications**:
- Infrastructure: cert-manager, istio, knative-serving, kserve, gpu-operator
- Platform: admin-api-service, user-org-service, api-router-service, analytics-service
- Monitoring: grafana, loki, promtail (if deployed)
- Web: web-portal

**Troubleshooting**:
- Out of sync: `kubectl get application <app-name> -n argocd -o yaml` - check sync status
- Failed sync: Check application events and logs
- Manual sync: `argocd app sync <app-name>`
- Check target branch matches environment (develop→development, staging→staging, main→production)

**Related Docs**: [argocd-testing-guide.md](argocd-testing-guide.md), [ci-cd-pipeline.md](ci-cd-pipeline.md)

---

## Platform Services

### 13. Admin API Service

**Purpose**: Primary administrative API

**Verification**:
```bash
# Check deployment
kubectl get deployment admin-api-service -n admin-api-service

# Check pods
kubectl get pods -n admin-api-service

# Check service
kubectl get svc admin-api-service -n admin-api-service

# Test health endpoint
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://admin-api-service.admin-api-service.svc.cluster.local:8080/healthz
```

**Expected State**:
- [ ] Deployment running (2/2 replicas ready)
- [ ] Pods in Running state
- [ ] Service exists on port 8080
- [ ] Health endpoint returns 200 OK
- [ ] Readiness probe passing

**ArgoCD Application**: `admin-api-service-<environment>`

**Source Files**:
- Helm: `services/admin-api-service/deployments/helm/admin-api-service/`
- Deployment spec: `services/admin-api-service/DEPLOYMENT.md`

**Dependencies**:
- PostgreSQL database (requires `DATABASE_URL`)
- user-org-service (requires `USER_ORG_SERVICE_URL`)

**Troubleshooting**:
- Check pod logs: `kubectl logs -n admin-api-service deployment/admin-api-service`
- Verify database connectivity
- Check secrets exist: `kubectl get secret admin-api-secrets -n admin-api-service`

**Related Docs**: [endpoints-and-urls.md](endpoints-and-urls.md)

---

### 14. User-Org Service

**Purpose**: User and organization management

**Verification**:
```bash
# Check deployment
kubectl get deployment user-org-service -n user-org-service

# Check pods
kubectl get pods -n user-org-service

# Test health endpoint
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://user-org-service.user-org-service.svc.cluster.local:8081/healthz
```

**Expected State**:
- [ ] Deployment running (2/2 replicas ready)
- [ ] Pods in Running state
- [ ] Service exists on port 8081
- [ ] Health endpoint returns 200 OK

**ArgoCD Application**: `user-org-service-<environment>`

**Source Files**:
- Helm: `services/user-org-service/deployments/helm/user-org-service/`
- Deployment spec: `services/user-org-service/DEPLOYMENT.md`

**Troubleshooting**:
- Check pod logs: `kubectl logs -n user-org-service deployment/user-org-service`
- Verify database connectivity

---

### 15. API Router Service

**Purpose**: API gateway and routing

**Verification**:
```bash
# Check deployment
kubectl get deployment api-router-service -n development

# Check pods
kubectl get pods -n development -l app=api-router-service

# Check ingress
kubectl get ingress -n development | grep api-router

# Test health endpoint
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://api-router-service.development.svc.cluster.local:8080/health
```

**Expected State**:
- [ ] Deployment running (replicas ready)
- [ ] Pods in Running state
- [ ] Ingress configured (if external access required)
- [ ] Health endpoint returns 200 OK

**ArgoCD Application**: `api-router-service-<environment>`

**Source Files**:
- Helm: `services/api-router-service/deployments/helm/api-router-service/`
- Deployment spec: `services/api-router-service/DEPLOYMENT.md`

**Backend Configuration**: Check `values-<environment>.yaml` for backend endpoints

**Troubleshooting**:
- Check backend connectivity from pod
- Verify backend service DNS resolution
- Check ingress controller logs

**Related Docs**: [endpoints-and-urls.md](endpoints-and-urls.md)

---

### 16. Analytics Service

**Purpose**: Analytics and metrics aggregation

**Verification**:
```bash
# Check deployment
kubectl get deployment analytics-service -n development

# Check pods
kubectl get pods -n development -l app=analytics-service

# Test health endpoint
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://analytics-service.development.svc.cluster.local:8080/health
```

**Expected State**:
- [ ] Deployment running (replicas ready)
- [ ] Pods in Running state
- [ ] Health endpoint returns 200 OK

**ArgoCD Application**: `analytics-service-<environment>`

**Source Files**:
- Helm: `services/analytics-service/deployments/helm/analytics-service/`
- Deployment spec: `services/analytics-service/DEPLOYMENT.md`

---

### 17. Web Portal

**Purpose**: Web user interface

**Verification**:
```bash
# Check deployment
kubectl get deployment web-portal -n development

# Check pods
kubectl get pods -n development -l app=web-portal

# Check ingress
kubectl get ingress -n development | grep web-portal
```

**Expected State**:
- [ ] Deployment running (replicas ready)
- [ ] Pods in Running state
- [ ] Ingress configured
- [ ] Static assets served correctly

**ArgoCD Application**: `web-portal-<environment>`

**Source Files**: `web/portal/deployments/helm/web-portal/`

**Troubleshooting**:
- Check Nginx logs: `kubectl logs -n development deployment/web-portal`
- Verify ingress path routing
- Check static file mounts

---

## Observability (Recommended)

### 18. Prometheus

**Purpose**: Metrics collection and alerting

**Verification**:
```bash
# Check Prometheus components
kubectl get pods -n observability | grep prometheus

# Check ServiceMonitors
kubectl get servicemonitors -A

# Check PrometheusRules (alerts)
kubectl get prometheusrules -A
```

**Expected State**:
- [ ] prometheus-operator running
- [ ] prometheus pods running (prometheus-kube-prometheus-stack-prometheus-0)
- [ ] alertmanager running
- [ ] ServiceMonitors discovering targets

**ArgoCD Application**: Part of monitoring stack or kube-prometheus-stack

**Access**:
```bash
kubectl port-forward -n observability svc/prometheus-kube-prometheus-stack-prometheus 9090:9090
# Open http://localhost:9090
```

**Troubleshooting**:
- Check Prometheus targets: Access UI → Status → Targets
- Check ServiceMonitor labels match Prometheus selector
- Check alerting rules: `kubectl get prometheusrules -A`

**Related Docs**: [observability-guide.md](observability-guide.md)

---

### 19. Grafana

**Purpose**: Metrics visualization

**Verification**:
```bash
# Check Grafana deployment
kubectl get deployment -n observability | grep grafana

# Check Grafana service
kubectl get svc -n observability | grep grafana
```

**Expected State**:
- [ ] grafana deployment running (1/1 pods ready)
- [ ] Service accessible

**ArgoCD Application**: `grafana-<environment>`

**Access**:
```bash
kubectl port-forward -n observability svc/grafana 3000:80
# Open http://localhost:3000
```

**Credentials**: Check `environment-access.md` or Grafana secret

**Troubleshooting**:
- Check datasource configuration: Prometheus should be pre-configured
- Verify dashboards are loaded
- Check persistent volume if dashboards missing

**Related Docs**: [observability-guide.md](observability-guide.md)

---

### 20. Loki

**Purpose**: Log aggregation

**Verification**:
```bash
# Check Loki components
kubectl get pods -n observability | grep loki

# Check Loki service
kubectl get svc -n observability | grep loki
```

**Expected State**:
- [ ] loki pods running
- [ ] Loki service accessible on port 3100

**ArgoCD Application**: `loki-<environment>`

**Verification Query**:
```bash
# From Grafana Explore, query Loki
{namespace="development"}
```

**Troubleshooting**:
- Check Loki is configured in Grafana datasources
- Verify Promtail is shipping logs (see next section)
- Check storage: Loki requires persistent volume or object storage

**Related Docs**: [observability-guide.md](observability-guide.md)

---

### 21. Promtail

**Purpose**: Log shipper for Loki

**Verification**:
```bash
# Check Promtail daemonset
kubectl get daemonset -n observability promtail

# Check Promtail pods (should be 1 per node)
kubectl get pods -n observability -l app.kubernetes.io/name=promtail
```

**Expected State**:
- [ ] Promtail daemonset running on all nodes
- [ ] Number of ready pods equals number of nodes

**ArgoCD Application**: `promtail-<environment>`

**Troubleshooting**:
- Check Promtail logs: `kubectl logs -n observability daemonset/promtail`
- Verify Promtail can reach Loki service
- Check log paths in Promtail config

**Related Docs**: [observability-guide.md](observability-guide.md)

---

## GPU Support (Environment-Specific)

### 22. GPU Operator

**Purpose**: NVIDIA GPU driver and device plugin management

**Verification**:
```bash
# Check GPU operator
kubectl get pods -n gpu-operator

# Check GPU nodes
kubectl get nodes -l nvidia.com/gpu.present=true

# Verify GPU capacity
kubectl describe nodes -l nvidia.com/gpu.present=true | grep -A 5 "Capacity:"
```

**Expected State**:
- [ ] GPU operator pods running (nvidia-driver-daemonset, nvidia-device-plugin, etc.)
- [ ] GPU nodes show `nvidia.com/gpu` capacity
- [ ] GPU nodes are schedulable

**ArgoCD Application**: `gpu-operator-<environment>`

**Source Files**: `gitops/clusters/<env>/apps/gpu-operator.yaml`

**Node Taints**: GPU nodes may have taints requiring tolerations:
- `nvidia.com/gpu=true:NoSchedule`
- `gpu-workload=true:NoSchedule`

**Troubleshooting**:
- Check driver installation: `kubectl logs -n gpu-operator daemonset/nvidia-driver-daemonset`
- Verify device plugin: `kubectl logs -n gpu-operator daemonset/nvidia-device-plugin-daemonset`
- Test GPU allocation: Deploy a test pod requesting `nvidia.com/gpu: 1`

**Related Docs**: [infrastructure-overview.md](infrastructure-overview.md)

---

### 23. GPU Node Labels

**Purpose**: Proper node scheduling for GPU workloads

**Verification**:
```bash
# Check GPU node labels
kubectl get nodes -l node-type=gpu --show-labels

# Verify expected labels
kubectl get nodes -l node-type=gpu -o json | \
  jq -r '.items[].metadata.labels | to_entries | .[] | select(.key | startswith("nvidia")) | "\(.key)=\(.value)"'
```

**Expected Labels** on GPU nodes:
- [ ] `node-type=gpu`
- [ ] `nvidia.com/gpu.present=true`
- [ ] `nvidia.com/gpu.family=<family>` (e.g., rtx6000)
- [ ] `nvidia.com/gpu.count=<number>`

**Apply Labels** (if missing):
```bash
# Use provided script
scripts/infra/apply-gpu-node-labels.sh
```

**Troubleshooting**:
- Labels missing: Run `scripts/infra/apply-gpu-node-labels.sh`
- NFD (Node Feature Discovery) should auto-detect GPU features
- Check GPU operator logs for label application

---

## Database & External Dependencies

### 24. PostgreSQL Database

**Purpose**: Primary data store for platform services

**Verification**:
```bash
# From a pod or locally, test connection
# Using DATABASE_URL from secrets/env/.env

psql $DATABASE_URL -c "SELECT version();"
```

**Expected State**:
- [ ] Database server accessible
- [ ] Database credentials valid
- [ ] Required databases exist (admin_api_db, user_org_db, etc.)
- [ ] Migrations completed (check service logs)

**Connection Strings**: Located in `secrets/env/.env` as `DATABASE_URL`

**Troubleshooting**:
- Connection refused: Check network policies, firewall rules
- Authentication failed: Verify credentials in secrets
- Database not found: Check database creation scripts or Terraform outputs

**Related Docs**: [environment-access.md](environment-access.md)

---

### 25. Redis (If Used)

**Purpose**: Caching and session storage

**Verification**:
```bash
# Check Redis deployment/pod
kubectl get pods -A | grep redis

# Test Redis connectivity
kubectl run -it --rm redis-test --image=redis:7 --restart=Never -- \
  redis-cli -h <redis-service-host> ping
```

**Expected State**:
- [ ] Redis pod running
- [ ] Redis service accessible
- [ ] Responds to PING with PONG

**Troubleshooting**:
- Connection issues: Check service DNS and network policies
- Authentication: Verify Redis password if configured

---

## Ingress & External Access

### 26. Ingress Controller

**Purpose**: HTTP/HTTPS routing from external traffic

**Verification**:
```bash
# Check ingress controller (may be Istio ingress gateway or NGINX)
kubectl get svc -A | grep -E "ingress|gateway"

# Check LoadBalancer IP
kubectl get svc -n istio-system istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

**Expected State**:
- [ ] Ingress controller service running
- [ ] LoadBalancer has external IP assigned
- [ ] Ingress controller pods healthy

**Common Controllers**:
- Istio Ingress Gateway (`istio-ingressgateway` in `istio-system`)
- NGINX Ingress Controller (`ingress-nginx` in `ingress-nginx`)

**Troubleshooting**:
- No external IP: Check cloud provider LoadBalancer service
- Traffic not routing: Check Ingress resource configurations
- TLS issues: Verify certificates attached to Ingress

**Related Docs**: [endpoints-and-urls.md](endpoints-and-urls.md)

---

### 27. Ingress Resources

**Purpose**: HTTP routing rules

**Verification**:
```bash
# List all ingress resources
kubectl get ingress -A

# Describe specific ingress
kubectl describe ingress <ingress-name> -n <namespace>
```

**Expected State**:
- [ ] Ingress resources created for exposed services
- [ ] Ingress has ADDRESS assigned (LoadBalancer IP or hostname)
- [ ] TLS configured for HTTPS ingresses
- [ ] Backend services referenced correctly

**Common Ingresses**:
- `api-router-ingress` - API Router external access
- `argocd-ingress` - ArgoCD UI
- `web-portal-ingress` - Web UI

**Troubleshooting**:
- No ADDRESS: Wait for LoadBalancer provisioning (~1-2 minutes)
- 404 errors: Check backend service and path configuration
- TLS errors: Verify certificate is Ready and matches host

**Related Docs**: [endpoints-and-urls.md](endpoints-and-urls.md)

---

## Configuration Verification

### 28. ConfigMaps

**Purpose**: Non-sensitive configuration data

**Verification**:
```bash
# List ConfigMaps in key namespaces
kubectl get configmaps -n development
kubectl get configmaps -n admin-api-service
kubectl get configmaps -n knative-serving

# Check specific ConfigMap
kubectl get configmap <name> -n <namespace> -o yaml
```

**Expected State**:
- [ ] Service-specific ConfigMaps present
- [ ] Knative domain configuration correct (`config-domain`)
- [ ] Observability configs present (if deployed)

**Troubleshooting**:
- Missing ConfigMaps: Check ArgoCD sync or Helm chart
- Incorrect values: Compare with Helm chart `values-<environment>.yaml`

---

### 29. Secrets

**Purpose**: Sensitive configuration data

**Verification**:
```bash
# List secrets (without showing values)
kubectl get secrets -n admin-api-service
kubectl get secrets -n user-org-service
kubectl get secrets -n argocd

# Verify secret has required keys
kubectl get secret admin-api-secrets -n admin-api-service -o jsonpath='{.data}' | jq 'keys'
```

**Expected State**:
- [ ] Service secrets present (admin-api-secrets, user-org-secrets, etc.)
- [ ] Database credentials stored
- [ ] API keys stored
- [ ] TLS secrets for ingresses (if using cert-manager)

**Security Note**: Never output secret values directly. Use `kubectl get secret <name> -o jsonpath='{.data.<key>}' | base64 -d` only when needed.

**Troubleshooting**:
- Missing secrets: Check if created imperatively or via Sealed Secrets
- Secret not mounted: Check pod volumeMounts and volumes sections

**Related Docs**: [environment-access.md](environment-access.md)

---

## Final Verification

### 30. Overall System Health

**Purpose**: Comprehensive health check

**Verification**:
```bash
# All pods should be running or completed
kubectl get pods -A | grep -v -E "Running|Completed"

# No stuck PVCs
kubectl get pvc -A | grep Pending

# No CrashLoopBackOff or ImagePullBackOff
kubectl get pods -A | grep -E "CrashLoop|ImagePull|Error"

# Check events for errors
kubectl get events -A --sort-by='.lastTimestamp' | tail -50
```

**Expected State**:
- [ ] All pods in Running or Completed state
- [ ] No persistent volume claims stuck in Pending
- [ ] No pods in CrashLoopBackOff or ImagePullBackOff
- [ ] No critical errors in recent events
- [ ] All ArgoCD applications Healthy and Synced

**Automated Check Script**:
```bash
#!/bin/bash
# Quick health check script

echo "=== Nodes ==="
kubectl get nodes

echo -e "\n=== Failed Pods ==="
kubectl get pods -A | grep -v -E "Running|Completed" || echo "None"

echo -e "\n=== ArgoCD Applications ==="
kubectl get applications -n argocd -o wide

echo -e "\n=== Recent Events (Errors) ==="
kubectl get events -A --field-selector type=Warning --sort-by='.lastTimestamp' | tail -20

echo -e "\n=== Ingress Status ==="
kubectl get ingress -A

echo -e "\n=== PVC Status ==="
kubectl get pvc -A
```

---

## Environment-Specific Considerations

### Development Environment

**Additional Checks**:
- [ ] Self-signed certificates trusted locally (if testing from laptop)
- [ ] Hosts file configured for `*.otherjamesbrown.com` domains
- [ ] ArgoCD auto-sync enabled (faster iteration)
- [ ] Resource requests/limits lower (cost optimization)

**Source Branch**: ArgoCD applications should target `develop` branch

**Related Docs**: [tls-ssl-setup.md](tls-ssl-setup.md)

---

### Staging Environment

**Additional Checks**:
- [ ] Production-like resource allocations
- [ ] GPU nodes available (if testing GPU workloads)
- [ ] Separate database from development
- [ ] TLS certificates valid (Let's Encrypt staging or prod)
- [ ] Network policies enforced

**Source Branch**: ArgoCD applications should target `staging` branch

**Related Docs**: [ci-cd-pipeline.md](ci-cd-pipeline.md)

---

### Production Environment

**Additional Checks**:
- [ ] High availability enabled (multiple replicas)
- [ ] Autoscaling configured for key services
- [ ] Backup strategy in place for databases
- [ ] Monitoring and alerting active
- [ ] ArgoCD auto-sync DISABLED (manual approvals)
- [ ] Production TLS certificates (Let's Encrypt prod)
- [ ] Network policies strictly enforced
- [ ] Pod security policies/standards enforced
- [ ] Resource quotas defined per namespace

**Source Branch**: ArgoCD applications should target `main` branch

**Related Docs**: [ci-cd-pipeline.md](ci-cd-pipeline.md), [observability-guide.md](observability-guide.md)

---

## Troubleshooting Guide

### Common Issues

#### 1. Pods Not Starting

**Symptoms**: Pods in Pending, CrashLoopBackOff, or ImagePullBackOff

**Steps**:
1. Check pod events: `kubectl describe pod <pod-name> -n <namespace>`
2. Check logs: `kubectl logs <pod-name> -n <namespace>`
3. Verify resource availability: `kubectl describe node <node-name>`
4. Check image pull secrets if private registry

#### 2. ArgoCD Application Out of Sync

**Symptoms**: Application shows "OutOfSync" status

**Steps**:
1. Check application diff: `argocd app diff <app-name>`
2. Check target branch: Ensure it matches environment (develop/staging/main)
3. Manual sync: `argocd app sync <app-name>`
4. Check application events: `kubectl describe application <app-name> -n argocd`

#### 3. Services Not Accessible

**Symptoms**: Cannot reach service endpoints

**Steps**:
1. Verify service exists: `kubectl get svc <service-name> -n <namespace>`
2. Check endpoints: `kubectl get endpoints <service-name> -n <namespace>`
3. Test from within cluster: `kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- curl <service-url>`
4. Check network policies: `kubectl get networkpolicies -n <namespace>`
5. Check ingress: `kubectl get ingress -n <namespace>`

#### 4. Certificate Issues

**Symptoms**: TLS errors, certificate not ready

**Steps**:
1. Check certificate status: `kubectl describe certificate <cert-name> -n <namespace>`
2. Check cert-manager logs: `kubectl logs -n cert-manager deployment/cert-manager`
3. Check challenges: `kubectl get challenges -A`
4. Verify ClusterIssuer: `kubectl get clusterissuer`

#### 5. GPU Not Available

**Symptoms**: Pods can't schedule on GPU nodes

**Steps**:
1. Check GPU operator: `kubectl get pods -n gpu-operator`
2. Verify GPU capacity: `kubectl describe node <gpu-node>`
3. Check node taints: Ensure pod has required tolerations
4. Check node labels: `kubectl get nodes -l node-type=gpu --show-labels`

---

## Automation Script

Save this as `scripts/infra/verify-environment.sh`:

```bash
#!/bin/bash
# Automated environment verification script
# Usage: ./scripts/infra/verify-environment.sh <environment>

set -euo pipefail

ENVIRONMENT="${1:-development}"
KUBECONFIG="${KUBECONFIG:-$HOME/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml}"

echo "Verifying environment: $ENVIRONMENT"
echo "Using kubeconfig: $KUBECONFIG"
export KUBECONFIG

# Run all verification checks
# (Implement checks from this checklist)

echo "✓ Environment verification complete"
```

---

## Related Documentation

| Document | Purpose |
|----------|---------|
| [agent-infra-ops-manager.md](agent-infra-ops-manager.md) | Document map and navigation |
| [infrastructure-overview.md](infrastructure-overview.md) | Architecture overview |
| [environment-access.md](environment-access.md) | Credentials and access |
| [endpoints-and-urls.md](endpoints-and-urls.md) | Service endpoints |
| [ci-cd-pipeline.md](ci-cd-pipeline.md) | CI/CD and deployment |
| [argocd-testing-guide.md](argocd-testing-guide.md) | ArgoCD troubleshooting |
| [certificate-architecture.md](certificate-architecture.md) | Certificate management |
| [observability-guide.md](observability-guide.md) | Monitoring and logging |

---

## Maintenance

**This document should be updated when**:
- New infrastructure components are added to the platform
- Component verification commands change
- New environment types are introduced
- Critical dependencies are added or removed

**Last comprehensive audit**: 2025-12-08
