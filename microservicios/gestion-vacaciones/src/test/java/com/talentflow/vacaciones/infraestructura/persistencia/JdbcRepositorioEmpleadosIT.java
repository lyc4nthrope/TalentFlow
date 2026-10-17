package com.talentflow.vacaciones.infraestructura.persistencia;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.talentflow.vacaciones.dominio.EmpleadoReplica;
import java.time.OffsetDateTime;
import java.util.Optional;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.dao.DataIntegrityViolationException;

/** Réplica local de empleados (alimentada por eventos) contra PostgreSQL 16 real. */
class JdbcRepositorioEmpleadosIT extends IntegracionPostgres {

    private JdbcRepositorioEmpleados repositorio;

    @BeforeEach
    void preparar() {
        repositorio = new JdbcRepositorioEmpleados(jdbc);
    }

    private OffsetDateTime actualizadoEn(String empleadoId) {
        return jdbc.sql("SELECT actualizado_en FROM empleados_replica WHERE empleado_id = :id")
                .param("id", empleadoId).query(OffsetDateTime.class).single();
    }

    /** Retrasa actualizado_en un día: así se nota si la operación lo vuelve a fijar con now(). */
    private void envejecer(String empleadoId) {
        jdbc.sql("UPDATE empleados_replica SET actualizado_en = actualizado_en - interval '1 day' WHERE empleado_id = :id")
                .param("id", empleadoId).update();
    }

    @Test
    void registraUnEmpleadoActivoConLosValoresPorDefectoDelEsquema() {
        repositorio.registrar("E001", "juan@empresa.com");

        assertThat(repositorio.buscar("E001")).contains(new EmpleadoReplica("E001", "juan@empresa.com", false));
        assertThat(actualizadoEn("E001")).isNotNull();
    }

    @Test
    void unEmpleadoCreadoRepetidoNoPisaLaReplica() {
        repositorio.registrar("E001", "juan@empresa.com");

        repositorio.registrar("E001", "otro@empresa.com");

        assertThat(repositorio.buscar("E001")).map(EmpleadoReplica::email).contains("juan@empresa.com");
    }

    @Test
    void actualizarEmailCambiaElEmailYLaFechaDeActualizacion() {
        repositorio.registrar("E001", "juan@empresa.com");
        envejecer("E001");
        OffsetDateTime antes = actualizadoEn("E001");

        repositorio.actualizarEmail("E001", "juan.perez@empresa.com");

        assertThat(repositorio.buscar("E001")).map(EmpleadoReplica::email).contains("juan.perez@empresa.com");
        assertThat(actualizadoEn("E001")).isAfter(antes);
    }

    /** Si se perdió el empleado.creado, el actualizado lo registra igual (upsert). */
    @Test
    void actualizarEmailDeUnEmpleadoDesconocidoLoRegistra() {
        repositorio.actualizarEmail("E002", "ana@empresa.com");

        assertThat(repositorio.buscar("E002")).contains(new EmpleadoReplica("E002", "ana@empresa.com", false));
    }

    @Test
    void marcarRetiradoConservaElEmailRegistradoYActualizaLaFecha() {
        repositorio.registrar("E001", "juan@empresa.com");
        envejecer("E001");
        OffsetDateTime antes = actualizadoEn("E001");

        repositorio.marcarRetirado("E001", "otro@empresa.com");

        assertThat(repositorio.buscar("E001")).contains(new EmpleadoReplica("E001", "juan@empresa.com", true));
        assertThat(actualizadoEn("E001")).isAfter(antes);
    }

    @Test
    void marcarRetiradoDeUnEmpleadoDesconocidoLoRegistraYaRetirado() {
        repositorio.marcarRetirado("E003", "luis@empresa.com");

        assertThat(repositorio.buscar("E003")).contains(new EmpleadoReplica("E003", "luis@empresa.com", true));
    }

    @Test
    void buscarYBuscarParaProgramarDevuelvenVacioSiNoExiste() {
        assertThat(repositorio.buscar("E999")).isEmpty();
        Optional<EmpleadoReplica> paraProgramar = transaccion.execute(estado -> repositorio.buscarParaProgramar("E999"));
        assertThat(paraProgramar).isEmpty();
    }

    /**
     * FOR UPDATE: dos solicitudes simultáneas del mismo empleado se validan en serie; la segunda
     * espera a que la primera confirme y ve su resultado (aquí, el retiro).
     */
    @Test
    void buscarParaProgramarBloqueaAlEmpleadoHastaQueTermineLaTransaccion() throws Exception {
        repositorio.registrar("E001", "juan@empresa.com");

        Carrera<Optional<EmpleadoReplica>, Optional<EmpleadoReplica>> carrera = carrera(
                () -> {
                    Optional<EmpleadoReplica> leido = repositorio.buscarParaProgramar("E001");
                    repositorio.marcarRetirado("E001", "juan@empresa.com");
                    return leido;
                },
                () -> repositorio.buscarParaProgramar("E001"));

        assertThat(carrera.primera()).map(EmpleadoReplica::retirado).contains(false);
        assertThat(carrera.segunda()).map(EmpleadoReplica::retirado).contains(true);
    }

    /**
     * ProcesadorEventosEmpleado no valida longitudes: confía en que la BD rechace los datos
     * fuera del esquema y en que Spring lo traduzca a DataIntegrityViolationException (rechazo
     * permanente, ver Configuracion). Aquí se comprueba esa suposición.
     */
    @Test
    void losDatosFueraDelEsquemaLleganComoDataIntegrityViolationException() {
        String idLargo = "E".repeat(21); // empleado_id VARCHAR(20)
        String emailLargo = "a".repeat(140) + "@empresa.com"; // email VARCHAR(150)

        assertThatThrownBy(() -> repositorio.registrar(idLargo, "juan@empresa.com"))
                .isInstanceOf(DataIntegrityViolationException.class);
        assertThatThrownBy(() -> repositorio.actualizarEmail("E001", emailLargo))
                .isInstanceOf(DataIntegrityViolationException.class);
        assertThatThrownBy(() -> repositorio.marcarRetirado("E001", null))
                .isInstanceOf(DataIntegrityViolationException.class);
        assertThat(repositorio.buscar("E001")).isEmpty();
    }
}
