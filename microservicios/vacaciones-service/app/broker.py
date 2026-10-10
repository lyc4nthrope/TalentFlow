"""Acceso al message broker (RabbitMQ).

Topología (compartida por todo el ecosistema, ver docs/eventos.md):
  - Exchange `talentflow.events`, tipo topic, durable.
  - Routing key = nombre del evento (p. ej. `vacaciones.programadas`).
  - Cada servicio consumidor tiene su propia cola durable enlazada a los eventos que le interesan.
"""
import asyncio
import json
import logging
from typing import Any, Awaitable, Callable, Optional, Protocol

import aio_pika
from aio_pika import DeliveryMode, ExchangeType
from aio_pika.abc import AbstractIncomingMessage

from .errores import EventoInvalido
from .eventos import construir_evento

log = logging.getLogger("vacaciones.broker")


class BrokerNoDisponible(Exception):
    pass


class Broker(Protocol):
    async def publicar(self, tipo: str, data: dict[str, Any]) -> dict[str, Any]: ...
    def conectado(self) -> bool: ...


Manejador = Callable[[bytes], Awaitable[str]]


class RabbitBroker:
    def __init__(self, url: str, exchange: str, productor: str):
        self.url = url
        self.exchange_nombre = exchange
        self.productor = productor
        self._conexion: Optional[aio_pika.abc.AbstractRobustConnection] = None
        self._canal = None
        self._exchange = None

    def conectado(self) -> bool:
        return self._conexion is not None and not self._conexion.is_closed and self._exchange is not None

    async def conectar(self, espera_s: float = 3.0) -> None:
        """Reintenta hasta lograr conexión: el broker puede tardar más que este servicio en arrancar."""
        while True:
            try:
                self._conexion = await aio_pika.connect_robust(self.url)
                break
            except Exception as exc:  # noqa: BLE001 - cualquier fallo de red/credenciales
                log.warning("Broker no disponible (%s). Reintentando en %.0fs", exc, espera_s)
                await asyncio.sleep(espera_s)
        self._canal = await self._conexion.channel()
        self._exchange = await self._canal.declare_exchange(
            self.exchange_nombre, ExchangeType.TOPIC, durable=True
        )
        log.info("Conectado al broker, exchange '%s'", self.exchange_nombre)

    async def publicar(self, tipo: str, data: dict[str, Any]) -> dict[str, Any]:
        if not self.conectado():
            raise BrokerNoDisponible("no hay conexión con el broker")
        evento = construir_evento(tipo, data, self.productor)
        mensaje = aio_pika.Message(
            body=json.dumps(evento, ensure_ascii=False).encode("utf-8"),
            content_type="application/json",
            delivery_mode=DeliveryMode.PERSISTENT,
            message_id=evento["id"],
            type=tipo,
        )
        # El canal usa publisher confirms: esta llamada espera la confirmación del broker.
        await self._exchange.publish(mensaje, routing_key=tipo)
        log.info("Evento publicado: %s id=%s", tipo, evento["id"])
        return evento

    async def consumir(self, cola: str, eventos: list[str], manejador: Manejador) -> None:
        await self._canal.set_qos(prefetch_count=10)
        queue = await self._canal.declare_queue(cola, durable=True)
        for clave in eventos:
            await queue.bind(self._exchange, routing_key=clave)
        await queue.consume(_envolver(manejador))
        log.info("Consumiendo cola '%s' (%s)", cola, ", ".join(eventos))

    async def cerrar(self) -> None:
        if self._conexion is not None and not self._conexion.is_closed:
            await self._conexion.close()


def _envolver(manejador: Manejador):
    """Convierte un manejador de bytes en un callback de aio-pika con la política de ack:

    - procesado / duplicado / ignorado -> ack
    - mensaje inválido (no tiene arreglo reintentando) -> reject sin reencolar
    - fallo transitorio (p. ej. BD caída) -> nack con reencolado tras una pausa
    """

    async def callback(mensaje: AbstractIncomingMessage) -> None:
        try:
            resultado = await manejador(mensaje.body)
            await mensaje.ack()
            log.info("Mensaje %s -> %s", mensaje.message_id, resultado)
        except EventoInvalido as exc:
            log.error("Mensaje descartado por inválido: %s", exc)
            await mensaje.reject(requeue=False)
        except Exception:  # noqa: BLE001
            log.exception("Fallo procesando el mensaje; se reencola")
            await asyncio.sleep(2)
            await mensaje.nack(requeue=True)

    return callback
