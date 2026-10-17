"""Procesamiento de eventos: efectos, deduplicación y clasificación de fallos."""

import asyncio
import json

import pytest

from app.aplicacion import ProcesadorEventos, RechazoPermanente, Resultado
from tests.fakes import RepositorioEnMemoria


def evento(tipo: str, data: dict, evento_id: str = "evt-1") -> bytes:
    return json.dumps(
        {
            "id": evento_id,
            "type": tipo,
            "version": 1,
            "occurredAt": "2026-09-28T14:00:00Z",
            "producer": "empleados-service",
            "data": data,
        }
    ).encode()


CREADO = {
    "empleadoId": "E001",
    "nombre": "Juan",
    "apellido": "Pérez",
    "email": "juan.perez@empresa.com",
    "numeroEmpleado": "EMP-2026-001",
    "cargo": "Desarrollador Senior",
    "area": "Tecnología",
    "departamentoId": "IT",
    "fechaIngreso": "2026-03-01",
    "estado": "ACTIVO",
}
ACTUALIZADO = {
    "empleadoId": "E001",
    "nombre": "Juan",
    "apellido": "Pérez Gómez",
    "email": "juan.pg@empresa.com",
    "cargo": "Tech Lead",
    "area": "Tecnología",
    "departamentoId": "IT",
}
RETIRADO = {
    "empleadoId": "E001",
    "email": "juan.perez@empresa.com",
    "fechaRetiro": "2026-11-30T16:45:00Z",
    "motivo": "RENUNCIA",
}


@pytest.fixture
def repo():
    return RepositorioEnMemoria()


def procesar(repo, cuerpo: bytes) -> Resultado:
    return asyncio.run(ProcesadorEventos(repo).procesar(cuerpo))


def test_empleado_creado_crea_el_perfil_por_defecto(repo):
    assert procesar(repo, evento("empleado.creado", CREADO)) is Resultado.PROCESADO

    perfil = repo.perfiles["E001"]
    assert perfil.nombre == "Juan Pérez"
    assert perfil.email == "juan.perez@empresa.com"
    assert (perfil.telefono, perfil.direccion, perfil.ciudad, perfil.biografia) == ("", "", "", "")
    assert perfil.archivado is False


def test_empleado_actualizado_sincroniza_nombre_y_email_sin_tocar_lo_demas(repo):
    procesar(repo, evento("empleado.creado", CREADO, "e1"))
    asyncio.run(repo.actualizar("E001", {"telefono": "3001234567"}))

    procesar(repo, evento("empleado.actualizado", ACTUALIZADO, "e2"))

    perfil = repo.perfiles["E001"]
    assert perfil.nombre == "Juan Pérez Gómez"
    assert perfil.email == "juan.pg@empresa.com"
    assert perfil.telefono == "3001234567", "los datos propios del perfil no se pisan"


def test_empleado_actualizado_sin_perfil_lo_crea(repo):
    """Recupera un empleado.creado perdido (broker caído al publicarlo)."""
    assert procesar(repo, evento("empleado.actualizado", ACTUALIZADO)) is Resultado.PROCESADO
    assert repo.perfiles["E001"].nombre == "Juan Pérez Gómez"


def test_empleado_retirado_archiva_sin_borrar(repo):
    procesar(repo, evento("empleado.creado", CREADO, "e1"))

    procesar(repo, evento("empleado.retirado", RETIRADO, "e2"))

    perfil = repo.perfiles["E001"]
    assert perfil.archivado is True
    assert perfil.fechaArchivado is not None


def test_el_mismo_evento_dos_veces_tiene_un_solo_efecto(repo):
    assert procesar(repo, evento("empleado.creado", CREADO, "mismo-id")) is Resultado.PROCESADO
    repo.perfiles["E001"] = repo.perfiles["E001"].model_copy(update={"telefono": "123"})

    assert procesar(repo, evento("empleado.creado", CREADO, "mismo-id")) is Resultado.DUPLICADO
    assert len(repo.perfiles) == 1
    assert repo.perfiles["E001"].telefono == "123", "el duplicado no recrea ni pisa el perfil"


@pytest.mark.parametrize(
    "cuerpo",
    [
        b"{no es json",
        evento("vacaciones.programadas", {"empleadoId": "E001"}),  # tipo que no consume
        evento("empleado.creado", {"empleadoId": "E001"}),  # carga incompleta
        json.dumps(
            {
                "id": "x" * 101,
                "type": "empleado.creado",
                "version": 1,
                "occurredAt": "2026-09-28T14:00:00Z",
                "producer": "p",
                "data": CREADO,
            }
        ).encode(),  # id más largo que la columna
    ],
)
def test_descarta_mensajes_que_nunca_podran_procesarse(repo, cuerpo):
    assert procesar(repo, cuerpo) is Resultado.DESCARTADO
    assert repo.perfiles == {}


def test_error_transitorio_de_la_bd_se_reintenta(repo):
    repo.fallo = ConnectionError("la BD no responde")
    assert procesar(repo, evento("empleado.creado", CREADO)) is Resultado.ERROR_TRANSITORIO


def test_rechazo_permanente_de_la_bd_se_descarta(repo):
    repo.fallo = RechazoPermanente("value too long")
    assert procesar(repo, evento("empleado.creado", CREADO)) is Resultado.DESCARTADO
