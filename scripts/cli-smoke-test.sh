#!/usr/bin/env bash
#
# CLI Smoke Tests for AI-AAS Platform
# Runs health checks and functional tests against Development and Staging environments
#
# Usage: ./scripts/cli-smoke-test.sh [--dev-only|--staging-only] [--json]
#

set -o pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CLI="$PROJECT_ROOT/services/ai-aas-cli/ai-aas-cli"
ENV_FILE="$PROJECT_ROOT/secrets/env/.env"

# Environment endpoints
DEV_USER_ORG="https://user-org.dev.otherjamesbrown.com"
DEV_API_ROUTER="https://api.dev.otherjamesbrown.com"
DEV_ADMIN_API="https://admin-api.dev.otherjamesbrown.com"

STAGING_USER_ORG="https://user-org.staging.otherjamesbrown.com"
STAGING_API_ROUTER="https://api.staging.otherjamesbrown.com"
STAGING_ADMIN_API="https://admin-api.staging.otherjamesbrown.com"
STAGING_API_KEY="ai-aas__HYQk1SQgY4P_f2aMjYM39zL9NAxG63tcHn_Gx4If3M"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Parse arguments
RUN_DEV=true
RUN_STAGING=true
JSON_OUTPUT=false

for arg in "$@"; do
    case $arg in
        --dev-only)
            RUN_STAGING=false
            ;;
        --staging-only)
            RUN_DEV=false
            ;;
        --json)
            JSON_OUTPUT=true
            ;;
        --help|-h)
            echo "Usage: $0 [--dev-only|--staging-only] [--json]"
            echo ""
            echo "Options:"
            echo "  --dev-only      Run only development environment tests"
            echo "  --staging-only  Run only staging environment tests"
            echo "  --json          Output results as JSON"
            exit 0
            ;;
    esac
done

# Load environment variables
if [[ -f "$ENV_FILE" ]]; then
    source "$ENV_FILE"
fi

if [[ -z "$MASTER_ADMIN_API_KEY" && "$RUN_DEV" == "true" ]]; then
    echo "Error: MASTER_ADMIN_API_KEY not set. Source $ENV_FILE or set the variable."
    exit 1
fi

# Results storage
declare -A RESULTS

# Helper function to check health endpoint
check_health() {
    local name="$1"
    local url="$2"
    local result

    result=$(curl -sk --connect-timeout 5 --max-time 10 "$url" 2>/dev/null)
    if [[ $? -ne 0 ]]; then
        echo "unreachable"
        return 1
    fi

    local status
    status=$(echo "$result" | jq -r '.status // "unknown"' 2>/dev/null)
    if [[ "$status" == "ok" || "$status" == "healthy" ]]; then
        echo "healthy"
        return 0
    else
        echo "$status"
        return 1
    fi
}

# Helper function to run CLI command and extract JSON field
run_cli() {
    local output
    output=$("$CLI" "$@" --format json 2>&1)
    local exit_code=$?
    echo "$output"
    return $exit_code
}

# Helper function to extract field from JSON (handles multi-object output)
extract_json_field() {
    local json="$1"
    local field="$2"
    # Get the last JSON object (the actual response, not the audit log)
    echo "$json" | jq -s '.[-1]' 2>/dev/null | jq -r ".$field // empty" 2>/dev/null
}

