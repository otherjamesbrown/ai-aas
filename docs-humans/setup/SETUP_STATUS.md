# Setup Status - Current Progress

## ✅ Completed (No sudo required)

1. **Go 1.24.6** - Installed to `~/go-bin/go/bin`
   - Added to PATH in `~/.bashrc`
   - Go workspace synced successfully
   - Verify: `~/go-bin/go/bin/go version` (or source ~/.bashrc first)

2. **pnpm 10.22.0** - Installed to `~/.local/share/pnpm`
   - Added to PATH in `~/.bashrc`
   - Verify: `~/.local/share/pnpm/pnpm --version` (or source ~/.bashrc first)

## ⏳ Pending (Requires sudo)

Run this command to install remaining required tools:

```bash
sudo bash install-remaining-tools.sh
```

This will install:
- GNU Make (required)
- GitHub CLI (gh) (required)
- Node.js 20.x LTS (required)
- build-essential (includes make and other build tools)

## 🔄 After Running sudo install

Once you run the sudo install script, execute:

```bash
# Reload shell environment to pick up all PATH changes
source ~/.bashrc

# Verify all tools
go version          # Should show go1.24.6
make --version      # Should show GNU Make 4.x
gh --version        # Should show GitHub CLI version
node --version      # Should show v20.x or higher
npm --version       # Should be installed with Node
pnpm --version      # Should show 10.22.0

# Authenticate GitHub CLI (required for make ci-remote)
gh auth login
# Select: GitHub.com
# Select: HTTPS
# Authenticate in browser
# Select scopes: repo, read:actions, workflow

# Run bootstrap check again
cd /home/dev/ai-aas
./scripts/setup/bootstrap.sh --check-only

# Initialize workspace
./scripts/setup/bootstrap.sh

# Install TypeScript dependencies
make shared-ts-install

# Install Playwright browsers (required for web portal E2E tests)
cd web/portal
pnpm exec playwright install --with-deps

# Verify everything works
cd ../..
make version
make shared-build
```

## 📝 Quick Command Reference

```bash
# Activate Go and pnpm in current shell (already in ~/.bashrc)
export PATH=$PATH:$HOME/go-bin/go/bin:$HOME/.local/share/pnpm

# Or reload shell config
source ~/.bashrc

# Install remaining tools (requires sudo)
sudo bash install-remaining-tools.sh

# After installation, verify
./scripts/setup/bootstrap.sh --check-only
```

## ⚠️ Notes

- Go and pnpm are already installed and working (just need to source ~/.bashrc or open new terminal)
- The bootstrap check will pass once Make, GitHub CLI, and Node.js are installed via sudo
- LINODE_TOKEN warning is normal if you're not doing remote infrastructure work
- Optional tools (act, AWS CLI, MinIO Client) can be installed later if needed

