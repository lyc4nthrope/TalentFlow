"""Caso de uso "procesar un evento": validar, deduplicar por id y aplicar el efecto."""

import logging
from enum import Enum
from typing import Protocol

from pydantic import BaseModel, ValidationError

from app import dominio
from app.eventos import MensajeInvalido, parsear

logger = logging.getLogger("perfiles.eventos")


class RechazoPermanente(Exception):
    """La BD rechazó los datos en sí (valor inválido, restricción violada).
    Reintentar no cambiaría nada: el mensaje se descarta (mensaje envenenado)."""


class RepositorioPerfiles(Protocol):
    # Cada aplicar_* registra el id del evento y aplica su efecto en UNA transacción.
    # Devuelven False si ese id ya se había procesado (duplicado): no hacen nada.
    async def aplicar_creado(self, evento_id: str, datos: dominio.EmpleadoCreado) -> bool: ...
    async def aplicar_actualizado(self, evento_id: str, datos: dominio.EmpleadoActualizado) -> bool: ...
    async def aplicar_retirado(self, evento_id: str, datos: dominio.EmpleadoRetirado) -> bool: ...

    async def obtener(self, empleado_id: str) -> dominio.Perfil | None: ...
    async def listar(self) -> list[dominio.Perfil]: ...
    async def actualizar(self, empleado_id: str, cambios: dict[str, str]) -> dominio.Perfil: ...
    async def ping(self) -> None: ...


class Resultado(Enum):
    PROCESADO = "procesado"  # efecto aplicado -> ack
    DUPLICADO = "duplicado"  # id ya procesado -> ack sin repetir el efecto
    DESCARTADO = "descartado"  # nunca podrá procesarse -> ack + log (DLQ: Reto 29)
    ERROR_TRANSITORIO = "error_transitorio"  # recuperable (BD caída) -> nack con reencolado


class ProcesadorEventos:
    def __init__(self, repositorio: RepositorioPerfiles):
        # tipo de evento -> (modelo de su carga útil, efecto en el repositorio)
        self._manejadores: dict[str, tuple[type[BaseModel], object]] = {
            dominio.EMPLEADO_CREADO: (dominio.EmpleadoCreado, repositorio.aplicar_creado),
            dominio.EMPLEADO_ACTUALIZADO: (dominio.EmpleadoActualizado, repositorio.aplicar_actualizado),
            dominio.EMPLEADO_RETIRADO: (dominio.EmpleadoRetirado, repositorio.aplicar_retirado),
        }

    async def procesar(self, cuerpo: bytes) -> Resultado:
        try:
            sobre = parsear(cuerpo)
        except MensajeInvalido as error:
            logger.error("mensaje descartado: %s", error)
            return Resultado.DESCARTADO

        contexto = f"eventoId={sobre.id} tipo={sobre.type}"
        manejador = self._manejadores.get(sobre.type)
        if manejador is None:
            logger.warning("evento ignorado (este servicio no lo consume) %s", contexto)
            return Resultado.DESCARTADO

        modelo, efecto = manejador
        try:
            datos = modelo.model_validate(sobre.data)
        except ValidationError as error:
            logger.error(
                "evento descartado: carga útil inválida %s %s", contexto, error.errors(include_url=False)
            )
            return Resultado.DESCARTADO

        try:
            nuevo = await efecto(sobre.id, datos)
        except RechazoPermanente as error:
            logger.error("evento descartado: la BD rechazó sus datos %s: %s", contexto, error)
            return Resultado.DESCARTADO
        except Exception as error:  # noqa: BLE001 — cualquier otro fallo se trata como transitorio
            logger.error("no se pudo aplicar el evento; se reintentará %s: %s", contexto, error)
            return Resultado.ERROR_TRANSITORIO

        if not nuevo:
            logger.info("evento duplicado descartado: ya se había procesado este id %s", contexto)
            return Resultado.DUPLICADO

        logger.info("evento aplicado al perfil %s empleadoId=%s", contexto, datos.empleadoId)
        return Resultado.PROCESADO
