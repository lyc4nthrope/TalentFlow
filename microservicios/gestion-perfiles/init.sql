-- Base de datos propia del servicio de perfiles (PostgreSQL).

-- Deduplicación (Catálogo de Eventos, 2.1): id de cada mensaje ya procesado.
CREATE TABLE IF NOT EXISTS eventos_procesados (
    id            VARCHAR(100) PRIMARY KEY,
    procesado_en  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS perfiles (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Un perfil por empleado: la UNIQUE hace idempotente la creación por evento.
    empleado_id      VARCHAR(20)   NOT NULL UNIQUE,
    -- Campos replicados desde empleados-service (empleado.creado / .actualizado).
    nombre           VARCHAR(201)  NOT NULL,
    email            VARCHAR(150)  NOT NULL,
    -- Campos que gestiona el propio empleado (PUT /perfiles/{empleadoId}).
    telefono         VARCHAR(20)   NOT NULL DEFAULT '',
    direccion        VARCHAR(200)  NOT NULL DEFAULT '',
    ciudad           VARCHAR(100)  NOT NULL DEFAULT '',
    biografia        VARCHAR(1000) NOT NULL DEFAULT '',
    fecha_creacion   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    -- Baja lógica al consumir empleado.retirado: se archiva, no se borra.
    archivado        BOOLEAN       NOT NULL DEFAULT false,
    fecha_archivado  TIMESTAMPTZ,
    CONSTRAINT chk_archivo_coherente CHECK (archivado = (fecha_archivado IS NOT NULL))
);
