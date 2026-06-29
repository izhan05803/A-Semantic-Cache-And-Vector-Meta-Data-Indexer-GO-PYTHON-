import sys
import os
import logging
import signal
import threading
from concurrent import futures

# Configure structured logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("indexer_server")

# Ensure this script's directory is on the path so local imports work
_script_dir = os.path.dirname(os.path.abspath(__file__))
if _script_dir not in sys.path:
    sys.path.insert(0, _script_dir)

import grpc
from grpc_health.v1 import health_pb2, health_pb2_grpc
from proto_gen import cache_service_pb2
from proto_gen import cache_service_pb2_grpc
from indexer import Indexer
from config import settings
from metrics import REQUESTS_TOTAL, CACHE_HITS, CACHE_MISSES, UPDATES_TOTAL, REQUEST_DURATION, start_metrics_server
from tracing import init_tracing

class CacheService(cache_service_pb2_grpc.CacheServiceServicer):
    def __init__(self):
        self.indexer = Indexer()

    def CheckCache(self, request, context):
        logger.info(f"Received CheckCache request for prompt: {request.prompt[:50]}...")
        REQUESTS_TOTAL.labels(method="CheckCache").inc()

        with REQUEST_DURATION.labels(method="CheckCache").time():
            response_content = self.indexer.search_cache(request.prompt, request.similarity_threshold)

        if response_content:
            CACHE_HITS.inc()
            logger.info("Cache hit")
            return cache_service_pb2.CacheResponse(hit=True, response=response_content, score=1.0)
        else:
            CACHE_MISSES.inc()
            logger.info("Cache miss")
            return cache_service_pb2.CacheResponse(hit=False, response="", score=0.0)

    def UpdateCache(self, request, context):
        logger.info(f"Received UpdateCache request for prompt: {request.prompt[:50]}...")
        REQUESTS_TOTAL.labels(method="UpdateCache").inc()
        UPDATES_TOTAL.inc()

        with REQUEST_DURATION.labels(method="UpdateCache").time():
            self.indexer.add_to_cache(request.prompt, request.response, dict(request.metadata))
        return cache_service_pb2.UpdateResponse(success=True)


class HealthServicer(health_pb2_grpc.HealthServicer):
    def Check(self, request, context):
        return health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.SERVING
        )

    def Watch(self, request, context):
        yield health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.SERVING
        )


class AuthInterceptor(grpc.ServerInterceptor):
    """Rejects requests without a valid x-api-key in metadata"""

    def __init__(self, key):
        self._valid_key = key

    def intercept_service(self, continuation, handler_call_details):
        handler = continuation(handler_call_details)
        if handler and not handler.request_streaming and not handler.response_streaming:
            return grpc.unary_unary_rpc_method_handler(
                self._wrap_unary(handler.unary_unary),
                request_deserializer=handler.request_deserializer,
                response_serializer=handler.response_serializer,
            )
        return handler

    def _wrap_unary(self, behavior):
        def wrapper(request, context):
            metadata = dict(context.invocation_metadata())
            if metadata.get("x-api-key") != self._valid_key:
                context.abort(grpc.StatusCode.UNAUTHENTICATED, "invalid API key")
            return behavior(request, context)
        return wrapper


def serve():
    init_tracing()
    start_metrics_server(settings.METRICS_PORT)

    api_key = settings.INDEXER_API_KEY
    if not api_key:
        logger.warning("INDEXER_API_KEY not set — gRPC auth disabled")

    interceptor = AuthInterceptor(api_key) if api_key else None
    interceptors = [interceptor] if interceptor else []

    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=10),
        interceptors=interceptors,
    )

    # Register services
    cache_service_pb2_grpc.add_CacheServiceServicer_to_server(CacheService(), server)
    health_pb2_grpc.add_HealthServicer_to_server(HealthServicer(), server)

    server.add_insecure_port('[::]:50051')
    server.start()
    logger.info("Indexer gRPC server started on port 50051")

    # Graceful shutdown
    stop_event = threading.Event()

    def handle_signal(signum, frame):
        logger.info(f"Received signal {signum}, initiating graceful shutdown...")
        stop_event.set()

    signal.signal(signal.SIGTERM, handle_signal)
    signal.signal(signal.SIGINT, handle_signal)

    stop_event.wait()
    logger.info("Shutting down server (5s grace period)...")
    server.stop(5)
    logger.info("Server stopped")


if __name__ == '__main__':
    serve()
