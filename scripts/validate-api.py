#!/usr/bin/env python3
"""
Validate API Router Service OpenAI-compatible endpoints using official OpenAI SDK.

This validates that the API is truly OpenAI-compatible by using the official
OpenAI Python client library that real applications would use.

Usage:
    python3 scripts/validate-api.py --base-url https://api.172.232.58.222.nip.io/v1 --api-key YOUR_KEY
    python3 scripts/validate-api.py --base-url http://localhost:8080/v1 --api-key YOUR_KEY
"""

import argparse
import sys
import urllib3

from openai import OpenAI

# Disable SSL warnings for self-signed certificates
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)


class Colors:
    """ANSI color codes for terminal output."""
    GREEN = '\033[92m'
    RED = '\033[91m'
    YELLOW = '\033[93m'
    BLUE = '\033[94m'
    RESET = '\033[0m'
    BOLD = '\033[1m'


def print_success(message: str):
    print(f"{Colors.GREEN}✓{Colors.RESET} {message}")


def print_error(message: str):
    print(f"{Colors.RED}✗{Colors.RESET} {message}")


def print_info(message: str):
    print(f"{Colors.BLUE}ℹ{Colors.RESET} {message}")


def print_warning(message: str):
    print(f"{Colors.YELLOW}⚠{Colors.RESET} {message}")


def print_section(title: str):
    print(f"\n{Colors.BOLD}{title}{Colors.RESET}")
    print("=" * len(title))


def validate_models_endpoint(client: OpenAI) -> tuple[bool, list]:
    """Validate the /v1/models endpoint using OpenAI SDK."""
    print_section("Testing /v1/models endpoint")

    try:
        print_info("Calling client.models.list()")
        models = client.models.list()

        print_success("Successfully retrieved model list")

        # Access models
        model_list = list(models.data)

        if not model_list:
            print_error("No models returned")
            return False, []

        print_success(f"Found {len(model_list)} model(s)")

        # Print model details
        for model in model_list:
            print_info(f"  Model: {model.id} (type={model.object}, owner={model.owned_by})")

        return True, [m.id for m in model_list]

    except Exception as e:
        print_error(f"Request failed: {e}")
        return False, []


def validate_chat_completions_endpoint(client: OpenAI, model: str) -> bool:
    """Validate the /v1/chat/completions endpoint using OpenAI SDK."""
    print_section("Testing /v1/chat/completions endpoint")

    try:
        print_info(f"Calling client.chat.completions.create()")
        print_info(f"Model: {model}")
        print_info(f"Message: Say 'Hello, World!' and nothing else.")

        response = client.chat.completions.create(
            model=model,
            messages=[
                {"role": "user", "content": "Say 'Hello, World!' and nothing else."}
            ],
            max_tokens=20,
            temperature=0.7
        )

        print_success("Successfully created chat completion")

        # Validate response structure (OpenAI SDK already validates this)
        if not response.choices:
            print_error("Response has no choices")
            return False

        print_success(f"Found {len(response.choices)} choice(s)")

        # Print response content
        for i, choice in enumerate(response.choices):
            content = choice.message.content
            print_info(f"  Choice {i}: {content}")

        # Validate metadata
        print_info(f"  Model: {response.model}")
        print_info(f"  Usage: {response.usage.total_tokens} tokens " +
                   f"({response.usage.prompt_tokens} prompt + {response.usage.completion_tokens} completion)")

        return True

    except Exception as e:
        print_error(f"Request failed: {e}")
        return False


def main():
    parser = argparse.ArgumentParser(
        description="Validate API Router Service OpenAI-compatible endpoints using official OpenAI SDK"
    )
    parser.add_argument(
        "--base-url",
        required=True,
        help="Base URL for the API (e.g., https://api.172.232.58.222.nip.io/v1)"
    )
    parser.add_argument(
        "--api-key",
        required=True,
        help="API key for authentication"
    )
    parser.add_argument(
        "--skip-inference",
        action="store_true",
        help="Skip inference test (only test /v1/models)"
    )

    args = parser.parse_args()

    # Note: OpenAI SDK doesn't support disabling SSL verification directly
    # For self-signed certs, you'd need to set CURL_CA_BUNDLE or use httpx with custom settings
    verify_ssl = not args.base_url.startswith("http://")

    if verify_ssl and "nip.io" in args.base_url:
        print_warning("Using HTTPS with self-signed cert - may require CURL_CA_BUNDLE environment variable")

    print(f"\n{Colors.BOLD}API Router Service Validator (OpenAI SDK){Colors.RESET}")
    print(f"Base URL: {args.base_url}")
    print(f"Using: Official OpenAI Python SDK")

    # Create OpenAI client pointing to our custom endpoint
    try:
        client = OpenAI(
            base_url=args.base_url,
            api_key=args.api_key,
            # For HTTP or self-signed HTTPS, we need to configure httpx
            http_client=None if verify_ssl else __create_insecure_http_client()
        )
    except Exception as e:
        print_error(f"Failed to create OpenAI client: {e}")
        sys.exit(1)

    # Test /v1/models endpoint
    models_ok, available_models = validate_models_endpoint(client)

    if not models_ok:
        print_error("\n❌ Models endpoint validation failed")
        sys.exit(1)

    # Skip inference test if requested
    if args.skip_inference:
        print_warning("\nSkipping inference test (--skip-inference flag set)")
        print_success("\n✅ All tests passed")
        sys.exit(0)

    # Get first model for inference test
    if not available_models:
        print_error("No models available for inference test")
        sys.exit(1)

    model_id = available_models[0]

    # Test /v1/chat/completions endpoint
    inference_ok = validate_chat_completions_endpoint(client, model_id)

    if not inference_ok:
        print_error("\n❌ Inference endpoint validation failed")
        sys.exit(1)

    print_success("\n✅ All tests passed")
    sys.exit(0)


def __create_insecure_http_client():
    """Create an HTTP client that accepts self-signed certificates."""
    import httpx
    return httpx.Client(verify=False)


if __name__ == "__main__":
    main()
