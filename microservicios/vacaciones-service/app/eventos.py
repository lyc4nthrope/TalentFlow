"""Envelope común del Catálogo de Eventos (sección 2): id, type, version, occurredAt, producer, data."""
import json
import uuid
from datetime import datetime, timezone
from typing import Any

from .errores import EventoInvalido

CAMPOS_ENVELOPE = ("id", "type", "version", "occurredAt", "producer", "data")


def construir_evento(tipo: str, data: dict[str, Any], productor: str, version: int = 1) -> dict[str, Any]:
    return {
        "id": str(uuid.uuid4()),
        "type": tipo,
        "version": version,
        "occurredAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "producer": productor,
        "data": data,
    }


def parsear_envelope(cuerpo: bytes) -> dict[str, Any]:
    try:
        evento = json.loads(cuerpo)
    except (ValueError, UnicodeDecodeError) as exc:
        raise EventoInvalido(f"el mensaje no es JSON válido: {exc}") from exc
    if not isinstance(evento, dict):
        raise EventoInvalido("el mensaje no es un objeto JSON")
    faltan = [c for c in CAMPOS_ENVELOPE if c not in evento]
    if faltan:
        raise EventoInvalido(f"faltan campos del envelope: {', '.join(faltan)}")
    if not isinstance(evento["id"], str) or not evento["id"].strip():
        raise EventoInvalido("el campo id del envelope debe ser un texto no vacío")
    if not isinstance(evento["data"], dict):
        raise EventoInvalido("el campo data debe ser un objeto")
    return evento
