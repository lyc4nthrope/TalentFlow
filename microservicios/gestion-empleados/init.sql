-- Modelo de base de datos para el microservicio de gestión de empleados
-- Motor: PostgreSQL (ver docker-compose.yml -> database-empleados)

CREATE TABLE IF NOT EXISTS empleados (
    id                VARCHAR(20)  PRIMARY KEY,
    nombre            VARCHAR(100) NOT NULL,
    apellido          VARCHAR(100) NOT NULL,
    email             VARCHAR(150) NOT NULL UNIQUE,
    numero_empleado   VARCHAR(30)  NOT NULL UNIQUE,
    cargo             VARCHAR(100) NOT NULL,
    area              VARCHAR(100) NOT NULL,
    departamento_id   VARCHAR(20)  NOT NULL,
    fecha_ingreso     DATE         NOT NULL,
    estado            VARCHAR(20)  NOT NULL DEFAULT 'ACTIVO'
                       CHECK (estado IN ('ACTIVO', 'EN_VACACIONES', 'RETIRADO')),
    -- Estado de la verificación del departamento, independiente del ciclo de vida
    -- laboral (columna "estado" de arriba). PENDIENTE cuando el registro se aceptó
    -- con departamentos-service caído (Circuit Breaker abierto); se reconcilia a
    -- ACEPTADO o RECHAZADO automáticamente cuando el servicio se restablece
    -- (ver reconciliarPendientes() en services/empleados.service.js).
    validacion_departamento VARCHAR(20) NOT NULL DEFAULT 'ACEPTADO'
                       CHECK (validacion_departamento IN ('PENDIENTE', 'ACEPTADO', 'RECHAZADO')),
    creado_en         TIMESTAMP    NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_empleados_departamento_id ON empleados (departamento_id);
CREATE INDEX IF NOT EXISTS idx_empleados_email ON empleados (email);
CREATE INDEX IF NOT EXISTS idx_empleados_validacion_departamento ON empleados (validacion_departamento);