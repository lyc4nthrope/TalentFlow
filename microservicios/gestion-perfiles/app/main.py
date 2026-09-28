"""Punto de entrada: conecta la BD, el consumidor de eventos y la API."""

import asyncio
import contextlib
import logging
from contextlib import asynccontextmanager

from psycopg_pool import AsyncConnectionPool

from app.api import crear_app
from app.aplicacion import ProcesadorEventos
from app.config import Config
from app.consumidor import Consumidor
from app.repositorio import RepositorioPostgres

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s service=perfiles-service logger=%(name)s %(message)s",
)
logger = logging.getLogger("perfiles")


class _SinHealthcheck(logging.Filter):
    """El healthcheck de Docker consulta /health cada 5 s: sin este filtro, el log de
    acceso se llena de esas líneas y tapa los eventos. Las demás peticiones se registran."""

    def filter(self, registro: logging.LogRecord) -> bool:
        return "GET /health " not in registro.getMessage()


logging.getLogger("uvicorn.access").addFilter(_SinHealthcheck())

config = Config.desde_entorno()
pool = AsyncConnectionPool(config.url_postgres, min_size=1, max_size=5, open=False)
repositorio = RepositorioPostgres(pool)
consumidor = Consumidor(config.url_rabbitmq, config.cola, ProcesadorEventos(repositorio))


def _registrar_fallo(tarea: asyncio.Task) -> None:
    # Sin esto, una excepción en la tarea del consumidor se perdería en silencio.
    if not tarea.cancelled() and tarea.exception() is not None:
        logger.critical("el consumidor de eventos se detuvo", exc_info=tarea.exception())


@asynccontextmanager
async def ciclo_de_vida(_app):
    await pool.open(wait=False)  # no bloquea el arranque si la BD tarda en responder
    tarea = asyncio.create_task(consumidor.ejecutar(), name="consumidor-eventos")
    tarea.add_done_callback(_registrar_fallo)
    yield
    # Cierre ordenado: detener el consumo primero (los mensajes sin confirmar vuelven a
    # la cola) y luego cerrar conexiones.
    tarea.cancel()
    with contextlib.suppress(asyncio.CancelledError):
        await tarea
    await consumidor.cerrar()
    await pool.close()


app = crear_app(repositorio, consumidor.estado, lifespan=ciclo_de_vida)
