# Ingress Configuration Summary

## ✅ Configured Services

### 1. API Router Service
**File**: `services/api-router-service/deployments/helm/api-router-service/values.yaml`

- ✅ Ingress enabled
- ✅ URL: `api.dev.ai-aas.local`
- ✅ TLS configured with `ai-aas-tls` secret
- ✅ HTTPS redirect enabled

**To apply**:
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
helm upgrade --install api-router-service ./services/api-router-service/deployments/helm/api-router-service \
  --namespace default \
  -f ./services/api-router-service/deployments/helm/api-router-service/values.yaml
```

### 2. Web Portal
**File**: `web/portal/deployments/helm/web-portal/values.yaml`

- ✅ Ingress enabled
- ✅ URL: `portal.dev.ai-aas.local`
- ✅ TLS configured with `ai-aas-tls` secret
- ✅ HTTPS redirect enabled
- ✅ Environment variables updated to use HTTPS URLs

**To apply**:
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
helm upgrade --install web-portal ./web/portal/deployments/helm/web-portal \
  --namespace default \
  -f ./web/portal/deployments/helm/web-portal/values.yaml
```

### 3. ArgoCD
**File**: `gitops/templates/argocd-values.yaml`

- ✅ Ingress enabled
- ✅ URL: `argocd.dev.ai-aas.local`
- ✅ TLS configured with `ai-aas-tls` secret
- ✅ SSL passthrough enabled (ArgoCD uses gRPC)

**To apply**:
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
helm upgrade --install argocd argo/argo-cd \
  --namespace argocd \
  -f ./gitops/templates/argocd-values.yaml
```

### 4. Grafana (Ready to Apply)
**File**: `infra/helm/charts/grafana-ingress.yaml`

- ✅ Ingress manifest created
- ✅ URL: `grafana.dev.ai-aas.local`
- ✅ TLS configured with `ai-aas-tls` secret
- ⚠️ Requires Grafana service to exist first

**To apply** (after Grafana is deployed):
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl apply -f ./infra/helm/charts/grafana-ingress.yaml
```

### 5. Prometheus (Optional)
**File**: `infra/helm/charts/prometheus-ingress.yaml`

- ✅ Ingress manifest created
- ✅ URL: `prometheus.dev.ai-aas.local`
- ✅ TLS configured with `ai-aas-tls` secret
- ⚠️ Basic auth recommended (configure auth secret)
- ⚠️ Usually accessed via Grafana (optional direct access)

**To apply** (if you want direct Prometheus access):
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
# Create basic auth secret first (optional)
kubectl create secret generic prometheus-basic-auth \
  --from-literal=auth=$(echo -n 'admin:password' | base64) \
  --namespace default

kubectl apply -f ./infra/helm/charts/prometheus-ingress.yaml
```

## 📋 All Configured URLs

### Development Environment
- `https://api.dev.ai-aas.local` - API Router Service
- `https://portal.dev.ai-aas.local` - Web Portal
- `https://argocd.dev.ai-aas.local` - ArgoCD
- `https://grafana.dev.ai-aas.local` - Grafana (when deployed)
- `https://prometheus.dev.ai-aas.local` - Prometheus (optional)

### Production Environment
- `https://api.prod.ai-aas.local` - API Router Service
- `https://portal.prod.ai-aas.local` - Web Portal
- `https://argocd.prod.ai-aas.local` - ArgoCD
- `https://grafana.prod.ai-aas.local` - Grafana (when deployed)
- `https://prometheus.prod.ai-aas.local` - Prometheus (optional)

## 🔐 TLS Configuration

All services use the same TLS secret: `ai-aas-tls` in namespace `ingress-nginx`

**Secret contains**:
- Certificate covering all `*.ai-aas.local` domains
- Valid for both dev and prod environments

## 📝 Next Steps

1. **Update hosts file** (if not done):
   ```bash
   sudo ./scripts/infra/update-hosts-file.sh --ingress-ip 172.232.58.222
   ```

2. **Trust CA certificate** (if not done):
   ```bash
   sudo cp infra/secrets/certs/ai-aas-ca.crt /usr/local/share/ca-certificates/ai-aas-ca.crt
   sudo update-ca-certificates
   ```

3. **Deploy services** with updated Helm values:
   ```bash
   export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
   
   # Deploy API Router
   helm upgrade --install api-router-service ./services/api-router-service/deployments/helm/api-router-service \
     --namespace default \
     -f ./services/api-router-service/deployments/helm/api-router-service/values.yaml
   
   # Deploy Web Portal
   helm upgrade --install web-portal ./web/portal/deployments/helm/web-portal \
     --namespace default \
     -f ./web/portal/deployments/helm/web-portal/values.yaml
   
   # Update ArgoCD (if already installed)
   helm upgrade argocd argo/argo-cd \
     --namespace argocd \
     -f ./gitops/templates/argocd-values.yaml
   ```

4. **Apply Grafana ingress** (when Grafana is deployed):
   ```bash
   kubectl apply -f ./infra/helm/charts/grafana-ingress.yaml
   ```

5. **Test endpoints**:
   ```bash
   curl https://api.dev.ai-aas.local/healthz
   curl https://portal.dev.ai-aas.local
   # Open in browser: https://argocd.dev.ai-aas.local
   ```

## 🔍 Verification

Check ingress resources:
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl get ingress --all-namespaces
```

Check TLS secret:
```bash
kubectl get secret ai-aas-tls -n ingress-nginx
```

Test DNS resolution:
```bash
ping api.dev.ai-aas.local
nslookup api.dev.ai-aas.local
```

