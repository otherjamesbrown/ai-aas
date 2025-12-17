#!/bin/bash
# cleanup-stale-revisions.sh
#
# Cleanup stale Knative revisions that are holding GPU resources.
# Part of the GPU Deployment Mode Migration (Spec 030).
#
# Usage:
#   ./cleanup-stale-revisions.sh              # Dry-run mode (default)
#   DRY_RUN=false ./cleanup-stale-revisions.sh  # Actually delete revisions
#
# Environment variables:
#   KUBECONFIG - Path to kubeconfig (default: uses current context)
#   DRY_RUN    - Set to "false" to actually delete (default: true)
#   NAMESPACE  - Specific namespace to clean (default: all namespaces)

set -euo pipefail

DRY_RUN="${DRY_RUN:-true}"
NAMESPACE="${NAMESPACE:-}"

echo "============================================"
echo "Knative Stale Revision Cleanup"
echo "============================================"
echo "Mode: $([ "$DRY_RUN" = "true" ] && echo "DRY-RUN (no deletions)" || echo "LIVE (will delete)")"
echo "Namespace: ${NAMESPACE:-all namespaces}"
echo "============================================"
echo ""

# Check for required tools
if ! command -v kubectl &> /dev/null; then
    echo "ERROR: kubectl is required but not found"
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo "ERROR: jq is required but not found"
    exit 1
fi

# Build namespace flag
NS_FLAG=""
if [ -n "$NAMESPACE" ]; then
    NS_FLAG="-n $NAMESPACE"
else
    NS_FLAG="-A"
fi

echo "Finding stale revisions (not receiving traffic)..."
echo ""

# Get all revisions that are not active (not receiving traffic)
STALE_REVISIONS=$(kubectl get revisions $NS_FLAG -o json 2>/dev/null | jq -r '
  .items[] |
  select(.status.conditions != null) |
  select(.status.conditions[] | select(.type=="Active" and .status=="False")) |
  "\(.metadata.namespace)/\(.metadata.name)"
' 2>/dev/null || echo "")

if [ -z "$STALE_REVISIONS" ]; then
    echo "No stale revisions found."
    echo ""
    echo "Note: Revisions are considered 'stale' when:"
    echo "  - Active condition is False (not receiving traffic)"
    echo ""
    exit 0
fi

# Count revisions
REVISION_COUNT=$(echo "$STALE_REVISIONS" | wc -l)
echo "Found $REVISION_COUNT stale revision(s):"
echo ""

# Process each revision
DELETED_COUNT=0
FAILED_COUNT=0

while IFS= read -r revision; do
    [ -z "$revision" ] && continue

    namespace=$(echo "$revision" | cut -d'/' -f1)
    name=$(echo "$revision" | cut -d'/' -f2)

    # Get additional info about the revision
    OWNER=$(kubectl get revision "$name" -n "$namespace" -o jsonpath='{.metadata.ownerReferences[0].name}' 2>/dev/null || echo "unknown")
    CREATED=$(kubectl get revision "$name" -n "$namespace" -o jsonpath='{.metadata.creationTimestamp}' 2>/dev/null || echo "unknown")

    echo "  Revision: $namespace/$name"
    echo "    Owner (KSVC): $OWNER"
    echo "    Created: $CREATED"

    if [ "$DRY_RUN" = "true" ]; then
        echo "    Action: Would delete (dry-run)"
    else
        echo -n "    Action: Deleting... "
        if kubectl delete revision "$name" -n "$namespace" --wait=false; then
            echo "OK"
            ((DELETED_COUNT++))
        else
            echo "FAILED"
            ((FAILED_COUNT++))
        fi
    fi
    echo ""
done <<< "$STALE_REVISIONS"

echo "============================================"
echo "Summary"
echo "============================================"
if [ "$DRY_RUN" = "true" ]; then
    echo "Dry-run complete. $REVISION_COUNT revision(s) would be deleted."
    echo ""
    echo "To actually delete, run:"
    echo "  DRY_RUN=false $0"
else
    echo "Deleted: $DELETED_COUNT"
    echo "Failed: $FAILED_COUNT"
    echo "Total processed: $REVISION_COUNT"
fi
echo ""
