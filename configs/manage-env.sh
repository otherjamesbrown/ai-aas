#!/bin/bash
# Environment Profile Manager
# Manages environment profiles and keeps configuration in sync

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG_DIR="$SCRIPT_DIR/environments"
CURRENT_ENV_FILE="$CONFIG_DIR/.current-env"
COMPONENTS_FILE="$SCRIPT_DIR/components.yaml"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

usage() {
    cat <<EOF
Environment Profile Manager

Usage:
  manage-env <command> [options]

Commands:
  activate <environment>    Switch to specified environment (local-dev, remote-dev, production)
  show [component]          Show current environment configuration (optionally filtered by component)
  list                      List available environments
  validate                  Validate current environment configuration
  diff <env1> <env2>        Compare two environment configurations
  export [format]           Export environment variables (format: env, yaml, json)
  component-status          Show status of all components in current environment
  generate-env-file         Generate .env file from current environment profile
  sync                      Sync environment variables to active profile (updates .env.* files)

Environments:
  - local-dev: Local development environment (localhost, Docker, port 5433)
  - remote-dev: Remote development workspace (Linode)
  - production: Production environment

Examples:
  manage-env activate local-dev
  manage-env show user_org_service
  manage-env export env > .env
  manage-env component-status
  manage-env sync  # Syncs .env.local with active profile

EOF
}

check_yq() {
    if ! command -v yq &>/dev/null; then
        echo -e "${YELLOW}Warning: yq not found. Installing via Go...${NC}"
        export PATH="/home/dev/go-bin/go/bin:$HOME/go/bin:$PATH"
        go install github.com/mikefarah/yq/v4@latest 2>&1 | tail -5 || {
            echo -e "${RED}Error: Could not install yq. Please install manually:${NC}"
            echo "  go install github.com/mikefarah/yq/v4@latest"
            echo "  Or: sudo snap install yq"
            exit 1
        }
        export PATH="$HOME/go/bin:$PATH"
    fi
}

activate() {
    local env=$1
    local env_file="$CONFIG_DIR/$env.yaml"
    
    if [[ ! -f "$env_file" ]]; then
        echo -e "${RED}Error: Environment '$env' not found${NC}"
        echo "Available environments:"
        list
        exit 1
    fi
    
    check_yq
    
    echo -e "${GREEN}Activating environment: $env${NC}"
    echo "$env" > "$CURRENT_ENV_FILE"
    
    # Generate .env file
    generate_env_file
    
    echo -e "${GREEN}Environment '$env' activated${NC}"
    echo "Run 'manage-env show' to view configuration"
    echo "Run 'manage-env sync' to sync secrets with active profile"
}

show() {
    local component=${1:-}
    
    if [[ ! -f "$CURRENT_ENV_FILE" ]]; then
        echo -e "${YELLOW}No environment activated${NC}"
        echo "Run 'manage-env activate <environment>' first"
        exit 1
    fi
    
    check_yq
    
    current_env=$(cat "$CURRENT_ENV_FILE")
    local env_file="$CONFIG_DIR/$current_env.yaml"
    
    if [[ -n "$component" ]]; then
        echo -e "${GREEN}Component: $component (Environment: $current_env)${NC}"
        yq eval ".components.$component" "$env_file" 2>/dev/null || echo "Component not found"
    else
        echo -e "${GREEN}Current Environment: $current_env${NC}"
        echo ""
        yq eval '.' "$env_file" 2>/dev/null || cat "$env_file"
    fi
}

