# Missing Credentials & Tokens

## Current Status

✅ **Configured:**
- Git user: `James Brown <otherjamesbrown@users.noreply.github.com>`

❌ **Missing:**

### Required for Development

1. **GitHub CLI Authentication** (Required for `make ci-remote`)
   - Status: Not authenticated
   - Fix: `gh auth login`
   - Scopes needed: `repo`, `read:actions`, `workflow`
   - This allows triggering GitHub Actions workflows remotely

2. **SSH Key for Git Operations** (Required for git clone/push)
   - Status: No SSH keys found
   - Fix: Generate SSH key and add to GitHub
   - Commands:
     ```bash
     # Generate SSH key (if needed)
     ssh-keygen -t ed25519 -C "otherjamesbrown@users.noreply.github.com"
     
     # Add to SSH agent
     eval "$(ssh-agent -s)"
     ssh-add ~/.ssh/id_ed25519
     
     # Display public key to add to GitHub
     cat ~/.ssh/id_ed25519.pub
     ```
   - Then add to GitHub: https://github.com/settings/keys

### Required for Infrastructure Work (Optional - only if using Linode)

3. **Linode API Token** (Required for remote workspace provisioning)
   - Status: `LINODE_TOKEN` not set
   - Fix: Create token in Akamai Control Panel
   - Scopes needed:
     - `linodes:read_write` (VM provisioning)
     - `lke:read_write` (Kubernetes cluster provisioning)
     - `object-storage:read_write` (Object storage bucket management)
   - How to create:
     1. Log into https://cloud.linode.com/
     2. Navigate to **My Profile → API Tokens → Add a Personal Access Token**
     3. Grant the scopes above
     4. Copy token and set: `export LINODE_TOKEN=<token>`
   - Add to `~/.bashrc` or `~/.envrc`:
     ```bash
     export LINODE_TOKEN=<your-token>
     export LINODE_DEFAULT_REGION=fr-par  # or your preferred region
     ```

4. **Linode Object Storage Credentials** (Optional - for metrics/S3 operations)
   - Status: Not configured
   - Variables needed:
     - `LINODE_OBJECT_STORAGE_ACCESS_KEY`
     - `LINODE_OBJECT_STORAGE_SECRET_KEY`
   - These are used by Terraform and S3-compatible tools
   - Also set AWS-compatible variables:
     ```bash
     export AWS_ACCESS_KEY_ID=$LINODE_OBJECT_STORAGE_ACCESS_KEY
     export AWS_SECRET_ACCESS_KEY=$LINODE_OBJECT_STORAGE_SECRET_KEY
     ```

### Optional for Observability

5. **OpenTelemetry Token** (Optional - for migration telemetry)
   - Status: Not configured
   - Variable: `OTEL_EXPORTER_OTLP_HEADERS="x-api-key=<token>"`
   - Endpoint: `OTEL_EXPORTER_OTLP_ENDPOINT=https://otel.dev.ai-aas.internal`
   - Can store token in: `~/.config/ai-aas/otel.token`
   - Only needed if you want telemetry from database migrations

## Quick Setup Commands

### 1. Authenticate GitHub CLI (Required)
```bash
gh auth login
# Select: GitHub.com
# Select: HTTPS (or SSH if you prefer)
# Authenticate via browser
# Select scopes: repo, read:actions, workflow
```

### 2. Set up SSH Key for Git (Required if using SSH URLs)
```bash
# Check if key exists
ls ~/.ssh/id_ed25519.pub || ls ~/.ssh/id_rsa.pub

# If no key, generate one
ssh-keygen -t ed25519 -C "otherjamesbrown@users.noreply.github.com"
# Press Enter to accept default location
# Optionally set a passphrase

# Start SSH agent
eval "$(ssh-agent -s)"
ssh-add ~/.ssh/id_ed25519

# Display public key to add to GitHub
cat ~/.ssh/id_ed25519.pub
# Copy output and add at: https://github.com/settings/keys

# Test connection
ssh -T git@github.com
```

### 3. Set Linode Token (Optional - only if using Linode)
```bash
# Add to ~/.bashrc
echo 'export LINODE_TOKEN=<your-token>' >> ~/.bashrc
echo 'export LINODE_DEFAULT_REGION=fr-par' >> ~/.bashrc

# Reload
source ~/.bashrc

# Verify
echo $LINODE_TOKEN  # Should show your token
```

### 4. Create Credential Store (Recommended)
Create `~/.config/ai-aas/tokens.sh` (gitignored):
```bash
#!/bin/bash
# Source this file: source ~/.config/ai-aas/tokens.sh

export LINODE_TOKEN=<your-token>
export LINODE_DEFAULT_REGION=fr-par

# Optional
export OTEL_EXPORTER_OTLP_ENDPOINT=https://otel.dev.ai-aas.internal
export OTEL_EXPORTER_OTLP_HEADERS="x-api-key=$(cat ~/.config/ai-aas/otel.token 2>/dev/null || echo '')"

# Optional - Object Storage
export LINODE_OBJECT_STORAGE_ACCESS_KEY=<access-key>
export LINODE_OBJECT_STORAGE_SECRET_KEY=<secret-key>
export AWS_ACCESS_KEY_ID=$LINODE_OBJECT_STORAGE_ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=$LINODE_OBJECT_STORAGE_SECRET_KEY
```

Then source it:
```bash
chmod 600 ~/.config/ai-aas/tokens.sh  # Secure permissions
source ~/.config/ai-aas/tokens.sh
```

## Verification

After setting up credentials, verify:

```bash
# GitHub CLI
gh auth status
# Should show: Logged in as otherjamesbrown

# SSH to GitHub
ssh -T git@github.com
# Should show: Hi otherjamesbrown! You've successfully authenticated...

# Linode (if configured)
echo $LINODE_TOKEN
# Should show your token (not empty)

# Git operations
git remote -v
# Should show your remotes configured
```

## Priority

**Must have for basic development:**
1. ✅ Git user configured (done)
2. ⏳ GitHub CLI authentication (`gh auth login`)
3. ⏳ SSH key for Git (if using SSH URLs)

**Only needed for infrastructure work:**
4. ⏳ LINODE_TOKEN (only if provisioning remote workspaces)
5. ⏳ Object Storage credentials (only if using S3-compatible operations)

**Optional:**
6. ⏳ OpenTelemetry token (only if you want migration telemetry)

## Security Notes

- **Never commit tokens to Git**
- Use `.gitignore` to exclude credential files
- Set secure file permissions: `chmod 600 ~/.config/ai-aas/tokens.sh`
- Use environment variables or secret managers (1Password, Vault, etc.)
- Rotate tokens periodically
- Use least-privilege scopes when creating tokens

