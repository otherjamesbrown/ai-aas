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
CLI_DIR="$PROJECT_ROOT/services/ai-aas-cli"
CLI="$CLI_DIR/ai-aas-cli"
ENV_FILE="$PROJECT_ROOT/secrets/env/.env"

# Build CLI to ensure we have the latest version
build_cli() {
    local quiet="${1:-false}"
    [[ "$quiet" != "true" ]] && echo "Building CLI..."
    pushd "$CLI_DIR" > /dev/null
    if CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ai-aas-cli . 2>&1; then
        [[ "$quiet" != "true" ]] && echo "CLI built successfully"
    else
        echo "Error: Failed to build CLI" >&2
        exit 1
    fi
    popd > /dev/null
}

# Environment endpoints
DEV_USER_ORG="https://user-org.dev.otherjamesbrown.com"
DEV_API_ROUTER="https://api.dev.otherjamesbrown.com"
DEV_ADMIN_API="https://admin-api.dev.otherjamesbrown.com"
DEV_KUBECONFIG="$PROJECT_ROOT/secrets/kubeconfigs/kubeconfig-development.yaml"
DEV_NAMESPACE="development"

STAGING_USER_ORG="https://user-org.staging.otherjamesbrown.com"
STAGING_API_ROUTER="https://api.staging.otherjamesbrown.com"
STAGING_ADMIN_API="https://admin-api.staging.otherjamesbrown.com"
STAGING_API_KEY="ai-aas__HYQk1SQgY4P_f2aMjYM39zL9NAxG63tcHn_Gx4If3M"
STAGING_KUBECONFIG="$PROJECT_ROOT/secrets/kubeconfigs/kubeconfig-staging.yaml"
STAGING_NAMESPACE="staging"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Parse arguments
RUN_DEV=true
RUN_STAGING=true
JSON_OUTPUT=false
VERBOSE=false

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
        --verbose|-v)
            VERBOSE=true
            ;;
        --help|-h)
            echo "Usage: $0 [--dev-only|--staging-only] [--json] [--verbose]"
            echo ""
            echo "Options:"
            echo "  --dev-only      Run only development environment tests"
            echo "  --staging-only  Run only staging environment tests"
            echo "  --json          Output results as JSON"
            echo "  --verbose, -v   Show published models table and Q&A for each inference"
            echo ""
            echo "By default, shows inference details (model, tokens, time) for ALL models."
            echo "With --verbose, additionally shows the published models table and"
            echo "the question/answer for each inference test."
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
declare -A VERBOSE_DATA

# Format duration from ISO timestamp to human-readable (e.g., "7d 17h", "2h 30m")
format_duration() {
    local timestamp="$1"
    if [[ -z "$timestamp" || "$timestamp" == "null" ]]; then
        echo "-"
        return
    fi

    local then_epoch now_epoch diff_seconds
    then_epoch=$(date -d "$timestamp" +%s 2>/dev/null || echo "0")
    now_epoch=$(date +%s)
    diff_seconds=$((now_epoch - then_epoch))

    if [[ $diff_seconds -lt 0 ]]; then
        echo "-"
        return
    fi

    local days hours minutes
    days=$((diff_seconds / 86400))
    hours=$(((diff_seconds % 86400) / 3600))
    minutes=$(((diff_seconds % 3600) / 60))

    if [[ $days -gt 0 ]]; then
        echo "${days}d ${hours}h"
    elif [[ $hours -gt 0 ]]; then
        echo "${hours}h ${minutes}m"
    else
        echo "${minutes}m"
    fi
}

