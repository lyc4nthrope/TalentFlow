"""Pruebas de integración contra un PostgreSQL real (pgserver) y un broker simulado."""
import tempfile
from datetime import datetime
from pathlib import Path
from zoneinfo import ZoneInfo

import pgserver
import pytest
from fastapi.testclient import TestClient

from app.config import Settings
from app.main import create_app

INIT_SQL = Path(__file__).resolve().parent.parent / "init.sql"
ZONA = ZoneInfo("America/Bogota")


class BrokerFalso:
    """Reemplaza a RabbitMQ: guarda lo publicado para poder inspeccionarlo."""

    def __init__(self):
        self.publicados = []
        self.fallar = False

    async def publicar(self, tipo, data):
        if self.fallar:
            raise RuntimeError("broker caído")
        self.publicados.append((tipo, data))
        return {"type": tipo, "data": data}

    def conectado(self):
        return True


class Reloj:
    def __init__(self):
        self.ahora = datetime(2026, 10, 10, 12, 0, tzinfo=ZONA)

    def __call__(self):
        return self.ahora


@pytest.fixture(scope="session")
def pg_uri():
    with tempfile.TemporaryDirectory() as d:
        servidor = pgserver.get_server(Path(d) / "pgdata")
        servidor.psql(INIT_SQL.read_text())
        yield servidor.get_uri()
        servidor.cleanup()


@pytest.fixture
def entorno(pg_uri):
    broker, reloj = BrokerFalso(), Reloj()
    settings = Settings(
        port=8085, db_host="", db_port=0, db_name="", db_user="", db_pass="",
        broker_url="", broker_exchange="x", zona_horaria="America/Bogota", dsn_url=pg_uri,
    )
    app = create_app(settings, broker=broker, arrancar_consumidores=False, reloj=reloj)
    with TestClient(app) as client:
        # Limpieza entre pruebas
        async def limpiar():
            async with app.state.repo.transaccion() as conn:
                await conn.execute(
                    "TRUNCATE vacaciones, secuencias, empleados_replica, eventos_procesados"
                )
        client.portal.call(limpiar)
        yield client, app, broker, reloj
