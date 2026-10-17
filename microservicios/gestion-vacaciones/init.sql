-- Base de datos propia del servicio de vacaciones (PostgreSQL).

-- Permite combinar igualdad (empleado_id) y rangos (&&) en una restricción EXCLUDE.
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- Secuencia para los ids V-AAAA-NNNN.
CREATE SEQUENCE IF NOT EXISTS vacaciones_seq;

-- Réplica local de empleados (decisión (b): alimentada por empleado.creado/actualizado/retirado).
CREATE TABLE IF NOT EXISTS empleados_replica (
    empleado_id     VARCHAR(20)  PRIMARY KEY,
    email           VARCHAR(150) NOT NULL,
    retirado        BOOLEAN      NOT NULL DEFAULT false,
    actualizado_en  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vacaciones (
    id              VARCHAR(20)  PRIMARY KEY,
    empleado_id     VARCHAR(20)  NOT NULL,
    -- Fecha y hora locales en la zona horaria del negocio (ZONA_HORARIA), Reto 5 D6.
    fecha_inicio    TIMESTAMP    NOT NULL,
    fecha_fin       TIMESTAMP    NOT NULL,
    estado          VARCHAR(12)  NOT NULL
                    CHECK (estado IN ('PROGRAMADA', 'EN_CURSO', 'FINALIZADA', 'CANCELADA')),
    fecha_creacion  TIMESTAMPTZ  NOT NULL,
    CONSTRAINT chk_fechas_coherentes CHECK (fecha_fin > fecha_inicio),
    -- Validación 3 garantizada por la BD: un empleado no puede tener dos períodos vigentes
    -- que se crucen, aunque dos solicitudes lleguen a la vez (red de seguridad del código).
    -- Rango semiabierto [inicio, fin): un período puede empezar justo cuando termina otro.
    CONSTRAINT excl_sin_solapamiento EXCLUDE USING gist (
        empleado_id WITH =,
        tsrange(fecha_inicio, fecha_fin, '[)') WITH &&
    ) WHERE (estado IN ('PROGRAMADA', 'EN_CURSO'))
);

CREATE INDEX IF NOT EXISTS idx_vacaciones_empleado_id ON vacaciones (empleado_id);
-- Consultas del scheduler (períodos por iniciar y por finalizar).
CREATE INDEX IF NOT EXISTS idx_vacaciones_estado_inicio ON vacaciones (estado, fecha_inicio);
CREATE INDEX IF NOT EXISTS idx_vacaciones_estado_fin ON vacaciones (estado, fecha_fin);

-- Deduplicación (Catálogo de Eventos, 2.1).
CREATE TABLE IF NOT EXISTS eventos_procesados (
    id            VARCHAR(100) PRIMARY KEY,
    procesado_en  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