# Fetch AIModel data from Kubernetes cluster
fetch_aimodels() {
    local kubeconfig="$1"
    local namespace="$2"
    local output_file="$3"

    if [[ ! -f "$kubeconfig" ]]; then
        echo "[]" > "$output_file"
        return 1
    fi

    kubectl --kubeconfig="$kubeconfig" get aimodels -n "$namespace" -o json 2>/dev/null | \
        jq '[.items[] | {
            internalName: .metadata.name,
            externalName: (.spec.externalName // .metadata.name),
            modelID: .spec.modelID,
            runtime: .spec.runtime,
            enabled: .spec.enabled,
            phase: (.status.phase // "Unknown"),
            statusChangedAt: (.status.statusChangedAt // .metadata.creationTimestamp)
        }]' > "$output_file" 2>/dev/null || echo "[]" > "$output_file"
}

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

# Sanitize string for use in results (replace spaces with underscores)
sanitize_result() {
    echo "$1" | tr ' ' '_' | tr -d '\n' | head -c 30
}

# Detect if a model is a VLM (Vision Language Model)
is_vlm() {
    local model_name="$1"
    local model_lower
    model_lower=$(echo "$model_name" | tr '[:upper:]' '[:lower:]')

    # VLM patterns (case-insensitive)
    if echo "$model_lower" | grep -qE '(qwen2-vl|llava|internvl|pixtral|phi-3-vision)'; then
        return 0  # true
    else
        return 1  # false
    fi
}

# Get embedded test image base64
get_test_image_base64() {
    # Embedded 128x128 cat image as JPEG base64 (~4KB, from cataas.com CC0)
    echo "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"
}

# Run tests for a single environment
run_environment_tests() {
    local env_name="$1"
    local user_org_endpoint="$2"
    local api_router_endpoint="$3"
    local admin_api_endpoint="$4"
    local api_key="$5"
    local kubeconfig="$6"
    local namespace="$7"

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
        error_msg=$(sanitize_result "$(echo "$org_output" | grep -o '"error":[^,}]*' | head -1 || echo "unknown_error")")
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
        error_msg=$(sanitize_result "$(echo "$user_output" | grep -o '"error":[^,}]*' | head -1 || echo "unknown_error")")
        results+=("create_user:FAIL:$error_msg")
        # Try cleanup and return
        run_cli org delete --org-id "$org_slug" --force \
            --user-org-endpoint "$user_org_endpoint" --api-key "$api_key" >/dev/null 2>&1
        echo "${results[*]}"
        return 1
    fi

    # Test 2.5: Grant Model Access (set to auto_grant mode)
    local model_access_output
    model_access_output=$(run_cli user model-access set-mode \
        --org-id "$org_slug" \
        --user-id "$user_id" \
        --mode auto_grant \
        --user-org-endpoint "$user_org_endpoint" \
        --api-key "$api_key" 2>&1)

    if echo "$model_access_output" | grep -q '"outcome": "success"'; then
        results+=("grant_model_access:PASS:auto_grant")
    else
        local error_msg
        error_msg=$(sanitize_result "$(echo "$model_access_output" | grep -o '"error":[^,}]*' | head -1 || echo "unknown_error")")
        results+=("grant_model_access:FAIL:$error_msg")
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
        error_msg=$(sanitize_result "$(echo "$activate_output" | grep -o '"error":[^,}]*' | head -1 || echo "unknown_error")")
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
        error_msg=$(sanitize_result "$(echo "$apikey_output" | grep -o '"error":[^,}]*' | head -1 || echo "unknown_error")")
        results+=("create_apikey:FAIL:$error_msg")
    fi

    # Test 5: List Models (with new API key)
    local models_json=""
    local first_model_id=""
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
                results+=("list_models:FAIL:mock-backend_detected")
            else
                local model_ids
                model_ids=$(echo "$models_output" | jq -r '[.data[].id] | join(", ")' | cut -c1-60)
                results+=("list_models:PASS:${model_count}_models")
                # Store full models JSON for verbose output
                models_json="$models_output"
                # Get first model for inference test
                first_model_id=$(echo "$models_output" | jq -r '.data[0].id // empty')
            fi
        else
            results+=("list_models:FAIL:invalid_response")
        fi
    else
        results+=("list_models:SKIP:no_API_key")
    fi

    # Test 6: Inference Test (chat completion)
    # Test ALL published models and report results for each
    local inference_json_array="[]"
    local inference_prompt="What is the capital of France?"
    local vlm_prompt="What animal is in this image? Answer in one word."
    if [[ -n "$new_api_key" ]]; then
        local all_model_ids
        all_model_ids=$(echo "$models_output" | jq -r '.data[].id' 2>/dev/null)
        local inference_pass_count=0
        local inference_fail_count=0
        local inference_results=()

        for model_id in $all_model_ids; do
            [[ -z "$model_id" ]] && continue

            local start_time end_time duration_ms
            start_time=$(date +%s%3N)

            # Detect if this is a VLM and prepare appropriate request
            local request_body
            local test_prompt
            if is_vlm "$model_id"; then
                # VLM: Send multimodal content (image + text)
                local image_base64
                image_base64=$(get_test_image_base64)
                test_prompt="$vlm_prompt"
                request_body=$(jq -n \
                    --arg model "$model_id" \
                    --arg prompt "$test_prompt" \
                    --arg image_data "data:image/jpeg;base64,$image_base64" \
                    '{
                        "model": $model,
                        "messages": [
                            {
                                "role": "user",
                                "content": [
                                    {
                                        "type": "image_url",
                                        "image_url": {"url": $image_data}
                                    },
                                    {
                                        "type": "text",
                                        "text": $prompt
                                    }
                                ]
                            }
                        ],
                        "max_tokens": 50
                    }')
            else
                # Regular LLM: Send simple text
                test_prompt="$inference_prompt"
                request_body=$(jq -n \
                    --arg model "$model_id" \
                    --arg prompt "$test_prompt" \
                    '{
                        "model": $model,
                        "messages": [{"role": "user", "content": $prompt}],
                        "max_tokens": 50
                    }')
            fi

            local inference_output
            inference_output=$(curl -sk --connect-timeout 10 --max-time 30 \
                "$api_router_endpoint/v1/chat/completions" \
                -H "Authorization: Bearer $new_api_key" \
                -H "Content-Type: application/json" \
                -d "$request_body" 2>&1)

            end_time=$(date +%s%3N)
            duration_ms=$((end_time - start_time))

            if echo "$inference_output" | jq -e '.choices[0].message.content' >/dev/null 2>&1; then
                # Parse all fields in one jq call for efficiency
                local parsed_fields
                parsed_fields=$(echo "$inference_output" | jq -r '[
                    .usage.prompt_tokens // 0,
                    .usage.completion_tokens // 0,
                    .usage.total_tokens // 0,
                    (.choices[0].message.content // "" | gsub("\n"; " ") | .[0:100])
                ] | @tsv')
                local prompt_tokens completion_tokens total_tokens response_text
                IFS=$'\t' read -r prompt_tokens completion_tokens total_tokens response_text <<< "$parsed_fields"
                # Escape quotes for JSON
                response_text=$(echo "$response_text" | sed 's/"/\\"/g')

                inference_results+=("{\"model\":\"$model_id\",\"status\":\"PASS\",\"prompt_tokens\":$prompt_tokens,\"completion_tokens\":$completion_tokens,\"total_tokens\":$total_tokens,\"duration_ms\":$duration_ms,\"prompt\":\"$test_prompt\",\"response\":\"$response_text\"}")
                ((inference_pass_count++))
            else
                local error_msg
                error_msg=$(echo "$inference_output" | jq -r '.error.message // .error // "unknown error"' 2>/dev/null | head -c 50 | sed 's/"/\\"/g')
                inference_results+=("{\"model\":\"$model_id\",\"status\":\"FAIL\",\"prompt_tokens\":0,\"completion_tokens\":0,\"total_tokens\":0,\"duration_ms\":$duration_ms,\"prompt\":\"$test_prompt\",\"response\":\"\",\"error\":\"$error_msg\"}")
                ((inference_fail_count++))
            fi
        done

        # Determine overall inference test result
        local total_models=$((inference_pass_count + inference_fail_count))
        if [[ $total_models -eq 0 ]]; then
            results+=("inference:SKIP:no_models")
        elif [[ $inference_fail_count -eq 0 ]]; then
            results+=("inference:PASS:${inference_pass_count}/${total_models}_models")
        elif [[ $inference_pass_count -eq 0 ]]; then
            results+=("inference:FAIL:0/${total_models}_models")
        else
            results+=("inference:PASS:${inference_pass_count}/${total_models}_models")
        fi

        # Build JSON array of all inference results
        local old_ifs="$IFS"
        IFS=','
        inference_json_array="[${inference_results[*]}]"
        IFS="$old_ifs"
    else
        results+=("inference:SKIP:no_API_key")
    fi

    # Test 7: Inference Health
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
        results+=("cleanup:FAIL:delete_failed_$org_slug")
    fi

    # Return results as space-separated string
    echo "${results[*]}"

    # Write data to temp files (always write, cleanup happens in print_inference_details)
    local verbose_dir="/tmp/smoke-test-verbose-${SMOKE_TEST_ID:-$$}"
    mkdir -p "$verbose_dir"
    [[ -n "$models_json" ]] && echo "$models_json" > "$verbose_dir/${env_name}_models.json"
    [[ "$inference_json_array" != "[]" ]] && echo "$inference_json_array" > "$verbose_dir/${env_name}_inference.json"

    # Fetch AIModel data from Kubernetes for enhanced model table
    if [[ -n "$kubeconfig" && -n "$namespace" ]]; then
        fetch_aimodels "$kubeconfig" "$namespace" "$verbose_dir/${env_name}_aimodels.json"
    fi
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

    for test in $TESTS; do
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
        "grant_model_access": "${RESULTS[dev_grant_model_access]}",
        "activate_user": "${RESULTS[dev_activate_user]}",
        "create_apikey": "${RESULTS[dev_create_apikey]}",
        "list_models": "${RESULTS[dev_list_models]}",
        "inference": "${RESULTS[dev_inference]}",
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
        "grant_model_access": "${RESULTS[staging_grant_model_access]}",
        "activate_user": "${RESULTS[staging_activate_user]}",
        "create_apikey": "${RESULTS[staging_create_apikey]}",
        "list_models": "${RESULTS[staging_list_models]}",
        "inference": "${RESULTS[staging_inference]}",
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

# Test list
TESTS="create_org create_user grant_model_access activate_user create_apikey list_models inference inference_health cleanup"

# Count failures
count_failures() {
    local env_name="$1"
    local count=0

    for test in $TESTS; do
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

    for test in $TESTS; do
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
    echo "┌──────────────────────┬─────────────┬─────────────┐"
    echo "│ Test                 │ Development │ Staging     │"
    echo "├──────────────────────┼─────────────┼─────────────┤"

    for test in $TESTS; do
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

        printf "│ %-20s │ %-11s │ %-11s │\n" "$test" "$dev_result" "$staging_result"
    done

    echo "└──────────────────────┴─────────────┴─────────────┘"
}

# Print inference details for all models (shown by default)
print_inference_details() {
    local verbose_dir="/tmp/smoke-test-verbose-${SMOKE_TEST_ID:-$$}"

    echo ""
    echo "╔══════════════════════════════════════════════════════════════════════════════════════╗"
    echo "║                               INFERENCE DETAILS                                      ║"
    echo "╠═════════════╦════════════════════════════════════════╦════════╦════════╦════════════╣"
    echo "║ Environment ║ Model                                  ║ Prompt ║ Compl. ║ Time (ms)  ║"
    echo "╠═════════════╬════════════════════════════════════════╬════════╬════════╬════════════╣"

    for env in dev staging; do
        [[ "$env" == "dev" && "$RUN_DEV" != "true" ]] && continue
        [[ "$env" == "staging" && "$RUN_STAGING" != "true" ]] && continue

        local env_label="Development"
        [[ "$env" == "staging" ]] && env_label="Staging"

        local inference_file="$verbose_dir/${env}_inference.json"
        if [[ -f "$inference_file" ]]; then
            local first=true
            # Read each inference result from the array
            local count
            count=$(cat "$inference_file" | jq 'length' 2>/dev/null)
            for ((i=0; i<count; i++)); do
                local model status prompt_tokens completion_tokens duration_ms
                model=$(cat "$inference_file" | jq -r ".[$i].model // \"unknown\"" 2>/dev/null)
                status=$(cat "$inference_file" | jq -r ".[$i].status // \"UNKNOWN\"" 2>/dev/null)
                prompt_tokens=$(cat "$inference_file" | jq -r ".[$i].prompt_tokens // 0" 2>/dev/null)
                completion_tokens=$(cat "$inference_file" | jq -r ".[$i].completion_tokens // 0" 2>/dev/null)
                duration_ms=$(cat "$inference_file" | jq -r ".[$i].duration_ms // 0" 2>/dev/null)

                local display_env=""
                if [[ "$first" == "true" ]]; then
                    display_env="$env_label"
                    first=false
                fi

                # Truncate model name and add status indicator
                local truncated_model="${model:0:36}"
                local status_icon="✅"
                [[ "$status" == "FAIL" ]] && status_icon="❌"
                printf "║ %-11s ║ %s %-36s ║ %6s ║ %6s ║ %10s ║\n" \
                    "$display_env" "$status_icon" "$truncated_model" "$prompt_tokens" "$completion_tokens" "$duration_ms"
            done
        else
            printf "║ %-11s ║   %-36s ║ %6s ║ %6s ║ %10s ║\n" \
                "$env_label" "(no inference test)" "-" "-" "-"
        fi
    done

    echo "╚═════════════╩════════════════════════════════════════╩════════╩════════╩════════════╝"
}

# Print verbose details (models table and Q&A for each inference)
print_verbose_details() {
    local verbose_dir="/tmp/smoke-test-verbose-${SMOKE_TEST_ID:-$$}"

    # Enhanced ALL MODELS table with runtime, status, duration
    echo ""
    echo "╔══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╗"
    echo "║                                                           ALL MODELS                                                                         ║"
    echo "╠═════════════╦══════════════════════════╦════════════════════════════════════╦══════════════╦══════════════════════════╦══════════════════════╣"
    echo "║ Environment ║ Published Name           ║ Internal Name                      ║ Engine       ║ Status                   ║ HuggingFace Model ID ║"
    echo "╠═════════════╬══════════════════════════╬════════════════════════════════════╬══════════════╬══════════════════════════╬══════════════════════╣"

    for env in dev staging; do
        [[ "$env" == "dev" && "$RUN_DEV" != "true" ]] && continue
        [[ "$env" == "staging" && "$RUN_STAGING" != "true" ]] && continue

        local env_label="Development"
        [[ "$env" == "staging" ]] && env_label="Staging"

        local aimodels_file="$verbose_dir/${env}_aimodels.json"
        if [[ -f "$aimodels_file" ]]; then
            local count
            count=$(cat "$aimodels_file" | jq 'length' 2>/dev/null || echo "0")

            if [[ "$count" -gt 0 ]]; then
                local first=true
                for ((i=0; i<count; i++)); do
                    local external_name internal_name model_id runtime phase status_changed_at
                    external_name=$(cat "$aimodels_file" | jq -r ".[$i].externalName // \"\"" 2>/dev/null)
                    internal_name=$(cat "$aimodels_file" | jq -r ".[$i].internalName // \"\"" 2>/dev/null)
                    model_id=$(cat "$aimodels_file" | jq -r ".[$i].modelID // \"\"" 2>/dev/null)
                    runtime=$(cat "$aimodels_file" | jq -r ".[$i].runtime // \"unknown\"" 2>/dev/null)
                    phase=$(cat "$aimodels_file" | jq -r ".[$i].phase // \"Unknown\"" 2>/dev/null)
                    status_changed_at=$(cat "$aimodels_file" | jq -r ".[$i].statusChangedAt // \"\"" 2>/dev/null)

                    # Calculate duration in current status
                    local duration
                    duration=$(format_duration "$status_changed_at")

                    # Format status with duration
                    local status_with_duration
                    local status_icon="⏳"
                    case "$phase" in
                        Ready) status_icon="✅" ;;
                        Disabled) status_icon="⏸️" ;;
                        Failed|Error) status_icon="❌" ;;
                        Deploying|Downloading) status_icon="⏳" ;;
                    esac
                    status_with_duration="$status_icon $phase ($duration)"

                    local display_env=""
                    if [[ "$first" == "true" ]]; then
                        display_env="$env_label"
                        first=false
                    fi

                    # Truncate fields to fit columns
                    printf "║ %-11s ║ %-24s ║ %-34s ║ %-12s ║ %-24s ║ %-20s ║\n" \
                        "$display_env" \
                        "${external_name:0:24}" \
                        "${internal_name:0:34}" \
                        "${runtime:0:12}" \
                        "${status_with_duration:0:24}" \
                        "${model_id:0:20}"
                done
            else
                printf "║ %-11s ║ %-24s ║ %-34s ║ %-12s ║ %-24s ║ %-20s ║\n" \
                    "$env_label" "(no models found)" "-" "-" "-" "-"
            fi
        else
            # Fall back to published models from API if no kubectl access
            local models_file="$verbose_dir/${env}_models.json"
            if [[ -f "$models_file" ]]; then
                local models
                models=$(cat "$models_file" | jq -r '.data[] | "\(.id)|\(.owned_by // "unknown")"' 2>/dev/null)
                local first=true
                while IFS='|' read -r pub_model_id owner; do
                    if [[ -n "$pub_model_id" ]]; then
                        local display_env=""
                        if [[ "$first" == "true" ]]; then
                            display_env="$env_label"
                            first=false
                        fi
                        printf "║ %-11s ║ %-24s ║ %-34s ║ %-12s ║ %-24s ║ %-20s ║\n" \
                            "$display_env" "${pub_model_id:0:24}" "(via API)" "-" "✅ Published" "-"
                    fi
                done <<< "$models"
            fi
        fi
    done

    echo "╚═════════════╩══════════════════════════╩════════════════════════════════════╩══════════════╩══════════════════════════╩══════════════════════╝"

    # Q&A details for each inference
    echo ""
    echo "╔══════════════════════════════════════════════════════════════════════════════════════╗"
    echo "║                            INFERENCE Q&A DETAILS                                     ║"
    echo "╚══════════════════════════════════════════════════════════════════════════════════════╝"

    for env in dev staging; do
        [[ "$env" == "dev" && "$RUN_DEV" != "true" ]] && continue
        [[ "$env" == "staging" && "$RUN_STAGING" != "true" ]] && continue

        local env_label="Development"
        [[ "$env" == "staging" ]] && env_label="Staging"

        local inference_file="$verbose_dir/${env}_inference.json"
        if [[ -f "$inference_file" ]]; then
            echo ""
            echo "┌─ $env_label ─────────────────────────────────────────────────────────────────────────┐"

            local count
            count=$(cat "$inference_file" | jq 'length' 2>/dev/null)
            for ((i=0; i<count; i++)); do
                local model status prompt response error
                model=$(cat "$inference_file" | jq -r ".[$i].model // \"unknown\"" 2>/dev/null)
                status=$(cat "$inference_file" | jq -r ".[$i].status // \"UNKNOWN\"" 2>/dev/null)
                prompt=$(cat "$inference_file" | jq -r ".[$i].prompt // \"\"" 2>/dev/null)
                response=$(cat "$inference_file" | jq -r ".[$i].response // \"\"" 2>/dev/null)
                error=$(cat "$inference_file" | jq -r ".[$i].error // \"\"" 2>/dev/null)

                local status_icon="✅"
                [[ "$status" == "FAIL" ]] && status_icon="❌"

                echo "│"
                echo "│  $status_icon Model: $model"
                echo "│  Q: $prompt"
                if [[ "$status" == "PASS" ]]; then
                    echo "│  A: ${response:0:75}"
                else
                    echo "│  Error: ${error:0:70}"
                fi
            done

            echo "└────────────────────────────────────────────────────────────────────────────────────┘"
        fi
    done
}

