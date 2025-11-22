#!/bin/bash
# Quick script to add hosts entries - run with sudo or copy the commands

INGRESS_IP="172.232.58.222"

echo "Adding entries to /etc/hosts for IP: $INGRESS_IP"
echo ""
echo "Run these commands (or run this script with sudo):"
echo ""
echo "sudo sh -c 'echo \"$INGRESS_IP api.dev.ai-aas.local\" >> /etc/hosts'"
echo "sudo sh -c 'echo \"$INGRESS_IP portal.dev.ai-aas.local\" >> /etc/hosts'"
echo "sudo sh -c 'echo \"$INGRESS_IP user-org.api.ai-aas.dev\" >> /etc/hosts'"
echo "sudo sh -c 'echo \"$INGRESS_IP router.api.ai-aas.dev\" >> /etc/hosts'"
echo "sudo sh -c 'echo \"$INGRESS_IP analytics.api.ai-aas.dev\" >> /etc/hosts'"
echo ""

# If running with sudo, add them directly
if [ "$EUID" -eq 0 ]; then
    echo "$INGRESS_IP api.dev.ai-aas.local" >> /etc/hosts
    echo "$INGRESS_IP portal.dev.ai-aas.local" >> /etc/hosts
    echo "$INGRESS_IP user-org.api.ai-aas.dev" >> /etc/hosts
    echo "$INGRESS_IP router.api.ai-aas.dev" >> /etc/hosts
    echo "$INGRESS_IP analytics.api.ai-aas.dev" >> /etc/hosts
    echo "✓ Entries added successfully"
else
    echo "Not running as root. Please run with sudo or copy the commands above."
fi

