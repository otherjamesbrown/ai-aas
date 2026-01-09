"""Main entry point for the preprocessor gRPC service."""

import logging
import signal
import sys
import threading
from concurrent import futures
from wsgiref.simple_server import make_server, WSGIServer

import grpc
import structlog
from grpc_health.v1 import health, health_pb2, health_pb2_grpc
from grpc_reflection.v1alpha import reflection
from prometheus_client import make_wsgi_app

from . import preprocessor_pb2
from . import preprocessor_pb2_grpc
from .config import settings
from .service import PreprocessorServicer
from .tokenizer_cache import tokenizer_cache

# Configure Python's standard logging level (required for structlog filter_by_level)
logging.basicConfig(
    format="%(message)s",
    stream=sys.stdout,
    level=getattr(logging, settings.log_level.upper(), logging.INFO),
)

# Configure structured logging
structlog.configure(
    processors=[
        structlog.stdlib.filter_by_level,
        structlog.stdlib.add_logger_name,
        structlog.stdlib.add_log_level,
        structlog.stdlib.PositionalArgumentsFormatter(),
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.StackInfoRenderer(),
        structlog.processors.format_exc_info,
        structlog.processors.UnicodeDecoder(),
        structlog.processors.JSONRenderer() if settings.log_format == "json"
        else structlog.dev.ConsoleRenderer(),
    ],
    wrapper_class=structlog.stdlib.BoundLogger,
    context_class=dict,
    logger_factory=structlog.stdlib.LoggerFactory(),
    cache_logger_on_first_use=True,
)

logger = structlog.get_logger(__name__)


def start_metrics_server() -> WSGIServer:
    """Start the Prometheus metrics HTTP server.

    Returns:
        The metrics server instance
    """
    metrics_app = make_wsgi_app()
    metrics_server = make_server("", settings.metrics_port, metrics_app)

    def serve_metrics():
        logger.info("metrics_server_started", port=settings.metrics_port)
        metrics_server.serve_forever()

    # Run metrics server in a daemon thread
    metrics_thread = threading.Thread(target=serve_metrics, daemon=True)
    metrics_thread.start()

    return metrics_server


def serve() -> None:
    """Start the gRPC server."""
    # Start metrics server
    metrics_server = start_metrics_server()

    # Create gRPC server
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=settings.grpc_max_workers),
        options=[
            ("grpc.max_send_message_length", 16 * 1024 * 1024),  # 16MB
            ("grpc.max_receive_message_length", 16 * 1024 * 1024),  # 16MB
            ("grpc.keepalive_time_ms", 30000),  # 30s
            ("grpc.keepalive_timeout_ms", 10000),  # 10s
            ("grpc.keepalive_permit_without_calls", True),
        ],
    )

    # Add preprocessor service
    preprocessor_pb2_grpc.add_PreprocessorServiceServicer_to_server(
        PreprocessorServicer(),
        server,
    )

    # Add health service (for Kubernetes probes)
    health_servicer = health.HealthServicer()
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)

    # Set health status
    health_servicer.set(
        "preprocessor.PreprocessorService",
        health_pb2.HealthCheckResponse.SERVING,
    )

    # Add reflection service (for grpcurl debugging)
    service_names = (
        preprocessor_pb2.DESCRIPTOR.services_by_name["PreprocessorService"].full_name,
        reflection.SERVICE_NAME,
        health.SERVICE_NAME,
    )
    reflection.enable_server_reflection(service_names, server)

    # Bind to port
    address = f"[::]:{settings.grpc_port}"
    server.add_insecure_port(address)

    logger.info(
        "starting_grpc_server",
        address=address,
        max_workers=settings.grpc_max_workers,
    )

    # Preload tokenizers if configured
    if settings.preload_models:
        model_ids = [m.strip() for m in settings.preload_models.split(",") if m.strip()]
        if model_ids:
            logger.info("preloading_tokenizers", models=model_ids)
            results = tokenizer_cache.preload(model_ids)
            for model_id, success in results.items():
                if success:
                    logger.info("tokenizer_preloaded", model_id=model_id)
                else:
                    logger.warning("tokenizer_preload_failed", model_id=model_id)

    # Start server
    server.start()
    logger.info("grpc_server_started", port=settings.grpc_port)

    # Handle shutdown signals
    def handle_shutdown(signum, frame):
        logger.info("shutdown_signal_received", signal=signum)
        health_servicer.set(
            "preprocessor.PreprocessorService",
            health_pb2.HealthCheckResponse.NOT_SERVING,
        )
        # Grace period for in-flight requests
        server.stop(grace=10)
        metrics_server.shutdown()
        logger.info("servers_stopped")
        sys.exit(0)

    signal.signal(signal.SIGTERM, handle_shutdown)
    signal.signal(signal.SIGINT, handle_shutdown)

    # Wait for termination
    server.wait_for_termination()


def main() -> None:
    """Main entry point."""
    logger.info(
        "preprocessor_service_starting",
        version="0.1.0",
        log_level=settings.log_level,
        hf_home=settings.hf_home,
    )

    try:
        serve()
    except Exception as e:
        logger.exception("server_failed", error=str(e))
        sys.exit(1)


if __name__ == "__main__":
    main()
