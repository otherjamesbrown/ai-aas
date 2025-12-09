# Quick Authentication Setup

## Missing Credentials Summary

Based on your current setup, here's what's missing:

### ❌ Required for Development

1. **GitHub CLI Authentication** - Not authenticated
2. **SSH Key for Git** - No SSH keys found (I can help generate one)

### ⚠️ Optional (Only needed for infrastructure work)

3. **Linode API Token** - Not set (only needed if provisioning remote workspaces)
4. **Linode Object Storage Credentials** - Not set (only needed for S3 operations)

## Immediate Actions Needed

### 1. Generate SSH Key (I can do this automatically)

```bash
# Check if SSH key was generated
cat ~/.ssh/id_ed25519.pub

# If key exists, add it to GitHub:
# 1. Copy the output above
# 2. Go to: https://github.com/settings/keys
# 3. Click "New SSH key"
# 4. Paste the key and save

# Test connection (after adding to GitHub)
ssh -T git@github.com
```

### 2. Authenticate GitHub CLI

```bash
# This requires interactive authentication
gh auth login

# Follow prompts:
# - Select: GitHub.com
# - Select: HTTPS (or SSH if you prefer)
# - Authenticate in browser
# - Select scopes: repo, read:actions, workflow
```

### 3. Set Linode Token (Optional - only if using Linode)

If you have a Linode account and want to provision infrastructure:

```bash
# Create credential store
mkdir -p ~/.config/ai-aas

# Add token (replace <token> with your actual token)
cat > ~/.config/ai-aas/tokens.sh << 'EOF'
#!/bin/bash
export LINODE_TOKEN=<your-token>
export LINODE_DEFAULT_REGION=fr-par
EOF

chmod 600 ~/.config/ai-aas/tokens.sh
source ~/.config/ai-aas/tokens.sh
```

## Check Current Status

Run this to see what's configured:

```bash
echo "=== Git Config ==="
git config --global --get user.name
git config --global --get user.email

echo -e "\n=== SSH Keys ==="
ls -la ~/.ssh/*.pub 2>/dev/null || echo "No SSH keys found"

echo -e "\n=== GitHub CLI ==="
gh auth status 2>&1 || echo "GitHub CLI not authenticated"

echo -e "\n=== Environment Tokens ==="
env | grep -E "(LINODE_TOKEN|GITHUB_TOKEN|AWS_ACCESS)" | wc -l | xargs echo "Tokens in environment:"
```

## Next Steps

1. ✅ **Git user is configured** - Done!
2. ⏳ **Generate SSH key** - Run commands above
3. ⏳ **Add SSH key to GitHub** - https://github.com/settings/keys
4. ⏳ **Authenticate GitHub CLI** - Run `gh auth login`
5. ⏳ **Set Linode token** (optional) - Only if you need infrastructure provisioning

See `docs/setup/MISSING_CREDENTIALS.md` for complete details on each credential.

