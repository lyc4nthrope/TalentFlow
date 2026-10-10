"""Servicio de Gestión de Vacaciones (Python + FastAPI).

REST hacia RRHH (a través del API Gateway) + productor del evento vacaciones.programadas
+ consumidor de empleado.creado / empleado.retirado para su réplica local de empleados.
"""
import asyncio
import logging
from contextlib import asynccontextmanager
from datetime import date, datetime, timezone
from typing import Any, Optional

from fastapi import FastAPI, Query, Request, Response
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field

from .broker import Broker, RabbitBroker
from .config import Settings
from .consumidores import COLA, EVENTOS_CONSUMIDOS, ConsumidorEmpleados
from .errores import AppError
from .repository import VacacionesRepository, crear_pool
from .servicio import VacacionesService

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
log = logging.getLogger("vacaciones")

NOMBRE_SERVICIO = "vacaciones-service"
FRASES_ESTADO = {
    400: "Bad Request",
    404: "Not Found",
    409: "Conflict",
    500: "Internal Server Error",
    503: "Service Unavailable",
}


# ---------------------------------------------------------------- modelos OpenAPI

class ProgramarVacaciones(BaseModel):
    empleadoId: str = Field(min_length=1, max_length=20, examples=["E001"])
    fechaInicio: date = Field(examples=["2027-03-15"])
    fechaFin: date = Field(examples=["2027-03-30"])


class PeriodoVacaciones(BaseModel):
    id: str = Field(examples=["V-2026-0042"])
    empleadoId: str = Field(examples=["E001"])
    fechaInicio: date = Field(examples=["2027-03-15"])
    fechaFin: date = Field(examples=["2027-03-30"])
    estado: str = Field(
        description="PROGRAMADA → EN_CURSO → FINALIZADA, o CANCELADA. "
        "En el Reto 4 solo se llega a PROGRAMADA o CANCELADA.",
        examples=["PROGRAMADA"],
    )
    fechaCreacion: str = Field(examples=["2026-10-10T10:00:00Z"])


class ErrorCampo(BaseModel):
    campo: str
    mensaje: str


class ErrorRespuesta(BaseModel):
    status: int = Field(examples=[400])
    error: str = Field(examples=["Bad Request"])
    message: str
    timestamp: str
    path: str
    errors: Optional[list[ErrorCampo]] = None
    periodoEnConflicto: Optional[PeriodoVacaciones] = Field(
        default=None, description="Solo en el error de solapamiento: el período que se cruza."
    )


# ---------------------------------------------------------------- errores

