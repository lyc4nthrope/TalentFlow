"""Reglas de ActualizacionPerfil para los campos del proyecto final (foto, redes, código postal)."""

import re
from pathlib import Path

import pytest
from pydantic import ValidationError

from app import dominio
from app.repositorio import MIGRACION


def errores(cuerpo: dict) -> list[dict]:
    with pytest.raises(ValidationError) as error:
        dominio.ActualizacionPerfil.model_validate(cuerpo)
    return error.value.errors()


@pytest.mark.parametrize("url", ["https://cdn.empresa.com/fotos/E001.jpg", "http://intranet/foto.png", ""])
def test_foto_acepta_http_https_o_vacia(url):
    assert dominio.ActualizacionPerfil(foto=url).cambios() == {"foto": url}


@pytest.mark.parametrize(
    "url",
    [
        "javascript:alert(1)",
        "data:image/png;base64,AAAA",
        "ftp://servidor/foto.jpg",
        "https://",  # sin host
        "https://:443",  # puerto sin host
        "foto.jpg",  # relativa
        "https://linkedin.com@evil.com",  # userinfo: el navegador va a evil.com
        "https://evil.com\\@linkedin.com",  # barra invertida: urlsplit ve linkedin.com, el navegador evil.com
        "https://exa mple.com",  # espacio dentro de la URL
        "https://a\nb.com",  # urlsplit borra el salto de línea en silencio
        "https://a.com/\tx",  # tabulador
        "https://evil.com\\.linkedin.com",  # barra invertida sin "@": el navegador la toma como "/"
        "https://a.com/\x07",  # carácter de control que no es espacio
        "https://a.com/\x7f",  # DEL
    ],
)
def test_foto_rechaza_otros_esquemas(url):
    assert errores({"foto": url})[0]["type"] == "url_http"


def test_foto_maximo_500():
    largo = "https://x.co/" + "a" * (500 - len("https://x.co/"))
    assert dominio.ActualizacionPerfil(foto=largo).cambios()["foto"] == largo
    assert errores({"foto": largo + "a"})[0]["type"] == "string_too_long"


def test_redes_sociales_reemplaza_el_objeto_completo():
    cambios = dominio.ActualizacionPerfil(redesSociales={"github": "https://github.com/juan"}).cambios()
    # Las tres claves van siempre: las no enviadas quedan None y borran lo guardado.
    assert cambios == {
        "redesSociales": {"linkedin": None, "github": "https://github.com/juan", "twitter": None}
    }


def test_red_social_vacia_queda_null():
    # null = "no registrada" (contrato de RedesSociales): "" no se guarda como texto.
    cambios = dominio.ActualizacionPerfil(redesSociales={"github": ""}).cambios()
    assert cambios["redesSociales"]["github"] is None


def test_redes_sociales_vacio_borra_todas():
    assert dominio.ActualizacionPerfil(redesSociales={}).cambios() == {
        "redesSociales": {"linkedin": None, "github": None, "twitter": None}
    }


@pytest.mark.parametrize(
    ("redes", "tipo"),
    [
        ({"facebook": "https://facebook.com/juan"}, "extra_forbidden"),
        ({"linkedin": "javascript:alert(1)"}, "url_http"),
        ({"twitter": "https://x.com/" + "a" * 300}, "string_too_long"),
        ("https://github.com/juan", "model_type"),
    ],
)
def test_redes_sociales_invalidas(redes, tipo):
    assert errores({"redesSociales": redes})[0]["type"] == tipo


@pytest.mark.parametrize("codigo", ["630001", "SW1A 1AA", "28-001", ""])
def test_codigo_postal_valido(codigo):
    assert dominio.ActualizacionPerfil(codigoPostal=codigo).cambios() == {"codigoPostal": codigo}


@pytest.mark.parametrize(
    ("codigo", "tipo"), [("63000#", "string_pattern_mismatch"), ("1" * 11, "string_too_long")]
)
def test_codigo_postal_invalido(codigo, tipo):
    assert errores({"codigoPostal": codigo})[0]["type"] == tipo


def test_solo_los_campos_enviados_son_cambios():
    assert dominio.ActualizacionPerfil(ciudad="Armenia", codigoPostal="630001").cambios() == {
        "ciudad": "Armenia",
        "codigoPostal": "630001",
    }


def test_cuerpo_vacio_lista_todos_los_campos_editables():
    mensaje = errores({})[0]["msg"]
    for campo in ("telefono", "direccion", "ciudad", "biografia", "foto", "redesSociales", "codigoPostal"):
        assert campo in mensaje


def test_perfil_por_defecto_trae_las_tres_redes_en_null():
    perfil = dominio.Perfil(
        id="1", empleadoId="E001", nombre="J", email="j@e.com", fechaCreacion="2026-01-01T00:00:00Z"
    )
    assert (perfil.foto, perfil.codigoPostal) == ("", "")
    assert perfil.redesSociales.model_dump() == {"linkedin": None, "github": None, "twitter": None}


def test_la_migracion_del_arranque_coincide_con_init_sql():
    """Un volumen viejo (migración) y uno nuevo (init.sql) deben quedar con las mismas columnas."""
    init = (Path(__file__).parents[1] / "init.sql").read_text()
    for columna in ("foto", "codigo_postal", "redes_sociales"):
        definicion = re.search(rf"^\s*{columna}\s+(.+?),?$", init, re.MULTILINE).group(1).strip()
        assert f"ADD COLUMN IF NOT EXISTS {columna} {' '.join(definicion.split())};" in MIGRACION
