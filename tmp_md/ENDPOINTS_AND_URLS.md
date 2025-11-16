# Endpoints and URLs Configuration Guide

## Overview

This document identifies all endpoints that need to be exposed with consistent URLs and SSL certificates across development and production environments.

**Access Pattern**: Local DNS (hosts file) + Self-signed SSL certificates  
**Security**: Firewall-restricted access (no VPN required)  
**DNS**: Managed via `/etc/hosts` (Linux/macOS) or `C:\Windows\System32\drivers\etc\hosts` (Windows)

## Endpoint Categories

### 1. Application Services (Microservices)

These are the core application services that need external access:

#### API Router Service (Gateway)
- **Purpose**: Main API gateway for all API requests
- **Port**: 8080 (HTTP)
- **Health**: `/v1/status/healthz`, `/v1/status/readyz`
- **API Paths**: `/api/*`, `/v1/*`
- **Current Dev URL**: `router.api.ai-aas.dev` (from values.yaml)
- **Current Prod URL**: `router.api.ai-aas.prod` (from values-production.yaml)
- **Ingress**: ✅ Configured (disabled in dev, enabled in prod)
- **TLS**: ✅ Configured with cert-manager

#### User-Org Service
- **Purpose**: User and organization management, authentication
- **Port**: 8081 (HTTP), 8443 (HTTPS admin)
- **Health**: `/healthz`
- **API Paths**: `/v1/auth/*`, `/v1/orgs/*`, `/v1/users/*`
- **Current URL**: Not exposed via ingress (internal only)
- **Ingress**: ❌ Not configured
- **TLS**: ❌ Not configured
- **Note**: Currently accessed internally or via API router

#### Analytics Service
- **Purpose**: Analytics, usage metrics, reliability data, exports
- **Port**: 8084 (HTTP)
- **Health**: `/analytics/v1/status/healthz`, `/analytics/v1/status/readyz`
- **API Paths**: `/analytics/v1/orgs/{orgId}/usage`, `/analytics/v1/orgs/{orgId}/reliability`, `/analytics/v1/orgs/{orgId}/exports`
- **Current URL**: Not exposed via ingress (internal only)
- **Ingress**: ❌ Not configured
- **TLS**: ❌ Not configured
- **Note**: Currently accessed internally or via API router

