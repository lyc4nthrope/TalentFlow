"""Métricas Prometheus del servicio (Reto 8, D5). Infraestructura (D7): fuera del umbral de
cobertura de negocio, con su propia prueba (tests/test_metricas.py)."""

from time import perf_counter

from fastapi import FastAPI, Response
from prometheus_client import (
    CONTENT_TYPE_LATEST,
    CollectorRegistry,
    Counter,
    GCCollector,
    Histogram,
    PlatformCollector,
    ProcessCollector,
    generate_latest,
)

RUTA_METRICAS = "/metrics"
# Lo que no casa con ninguna ruta: sin esto, cada URL inventada sería una serie nueva.
RUTA_OTRA = "other"
# Hasta 10 s: el umbral de la alerta de latencia (2 s) cae en un bucket exacto.
BUCKETS_SEGUNDOS = (0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10)
_ETIQUETAS = ("method", "route", "status")


class _MiddlewareMetricas:
    """ASGI puro: cuenta y cronometra cada petición por método, patrón de ruta y código."""

    def __init__(self, app, peticiones: Counter, duracion: Histogram):
        self.app = app
        self.peticiones = peticiones
        self.duracion = duracion

    async def __call__(self, scope, receive, send):
        if scope["type"] != "http" or scope["path"] == RUTA_METRICAS:
            await self.app(scope, receive, send)
            return
        estado = 500  # si la app revienta sin responder, cuenta como error del servidor
        inicio = perf_counter()

        async def enviar(mensaje):
            nonlocal estado
            if mensaje["type"] == "http.response.start":
                estado = mensaje["status"]
            await send(mensaje)

        try:
            await self.app(scope, receive, enviar)
        finally:
            # El router de FastAPI deja en el scope la ruta que casó: se usa su patrón
            # (/perfiles/{empleadoId}) y no la URL, para no crear una serie por cada id.
            ruta = scope.get("route")
            etiquetas = (scope["method"], ruta.path if ruta else RUTA_OTRA, str(estado))
            self.peticiones.labels(*etiquetas).inc()
            self.duracion.labels(*etiquetas).observe(perf_counter() - inicio)


def instrumentar(app: FastAPI) -> CollectorRegistry:
    """Registro propio (no el global) con CPU, memoria y GC del proceso y el contrato común
    de los 7 servicios; publica GET /metrics, que no se cuenta a sí mismo."""
    registro = CollectorRegistry()
    ProcessCollector(registry=registro)
    PlatformCollector(registry=registro)
    GCCollector(registry=registro)
    peticiones = Counter("http_requests", "Peticiones HTTP recibidas", _ETIQUETAS, registry=registro)
    duracion = Histogram(
        "http_request_duration_seconds",
        "Duración de las peticiones HTTP en segundos",
        _ETIQUETAS,
        buckets=BUCKETS_SEGUNDOS,
        registry=registro,
    )
    app.add_middleware(_MiddlewareMetricas, peticiones=peticiones, duracion=duracion)

    @app.get(RUTA_METRICAS, include_in_schema=False)
    async def metricas() -> Response:
        return Response(generate_latest(registro), media_type=CONTENT_TYPE_LATEST)

    return registro
