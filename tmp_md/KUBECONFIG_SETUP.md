# Kubeconfig Setup from 1Password (Manual Copy-Paste)

## Quick Steps

### Step 1: Create Directory

```bash
mkdir -p ~/kubeconfigs
```

### Step 2: Get Development Kubeconfig from 1Password

1. Open the **1Password** app
2. Search for "kubeconfig" or "kubernetes" or "development"
3. Find your development cluster kubeconfig
4. **Copy the entire kubeconfig content** (it's usually a YAML file)
5. Save it to file:

```bash
# Open editor
nano ~/kubeconfigs/kubeconfig-development.yaml

# Paste the content (right-click or Ctrl+Shift+V)
# Save: Ctrl+X, then Y, then Enter
```

**Or use your preferred editor:**
```bash
# Using vim
vim ~/kubeconfigs/kubeconfig-development.yaml
# Press 'i' to insert, paste content, press Esc, type ':wq' and Enter

# Using VS Code
code ~/kubeconfigs/kubeconfig-development.yaml
# Paste and save
```

### Step 3: Get Production Kubeconfig from 1Password

1. In **1Password** app, find your production cluster kubeconfig
2. **Copy the entire kubeconfig content**
3. Save it to file:

```bash
nano ~/kubeconfigs/kubeconfig-production.yaml
# Paste content, save (Ctrl+X, Y, Enter)
```

### Step 4: Set Permissions

```bash
# Make sure files are readable but not world-readable
chmod 600 ~/kubeconfigs/kubeconfig-development.yaml
chmod 600 ~/kubeconfigs/kubeconfig-production.yaml
```

### Step 5: Verify Setup

```bash
# Test development kubeconfig
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl config use-context lke531921-ctx
kubectl cluster-info

# Test production kubeconfig
export KUBECONFIG=~/kubeconfigs/kubeconfig-production.yaml
kubectl config use-context lke531922-ctx
kubectl cluster-info
```

## What to Look For in 1Password

The kubeconfig items might be named:
- "kubeconfig-dev" or "kubeconfig-development"
- "kubeconfig-production" or "kubeconfig-prod"
- "Kubernetes Development Cluster"
- "Kubernetes Production Cluster"
- Or similar variations

**Tip**: In 1Password, look for items with:
- Type: "Secure Note" or "Document"
- Contains: YAML content starting with `apiVersion: v1` and `kind: Config`
- Fields like: `clusters:`, `contexts:`, `users:`

## Alternative: Using 1Password CLI (Optional)

1. Open 1Password app
2. Find your kubeconfig item (search for "kubeconfig" or "kubernetes")
3. Copy the kubeconfig content
4. Save to file:

```bash
mkdir -p ~/kubeconfigs
nano ~/kubeconfigs/kubeconfig-development.yaml
# Paste content, save (Ctrl+X, Y, Enter)

nano ~/kubeconfigs/kubeconfig-production.yaml
# Paste content, save (Ctrl+X, Y, Enter)

chmod 600 ~/kubeconfigs/*.yaml
```

## Troubleshooting

### "context not found" error
Make sure the kubeconfig contains the correct context:
- Development: `lke531921-ctx`
- Production: `lke531922-ctx`

Check contexts in your kubeconfig:
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl config get-contexts
```

### "Invalid kubeconfig" or "unable to connect"
- Make sure you copied the **entire** kubeconfig content
- Check that the file ends with proper YAML formatting
- Verify there are no extra characters or formatting issues
- Try viewing the file: `cat ~/kubeconfigs/kubeconfig-development.yaml`

### Can't find kubeconfig in 1Password
- Search for: "kubeconfig", "kubernetes", "k8s", "cluster"
- Check different vaults if you have multiple
- Look for items with type "Secure Note" or "Document"
- The kubeconfig is usually a multi-line text field or document attachment

## Expected File Structure

After setup, you should have:

```
~/kubeconfigs/
├── kubeconfig-development.yaml  (context: lke531921-ctx)
└── kubeconfig-production.yaml   (context: lke531922-ctx)
```

## Security Notes

- Kubeconfigs contain sensitive credentials
- Files are set to `600` permissions (owner read/write only)
- Consider using 1Password CLI directly instead of saving to disk
- Never commit kubeconfigs to git (they're in `.gitignore`)

