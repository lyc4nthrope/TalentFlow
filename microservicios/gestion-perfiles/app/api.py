"""API REST de perfiles. Construida con crear_app() para poder probarla sin BD ni broker."""

import asyncio
import logging
from collections.abc import Callable
from datetime import UTC, datetime

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException

from app import dominio
from app.aplicacion import RepositorioPerfiles

logger = logging.getLogger("perfiles.api")

TIMEOUT_PING_BD_S = 2

# Mensajes en español para los errores de validación más comunes de Pydantic.
_MENSAJES_VALIDACION = {
    "extra_forbidden": "No es un campo editable del perfil",
    "string_too_long": "Supera la longitud máxima permitida",
    "string_pattern_mismatch": "Formato no válido",
    "string_type": "Debe ser un texto",
    "json_invalid": "Cuerpo JSON inválido",
}


def _cuerpo_error(status: int, error: str, mensaje: str, ruta: str, errores: list | None = None) -> dict:
    """Mismo formato de error que el resto del ecosistema."""
    cuerpo = {
        "status": status,
        "error": error,
        "message": mensaje,
        "timestamp": datetime.now(UTC).isoformat(timespec="seconds").replace("+00:00", "Z"),
        "path": ruta,
    }
    if errores:
        cuerpo["errors"] = errores
    return cuerpo


def crear_app(
    repositorio: RepositorioPerfiles,
    estado_broker: Callable[[], str],
    lifespan=None,
) -> FastAPI:
    app = FastAPI(
        title="TalentFlow - Servicio de Gestión de Perfiles",
        version="1.0.0",
        description=(
            "Combina comunicación asincrónica y sincrónica: consume empleado.creado, "
            "empleado.actualizado y empleado.retirado para crear, sincronizar y archivar "
            "perfiles, y expone REST para consultarlos y actualizarlos. Descarta eventos "
            "duplicados por el id del mensaje."
        ),
        # Documentación bajo el prefijo del servicio: el Gateway solo enruta /perfiles/*.
        docs_url="/perfiles/docs",
        openapi_url="/perfiles/openapi.json",
        redoc_url=None,
        lifespan=lifespan,
    )

    @app.exception_handler(RequestValidationError)
    async def validacion(request: Request, error: RequestValidationError) -> JSONResponse:
        errores = [
            {
                "field": ".".join(str(parte) for parte in e["loc"] if parte != "body") or "body",
                "message": _MENSAJES_VALIDACION.get(e["type"], e["msg"]),
            }
            for e in error.errors()
        ]
        return JSONResponse(
            status_code=400,
            content=_cuerpo_error(400, "Bad Request", "La petición no es válida", request.url.path, errores),
        )

    @app.exception_handler(dominio.PerfilNoEncontrado)
    async def no_encontrado(request: Request, error: dominio.PerfilNoEncontrado) -> JSONResponse:
        return JSONResponse(
            status_code=404, content=_cuerpo_error(404, "Not Found", str(error), request.url.path)
        )

    @app.exception_handler(dominio.PerfilArchivado)
    async def archivado(request: Request, error: dominio.PerfilArchivado) -> JSONResponse:
        return JSONResponse(
            status_code=409, content=_cuerpo_error(409, "Conflict", str(error), request.url.path)
        )

    @app.exception_handler(StarletteHTTPException)
    async def http(request: Request, error: StarletteHTTPException) -> JSONResponse:
        mensaje = "Recurso no encontrado" if error.status_code == 404 else str(error.detail)
        frase = "Not Found" if error.status_code == 404 else "Error"
        return JSONResponse(
            status_code=error.status_code,
            content=_cuerpo_error(error.status_code, frase, mensaje, request.url.path),
        )

    @app.exception_handler(Exception)
    async def inesperado(request: Request, error: Exception) -> JSONResponse:
        # El detalle queda en el log; al cliente no se le filtran errores internos.
        logger.exception("error no controlado en %s", request.url.path)
        return JSONResponse(
            status_code=500,
            content=_cuerpo_error(
                500, "Internal Server Error", "Error interno del servidor", request.url.path
            ),
        )

    @app.get("/perfiles", response_model=list[dominio.Perfil], summary="Lista todos los perfiles")
    async def listar() -> list[dominio.Perfil]:
        return await repositorio.listar()

    @app.get(
        "/perfiles/{empleadoId}",
        response_model=dominio.Perfil,
        summary="Consulta el perfil de un empleado",
        responses={404: {"description": "El perfil no existe"}},
    )
    async def obtener(empleadoId: str) -> dominio.Perfil:
        perfil = await repositorio.obtener(empleadoId)
        if perfil is None:
            raise dominio.PerfilNoEncontrado(empleadoId)
        return perfil

    @app.put(
        "/perfiles/{empleadoId}",
        response_model=dominio.Perfil,
        summary="Actualiza el perfil (teléfono, dirección, ciudad, biografía)",
        description=(
            "Actualización parcial: solo cambian los campos enviados. nombre y email no se "
            "editan aquí: se sincronizan desde empleados-service (empleado.actualizado)."
        ),
        responses={
            400: {"description": "Campo no editable, formato inválido o cuerpo vacío"},
            404: {"description": "El perfil no existe"},
            409: {"description": "El perfil está archivado (empleado retirado)"},
        },
    )
    async def actualizar(empleadoId: str, cambios: dominio.ActualizacionPerfil) -> dominio.Perfil:
        return await repositorio.actualizar(empleadoId, cambios.cambios())

    @app.get("/health", include_in_schema=False)
    async def salud() -> JSONResponse:
        # DOWN solo si la BD propia no responde; sin broker el servicio sigue sirviendo
        # los perfiles (degradado) y el broker se reporta como componente.
        try:
            await asyncio.wait_for(repositorio.ping(), timeout=TIMEOUT_PING_BD_S)
            db = "UP"
        except Exception:  # noqa: BLE001
            db = "DOWN"
        status = "UP" if db == "UP" else "DOWN"
        return JSONResponse(
            status_code=200 if status == "UP" else 503,
            content={
                "status": status,
                "service": "perfiles-service",
                "timestamp": datetime.now(UTC).isoformat(timespec="seconds").replace("+00:00", "Z"),
                "components": {"app": "UP", "db": db, "broker": estado_broker()},
            },
        )

    return app