# Run tests for a single environment
run_environment_tests() {
    local env_name="$1"
    local user_org_endpoint="$2"
    local api_router_endpoint="$3"
    local admin_api_endpoint="$4"
    local api_key="$5"

    local test_id
    test_id=$(date +%s)-$$-$RANDOM
    local org_slug="smoke-test-$test_id"
    local user_email="test-user-$test_id@smoke.test"

    local results=()
    local user_id=""
    local new_api_key=""

    # Health checks
    local user_org_health api_router_health admin_api_health
    user_org_health=$(check_health "user-org" "$user_org_endpoint/healthz")
    api_router_health=$(check_health "api-router" "$api_router_endpoint/v1/status/healthz")
    admin_api_health=$(check_health "admin-api" "$admin_api_endpoint/healthz")

    results+=("health_user_org:$user_org_health")
    results+=("health_api_router:$api_router_health")
    results+=("health_admin_api:$admin_api_health")

    # Test 1: Create Organization
    local org_output org_result
    org_output=$(run_cli org create \
        --name "Smoke Test $test_id" \
        --slug "$org_slug" \
        --user-org-endpoint "$user_org_endpoint" \
        --api-key "$api_key" 2>&1)

    if echo "$org_output" | grep -q '"outcome": "success"'; then
        results+=("create_org:PASS:$org_slug")
    else
        local error_msg
        error_msg=$(echo "$org_output" | grep -o '"error":[^,}]*' | head -1 || echo "unknown error")
        results+=("create_org:FAIL:$error_msg")
        # Can't continue without org
        echo "${results[*]}"
        return 1
    fi

    # Test 2: Create User
    local user_output
    user_output=$(run_cli user create \
        --org-id "$org_slug" \
        --email "$user_email" \
        --display-name "Smoke Test User" \
        --roles admin \
        --user-org-endpoint "$user_org_endpoint" \
        --api-key "$api_key" 2>&1)

    if echo "$user_output" | grep -q '"outcome": "success"'; then
        user_id=$(extract_json_field "$user_output" "userId")
        results+=("create_user:PASS:$user_id")
    else
        local error_msg
        error_msg=$(echo "$user_output" | grep -o '"error":[^,}]*' | head -1 || echo "unknown error")
        results+=("create_user:FAIL:$error_msg")
        # Try cleanup and return
        run_cli org delete --org-id "$org_slug" --force \
            --user-org-endpoint "$user_org_endpoint" --api-key "$api_key" >/dev/null 2>&1
        echo "${results[*]}"
        return 1
    fi

    # Test 3: Activate User
    local activate_output
    activate_output=$(run_cli user update \
        --org-id "$org_slug" \
        --user-id "$user_id" \
        --status active \
        --user-org-endpoint "$user_org_endpoint" \
        --api-key "$api_key" 2>&1)

    if echo "$activate_output" | grep -q '"outcome": "success"'; then
        results+=("activate_user:PASS:active")
    else
        local error_msg
        error_msg=$(echo "$activate_output" | grep -o '"error":[^,}]*' | head -1 || echo "unknown error")
        results+=("activate_user:FAIL:$error_msg")
    fi

    # Test 4: Create API Key
    local apikey_output
    apikey_output=$(run_cli apikey create \
        --org-id "$org_slug" \
        --user-id "$user_id" \
        --scopes "inference:read,inference:write,models:read" \
        --user-org-endpoint "$user_org_endpoint" \
        --api-key "$api_key" 2>&1)

    if echo "$apikey_output" | grep -q '"outcome": "success"'; then
        new_api_key=$(extract_json_field "$apikey_output" "token")
        local key_id
        key_id=$(extract_json_field "$apikey_output" "keyId")
        results+=("create_apikey:PASS:$key_id")
    else
        local error_msg
        error_msg=$(echo "$apikey_output" | grep -o '"error":[^,}]*' | head -1 || echo "unknown error")
        results+=("create_apikey:FAIL:$error_msg")
    fi

    # Test 5: List Models (with new API key)
    if [[ -n "$new_api_key" ]]; then
        local models_output
        models_output=$(curl -sk --connect-timeout 5 --max-time 10 \
            "$api_router_endpoint/v1/models" \
            -H "Authorization: Bearer $new_api_key" 2>&1)

        if echo "$models_output" | jq -e '.data' >/dev/null 2>&1; then
            local model_count
            model_count=$(echo "$models_output" | jq '.data | length')
            local has_mock
            has_mock=$(echo "$models_output" | jq '[.data[].id | select(contains("mock-backend"))] | length')

            if [[ "$has_mock" -gt 0 ]]; then
                results+=("list_models:FAIL:mock-backend detected")
            else
                local model_ids
                model_ids=$(echo "$models_output" | jq -r '[.data[].id] | join(", ")' | cut -c1-60)
                results+=("list_models:PASS:$model_count models ($model_ids)")
            fi
        else
            results+=("list_models:FAIL:invalid response")
        fi
    else
        results+=("list_models:SKIP:no API key")
    fi

    # Test 6: Inference Health
    local health_output
    health_output=$(curl -sk --connect-timeout 5 --max-time 10 \
        "$api_router_endpoint/v1/status/healthz" 2>&1)

    if echo "$health_output" | jq -e '.status == "healthy"' >/dev/null 2>&1; then
        results+=("inference_health:PASS:healthy")
    else
        local status
        status=$(echo "$health_output" | jq -r '.status // "unknown"' 2>/dev/null)
        results+=("inference_health:FAIL:$status")
    fi

    # Cleanup: Delete Organization
    local delete_output
    delete_output=$(run_cli org delete \
        --org-id "$org_slug" \
        --force \
        --user-org-endpoint "$user_org_endpoint" \
        --api-key "$api_key" 2>&1)

    if echo "$delete_output" | grep -q '"outcome": "success"'; then
        results+=("cleanup:PASS:deleted")
    else
        results+=("cleanup:FAIL:could not delete $org_slug")
    fi

    # Return results as space-separated string
    echo "${results[*]}"
}

