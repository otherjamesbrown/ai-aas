# Next Steps - Development Environment Setup

## ✅ Completed

1. **All required tools installed:**
   - ✅ Go 1.24.6
   - ✅ GNU Make
   - ✅ GitHub CLI (gh)
   - ✅ Node.js (v20+)
   - ✅ pnpm
   - ✅ Docker
   - ✅ Git

2. **SSH key generated:**
   - ✅ Key created at `~/.ssh/id_ed25519`
   - ⏳ Still needs to be added to GitHub

3. **Documentation organized:**
   - ✅ All setup docs moved to `docs/setup/`

## ⏳ Next Steps (In Order)

### 1. Authenticate GitHub CLI (Required)

```bash
gh auth login
```

Follow the prompts:
- Select: **GitHub.com**
- Select: **HTTPS** (or SSH if you prefer)
- Authenticate in browser
- Select scopes: **repo**, **read:actions**, **workflow**

**Verify:**
```bash
gh auth status
# Should show: Logged in as otherjamesbrown
```

### 2. Add SSH Key to GitHub (Required for Git operations)

Your SSH public key:
```
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOxSrxw2LCFqaWAZ24IDwBqkOeBmhAaZ6/qK1RQHYtS/ otherjamesbrown@users.noreply.github.com
```

**Steps:**
1. Display the key: `cat ~/.ssh/id_ed25519.pub`
2. Copy the entire output
3. Go to: https://github.com/settings/keys
4. Click **"New SSH key"**
5. Paste the key and save

**Verify:**
```bash
ssh -T git@github.com
# Should show: Hi otherjamesbrown! You've successfully authenticated...
```

### 3. Complete Workspace Setup

```bash
cd /home/dev/ai-aas

# Make sure PATH includes Go and pnpm
export PATH=$PATH:$HOME/go-bin/go/bin:$HOME/.local/share/pnpm

# Or reload shell config
source ~/.bashrc

# Run full bootstrap (not just check)
./scripts/setup/bootstrap.sh

# Sync Go workspace
go work sync

# Install TypeScript dependencies
make shared-ts-install

# Verify everything works
make version          # Show all tool versions
make shared-build     # Build shared libraries
make shared-test      # Test shared libraries
```

### 4. Optional: Set Linode Token (Only if using Linode infrastructure)

If you need to provision remote workspaces or use Terraform:

```bash
# Create credential directory
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

Get token from: https://cloud.linode.com/ → Profile → API Tokens

### 5. Verify Complete Setup

```bash
cd /home/dev/ai-aas

# Run bootstrap check (should pass all checks)
./scripts/setup/bootstrap.sh --check-only

# Build a test service
make build SERVICE=hello-service

# Run checks
make check SERVICE=hello-service
```

## 🎯 Priority Order

1. **GitHub CLI auth** (`gh auth login`) - Required for `make ci-remote`
2. **Add SSH key to GitHub** - Required for Git push/pull operations
3. **Complete workspace setup** (`./scripts/setup/bootstrap.sh`) - Initialize the project
4. **Install TypeScript deps** (`make shared-ts-install`) - Enable web portal development
5. **Linode token** (optional) - Only if doing infrastructure work

## 📚 Helpful Documentation

- **Quick Auth Setup**: `docs/setup/QUICK_AUTH_SETUP.md`
- **Missing Credentials**: `docs/setup/MISSING_CREDENTIALS.md`
- **Setup Checklist**: `docs/setup/SETUP_CHECKLIST.md`
- **Quickstart Guide**: `specs/000-project-setup/quickstart.md`

## ✨ Once Complete

You'll be able to:
- ✅ Build and test all services locally
- ✅ Run `make ci-remote` to trigger GitHub Actions
- ✅ Use Git operations with SSH
- ✅ Develop TypeScript/React web portal
- ✅ Provision infrastructure (if Linode token is set)

