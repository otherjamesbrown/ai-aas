# TLS/SSL Certificate Setup Guide

**Feature**: `013-ingress-tls`  
**Last Updated**: 2025-11-16  
**Owner**: Platform Engineering

## Overview

This guide covers TLS/SSL certificate setup for both production and local development environments.

## Production Environment

### Certificate Management

- **Provider**: Let's Encrypt via cert-manager
- **Auto-Renewal**: Certificates automatically renewed before expiration
- **DNS**: Public DNS records required for ACME validation
- **Ingress**: NGINX Ingress Controller handles TLS termination

### Configuration

Certificates are configured via Helm values in service charts:

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
  hosts:
    - host: api.ai-aas.prod
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: api-router-service-tls
      hosts:
        - api.ai-aas.prod
```

### Domains

Production domains follow the pattern:
- `api.ai-aas.prod` - API Router Service
- `portal.ai-aas.prod` - Web Portal
- `grafana.prod.ai-aas.prod` - Grafana (if exposed)
- `argocd.prod.ai-aas.prod` - ArgoCD (if exposed)

## Local Development Environment

### Certificate Management

- **Type**: Self-signed certificates
- **Location**: `infra/secrets/certs/`
- **DNS**: Local hosts file (`/etc/hosts` or Windows hosts file)
- **Access**: Firewall-restricted (no VPN required)

### Quick Setup

1. **Generate certificates**:
   ```bash
   ./scripts/infra/generate-self-signed-certs.sh
   ```

2. **Trust CA certificate** (one-time per machine):
   ```bash
   # Linux
   sudo cp infra/secrets/certs/ai-aas-ca.crt /usr/local/share/ca-certificates/ai-aas-ca.crt
   sudo update-ca-certificates
   
   # macOS
   sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain infra/secrets/certs/ai-aas-ca.crt
   ```

3. **Create Kubernetes secrets**:
   ```bash
   export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
   ./scripts/infra/create-tls-secrets.sh --namespace ingress-nginx
   ```

4. **Update hosts file**:
   ```bash
   sudo ./scripts/infra/update-hosts-file.sh
   ```

### Domains

Local development domains use `.ai-aas.local`:
- `api.dev.ai-aas.local` - API Router Service
- `portal.dev.ai-aas.local` - Web Portal
- `grafana.dev.ai-aas.local` - Grafana
- `argocd.dev.ai-aas.local` - ArgoCD

### Certificate Files

**Public (Committed to Git)**:
- `ai-aas-ca.crt` - CA certificate (trust this on all machines)
- `tls.crt` - TLS certificate for all domains

**Private (NOT Committed)**:
- `ai-aas-ca.key` - CA private key (keep secure backup)
- `tls.key` - TLS private key

See `infra/secrets/certs/README.md` for detailed certificate management.

## Setting Up on a New Machine

### Step 1: Pull Repository

```bash
git pull
```

### Step 2: Trust CA Certificate

Follow the trust instructions above for your OS.

### Step 3: Get Private Keys

**Option A**: Copy from another machine (secure transfer)
**Option B**: Regenerate certificates (requires CA private key)

```bash
./scripts/infra/generate-self-signed-certs.sh
```

### Step 4: Configure Kubernetes

```bash
# Set up kubeconfigs
# See tmp_md/KUBECONFIG_SETUP_SIMPLE.md

# Create TLS secrets
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
./scripts/infra/create-tls-secrets.sh --namespace ingress-nginx
```

### Step 5: Update Hosts File

```bash
sudo ./scripts/infra/update-hosts-file.sh
```

## Troubleshooting

### Certificate Not Trusted

- Verify CA certificate is in trusted store
- Restart browser after trusting CA
- Check certificate validity: `openssl x509 -in infra/secrets/certs/ai-aas-ca.crt -text -noout`

### Certificate Expired

- Regenerate certificates: `./scripts/infra/generate-self-signed-certs.sh`
- Re-create Kubernetes secrets: `./scripts/infra/create-tls-secrets.sh`

### Private Key Missing

- Copy from another machine or regenerate certificates
- Ensure proper permissions: `chmod 600 infra/secrets/certs/*.key`

### DNS Resolution Issues

- Verify hosts file entries: `cat /etc/hosts | grep ai-aas.local`
- Check ingress IP: `kubectl get svc -n ingress-nginx ingress-nginx-controller`
- Update hosts file: `sudo ./scripts/infra/update-hosts-file.sh`

## Related Documentation

- `infra/secrets/certs/README.md` - Detailed certificate management
- `docs/platform/endpoints-and-urls.md` - Complete endpoint configuration guide
- `tmp_md/KUBECONFIG_SETUP_SIMPLE.md` - Kubeconfig setup
- `specs/013-ingress-tls/spec.md` - Ingress TLS specification

