"""API REST de perfiles (FastAPI TestClient con repositorio en memoria)."""

import asyncio
from datetime import UTC, datetime

import pytest
from fastapi.testclient import TestClient

from app import dominio
from app.api import crear_app
from tests.fakes import RepositorioEnMemoria


@pytest.fixture
def repo():
    repositorio = RepositorioEnMemoria()
    datos = dominio.EmpleadoCreado(
        empleadoId="E001", nombre="Juan", apellido="Pérez", email="juan@empresa.com"
    )
    asyncio.run(repositorio.aplicar_creado("e1", datos))
    return repositorio


@pytest.fixture
def cliente(repo):
    return TestClient(crear_app(repo, lambda: "UP"))


def test_get_perfil_existente(cliente):
    respuesta = cliente.get("/perfiles/E001")
    assert respuesta.status_code == 200
    assert respuesta.json()["nombre"] == "Juan Pérez"


def test_get_perfil_inexistente_404_con_mensaje_descriptivo(cliente):
    respuesta = cliente.get("/perfiles/E999")
    assert respuesta.status_code == 404
    cuerpo = respuesta.json()
    assert cuerpo["status"] == 404
    assert "E999" in cuerpo["message"]


def test_listar_perfiles(cliente):
    respuesta = cliente.get("/perfiles")
    assert respuesta.status_code == 200
    assert [p["empleadoId"] for p in respuesta.json()] == ["E001"]


def test_put_parcial_como_el_del_reto(cliente):
    # Cuerpo exacto del paso 6 de la sección "Pruebas del Sistema" del reto4.pdf.
    respuesta = cliente.put(
        "/perfiles/E001",
        json={"telefono": "3001234567", "ciudad": "Armenia", "biografia": "Ingeniero de sistemas"},
    )
    assert respuesta.status_code == 200
    perfil = respuesta.json()
    assert (perfil["telefono"], perfil["ciudad"], perfil["biografia"]) == (
        "3001234567",
        "Armenia",
        "Ingeniero de sistemas",
    )
    assert perfil["direccion"] == "", "lo no enviado no cambia"


@pytest.mark.parametrize(
    ("cuerpo", "campo"),
    [
        ({"nombre": "Otro"}, "nombre"),  # replicado desde empleados: no editable aquí
        ({"email": "x@y.com"}, "email"),
        ({"salario": 1}, "salario"),  # desconocido: asignación masiva
        ({"telefono": "no-es-un-telefono!"}, "telefono"),
        ({"biografia": "x" * 1001}, "biografia"),
    ],
)
def test_put_rechaza_campos_no_editables_o_invalidos_con_400(cliente, cuerpo, campo):
    respuesta = cliente.put("/perfiles/E001", json=cuerpo)
    assert respuesta.status_code == 400
    assert any(e["field"] == campo for e in respuesta.json()["errors"])


def test_put_vacio_400(cliente):
    assert cliente.put("/perfiles/E001", json={}).status_code == 400


def test_put_perfil_inexistente_404(cliente):
    assert cliente.put("/perfiles/E999", json={"ciudad": "Armenia"}).status_code == 404


def test_put_perfil_archivado_409(cliente, repo):
    retiro = dominio.EmpleadoRetirado(
        empleadoId="E001",
        email="juan@empresa.com",
        fechaRetiro=datetime(2026, 11, 30, tzinfo=UTC),
        motivo="RENUNCIA",
    )
    asyncio.run(repo.aplicar_retirado("e2", retiro))

    respuesta = cliente.put("/perfiles/E001", json={"ciudad": "Armenia"})
    assert respuesta.status_code == 409
    assert cliente.get("/perfiles/E001").json()["archivado"] is True


def test_health_reporta_bd_y_broker(repo):
    sano = TestClient(crear_app(repo, lambda: "UP")).get("/health")
    assert sano.status_code == 200
    assert sano.json()["components"] == {"app": "UP", "db": "UP", "broker": "UP"}

    repo.fallo = ConnectionError("sin BD")
    caido = TestClient(crear_app(repo, lambda: "DOWN")).get("/health")
    assert caido.status_code == 503
    assert caido.json()["components"]["db"] == "DOWN"


def test_swagger_bajo_el_prefijo_del_servicio(cliente):
    assert cliente.get("/perfiles/docs").status_code == 200
    especificacion = cliente.get("/perfiles/openapi.json").json()
    assert {"/perfiles", "/perfiles/{empleadoId}"} <= set(especificacion["paths"])


def test_ruta_desconocida_404_en_el_formato_del_ecosistema(cliente):
    respuesta = cliente.get("/otra-cosa")
    assert respuesta.status_code == 404
    assert respuesta.json()["message"] == "Recurso no encontrado"
