package com.talentflow.vacaciones.infraestructura.persistencia;

import java.time.Duration;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.function.Supplier;
import org.junit.jupiter.api.BeforeEach;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.jdbc.AutoConfigureTestDatabase;
import org.springframework.boot.test.autoconfigure.jdbc.JdbcTest;
import org.springframework.context.annotation.Import;
import org.springframework.jdbc.core.simple.JdbcClient;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Base de las pruebas de integración de la persistencia: DataSource, JdbcClient y
 * TransactionTemplate reales contra el contenedor de {@link PostgresDePrueba}.
 *
 * Sin la transacción por prueba de @JdbcTest (NOT_SUPPORTED): cada operación confirma como en
 * producción, lo que permite probar dos transacciones concurrentes. Por eso se vacían las tablas
 * antes de cada prueba.
 */
@JdbcTest
@AutoConfigureTestDatabase(replace = AutoConfigureTestDatabase.Replace.NONE)
@Import(PostgresDePrueba.class)
@Transactional(propagation = Propagation.NOT_SUPPORTED)
abstract class IntegracionPostgres {

    private static final Duration ESPERA_MAXIMA = Duration.ofSeconds(10);

    @Autowired
    protected JdbcClient jdbc;

    @Autowired
    protected TransactionTemplate transaccion;

    @BeforeEach
    void vaciarTablas() {
        jdbc.sql("TRUNCATE vacaciones, empleados_replica, eventos_procesados").update();
    }

    /** Resultado de dos transacciones que compiten por las mismas filas. */
    protected record Carrera<A, B>(A primera, B segunda) {
    }

    /**
     * Ejecuta la primera transacción hasta justo antes de confirmar, lanza la segunda, espera a
     * que PostgreSQL la tenga bloqueada por las filas de la primera y solo entonces confirma la
     * primera. Así la carrera es determinista: la segunda siempre ve el resultado confirmado.
     */
    protected <A, B> Carrera<A, B> carrera(Supplier<A> primera, Supplier<B> segunda) throws Exception {
        CountDownLatch primeraLista = new CountDownLatch(1);
        CountDownLatch confirmarPrimera = new CountDownLatch(1);
        CompletableFuture<A> resultadoPrimera = CompletableFuture.supplyAsync(() -> transaccion.execute(estado -> {
            A valor = primera.get();
            primeraLista.countDown();
            esperar(confirmarPrimera);
            return valor;
        }));
        if (!primeraLista.await(ESPERA_MAXIMA.toSeconds(), TimeUnit.SECONDS)) {
            throw new IllegalStateException("la primera transacción no llegó a ejecutarse");
        }
        CompletableFuture<B> resultadoSegunda = CompletableFuture.supplyAsync(
                () -> transaccion.execute(estado -> segunda.get()));
        try {
            esperarTransaccionBloqueada();
        } finally {
            confirmarPrimera.countDown();
        }
        return new Carrera<>(resultadoPrimera.get(ESPERA_MAXIMA.toSeconds(), TimeUnit.SECONDS),
                resultadoSegunda.get(ESPERA_MAXIMA.toSeconds(), TimeUnit.SECONDS));
    }

    private void esperarTransaccionBloqueada() throws InterruptedException {
        long limite = System.nanoTime() + ESPERA_MAXIMA.toNanos();
        while (jdbc.sql("SELECT count(*) FROM pg_locks WHERE NOT granted").query(Long.class).single() == 0) {
            if (System.nanoTime() > limite) {
                throw new IllegalStateException("la segunda transacción nunca quedó esperando un bloqueo");
            }
            Thread.sleep(20);
        }
    }

    private static void esperar(CountDownLatch senal) {
        try {
            if (!senal.await(ESPERA_MAXIMA.toSeconds(), TimeUnit.SECONDS)) {
                throw new IllegalStateException("tiempo agotado esperando la señal de confirmar");
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new IllegalStateException(e);
        }
    }
}
