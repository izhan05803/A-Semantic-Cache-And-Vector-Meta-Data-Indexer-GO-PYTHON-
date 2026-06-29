import logging
import threading

from prometheus_client import Counter, Histogram, start_http_server

logger = logging.getLogger("indexer_metrics")

REQUESTS_TOTAL = Counter(
    'indexer_requests_total',
    'Total gRPC requests processed',
    ['method']
)
CACHE_HITS = Counter(
    'indexer_cache_hits_total',
    'Total cache hits served'
)
CACHE_MISSES = Counter(
    'indexer_cache_misses_total',
    'Total cache misses (no match above threshold)'
)
UPDATES_TOTAL = Counter(
    'indexer_updates_total',
    'Total cache update requests'
)
REQUEST_DURATION = Histogram(
    'indexer_request_duration_seconds',
    'Latency of gRPC requests in seconds',
    ['method'],
    buckets=(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10)
)


def start_metrics_server(port: int = 8000) -> threading.Thread:
    """Start a Prometheus metrics HTTP server in a daemon thread."""
    thread = threading.Thread(target=start_http_server, args=(port,), daemon=True)
    thread.start()
    logger.info("Prometheus metrics HTTP server started on port %d", port)
    return thread
