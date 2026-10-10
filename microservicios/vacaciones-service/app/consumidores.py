"""Consumo de eventos de empleados para mantener la réplica local (opción b del reto).

La deduplicación y el efecto ocurren en UNA sola transacción: si el efecto falla, el
registro del id se deshace y la reentrega del broker vuelve a intentarlo; si el mensaje
llega duplicado, el INSERT en eventos_procesados no inserta nada y se descarta.
"""
import logging

from .errores import EventoInvalido
from .eventos import parsear_envelope
from .repository import VacacionesRepository

log = logging.getLogger("vacaciones.consumidor")

EVENTOS_CONSUMIDOS = ["empleado.creado", "empleado.retirado"]
COLA = "vacaciones-service.q"


def _texto(data: dict, campo: str) -> str:
    valor = data.get(campo)
    if not isinstance(valor, str) or not valor.strip():
        raise EventoInvalido(f"data.{campo} es obligatorio")
    return valor.strip()


class ConsumidorEmpleados:
    def __init__(self, repo: VacacionesRepository):
        self.repo = repo

    async def manejar(self, cuerpo: bytes) -> str:
        evento = parsear_envelope(cuerpo)
        tipo = evento["type"]
        data = evento["data"]

        if tipo not in EVENTOS_CONSUMIDOS:
            return "ignorado"

        # Validar ANTES de abrir la transacción: un mensaje malformado no debe consumir el id.
        empleado_id = _texto(data, "empleadoId")
        email = _texto(data, "email")

        async with self.repo.transaccion() as conn:
            if not await self.repo.registrar_evento(conn, evento["id"]):
                log.info("Evento duplicado descartado: %s id=%s", tipo, evento["id"])
                return "duplicado"
            if tipo == "empleado.creado":
                await self.repo.registrar_empleado(conn, empleado_id, email)
            else:
                await self.repo.retirar_empleado(conn, empleado_id, email)
        return "procesado"
