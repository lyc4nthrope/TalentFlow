"""Envelope común a todos los eventos (Catálogo de Eventos, sección 2)."""

from datetime import datetime
from typing import Any

from pydantic import BaseModel, Field, ValidationError

# Coincide con la columna eventos_procesados.id. Sin límite, un id más largo haría
# fallar el INSERT en cada intento y el mensaje se reintentaría para siempre.
LONGITUD_MAXIMA_ID = 100


class MensajeInvalido(Exception):
    """El mensaje nunca podrá procesarse (JSON corrupto o envelope incompleto)."""


class Envelope(BaseModel):
    id: str = Field(min_length=1, max_length=LONGITUD_MAXIMA_ID)
    type: str = Field(min_length=1)
    version: int = Field(ge=1)
    occurredAt: datetime
    producer: str = Field(min_length=1)
    data: dict[str, Any]


def parsear(cuerpo: bytes) -> Envelope:
    try:
        return Envelope.model_validate_json(cuerpo)
    except ValidationError as error:
        raise MensajeInvalido(f"envelope no válido: {error.errors(include_url=False)}") from error
