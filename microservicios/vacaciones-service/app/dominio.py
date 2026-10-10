"""Reglas de dominio puras (sin I/O): fechas y formato de salida."""
from datetime import date, datetime, timedelta, timezone
from typing import Any


def dias_habiles(inicio: date, fin: date) -> int:
    """Días de lunes a viernes entre inicio y fin, ambos inclusive.

    No descuenta festivos: el calendario laboral no es parte del alcance del reto.
    """
    total = 0
    dia = inicio
    while dia <= fin:
        if dia.weekday() < 5:
            total += 1
        dia += timedelta(days=1)
    return total


def iso_utc(valor: datetime) -> str:
    """ISO-8601 en UTC con sufijo Z y sin fracciones, como en el catálogo de eventos."""
    return valor.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def periodo_a_dto(fila: dict[str, Any]) -> dict[str, Any]:
    return {
        "id": fila["id"],
        "empleadoId": fila["empleado_id"],
        "fechaInicio": fila["fecha_inicio"].isoformat(),
        "fechaFin": fila["fecha_fin"].isoformat(),
        "estado": fila["estado"],
        "fechaCreacion": iso_utc(fila["fecha_creacion"]),
    }
