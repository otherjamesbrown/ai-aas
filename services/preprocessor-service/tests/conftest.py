"""Pytest configuration and fixtures for preprocessor-service tests."""

import pytest


@pytest.fixture
def sample_messages():
    """Sample chat messages for testing."""
    return [
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Hello!"},
    ]


@pytest.fixture
def llama_model_id():
    """Llama-3 model ID for testing."""
    return "meta-llama/Llama-3.1-8B-Instruct"
