-- Base de datos propia del servicio de notificaciones (PostgreSQL).

-- Deduplicación (Catálogo de Eventos, 2.1): id de cada mensaje ya procesado.
-- La PRIMARY KEY es la que garantiza que un mismo id no se procese dos veces, incluso
-- con dos consumidores procesando el mismo mensaje al mismo tiempo.
CREATE TABLE IF NOT EXISTS eventos_procesados (
    id            VARCHAR(100) PRIMARY KEY,
    procesado_en  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Historial de notificaciones enviadas (reto4.pdf, "Estructura de una notificación").
CREATE TABLE IF NOT EXISTS notificaciones (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tipo          VARCHAR(20)  NOT NULL
                  CHECK (tipo IN ('BIENVENIDA', 'DESVINCULACION', 'VACACIONES')),
    destinatario  VARCHAR(150) NOT NULL,
    mensaje       TEXT         NOT NULL,
    fecha_envio   TIMESTAMPTZ  NOT NULL,
    empleado_id   VARCHAR(20)  NOT NULL,
    -- Trazabilidad: qué evento originó cada notificación (y a lo sumo una por evento).
    evento_id     VARCHAR(100) NOT NULL UNIQUE REFERENCES eventos_procesados (id)
);

CREATE INDEX IF NOT EXISTS idx_notificaciones_empleado_id ON notificaciones (empleado_id);
