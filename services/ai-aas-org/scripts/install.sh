#!/bin/bash
# AI-as-a-Service Organization CLI Installer
# Usage: curl -sSL https://get.ai-aas.example.com/org | bash
#
# Environment variables:
#   AI_AAS_ORG_VERSION  - Specific version to install (default: latest)
#   AI_AAS_ORG_INSTALL_DIR - Installation directory (default: /usr/local/bin)

set -e

# Configuration
REPO="otherjamesbrown/ai-aas"
BINARY_NAME="ai-aas-org"
DEFAULT_INSTALL_DIR="/usr/local/bin"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print functions
print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}!${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Detect OS and architecture
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)

    case "$os" in
        linux)   OS="linux" ;;
        darwin)  OS="darwin" ;;
        *)       print_error "Unsupported OS: $os"; exit 1 ;;
    esac

    case "$arch" in
        x86_64|amd64)  ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *)             print_error "Unsupported architecture: $arch"; exit 1 ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    print_info "Detected platform: $PLATFORM"
}

# Get latest version from GitHub releases
get_latest_version() {
    if command -v curl &> /dev/null; then
        VERSION=$(curl -sI "https://github.com/${REPO}/releases/latest" | grep -i "location:" | sed 's/.*tag\///' | tr -d '\r\n')
    elif command -v wget &> /dev/null; then
        VERSION=$(wget -qO- --server-response "https://github.com/${REPO}/releases/latest" 2>&1 | grep -i "location:" | sed 's/.*tag\///' | tr -d '\r\n')
    else
        print_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi

    if [ -z "$VERSION" ]; then
        print_error "Could not determine latest version"
        exit 1
    fi

    print_info "Latest version: $VERSION"
}

# Download and install binary
install_binary() {
    local install_dir="${AI_AAS_ORG_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
    local tmp_dir=$(mktemp -d)
    local download_url="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}_${VERSION}_${PLATFORM}.tar.gz"

    print_info "Downloading ${BINARY_NAME} ${VERSION}..."

    if command -v curl &> /dev/null; then
        curl -sL "$download_url" -o "${tmp_dir}/${BINARY_NAME}.tar.gz"
    else
        wget -q "$download_url" -O "${tmp_dir}/${BINARY_NAME}.tar.gz"
    fi

    print_info "Extracting..."
    tar -xzf "${tmp_dir}/${BINARY_NAME}.tar.gz" -C "${tmp_dir}"

    print_info "Installing to ${install_dir}..."

    # Check if we need sudo
    if [ -w "$install_dir" ]; then
        mv "${tmp_dir}/${BINARY_NAME}" "${install_dir}/"
        chmod +x "${install_dir}/${BINARY_NAME}"
    else
        print_warning "Need sudo to install to ${install_dir}"
        sudo mv "${tmp_dir}/${BINARY_NAME}" "${install_dir}/"
        sudo chmod +x "${install_dir}/${BINARY_NAME}"
    fi

    # Cleanup
    rm -rf "${tmp_dir}"

    print_success "Installed ${BINARY_NAME} to ${install_dir}/${BINARY_NAME}"
}

# Verify installation
verify_installation() {
    if command -v "${BINARY_NAME}" &> /dev/null; then
        local installed_version=$("${BINARY_NAME}" version 2>/dev/null | grep -i version | head -1 || echo "unknown")
        print_success "Installation verified: ${installed_version}"
    else
        print_warning "Binary installed but not in PATH. Add ${AI_AAS_ORG_INSTALL_DIR:-$DEFAULT_INSTALL_DIR} to your PATH."
    fi
}

# Print next steps
print_next_steps() {
    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  Installation Complete!${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo "Next steps:"
    echo ""
    echo "  1. Initialize with your bootstrap key:"
    echo "     ${BINARY_NAME} init --key <your-bootstrap-key>"
    echo ""
    echo "  2. Or run the guided setup:"
    echo "     ${BINARY_NAME} --guided"
    echo ""
    echo "  3. View all commands:"
    echo "     ${BINARY_NAME} --help"
    echo ""
    echo "For documentation, visit:"
    echo "  https://docs.ai-aas.example.com/org-cli"
    echo ""
}

# Main
main() {
    echo ""
    echo "╔═══════════════════════════════════════════════╗"
    echo "║  AI-as-a-Service Organization CLI Installer   ║"
    echo "╚═══════════════════════════════════════════════╝"
    echo ""

    detect_platform

    if [ -n "${AI_AAS_ORG_VERSION:-}" ]; then
        VERSION="$AI_AAS_ORG_VERSION"
        print_info "Using specified version: $VERSION"
    else
        get_latest_version
    fi

    install_binary
    verify_installation
    print_next_steps
}

main "$@"
