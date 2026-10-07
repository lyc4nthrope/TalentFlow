CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS usuarios (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email               VARCHAR(255) NOT NULL UNIQUE,
    password_hash       VARCHAR(255) NOT NULL,
    roles               TEXT[] NOT NULL DEFAULT ARRAY['USER'],
    estado              VARCHAR(30) NOT NULL DEFAULT 'INACTIVA',
    empleado_id         VARCHAR(36),
    token_recuperacion  VARCHAR(255),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_usuarios_email ON usuarios (email);
CREATE INDEX IF NOT EXISTS idx_usuarios_estado ON usuarios (estado);

INSERT INTO usuarios (email, password_hash, roles, estado, empleado_id, token_recuperacion, created_at, updated_at)
VALUES (
    'admin@empresa.com',
    '$2a$10$DMGkwrNmvD0BsaM8Se.Gb.TtZQMKIH6DrhEMjBLf0QWs9if9NtSR.',
    ARRAY['ADMIN'],
    'ACTIVA',
    NULL,
    NULL,
    NOW(),
    NOW()
)
ON CONFLICT (email) DO NOTHING;