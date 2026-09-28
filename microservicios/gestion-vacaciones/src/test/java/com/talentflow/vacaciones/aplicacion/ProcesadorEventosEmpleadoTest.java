package com.talentflow.vacaciones.aplicacion;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.charset.StandardCharsets;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.dao.DataAccessResourceFailureException;

class ProcesadorEventosEmpleadoTest {

    private Fakes.EmpleadosMem empleados;
    private Fakes.DeduplicadorMem deduplicador;
    private ProcesadorEventosEmpleado procesador;

    @BeforeEach
    void preparar() {
        empleados = new Fakes.EmpleadosMem();
        deduplicador = new Fakes.DeduplicadorMem();
        procesador = new ProcesadorEventosEmpleado(empleados, deduplicador, new Fakes.SinTransaccion(),
                e -> e instanceof DataIntegrityViolationException);
    }

    private static byte[] evento(String id, String tipo, String data) {
        return ("{\"id\":\"" + id + "\",\"type\":\"" + tipo + "\",\"version\":1,\"occurredAt\":\"2026-09-28T15:00:00Z\","
                + "\"producer\":\"empleados-service\",\"data\":" + data + "}").getBytes(StandardCharsets.UTF_8);
    }

    private static final String CREADO = "{\"empleadoId\":\"E001\",\"nombre\":\"Juan\",\"apellido\":\"Pérez\","
            + "\"email\":\"juan@empresa.com\",\"numeroEmpleado\":\"EMP-1\",\"cargo\":\"Dev\",\"area\":\"TI\","
            + "\"departamentoId\":\"IT\",\"fechaIngreso\":\"2026-03-01\",\"estado\":\"ACTIVO\"}";

    @Test
    void creadoRegistraAlEmpleadoEnLaReplica() {
        assertThat(procesador.procesar(evento("e1", "empleado.creado", CREADO))).isEqualTo(Resultado.PROCESADO);
        assertThat(empleados.datos.get("E001").email()).isEqualTo("juan@empresa.com");
        assertThat(empleados.datos.get("E001").retirado()).isFalse();
    }

    @Test
    void actualizadoMantieneElEmailAlDiaYRetiradoLoMarca() {
        procesador.procesar(evento("e1", "empleado.creado", CREADO));
        procesador.procesar(evento("e2", "empleado.actualizado",
                "{\"empleadoId\":\"E001\",\"nombre\":\"Juan\",\"apellido\":\"P\",\"email\":\"nuevo@empresa.com\","
                        + "\"cargo\":\"Dev\",\"area\":\"TI\",\"departamentoId\":\"IT\"}"));
        procesador.procesar(evento("e3", "empleado.retirado",
                "{\"empleadoId\":\"E001\",\"email\":\"nuevo@empresa.com\",\"fechaRetiro\":\"2026-11-30T16:45:00Z\","
                        + "\"motivo\":\"RENUNCIA\"}"));

        assertThat(empleados.datos.get("E001").email()).isEqualTo("nuevo@empresa.com");
        assertThat(empleados.datos.get("E001").retirado()).isTrue();
    }

    @Test
    void elMismoEventoDosVecesTieneUnSoloEfecto() {
        assertThat(procesador.procesar(evento("mismo", "empleado.creado", CREADO))).isEqualTo(Resultado.PROCESADO);
        empleados.marcarRetirado("E001", "juan@empresa.com");

        assertThat(procesador.procesar(evento("mismo", "empleado.creado", CREADO))).isEqualTo(Resultado.DUPLICADO);
        assertThat(empleados.datos.get("E001").retirado()).as("el duplicado no reaplica nada").isTrue();
    }

    @Test
    void descartaMensajesQueNuncaPodranProcesarse() {
        assertThat(procesador.procesar("{roto".getBytes(StandardCharsets.UTF_8))).isEqualTo(Resultado.DESCARTADO);
        assertThat(procesador.procesar(evento("e1", "vacaciones.programadas", "{}"))).isEqualTo(Resultado.DESCARTADO);
        assertThat(procesador.procesar(evento("e2", "empleado.creado", "{\"empleadoId\":\"E1\"}")))
                .isEqualTo(Resultado.DESCARTADO);
        assertThat(procesador.procesar(evento("x".repeat(101), "empleado.creado", CREADO))).isEqualTo(Resultado.DESCARTADO);
        assertThat(empleados.datos).isEmpty();
    }

    @Test
    void clasificaLosFallosDeLaBd() {
        deduplicador.fallo = new DataAccessResourceFailureException("BD caída");
        assertThat(procesador.procesar(evento("e1", "empleado.creado", CREADO))).isEqualTo(Resultado.ERROR_TRANSITORIO);

        deduplicador.fallo = new DataIntegrityViolationException("value too long");
        assertThat(procesador.procesar(evento("e2", "empleado.creado", CREADO))).isEqualTo(Resultado.DESCARTADO);
    }
}
