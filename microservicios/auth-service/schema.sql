-- Base de datos propia del auth-service (PostgreSQL). Reto 5.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS usuarios (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- "sub" del JWT: el empleadoId de un empleado o 'admin' para el administrador semilla.
    sujeto           VARCHAR(36)  NOT NULL UNIQUE,
    empleado_id      VARCHAR(36)  UNIQUE,            -- NULL para el administrador
    email            VARCHAR(255) NOT NULL UNIQUE,   -- se guarda en minúsculas
    -- NULL mientras la cuenta no se activa: nunca hay contraseña temporal (catálogo 3.4).
    password_hash    VARCHAR(255),
    rol              VARCHAR(10)  NOT NULL CHECK (rol IN ('ADMIN', 'USER')),
    -- Reto 5, D3: la suspensión temporal y la baja permanente son estados distintos.
    estado           VARCHAR(30)  NOT NULL CHECK (estado IN
                     ('PENDIENTE_ACTIVACION', 'ACTIVA', 'SUSPENDIDA_TEMPORAL', 'DESACTIVADA_PERMANENTE')),
    -- Un único token activo (activación o recuperación); solo se guarda su SHA-256 (D1).
    token_hash       CHAR(64)     UNIQUE,
    token_tipo       VARCHAR(12)  CHECK (token_tipo IN ('ACTIVACION', 'RECUPERACION')),
    token_expira_en  TIMESTAMPTZ,
    -- D18: id de las vacaciones que suspendieron la cuenta. vacaciones.finalizadas solo
    -- reactiva si trae este mismo id (períodos contiguos: el fin de A no reactiva durante B).
    vacaciones_suspension VARCHAR(20),
    creado_en        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    actualizado_en   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT chk_token_completo CHECK (
        (token_hash IS NULL AND token_tipo IS NULL AND token_expira_en IS NULL) OR
        (token_hash IS NOT NULL AND token_tipo IS NOT NULL AND token_expira_en IS NOT NULL)),
    CONSTRAINT chk_activa_con_password CHECK (estado <> 'ACTIVA' OR password_hash IS NOT NULL),
    CONSTRAINT chk_suspension_solo_suspendida CHECK (
        vacaciones_suspension IS NULL OR estado = 'SUSPENDIDA_TEMPORAL')
);

-- Deduplicación (Catálogo de Eventos 2.1, D4): el id del envelope se registra en la
-- misma transacción que el efecto; un mensaje repetido no vuelve a aplicarse.
CREATE TABLE IF NOT EXISTS eventos_procesados (
    id            VARCHAR(100) PRIMARY KEY,
    procesado_en  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- El administrador semilla lo crea el servicio al arrancar desde ADMIN_EMAIL /
-- ADMIN_PASSWORD (D15): ninguna credencial queda escrita en este archivo.
