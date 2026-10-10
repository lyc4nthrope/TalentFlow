import asyncio
import json
import uuid

import pytest

from app.consumidores import ConsumidorEmpleados
from app.dominio import dias_habiles
from app.errores import AppError, EventoInvalido
from datetime import date


def evento(tipo, data, id_=None):
    return json.dumps(
        {
            "id": id_ or str(uuid.uuid4()),
            "type": tipo,
            "version": 1,
            "occurredAt": "2026-10-10T12:00:00Z",
            "producer": "empleados-service",
            "data": data,
        }
    ).encode()


def alta_empleado(client, app, empleado_id="E001", email="juan.perez@empresa.com", id_=None):
    consumidor = ConsumidorEmpleados(app.state.repo)
    cuerpo = evento("empleado.creado", {"empleadoId": empleado_id, "email": email, "estado": "ACTIVO"}, id_)
    return client.portal.call(consumidor.manejar, cuerpo)


def programar(client, empleado="E001", inicio="2027-03-15", fin="2027-03-30"):
    return client.post("/vacaciones", json={"empleadoId": empleado, "fechaInicio": inicio, "fechaFin": fin})


# ------------------------------------------------------------ dominio

def test_dias_habiles_cuenta_solo_lunes_a_viernes():
    assert dias_habiles(date(2027, 3, 15), date(2027, 3, 19)) == 5  # lun-vie
    assert dias_habiles(date(2027, 3, 20), date(2027, 3, 21)) == 0  # sáb-dom
    assert dias_habiles(date(2027, 3, 15), date(2027, 3, 30)) == 12


# ------------------------------------------------------------ programar: caso feliz

def test_programar_crea_periodo_y_publica_evento(entorno):
    client, app, broker, _ = entorno
    alta_empleado(client, app)

    r = programar(client)

    assert r.status_code == 201
    cuerpo = r.json()
    assert cuerpo["id"] == "V-2026-0001"
    assert cuerpo["empleadoId"] == "E001"
    assert cuerpo["estado"] == "PROGRAMADA"
    assert cuerpo["fechaInicio"] == "2027-03-15"
    assert cuerpo["fechaCreacion"].endswith("Z")
    assert r.headers["location"] == "/vacaciones/V-2026-0001"

    assert broker.publicados == [
        (
            "vacaciones.programadas",
            {
                "vacacionesId": "V-2026-0001",
                "empleadoId": "E001",
                "email": "juan.perez@empresa.com",
                "fechaInicio": "2027-03-15",
                "fechaFin": "2027-03-30",
                "diasHabiles": 12,
            },
        )
    ]


def test_ids_consecutivos(entorno):
    client, app, _, _ = entorno
    alta_empleado(client, app)
    assert programar(client, inicio="2027-03-01", fin="2027-03-05").json()["id"] == "V-2026-0001"
    assert programar(client, inicio="2027-04-01", fin="2027-04-05").json()["id"] == "V-2026-0002"


# ------------------------------------------------------------ las 4 validaciones

def test_validacion_fechas_incoherentes(entorno):
    client, app, broker, _ = entorno
    alta_empleado(client, app)
    r = programar(client, inicio="2027-06-30", fin="2027-06-15")
    assert r.status_code == 400
    assert "posterior" in r.json()["message"]
    assert r.json()["errors"][0]["campo"] == "fechaFin"
    assert broker.publicados == []


def test_validacion_fechas_iguales_tambien_son_incoherentes(entorno):
    client, app, _, _ = entorno
    alta_empleado(client, app)
    assert programar(client, inicio="2027-06-15", fin="2027-06-15").status_code == 400


def test_validacion_fecha_en_el_pasado(entorno):
    client, app, _, _ = entorno
    alta_empleado(client, app)
    r = programar(client, inicio="2026-10-09", fin="2026-10-20")  # "hoy" simulado: 2026-10-10
    assert r.status_code == 400
    assert "anterior a la fecha actual" in r.json()["message"]


def test_inicio_hoy_es_valido(entorno):
    client, app, _, _ = entorno
    alta_empleado(client, app)
    assert programar(client, inicio="2026-10-10", fin="2026-10-20").status_code == 201


def test_validacion_solapamiento_incluye_periodo_en_conflicto(entorno):
    client, app, _, _ = entorno
    alta_empleado(client, app)
    primero = programar(client, inicio="2027-06-15", fin="2027-06-30").json()

    r = programar(client, inicio="2027-06-20", fin="2027-07-05")

    assert r.status_code == 400
    cuerpo = r.json()
    assert cuerpo["periodoEnConflicto"]["id"] == primero["id"]
    assert primero["id"] in cuerpo["message"]


