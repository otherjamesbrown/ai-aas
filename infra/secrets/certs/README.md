# Self-Signed Certificates for Local Development

This directory contains self-signed SSL certificates for local development environments using local DNS (hosts file) and firewall-restricted access.

## Files

### Public Certificates (Committed to Git)

- **`ai-aas-ca.crt`** - Certificate Authority (CA) certificate
  - **Purpose**: Trust this certificate on your local machines to avoid browser warnings
  - **Distribution**: Committed to git so it can be trusted on all development machines
  - **Security**: Public certificate, safe to commit

- **`tls.crt`** - TLS certificate for all domains
  - **Purpose**: Used by Kubernetes ingress for HTTPS
  - **Domains**: Covers all dev/prod endpoints (`*.ai-aas.local`)
  - **Distribution**: Committed to git for consistency

- **`ai-aas-ca.srl`** - Serial number file
  - **Purpose**: OpenSSL serial number tracking
  - **Distribution**: Committed to git

### Private Keys (NOT Committed - Git Ignored)

- **`ai-aas-ca.key`** - CA private key
  - **Purpose**: Used to sign TLS certificates
  - **Security**: **SECRET** - Never commit to git
  - **Backup**: Keep secure backup of this file

- **`tls.key`** - TLS private key
  - **Purpose**: Used with TLS certificate for HTTPS
  - **Security**: **SECRET** - Never commit to git
  - **Regeneration**: Can be regenerated using the CA key

## Setup on a New Machine

### Step 1: Pull Repository

```bash
git pull
```

### Step 2: Trust the CA Certificate

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

### Step 3: Get Private Keys

You have two options:

**Option A: Copy from Another Machine (Recommended)**
- Securely copy `ai-aas-ca.key` and `tls.key` from your other machine
- Use encrypted transfer (SSH, encrypted USB, etc.)
- Place them in `infra/secrets/certs/`

**Option B: Regenerate Certificates**
```bash
# This will generate new certificates using the same CA
# Note: You'll need the CA private key for this to work
./scripts/infra/generate-self-signed-certs.sh
```

### Step 4: Set Permissions

```bash
chmod 600 infra/secrets/certs/*.key
```

### Step 5: Create Kubernetes Secrets

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
./scripts/infra/create-tls-secrets.sh --namespace ingress-nginx
```

## Regenerating Certificates

If you need to regenerate certificates (e.g., adding new domains):

```bash
# Regenerate with existing CA
./scripts/infra/generate-self-signed-certs.sh

# Or regenerate with custom domains
./scripts/infra/generate-self-signed-certs.sh --domains "api.dev.ai-aas.local,portal.dev.ai-aas.local"
```

**Note**: Regenerating requires the CA private key (`ai-aas-ca.key`). If you've lost it, you'll need to generate a new CA and re-trust it on all machines.

## Security Notes

1. **CA Private Key**: Keep `ai-aas-ca.key` secure and backed up
   - Losing it means regenerating the CA and re-trusting on all machines
   - Consider storing in a password manager or secure backup

2. **TLS Private Key**: `tls.key` can be regenerated if lost (using CA key)

3. **Certificate Validity**: Certificates are valid for 365 days
   - CA certificate: 10 years
   - TLS certificate: 1 year (regenerate annually)

4. **Domain Coverage**: Current certificates cover:
   - `api.dev.ai-aas.local`
   - `portal.dev.ai-aas.local`
   - `grafana.dev.ai-aas.local`
   - `argocd.dev.ai-aas.local`
   - `api.prod.ai-aas.local`
   - `portal.prod.ai-aas.local`
   - `grafana.prod.ai-aas.local`
   - `argocd.prod.ai-aas.local`

## Troubleshooting

### "Certificate not trusted" error
- Make sure you've trusted the CA certificate (Step 2 above)
- Restart your browser after trusting the CA
- Check certificate is in trusted store: `certutil -L` (Linux) or Keychain Access (macOS)

### "Certificate expired" error
- Regenerate certificates: `./scripts/infra/generate-self-signed-certs.sh`
- Re-create Kubernetes secrets: `./scripts/infra/create-tls-secrets.sh`

### "Private key not found" error
- Copy private keys from another machine or regenerate certificates
- Ensure files have correct permissions: `chmod 600 *.key`

## Related Documentation

- `tmp_md/ENDPOINTS_AND_URLS.md` - Complete endpoint and URL configuration guide
- `tmp_md/KUBECONFIG_SETUP_SIMPLE.md` - Kubeconfig setup guide
- `scripts/infra/generate-self-signed-certs.sh` - Certificate generation script
- `scripts/infra/create-tls-secrets.sh` - Kubernetes secret creation script

