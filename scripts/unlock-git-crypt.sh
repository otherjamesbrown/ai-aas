#!/bin/bash
#
# Unlock script for git-crypt on SECONDARY machines
# This unlocks the repository using the symmetric key
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
KEY_FILE="$PROJECT_ROOT/secrets/ai-aas-git-crypt.key"

echo "=== Git-Crypt Unlock for AI-AAS ==="
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

# Check if already unlocked
if git-crypt status &> /dev/null; then
    if git-crypt status | grep -q "not encrypted"; then
        echo "✓ Repository is already unlocked"
        exit 0
    fi
fi

# Check if key file exists
if [ ! -f "$KEY_FILE" ]; then
    echo "❌ Key file not found: $KEY_FILE"
    echo ""
    echo "Please copy the key file from your primary machine to:"
    echo "  $KEY_FILE"
    echo ""
    echo "You can use scp, for example:"
    echo "  scp user@primary-machine:~/ai-aas/secrets/ai-aas-git-crypt.key $KEY_FILE"
    exit 1
fi

echo "Found key file: $KEY_FILE"
echo "Unlocking repository..."

git-crypt unlock "$KEY_FILE"

echo "✓ Repository unlocked successfully!"
echo ""

# Verify unlock
echo "Verifying decryption..."
if [ -d "secrets/env" ] && [ "$(ls -A secrets/env 2>/dev/null)" ]; then
    echo "✓ Files in secrets/env/ are decrypted"
elif [ -d "secrets/kubeconfigs" ] && [ "$(ls -A secrets/kubeconfigs 2>/dev/null)" ]; then
    echo "✓ Files in secrets/kubeconfigs/ are decrypted"
else
    echo "⚠ No encrypted files found yet. This is normal for a new setup."
fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "You can now pull secrets from the repository:"
echo "  git pull"
echo ""
echo "All files matching the patterns in .gitattributes will be"
echo "automatically decrypted when you pull or checkout."
echo ""
