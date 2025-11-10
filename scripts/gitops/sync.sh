#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <src_dir> <dest_dir>" >&2
  echo "Example: $0 infra/generated/development gitops/clusters/development/apps" >&2
  exit 1
fi

SRC="$1"
DEST="$2"

if [[ ! -d "$SRC" ]]; then
  echo "Source directory not found: $SRC" >&2
  exit 1
fi

mkdir -p "$DEST"
rsync -av --delete "$SRC"/ "$DEST"/
