"""Reglas del dominio de perfiles. No conoce ni HTTP, ni el broker, ni SQL."""

from datetime import datetime
from typing import Annotated
from urllib.parse import urlsplit

from pydantic import AfterValidator, BaseModel, ConfigDict, Field, model_validator
from pydantic_core import PydanticCustomError

# Tipos de evento que consume este servicio (Catálogo de Eventos, sección 3).
EMPLEADO_CREADO = "empleado.creado"
EMPLEADO_ACTUALIZADO = "empleado.actualizado"
EMPLEADO_RETIRADO = "empleado.retirado"


def _url_http(valor: str) -> str:
    """Solo http/https con host: bloquea javascript:, data:, ftp: y similares (XSS al
    pintar la foto o el enlace en el frontend). La cadena vacía se acepta: quita el dato.

    También rechaza lo que hace que el navegador vaya a otro sitio del que parece:
    usuario en la URL ("https://linkedin.com@evil.com"), barra invertida, espacios y
    caracteres de control (urlsplit borra tab y saltos de línea sin avisar)."""
    if valor == "":
        return valor
    sospechosa = "\\" in valor or any(c.isspace() or ord(c) < 32 or ord(c) == 127 for c in valor)
    partes = urlsplit(valor)
    if (
        sospechosa
        or partes.scheme.lower() not in ("http", "https")
        or not partes.hostname
        or partes.username is not None
    ):
        raise PydanticCustomError("url_http", "Debe ser una URL http o https")
    return valor


def _url(max_longitud: int, ejemplo: str, descripcion: str, vacia_es_nula: bool = False):
    """vacia_es_nula: en redesSociales "" significa "no registrada" y se guarda como null."""

    def validar(valor: str | None) -> str | None:
        if valor is None or (vacia_es_nula and valor == ""):
            return None
        return _url_http(valor)

    return Annotated[
        str | None,
        Field(default=None, max_length=max_longitud, description=descripcion, examples=[ejemplo]),
        AfterValidator(validar),
    ]


class RedesSociales(BaseModel):
    """Enlaces a redes sociales del empleado. Las tres claves siempre están en la
    respuesta; null significa "no registrada"."""

    linkedin: str | None = Field(default=None, examples=["https://www.linkedin.com/in/juan-perez"])
    github: str | None = Field(default=None, examples=["https://github.com/juanperez"])
    twitter: str | None = Field(default=None, examples=["https://x.com/juanperez"])


class RedesSocialesEntrada(BaseModel):
    """redesSociales en PUT: reemplaza el objeto guardado completo (lo no enviado queda
    null). extra="forbid": una red desconocida responde 400."""

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    linkedin: _url(
        300, "https://www.linkedin.com/in/juan-perez", "URL http/https, máx. 300", vacia_es_nula=True
    )
    github: _url(300, "https://github.com/juanperez", "URL http/https, máx. 300", vacia_es_nula=True)
    twitter: _url(300, "https://x.com/juanperez", "URL http/https, máx. 300", vacia_es_nula=True)


class Perfil(BaseModel):
    """Perfil del empleado (reto4.pdf, "Perfil por defecto") + estado de archivo."""

    id: str
    empleadoId: str
    nombre: str
    email: str
    telefono: str = ""
    direccion: str = ""
    ciudad: str = ""
    biografia: str = ""
    foto: str = Field(default="", description="URL de la foto; vacío si no tiene")
    redesSociales: RedesSociales = Field(default_factory=RedesSociales)
    codigoPostal: str = ""
    fechaCreacion: datetime
    # "Archivar el perfil (marcarlo como archivado, no borrarlo)".
    archivado: bool = False
    fechaArchivado: datetime | None = None


# --- Cargas útiles de los eventos, campo por campo según el Catálogo de Eventos ---
# extra="ignore": el perfil solo usa algunos campos; los demás no le conciernen.


class _Carga(BaseModel):
    model_config = ConfigDict(extra="ignore")


class EmpleadoCreado(_Carga):  # 3.1
    empleadoId: str = Field(min_length=1)
    nombre: str = Field(min_length=1)
    apellido: str
    email: str = Field(min_length=1)


class EmpleadoActualizado(_Carga):  # 3.2
    empleadoId: str = Field(min_length=1)
    nombre: str = Field(min_length=1)
    apellido: str
    email: str = Field(min_length=1)


class EmpleadoRetirado(_Carga):  # 3.3
    empleadoId: str = Field(min_length=1)
    email: str
    fechaRetiro: datetime
    motivo: str


def nombre_completo(nombre: str, apellido: str) -> str:
    """El perfil guarda un solo "nombre": el completo, como lo muestra el reto."""
    return f"{nombre} {apellido}".strip()


class ActualizacionPerfil(BaseModel):
    """Cuerpo de PUT /perfiles/{empleadoId}: actualización PARCIAL de los campos que
    el empleado gestiona en su perfil (el reto4.pdf envía solo algunos).

    nombre y email NO son editables aquí: se replican desde empleados-service
    (empleado.actualizado) y ese servicio es su dueño. extra="forbid" hace que
    cualquier otro campo responda 400 (evita la asignación masiva).
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    telefono: str | None = Field(default=None, max_length=20, pattern=r"^\+?[0-9 ()-]*$")
    direccion: str | None = Field(default=None, max_length=200)
    ciudad: str | None = Field(default=None, max_length=100)
    biografia: str | None = Field(default=None, max_length=1000)
    foto: _url(500, "https://cdn.empresa.com/fotos/E001.jpg", "URL http/https de la foto, máx. 500")
    redesSociales: RedesSocialesEntrada | None = Field(
        default=None, description="Reemplaza el objeto completo: lo no enviado queda null"
    )
    codigoPostal: str | None = Field(
        default=None, max_length=10, pattern=r"^[A-Za-z0-9 -]*$", examples=["630001"]
    )

    @model_validator(mode="after")
    def al_menos_un_campo(self) -> "ActualizacionPerfil":
        if not self.cambios():
            raise ValueError(
                "Debe enviar al menos uno de: telefono, direccion, ciudad, biografia, "
                "foto, redesSociales, codigoPostal"
            )
        return self

    def cambios(self) -> dict[str, object]:
        """Solo los campos enviados (los omitidos no se tocan). redesSociales, si llega,
        va completo con sus tres claves: reemplaza lo guardado, no se mezcla."""
        cambios = self.model_dump(exclude_none=True, exclude={"redesSociales"})
        if self.redesSociales is not None:
            cambios["redesSociales"] = self.redesSociales.model_dump()
        return cambios


class PerfilNoEncontrado(Exception):
    def __init__(self, empleado_id: str):
        super().__init__(f"El perfil del empleado {empleado_id} no existe")


class PerfilArchivado(Exception):
    def __init__(self, empleado_id: str):
        super().__init__(f"El perfil del empleado {empleado_id} está archivado y no se puede modificar")
