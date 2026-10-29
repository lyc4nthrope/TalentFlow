from fastapi.testclient import TestClient
from opentelemetry.trace import SpanKind
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter

from app import trazas
from app.api import crear_app
from tests.fakes import RepositorioEnMemoria


def test_apagado_no_instrumenta(monkeypatch):
    monkeypatch.setenv("OTEL_SDK_DISABLED", "true")
    assert trazas.iniciar(crear_app(RepositorioEnMemoria(), lambda: "UP")) is None


def test_span_por_patron_que_continua_el_traceparent_y_sin_ruido(monkeypatch):
    monkeypatch.delenv("OTEL_SDK_DISABLED", raising=False)
    exportador = InMemorySpanExporter()
    app = crear_app(RepositorioEnMemoria(), lambda: "UP")
    proveedor = trazas.iniciar(app, exportador)
    cliente = TestClient(app)

    cliente.get("/health")
    cliente.get("/metrics")
    cliente.get(
        "/perfiles/E001",
        headers={"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"},
    )
    # Una consulta sin petición que la origine (el ping del healthcheck) no se exporta.
    proveedor.get_tracer("prueba").start_span("SELECT", kind=SpanKind.CLIENT).end()
    proveedor.force_flush()

    assert "SELECT" not in [s.name for s in exportador.get_finished_spans()]

    servidor = [s for s in exportador.get_finished_spans() if s.kind.name == "SERVER"]
    assert [s.name for s in servidor] == ["GET /perfiles/{empleadoId}"]
    assert format(servidor[0].context.trace_id, "032x") == "4bf92f3577b34da6a3ce929d0e0e4736"
    proveedor.shutdown()