@pytest.mark.parametrize(
    "inicio,fin,esperado",
    [
        ("2027-06-01", "2027-06-14", 201),  # termina justo antes
        ("2027-07-01", "2027-07-10", 201),  # empieza justo después
        ("2027-06-30", "2027-07-02", 400),  # toca el último día
        ("2027-06-10", "2027-06-15", 400),  # toca el primer día
        ("2027-06-01", "2027-07-31", 400),  # lo envuelve
        ("2027-06-18", "2027-06-20", 400),  # queda dentro
    ],
)
def test_bordes_del_solapamiento(entorno, inicio, fin, esperado):
    client, app, _, _ = entorno
    alta_empleado(client, app)
    assert programar(client, inicio="2027-06-15", fin="2027-06-30").status_code == 201
    assert programar(client, inicio=inicio, fin=fin).status_code == esperado


def test_otro_empleado_puede_tomar_las_mismas_fechas(entorno):
    client, app, _, _ = entorno
    alta_empleado(client, app, "E001", "a@empresa.com")
    alta_empleado(client, app, "E002", "b@empresa.com")
    assert programar(client, "E001").status_code == 201
    assert programar(client, "E002").status_code == 201


def test_periodo_cancelado_no_bloquea_fechas(entorno):
    client, app, _, _ = entorno
    alta_empleado(client, app)
    id_ = programar(client).json()["id"]
    assert client.delete(f"/vacaciones/{id_}").status_code == 200
    assert programar(client).status_code == 201


def test_validacion_empleado_inexistente(entorno):
    client, _, broker, _ = entorno
    r = programar(client, empleado="NO-EXISTE")
    assert r.status_code == 400
    assert "NO-EXISTE" in r.json()["message"]
    assert broker.publicados == []


def test_empleado_retirado_no_puede_programar(entorno):
    client, app, _, _ = entorno
    alta_empleado(client, app)
    consumidor = ConsumidorEmpleados(app.state.repo)
    client.portal.call(
        consumidor.manejar,
        evento("empleado.retirado", {"empleadoId": "E001", "email": "juan.perez@empresa.com",
                                     "fechaRetiro": "2026-10-10T12:00:00Z", "motivo": "RENUNCIA"}),
    )
    r = programar(client)
    assert r.status_code == 400
    assert "RETIRADO" in r.json()["message"]


def test_validacion_de_formato_responde_400(entorno):
    client, app, _, _ = entorno
    alta_empleado(client, app)
    assert client.post("/vacaciones", json={"empleadoId": "E001"}).status_code == 400
    assert programar(client, inicio="15/03/2027", fin="2027-03-30").status_code == 400
    r = client.post("/vacaciones", content="{no es json", headers={"content-type": "application/json"})
    assert r.status_code == 400
    assert r.json()["message"] == "Cuerpo JSON inválido"


# ------------------------------------------------------------ consultas y cancelación

def test_consultas(entorno):
    client, app, _, _ = entorno
    alta_empleado(client, app, "E001", "a@empresa.com")
    alta_empleado(client, app, "E002", "b@empresa.com")
    v1 = programar(client, "E001", "2027-03-01", "2027-03-05").json()
    programar(client, "E002", "2027-04-01", "2027-04-05")

    assert client.get(f"/vacaciones/{v1['id']}").json() == v1
    assert len(client.get("/vacaciones").json()) == 2
    solo_e1 = client.get("/vacaciones", params={"empleadoId": "E001"}).json()
    assert [p["id"] for p in solo_e1] == [v1["id"]]
    assert client.get("/vacaciones", params={"empleadoId": "NADIE"}).json() == []

    r = client.get("/vacaciones/V-2026-9999")
    assert r.status_code == 404
    assert r.json()["status"] == 404 and "timestamp" in r.json() and r.json()["path"] == "/vacaciones/V-2026-9999"


def test_cancelar(entorno):
    client, app, _, _ = entorno
    alta_empleado(client, app)
    id_ = programar(client).json()["id"]

    r = client.delete(f"/vacaciones/{id_}")
    assert r.status_code == 200 and r.json()["estado"] == "CANCELADA"
    assert client.get(f"/vacaciones/{id_}").json()["estado"] == "CANCELADA"  # no se borra
    assert client.delete(f"/vacaciones/{id_}").status_code == 409  # ya cancelada
    assert client.delete("/vacaciones/V-2026-9999").status_code == 404


def test_no_se_cancela_un_periodo_que_ya_inicio(entorno):
    client, app, _, reloj = entorno
    alta_empleado(client, app)
    id_ = programar(client, inicio="2026-10-20", fin="2026-10-30").json()["id"]

    from datetime import datetime
    from tests.conftest import ZONA
    reloj.ahora = datetime(2026, 10, 20, 8, 0, tzinfo=ZONA)  # llegó la fechaInicio

    r = client.delete(f"/vacaciones/{id_}")
    assert r.status_code == 409
    assert "ya inició" in r.json()["message"]