# Cleanup temp files
cleanup_temp_files() {
    local verbose_dir="/tmp/smoke-test-verbose-${SMOKE_TEST_ID:-$$}"
    rm -rf "$verbose_dir"
}

# Main execution
main() {
    local dev_results_str=""
    local staging_results_str=""

    # Build CLI (quiet in JSON mode)
    build_cli "$JSON_OUTPUT"

    # Set a unique ID for this test run (used for temp file paths across subshells)
    export SMOKE_TEST_ID="$$-$(date +%s)"

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
        (run_environment_tests "dev" "$DEV_USER_ORG" "$DEV_API_ROUTER" "$DEV_ADMIN_API" "$MASTER_ADMIN_API_KEY" "$DEV_KUBECONFIG" "$DEV_NAMESPACE" > "$dev_tmp") &
        local dev_pid=$!

        (run_environment_tests "staging" "$STAGING_USER_ORG" "$STAGING_API_ROUTER" "$STAGING_ADMIN_API" "$STAGING_API_KEY" "$STAGING_KUBECONFIG" "$STAGING_NAMESPACE" > "$staging_tmp") &
        local staging_pid=$!

        # Wait for both
        wait $dev_pid
        wait $staging_pid

        dev_results_str=$(cat "$dev_tmp")
        staging_results_str=$(cat "$staging_tmp")

        rm -f "$dev_tmp" "$staging_tmp"

    elif [[ "$RUN_DEV" == "true" ]]; then
        dev_results_str=$(run_environment_tests "dev" "$DEV_USER_ORG" "$DEV_API_ROUTER" "$DEV_ADMIN_API" "$MASTER_ADMIN_API_KEY" "$DEV_KUBECONFIG" "$DEV_NAMESPACE")
    elif [[ "$RUN_STAGING" == "true" ]]; then
        staging_results_str=$(run_environment_tests "staging" "$STAGING_USER_ORG" "$STAGING_API_ROUTER" "$STAGING_ADMIN_API" "$STAGING_API_KEY" "$STAGING_KUBECONFIG" "$STAGING_NAMESPACE")
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

        # Always print inference details for all models
        print_inference_details

        # Print additional verbose details if requested (models table + Q&A)
        if [[ "$VERBOSE" == "true" ]]; then
            print_verbose_details
        fi

        # Cleanup temp files
        cleanup_temp_files

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
