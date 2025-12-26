#!/usr/bin/env bash
#
# Upload E2E test results to S3
#
# Usage:
#   ./scripts/upload-results.sh [--dry-run] [--env development|staging]
#
# Environment variables:
#   AWS_ACCESS_KEY_ID     - AWS credentials
#   AWS_SECRET_ACCESS_KEY - AWS credentials
#   AWS_REGION            - AWS region (default: us-east-1)
#   RESULTS_BUCKET        - S3 bucket name (default: ai-aas-test-results)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(dirname "$SCRIPT_DIR")"

# Defaults - uses existing Linode Object Storage
DRY_RUN=false
ENVIRONMENT="development"
BUCKET="${RESULTS_BUCKET:-ai-aas-models}"
PREFIX="${RESULTS_PREFIX:-e2e-results}"
S3_ENDPOINT="${S3_ENDPOINT:-https://us-east-1.linodeobjects.com}"
DATE=$(date -u +%Y-%m-%d)
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

usage() {
    cat <<EOF
Upload E2E test results to Linode Object Storage

Usage: $(basename "$0") [OPTIONS]

Options:
  --dry-run         Print what would be uploaded without uploading
  --env ENV         Environment (development|staging) [default: development]
  --date DATE       Date for results folder [default: today]
  --bucket BUCKET   S3 bucket name [default: ai-aas-models]
  -h, --help        Show this help message

Environment Variables:
  AWS_ACCESS_KEY_ID      Linode Object Storage access key (or LINODE_OBJECT_STORAGE_ACCESS_KEY)
  AWS_SECRET_ACCESS_KEY  Linode Object Storage secret key (or LINODE_OBJECT_STORAGE_SECRET_KEY)
  RESULTS_BUCKET         S3 bucket [default: ai-aas-models]
  RESULTS_PREFIX         S3 prefix [default: e2e-results]
  S3_ENDPOINT            S3 endpoint [default: https://us-east-1.linodeobjects.com]

Examples:
  # Upload today's results
  ./scripts/upload-results.sh

  # Dry run
  ./scripts/upload-results.sh --dry-run

  # Upload to staging folder
  ./scripts/upload-results.sh --env staging
EOF
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --env)
            ENVIRONMENT="$2"
            shift 2
            ;;
        --date)
            DATE="$2"
            shift 2
            ;;
        --bucket)
            BUCKET="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Validate environment
if [[ "$ENVIRONMENT" != "development" && "$ENVIRONMENT" != "staging" ]]; then
    echo "Error: Environment must be 'development' or 'staging'"
    exit 1
fi

# Check for required files
RESULTS_JSON="$E2E_DIR/results.json"
RESULTS_XML="$E2E_DIR/results.xml"
METRICS_JSON="$E2E_DIR/metrics.json"

check_file() {
    if [[ ! -f "$1" ]]; then
        echo "Warning: $1 not found"
        return 1
    fi
    return 0
}

# Generate metrics.json if it doesn't exist
generate_metrics() {
    if [[ -f "$RESULTS_JSON" ]]; then
        PASSED=$(jq -c 'select(.Action == "pass")' "$RESULTS_JSON" 2>/dev/null | wc -l || echo 0)
        FAILED=$(jq -c 'select(.Action == "fail")' "$RESULTS_JSON" 2>/dev/null | wc -l || echo 0)
    else
        PASSED=0
        FAILED=0
    fi

    cat > "$METRICS_JSON" <<EOF
{
  "timestamp": "$TIMESTAMP",
  "date": "$DATE",
  "environment": "$ENVIRONMENT",
  "tier": "manual",
  "duration_seconds": 0,
  "tests_passed": $PASSED,
  "tests_failed": $FAILED,
  "exit_code": 0,
  "run_id": "local-$(date +%s)",
  "run_url": "",
  "commit_sha": "$(git rev-parse HEAD 2>/dev/null || echo 'unknown')",
  "branch": "$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo 'unknown')"
}
EOF
    echo "Generated $METRICS_JSON"
}

upload_file() {
    local src="$1"
    local dest="$2"

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "[DRY-RUN] Would upload: $src -> s3://$BUCKET/$dest"
    else
        echo "Uploading: $src -> s3://$BUCKET/$dest"
        aws s3 cp "$src" "s3://$BUCKET/$dest" --endpoint-url "$S3_ENDPOINT"
    fi
}

main() {
    echo "=== E2E Results Upload ==="
    echo "Environment: $ENVIRONMENT"
    echo "Date: $DATE"
    echo "Bucket: $BUCKET"
    echo "Prefix: $PREFIX"
    echo "Endpoint: $S3_ENDPOINT"
    echo "Dry run: $DRY_RUN"
    echo ""

    # Check credentials (support both AWS and Linode naming)
    if [[ -z "${AWS_ACCESS_KEY_ID:-}" ]]; then
        export AWS_ACCESS_KEY_ID="${LINODE_OBJECT_STORAGE_ACCESS_KEY:-}"
    fi
    if [[ -z "${AWS_SECRET_ACCESS_KEY:-}" ]]; then
        export AWS_SECRET_ACCESS_KEY="${LINODE_OBJECT_STORAGE_SECRET_KEY:-}"
    fi

    if [[ -z "${AWS_ACCESS_KEY_ID:-}" || -z "${AWS_SECRET_ACCESS_KEY:-}" ]]; then
        echo "Error: Object storage credentials not set"
        echo "Set AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY or LINODE_OBJECT_STORAGE_ACCESS_KEY/LINODE_OBJECT_STORAGE_SECRET_KEY"
        exit 1
    fi

    # Check/generate files
    has_results=false

    if check_file "$RESULTS_JSON"; then
        has_results=true
    fi

    if ! check_file "$METRICS_JSON"; then
        if [[ "$has_results" == "true" ]]; then
            generate_metrics
        else
            echo "Error: No results.json or metrics.json found"
            echo "Run E2E tests first: go test -v ./suites/... -json > results.json"
            exit 1
        fi
    fi

    # Upload files
    S3_PATH="$PREFIX/nightly/$ENVIRONMENT/$DATE"

    if [[ -f "$RESULTS_JSON" ]]; then
        upload_file "$RESULTS_JSON" "$S3_PATH/results.json"
    fi

    if [[ -f "$RESULTS_XML" ]]; then
        upload_file "$RESULTS_XML" "$S3_PATH/results.xml"
    fi

    if [[ -f "$METRICS_JSON" ]]; then
        upload_file "$METRICS_JSON" "$S3_PATH/metrics.json"
        # Also update latest
        upload_file "$METRICS_JSON" "$PREFIX/nightly/$ENVIRONMENT/latest/metrics.json"
    fi

    echo ""
    echo "Upload complete!"

    if [[ "$DRY_RUN" == "false" ]]; then
        echo ""
        echo "View results:"
        echo "  aws s3 ls s3://$BUCKET/$S3_PREFIX/"
    fi
}

main
