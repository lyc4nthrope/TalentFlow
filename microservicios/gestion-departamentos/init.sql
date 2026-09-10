-- Modelo de base de datos para el microservicio de gestión de departamentos
-- Motor: MySQL (ver docker-compose.yml -> database-departamentos)

CREATE TABLE IF NOT EXISTS departamentos (
    id          VARCHAR(20)  PRIMARY KEY,
    nombre      VARCHAR(100) NOT NULL,
    descripcion VARCHAR(255),
    creado_en   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);