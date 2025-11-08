#!/usr/bin/env bash
set -euo pipefail

# Helper functions for interacting with Akamai Linode APIs.

require_linode_token() {
  if [[ -z "${LINODE_TOKEN:-}" ]]; then
    echo "LINODE_TOKEN is not set. See docs/platform/linode-access.md." >&2
    exit 1
  fi
}

linode_api() {
  require_linode_token
  local method=$1
  local path=$2
  shift 2
  curl -sS \
    -H "Authorization: Bearer ${LINODE_TOKEN}" \
    -H "Content-Type: application/json" \
    -X "${method}" \
    "$@" \
    "https://api.linode.com/v4${path}"
}

