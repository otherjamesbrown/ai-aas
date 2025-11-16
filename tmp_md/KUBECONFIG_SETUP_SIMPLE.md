# Quick Kubeconfig Setup

## Simple 3-Step Process

### Step 1: Create Directory
```bash
mkdir -p ~/kubeconfigs
```

### Step 2: Copy Development Kubeconfig

1. Open **1Password** app
2. Find your development kubeconfig (search for "kubeconfig" or "development")
3. **Copy** the entire kubeconfig content
4. **Paste** into file:

```bash
nano ~/kubeconfigs/kubeconfig-development.yaml
# Paste (Ctrl+Shift+V), then Ctrl+X, Y, Enter
```

### Step 3: Copy Production Kubeconfig

1. In **1Password**, find your production kubeconfig
2. **Copy** the entire kubeconfig content
3. **Paste** into file:

```bash
nano ~/kubeconfigs/kubeconfig-production.yaml
# Paste (Ctrl+Shift+V), then Ctrl+X, Y, Enter
```

### Step 4: Set Permissions
```bash
chmod 600 ~/kubeconfigs/*.yaml
```

## Verify It Works

```bash
# Test development
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl config use-context lke531921-ctx
kubectl cluster-info

# Test production
export KUBECONFIG=~/kubeconfigs/kubeconfig-production.yaml
kubectl config use-context lke531922-ctx
kubectl cluster-info
```

## That's It!

Now you can use the kubeconfigs:
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
./scripts/infra/create-tls-secrets.sh --namespace ingress-nginx
```