# Parse results string into associative array
parse_results() {
    local env_name="$1"
    local results_str="$2"

    for item in $results_str; do
        local key="${item%%:*}"
        local rest="${item#*:}"
        RESULTS["${env_name}_${key}"]="$rest"
    done
}

# Print results in human-readable format
print_results() {
    local env_name="$1"
    local label="$2"

    echo ""
    echo "=== $label ==="
    echo ""

    # Health checks
    printf "%-15s %s\n" "Service" "Status"
    printf "%-15s %s\n" "-------" "------"

    for service in user_org api_router admin_api; do
        local status="${RESULTS[${env_name}_health_${service}]}"
        local icon="❌"
        [[ "$status" == "healthy" ]] && icon="✅"
        printf "%-15s %s %s\n" "$service" "$icon" "$status"
    done

    echo ""
    printf "%-20s %-8s %s\n" "Test" "Result" "Details"
    printf "%-20s %-8s %s\n" "----" "------" "-------"

    for test in create_org create_user activate_user create_apikey list_models inference_health cleanup; do
        local value="${RESULTS[${env_name}_${test}]}"
        local result="${value%%:*}"
        local details="${value#*:}"
        local icon="❌"
        [[ "$result" == "PASS" ]] && icon="✅"
        [[ "$result" == "SKIP" ]] && icon="⏭️"
        printf "%-20s %s %-6s %s\n" "$test" "$icon" "$result" "$details"
    done
}

# Print results as JSON
print_json_results() {
    local dev_results=""
    local staging_results=""

    if [[ "$RUN_DEV" == "true" ]]; then
        dev_results=$(cat <<EOF
    "development": {
      "health": {
        "user_org": "${RESULTS[dev_health_user_org]}",
        "api_router": "${RESULTS[dev_health_api_router]}",
        "admin_api": "${RESULTS[dev_health_admin_api]}"
      },
      "tests": {
        "create_org": "${RESULTS[dev_create_org]}",
        "create_user": "${RESULTS[dev_create_user]}",
        "activate_user": "${RESULTS[dev_activate_user]}",
        "create_apikey": "${RESULTS[dev_create_apikey]}",
        "list_models": "${RESULTS[dev_list_models]}",
        "inference_health": "${RESULTS[dev_inference_health]}",
        "cleanup": "${RESULTS[dev_cleanup]}"
      }
    }
EOF
)
    fi

    if [[ "$RUN_STAGING" == "true" ]]; then
        staging_results=$(cat <<EOF
    "staging": {
      "health": {
        "user_org": "${RESULTS[staging_health_user_org]}",
        "api_router": "${RESULTS[staging_health_api_router]}",
        "admin_api": "${RESULTS[staging_health_admin_api]}"
      },
      "tests": {
        "create_org": "${RESULTS[staging_create_org]}",
        "create_user": "${RESULTS[staging_create_user]}",
        "activate_user": "${RESULTS[staging_activate_user]}",
        "create_apikey": "${RESULTS[staging_create_apikey]}",
        "list_models": "${RESULTS[staging_list_models]}",
        "inference_health": "${RESULTS[staging_inference_health]}",
        "cleanup": "${RESULTS[staging_cleanup]}"
      }
    }
EOF
)
    fi

    local comma=""
    [[ -n "$dev_results" && -n "$staging_results" ]] && comma=","

    echo "{"
    echo "  \"timestamp\": \"$(date -Iseconds)\","
    echo "  \"environments\": {"
    [[ -n "$dev_results" ]] && echo "$dev_results$comma"
    [[ -n "$staging_results" ]] && echo "$staging_results"
    echo "  }"
    echo "}"
}

