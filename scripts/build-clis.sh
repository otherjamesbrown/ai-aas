#!/bin/bash
# Build script for ai-aas-cli and ai-aas-org CLI binaries
# Usage: ./scripts/build-clis.sh [--install]
#
# Options:
#   --install    Install binaries to ~/.local/bin after building

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
INSTALL_DIR="$HOME/.local/bin"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Parse arguments
INSTALL=false
for arg in "$@"; do
    case $arg in
        --install)
            INSTALL=true
            shift
            ;;
    esac
done

# Ensure install directory exists
if [ "$INSTALL" = true ]; then
    mkdir -p "$INSTALL_DIR"
fi

# Build ai-aas-cli
# NOTE: main.go is at service root, not in cmd/
log_info "Building ai-aas-cli..."
cd "$REPO_ROOT/services/ai-aas-cli"
rm -f ai-aas-cli
go build -o ai-aas-cli .
chmod +x ai-aas-cli
log_info "Built: $REPO_ROOT/services/ai-aas-cli/ai-aas-cli"

if [ "$INSTALL" = true ]; then
    cp ai-aas-cli "$INSTALL_DIR/"
    log_info "Installed: $INSTALL_DIR/ai-aas-cli"
fi

# Build ai-aas-org
# NOTE: main.go is at cmd/ai-aas-org/main.go
log_info "Building ai-aas-org..."
cd "$REPO_ROOT/services/ai-aas-org"
rm -f ai-aas-org
go build -o ai-aas-org ./cmd/ai-aas-org
chmod +x ai-aas-org
log_info "Built: $REPO_ROOT/services/ai-aas-org/ai-aas-org"

if [ "$INSTALL" = true ]; then
    cp ai-aas-org "$INSTALL_DIR/"
    log_info "Installed: $INSTALL_DIR/ai-aas-org"
fi

# Refresh shell hash table
hash -r 2>/dev/null || true

# Show versions
echo ""
log_info "Build complete!"
echo ""

if [ "$INSTALL" = true ]; then
    echo "Installed binaries:"
    echo "  $INSTALL_DIR/ai-aas-cli"
    echo "  $INSTALL_DIR/ai-aas-org"
    echo ""
    echo "Run 'hash -r' to refresh your shell's command cache."
else
    echo "Built binaries (not installed):"
    echo "  $REPO_ROOT/services/ai-aas-cli/ai-aas-cli"
    echo "  $REPO_ROOT/services/ai-aas-org/ai-aas-org"
    echo ""
    echo "To install to $INSTALL_DIR, run:"
    echo "  $0 --install"
fi
