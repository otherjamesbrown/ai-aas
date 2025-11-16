#!/usr/bin/env bash
# Install kubectl on Ubuntu using official Kubernetes apt repository
set -euo pipefail

echo "Installing kubectl..."

# 1. Add the signing key
if [ -f /tmp/kubernetes-release.key ]; then
    echo "Using downloaded key..."
    sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg < /tmp/kubernetes-release.key
else
    echo "Downloading signing key..."
    curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.31/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
fi

# 2. Add the Kubernetes apt repository
echo "Adding Kubernetes repository..."
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.31/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list

# 3. Update package list
echo "Updating package list..."
sudo apt-get update

# 3.5. Fix any broken dependencies
echo "Fixing broken dependencies..."
sudo apt --fix-broken install -y || true

# 4. Install kubectl
echo "Installing kubectl..."
sudo apt-get install -y kubectl

# 5. Verify installation
echo "Verifying installation..."
kubectl version --client

echo "kubectl installed successfully!"