# ------------------------------------------------------------ eventos / deduplicación

def test_replica_se_alimenta_con_empleado_creado(entorno):
    client, app, _, _ = entorno
    assert alta_empleado(client, app) == "procesado"


def test_deduplicacion_mismo_id_un_solo_efecto(entorno):
    client, app, _, _ = entorno
    mismo_id = str(uuid.uuid4())
    assert alta_empleado(client, app, email="original@empresa.com", id_=mismo_id) == "procesado"
    # Reentrega del MISMO mensaje (mismo id de envelope), incluso con otro contenido:
    assert alta_empleado(client, app, email="otro@empresa.com", id_=mismo_id) == "duplicado"

    async def contar():
        async with app.state.repo.transaccion() as conn:
            cur = await conn.execute("SELECT count(*) AS n FROM eventos_procesados WHERE id = %s", (mismo_id,))
            registrados = (await cur.fetchone())["n"]
            emp = await app.state.repo.obtener_empleado(conn, "E001")
            return registrados, emp["email"]

    registrados, email = client.portal.call(contar)
    assert registrados == 1
    assert email == "original@empresa.com"  # el duplicado NO repitió el efecto


def test_si_el_efecto_falla_el_id_no_queda_registrado(entorno):
    """Dedup y efecto van en la misma transacción: un fallo permite reintentar."""
    client, app, _, _ = entorno
    consumidor = ConsumidorEmpleados(app.state.repo)
    id_ = str(uuid.uuid4())

    async def romper(conn, *a, **k):
        raise RuntimeError("BD caída a mitad de camino")

    original = app.state.repo.registrar_empleado
    app.state.repo.registrar_empleado = romper
    with pytest.raises(RuntimeError):
        client.portal.call(consumidor.manejar, evento("empleado.creado", {"empleadoId": "E9", "email": "x@x.com"}, id_))
    app.state.repo.registrar_empleado = original

    # La reentrega ahora sí se procesa (el id no quedó marcado como visto)
    assert client.portal.call(consumidor.manejar, evento("empleado.creado", {"empleadoId": "E9", "email": "x@x.com"}, id_)) == "procesado"


def test_eventos_invalidos_y_ajenos(entorno):
    client, app, _, _ = entorno
    consumidor = ConsumidorEmpleados(app.state.repo)
    with pytest.raises(EventoInvalido):
        client.portal.call(consumidor.manejar, b"esto no es json")
    with pytest.raises(EventoInvalido):
        client.portal.call(consumidor.manejar, json.dumps({"type": "empleado.creado"}).encode())
    with pytest.raises(EventoInvalido):
        client.portal.call(consumidor.manejar, evento("empleado.creado", {"email": "a@a.com"}))  # sin empleadoId
    # Un evento que este servicio no consume se ignora (y se confirma)
    assert client.portal.call(consumidor.manejar, evento("empleado.actualizado", {"empleadoId": "E001"})) == "ignorado"


# ------------------------------------------------------------ resiliencia

def test_si_falla_la_publicacion_el_periodo_no_se_revierte(entorno):
    client, app, broker, _ = entorno
    alta_empleado(client, app)
    broker.fallar = True

    r = programar(client)

    assert r.status_code == 201
    assert client.get(f"/vacaciones/{r.json()['id']}").status_code == 200


def test_programaciones_simultaneas_solo_una_gana(entorno):
    client, app, _, _ = entorno
    alta_empleado(client, app)

    async def carrera():
        servicio = app.state.servicio
        tareas = [servicio.programar("E001", date(2027, 5, 1), date(2027, 5, 10)) for _ in range(5)]
        return await asyncio.gather(*tareas, return_exceptions=True)

    resultados = client.portal.call(carrera)
    exitos = [r for r in resultados if isinstance(r, dict)]
    rechazos = [r for r in resultados if isinstance(r, AppError)]
    assert len(exitos) == 1 and len(rechazos) == 4


# ------------------------------------------------------------ salud y OpenAPI

def test_health_y_openapi(entorno):
    client, _, _, _ = entorno
    h = client.get("/health")
    assert h.status_code == 200 and h.json()["components"]["db"] == "UP"

    spec = client.get("/vacaciones/openapi.json").json()
    assert set(spec["paths"]) == {"/vacaciones", "/vacaciones/{id}"}
    assert set(spec["paths"]["/vacaciones"]) == {"get", "post"}
    assert "delete" in spec["paths"]["/vacaciones/{id}"]
    assert "422" not in json.dumps(spec)
    assert client.get("/vacaciones/docs").status_code == 200  # Swagger UI
