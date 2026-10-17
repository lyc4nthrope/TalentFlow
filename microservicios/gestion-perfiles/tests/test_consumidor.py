"""Estado del vínculo con el broker que reporta /health."""

import asyncio
from types import SimpleNamespace

from app.consumidor import Consumidor


def consumidor_con(conexion) -> Consumidor:
    consumidor = Consumidor("amqp://prueba", "perfiles.eventos", procesador=None)
    consumidor._conexion = conexion
    return consumidor


def test_sin_conexion_es_down():
    assert consumidor_con(None).estado() == "DOWN"


def test_refleja_la_perdida_y_la_recuperacion_de_la_conexion():
    conectado = asyncio.Event()
    consumidor = consumidor_con(SimpleNamespace(connected=conectado, is_closed=False))

    assert consumidor.estado() == "DOWN", "reconectando: no está cerrada, pero tampoco conectada"
    conectado.set()
    assert consumidor.estado() == "UP"
