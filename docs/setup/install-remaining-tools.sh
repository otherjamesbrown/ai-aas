#!/bin/bash
# Script to install remaining required tools that need sudo privileges
# Run with: sudo bash install-remaining-tools.sh

set -euo pipefail

echo "Installing remaining required tools..."

# Update package lists
apt update

# Install GNU Make
echo "Installing GNU Make..."
apt install -y make build-essential

# Install GitHub CLI
echo "Installing GitHub CLI..."
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg

echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null

apt update
apt install -y gh

# Install Node.js 20.x LTS
echo "Installing Node.js 20.x..."
curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
apt install -y nodejs

# Verify installations
echo ""
echo "Verifying installations..."
make --version
gh --version
node --version
npm --version

echo ""
echo "✅ All required tools installed!"
echo ""
echo "Next steps:"
echo "1. Run: gh auth login"
echo "2. Run: cd /home/dev/ai-aas && ./scripts/setup/bootstrap.sh"
echo "3. Run: export PATH=\$PATH:\$HOME/go-bin/go/bin && source ~/.bashrc"
echo "4. Run: make shared-ts-install"

