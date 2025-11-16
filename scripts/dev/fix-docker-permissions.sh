#!/usr/bin/env bash
# Script to diagnose and fix Docker permission issues permanently
# Usage: ./scripts/dev/fix-docker-permissions.sh

set -euo pipefail

echo "🔍 Diagnosing Docker Permission Issues..."
echo ""

# Check current user
CURRENT_USER=$(whoami)
echo "Current user: ${CURRENT_USER}"

# Check if docker group exists
echo ""
echo "1. Checking docker group..."
if getent group docker >/dev/null 2>&1; then
    echo "   ✅ docker group exists"
    DOCKER_GID=$(getent group docker | cut -d: -f3)
    echo "   Group ID: ${DOCKER_GID}"
else
    echo "   ❌ docker group does not exist!"
    echo "   Creating docker group..."
    sudo groupadd docker
fi

# Check if user is in docker group
echo ""
echo "2. Checking if user is in docker group..."
if groups | grep -q docker; then
    echo "   ✅ User ${CURRENT_USER} is in docker group"
    echo "   Current groups: $(groups)"
else
    echo "   ❌ User ${CURRENT_USER} is NOT in docker group"
    echo "   Adding user to docker group..."
    sudo usermod -aG docker "${CURRENT_USER}"
    echo "   ✅ User added to docker group"
    echo ""
    echo "   ⚠️  IMPORTANT: You need to log out and back in for this to take effect!"
    echo "   Or run: newgrp docker"
fi

# Check Docker socket permissions
echo ""
echo "3. Checking Docker socket permissions..."
if [ -S /var/run/docker.sock ]; then
    SOCK_PERMS=$(stat -c "%a %U:%G" /var/run/docker.sock)
    echo "   Socket permissions: ${SOCK_PERMS}"
    
    if [ -r /var/run/docker.sock ] && [ -w /var/run/docker.sock ]; then
        echo "   ✅ Socket is readable and writable"
    else
        echo "   ⚠️  Socket may not be accessible"
    fi
    
    # Check if socket is owned by docker group
    SOCK_GROUP=$(stat -c "%G" /var/run/docker.sock)
    if [ "${SOCK_GROUP}" = "docker" ]; then
        echo "   ✅ Socket is owned by docker group"
    else
        echo "   ⚠️  Socket is owned by ${SOCK_GROUP}, not docker group"
        echo "   Fixing socket ownership..."
        sudo chown root:docker /var/run/docker.sock
        sudo chmod 660 /var/run/docker.sock
        echo "   ✅ Socket ownership fixed"
    fi
else
    echo "   ❌ Docker socket not found at /var/run/docker.sock"
    echo "   Is Docker daemon running?"
fi

# Test Docker access
echo ""
echo "4. Testing Docker access..."
if docker ps >/dev/null 2>&1; then
    echo "   ✅ Docker is accessible!"
    docker ps --format "table {{.Names}}\t{{.Status}}" | head -5
else
    echo "   ❌ Docker is NOT accessible"
    echo ""
    echo "   To fix this, try one of these:"
    echo "   1. Log out and log back in"
    echo "   2. Run: newgrp docker"
    echo "   3. Restart your terminal session"
    echo "   4. If still not working, restart Docker daemon:"
    echo "      sudo systemctl restart docker"
fi

echo ""
echo "📝 Summary:"
echo "   User: ${CURRENT_USER}"
echo "   In docker group: $(groups | grep -q docker && echo 'YES' || echo 'NO')"
echo "   Docker accessible: $(docker ps >/dev/null 2>&1 && echo 'YES' || echo 'NO')"
echo ""
echo "💡 If Docker is still not accessible after running this script:"
echo "   1. Log out completely and log back in"
echo "   2. Or restart your system"
echo "   3. Or run: newgrp docker (creates new shell with docker group)"

