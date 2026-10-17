package com.talentflow.vacaciones.aplicacion;

import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.nio.charset.StandardCharsets;
import java.util.function.Consumer;
import java.util.stream.Stream;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;
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

    private static final ObjectMapper JSON = new ObjectMapper();

    /** Un empleado.creado válido con una sola alteración, para aislar cada regla del envelope. */
    private static byte[] alterado(Consumer<ObjectNode> alteracion) throws Exception {
        ObjectNode sobre = (ObjectNode) JSON.readTree(evento("e1", "empleado.creado", CREADO));
        alteracion.accept(sobre);
        return JSON.writeValueAsBytes(sobre);
    }

    private static Arguments caso(String nombre, Consumer<ObjectNode> alteracion) {
        return Arguments.of(nombre, alteracion);
    }

    private static ObjectNode data(ObjectNode sobre) {
        return (ObjectNode) sobre.get("data");
    }

    // Catálogo de Eventos, sección 2: todos los campos del envelope son obligatorios.
    static Stream<Arguments> mensajesInvalidos() {
        return Stream.of(
                caso("sin id", s -> s.remove("id")),
                caso("id en blanco", s -> s.put("id", "  ")),
                caso("id que no es texto", s -> s.put("id", 42)),
                caso("sin type", s -> s.remove("type")),
                caso("sin data", s -> s.remove("data")),
                caso("data que no es un objeto", s -> s.put("data", "E001")),
                caso("sin version", s -> s.remove("version")),
                caso("version 0", s -> s.put("version", 0)),
                caso("sin occurredAt", s -> s.remove("occurredAt")),
                caso("sin producer", s -> s.remove("producer")),
                caso("producer en blanco", s -> s.put("producer", "")),
                // Carga útil: la réplica necesita empleadoId y email.
                caso("data sin empleadoId", s -> data(s).remove("empleadoId")),
                caso("data con empleadoId en blanco", s -> data(s).put("empleadoId", " ")),
                caso("data con email en blanco", s -> data(s).put("email", "")),
                caso("data con empleadoId que no es texto", s -> data(s).putObject("empleadoId").put("valor", "E001")));
    }

    @ParameterizedTest(name = "[{index}] {0} → DESCARTADO")
    @MethodSource("mensajesInvalidos")
    void descartaElMensajeSinTocarLaReplicaNiConsumirSuId(String caso, Consumer<ObjectNode> alteracion) throws Exception {
        assertThat(procesador.procesar(alterado(alteracion))).isEqualTo(Resultado.DESCARTADO);

        assertThat(empleados.datos).isEmpty();
        assertThat(deduplicador.procesados).as("el id no queda registrado").isEmpty();
    }

    @Test
    void aceptaUnIdDeExactamenteLaLongitudDeLaColumna() throws Exception {
        String id = "x".repeat(ProcesadorEventosEmpleado.LONGITUD_MAXIMA_ID);

        assertThat(procesador.procesar(alterado(s -> s.put("id", id)))).isEqualTo(Resultado.PROCESADO);
        assertThat(deduplicador.procesados).containsExactly(id);
    }
}
