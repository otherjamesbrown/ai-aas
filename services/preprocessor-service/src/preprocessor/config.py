"""Configuration management for the preprocessor service."""

from pydantic_settings import BaseSettings
from pydantic import Field


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""

    # gRPC server configuration
    grpc_port: int = Field(default=50051, description="gRPC server port")
    grpc_max_workers: int = Field(default=10, description="Max gRPC worker threads")

    # Logging
    log_level: str = Field(default="INFO", description="Log level (DEBUG, INFO, WARNING, ERROR)")
    log_format: str = Field(default="json", description="Log format (json, console)")

    # HuggingFace configuration
    hf_home: str = Field(default="/app/hf_cache", description="HuggingFace cache directory")
    hf_token: str | None = Field(default=None, description="HuggingFace API token (optional)")
    transformers_offline: bool = Field(
        default=False,
        description="Use local files only, no network access (set via TRANSFORMERS_OFFLINE=1)"
    )

    # Tokenizer cache settings
    max_cached_tokenizers: int = Field(default=50, description="Max tokenizers to keep in memory")
    tokenizer_load_timeout: int = Field(default=30, description="Timeout for loading tokenizers (seconds)")

    # Pre-loaded models (comma-separated)
    preload_models: str = Field(
        default="meta-llama/Llama-3.1-8B-Instruct",
        description="Models to preload at startup (comma-separated)"
    )

    class Config:
        env_prefix = ""
        case_sensitive = False


settings = Settings()