def _cuerpo_error(status: int, mensaje: str, path: str, errores=None, extra=None) -> dict[str, Any]:
    cuerpo: dict[str, Any] = {
        "status": status,
        "error": FRASES_ESTADO.get(status, "Error"),
        "message": mensaje,
        "timestamp": datetime.now(timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z"),
        "path": path,
    }
    if errores:
        cuerpo["errors"] = errores
    if extra:
        cuerpo.update(extra)
    return cuerpo


def _instalar_manejadores_error(app: FastAPI) -> None:
    @app.exception_handler(AppError)
    async def _app_error(request: Request, exc: AppError):
        return JSONResponse(
            status_code=exc.codigo_estado,
            content=_cuerpo_error(exc.codigo_estado, exc.mensaje, request.url.path, exc.errores, exc.extra),
        )

    @app.exception_handler(RequestValidationError)
    async def _validacion(request: Request, exc: RequestValidationError):
        # El reto exige 400 (FastAPI usa 422 por defecto).
        errores = []
        json_invalido = False
        for e in exc.errors():
            if e.get("type") == "json_invalid":
                json_invalido = True
                continue
            loc = [str(p) for p in e.get("loc", ()) if p not in ("body", "query", "path")]
            errores.append({"campo": ".".join(loc) or "cuerpo", "mensaje": e.get("msg", "valor inválido")})
        mensaje = "Cuerpo JSON inválido" if json_invalido else "La petición no es válida"
        return JSONResponse(
            status_code=400,
            content=_cuerpo_error(400, mensaje, request.url.path, errores),
        )

    @app.exception_handler(404)
    async def _no_encontrado(request: Request, exc):
        return JSONResponse(status_code=404, content=_cuerpo_error(404, "Recurso no encontrado", request.url.path))

    @app.exception_handler(Exception)
    async def _inesperado(request: Request, exc: Exception):
        log.exception("Error no controlado en %s", request.url.path)
        return JSONResponse(
            status_code=500, content=_cuerpo_error(500, "Error interno del servidor", request.url.path)
        )


# ---------------------------------------------------------------- aplicación

def create_app(
    settings: Optional[Settings] = None,
    *,
    broker: Optional[Broker] = None,
    arrancar_consumidores: bool = True,
    reloj=None,
) -> FastAPI:
    settings = settings or Settings.desde_entorno()
    pool = crear_pool(settings.dsn)
    repo = VacacionesRepository(pool)
    broker = broker or RabbitBroker(settings.broker_url, settings.broker_exchange, NOMBRE_SERVICIO)
    servicio = VacacionesService(repo, broker, settings.zona_horaria, reloj)
    consumidor = ConsumidorEmpleados(repo)

    async def _arrancar_broker() -> None:
        # Segundo plano: la API responde aunque el broker aún no esté listo.
        try:
            await broker.conectar()
            if arrancar_consumidores:
                await broker.consumir(COLA, EVENTOS_CONSUMIDOS, consumidor.manejar)
        except asyncio.CancelledError:
            raise
        except Exception:  # noqa: BLE001
            log.exception("No se pudo inicializar el broker")

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        await pool.open()
        tarea = None
        if hasattr(broker, "conectar"):
            tarea = asyncio.create_task(_arrancar_broker())
        log.info("vacaciones-service listo en el puerto %s", settings.port)
        try:
            yield
        finally:
            if tarea:
                tarea.cancel()
            if hasattr(broker, "cerrar"):
                await broker.cerrar()
            await pool.close()

    app = FastAPI(
        title="TalentFlow — Servicio de Vacaciones",
        version="1.0.0",
        description=(
            "Programa y consulta períodos de vacaciones. Publica `vacaciones.programadas` "
            "y mantiene una réplica local de empleados consumiendo `empleado.creado` y "
            "`empleado.retirado` (ver Catálogo de Eventos).\n\n"
            "Todos los errores de validación responden **400**."
        ),
        lifespan=lifespan,
        # Bajo /vacaciones/* a propósito: así Swagger es alcanzable a través del Gateway
        # sin agregarle rutas nuevas (solo enruta el prefijo /vacaciones).
        docs_url="/vacaciones/docs",
        openapi_url="/vacaciones/openapi.json",
        redoc_url=None,
    )
    app.state.servicio = servicio
    app.state.repo = repo
    app.state.broker = broker
    _instalar_manejadores_error(app)

    errores_doc = {400: {"model": ErrorRespuesta}, 404: {"model": ErrorRespuesta}}

    @app.get("/health", include_in_schema=False)
    async def health():
        try:
            await repo.ping()
            db = "UP"
        except Exception:  # noqa: BLE001
            db = "DOWN"
        status = "UP" if db == "UP" else "DOWN"
        return JSONResponse(
            status_code=200 if status == "UP" else 503,
            content={
                "status": status,
                "service": NOMBRE_SERVICIO,
                "timestamp": datetime.now(timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z"),
                "components": {
                    "app": "UP",
                    "db": db,
                    "broker": "UP" if broker.conectado() else "DOWN",
                },
            },
        )

    @app.post(
        "/vacaciones",
        status_code=201,
        response_model=PeriodoVacaciones,
        tags=["Vacaciones"],
        summary="Programa un período de vacaciones",
        description=(
            "Rechaza con **400** si: (1) `fechaFin` no es posterior a `fechaInicio`; "
            "(2) `fechaInicio` es anterior a hoy; (3) el empleado ya tiene un período "
            "PROGRAMADA o EN_CURSO que se cruza (la respuesta incluye `periodoEnConflicto`); "
            "(4) el `empleadoId` no corresponde a un empleado registrado (o está RETIRADO). "
            "Si todo es válido, publica `vacaciones.programadas`."
        ),
        responses={400: {"model": ErrorRespuesta}},
    )
    async def programar(cuerpo: ProgramarVacaciones, response: Response):
        periodo = await servicio.programar(cuerpo.empleadoId.strip(), cuerpo.fechaInicio, cuerpo.fechaFin)
        response.headers["Location"] = f"/vacaciones/{periodo['id']}"
        return periodo

    @app.get(
        "/vacaciones",
        response_model=list[PeriodoVacaciones],
        tags=["Vacaciones"],
        summary="Lista los períodos (todos, o los de un empleado con ?empleadoId=)",
    )
    async def listar(empleadoId: Optional[str] = Query(default=None, examples=["E001"])):
        return await servicio.listar(empleadoId)

    @app.get(
        "/vacaciones/{id}",
        response_model=PeriodoVacaciones,
        tags=["Vacaciones"],
        summary="Consulta un período por su identificador",
        responses={404: {"model": ErrorRespuesta}},
    )
    async def obtener(id: str):
        return await servicio.obtener(id)

    @app.delete(
        "/vacaciones/{id}",
        response_model=PeriodoVacaciones,
        tags=["Vacaciones"],
        summary="Cancela un período que aún no ha iniciado",
        description=(
            "Pasa el período a CANCELADA (no se borra). Responde **409** si el período no está "
            "PROGRAMADA o si su `fechaInicio` ya llegó."
        ),
        responses={404: {"model": ErrorRespuesta}, 409: {"model": ErrorRespuesta}},
    )
    async def cancelar(id: str):
        return await servicio.cancelar(id)

    _limpiar_openapi(app)
    return app


def _limpiar_openapi(app: FastAPI) -> None:
    """Quita del OpenAPI los 422 automáticos de FastAPI: este servicio responde 400."""
    original = app.openapi

    def openapi_personalizado():
        if app.openapi_schema:
            return app.openapi_schema
        esquema = original()
        for ruta in esquema.get("paths", {}).values():
            for operacion in ruta.values():
                operacion.get("responses", {}).pop("422", None)
        componentes = esquema.get("components", {}).get("schemas", {})
        componentes.pop("HTTPValidationError", None)
        componentes.pop("ValidationError", None)
        return esquema

    app.openapi = openapi_personalizado
