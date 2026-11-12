"""Persistencia de perfiles sobre PostgreSQL (psycopg 3, asíncrono)."""

import logging
from collections.abc import Awaitable, Callable

import psycopg
from psycopg import AsyncConnection, sql
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb
from psycopg_pool import AsyncConnectionPool

from app import dominio
from app.aplicacion import RechazoPermanente

logger = logging.getLogger("perfiles.repositorio")

_COLUMNAS = sql.SQL(
    "id::text AS id, empleado_id, nombre, email, telefono, direccion, ciudad, biografia, "
    "foto, redes_sociales, codigo_postal, fecha_creacion, archivado, fecha_archivado"
)

# Campo del API -> columna, solo donde el nombre cambia (camelCase vs snake_case).
_COLUMNA_DE = {"redesSociales": "redes_sociales", "codigoPostal": "codigo_postal"}

# Migración al arrancar (proyecto final, F2): los volúmenes creados antes de estas columnas
# no vuelven a ejecutar init.sql. IF NOT EXISTS la hace idempotente y el candado de
# transacción serializa varias réplicas que arrancan a la vez. Debe coincidir con init.sql.
MIGRACION = """
SELECT pg_advisory_xact_lock(hashtext('perfiles.migracion'));
ALTER TABLE perfiles ADD COLUMN IF NOT EXISTS foto VARCHAR(500) NOT NULL DEFAULT '';
ALTER TABLE perfiles ADD COLUMN IF NOT EXISTS codigo_postal VARCHAR(10) NOT NULL DEFAULT '';
ALTER TABLE perfiles ADD COLUMN IF NOT EXISTS redes_sociales JSONB NOT NULL DEFAULT '{}'::jsonb;
"""


def _a_perfil(fila: dict) -> dominio.Perfil:
    return dominio.Perfil(
        id=fila["id"],
        empleadoId=fila["empleado_id"],
        nombre=fila["nombre"],
        email=fila["email"],
        telefono=fila["telefono"],
        direccion=fila["direccion"],
        ciudad=fila["ciudad"],
        biografia=fila["biografia"],
        foto=fila["foto"],
        redesSociales=fila["redes_sociales"],  # psycopg ya entrega el JSONB como dict
        codigoPostal=fila["codigo_postal"],
        fechaCreacion=fila["fecha_creacion"],
        archivado=fila["archivado"],
        fechaArchivado=fila["fecha_archivado"],
    )


