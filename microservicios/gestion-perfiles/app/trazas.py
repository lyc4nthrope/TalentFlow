"""Trazas OpenTelemetry (Reto 8, D1, D7, D10 y D12): spans de cada petición HTTP, de cada
consulta a Postgres y de cada mensaje de RabbitMQ, enviados a Jaeger por OTLP/HTTP. Configuración por entorno
(OTEL_SERVICE_NAME, OTEL_EXPORTER_OTLP_ENDPOINT); OTEL_SDK_DISABLED=true lo apaga.
Infraestructura: fuera del umbral de cobertura, con su prueba (tests/test_trazas.py)."""

import os

from fastapi import FastAPI
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.aio_pika import AioPikaInstrumentor
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.psycopg import PsycopgInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor, SpanExporter
from opentelemetry.sdk.trace.sampling import Decision, ParentBased, Sampler, SamplingResult
from opentelemetry.trace import SpanKind

# Healthchecks (cada 5 s) y scrapes de Prometheus: taparían las trazas de negocio.
RUTAS_EXCLUIDAS = "health,metrics"


class _SinConsultasSueltas(Sampler):
    """Muestrea el 100 % (D12) salvo los spans CLIENT sin padre: la consulta del ping a la BD
    del healthcheck, que sin esto sería una traza suelta de un solo SELECT cada 5 s."""

    def should_sample(self, parent_context, trace_id, name, kind=None, attributes=None, links=None,
                      trace_state=None) -> SamplingResult:
        if kind == SpanKind.CLIENT:
            return SamplingResult(Decision.DROP)
        return SamplingResult(Decision.RECORD_AND_SAMPLE, attributes)

    def get_description(self) -> str:
        return "SinConsultasSueltas"


def iniciar(app: FastAPI, exportador: SpanExporter | None = None) -> TracerProvider | None:
    """Registra el proveedor global e instrumenta la app. Debe llamarse antes de abrir el
    pool de conexiones, para que psycopg quede instrumentado."""
    if os.getenv("OTEL_SDK_DISABLED", "").lower() == "true":
        return None
    # Resource.create() lee OTEL_SERVICE_NAME y OTEL_RESOURCE_ATTRIBUTES.
    proveedor = TracerProvider(resource=Resource.create(), sampler=ParentBased(root=_SinConsultasSueltas()))
    proveedor.add_span_processor(BatchSpanProcessor(exportador or OTLPSpanExporter()))
    trace.set_tracer_provider(proveedor)
    FastAPIInstrumentor.instrument_app(app, tracer_provider=proveedor, excluded_urls=RUTAS_EXCLUIDAS)
    PsycopgInstrumentor().instrument(tracer_provider=proveedor, enable_commenter=False)
    # RabbitMQ (D10): cada mensaje consumido continúa la traza del traceparent de sus cabeceras.
    AioPikaInstrumentor().instrument(tracer_provider=proveedor)
    return proveedor
