# Installation Requirements for Linux Development Environment

Based on the current system check, here's what needs to be installed.

## Current Status

✅ **Already Installed:**
- Git (v2.51.0)
- Docker (v29.0.1)

❌ **Missing (Required):**
- Go compiler (v1.24.6+)
- GNU Make (v4.x+)
- Node.js (v20+)
- pnpm
- GitHub CLI (gh)

⚠️ **Optional but Recommended:**
- act (local GitHub Actions)
- AWS CLI or MinIO Client

## Installation Commands (Ubuntu/Debian)

### 1. Install Go (v1.24.6+)

**Option A: Using official Go installer (Recommended)**
```bash
# Download Go 1.24.6 (or latest 1.24.x)
cd /tmp
wget https://go.dev/dl/go1.24.6.linux-amd64.tar.gz

# Remove any old Go installation
sudo rm -rf /usr/local/go

# Extract to /usr/local
sudo tar -C /usr/local -xzf go1.24.6.linux-amd64.tar.gz

# Add to PATH (add to ~/.bashrc or ~/.profile)
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify
go version
```

**Option B: Using apt (may have older version)**
```bash
sudo apt update
sudo apt install golang-go
# Note: Check version after install - may need to use Option A if too old
go version
```

**Option C: Using snap (has 1.25.4 available)**
```bash
sudo snap install go --classic
go version
```

### 2. Install GNU Make

```bash
sudo apt update
sudo apt install make

# Verify
make --version
```

Or install build-essential (includes make and other build tools):
```bash
sudo apt install build-essential
```

### 3. Install Node.js (v20+)

**Option A: Using NodeSource repository (Recommended for v20+)**
```bash
# Install Node.js 20.x LTS
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs

# Verify
node --version
npm --version
```

**Option B: Using nvm (Node Version Manager - Recommended for flexibility)**
```bash
# Install nvm
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash

# Reload shell or run:
source ~/.bashrc

# Install Node.js 20
nvm install 20
nvm use 20

# Verify
node --version
npm --version
```

**Option C: Using apt (may have older version)**
```bash
sudo apt update
sudo apt install nodejs npm

# Check version - may need Option A or B if version < 20
node --version
```

### 4. Install pnpm

Once Node.js is installed:
```bash
# Install pnpm globally
npm install -g pnpm

# Or using standalone script (recommended)
curl -fsSL https://get.pnpm.io/install.sh | sh -
source ~/.bashrc

# Verify
pnpm --version
```

### 5. Install GitHub CLI

**Option A: Using official GitHub repository (Recommended)**
```bash
# Add GitHub CLI repository
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
sudo chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg

echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null

sudo apt update
sudo apt install gh

# Verify
gh --version

# Authenticate (required for ci-remote)
gh auth login
```

**Option B: Using snap**
```bash
sudo snap install gh
gh auth login
```

**Option C: Using binary download**
```bash
# Download latest release from GitHub
cd /tmp
wget https://github.com/cli/cli/releases/latest/download/gh_*_linux_amd64.tar.gz
tar -xzf gh_*_linux_amd64.tar.gz
sudo cp gh_*/bin/gh /usr/local/bin/
gh --version
gh auth login
```

## Optional Tools

### Install act (Local GitHub Actions)

```bash
# Using GitHub releases
cd /tmp
wget https://github.com/nektos/act/releases/download/v0.2.61/act_Linux_x86_64.tar.gz
tar -xzf act_Linux_x86_64.tar.gz
sudo mv act /usr/local/bin/

# Verify
act --version
```

### Install AWS CLI

```bash
cd /tmp
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install

# Verify
aws --version
```

### Install MinIO Client (mc)

```bash
cd /tmp
wget https://dl.min.io/client/mc/release/linux-amd64/mc
chmod +x mc
sudo mv mc /usr/local/bin/

# Verify
mc --version
```

## Quick Install Script

You can run these commands together (adjust Go version as needed):

```bash
#!/bin/bash
set -e

echo "Installing Go..."
cd /tmp
wget https://go.dev/dl/go1.24.6.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.6.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc

echo "Installing Make..."
sudo apt update
sudo apt install -y make build-essential

echo "Installing Node.js..."
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs

echo "Installing pnpm..."
npm install -g pnpm

echo "Installing GitHub CLI..."
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
sudo chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
sudo apt update
sudo apt install -y gh

echo "Reloading shell environment..."
source ~/.bashrc

echo "Verifying installations..."
go version
make --version
node --version
pnpm --version
gh --version

echo ""
echo "✅ Installation complete!"
echo "Next steps:"
echo "1. Run: gh auth login"
echo "2. Run: cd /home/dev/ai-aas && ./scripts/setup/bootstrap.sh"
echo "3. Run: make shared-ts-install"
```

Save this as `install-deps.sh` and run:
```bash
chmod +x install-deps.sh
./install-deps.sh
```

## Post-Installation Steps

### 1. Authenticate GitHub CLI
```bash
gh auth login
# Select: GitHub.com
# Select: HTTPS
# Authenticate in browser
# Select scopes: repo, read:actions, workflow
```

### 2. Run Bootstrap
```bash
cd /home/dev/ai-aas
./scripts/setup/bootstrap.sh
```

### 3. Install TypeScript Dependencies
```bash
make shared-ts-install
```

### 4. Install Playwright Browsers
```bash
# Navigate to web portal directory
cd web/portal

# Install Playwright browsers (Chromium, Firefox, WebKit)
# This installs system dependencies and browser binaries
pnpm exec playwright install --with-deps

# Verify installation
pnpm exec playwright --version
```

### 5. Sync Go Workspace
```bash
go work sync
```

### 6. Verify Everything Works
```bash
# Check tool versions
make version

# Build shared libraries
make shared-build

# Test a service
make build SERVICE=hello-service
```

## Environment Variables to Set (Optional)

For remote operations and metrics:
```bash
# Add to ~/.bashrc or ~/.profile
export LINODE_TOKEN=<your-token-if-needed>
export METRICS_BUCKET=ai-aas-build-metrics
```

For local development databases (create these when needed):
```bash
# Copy example files
cp configs/migrate.example.env migrate.env
cp configs/build.env.example configs/build.env

# Edit with your local database URLs
```

## Verification Checklist

After installation, verify everything:

```bash
# Check all tools
go version          # Should show go1.24.x
make --version      # Should show GNU Make 4.x
node --version      # Should show v20.x or higher
npm --version       # Should be installed with Node
pnpm --version      # Should show version number
gh --version        # Should show GitHub CLI version
docker --version    # Already installed
git --version       # Already installed

# Check GitHub authentication
gh auth status      # Should show authenticated

# Check Playwright browsers (after installing TypeScript dependencies)
cd /home/dev/ai-aas/web/portal
pnpm exec playwright --version  # Should show Playwright version

# Run bootstrap check
cd /home/dev/ai-aas
./scripts/setup/bootstrap.sh --check-only
```

All checks should pass (no ERROR messages).
