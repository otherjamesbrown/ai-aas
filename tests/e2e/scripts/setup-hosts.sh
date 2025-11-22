#!/bin/bash
# Script to add development environment ingress entries to /etc/hosts

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Get ingress IP from cluster
get_ingress_ip() {
    local namespace="${1:-development}"
    local kubeconfig="${KUBECONFIG:-${HOME}/kubeconfigs/kubeconfig-development.yaml}"
    
    if [ -f "$kubeconfig" ]; then
        export KUBECONFIG="$kubeconfig"
    fi
    
    # Try to get ingress IP
    local ip=$(kubectl get ingress -n "$namespace" -o jsonpath='{.items[0].status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")
    
    if [ -z "$ip" ]; then
        # Try hostname
        local hostname=$(kubectl get ingress -n "$namespace" -o jsonpath='{.items[0].status.loadBalancer.ingress[0].hostname}' 2>/dev/null || echo "")
        if [ -n "$hostname" ]; then
            # Resolve hostname to IP
            ip=$(dig +short "$hostname" 2>/dev/null | head -1 || echo "")
        fi
    fi
    
    echo "$ip"
}

# Add entries to /etc/hosts
add_hosts_entries() {
    local ip="$1"
    
    if [ -z "$ip" ]; then
        echo -e "${RED}Error: Could not determine ingress IP${NC}"
        echo "Please provide the ingress IP address:"
        read -p "Ingress IP: " ip
        if [ -z "$ip" ]; then
            echo -e "${RED}Error: IP address is required${NC}"
            exit 1
        fi
    fi
    
    echo -e "${YELLOW}Adding entries to /etc/hosts for IP: $ip${NC}"
    
    # Entries to add
    local entries=(
        "$ip api.dev.ai-aas.local"
        "$ip portal.dev.ai-aas.local"
        "$ip user-org.api.ai-aas.dev"
        "$ip router.api.ai-aas.dev"
        "$ip analytics.api.ai-aas.dev"
    )
    
    # Check if entries already exist
    for entry in "${entries[@]}"; do
        local hostname=$(echo "$entry" | awk '{print $2}')
        if grep -q "$hostname" /etc/hosts 2>/dev/null; then
            echo -e "${YELLOW}  Entry for $hostname already exists, skipping...${NC}"
        else
            echo "$entry" | sudo tee -a /etc/hosts > /dev/null
            echo -e "${GREEN}  ✓ Added: $entry${NC}"
        fi
    done
    
    echo -e "${GREEN}✓ Hosts entries added successfully${NC}"
}

# Main
main() {
    echo -e "${GREEN}=== Setting up /etc/hosts for development environment ===${NC}"
    echo ""
    
    # Try to get IP from cluster
    echo -e "${YELLOW}Attempting to get ingress IP from cluster...${NC}"
    local ip=$(get_ingress_ip)
    
    if [ -z "$ip" ]; then
        echo -e "${YELLOW}Could not automatically determine ingress IP${NC}"
        echo "Please provide the ingress IP address or load balancer IP:"
        read -p "Ingress IP: " ip
    else
        echo -e "${GREEN}Found ingress IP: $ip${NC}"
    fi
    
    echo ""
    add_hosts_entries "$ip"
    
    echo ""
    echo -e "${GREEN}Setup complete! You can now run tests with:${NC}"
    echo "  cd tests/e2e"
    echo "  export USER_ORG_SERVICE_URL=http://api.dev.ai-aas.local"
    echo "  export API_ROUTER_SERVICE_URL=http://api.dev.ai-aas.local"
    echo "  make test-dev-internet"
}

main "$@"

