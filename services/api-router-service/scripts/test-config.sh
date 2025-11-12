#!/bin/bash
# Test script for Config Service integration
# This script helps run tests with or without etcd

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== Config Service Integration Tests ==="
echo ""

# Check if etcd endpoint is provided
ETCD_ENDPOINT="${ETCD_ENDPOINT:-localhost:2379}"

# Test etcd connectivity
echo "Checking etcd connectivity at $ETCD_ENDPOINT..."
if timeout 2 bash -c "cat < /dev/null > /dev/tcp/${ETCD_ENDPOINT/:/\/}" 2>/dev/null; then
    echo "✓ etcd is available at $ETCD_ENDPOINT"
    echo "Running full integration tests..."
    echo ""
    ETCD_ENDPOINT="$ETCD_ENDPOINT" go test -v "$SERVICE_DIR/internal/config" -run "TestLoader"
else
    echo "⚠ etcd not available at $ETCD_ENDPOINT"
    echo "Running fallback tests only..."
    echo ""
    go test -v "$SERVICE_DIR/internal/config" -run "TestLoader" -skip "FromEtcd|FromEtcd|Watch|Stop"
fi

echo ""
echo "=== Test Summary ==="
echo "To run with etcd, start etcd and set ETCD_ENDPOINT:"
echo "  docker run -d -p 2379:2379 quay.io/coreos/etcd:v3.6.6 etcd --listen-client-urls http://0.0.0.0:2379 --advertise-client-urls http://localhost:2379"
echo "  ETCD_ENDPOINT=localhost:2379 $0"

