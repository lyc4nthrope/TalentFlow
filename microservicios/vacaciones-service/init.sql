-- Modelo de base de datos del microservicio de gestión de vacaciones
-- Motor: PostgreSQL (ver docker-compose.yml -> database-vacaciones)

-- Períodos de vacaciones. Estados: PROGRAMADA -> EN_CURSO -> FINALIZADA, o CANCELADA.
-- (En el Reto 4 solo se llega a PROGRAMADA o CANCELADA; las transiciones por fecha
-- las hará el scheduler del Reto 5.)
CREATE TABLE IF NOT EXISTS vacaciones (
    id              VARCHAR(20)  PRIMARY KEY,
    empleado_id     VARCHAR(20)  NOT NULL,
    fecha_inicio    DATE         NOT NULL,
    fecha_fin       DATE         NOT NULL,
    estado          VARCHAR(20)  NOT NULL DEFAULT 'PROGRAMADA'
                    CHECK (estado IN ('PROGRAMADA', 'EN_CURSO', 'FINALIZADA', 'CANCELADA')),
    fecha_creacion  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT ck_fechas_coherentes CHECK (fecha_fin > fecha_inicio)
);

CREATE INDEX IF NOT EXISTS idx_vacaciones_empleado ON vacaciones (empleado_id);
CREATE INDEX IF NOT EXISTS idx_vacaciones_estado_fechas ON vacaciones (estado, fecha_inicio, fecha_fin);

-- Contador por año para generar identificadores V-AAAA-NNNN de forma atómica.
CREATE TABLE IF NOT EXISTS secuencias (
    anio    INTEGER PRIMARY KEY,
    ultimo  INTEGER NOT NULL
);

-- Réplica local mínima de empleados válidos, mantenida por eventos
-- (empleado.creado / empleado.retirado). Ver la justificación en el README.
CREATE TABLE IF NOT EXISTS empleados_replica (
    empleado_id     VARCHAR(20)  PRIMARY KEY,
    email           VARCHAR(150) NOT NULL,
    estado          VARCHAR(20)  NOT NULL DEFAULT 'ACTIVO'
                    CHECK (estado IN ('ACTIVO', 'RETIRADO')),
    actualizado_en  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Deduplicación de eventos por id de mensaje (Catálogo de Eventos, sección 2.1).
CREATE TABLE IF NOT EXISTS eventos_procesados (
    id            VARCHAR(64)  PRIMARY KEY,
    procesado_en  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
