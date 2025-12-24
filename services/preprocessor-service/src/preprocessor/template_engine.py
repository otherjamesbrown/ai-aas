"""Template engine for applying model-specific chat templates."""

from typing import Any

import structlog
from transformers import PreTrainedTokenizerBase

from .tokenizer_cache import tokenizer_cache

logger = structlog.get_logger(__name__)


class TemplateEngine:
    """Applies HuggingFace chat templates to messages."""

    def __init__(self):
        self._cache = tokenizer_cache

    def apply_chat_template(
        self,
        model_id: str,
        messages: list[dict[str, str]],
        add_generation_prompt: bool = True,
        tokenize: bool = False,
    ) -> str | list[int]:
        """Apply the model's chat template to format messages.

        This uses HuggingFace's apply_chat_template which handles:
        - Llama-3 format: <|begin_of_text|><|start_header_id|>system<|end_header_id|>...
        - Harmony format: <|start|>user\n...<|end|>
        - ChatML format: <|im_start|>system\n...<|im_end|>
        - Mistral format: [INST] ... [/INST]
        - And many more via the model's tokenizer_config.json

        Args:
            model_id: HuggingFace model identifier
            messages: List of chat messages in OpenAI format
                [{"role": "user", "content": "Hello"}]
            add_generation_prompt: Whether to add the assistant prompt prefix
            tokenize: If True, return token IDs instead of string

        Returns:
            Formatted prompt string or list of token IDs

        Raises:
            ValueError: If model doesn't support chat templates
        """
        tokenizer = self._cache.get(model_id)

        # Convert messages to HuggingFace format
        hf_messages = self._convert_messages(messages)

        # Check if tokenizer supports chat templates
        if not hasattr(tokenizer, "apply_chat_template"):
            raise ValueError(
                f"Model {model_id} tokenizer does not support chat templates. "
                "Consider using a model with a chat_template in tokenizer_config.json"
            )

        try:
            result = tokenizer.apply_chat_template(
                hf_messages,
                tokenize=tokenize,
                add_generation_prompt=add_generation_prompt,
                return_tensors=None,  # Return list, not tensor
            )

            if tokenize:
                logger.debug(
                    "template_applied_tokenized",
                    model_id=model_id,
                    num_messages=len(messages),
                    num_tokens=len(result),
                )
            else:
                logger.debug(
                    "template_applied",
                    model_id=model_id,
                    num_messages=len(messages),
                    prompt_length=len(result),
                )

            return result

        except Exception as e:
            logger.error(
                "template_apply_failed",
                model_id=model_id,
                error=str(e),
                num_messages=len(messages),
            )
            raise ValueError(f"Failed to apply chat template: {e}") from e

    def count_tokens(self, model_id: str, text: str) -> int:
        """Count tokens in a text string.

        Args:
            model_id: HuggingFace model identifier
            text: Text to tokenize

        Returns:
            Number of tokens
        """
        tokenizer = self._cache.get(model_id)
        tokens = tokenizer.encode(text, add_special_tokens=False)
        return len(tokens)

    def tokenize(self, model_id: str, text: str) -> list[int]:
        """Tokenize text to token IDs.

        Args:
            model_id: HuggingFace model identifier
            text: Text to tokenize

        Returns:
            List of token IDs
        """
        tokenizer = self._cache.get(model_id)
        return tokenizer.encode(text, add_special_tokens=False)

    def _convert_messages(self, messages: list[dict[str, str]]) -> list[dict[str, str]]:
        """Convert OpenAI-format messages to HuggingFace format.

        OpenAI and HuggingFace use the same format:
        [{"role": "user", "content": "Hello"}]

        This method validates the format and handles any edge cases.
        """
        converted = []
        for msg in messages:
            role = msg.get("role", "")
            content = msg.get("content", "")

            # Validate role
            if role not in ("system", "user", "assistant", "tool"):
                logger.warning(
                    "unknown_message_role",
                    role=role,
                    valid_roles=["system", "user", "assistant", "tool"],
                )

            converted.append({
                "role": role,
                "content": content,
            })

        return converted

    def get_tokenizer_info(self, model_id: str) -> dict[str, Any]:
        """Get information about a tokenizer.

        Args:
            model_id: HuggingFace model identifier

        Returns:
            Dict with tokenizer metadata
        """
        tokenizer = self._cache.get(model_id)

        info = {
            "model_id": model_id,
            "vocab_size": tokenizer.vocab_size,
            "model_max_length": getattr(tokenizer, "model_max_length", None),
            "has_chat_template": hasattr(tokenizer, "chat_template") and tokenizer.chat_template is not None,
            "special_tokens": {
                "bos_token": getattr(tokenizer, "bos_token", None),
                "eos_token": getattr(tokenizer, "eos_token", None),
                "pad_token": getattr(tokenizer, "pad_token", None),
            },
        }

        # Try to get a sample of the chat template
        if info["has_chat_template"]:
            try:
                sample = self.apply_chat_template(
                    model_id,
                    [{"role": "user", "content": "test"}],
                    add_generation_prompt=True,
                )
                # Show first 100 chars as a preview
                info["template_preview"] = sample[:100] + "..." if len(sample) > 100 else sample
            except Exception:
                info["template_preview"] = None

        return info


# Global template engine instance
template_engine = TemplateEngine()
