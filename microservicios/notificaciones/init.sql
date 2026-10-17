-- Base de datos propia del servicio de notificaciones (PostgreSQL).

-- Deduplicación (Catálogo de Eventos, 2.1): id de cada mensaje ya procesado.
-- La PRIMARY KEY es la que garantiza que un mismo id no se procese dos veces, incluso
-- con dos consumidores procesando el mismo mensaje al mismo tiempo.
CREATE TABLE IF NOT EXISTS eventos_procesados (
    id            VARCHAR(100) PRIMARY KEY,
    procesado_en  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Historial de notificaciones enviadas (reto4.pdf, "Estructura de una notificación").
-- Reto 5: tipos SEGURIDAD y CUENTA (formato del reto5.pdf) además de VACACIONES. Los
-- tipos BIENVENIDA y DESVINCULACION del Reto 4 ya no se escriben. empleado_id es
-- nulo cuando el evento no lo trae (usuario.recuperacion solo trae el email, D20).
-- Un volumen existente debe recrearse: este script solo corre con un volumen nuevo.
CREATE TABLE IF NOT EXISTS notificaciones (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tipo          VARCHAR(20)  NOT NULL
                  CHECK (tipo IN ('SEGURIDAD', 'CUENTA', 'VACACIONES')),
    destinatario  VARCHAR(150) NOT NULL,
    mensaje       TEXT         NOT NULL,
    fecha_envio   TIMESTAMPTZ  NOT NULL,
    empleado_id   VARCHAR(20),
    -- Trazabilidad: qué evento originó cada notificación (y a lo sumo una por evento).
    evento_id     VARCHAR(100) NOT NULL UNIQUE REFERENCES eventos_procesados (id)
);

CREATE INDEX IF NOT EXISTS idx_notificaciones_empleado_id ON notificaciones (empleado_id);
