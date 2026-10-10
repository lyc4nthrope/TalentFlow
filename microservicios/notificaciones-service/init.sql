-- Modelo de base de datos del microservicio de notificaciones
-- Motor: PostgreSQL (ver docker-compose.yml -> database-notificaciones)

CREATE TABLE IF NOT EXISTS notificaciones (
    id            VARCHAR(36)  PRIMARY KEY,
    tipo          VARCHAR(20)  NOT NULL
                  CHECK (tipo IN ('BIENVENIDA', 'DESVINCULACION', 'VACACIONES')),
    destinatario  VARCHAR(150) NOT NULL,
    mensaje       TEXT         NOT NULL,
    fecha_envio   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    empleado_id   VARCHAR(20)  NOT NULL,
    -- Mensaje (envelope.id) que originó la notificación. UNIQUE = segunda red de seguridad
    -- contra duplicados, además de la tabla eventos_procesados.
    evento_id     VARCHAR(64)  NOT NULL UNIQUE
);

CREATE INDEX IF NOT EXISTS idx_notificaciones_empleado ON notificaciones (empleado_id, fecha_envio);

-- Deduplicación de eventos por id de mensaje (Catálogo de Eventos, sección 2.1).
CREATE TABLE IF NOT EXISTS eventos_procesados (
    id            VARCHAR(64)  PRIMARY KEY,
    procesado_en  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
