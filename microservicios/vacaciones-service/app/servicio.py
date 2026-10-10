"""Lógica de negocio de vacaciones: validaciones, persistencia y publicación del evento."""
import logging
from datetime import date, datetime
from typing import Any, Callable, Optional
from zoneinfo import ZoneInfo

from .broker import Broker
from .dominio import dias_habiles, periodo_a_dto
from .errores import AppError
from .repository import VacacionesRepository

log = logging.getLogger("vacaciones.servicio")


class VacacionesService:
    def __init__(
        self,
        repo: VacacionesRepository,
        broker: Broker,
        zona_horaria: str = "America/Bogota",
        reloj: Optional[Callable[[], datetime]] = None,
    ):
        self.repo = repo
        self.broker = broker
        self.zona = ZoneInfo(zona_horaria)
        self._reloj = reloj

    def hoy(self) -> date:
        ahora = self._reloj() if self._reloj else datetime.now(self.zona)
        return ahora.date()

    async def programar(self, empleado_id: str, inicio: date, fin: date) -> dict[str, Any]:
        hoy = self.hoy()

        # 1. Fechas incoherentes
        if fin <= inicio:
            raise AppError(
                f"fechaFin ({fin}) debe ser posterior a fechaInicio ({inicio})",
                400,
                [{"campo": "fechaFin", "mensaje": "debe ser posterior a fechaInicio"}],
            )
        # 2. Fechas en el pasado
        if inicio < hoy:
            raise AppError(
                f"fechaInicio ({inicio}) no puede ser anterior a la fecha actual ({hoy})",
                400,
                [{"campo": "fechaInicio", "mensaje": "no puede estar en el pasado"}],
            )

        async with self.repo.transaccion() as conn:
            # Serializa programaciones concurrentes del mismo empleado hasta el commit.
            await self.repo.bloquear_empleado(conn, empleado_id)

            # 4. Empleado inexistente (según la réplica local alimentada por eventos)
            empleado = await self.repo.obtener_empleado(conn, empleado_id)
            if empleado is None:
                raise AppError(
                    f"El empleado {empleado_id} no existe",
                    400,
                    [{"campo": "empleadoId", "mensaje": "no corresponde a un empleado registrado"}],
                )
            if empleado["estado"] == "RETIRADO":
                raise AppError(
                    f"El empleado {empleado_id} está RETIRADO y no puede programar vacaciones",
                    400,
                    [{"campo": "empleadoId", "mensaje": "el empleado está retirado"}],
                )

            # 3. Solapamiento con un período PROGRAMADA o EN_CURSO
            conflicto = await self.repo.buscar_solapado(conn, empleado_id, inicio, fin)
            if conflicto is not None:
                raise AppError(
                    f"El rango {inicio} a {fin} se cruza con el período {conflicto['id']} "
                    f"({conflicto['fecha_inicio']} a {conflicto['fecha_fin']}, {conflicto['estado']})",
                    400,
                    [{"campo": "fechaInicio", "mensaje": "se cruza con otro período del empleado"}],
                    extra={"periodoEnConflicto": periodo_a_dto(conflicto)},
                )

            nuevo_id = await self.repo.siguiente_id(conn, hoy.year)
            fila = await self.repo.insertar(conn, nuevo_id, empleado_id, inicio, fin)
            email = empleado["email"]

        # La transacción ya confirmó. Publicar DESPUÉS: si falla, se registra el error
        # pero NO se revierte el período (misma regla que el servicio de empleados).
        await self._publicar_programadas(fila, email)
        return periodo_a_dto(fila)

    async def _publicar_programadas(self, fila: dict[str, Any], email: str) -> None:
        data = {
            "vacacionesId": fila["id"],
            "empleadoId": fila["empleado_id"],
            "email": email,
            "fechaInicio": fila["fecha_inicio"].isoformat(),
            "fechaFin": fila["fecha_fin"].isoformat(),
            "diasHabiles": dias_habiles(fila["fecha_inicio"], fila["fecha_fin"]),
        }
        try:
            await self.broker.publicar("vacaciones.programadas", data)
        except Exception:  # noqa: BLE001
            log.exception(
                "No se pudo publicar vacaciones.programadas del período %s; "
                "el período quedó registrado (no se revierte)",
                fila["id"],
            )

    async def obtener(self, id_: str) -> dict[str, Any]:
        async with self.repo.transaccion() as conn:
            fila = await self.repo.obtener(conn, id_)
        if fila is None:
            raise AppError(f"No existe el período de vacaciones {id_}", 404)
        return periodo_a_dto(fila)

    async def listar(self, empleado_id: Optional[str] = None) -> list[dict[str, Any]]:
        async with self.repo.transaccion() as conn:
            filas = await self.repo.listar(conn, empleado_id)
        return [periodo_a_dto(f) for f in filas]

    async def cancelar(self, id_: str) -> dict[str, Any]:
        hoy = self.hoy()
        async with self.repo.transaccion() as conn:
            fila = await self.repo.obtener(conn, id_, para_actualizar=True)
            if fila is None:
                raise AppError(f"No existe el período de vacaciones {id_}", 404)
            if fila["estado"] != "PROGRAMADA":
                raise AppError(
                    f"El período {id_} está {fila['estado']}: solo se puede cancelar uno PROGRAMADA",
                    409,
                )
            if fila["fecha_inicio"] <= hoy:
                raise AppError(
                    f"El período {id_} ya inició el {fila['fecha_inicio']}: no se puede cancelar",
                    409,
                )
            fila = await self.repo.marcar_cancelada(conn, id_)
        return periodo_a_dto(fila)
