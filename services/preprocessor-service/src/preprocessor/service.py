"""gRPC service implementation for the preprocessor."""

import grpc
import structlog

from . import preprocessor_pb2
from . import preprocessor_pb2_grpc
from .template_engine import template_engine
from .tokenizer_cache import tokenizer_cache

logger = structlog.get_logger(__name__)


class PreprocessorServicer(preprocessor_pb2_grpc.PreprocessorServiceServicer):
    """gRPC servicer for preprocessing inference requests."""

    def Preprocess(
        self,
        request: preprocessor_pb2.PreprocessRequest,
        context: grpc.ServicerContext,
    ) -> preprocessor_pb2.PreprocessResponse:
        """Apply model-specific preprocessing to chat messages.

        Args:
            request: PreprocessRequest with model_id, engine_type, and messages
            context: gRPC context

        Returns:
            PreprocessResponse with formatted prompt or token IDs
        """
        log = logger.bind(
            model_id=request.model_id,
            engine_type=request.engine_type,
            num_messages=len(request.messages),
        )
        log.info("preprocess_request_received")

        try:
            # Convert proto messages to dict format
            messages = [
                {"role": msg.role, "content": msg.content}
                for msg in request.messages
            ]

            # Determine output mode based on engine type
            if request.engine_type in ("triton_tensor",):
                # Tensor mode: return tokenized input IDs
                return self._preprocess_tensor_mode(request, messages, log)
            else:
                # String mode: return formatted prompt (vllm, triton_string, or default)
                return self._preprocess_string_mode(request, messages, log)

        except ValueError as e:
            log.error("preprocess_failed", error=str(e))
            return preprocessor_pb2.PreprocessResponse(
                error_message=str(e),
            )
        except Exception as e:
            log.exception("preprocess_unexpected_error")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Internal error: {e}")
            return preprocessor_pb2.PreprocessResponse(
                error_message=f"Internal error: {e}",
            )

    def _preprocess_string_mode(
        self,
        request: preprocessor_pb2.PreprocessRequest,
        messages: list[dict[str, str]],
        log: structlog.BoundLogger,
    ) -> preprocessor_pb2.PreprocessResponse:
        """Preprocess for string output mode (vLLM, triton_string)."""
        # Apply chat template
        formatted_prompt = template_engine.apply_chat_template(
            model_id=request.model_id,
            messages=messages,
            add_generation_prompt=True,
            tokenize=False,
        )

        # Count tokens for usage tracking
        token_count = template_engine.count_tokens(request.model_id, formatted_prompt)

        log.info(
            "preprocess_string_complete",
            prompt_length=len(formatted_prompt),
            token_count=token_count,
        )

        return preprocessor_pb2.PreprocessResponse(
            formatted_prompt=formatted_prompt,
            prompt_token_count=token_count,
        )

    def _preprocess_tensor_mode(
        self,
        request: preprocessor_pb2.PreprocessRequest,
        messages: list[dict[str, str]],
        log: structlog.BoundLogger,
    ) -> preprocessor_pb2.PreprocessResponse:
        """Preprocess for tensor output mode (triton_tensor/TRT-LLM)."""
        # Apply chat template with tokenization
        input_ids = template_engine.apply_chat_template(
            model_id=request.model_id,
            messages=messages,
            add_generation_prompt=True,
            tokenize=True,
        )

        # Ensure input_ids is a list of ints
        if hasattr(input_ids, "tolist"):
            input_ids = input_ids.tolist()

        # Shape is [1, seq_len] for single request
        input_shape = [1, len(input_ids)]

        log.info(
            "preprocess_tensor_complete",
            num_tokens=len(input_ids),
            input_shape=input_shape,
        )

        return preprocessor_pb2.PreprocessResponse(
            input_ids=input_ids,
            input_shape=input_shape,
            prompt_token_count=len(input_ids),
        )

    def HealthCheck(
        self,
        request: preprocessor_pb2.HealthCheckRequest,
        context: grpc.ServicerContext,
    ) -> preprocessor_pb2.HealthCheckResponse:
        """Check service health and list loaded tokenizers.

        Args:
            request: Empty health check request
            context: gRPC context

        Returns:
            HealthCheckResponse with ready status and loaded tokenizers
        """
        loaded = tokenizer_cache.list_loaded()

        return preprocessor_pb2.HealthCheckResponse(
            ready=True,
            message=f"Service ready with {len(loaded)} tokenizers loaded",
            loaded_tokenizers=loaded,
        )
