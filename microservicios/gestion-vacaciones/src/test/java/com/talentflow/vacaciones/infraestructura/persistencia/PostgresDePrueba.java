package com.talentflow.vacaciones.infraestructura.persistencia;

import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.boot.testcontainers.service.connection.ServiceConnection;
import org.springframework.context.annotation.Bean;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.utility.MountableFile;

/**
 * PostgreSQL 16 real para las pruebas de integración (Reto 6, D8): la misma imagen que el
 * Compose y el mismo init.sql montado en docker-entrypoint-initdb.d, así el esquema probado es
 * exactamente el de producción (sin DDL duplicado).
 *
 * Es un bean: Spring lo arranca una vez por contexto (compartido por todas las *IT gracias a la
 * caché de contextos) y lo detiene al cerrarlo; Ryuk lo elimina aunque la JVM muera.
 * {@code @ServiceConnection} configura el DataSource con su URL, usuario y contraseña.
 */
@TestConfiguration(proxyBeanMethods = false)
class PostgresDePrueba {

    @Bean
    @ServiceConnection
    PostgreSQLContainer<?> postgres() {
        return new PostgreSQLContainer<>("postgres:16-alpine")
                .withDatabaseName("db_vacaciones")
                .withCopyFileToContainer(MountableFile.forHostPath("init.sql"),
                        "/docker-entrypoint-initdb.d/01-init.sql");
    }
}
