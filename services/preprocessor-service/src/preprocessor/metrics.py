"""Prometheus metrics for the preprocessor service."""

import threading
from typing import Dict, Optional

from prometheus_client import Counter, Gauge, Histogram


# Request metrics
preprocess_requests_total = Counter(
    "preprocessor_requests_total",
    "Total number of preprocess requests",
    ["model_id", "engine_type", "status"],  # status: "success", "error"
)

preprocess_request_duration_seconds = Histogram(
    "preprocessor_request_duration_seconds",
    "Request latency for preprocessing in seconds",
    ["model_id", "engine_type"],
    buckets=[0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0],
)

preprocess_errors_total = Counter(
    "preprocessor_errors_total",
    "Total number of preprocessing errors",
    ["model_id", "error_type"],  # error_type: "tokenizer_load", "template_apply", "internal"
)

# Tokenizer cache metrics
tokenizer_cache_hits_total = Counter(
    "preprocessor_tokenizer_cache_hits_total",
    "Total number of tokenizer cache hits",
    ["model_id"],
)

tokenizer_cache_misses_total = Counter(
    "preprocessor_tokenizer_cache_misses_total",
    "Total number of tokenizer cache misses (loads)",
    ["model_id"],
)

tokenizer_load_duration_seconds = Histogram(
    "preprocessor_tokenizer_load_duration_seconds",
    "Tokenizer load latency in seconds",
    ["model_id"],
    buckets=[0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0],
)

tokenizer_cache_size = Gauge(
    "preprocessor_tokenizer_cache_size",
    "Current number of tokenizers in cache",
)

tokenizer_cache_evictions_total = Counter(
    "preprocessor_tokenizer_cache_evictions_total",
    "Total number of tokenizer cache evictions",
    ["model_id"],
)

# Active requests gauge
active_requests = Gauge(
    "preprocessor_active_requests",
    "Current number of active preprocessing requests",
)

# Token count metrics
tokens_processed_total = Counter(
    "preprocessor_tokens_processed_total",
    "Total number of tokens processed",
    ["model_id", "mode"],  # mode: "string", "tensor"
)


class MetricsContext:
    """Context manager for tracking request metrics."""

    def __init__(self, model_id: str, engine_type: str):
        """Initialize metrics context.

        Args:
            model_id: HuggingFace model identifier
            engine_type: Engine type (vllm, triton_tensor, etc.)
        """
        self.model_id = model_id
        self.engine_type = engine_type
        self.timer: Optional[object] = None

    def __enter__(self):
        """Enter context: increment active requests and start timer."""
        active_requests.inc()
        self.timer = preprocess_request_duration_seconds.labels(
            model_id=self.model_id,
            engine_type=self.engine_type,
        ).time()
        self.timer.__enter__()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """Exit context: decrement active requests and record status."""
        active_requests.dec()
        if self.timer:
            self.timer.__exit__(exc_type, exc_val, exc_tb)

        status = "error" if exc_type else "success"
        preprocess_requests_total.labels(
            model_id=self.model_id,
            engine_type=self.engine_type,
            status=status,
        ).inc()

        # Record error type if failed
        if exc_type:
            error_type = "internal"
            if exc_val and "tokenizer" in str(exc_val).lower():
                error_type = "tokenizer_load"
            elif exc_val and "template" in str(exc_val).lower():
                error_type = "template_apply"

            preprocess_errors_total.labels(
                model_id=self.model_id,
                error_type=error_type,
            ).inc()


def record_token_count(model_id: str, mode: str, count: int):
    """Record token count metrics.

    Args:
        model_id: HuggingFace model identifier
        mode: Processing mode (string or tensor)
        count: Number of tokens processed
    """
    tokens_processed_total.labels(
        model_id=model_id,
        mode=mode,
    ).inc(count)


def record_tokenizer_cache_hit(model_id: str):
    """Record a tokenizer cache hit.

    Args:
        model_id: HuggingFace model identifier
    """
    tokenizer_cache_hits_total.labels(model_id=model_id).inc()


def record_tokenizer_cache_miss(model_id: str):
    """Record a tokenizer cache miss (load).

    Args:
        model_id: HuggingFace model identifier
    """
    tokenizer_cache_misses_total.labels(model_id=model_id).inc()


def record_tokenizer_eviction(model_id: str):
    """Record a tokenizer cache eviction.

    Args:
        model_id: HuggingFace model identifier
    """
    tokenizer_cache_evictions_total.labels(model_id=model_id).inc()


def update_tokenizer_cache_size(size: int):
    """Update the tokenizer cache size gauge.

    Args:
        size: Current number of tokenizers in cache
    """
    tokenizer_cache_size.set(size)
