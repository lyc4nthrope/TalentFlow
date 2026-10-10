"""Configuración del servicio. Todo viene de variables de entorno (nada en el código)."""
import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    port: int
    db_host: str
    db_port: int
    db_name: str
    db_user: str
    db_pass: str
    broker_url: str
    broker_exchange: str
    zona_horaria: str
    dsn_url: str = ""  # opcional: URI completa (la usan las pruebas)

    @property
    def dsn(self) -> str:
        if self.dsn_url:
            return self.dsn_url
        return (
            f"host={self.db_host} port={self.db_port} dbname={self.db_name} "
            f"user={self.db_user} password={self.db_pass}"
        )

    @staticmethod
    def desde_entorno() -> "Settings":
        return Settings(
            port=int(os.getenv("PORT", "8085")),
            db_host=os.getenv("DB_HOST", "localhost"),
            db_port=int(os.getenv("DB_PORT", "5432")),
            db_name=os.getenv("DB_NAME", "db_vacaciones"),
            db_user=os.getenv("DB_USER", "user_vacaciones"),
            db_pass=os.getenv("DB_PASS", "pass_vacaciones"),
            broker_url=os.getenv("BROKER_URL", "amqp://talentflow:talentflow_dev@message-broker:5672/"),
            broker_exchange=os.getenv("BROKER_EXCHANGE", "talentflow.events"),
            # Zona usada para decidir qué es "hoy" al validar fechas en el pasado.
            zona_horaria=os.getenv("TZ_NEGOCIO", "America/Bogota"),
        )
