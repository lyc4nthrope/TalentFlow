"""Logs JSON (Reto 8, F8): formato común de los 7 servicios (timestamp, level, service,
message, traceId, spanId) para Loki; Grafana enlaza el traceId con Jaeger. Infraestructura
(D7): fuera del umbral de cobertura, con su prueba (tests/test_logs.py)."""

import json
import logging
from datetime import UTC, datetime

from opentelemetry import trace


class FormatoJson(logging.Formatter):
    def __init__(self, servicio: str):
        super().__init__()
        self.servicio = servicio

    def format(self, registro: logging.LogRecord) -> str:
        linea = {
            "timestamp": datetime.fromtimestamp(registro.created, UTC).isoformat(timespec="milliseconds"),
            "level": registro.levelname,
            "service": self.servicio,
            "logger": registro.name,
            "message": registro.getMessage(),
        }
        contexto = trace.get_current_span().get_span_context()
        if contexto.is_valid:
            linea["traceId"] = format(contexto.trace_id, "032x")
            linea["spanId"] = format(contexto.span_id, "016x")
        if registro.exc_info:
            linea["error"] = self.formatException(registro.exc_info)
        # Sin espacios, como los demás servicios: Grafana busca "traceId":"<id>" (derivedFields).
        return json.dumps(linea, ensure_ascii=False, separators=(",", ":"))


def configurar(servicio: str, filtros: tuple[logging.Filter, ...] = ()) -> None:
    """Un solo manejador JSON para la app y para uvicorn (que ya configuró los suyos antes de
    importar la app, por eso se reemplazan aquí)."""
    manejador = logging.StreamHandler()
    manejador.setFormatter(FormatoJson(servicio))
    raiz = logging.getLogger()
    raiz.handlers = [manejador]
    raiz.setLevel(logging.INFO)
    for nombre in ("uvicorn", "uvicorn.error", "uvicorn.access"):
        registrador = logging.getLogger(nombre)
        registrador.handlers = []
        registrador.propagate = True
    for filtro in filtros:
        logging.getLogger("uvicorn.access").addFilter(filtro)
