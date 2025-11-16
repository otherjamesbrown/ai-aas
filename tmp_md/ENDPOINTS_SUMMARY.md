# Endpoints Summary - Quick Reference

## All Endpoints Requiring URLs and SSL Certificates

### Development Environment

| Service | URL | Purpose | Port | Status |
|---------|-----|---------|------|--------|
| **API Router** | `api.dev.ai-aas.local` | API Gateway | 8080 | ⚠️ Needs ingress config |
| **Web Portal** | `portal.dev.ai-aas.local` | Frontend UI | 80 | ⚠️ Needs URL update |
| **Grafana** | `grafana.dev.ai-aas.local` | Dashboards/Metrics | 3000 | ❌ Not configured |
| **ArgoCD** | `argocd.dev.ai-aas.local` | GitOps | 8080/443 | ❌ Not configured |
| **Prometheus** | `prometheus.dev.ai-aas.local` | Metrics (optional) | 9090 | ❌ Not configured |

### Production Environment

| Service | URL | Purpose | Port | Status |
|---------|-----|---------|------|--------|
| **API Router** | `api.prod.ai-aas.local` | API Gateway | 8080 | ⚠️ Needs ingress config |
| **Web Portal** | `portal.prod.ai-aas.local` | Frontend UI | 80 | ❌ Not configured |
| **Grafana** | `grafana.prod.ai-aas.local` | Dashboards/Metrics | 3000 | ❌ Not configured |
| **ArgoCD** | `argocd.prod.ai-aas.local` | GitOps | 8080/443 | ❌ Not configured |
| **Prometheus** | `prometheus.prod.ai-aas.local` | Metrics (optional) | 9090 | ❌ Not configured |

## Setup Checklist

- [ ] Generate self-signed certificates (`./scripts/infra/generate-self-signed-certs.sh`)
- [ ] Trust CA certificate on local machine
- [ ] Create Kubernetes TLS secrets (`./scripts/infra/create-tls-secrets.sh`)
- [ ] Update hosts file (`sudo ./scripts/infra/update-hosts-file.sh`)
- [ ] Configure API Router ingress with TLS
- [ ] Configure Web Portal ingress with TLS
- [ ] Configure Grafana ingress with TLS
- [ ] Configure ArgoCD ingress with TLS
- [ ] Test all endpoints with HTTPS
- [ ] Update environment profiles with new URLs

## Current Configuration Status

### ✅ Already Configured
- Web Portal has ingress (needs URL update)
- API Router has ingress template (disabled in dev, enabled in prod)

### ❌ Not Configured
- Grafana ingress
- ArgoCD ingress (currently disabled)
- Prometheus ingress (optional)
- Consistent URLs across environments
- Self-signed certificate setup

## Access Pattern

- **DNS**: Local hosts file (`/etc/hosts` or Windows hosts file)
- **SSL**: Self-signed certificates (CA trusted locally)
- **Security**: Firewall-restricted access (no VPN needed)
- **Domains**: `.ai-aas.local` TLD for local development

## Quick Commands

```bash
# Generate certificates
./scripts/infra/generate-self-signed-certs.sh

# Trust CA (Linux)
sudo cp infra/secrets/certs/ai-aas-ca.crt /usr/local/share/ca-certificates/ && sudo update-ca-certificates

# Create K8s secrets
./scripts/infra/create-tls-secrets.sh --namespace ingress-nginx

# Update hosts file
sudo ./scripts/infra/update-hosts-file.sh

# Test endpoints
curl https://api.dev.ai-aas.local/healthz
curl https://portal.dev.ai-aas.local
```

