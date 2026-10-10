from typing import Any, Optional


class AppError(Exception):
    """Error de negocio con código HTTP. `extra` se agrega al cuerpo de la respuesta."""

    def __init__(
        self,
        mensaje: str,
        codigo_estado: int = 400,
        errores: Optional[list[dict[str, str]]] = None,
        extra: Optional[dict[str, Any]] = None,
    ):
        super().__init__(mensaje)
        self.mensaje = mensaje
        self.codigo_estado = codigo_estado
        self.errores = errores or []
        self.extra = extra or {}


class EventoInvalido(Exception):
    """El mensaje del broker no cumple el envelope del catálogo (no tiene sentido reintentarlo)."""
