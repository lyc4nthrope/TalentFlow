-- Modelo de base de datos del microservicio de gestión de perfiles
-- Motor: PostgreSQL (ver docker-compose.yml -> database-perfiles)

CREATE TABLE IF NOT EXISTS perfiles (
    id                  VARCHAR(36)   PRIMARY KEY,
    empleado_id         VARCHAR(20)   NOT NULL UNIQUE,
    -- Campos replicados desde empleados-service (se actualizan SOLO por eventos):
    nombre              VARCHAR(100)  NOT NULL DEFAULT '',
    apellido            VARCHAR(100)  NOT NULL DEFAULT '',
    email               VARCHAR(150)  NOT NULL DEFAULT '',
    cargo               VARCHAR(100)  NOT NULL DEFAULT '',
    area                VARCHAR(100)  NOT NULL DEFAULT '',
    departamento_id     VARCHAR(20)   NOT NULL DEFAULT '',
    -- Campos propios del perfil (editables por REST):
    telefono            VARCHAR(30)   NOT NULL DEFAULT '',
    direccion           VARCHAR(200)  NOT NULL DEFAULT '',
    ciudad              VARCHAR(100)  NOT NULL DEFAULT '',
    biografia           VARCHAR(2000) NOT NULL DEFAULT '',
    -- Archivado lógico (empleado.retirado): el perfil nunca se borra.
    archivado           BOOLEAN       NOT NULL DEFAULT FALSE,
    fecha_archivado     TIMESTAMPTZ,
    fecha_creacion      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    fecha_actualizacion TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_perfiles_archivado ON perfiles (archivado);

-- Deduplicación de eventos por id de mensaje (Catálogo de Eventos, sección 2.1).
CREATE TABLE IF NOT EXISTS eventos_procesados (
    id            VARCHAR(64)  PRIMARY KEY,
    procesado_en  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
