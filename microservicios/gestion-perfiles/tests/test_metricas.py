from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.api import crear_app
from app.metricas import instrumentar
from tests.fakes import RepositorioEnMemoria


def test_cuenta_por_patron_metodo_y_estado():
    cliente = TestClient(crear_app(RepositorioEnMemoria(), lambda: "UP"))
    cliente.get("/perfiles/E001")
    cliente.get("/perfiles/E002")
    cliente.get("/no-existe")
    cliente.get("/metrics")

    texto = cliente.get("/metrics").text
    assert 'http_requests_total{method="GET",route="/perfiles/{empleadoId}",status="404"} 2.0' in texto
    assert 'http_requests_total{method="GET",route="other",status="404"} 1.0' in texto
    assert (
        'http_request_duration_seconds_bucket{le="2.0",method="GET",route="other",status="404"} 1.0'
        in texto
    )
    assert 'route="/metrics"' not in texto
    for metrica in ("process_cpu_seconds_total", "process_resident_memory_bytes", "python_gc_"):
        assert metrica in texto


def test_un_fallo_sin_respuesta_cuenta_como_500():
    app = FastAPI()
    instrumentar(app)

    @app.get("/falla")
    async def falla():
        raise RuntimeError("boom")

    cliente = TestClient(app, raise_server_exceptions=False)
    assert cliente.get("/falla").status_code == 500
    assert 'http_requests_total{method="GET",route="/falla",status="500"} 1.0' in cliente.get("/metrics").text