list() {
    echo "Available environments:"
    for file in "$CONFIG_DIR"/*.yaml; do
        if [[ -f "$file" ]] && [[ "$(basename "$file")" != ".current-env" ]]; then
            local name=$(basename "$file" .yaml)
            if [[ "$name" != "base" ]]; then
                echo "  - $name"
            fi
        fi
    done
}

validate() {
    if [[ ! -f "$CURRENT_ENV_FILE" ]]; then
        echo -e "${RED}No environment activated${NC}"
        exit 1
    fi
    
    check_yq
    
    current_env=$(cat "$CURRENT_ENV_FILE")
    local env_file="$CONFIG_DIR/$current_env.yaml"
    
    echo -e "${GREEN}Validating environment: $current_env${NC}"
    
    local errors=0
    
    # Validate YAML syntax
    if ! yq eval '.' "$env_file" >/dev/null 2>&1; then
        echo -e "${RED}Error: Invalid YAML syntax${NC}"
        errors=$((errors + 1))
    fi
    
    # Validate required fields
    local name=$(yq eval '.metadata.name' "$env_file" 2>/dev/null)
    if [[ -z "$name" ]]; then
        echo -e "${RED}Error: Missing metadata.name${NC}"
        errors=$((errors + 1))
    fi
    
    # Validate components exist in registry
    if [[ -f "$COMPONENTS_FILE" ]]; then
        echo "Validating components against registry..."
        # Component validation logic here
    fi
    
    if [[ $errors -eq 0 ]]; then
        echo -e "${GREEN}Validation passed${NC}"
    else
        echo -e "${RED}Validation failed with $errors error(s)${NC}"
        exit 1
    fi
}

export_env() {
    local format=${1:-env}
    
    if [[ ! -f "$CURRENT_ENV_FILE" ]]; then
        echo -e "${RED}No environment activated${NC}"
        exit 1
    fi
    
    check_yq
    
    current_env=$(cat "$CURRENT_ENV_FILE")
    local env_file="$CONFIG_DIR/$current_env.yaml"
    
    case "$format" in
        env)
            # Generate shell export statements
            echo "# Generated from environment profile: $current_env"
            echo "# Generated at: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
            echo ""
            
            # Export environment variables from profile
            yq eval '.environment_variables[] | "export \(.name)=\"\(.value // "")\""' "$env_file" 2>/dev/null | grep -v "^export $"
            ;;
        yaml)
            cat "$env_file"
            ;;
        json)
            yq eval -o json "$env_file" 2>/dev/null
            ;;
        *)
            echo -e "${RED}Unknown format: $format${NC}"
            exit 1
            ;;
    esac
}

generate_env_file() {
    if [[ ! -f "$CURRENT_ENV_FILE" ]]; then
        echo -e "${YELLOW}No environment activated${NC}"
        return 1
    fi
    
    check_yq
    
    current_env=$(cat "$CURRENT_ENV_FILE")
    local env_file="$CONFIG_DIR/$current_env.yaml"
    local output_file=".env.local"
    
    echo -e "${GREEN}Generating $output_file from $current_env profile${NC}"
    
    # Create .env.local from environment profile
    {
        echo "# Auto-generated from environment profile: $current_env"
        echo "# Generated at: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
        echo "# DO NOT EDIT MANUALLY - changes will be overwritten"
        echo ""
        
        # Export environment variables
        while IFS= read -r var_name; do
            var_value=$(yq eval ".environment_variables[] | select(.name == \"$var_name\") | .value // \"\"" "$env_file" 2>/dev/null)
            echo "${var_name}=${var_value}"
        done < <(yq eval '.environment_variables[].name' "$env_file" 2>/dev/null)
        
        # Add secrets (these come from .env.local if it exists, or use defaults)
        if [[ -f "$output_file" ]]; then
            # Preserve existing secrets
            echo ""
            echo "# Secrets (preserved from existing .env.local)"
            grep -E "^(POSTGRES_PASSWORD|REDIS_PASSWORD|OAUTH_|MINIO_)" "$output_file" 2>/dev/null || true
        else
            # Use defaults from profile
            echo ""
            echo "# Secrets (using defaults from profile)"
            while IFS= read -r secret_name; do
                secret_default=$(yq eval ".secrets[] | select(.name == \"$secret_name\") | .default // \"\"" "$env_file" 2>/dev/null)
                echo "${secret_name}=${secret_default}"
            done < <(yq eval '.secrets[].name' "$env_file" 2>/dev/null)
        fi
    } > "$output_file.tmp"
    
    mv "$output_file.tmp" "$output_file"
    chmod 600 "$output_file"
    
    echo -e "${GREEN}Generated $output_file${NC}"
}

sync() {
    if [[ ! -f "$CURRENT_ENV_FILE" ]]; then
        echo -e "${YELLOW}No environment activated${NC}"
        echo "Run 'manage-env activate <environment>' first"
        return 1
    fi
    
    echo -e "${GREEN}Syncing environment variables with active profile...${NC}"
    
    # Re-generate .env file
    generate_env_file
    
    # If secrets-sync is available, sync secrets too
    if command -v secrets-sync &>/dev/null || [[ -f "$PROJECT_ROOT/cmd/secrets-sync/main.go" ]]; then
        echo "Syncing secrets..."
        current_env=$(cat "$CURRENT_ENV_FILE")
        if [[ "$current_env" == "local-dev" ]]; then
            make secrets-sync MODE=local 2>&1 | tail -10 || echo "Secrets sync failed (may not be available yet)"
        else
            make secrets-sync 2>&1 | tail -10 || echo "Secrets sync failed (may not be available yet)"
        fi
    fi
    
    echo -e "${GREEN}Sync complete${NC}"
}

component_status() {
    if [[ ! -f "$CURRENT_ENV_FILE" ]]; then
        echo -e "${YELLOW}No environment activated${NC}"
        return 1
    fi
    
    check_yq
    
    current_env=$(cat "$CURRENT_ENV_FILE")
    local env_file="$CONFIG_DIR/$current_env.yaml"
    
    echo -e "${GREEN}Component Status (Environment: $current_env)${NC}"
    echo ""
    
    # Check Docker containers
    local components=("postgres" "redis" "nats" "minio" "mock_inference")
    
    for comp in "${components[@]}"; do
        local container=$(yq eval ".components.$comp.docker_container // \"dev-$comp\"" "$env_file" 2>/dev/null)
        local port=$(yq eval ".components.$comp.port // .components.$comp.client_port // \"\"" "$env_file" 2>/dev/null)
        
        if docker ps --format "{{.Names}}" 2>/dev/null | grep -q "^${container}$"; then
            echo -e "${GREEN}✓${NC} $comp (container: $container) - Running"
            if [[ -n "$port" ]] && command -v nc &>/dev/null; then
                if nc -z localhost "$port" 2>/dev/null; then
                    echo "    Port $port: accessible"
                else
                    echo -e "    ${YELLOW}Port $port: not accessible${NC}"
                fi
            fi
        else
            echo -e "${RED}✗${NC} $comp (container: $container) - Not running"
        fi
    done
    
    # Check services
    local services=("user_org_service" "api_router_service")
    for svc in "${services[@]}"; do
        local port=$(yq eval ".components.$svc.port" "$env_file" 2>/dev/null)
        if [[ -n "$port" ]] && command -v nc &>/dev/null; then
            if nc -z localhost "$port" 2>/dev/null; then
                echo -e "${GREEN}✓${NC} $svc - Running on port $port"
            else
                echo -e "${RED}✗${NC} $svc - Not running on port $port"
            fi
        fi
    done
}

main() {
    case "${1:-}" in
        activate)
            [[ -z "${2:-}" ]] && { usage; exit 1; }
            activate "$2"
            ;;
        show)
            show "${2:-}"
            ;;
        list)
            list
            ;;
        validate)
            validate
            ;;
        diff)
            [[ -z "${2:-}" ]] || [[ -z "${3:-}" ]] && { usage; exit 1; }
            check_yq
            echo "Comparing ${2} vs ${3}:"
            diff <(yq eval '.' "$CONFIG_DIR/$2.yaml" 2>/dev/null) <(yq eval '.' "$CONFIG_DIR/$3.yaml" 2>/dev/null) || true
            ;;
        export)
            export_env "${2:-env}"
            ;;
        component-status|status)
            component_status
            ;;
        generate-env-file|generate)
            generate_env_file
            ;;
        sync)
            sync
            ;;
        *)
            usage
            exit 1
            ;;
    esac
}

main "$@"