#### Web Portal (Frontend)
- **Purpose**: React/TypeScript web UI
- **Port**: 80 (HTTP), 5173 (dev)
- **Health**: `/` (root)
- **Current Dev URL**: `portal.ai-aas.dev` (from values.yaml)
- **Current Prod URL**: Not explicitly set (should mirror dev pattern)
- **Ingress**: ✅ Configured
- **TLS**: ✅ Configured with cert-manager (Let's Encrypt staging in dev)

### 2. Observability Stack

These are monitoring and observability tools that operators need to access:

#### Grafana
- **Purpose**: Dashboards, metrics visualization, log exploration
- **Port**: 3000 (HTTP)
- **Health**: `/api/health`
- **Current URL**: Not exposed via ingress (port-forward only)
- **Ingress**: ❌ Not configured
- **TLS**: ❌ Not configured
- **Access Pattern**: Currently via `kubectl port-forward` or internal cluster access
- **Recommended URLs**:
  - Dev: `grafana.dev.ai-aas.dev` or `grafana.dev.ai-aas.internal`
  - Prod: `grafana.prod.ai-aas.prod` or `grafana.prod.ai-aas.internal`
- **Note**: May want internal-only access (`.internal` domain) or VPN-only access

#### Prometheus
- **Purpose**: Metrics collection and querying
- **Port**: 9090 (HTTP)
- **Health**: `/-/healthy`
- **Current URL**: Not exposed via ingress (internal only)
- **Ingress**: ❌ Not configured
- **TLS**: ❌ Not configured
- **Access Pattern**: Typically internal-only, accessed via Grafana
- **Recommended URLs**:
  - Dev: `prometheus.dev.ai-aas.internal` (internal only)
  - Prod: `prometheus.prod.ai-aas.internal` (internal only)
- **Note**: Usually kept internal for security

#### Loki
- **Purpose**: Log aggregation
- **Port**: 3100 (HTTP)
- **Health**: `/ready`
- **Current URL**: Not exposed via ingress (internal only)
- **Ingress**: ❌ Not configured
- **TLS**: ❌ Not configured
- **Access Pattern**: Internal-only, accessed via Grafana
- **Note**: Typically kept internal

### 3. GitOps & Infrastructure

#### ArgoCD
- **Purpose**: GitOps deployment management
- **Port**: 8080 (HTTP), 443 (HTTPS)
- **Health**: `/healthz`
- **Current URL**: Not exposed via ingress (port-forward only)
- **Ingress**: ❌ Disabled in `gitops/templates/argocd-values.yaml` (`server.ingress.enabled: false`)
- **TLS**: ❌ Not configured
- **Access Pattern**: Currently via `kubectl port-forward` or `scripts/access-argocd-ui.sh`
- **Recommended URLs**:
  - Dev: `argocd.dev.ai-aas.dev` or `argocd.dev.ai-aas.internal`
  - Prod: `argocd.prod.ai-aas.prod` or `argocd.prod.ai-aas.internal`
- **Note**: Should be internal-only or VPN-protected for security

### 4. Optional/Internal Services

These may not need external URLs but are worth documenting:

#### MinIO Console
- **Purpose**: S3-compatible object storage management UI
- **Port**: 9001 (HTTP)
- **Current URL**: Not exposed via ingress (internal only)
- **Ingress**: ❌ Not configured
- **TLS**: ❌ Not configured
- **Note**: Typically kept internal

## Recommended URL Structure

### Local DNS Approach (Using /etc/hosts)

Since we're using local DNS via hosts file and self-signed certificates, we'll use a consistent domain pattern that works locally:

### Development Environment

```
# Application Services
api.dev.ai-aas.local        → API Router Service (gateway)
portal.dev.ai-aas.local     → Web Portal (frontend)

# Observability
grafana.dev.ai-aas.local    → Grafana
prometheus.dev.ai-aas.local → Prometheus (optional, usually accessed via Grafana)

# GitOps
argocd.dev.ai-aas.local     → ArgoCD
```

### Production Environment

```
# Application Services
api.prod.ai-aas.local       → API Router Service (gateway)
portal.prod.ai-aas.local    → Web Portal (frontend)

# Observability
grafana.prod.ai-aas.local   → Grafana
prometheus.prod.ai-aas.local → Prometheus (optional, usually accessed via Grafana)

# GitOps
argocd.prod.ai-aas.local    → ArgoCD
```

**Note**: Using `.local` TLD is a common convention for local development. You can use any domain you prefer (e.g., `ai-aas.dev`, `ai-aas.internal`, `localhost.localdomain`).

### Hosts File Entries

You'll need to add entries like this to your hosts file, pointing to your ingress load balancer IP:

```
# Development Environment
<INGRESS_IP>  api.dev.ai-aas.local
<INGRESS_IP>  portal.dev.ai-aas.local
<INGRESS_IP>  grafana.dev.ai-aas.local
<INGRESS_IP>  argocd.dev.ai-aas.local

# Production Environment
<INGRESS_IP>  api.prod.ai-aas.local
<INGRESS_IP>  portal.prod.ai-aas.local
<INGRESS_IP>  grafana.prod.ai-aas.local
<INGRESS_IP>  argocd.prod.ai-aas.local
```

Replace `<INGRESS_IP>` with your actual ingress controller's external IP address.

## SSL Certificate Requirements

### Self-Signed Certificates

Since we're using local DNS and firewall-restricted access, we'll use **self-signed certificates** for all endpoints:

- ✅ API Router Service (gateway)
- ✅ Web Portal
- ✅ Grafana
- ✅ ArgoCD
- ✅ Prometheus (if exposed)

### Certificate Setup

1. **Generate a self-signed CA** (Certificate Authority)
2. **Create certificates** for each domain signed by the CA
3. **Trust the CA** on your local machine(s)
4. **Configure Kubernetes secrets** with the certificates
5. **Update ingress** to use the certificate secrets

This approach provides:
- ✅ Encrypted HTTPS connections
- ✅ No public DNS required
- ✅ No VPN required (firewall handles access)
- ✅ Consistent URLs across environments
- ⚠️ Browser warnings until CA is trusted (one-time setup)

## Implementation Checklist

### Phase 1: Application Services (Priority: P1)
- [ ] Configure consistent URLs for API Router Service in dev and prod
- [ ] Configure consistent URLs for Web Portal in dev and prod
- [ ] Set up SSL certificates via cert-manager for both services
- [ ] Update environment profiles with URL configurations
- [ ] Test HTTPS redirects and certificate validity

### Phase 2: Observability (Priority: P2)
- [ ] Decide on access pattern (public vs internal-only)
- [ ] Configure Grafana ingress with TLS
- [ ] Optionally expose Prometheus (recommend internal-only)
- [ ] Set up VPN or network policies for internal access

### Phase 3: GitOps (Priority: P2)
- [ ] Configure ArgoCD ingress (recommend internal-only)
- [ ] Set up TLS certificates
- [ ] Configure authentication/authorization
- [ ] Update access scripts

### Phase 4: Additional Services (Priority: P3)
- [ ] Review if User-Org Service needs direct external access
- [ ] Review if Analytics Service needs direct external access
- [ ] Configure MinIO Console if needed (recommend internal-only)

## Local DNS Configuration (Hosts File)

### Finding Your Ingress IP

First, get your ingress controller's external IP:

```bash
# For Kubernetes
kubectl get svc -n ingress-nginx ingress-nginx-controller

# Or for Linode NodeBalancer
kubectl get svc -n ingress-nginx ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

### Updating Hosts File

#### Linux/macOS

Edit `/etc/hosts` (requires sudo):

```bash
sudo nano /etc/hosts
# or
sudo vim /etc/hosts
```

Add entries:
```
# Development Environment
<INGRESS_IP>  api.dev.ai-aas.local
<INGRESS_IP>  portal.dev.ai-aas.local
<INGRESS_IP>  grafana.dev.ai-aas.local
<INGRESS_IP>  argocd.dev.ai-aas.local

# Production Environment
<INGRESS_IP>  api.prod.ai-aas.local
<INGRESS_IP>  portal.prod.ai-aas.local
<INGRESS_IP>  grafana.prod.ai-aas.local
<INGRESS_IP>  argocd.prod.ai-aas.local
```

#### Windows

Edit `C:\Windows\System32\drivers\etc\hosts` (run Notepad as Administrator):

1. Open Notepad as Administrator
2. File → Open → `C:\Windows\System32\drivers\etc\hosts`
3. Add the same entries as above
4. Save

### Verifying DNS Resolution

```bash
# Test DNS resolution
ping api.dev.ai-aas.local
nslookup api.dev.ai-aas.local

# Test HTTPS (will show cert warning until CA is trusted)
curl -k https://api.dev.ai-aas.local/healthz
```

## Security Considerations

### Firewall-Restricted Access

Since access is restricted via firewall to your machine:
- ✅ No VPN required
- ✅ No public DNS required
- ✅ Self-signed certificates are acceptable
- ✅ All endpoints can be exposed via ingress

### Authentication

- **Grafana**: Should have authentication enabled (OAuth, LDAP, or basic auth)
- **ArgoCD**: Already has authentication (admin password + RBAC)
- **Prometheus**: Consider basic auth or network policies
- **API Router**: Uses API keys and OAuth (already implemented)

### Certificate Trust

After generating self-signed certificates:
1. **Trust the CA certificate** on your local machine(s)
2. **Browsers will show warnings** until CA is trusted
3. **One-time setup** per machine that needs access

## Next Steps

1. **Generate self-signed certificates**: Use the provided script to create CA and certificates
2. **Create Kubernetes secrets**: Store certificates as Kubernetes secrets
3. **Update hosts file**: Add entries pointing to ingress IP
4. **Update Helm values**: Configure ingress with TLS secrets
5. **Trust CA certificate**: Install CA on your local machine(s)
6. **Test**: Verify HTTPS access and certificate validity
7. **Update environment profiles**: Add URL configurations to `configs/environments/*.yaml`
8. **Document**: Update runbooks and quickstart guides with new URLs

## Implementation Scripts

See the following scripts for automation:
- `scripts/infra/generate-self-signed-certs.sh` - Generate CA and certificates
- `scripts/infra/update-hosts-file.sh` - Update hosts file with ingress IP
- `scripts/infra/create-tls-secrets.sh` - Create Kubernetes TLS secrets

## Quick Start Guide

### Step 1: Generate Certificates

```bash
# Generate CA and certificates for all endpoints
./scripts/infra/generate-self-signed-certs.sh
```

This creates:
- `infra/secrets/certs/ai-aas-ca.crt` - CA certificate (trust this)
- `infra/secrets/certs/ai-aas-ca.key` - CA private key (keep secure)
- `infra/secrets/certs/tls.crt` - TLS certificate for all domains
- `infra/secrets/certs/tls.key` - TLS private key

### Step 2: Trust CA Certificate

**Linux (Debian/Ubuntu):**
```bash
sudo cp infra/secrets/certs/ai-aas-ca.crt /usr/local/share/ca-certificates/ai-aas-ca.crt
sudo update-ca-certificates
```

**macOS:**
```bash
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain infra/secrets/certs/ai-aas-ca.crt
```

**Windows:**
1. Open `infra/secrets/certs/ai-aas-ca.crt`
2. Click "Install Certificate"
3. Choose "Local Machine" → "Place all certificates in the following store"
4. Browse → Select "Trusted Root Certification Authorities"
5. Click OK and Finish

### Step 3: Create Kubernetes Secrets

```bash
# Create TLS secret in default namespace
./scripts/infra/create-tls-secrets.sh

# Or specify namespace
./scripts/infra/create-tls-secrets.sh --namespace ingress-nginx
```

### Step 4: Update Hosts File

```bash
# Auto-detect ingress IP and update hosts file
sudo ./scripts/infra/update-hosts-file.sh

# Or specify IP manually
sudo ./scripts/infra/update-hosts-file.sh --ingress-ip 192.168.1.100
```

### Step 5: Configure Ingress

Update your Helm values to use the TLS secret:

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
  hosts:
    - host: api.dev.ai-aas.local
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: ai-aas-tls  # Secret created in step 3
      hosts:
        - api.dev.ai-aas.local
        - portal.dev.ai-aas.local
        - grafana.dev.ai-aas.local
        - argocd.dev.ai-aas.local
        # ... add all domains
```

### Step 6: Test

```bash
# Test DNS resolution
ping api.dev.ai-aas.local

# Test HTTPS (should work without -k flag after trusting CA)
curl https://api.dev.ai-aas.local/healthz

# Open in browser (should not show certificate warning)
open https://portal.dev.ai-aas.local
```

## References

- Ingress TLS Spec: `specs/013-ingress-tls/spec.md`
- Infrastructure Overview: `docs/platform/infrastructure-overview.md`
- API Router Values: `services/api-router-service/deployments/helm/api-router-service/values*.yaml`
- Web Portal Values: `web/portal/deployments/helm/web-portal/values*.yaml`
- ArgoCD Values: `gitops/templates/argocd-values.yaml`
- Component Registry: `configs/components.yaml`

