# Using Kubeconfigs from 1Password (No Local Storage Required)

## Overview

You don't need to save kubeconfigs locally! The recommended approach is to keep them in 1Password and use them temporarily. This is more secure and aligns with the platform's security practices.

## Quick Start: Access Argo CD UI from 1Password

### Prerequisites
1. Install 1Password CLI: https://developer.1password.com/docs/cli
2. Authenticate: `op signin`
3. Store kubeconfigs in 1Password (as documents or secure notes)

### Method 1: Pipe from 1Password (Recommended)

```bash
# Development cluster
op read "op://vault/kubeconfig-development" | ./scripts/access-argocd-ui.sh development -

# Production cluster  
op read "op://vault/kubeconfig-production" | ./scripts/access-argocd-ui.sh production -
```

The script will:
- Read kubeconfig from stdin
- Use it temporarily (stored in `/tmp/`)
- Automatically clean up when done
- Never save to your home directory

### Method 2: Temporary File (Manual)

```bash
# Extract to temporary file
op read "op://vault/kubeconfig-development" > /tmp/kubeconfig-dev.yaml

# Use it
./scripts/access-argocd-ui.sh development /tmp/kubeconfig-dev.yaml

# Clean up when done
rm /tmp/kubeconfig-dev.yaml
```

### Method 3: Environment Variable

```bash
# Extract to environment variable (for single command)
KUBECONFIG=$(op read "op://vault/kubeconfig-development") \
  ./scripts/access-argocd-ui.sh development

# Or save to temp file first
TEMP_KUBE=$(mktemp)
op read "op://vault/kubeconfig-development" > "$TEMP_KUBE"
export KUBECONFIG="$TEMP_KUBE"
./scripts/access-argocd-ui.sh development
rm "$TEMP_KUBE"
```

## Storing Kubeconfigs in 1Password

### Option A: As a Document
1. Open 1Password
2. Create new item → Document
3. Name: `kubeconfig-development` (or `kubeconfig-production`)
4. Upload your kubeconfig file
5. Store in appropriate vault

### Option B: As a Secure Note
1. Open 1Password
2. Create new item → Secure Note
3. Name: `kubeconfig-development`
4. Paste kubeconfig content
5. Store in appropriate vault

### Finding the Reference Path

After storing, find the reference path:
```bash
# List items in vault
op item list --vault="vault-name"

# Get reference path for an item
op item get "kubeconfig-development" --vault="vault-name" --format=json | jq -r '.id'
```

Reference format: `op://vault-name/item-name` or `op://vault-name/item-id`

## Security Benefits

✅ **No secrets on disk** - Kubeconfigs never stored permanently  
✅ **Automatic cleanup** - Temporary files removed automatically  
✅ **Centralized management** - All secrets in 1Password  
✅ **Audit trail** - 1Password tracks access  
✅ **Access control** - 1Password manages who can access  

## Alternative: Using kubectl Directly

If you prefer not to use the script:

```bash
# Get kubeconfig from 1Password and use directly
op read "op://vault/kubeconfig-development" > /tmp/kubeconfig.yaml
export KUBECONFIG=/tmp/kubeconfig.yaml
kubectl config use-context lke531921-ctx
kubectl -n argocd port-forward svc/argocd-server 8080:80

# In another terminal, get password and access UI
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d && echo ""

# Clean up when done
rm /tmp/kubeconfig.yaml
unset KUBECONFIG
```

## Troubleshooting

### 1Password CLI Not Found
```bash
# Install 1Password CLI
# macOS
brew install --cask 1password-cli

# Linux
# Download from: https://developer.1password.com/docs/cli/get-started#install
```

### Not Signed In
```bash
op signin
# Follow prompts to authenticate
```

### Can't Find Item
```bash
# Search for kubeconfig items
op item list | grep kubeconfig

# Or search in specific vault
op item list --vault="your-vault-name" | grep kubeconfig
```

### Permission Denied
Make sure you have access to the vault containing the kubeconfig:
```bash
# List accessible vaults
op vault list

# Check item permissions
op item get "kubeconfig-development" --vault="vault-name"
```

## Best Practices

1. **Never commit kubeconfigs** - They're in `.gitignore` for a reason
2. **Use temporary files** - Always clean up after use
3. **Rotate regularly** - Update kubeconfigs in 1Password periodically
4. **Limit access** - Only share kubeconfigs with authorized team members
5. **Use separate vaults** - Keep dev and prod kubeconfigs in different vaults if possible

## Integration with Other Tools

### kubectl
```bash
# Use with kubectl directly
export KUBECONFIG=$(mktemp)
op read "op://vault/kubeconfig-development" > "$KUBECONFIG"
kubectl get nodes
rm "$KUBECONFIG"
```

### helm
```bash
# Same approach works with helm
export KUBECONFIG=$(mktemp)
op read "op://vault/kubeconfig-development" > "$KUBECONFIG"
helm list -n argocd
rm "$KUBECONFIG"
```

### argocd CLI
```bash
# After port-forwarding with kubeconfig from 1Password
export KUBECONFIG=$(mktemp)
op read "op://vault/kubeconfig-development" > "$KUBECONFIG"
kubectl -n argocd port-forward svc/argocd-server 8080:80 &
ARGOCD_PID=$!

# In another terminal
argocd login localhost:8080 --username admin --insecure --grpc-web

# Clean up
kill $ARGOCD_PID
rm "$KUBECONFIG"
```

