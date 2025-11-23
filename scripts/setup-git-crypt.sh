#!/bin/bash
#
# Setup script for git-crypt on the FIRST machine (primary machine)
# This initializes git-crypt and generates the symmetric key
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
KEY_FILE="$PROJECT_ROOT/secrets/ai-aas-git-crypt.key"

echo "=== Git-Crypt Setup for AI-AAS ==="
echo ""

# Check if git-crypt is installed
if ! command -v git-crypt &> /dev/null; then
    echo "git-crypt is not installed. Installing..."
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        sudo apt-get update && sudo apt-get install -y git-crypt
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        brew install git-crypt
    else
        echo "Please install git-crypt manually for your platform"
        exit 1
    fi
fi

echo "✓ git-crypt is installed"
echo ""

cd "$PROJECT_ROOT"

# Check if git-crypt is already initialized
if [ -d ".git/git-crypt" ]; then
    echo "⚠ git-crypt is already initialized in this repository"
    echo ""
    read -p "Do you want to export the key for other machines? (y/n) " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git-crypt export-key "$KEY_FILE"
        echo "✓ Key exported to: $KEY_FILE"
        echo ""
        echo "IMPORTANT: Copy this key file to your other machines securely!"
        echo "You can use scp, USB drive, or a password manager."
        echo ""
        echo "On other machines, run: ./scripts/unlock-git-crypt.sh"
    fi
    exit 0
fi

echo "Initializing git-crypt..."
git-crypt init

echo "✓ git-crypt initialized"
echo ""

# Export the symmetric key
echo "Exporting symmetric key..."
git-crypt export-key "$KEY_FILE"

echo "✓ Key exported to: $KEY_FILE"
echo ""

# Create a README for the key
cat > "$PROJECT_ROOT/secrets/KEY-INSTRUCTIONS.md" << 'EOF'
# Git-Crypt Key

This directory contains the symmetric encryption key for git-crypt.

## Important Security Notes

1. **Keep this key secure!** Anyone with this key can decrypt all secrets in the repository.
2. Store a backup of `ai-aas-git-crypt.key` in a secure location (password manager, encrypted USB, etc.)
3. Do NOT commit the key file itself to git (it's in .gitignore)

## Setup on Other Machines

1. Copy `ai-aas-git-crypt.key` to the new machine
2. Place it in the `secrets/` directory of your cloned repo
3. Run: `./scripts/unlock-git-crypt.sh`

## Verification

After unlocking, verify that files are decrypted:
```bash
# These files should be readable (not binary)
cat secrets/env/.env.development
cat secrets/kubeconfigs/kubeconfig-development.yaml
```
EOF

echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "1. Move your existing secrets to the secrets/ directory:"
echo "   - .env files → secrets/env/"
echo "   - kubeconfig files → secrets/kubeconfigs/"
echo ""
echo "2. Add and commit the files:"
echo "   git add secrets/ .gitattributes"
echo "   git commit -m 'Add encrypted secrets with git-crypt'"
echo "   git push"
echo ""
echo "3. Copy the key file to your other machines:"
echo "   Key location: $KEY_FILE"
echo ""
echo "4. On other machines:"
echo "   - git clone the repository"
echo "   - Copy the key file to secrets/ai-aas-git-crypt.key"
echo "   - Run: ./scripts/unlock-git-crypt.sh"
echo ""
