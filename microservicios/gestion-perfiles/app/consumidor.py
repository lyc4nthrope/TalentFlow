"""Consumidor de la cola perfiles.eventos en RabbitMQ (aio-pika)."""

import asyncio
import logging

import aio_pika
from aio_pika.abc import AbstractIncomingMessage, AbstractRobustConnection

from app.aplicacion import ProcesadorEventos, Resultado

logger = logging.getLogger("perfiles.consumidor")

PREFETCH = 10  # mensajes entregados sin confirmar a la vez (no cargar toda la cola en memoria)
ESPERA_REINTENTO_S = 5  # pausa antes de reencolar un error transitorio (evita el bucle cerrado)
ESPERA_MAXIMA_CONEXION_S = 30


class Consumidor:
    def __init__(self, url: str, cola: str, procesador: ProcesadorEventos):
        self._url = url
        self._cola = cola
        self._procesador = procesador
        self._conexion: AbstractRobustConnection | None = None

    def estado(self) -> str:
        conectado = self._conexion is not None and not self._conexion.is_closed
        return "UP" if conectado else "DOWN"

    async def ejecutar(self) -> None:
        """Conecta (reintentando con espera exponencial si el broker aún no está) y
        consume hasta que se cancele la tarea. Tras la primera conexión, connect_robust
        reconecta y restablece el consumo por su cuenta si la conexión se pierde."""
        espera = 1
        while self._conexion is None:
            try:
                self._conexion = await aio_pika.connect_robust(self._url)
            except Exception as error:  # noqa: BLE001
                logger.warning("broker no disponible (%s); reintento en %ss", error, espera)
                await asyncio.sleep(espera)
                espera = min(espera * 2, ESPERA_MAXIMA_CONEXION_S)

        canal = await self._conexion.channel()
        await canal.set_qos(prefetch_count=PREFETCH)
        # get_queue verifica que la cola exista (la crea broker-init) sin redeclararla.
        cola = await canal.get_queue(self._cola, ensure=True)
        # no_ack=False: el mensaje sale de la cola solo cuando lo confirmamos, es decir,
        # cuando su efecto ya quedó en la BD. Si el proceso muere antes, se reentrega.
        await cola.consume(self._manejar, no_ack=False)
        logger.info("consumiendo eventos de la cola %s", self._cola)
        await asyncio.Future()  # mantiene la tarea viva hasta que se cancele

    async def _manejar(self, mensaje: AbstractIncomingMessage) -> None:
        resultado = await self._procesador.procesar(mensaje.body)
        if resultado is Resultado.ERROR_TRANSITORIO:
            await asyncio.sleep(ESPERA_REINTENTO_S)
            await mensaje.nack(requeue=True)
        else:
            await mensaje.ack()

    async def cerrar(self) -> None:
        if self._conexion is not None:
            await self._conexion.close()
