#!/bin/bash
#
# compliance-review.sh - Post-implementation compliance check hook
#
# This hook runs after Claude finishes responding (Stop event) and checks:
# 1. Whether implementation work was done (jb-3.6-implement or related)
# 2. Use case compliance (scope violations, AC coverage)
# 3. Architecture compliance (API-first, GitOps patterns)
#
# Outputs JSON to stdout for Claude to process
# Exit 0 = success, Exit 2 = blocking issue

set -euo pipefail

# Read input from stdin
input=$(cat)

# Extract useful fields
transcript_path=$(echo "$input" | jq -r '.transcript_path // ""')
cwd=$(echo "$input" | jq -r '.cwd // ""')
session_id=$(echo "$input" | jq -r '.session_id // ""')

# Change to project directory
cd "${cwd:-$(pwd)}"

# Check if this was an implementation session
is_implementation_session() {
    if [[ -z "$transcript_path" ]] || [[ ! -f "$transcript_path" ]]; then
        return 1
    fi

    # Check for implementation-related keywords in recent transcript
    if grep -qE 'jb-3\.6-implement|speckit\.implement|Implementation Context|implementing UC-' "$transcript_path" 2>/dev/null; then
        return 0
    fi

    return 1
}

# Check for use case scope violations
check_uc_compliance() {
    local issues=()

    # Check if usecases directory exists
    if [[ ! -d "usecases" ]]; then
        echo "skip"
        return
    fi

    # Run linter if available
    if [[ -x "scripts/lint-usecases.sh" ]]; then
        if ! ./scripts/lint-usecases.sh >/dev/null 2>&1; then
            issues+=("Use case YAML has validation errors")
        fi
    fi

    # Run coverage check if available
    if [[ -x "scripts/uc-coverage.sh" ]]; then
        local coverage_output
        coverage_output=$(./scripts/uc-coverage.sh 2>/dev/null || true)

        # Check for missing test coverage
        if echo "$coverage_output" | grep -q "NO TEST"; then
            issues+=("Some acceptance criteria are missing tests")
        fi
    fi

    if [[ ${#issues[@]} -eq 0 ]]; then
        echo "pass"
    else
        printf '%s\n' "${issues[@]}"
    fi
}

# Check for architecture violations in recent changes
check_arch_compliance() {
    local issues=()

    # Get recently modified Go files
    local recent_files
    recent_files=$(git diff --name-only HEAD~1 2>/dev/null | grep '\.go$' || true)

    if [[ -z "$recent_files" ]]; then
        echo "skip"
        return
    fi

    # Check for direct database access in CLI commands
    # NOTE: This pattern detects sql.Open and gorm.Open. If new database libraries
    # are introduced (e.g., pgx, ent, sqlx), add them to this pattern.
    # The goal is to ensure CLI code uses API clients, not direct DB access.
    if echo "$recent_files" | xargs grep -l 'sql.Open\|gorm.Open' 2>/dev/null | grep -q 'cmd/'; then
        issues+=("CLI code contains direct database access - use API client instead")
    fi

    # Check for kubectl apply in scripts (should use GitOps)
    if echo "$recent_files" | xargs grep -l 'kubectl apply' 2>/dev/null | grep -qv '_test.go'; then
        issues+=("Code contains kubectl apply - use GitOps workflow instead")
    fi

    if [[ ${#issues[@]} -eq 0 ]]; then
        echo "pass"
    else
        printf '%s\n' "${issues[@]}"
    fi
}

# Main logic
main() {
    # Only run detailed checks for implementation sessions
    if ! is_implementation_session; then
        # Not an implementation session, approve silently
        echo '{"decision": "approve", "reason": "Non-implementation session", "checks": {"implementation_detected": false}}'
        exit 0
    fi

    # Run compliance checks
    local uc_result
    local arch_result

    uc_result=$(check_uc_compliance)
    arch_result=$(check_arch_compliance)

    # Build response
    local decision="approve"
    local reason="Implementation session completed"
    local findings=()

    if [[ "$uc_result" != "pass" ]] && [[ "$uc_result" != "skip" ]]; then
        while IFS= read -r line; do
            findings+=("$line")
        done <<< "$uc_result"
    fi

    if [[ "$arch_result" != "pass" ]] && [[ "$arch_result" != "skip" ]]; then
        while IFS= read -r line; do
            findings+=("$line")
        done <<< "$arch_result"
    fi

    # If there are findings, set decision to block (warning mode) or just report
    if [[ ${#findings[@]} -gt 0 ]]; then
        reason="Compliance issues detected: ${findings[*]}"
        # Use "approve" with findings to warn but not block
        # Change to "block" to enforce compliance
        decision="approve"
    fi

    # Output JSON response
    cat << EOF
{
  "decision": "$decision",
  "reason": "$reason",
  "checks": {
    "implementation_detected": true,
    "uc_compliance": "$(echo "$uc_result" | head -1)",
    "arch_compliance": "$(echo "$arch_result" | head -1)"
  },
  "findings": $(printf '%s\n' "${findings[@]:-}" | jq -R . | jq -s .),
  "suggestion": "Run /review-compliance for detailed analysis"
}
EOF

    exit 0
}

main "$@"