class RepositorioPostgres:
    def __init__(self, pool: AsyncConnectionPool):
        self._pool = pool

    async def migrar(self) -> None:
        """Añade las columnas nuevas si faltan. Seguro en cada arranque y con réplicas."""
        async with self._pool.connection() as conexion, conexion.transaction():
            await conexion.execute(MIGRACION)

    async def _una_vez(self, evento_id: str, efecto: Callable[[AsyncConnection], Awaitable[None]]) -> bool:
        """Registra el id del evento y aplica su efecto en UNA transacción: o quedan
        ambos o ninguno. Devuelve False si el id ya existía (duplicado).

        Con dos consumidores procesando el mismo id a la vez, el segundo INSERT espera
        al commit del primero y luego no inserta: nunca hay doble efecto."""
        try:
            async with self._pool.connection() as conexion, conexion.transaction():
                cursor = await conexion.execute(
                    "INSERT INTO eventos_procesados (id) VALUES (%s) ON CONFLICT (id) DO NOTHING",
                    (evento_id,),
                )
                if cursor.rowcount == 0:
                    return False
                await efecto(conexion)
            return True
        except (psycopg.DataError, psycopg.IntegrityError) as error:
            # SQLSTATE 22 (dato inválido) / 23 (restricción violada): permanente.
            raise RechazoPermanente(str(error)) from error

    async def aplicar_creado(self, evento_id: str, datos: dominio.EmpleadoCreado) -> bool:
        async def crear(conexion: AsyncConnection) -> None:
            # ON CONFLICT: si el perfil ya existe (p. ej. lo creó un empleado.actualizado
            # que llegó antes), no se pisa.
            await conexion.execute(
                "INSERT INTO perfiles (empleado_id, nombre, email) VALUES (%s, %s, %s) "
                "ON CONFLICT (empleado_id) DO NOTHING",
                (datos.empleadoId, dominio.nombre_completo(datos.nombre, datos.apellido), datos.email),
            )

        return await self._una_vez(evento_id, crear)

    async def aplicar_actualizado(self, evento_id: str, datos: dominio.EmpleadoActualizado) -> bool:
        async def sincronizar(conexion: AsyncConnection) -> None:
            # Upsert: si el perfil no existe (se perdió su empleado.creado, p. ej. porque
            # el broker estaba caído cuando empleados-service lo publicó), se crea con
            # los datos de este evento. El sistema se recupera solo.
            await conexion.execute(
                "INSERT INTO perfiles (empleado_id, nombre, email) VALUES (%s, %s, %s) "
                "ON CONFLICT (empleado_id) DO UPDATE SET nombre = EXCLUDED.nombre, email = EXCLUDED.email",
                (datos.empleadoId, dominio.nombre_completo(datos.nombre, datos.apellido), datos.email),
            )

        return await self._una_vez(evento_id, sincronizar)

    async def aplicar_retirado(self, evento_id: str, datos: dominio.EmpleadoRetirado) -> bool:
        async def archivar(conexion: AsyncConnection) -> None:
            cursor = await conexion.execute(
                "UPDATE perfiles SET archivado = true, fecha_archivado = %s "
                "WHERE empleado_id = %s AND NOT archivado",
                (datos.fechaRetiro, datos.empleadoId),
            )
            if cursor.rowcount == 0:
                logger.warning(
                    "empleado.retirado sin perfil activo que archivar: empleadoId=%s", datos.empleadoId
                )

        return await self._una_vez(evento_id, archivar)

    async def obtener(self, empleado_id: str) -> dominio.Perfil | None:
        async with self._pool.connection() as conexion:
            cursor = conexion.cursor(row_factory=dict_row)
            await cursor.execute(
                sql.SQL("SELECT {} FROM perfiles WHERE empleado_id = %s").format(_COLUMNAS), (empleado_id,)
            )
            fila = await cursor.fetchone()
        return _a_perfil(fila) if fila else None

    async def listar(self) -> list[dominio.Perfil]:
        async with self._pool.connection() as conexion:
            cursor = conexion.cursor(row_factory=dict_row)
            await cursor.execute(
                sql.SQL("SELECT {} FROM perfiles ORDER BY fecha_creacion, empleado_id").format(_COLUMNAS)
            )
            filas = await cursor.fetchall()
        return [_a_perfil(fila) for fila in filas]

    async def actualizar(self, empleado_id: str, cambios: dict[str, object]) -> dominio.Perfil:
        """Actualización parcial atómica: la condición NOT archivado va en el UPDATE,
        así un empleado.retirado concurrente no puede quedar "desarchivado"."""
        # redesSociales se guarda entero (reemplazo, no mezcla): Jsonb lo envía como jsonb.
        valores = {
            campo: Jsonb(valor) if campo == "redesSociales" else valor for campo, valor in cambios.items()
        }
        asignaciones = sql.SQL(", ").join(
            sql.SQL("{} = {}").format(sql.Identifier(_COLUMNA_DE.get(campo, campo)), sql.Placeholder(campo))
            for campo in cambios
        )
        consulta = sql.SQL(
            "UPDATE perfiles SET {} WHERE empleado_id = {} AND NOT archivado RETURNING {}"
        ).format(asignaciones, sql.Placeholder("empleado_id"), _COLUMNAS)

        async with self._pool.connection() as conexion:
            cursor = conexion.cursor(row_factory=dict_row)
            await cursor.execute(consulta, {**valores, "empleado_id": empleado_id})
            fila = await cursor.fetchone()

        if fila:
            return _a_perfil(fila)
        if await self.obtener(empleado_id) is None:
            raise dominio.PerfilNoEncontrado(empleado_id)
        raise dominio.PerfilArchivado(empleado_id)

    async def ping(self) -> None:
        async with self._pool.connection(timeout=2) as conexion:
            await conexion.execute("SELECT 1")
