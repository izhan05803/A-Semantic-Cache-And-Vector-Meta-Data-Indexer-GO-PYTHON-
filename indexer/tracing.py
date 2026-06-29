import logging
import os

logger = logging.getLogger("indexer_tracing")

try:
    from opentelemetry import trace
    from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
    from opentelemetry.instrumentation.grpc import GrpcInstrumentorServer
    from opentelemetry.sdk.resources import Resource
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import BatchSpanProcessor

    _enabled = True
except ImportError:
    logger.warning("OpenTelemetry packages not installed — tracing disabled")
    _enabled = False


def init_tracing():
    if not _enabled:
        return

    endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")

    resource = Resource.create({
        "service.name": "semantic-cache-indexer",
        "service.version": "1.0.0",
    })

    exporter = OTLPSpanExporter(endpoint=endpoint, insecure=True)

    provider = TracerProvider(resource=resource)
    provider.add_span_processor(BatchSpanProcessor(exporter))

    trace.set_tracer_provider(provider)

    GrpcInstrumentorServer().instrument()
    logger.info("OpenTelemetry tracing initialized, endpoint=%s", endpoint)