# Count failures
count_failures() {
    local env_name="$1"
    local count=0

    for test in create_org create_user activate_user create_apikey list_models inference_health cleanup; do
        local value="${RESULTS[${env_name}_${test}]}"
        local result="${value%%:*}"
        [[ "$result" == "FAIL" ]] && ((count++))
    done

    echo "$count"
}

# Count passes
count_passes() {
    local env_name="$1"
    local count=0

    for test in create_org create_user activate_user create_apikey list_models inference_health cleanup; do
        local value="${RESULTS[${env_name}_${test}]}"
        local result="${value%%:*}"
        [[ "$result" == "PASS" ]] && ((count++))
    done

    echo "$count"
}

# Print final summary table
print_summary_table() {
    echo ""
    echo "╔══════════════════════════════════════════════════════════════════════════════╗"
    echo "║                           CLI SMOKE TEST SUMMARY                             ║"
    echo "╠═════════════╦════════════════════════════════════════════╦═══════╦═══════════╣"
    echo "║ Environment ║ Base URL                                   ║ Tests ║ Status    ║"
    echo "╠═════════════╬════════════════════════════════════════════╬═══════╬═══════════╣"

    if [[ "$RUN_DEV" == "true" ]]; then
        local dev_passes=$(count_passes "dev")
        local dev_fails=$(count_failures "dev")
        local dev_total=$((dev_passes + dev_fails))
        local dev_status="PASS"
        local dev_icon="✅"
        if [[ $dev_fails -gt 0 ]]; then
            dev_status="FAIL"
            dev_icon="❌"
        fi
        printf "║ %-11s ║ %-42s ║ %s/%s   ║ %s %-6s ║\n" \
            "Development" "api.dev.otherjamesbrown.com" "$dev_passes" "$dev_total" "$dev_icon" "$dev_status"
    fi

    if [[ "$RUN_STAGING" == "true" ]]; then
        local staging_passes=$(count_passes "staging")
        local staging_fails=$(count_failures "staging")
        local staging_total=$((staging_passes + staging_fails))
        local staging_status="PASS"
        local staging_icon="✅"
        if [[ $staging_fails -gt 0 ]]; then
            staging_status="FAIL"
            staging_icon="❌"
        fi
        printf "║ %-11s ║ %-42s ║ %s/%s   ║ %s %-6s ║\n" \
            "Staging" "api.staging.otherjamesbrown.com" "$staging_passes" "$staging_total" "$staging_icon" "$staging_status"
    fi

    echo "╚═════════════╩════════════════════════════════════════════╩═══════╩═══════════╝"
    echo ""

    # Print detailed test breakdown
    echo "┌─────────────────────┬─────────────┬─────────────┐"
    echo "│ Test                │ Development │ Staging     │"
    echo "├─────────────────────┼─────────────┼─────────────┤"

    for test in create_org create_user activate_user create_apikey list_models inference_health cleanup; do
        local dev_result="-"
        local staging_result="-"

        if [[ "$RUN_DEV" == "true" ]]; then
            local dev_value="${RESULTS[dev_${test}]}"
            local dev_r="${dev_value%%:*}"
            if [[ "$dev_r" == "PASS" ]]; then
                dev_result="✅ PASS"
            elif [[ "$dev_r" == "FAIL" ]]; then
                dev_result="❌ FAIL"
            elif [[ "$dev_r" == "SKIP" ]]; then
                dev_result="⏭️  SKIP"
            fi
        fi

        if [[ "$RUN_STAGING" == "true" ]]; then
            local staging_value="${RESULTS[staging_${test}]}"
            local staging_r="${staging_value%%:*}"
            if [[ "$staging_r" == "PASS" ]]; then
                staging_result="✅ PASS"
            elif [[ "$staging_r" == "FAIL" ]]; then
                staging_result="❌ FAIL"
            elif [[ "$staging_r" == "SKIP" ]]; then
                staging_result="⏭️  SKIP"
            fi
        fi

        printf "│ %-19s │ %-11s │ %-11s │\n" "$test" "$dev_result" "$staging_result"
    done

    echo "└─────────────────────┴─────────────┴─────────────┘"
}

