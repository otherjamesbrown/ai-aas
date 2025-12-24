"""Tests for the template engine module."""

import pytest

# Note: These tests require the transformers library and internet access
# to download tokenizers. In CI, you may want to mock or skip these.


class TestTemplateEngine:
    """Tests for TemplateEngine class."""

    @pytest.mark.skip(reason="Requires HuggingFace transformers and network access")
    def test_llama_chat_template(self, sample_messages, llama_model_id):
        """Test Llama-3 chat template application."""
        from preprocessor.template_engine import template_engine

        result = template_engine.apply_chat_template(
            model_id=llama_model_id,
            messages=sample_messages,
            add_generation_prompt=True,
            tokenize=False,
        )

        # Llama-3 format should start with special tokens
        assert "<|begin_of_text|>" in result
        assert "Hello!" in result

    @pytest.mark.skip(reason="Requires HuggingFace transformers and network access")
    def test_count_tokens(self, llama_model_id):
        """Test token counting."""
        from preprocessor.template_engine import template_engine

        count = template_engine.count_tokens(
            model_id=llama_model_id,
            text="Hello, world!",
        )

        assert count > 0
        assert isinstance(count, int)

    @pytest.mark.skip(reason="Requires HuggingFace transformers and network access")
    def test_tokenize(self, llama_model_id):
        """Test tokenization to IDs."""
        from preprocessor.template_engine import template_engine

        tokens = template_engine.tokenize(
            model_id=llama_model_id,
            text="Hello, world!",
        )

        assert len(tokens) > 0
        assert all(isinstance(t, int) for t in tokens)


class TestTokenizerCache:
    """Tests for TokenizerCache class."""

    def test_cache_initialization(self):
        """Test cache initializes correctly."""
        from preprocessor.tokenizer_cache import TokenizerCache

        cache = TokenizerCache(max_size=10)
        assert cache._max_size == 10
        assert len(cache._cache) == 0

    def test_list_loaded_empty(self):
        """Test list_loaded returns empty dict initially."""
        from preprocessor.tokenizer_cache import TokenizerCache

        cache = TokenizerCache(max_size=10)
        loaded = cache.list_loaded()
        assert loaded == {}

    def test_clear_cache(self):
        """Test cache can be cleared."""
        from preprocessor.tokenizer_cache import TokenizerCache

        cache = TokenizerCache(max_size=10)
        cache.clear()
        assert len(cache._cache) == 0
        assert len(cache._load_errors) == 0
