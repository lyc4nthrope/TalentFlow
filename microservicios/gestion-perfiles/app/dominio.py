"""Reglas del dominio de perfiles. No conoce ni HTTP, ni el broker, ni SQL."""

from datetime import datetime

from pydantic import BaseModel, ConfigDict, Field, model_validator

# Tipos de evento que consume este servicio (Catálogo de Eventos, sección 3).
EMPLEADO_CREADO = "empleado.creado"
EMPLEADO_ACTUALIZADO = "empleado.actualizado"
EMPLEADO_RETIRADO = "empleado.retirado"


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

    @model_validator(mode="after")
    def al_menos_un_campo(self) -> "ActualizacionPerfil":
        if not self.cambios():
            raise ValueError("Debe enviar al menos uno de: telefono, direccion, ciudad, biografia")
        return self

    def cambios(self) -> dict[str, str]:
        """Solo los campos enviados (los omitidos no se tocan)."""
        return self.model_dump(exclude_none=True)


class PerfilNoEncontrado(Exception):
    def __init__(self, empleado_id: str):
        super().__init__(f"El perfil del empleado {empleado_id} no existe")


class PerfilArchivado(Exception):
    def __init__(self, empleado_id: str):
        super().__init__(f"El perfil del empleado {empleado_id} está archivado y no se puede modificar")
