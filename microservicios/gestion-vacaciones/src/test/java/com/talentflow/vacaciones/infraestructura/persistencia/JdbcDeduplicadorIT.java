package com.talentflow.vacaciones.infraestructura.persistencia;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.talentflow.vacaciones.aplicacion.ProcesadorEventosEmpleado;
import com.talentflow.vacaciones.aplicacion.Resultado;
import com.talentflow.vacaciones.dominio.EmpleadoReplica;
import java.nio.charset.StandardCharsets;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.dao.DataIntegrityViolationException;

/**
 * Deduplicación por el id del envelope (PRIMARY KEY + ON CONFLICT DO NOTHING) y su atomicidad
 * con el efecto, usando TransaccionesSpring, contra PostgreSQL 16 real.
 */
class JdbcDeduplicadorIT extends IntegracionPostgres {

    private JdbcDeduplicador deduplicador;
    private JdbcRepositorioEmpleados empleados;
    private TransaccionesSpring transacciones;

    @BeforeEach
    void preparar() {
        deduplicador = new JdbcDeduplicador(jdbc);
        empleados = new JdbcRepositorioEmpleados(jdbc);
        transacciones = new TransaccionesSpring(transaccion);
    }

    private long eventosRegistrados() {
        return jdbc.sql("SELECT count(*) FROM eventos_procesados").query(Long.class).single();
    }

    @Test
    void unIdNuevoSeRegistraYUnIdRepetidoSeDetecta() {
        assertThat(deduplicador.registrarSiEsNuevo("evt-1")).isTrue();
        assertThat(deduplicador.registrarSiEsNuevo("evt-1")).isFalse();
        assertThat(deduplicador.registrarSiEsNuevo("evt-2")).isTrue();
        assertThat(eventosRegistrados()).isEqualTo(2);
    }

    /** El mismo mensaje entregado dos veces a la vez: la segunda inserción espera y no inserta. */
    @Test
    void dosEntregasConcurrentesDelMismoIdSoloRegistranUna() throws Exception {
        Carrera<Boolean, Boolean> carrera = carrera(
                () -> deduplicador.registrarSiEsNuevo("evt-1"),
                () -> deduplicador.registrarSiEsNuevo("evt-1"));

        assertThat(carrera.primera()).isTrue();
        assertThat(carrera.segunda()).isFalse();
        assertThat(eventosRegistrados()).isEqualTo(1);
    }

    /** Si el efecto falla, el id tampoco queda registrado: el reintento se procesa de nuevo. */
    @Test
    void siElEfectoFallaElRegistroDelIdSeDeshace() {
        assertThatThrownBy(() -> transacciones.ejecutar(() -> {
            deduplicador.registrarSiEsNuevo("evt-1");
            throw new IllegalStateException("fallo del efecto");
        })).isInstanceOf(IllegalStateException.class);

        assertThat(eventosRegistrados()).isZero();
        assertThat(deduplicador.registrarSiEsNuevo("evt-1")).isTrue();
    }

    /** ProcesadorEventosEmpleado descarta ids de más de 100 caracteres: es el largo de la columna. */
    @Test
    void elLargoMaximoDelIdCoincideConElDeLaColumna() {
        assertThat(deduplicador.registrarSiEsNuevo("x".repeat(100))).isTrue();
        assertThatThrownBy(() -> deduplicador.registrarSiEsNuevo("x".repeat(101)))
                .isInstanceOf(DataIntegrityViolationException.class);
    }

    /**
     * De punta a punta con los adaptadores reales: el mismo envelope dos veces aplica su efecto
     * una vez. El predicado de rechazo permanente es el de Configuracion.
     */
    @Test
    void elProcesadorConLaBdRealAplicaUnEnvelopeRepetidoUnaSolaVez() {
        ProcesadorEventosEmpleado procesador = new ProcesadorEventosEmpleado(empleados, deduplicador,
                transacciones, e -> e instanceof DataIntegrityViolationException);
        byte[] creado = envelope("evt-1", "empleado.creado", "E001", "juan@empresa.com");

        assertThat(procesador.procesar(creado)).isEqualTo(Resultado.PROCESADO);
        empleados.actualizarEmail("E001", "cambiado@empresa.com");
        assertThat(procesador.procesar(creado)).isEqualTo(Resultado.DUPLICADO);

        assertThat(empleados.buscar("E001")).map(EmpleadoReplica::email).contains("cambiado@empresa.com");
    }

    /** Un dato que el esquema rechaza se descarta y no deja el id registrado (todo o nada). */
    @Test
    void unEventoQueElEsquemaRechazaSeDescartaSinRegistrarSuId() {
        ProcesadorEventosEmpleado procesador = new ProcesadorEventosEmpleado(empleados, deduplicador,
                transacciones, e -> e instanceof DataIntegrityViolationException);

        Resultado resultado = procesador.procesar(envelope("evt-1", "empleado.creado", "E".repeat(21), "juan@empresa.com"));

        assertThat(resultado).isEqualTo(Resultado.DESCARTADO);
        assertThat(eventosRegistrados()).isZero();
    }

    private static byte[] envelope(String id, String tipo, String empleadoId, String email) {
        return """
                {"id":"%s","type":"%s","version":1,"occurredAt":"2026-09-28T10:00:00-05:00",
                 "producer":"empleados-service","data":{"empleadoId":"%s","email":"%s"}}
                """.formatted(id, tipo, empleadoId, email).getBytes(StandardCharsets.UTF_8);
    }
}
