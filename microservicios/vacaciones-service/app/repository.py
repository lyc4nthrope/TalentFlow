"""Acceso a PostgreSQL. Todas las operaciones reciben la conexión para poder
componerlas dentro de una misma transacción."""
from contextlib import asynccontextmanager
from datetime import date
from typing import Any, Optional

from psycopg import AsyncConnection
from psycopg.rows import dict_row
from psycopg_pool import AsyncConnectionPool


def crear_pool(dsn: str) -> AsyncConnectionPool:
    return AsyncConnectionPool(
        dsn,
        min_size=1,
        max_size=10,
        open=False,
        kwargs={"row_factory": dict_row},
    )


class VacacionesRepository:
    def __init__(self, pool: AsyncConnectionPool):
        self.pool = pool

    @asynccontextmanager
    async def transaccion(self):
        # Commit al salir sin error; rollback si se lanza una excepción.
        async with self.pool.connection() as conn:
            yield conn

    async def ping(self) -> None:
        async with self.pool.connection(timeout=2) as conn:
            await conn.execute("SELECT 1")

    # ---------- vacaciones ----------

    async def bloquear_empleado(self, conn: AsyncConnection, empleado_id: str) -> None:
        # Serializa las programaciones del mismo empleado: evita que dos peticiones
        # simultáneas pasen la validación de solapamiento a la vez.
        await conn.execute("SELECT pg_advisory_xact_lock(hashtext(%s))", (empleado_id,))

    async def buscar_solapado(
        self, conn: AsyncConnection, empleado_id: str, inicio: date, fin: date
    ) -> Optional[dict[str, Any]]:
        cur = await conn.execute(
            """
            SELECT * FROM vacaciones
            WHERE empleado_id = %s
              AND estado IN ('PROGRAMADA', 'EN_CURSO')
              AND fecha_inicio <= %s
              AND fecha_fin >= %s
            ORDER BY fecha_inicio
            LIMIT 1
            """,
            (empleado_id, fin, inicio),
        )
        return await cur.fetchone()

    async def siguiente_id(self, conn: AsyncConnection, anio: int) -> str:
        cur = await conn.execute(
            """
            INSERT INTO secuencias (anio, ultimo) VALUES (%s, 1)
            ON CONFLICT (anio) DO UPDATE SET ultimo = secuencias.ultimo + 1
            RETURNING ultimo
            """,
            (anio,),
        )
        fila = await cur.fetchone()
        return f"V-{anio}-{fila['ultimo']:04d}"

    async def insertar(
        self, conn: AsyncConnection, id_: str, empleado_id: str, inicio: date, fin: date
    ) -> dict[str, Any]:
        cur = await conn.execute(
            """
            INSERT INTO vacaciones (id, empleado_id, fecha_inicio, fecha_fin, estado)
            VALUES (%s, %s, %s, %s, 'PROGRAMADA')
            RETURNING *
            """,
            (id_, empleado_id, inicio, fin),
        )
        return await cur.fetchone()

    async def obtener(
        self, conn: AsyncConnection, id_: str, para_actualizar: bool = False
    ) -> Optional[dict[str, Any]]:
        sql = "SELECT * FROM vacaciones WHERE id = %s"
        if para_actualizar:
            sql += " FOR UPDATE"
        cur = await conn.execute(sql, (id_,))
        return await cur.fetchone()

    async def listar(
        self, conn: AsyncConnection, empleado_id: Optional[str] = None
    ) -> list[dict[str, Any]]:
        if empleado_id:
            cur = await conn.execute(
                "SELECT * FROM vacaciones WHERE empleado_id = %s ORDER BY fecha_inicio, id",
                (empleado_id,),
            )
        else:
            cur = await conn.execute("SELECT * FROM vacaciones ORDER BY fecha_inicio, id")
        return await cur.fetchall()

    async def marcar_cancelada(self, conn: AsyncConnection, id_: str) -> dict[str, Any]:
        cur = await conn.execute(
            "UPDATE vacaciones SET estado = 'CANCELADA' WHERE id = %s RETURNING *", (id_,)
        )
        return await cur.fetchone()

    # ---------- réplica de empleados ----------

    async def obtener_empleado(
        self, conn: AsyncConnection, empleado_id: str
    ) -> Optional[dict[str, Any]]:
        cur = await conn.execute(
            "SELECT * FROM empleados_replica WHERE empleado_id = %s", (empleado_id,)
        )
        return await cur.fetchone()

    async def registrar_empleado(self, conn: AsyncConnection, empleado_id: str, email: str) -> None:
        # DO NOTHING: si por alguna razón el retiro llegó antes, no se "resucita" al empleado.
        await conn.execute(
            """
            INSERT INTO empleados_replica (empleado_id, email, estado)
            VALUES (%s, %s, 'ACTIVO')
            ON CONFLICT (empleado_id) DO NOTHING
            """,
            (empleado_id, email),
        )

    async def retirar_empleado(self, conn: AsyncConnection, empleado_id: str, email: str) -> None:
        await conn.execute(
            """
            INSERT INTO empleados_replica (empleado_id, email, estado)
            VALUES (%s, %s, 'RETIRADO')
            ON CONFLICT (empleado_id)
            DO UPDATE SET estado = 'RETIRADO', actualizado_en = now()
            """,
            (empleado_id, email),
        )

    # ---------- deduplicación ----------

    async def registrar_evento(self, conn: AsyncConnection, evento_id: str) -> bool:
        """True si el evento es nuevo; False si ya se había procesado (duplicado)."""
        cur = await conn.execute(
            "INSERT INTO eventos_procesados (id) VALUES (%s) ON CONFLICT (id) DO NOTHING",
            (evento_id,),
        )
        return cur.rowcount == 1
