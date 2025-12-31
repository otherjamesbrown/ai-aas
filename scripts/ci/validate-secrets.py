#!/usr/bin/env python3
"""
validate-secrets.py

Validates which GitHub repository secrets are configured.
Note: This script can only check if secrets exist, not their values.

Usage:
    ./scripts/ci/validate-secrets.py

Requirements:
    - gh CLI installed and authenticated
"""

import subprocess
import sys

REPO = "otherjamesbrown/ai-aas"

# ANSI color codes
RED = '\033[0;31m'
GREEN = '\033[0;32m'
YELLOW = '\033[1;33m'
NC = '\033[0m'  # No Color

CRITICAL_SECRETS = {
    "MASTER_ADMIN_API_KEY": "E2E test authentication (development)",
    "STAGING_MASTER_ADMIN_API_KEY": "E2E test authentication (staging)",
    "LINODE_OBJECT_STORAGE_ACCESS_KEY": "Upload E2E test results",
    "LINODE_OBJECT_STORAGE_SECRET_KEY": "Upload E2E test results",
    "DEV_KUBECONFIG_B64": "Development cluster access",
    "DEV_KUBE_CONTEXT": "Development cluster context",
    "PROD_KUBECONFIG_B64": "Production cluster access",
    "PROD_KUBE_CONTEXT": "Production cluster context",
    "GHCR_TOKEN": "Container registry authentication",
    "LINODE_TOKEN": "Infrastructure provisioning",
}

OPTIONAL_SECRETS = {
    "GRAFANA_API_KEY": "Grafana annotations (optional)",
    "GRAFANA_URL": "Grafana instance URL (optional)",
    "DEV_KUBECONFIG": "Non-base64 kubeconfig (failure mode tests)",
    "DEV_API_ENDPOINT": "Development API endpoint (failure mode tests)",
    "DEV_ADMIN_API_KEY": "Development admin key (failure mode tests)",
    "API_ROUTER_DEV_TEST_KEY": "API router validation (development)",
    "API_ROUTER_STAGING_TEST_KEY": "API router validation (staging)",
    "API_ROUTER_PROD_TEST_KEY": "API router validation (production)",
}


def fetch_existing_secrets():
    """Fetch list of existing GitHub secrets."""
    try:
        result = subprocess.run(
            ["gh", "secret", "list", "--repo", REPO],
            capture_output=True,
            text=True,
            check=True
        )
        # Parse secret names from output (first column)
        secrets = set()
        for line in result.stdout.strip().split('\n'):
            if line:
                secret_name = line.split()[0]
                secrets.add(secret_name)
        return secrets
    except subprocess.CalledProcessError as e:
        print(f"{RED}ERROR: Failed to fetch secrets. Are you authenticated with gh CLI?{NC}", file=sys.stderr)
        print("\nRun: gh auth login", file=sys.stderr)
        sys.exit(1)
    except FileNotFoundError:
        print(f"{RED}ERROR: gh CLI not found. Please install GitHub CLI.{NC}", file=sys.stderr)
        sys.exit(1)


def main():
    print("=" * 50)
    print("GitHub Secrets Validation")
    print(f"Repository: {REPO}")
    print("=" * 50)
    print()

    print("Fetching existing secrets...")
    existing_secrets = fetch_existing_secrets()
    print()

    # Check critical secrets
    print("=" * 50)
    print("CRITICAL SECRETS (block CI/CD)")
    print("=" * 50)
    print()

    critical_missing = 0
    for secret, description in CRITICAL_SECRETS.items():
        if secret in existing_secrets:
            print(f"{GREEN}✓{NC} {secret} - {description}")
        else:
            print(f"{RED}✗{NC} {secret} - {description}")
            critical_missing += 1

    print()

    # Check optional secrets
    print("=" * 50)
    print("OPTIONAL SECRETS (degrade functionality)")
    print("=" * 50)
    print()

    optional_missing = 0
    for secret, description in OPTIONAL_SECRETS.items():
        if secret in existing_secrets:
            print(f"{GREEN}✓{NC} {secret} - {description}")
        else:
            print(f"{YELLOW}⚠{NC} {secret} - {description}")
            optional_missing += 1

    print()

    # Summary
    print("=" * 50)
    print("SUMMARY")
    print("=" * 50)
    print()

    critical_ok = len(CRITICAL_SECRETS) - critical_missing
    optional_ok = len(OPTIONAL_SECRETS) - optional_missing

    print(f"Critical: {critical_ok}/{len(CRITICAL_SECRETS)} configured")
    print(f"Optional: {optional_ok}/{len(OPTIONAL_SECRETS)} configured")
    print()

    if critical_missing == 0:
        print(f"{GREEN}✓ All critical secrets are configured!{NC}")
        exit_code = 0
    else:
        print(f"{RED}✗ {critical_missing} critical secret(s) missing!{NC}")
        print()
        print("To fix:")
        print(f"1. Go to: https://github.com/{REPO}/settings/secrets/actions")
        print("2. Click 'New repository secret'")
        print("3. Add each missing secret (see docs/platform/github-secrets-setup.md)")
        exit_code = 1

    if optional_missing > 0:
        print(f"{YELLOW}⚠ {optional_missing} optional secret(s) missing{NC}")
        print("   Some workflows may have degraded functionality")

    print()

    # Show undocumented secrets
    print("=" * 50)
    print("UNDOCUMENTED SECRETS")
    print("=" * 50)
    print()

    documented_secrets = set(CRITICAL_SECRETS.keys()) | set(OPTIONAL_SECRETS.keys())
    undocumented = existing_secrets - documented_secrets

    if undocumented:
        for secret in sorted(undocumented):
            print(f"{YELLOW}?{NC} {secret} - Not documented in validation script")
    else:
        print("None - all secrets are documented")

    print()

    sys.exit(exit_code)


if __name__ == "__main__":
    main()
