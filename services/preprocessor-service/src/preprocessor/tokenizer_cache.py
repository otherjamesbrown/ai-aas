"""Tokenizer cache management with LRU eviction."""

import threading
from collections import OrderedDict
from typing import Any

import structlog
from transformers import AutoTokenizer, PreTrainedTokenizerBase

from .config import settings

logger = structlog.get_logger(__name__)


class TokenizerCache:
    """Thread-safe LRU cache for HuggingFace tokenizers."""

    def __init__(self, max_size: int = 50):
        self._cache: OrderedDict[str, PreTrainedTokenizerBase] = OrderedDict()
        self._lock = threading.RLock()
        self._max_size = max_size
        self._load_errors: dict[str, str] = {}

    def get(self, model_id: str) -> PreTrainedTokenizerBase:
        """Get tokenizer from cache, loading if necessary.

        Args:
            model_id: HuggingFace model identifier (e.g., "meta-llama/Llama-3.1-8B-Instruct")

        Returns:
            The tokenizer for the specified model

        Raises:
            ValueError: If the tokenizer cannot be loaded
        """
        with self._lock:
            # Check if already cached
            if model_id in self._cache:
                # Move to end (most recently used)
                self._cache.move_to_end(model_id)
                logger.debug("tokenizer_cache_hit", model_id=model_id)
                return self._cache[model_id]

            # Check for previous load error
            if model_id in self._load_errors:
                raise ValueError(f"Tokenizer load failed previously: {self._load_errors[model_id]}")

        # Load outside the lock to avoid blocking other requests
        logger.info("loading_tokenizer", model_id=model_id)
        try:
            tokenizer = AutoTokenizer.from_pretrained(
                model_id,
                trust_remote_code=True,
                token=settings.hf_token,
            )
        except Exception as e:
            error_msg = f"Failed to load tokenizer: {e}"
            logger.error("tokenizer_load_failed", model_id=model_id, error=str(e))
            with self._lock:
                self._load_errors[model_id] = error_msg
            raise ValueError(error_msg) from e

        with self._lock:
            # Evict oldest if at capacity
            while len(self._cache) >= self._max_size:
                evicted_id, _ = self._cache.popitem(last=False)
                logger.info("tokenizer_evicted", model_id=evicted_id)

            self._cache[model_id] = tokenizer
            logger.info("tokenizer_loaded", model_id=model_id, cache_size=len(self._cache))

        return tokenizer

    def preload(self, model_ids: list[str]) -> dict[str, bool]:
        """Preload multiple tokenizers at startup.

        Args:
            model_ids: List of model IDs to preload

        Returns:
            Dict mapping model_id to success status
        """
        results = {}
        for model_id in model_ids:
            try:
                self.get(model_id)
                results[model_id] = True
            except Exception as e:
                logger.error("preload_failed", model_id=model_id, error=str(e))
                results[model_id] = False
        return results

    def list_loaded(self) -> dict[str, bool]:
        """List all loaded tokenizers.

        Returns:
            Dict mapping model_id to True for all cached tokenizers
        """
        with self._lock:
            return {model_id: True for model_id in self._cache.keys()}

    def clear(self) -> None:
        """Clear all cached tokenizers."""
        with self._lock:
            self._cache.clear()
            self._load_errors.clear()
            logger.info("tokenizer_cache_cleared")


# Global tokenizer cache instance
tokenizer_cache = TokenizerCache(max_size=settings.max_cached_tokenizers)
