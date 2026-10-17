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
import java.time.LocalDateTime;
import java.time.ZoneId;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;

class ServicioVacacionesTest {

    // "Ahora" fijo: 28 de septiembre de 2026, 10:00 en Bogotá (15:00 UTC).
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

    /** Mismas reglas que el cuerpo HTTP: solo fecha = inicio a las 00:00, fin a las 23:59:59. */
    private static SolicitudVacaciones solicitud(String empleadoId, String inicio, String fin) {
        return new SolicitudVacaciones(empleadoId, FechaHoraEntrada.comoInicio(inicio).orElseThrow(),
                FechaHoraEntrada.comoFin(fin).orElseThrow());
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
                "2026-12-14T00:00:00-05:00", "2026-12-20T23:59:59-05:00", 5));
    }

    @Test
    void programaConHoraUnPeriodoDeMinutosQueEmpiezaHoy() {
        Vacaciones creada = servicio.programar(solicitud("E001", "2026-09-28T10:02", "2026-09-28T10:05"));

        assertThat(creada.fechaInicio()).isEqualTo(LocalDateTime.of(2026, 9, 28, 10, 2));
        assertThat(creada.fechaFin()).isEqualTo(LocalDateTime.of(2026, 9, 28, 10, 5));
        assertThat(((VacacionesProgramadas) publicador.publicados.get(0).data()).diasHabiles()).isEqualTo(1);
    }

    @Test
    void conLaFechaSolaHoyEsValidoYElPeriodoEmpiezaAhora() {
        // Como en el Reto 4 ("hoy es válido"): ahora son las 10:00 del 28 de septiembre.
        Vacaciones creada = servicio.programar(solicitud("E001", "2026-09-28", "2026-10-02"));

        assertThat(creada.fechaInicio()).isEqualTo(LocalDateTime.of(2026, 9, 28, 10, 0));
        assertThat(creada.fechaFin()).isEqualTo(LocalDateTime.of(2026, 10, 2, 23, 59, 59));
        assertThat(((VacacionesProgramadas) publicador.publicados.get(0).data()).fechaInicio())
                .isEqualTo("2026-09-28T10:00:00-05:00");
    }

    @Test
    void conLaFechaSolaAyerSigueSiendoPasado() {
        assertThatThrownBy(() -> servicio.programar(solicitud("E001", "2026-09-27", "2026-10-02")))
                .isInstanceOf(SolicitudInvalidaException.class)
                .hasMessageContaining("pasado");
        assertThat(publicador.publicados).isEmpty();
    }

    @Test
    void unMismoDiaConLaFechaSolaEsUnPeriodoDeUnDia() {
        // Cambio deliberado frente al Reto 4 (allí fechaFin = fechaInicio era 400): con fecha sola,
        // el fin es el último segundo del día, así que un día completo es válido (paso 12 del Reto 5).
        Vacaciones creada = servicio.programar(solicitud("E001", "2026-12-14", "2026-12-14"));

        assertThat(creada.fechaInicio()).isEqualTo(LocalDateTime.of(2026, 12, 14, 0, 0));
        assertThat(creada.fechaFin()).isEqualTo(LocalDateTime.of(2026, 12, 14, 23, 59, 59));
        assertThat(((VacacionesProgramadas) publicador.publicados.get(0).data()).diasHabiles()).isEqualTo(1);
    }

    @Test
    void publicaVacacionesProgramadasDespuesDeCerrarLaTransaccion() {
        Fakes.TransaccionesRastreadas transacciones = new Fakes.TransaccionesRastreadas();
        publicador.transacciones = transacciones;
        servicio = new ServicioVacaciones(vacaciones, empleados, publicador, transacciones, RELOJ);

        servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20"));

        assertThat(transacciones.abiertas).isEqualTo(1);
        assertThat(publicador.dentroDeTransaccionAlPublicar).containsExactly(false);
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
                .hasMessageContaining("pasado");
        // Hoy, pero una hora antes de ahora: también es pasado.
        assertThatThrownBy(() -> servicio.programar(solicitud("E001", "2026-09-28T09:00", "2026-09-28T18:00")))
                .isInstanceOf(SolicitudInvalidaException.class);
        assertThat(publicador.publicados).isEmpty();
    }

    @Test
    void validacion3SolapamientoIncluyeElPeriodoEnConflicto() {
        Vacaciones existente = servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20"));

        assertThatThrownBy(() -> servicio.programar(solicitud("E001", "2026-12-18", "2027-01-05")))
                .isInstanceOfSatisfying(SolicitudInvalidaException.class,
                        e -> assertThat(e.periodoEnConflicto()).isEqualTo(existente));
    }

    // Validación 3 con la fecha sola (formato del Reto 4): el existente va del 14 (00:00) al 20 (23:59:59).
    @ParameterizedTest(name = "[{index}] {0}: {1} a {2} → aceptado: {3}")
    @CsvSource(delimiter = '|', textBlock = """
            empieza el día en que termina el existente | 2026-12-20 | 2026-12-27 | false
            empieza el día siguiente                   | 2026-12-21 | 2026-12-27 | true
            termina el día en que empieza el existente | 2026-12-07 | 2026-12-14 | false
            termina el día anterior                    | 2026-12-07 | 2026-12-13 | true
            mismas fechas                              | 2026-12-14 | 2026-12-20 | false
            """)
    void conLaFechaSolaTocarseUnDiaEsCruce(String caso, String inicio, String fin, boolean aceptado) {
        Vacaciones existente = servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20"));

        if (aceptado) {
            assertThat(servicio.programar(solicitud("E001", inicio, fin)).estado()).isEqualTo(EstadoVacaciones.PROGRAMADA);
            assertThat(publicador.publicados).hasSize(2);
        } else {
            assertThatThrownBy(() -> servicio.programar(solicitud("E001", inicio, fin)))
                    .isInstanceOfSatisfying(SolicitudInvalidaException.class, e -> {
                        assertThat(e.campo()).isEqualTo("fechaInicio");
                        assertThat(e.periodoEnConflicto()).isEqualTo(existente);
                        assertThat(e.getMessage()).contains(existente.id(), "PROGRAMADA");
                    });
            assertThat(publicador.publicados).hasSize(1);
        }
    }

    @Test
    void unPeriodoPuedeEmpezarJustoCuandoTerminaOtro() {
        servicio.programar(solicitud("E001", "2026-12-14T08:00", "2026-12-16T12:00"));

        assertThat(servicio.programar(solicitud("E001", "2026-12-16T12:00", "2026-12-18T12:00")).estado())
                .isEqualTo(EstadoVacaciones.PROGRAMADA);
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
        // Empieza ahora mismo: ya inició.
        Vacaciones yaInicio = servicio.programar(solicitud("E001", "2026-09-28T10:00", "2026-10-05"));
        assertThatThrownBy(() -> servicio.cancelar(yaInicio.id())).isInstanceOf(ConflictoDeEstadoException.class);

        Vacaciones futura = servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20"));
        servicio.cancelar(futura.id());
        assertThatThrownBy(() -> servicio.cancelar(futura.id())).isInstanceOf(ConflictoDeEstadoException.class);
    }

    // El mensaje del 409 explica por qué: "ya inició" solo si el período sigue ocupando fechas.
    @ParameterizedTest(name = "[{index}] {0} → 409: {1}")
    @CsvSource(delimiter = '|', textBlock = """
            EN_CURSO   | está EN_CURSO y ya inició
            FINALIZADA | está FINALIZADA
            CANCELADA  | está CANCELADA
            """)
    void noCancelaSegunElEstadoYExplicaElMotivo(EstadoVacaciones estado, String motivo) {
        vacaciones.agregar("V-X", "E001", LocalDateTime.of(2026, 9, 20, 8, 0), LocalDateTime.of(2026, 10, 5, 18, 0), estado);

        assertThatThrownBy(() -> servicio.cancelar("V-X"))
                .isInstanceOf(ConflictoDeEstadoException.class)
                .hasMessage("El período V-X no se puede cancelar: " + motivo);
        assertThat(vacaciones.datos.get("V-X").estado()).isEqualTo(estado);
    }

    @Test
    void siElPeriodoCambiaDeEstadoJustoAntesDeCancelarloEs409YNoSeToca() {
        Vacaciones futura = servicio.programar(solicitud("E001", "2026-12-14", "2026-12-20"));
        // El scheduler (u otra petición) lo cambia entre la consulta y la actualización.
        vacaciones.alCancelar = () -> vacaciones.datos.computeIfPresent(futura.id(),
                (id, v) -> v.conEstado(EstadoVacaciones.EN_CURSO));

        assertThatThrownBy(() -> servicio.cancelar(futura.id()))
                .isInstanceOf(ConflictoDeEstadoException.class)
                .hasMessage("El período " + futura.id() + " cambió de estado y no se puede cancelar");
        assertThat(servicio.consultar(futura.id()).estado()).isEqualTo(EstadoVacaciones.EN_CURSO);
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
