"""Repositorio en memoria con el mismo comportamiento observable que el de PostgreSQL."""

import uuid
from datetime import UTC, datetime

from app import dominio


class RepositorioEnMemoria:
    def __init__(self):
        self.perfiles: dict[str, dominio.Perfil] = {}
        self.procesados: set[str] = set()
        self.fallo: Exception | None = None

    async def _una_vez(self, evento_id: str, efecto) -> bool:
        if self.fallo is not None:
            raise self.fallo
        if evento_id in self.procesados:
            return False
        self.procesados.add(evento_id)
        efecto()
        return True

    def _nuevo(self, empleado_id: str, nombre: str, email: str) -> dominio.Perfil:
        return dominio.Perfil(
            id=str(uuid.uuid4()),
            empleadoId=empleado_id,
            nombre=nombre,
            email=email,
            fechaCreacion=datetime.now(UTC),
        )

    async def aplicar_creado(self, evento_id, datos):
        def crear():
            if datos.empleadoId not in self.perfiles:
                self.perfiles[datos.empleadoId] = self._nuevo(
                    datos.empleadoId, dominio.nombre_completo(datos.nombre, datos.apellido), datos.email
                )

        return await self._una_vez(evento_id, crear)

    async def aplicar_actualizado(self, evento_id, datos):
        def sincronizar():
            nombre = dominio.nombre_completo(datos.nombre, datos.apellido)
            actual = self.perfiles.get(datos.empleadoId)
            if actual is None:
                self.perfiles[datos.empleadoId] = self._nuevo(datos.empleadoId, nombre, datos.email)
            else:
                self.perfiles[datos.empleadoId] = actual.model_copy(
                    update={"nombre": nombre, "email": datos.email}
                )

        return await self._una_vez(evento_id, sincronizar)

    async def aplicar_retirado(self, evento_id, datos):
        def archivar():
            actual = self.perfiles.get(datos.empleadoId)
            if actual is not None and not actual.archivado:
                self.perfiles[datos.empleadoId] = actual.model_copy(
                    update={"archivado": True, "fechaArchivado": datos.fechaRetiro}
                )

        return await self._una_vez(evento_id, archivar)

    async def obtener(self, empleado_id):
        return self.perfiles.get(empleado_id)

    async def listar(self):
        return list(self.perfiles.values())

    async def actualizar(self, empleado_id, cambios):
        actual = self.perfiles.get(empleado_id)
        if actual is None:
            raise dominio.PerfilNoEncontrado(empleado_id)
        if actual.archivado:
            raise dominio.PerfilArchivado(empleado_id)
        self.perfiles[empleado_id] = actual.model_copy(update=cambios)
        return self.perfiles[empleado_id]

    async def ping(self):
        if self.fallo is not None:
            raise self.fallo
