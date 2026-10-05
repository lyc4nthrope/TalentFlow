"""Configuración leída de variables de entorno (nunca credenciales en el código)."""

import os
from dataclasses import dataclass
from urllib.parse import quote


def _env(clave: str, por_defecto: str) -> str:
    valor = os.environ.get(clave, "")
    return valor if valor else por_defecto


@dataclass(frozen=True)
class Config:
    puerto: int
    url_postgres: str
    url_rabbitmq: str
    cola: str

    @classmethod
    def desde_entorno(cls) -> "Config":
        # quote(): usuario y contraseña quedan codificados aunque tengan @, : o /.
        usuario_bd = quote(_env("DB_USER", "postgres"), safe="")
        clave_bd = quote(_env("DB_PASS", ""), safe="")
        usuario_mq = quote(_env("RABBITMQ_USER", "guest"), safe="")
        clave_mq = quote(_env("RABBITMQ_PASS", "guest"), safe="")
        return cls(
            puerto=int(_env("PORT", "8083")),
            url_postgres=(
                f"postgresql://{usuario_bd}:{clave_bd}@{_env('DB_HOST', 'localhost')}:"
                f"{_env('DB_PORT', '5432')}/{_env('DB_NAME', 'db_perfiles')}"
            ),
            url_rabbitmq=(
                f"amqp://{usuario_mq}:{clave_mq}@{_env('RABBITMQ_HOST', 'localhost')}:"
                f"{_env('RABBITMQ_PORT', '5672')}/"
            ),
            cola=_env("RABBITMQ_COLA", "perfiles.eventos"),
        )
