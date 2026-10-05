package com.talentflow.vacaciones.aplicacion;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.talentflow.vacaciones.dominio.ConflictoDeEstadoException;
import com.talentflow.vacaciones.dominio.EstadoVacaciones;
import com.talentflow.vacaciones.dominio.SolicitudInvalidaException;
import com.talentflow.vacaciones.dominio.Vacaciones;
import com.talentflow.vacaciones.dominio.VacacionesNoEncontradasException;
import java.time.Clock;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneId;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class ServicioVacacionesTest {

    // "Hoy" fijo: 28 de septiembre de 2026 en Bogotá.
    private static final Clock RELOJ = Clock.fixed(Instant.parse("2026-09-28T15:00:00Z"), ZoneId.of("America/Bogota"));

    private Fakes.Vacacionesmem vacaciones;
    private Fakes.EmpleadosMem empleados;
    private Fakes.PublicadorEspia publicador;
    private ServicioVacaciones servicio;

    @BeforeEach
    void preparar() {
        vacaciones = new Fakes.Vacacionesmem();
        empleados = new Fakes.EmpleadosMem();
        publicador = new Fakes.PublicadorEspia();
        empleados.registrar("E001", "juan@empresa.com");
        servicio = new ServicioVacaciones(vacaciones, empleados, publicador, new Fakes.SinTransaccion(), RELOJ);
    }

    private static SolicitudVacaciones solicitud(String empleadoId, String inicio, String fin) {
        return new SolicitudVacaciones(empleadoId, LocalDate.parse(inicio), LocalDate.parse(fin));
    }

    @Test
    void programaUnPeriodoYPublicaVacacionesProgramadasSegunElCatalogo() {
        Vacaciones creada = servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20"));

        assertThat(creada.id()).matches("V-\\d{4}-\\d{4}");
        assertThat(creada.estado()).isEqualTo(EstadoVacaciones.PROGRAMADA);
        assertThat(publicador.publicados).hasSize(1);
        assertThat(publicador.publicados.get(0).tipo()).isEqualTo("vacaciones.programadas");
        assertThat(publicador.publicados.get(0).data()).isEqualTo(new VacacionesProgramadas(
                creada.id(), "E001", "juan@empresa.com",
                LocalDate.parse("2026-12-14"), LocalDate.parse("2026-12-20"), 5));
    }

    @Test
    void validacion1FechasIncoherentes() {
        assertThatThrownBy(() -> servicio.programar(solicitud("E001", "2026-12-30", "2026-12-15")))
                .isInstanceOf(SolicitudInvalidaException.class);
        assertThat(publicador.publicados).isEmpty();
    }

    @Test
    void validacion2FechasEnElPasado() {
        assertThatThrownBy(() -> servicio.programar(solicitud("E001", "2026-06-15", "2026-06-30")))
                .isInstanceOf(SolicitudInvalidaException.class)
                .hasMessageContaining("anterior a hoy");
    }

    @Test
    void validacion3SolapamientoIncluyeElPeriodoEnConflicto() {
        Vacaciones existente = servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20"));

        assertThatThrownBy(() -> servicio.programar(solicitud("E001", "2026-12-18", "2027-01-05")))
                .isInstanceOfSatisfying(SolicitudInvalidaException.class,
                        e -> assertThat(e.periodoEnConflicto()).isEqualTo(existente));
    }

    @Test
    void unPeriodoCanceladoNoBloqueaFechas() {
        Vacaciones existente = servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20"));
        servicio.cancelar(existente.id());

        assertThat(servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20")).estado())
                .isEqualTo(EstadoVacaciones.PROGRAMADA);
    }

    @Test
    void validacion4EmpleadoInexistenteORetirado() {
        assertThatThrownBy(() -> servicio.programar(solicitud("NO-EXISTE", "2026-12-14", "2026-12-20")))
                .isInstanceOf(SolicitudInvalidaException.class)
                .hasMessageContaining("no corresponde a un empleado registrado");

        empleados.marcarRetirado("E001", "juan@empresa.com");
        assertThatThrownBy(() -> servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20")))
                .isInstanceOf(SolicitudInvalidaException.class)
                .hasMessageContaining("retirado");
    }

    @Test
    void siLaPublicacionFallaElPeriodoQuedaRegistrado() {
        publicador.exito = false;

        Vacaciones creada = servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20"));

        assertThat(servicio.consultar(creada.id())).isEqualTo(creada);
    }

    @Test
    void cancelaUnPeriodoFuturoSinBorrarlo() {
        Vacaciones creada = servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20"));

        assertThat(servicio.cancelar(creada.id()).estado()).isEqualTo(EstadoVacaciones.CANCELADA);
        assertThat(servicio.consultar(creada.id()).estado()).isEqualTo(EstadoVacaciones.CANCELADA);
    }

    @Test
    void noCancelaUnPeriodoQueYaInicioNiUnoYaCancelado() {
        Vacaciones deHoy = servicio.programar(solicitud("E001", "2026-09-28", "2026-10-05"));
        assertThatThrownBy(() -> servicio.cancelar(deHoy.id())).isInstanceOf(ConflictoDeEstadoException.class);

        Vacaciones futura = servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20"));
        servicio.cancelar(futura.id());
        assertThatThrownBy(() -> servicio.cancelar(futura.id())).isInstanceOf(ConflictoDeEstadoException.class);
    }

    @Test
    void consultarUnoInexistenteEs404() {
        assertThatThrownBy(() -> servicio.consultar("V-2026-9999")).isInstanceOf(VacacionesNoEncontradasException.class);
    }

    @Test
    void listaTodosOLosDeUnEmpleado() {
        empleados.registrar("E002", "ana@empresa.com");
        servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20"));
        servicio.programar(solicitud("E002", "2026-12-14", "2026-12-20"));

        assertThat(servicio.listar(null)).hasSize(2);
        assertThat(servicio.listar("E002")).extracting(Vacaciones::empleadoId).containsExactly("E002");
    }
}