# Main execution
main() {
    local dev_results_str=""
    local staging_results_str=""

    if [[ "$JSON_OUTPUT" != "true" ]]; then
        echo "CLI Smoke Tests - $(date)"
        echo "Running tests..."
        echo ""
    fi

    # Run tests in parallel if both environments
    if [[ "$RUN_DEV" == "true" && "$RUN_STAGING" == "true" ]]; then
        # Create temp files for parallel execution
        local dev_tmp=$(mktemp)
        local staging_tmp=$(mktemp)

        # Run both in background
        (run_environment_tests "dev" "$DEV_USER_ORG" "$DEV_API_ROUTER" "$DEV_ADMIN_API" "$MASTER_ADMIN_API_KEY" > "$dev_tmp") &
        local dev_pid=$!

        (run_environment_tests "staging" "$STAGING_USER_ORG" "$STAGING_API_ROUTER" "$STAGING_ADMIN_API" "$STAGING_API_KEY" > "$staging_tmp") &
        local staging_pid=$!

        # Wait for both
        wait $dev_pid
        wait $staging_pid

        dev_results_str=$(cat "$dev_tmp")
        staging_results_str=$(cat "$staging_tmp")

        rm -f "$dev_tmp" "$staging_tmp"

    elif [[ "$RUN_DEV" == "true" ]]; then
        dev_results_str=$(run_environment_tests "dev" "$DEV_USER_ORG" "$DEV_API_ROUTER" "$DEV_ADMIN_API" "$MASTER_ADMIN_API_KEY")
    elif [[ "$RUN_STAGING" == "true" ]]; then
        staging_results_str=$(run_environment_tests "staging" "$STAGING_USER_ORG" "$STAGING_API_ROUTER" "$STAGING_ADMIN_API" "$STAGING_API_KEY")
    fi

    # Parse results
    [[ -n "$dev_results_str" ]] && parse_results "dev" "$dev_results_str"
    [[ -n "$staging_results_str" ]] && parse_results "staging" "$staging_results_str"

    # Output results
    if [[ "$JSON_OUTPUT" == "true" ]]; then
        print_json_results
    else
        # Print the summary table
        print_summary_table

        # Final status
        local total_failures=0
        [[ "$RUN_DEV" == "true" ]] && total_failures=$((total_failures + $(count_failures "dev")))
        [[ "$RUN_STAGING" == "true" ]] && total_failures=$((total_failures + $(count_failures "staging")))

        if [[ $total_failures -eq 0 ]]; then
            echo -e "${GREEN}All tests passed!${NC}"
        else
            echo -e "${RED}$total_failures test(s) failed${NC}"
        fi
    fi

    # Return exit code based on failures
    local exit_code=0
    [[ "$RUN_DEV" == "true" && $(count_failures "dev") -gt 0 ]] && exit_code=1
    [[ "$RUN_STAGING" == "true" && $(count_failures "staging") -gt 0 ]] && exit_code=1

    return $exit_code
}

main "$@"
