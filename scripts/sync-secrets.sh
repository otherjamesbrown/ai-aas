#!/bin/bash
#
# Helper script to sync secrets from common locations to the git-crypt managed directory
# Run this on your primary machine to organize secrets before committing
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SECRETS_DIR="$PROJECT_ROOT/secrets"

echo "=== Sync Secrets to Git-Crypt Directory ==="
echo ""

cd "$PROJECT_ROOT"

# Create directories if they don't exist
mkdir -p "$SECRETS_DIR/env"
mkdir -p "$SECRETS_DIR/kubeconfigs"

# Function to safely copy files
copy_if_exists() {
    local src="$1"
    local dst="$2"

    if [ -f "$src" ]; then
        cp "$src" "$dst"
        echo "✓ Copied: $(basename "$src")"
        return 0
    fi
    return 1
}

# Sync kubeconfigs from home directory
echo "Syncing kubeconfigs from ~/kubeconfigs/..."
if [ -d "$HOME/kubeconfigs" ]; then
    for file in "$HOME/kubeconfigs"/*; do
        if [ -f "$file" ]; then
            copy_if_exists "$file" "$SECRETS_DIR/kubeconfigs/$(basename "$file")"
        fi
    done
else
    echo "⚠ ~/kubeconfigs/ not found"
fi
echo ""

# Sync .env files from project root
echo "Syncing .env files from project root..."
for envfile in "$PROJECT_ROOT"/.env*; do
    if [ -f "$envfile" ] && [[ "$(basename "$envfile")" != ".env.example" ]]; then
        copy_if_exists "$envfile" "$SECRETS_DIR/env/$(basename "$envfile")"
    fi
done
echo ""

# Sync .env files from services
echo "Syncing .env files from services/..."
for service_dir in "$PROJECT_ROOT/services"/*; do
    if [ -d "$service_dir" ]; then
        service_name=$(basename "$service_dir")
        for envfile in "$service_dir"/.env*; do
            if [ -f "$envfile" ] && [[ "$(basename "$envfile")" != ".env.example" ]]; then
                dst_name="${service_name}-$(basename "$envfile")"
                copy_if_exists "$envfile" "$SECRETS_DIR/env/$dst_name"
            fi
        done
    fi
done
echo ""

# List what we have
echo "=== Current Secrets Directory ==="
echo ""
echo "Environment files:"
if [ -d "$SECRETS_DIR/env" ] && [ "$(ls -A "$SECRETS_DIR/env" 2>/dev/null)" ]; then
    ls -lh "$SECRETS_DIR/env/"
else
    echo "  (none)"
fi
echo ""

echo "Kubeconfig files:"
if [ -d "$SECRETS_DIR/kubeconfigs" ] && [ "$(ls -A "$SECRETS_DIR/kubeconfigs" 2>/dev/null)" ]; then
    ls -lh "$SECRETS_DIR/kubeconfigs/"
else
    echo "  (none)"
fi
echo ""

echo "=== Next Steps ==="
echo ""
echo "1. Review the files in secrets/ directory"
echo "2. Commit and push:"
echo "   git add secrets/"
echo "   git commit -m 'Update secrets'"
echo "   git push"
echo ""
echo "3. On other machines, pull to sync:"
echo "   git pull"
echo ""
