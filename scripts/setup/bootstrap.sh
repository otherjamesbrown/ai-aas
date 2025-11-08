#!/usr/bin/env bash
set -euo pipefail

CHECK_ONLY=false
VERBOSE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check-only)
      CHECK_ONLY=true
      shift
      ;;
    --verbose)
      VERBOSE=true
      shift
      ;;
    -h|--help)
      cat <<'EOF'
AI-AAS bootstrap script

Usage: ./scripts/setup/bootstrap.sh [--check-only] [--verbose]

  --check-only   Validate prerequisites without making changes.
  --verbose      Print additional diagnostics to stdout.
EOF
      exit 0
      ;;
    *)
      echo "Unknown flag: $1" >&2
      exit 1
      ;;
  esac
done

LOG_DIR="${HOME}/.ai-aas"
LOG_FILE="${LOG_DIR}/bootstrap.log"
mkdir -p "${LOG_DIR}"
touch "${LOG_FILE}"

exec 3>>"${LOG_FILE}"

timestamp() {
  date +"%Y-%m-%dT%H:%M:%S%z"
}

log() {
  local level="$1"
  shift
  local message="$*"
  printf "%s [%s] %s\n" "$(timestamp)" "${level}" "${message}" >&3
  if [[ "${VERBOSE}" == "true" || "${level}" != "DEBUG" ]]; then
    printf "%s %s\n" "${level}" "${message}"
  fi
}

STEP_INDEX=0
TOTAL_STEPS=5
step() {
  STEP_INDEX=$((STEP_INDEX + 1))
  local label="$1"
  printf "[%d/%d] %s\n" "${STEP_INDEX}" "${TOTAL_STEPS}" "${label}"
  log "STEP" "${label}"
}

detect_os() {
  local uname_out
  uname_out="$(uname -s)"
  case "${uname_out}" in
    Darwin) echo "macOS" ;;
    Linux)
      if grep -qi microsoft /proc/version 2>/dev/null; then
        echo "WSL"
      else
        echo "Linux"
      fi
      ;;
    *)
      echo "Unknown"
      ;;
  esac
}

OS_NAME="$(detect_os)"

print_install_hint() {
  local cmd="$1"
  case "${OS_NAME}" in
    macOS)
      case "${cmd}" in
        go) echo "Install Go via Homebrew: brew install go" ;;
        git) echo "Install Git via Xcode Command Line Tools: xcode-select --install" ;;
        docker) echo "Install Docker Desktop from https://www.docker.com/products/docker-desktop" ;;
        gh) echo "Install GitHub CLI via Homebrew: brew install gh" ;;
        act) echo "Install act via Homebrew: brew install act" ;;
        make) echo "Install GNU Make via Homebrew: brew install make" ;;
        aws) echo "Install AWS CLI via Homebrew: brew install awscli" ;;
        mc) echo "Install MinIO client via Homebrew: brew install minio/stable/mc" ;;
        *) echo "Install ${cmd} using Homebrew or download from vendor site." ;;
      esac
      ;;
    Linux|WSL)
      case "${cmd}" in
        go) echo "Install Go from https://go.dev/dl/ or use your package manager." ;;
        git) echo "Install Git via package manager, e.g. sudo apt install git." ;;
        docker) echo "Install Docker Engine: https://docs.docker.com/engine/install/." ;;
        gh) echo "Install GitHub CLI: https://cli.github.com/manual/install_linux." ;;
        act) echo "Install act from https://github.com/nektos/act/releases." ;;
        make) echo "Install GNU Make via package manager, e.g. sudo apt install build-essential." ;;
        aws) echo "Install AWS CLI v2: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html." ;;
        mc) echo "Install MinIO client: https://min.io/docs/minio/linux/reference/minio-mc.html." ;;
        *) echo "Install ${cmd} via your distro package manager." ;;
      esac
      ;;
    *)
      echo "Refer to vendor documentation for installing ${cmd}."
      ;;
  esac
}

require_command() {
  local cmd="$1"
  local label="$2"
  if command -v "${cmd}" >/dev/null 2>&1; then
    log "INFO" "${label}: found ($(command -v "${cmd}"))"
    return 0
  fi
  log "ERROR" "${label}: not found"
  print_install_hint "${cmd}"
  return 1
}

validate_prereqs() {
  local missing=0
  step "Detect host operating system"
  log "INFO" "Detected OS: ${OS_NAME}"

  step "Validate required tooling"
  for entry in "go:Go compiler" "git:Git" "make:GNU Make" "docker:Docker CLI" "gh:GitHub CLI"; do
    IFS=":" read -r cmd label <<<"${entry}"
    if ! require_command "${cmd}" "${label}"; then
      missing=1
    fi
  done

  for optional in "act:act (local GitHub Actions)" "aws:AWS CLI" "mc:MinIO Client"; do
    IFS=":" read -r cmd label <<<"${optional}"
    if ! command -v "${cmd}" >/dev/null 2>&1; then
      log "WARN" "${label} not detected; optional but recommended"
    else
      log "INFO" "${label} available"
    fi
  done

  if [[ "${missing}" -ne 0 ]]; then
    log "ERROR" "Required tooling missing. Resolve issues and rerun."
    return 1
  fi
  return 0
}

validate_gh_auth() {
  step "Verify GitHub CLI authentication"
  if command -v gh >/dev/null 2>&1; then
    if gh auth status >/dev/null 2>&1; then
      log "INFO" "GitHub CLI authenticated"
    else
      log "WARN" "GitHub CLI not authenticated. Run 'gh auth login'."
    fi
  else
    log "WARN" "GitHub CLI not available, skipping authentication check."
  fi
}

ensure_workspace_sync() {
  step "Sync Go workspace modules"
  if command -v go >/dev/null 2>&1; then
    if [[ "${CHECK_ONLY}" == "true" ]]; then
      log "INFO" "Skipping go work sync (check-only mode)"
    else
      log "INFO" "Running go work sync"
      if go work sync >/dev/null 2>&1; then
        log "INFO" "go.work synchronized"
      else
        log "WARN" "go work sync reported issues; verify module paths"
      fi
    fi
  else
    log "WARN" "Go compiler not available; cannot sync go.work"
  fi
}

post_checks() {
  step "Summarize bootstrap results"
  log "INFO" "Bootstrap completed in $(timestamp)"
  if [[ -z "${LINODE_TOKEN:-}" ]]; then
    log "WARN" "LINODE_TOKEN not set. Set export LINODE_TOKEN=<token> before remote operations."
  else
    log "INFO" "LINODE_TOKEN detected."
  fi
  log "INFO" "Review quickstart at specs/000-project-setup/quickstart.md for next steps."
}

main() {
  validate_prereqs
  validate_gh_auth
  ensure_workspace_sync
  post_checks
}

if ! main; then
  log "ERROR" "Bootstrap failed. See ${LOG_FILE} for details."
  exit 1
fi

log "INFO" "Bootstrap succeeded."

