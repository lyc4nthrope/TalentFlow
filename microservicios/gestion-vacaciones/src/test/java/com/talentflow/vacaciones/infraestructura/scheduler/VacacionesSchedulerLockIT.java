package com.talentflow.vacaciones.infraestructura.scheduler;

import static org.assertj.core.api.Assertions.assertThat;

import com.talentflow.vacaciones.aplicacion.ProcesadorCicloVacaciones;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import javax.sql.DataSource;
import net.javacrumbs.shedlock.core.LockProvider;
import net.javacrumbs.shedlock.provider.jdbctemplate.JdbcTemplateLockProvider;
import net.javacrumbs.shedlock.spring.annotation.EnableSchedulerLock;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.jdbc.AutoConfigureTestDatabase;
import org.springframework.boot.test.autoconfigure.jdbc.JdbcTest;
import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.boot.testcontainers.service.connection.ServiceConnection;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Import;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.simple.JdbcClient;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.utility.MountableFile;

/**
 * Verifica contra PostgreSQL 16 real (Testcontainers) que dos ejecuciones simultáneas de
 * {@link VacacionesScheduler} (simulando 2 réplicas del servicio) compiten por el candado de
 * ShedLock y solo una ejecuta el ciclo.
 */
@JdbcTest
@AutoConfigureTestDatabase(replace = AutoConfigureTestDatabase.Replace.NONE)
@Import(VacacionesSchedulerLockIT.ConfiguracionLockPrueba.class)
@Transactional(propagation = Propagation.NOT_SUPPORTED)
class VacacionesSchedulerLockIT {

    @Autowired
    private VacacionesScheduler schedulerReplica1;

    @Autowired
    private VacacionesScheduler schedulerReplica2;

    @Autowired
    private ProcesadorConcurrenteEspia cicloEspia;

    @Autowired
    private JdbcClient jdbc;

    @BeforeEach
    void limpiarCandados() {
        jdbc.sql("TRUNCATE shedlock").update();
        cicloEspia.reiniciar();
    }

    @Test
    void dosEjecucionesSimultaneasSoloProcesanUnaVez() throws Exception {
        CompletableFuture<Void> primera = CompletableFuture.runAsync(schedulerReplica1::ejecutar);
        assertThat(cicloEspia.enEjecucion.await(5, TimeUnit.SECONDS)).isTrue();

        CompletableFuture<Void> segunda = CompletableFuture.runAsync(schedulerReplica2::ejecutar);
        segunda.get(5, TimeUnit.SECONDS);

        cicloEspia.liberar.countDown();
        primera.get(5, TimeUnit.SECONDS);

        assertThat(cicloEspia.ejecuciones.get()).isEqualTo(1);
    }

    static class ProcesadorConcurrenteEspia extends ProcesadorCicloVacaciones {
        final AtomicInteger ejecuciones = new AtomicInteger();
        volatile CountDownLatch enEjecucion = new CountDownLatch(1);
        volatile CountDownLatch liberar = new CountDownLatch(1);

        ProcesadorConcurrenteEspia() {
            super(null, null, null, null, null);
        }

        void reiniciar() {
            ejecuciones.set(0);
            enEjecucion = new CountDownLatch(1);
            liberar = new CountDownLatch(1);
        }

        @Override
        public Resumen ejecutar() {
            ejecuciones.incrementAndGet();
            enEjecucion.countDown();
            try {
                if (!liberar.await(5, TimeUnit.SECONDS)) {
                    throw new IllegalStateException("tiempo agotado esperando liberar la primera réplica");
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw new IllegalStateException(e);
            }
            return new Resumen(0, 0, 0);
        }
    }

    @TestConfiguration(proxyBeanMethods = false)
    @EnableSchedulerLock(defaultLockAtMostFor = "PT50S")
    static class ConfiguracionLockPrueba {

        @Bean
        @ServiceConnection
        PostgreSQLContainer<?> postgres() {
            return new PostgreSQLContainer<>("postgres:16-alpine")
                    .withDatabaseName("db_vacaciones")
                    .withCopyFileToContainer(MountableFile.forHostPath("init.sql"),
                            "/docker-entrypoint-initdb.d/01-init.sql");
        }

        @Bean
        LockProvider lockProvider(DataSource dataSource) {
            return new JdbcTemplateLockProvider(
                    JdbcTemplateLockProvider.Configuration.builder()
                            .withJdbcTemplate(new JdbcTemplate(dataSource))
                            .usingDbTime()
                            .build());
        }

        @Bean
        ProcesadorConcurrenteEspia cicloEspia() {
            return new ProcesadorConcurrenteEspia();
        }

        @Bean
        VacacionesScheduler schedulerReplica1(ProcesadorConcurrenteEspia cicloEspia) {
            return new VacacionesScheduler(cicloEspia);
        }

        @Bean
        VacacionesScheduler schedulerReplica2(ProcesadorConcurrenteEspia cicloEspia) {
            return new VacacionesScheduler(cicloEspia);
        }
    }
}
