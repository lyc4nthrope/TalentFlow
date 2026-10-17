package com.talentflow.vacaciones.aplicacion;

import static org.assertj.core.api.Assertions.assertThat;

import com.talentflow.vacaciones.dominio.EstadoVacaciones;
import java.time.Clock;
import java.time.Instant;
import java.time.LocalDateTime;
import java.time.ZoneId;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class ProcesadorCicloVacacionesTest {

    // "Ahora": 15 de octubre de 2026, 10:00 en Bogotá (15:00 UTC).
    private static final Clock RELOJ = Clock.fixed(Instant.parse("2026-10-15T15:00:00Z"), ZoneId.of("America/Bogota"));
    private static final LocalDateTime AHORA = LocalDateTime.of(2026, 10, 15, 10, 0);

    private Fakes.Vacacionesmem vacaciones;
    private Fakes.PublicadorEspia publicador;
    private ProcesadorCicloVacaciones ciclo;

    @BeforeEach
    void preparar() {
        vacaciones = new Fakes.Vacacionesmem();
        publicador = new Fakes.PublicadorEspia();
        Fakes.EmpleadosMem empleados = new Fakes.EmpleadosMem();
        empleados.registrar("E001", "juan@empresa.com");
        empleados.registrar("E002", "ana@empresa.com");
        ciclo = new ProcesadorCicloVacaciones(vacaciones, empleados, publicador, new Fakes.SinTransaccion(), RELOJ);
    }

    private EstadoVacaciones estado(String id) {
        return vacaciones.datos.get(id).estado();
    }

    @Test
    void iniciaElPeriodoCuandoLlegaSuFechaYPublicaVacacionesIniciadasSegunElCatalogo() {
        vacaciones.agregar("V-1", "E001", AHORA.minusMinutes(1), AHORA.plusDays(3), EstadoVacaciones.PROGRAMADA);

        ProcesadorCicloVacaciones.Resumen resumen = ciclo.ejecutar();

        assertThat(resumen).isEqualTo(new ProcesadorCicloVacaciones.Resumen(1, 0, 0));
        assertThat(estado("V-1")).isEqualTo(EstadoVacaciones.EN_CURSO);
        assertThat(publicador.publicados).containsExactly(new Fakes.PublicadorEspia.Publicado("vacaciones.iniciadas",
                new VacacionesIniciadas("V-1", "E001", "juan@empresa.com",
                        "2026-10-15T09:59:00-05:00", "2026-10-18T10:00:00-05:00")));
    }

    @Test
    void iniciaJustoEnSuFechaPeroNoAntes() {
        vacaciones.agregar("V-EXACTO", "E001", AHORA, AHORA.plusDays(1), EstadoVacaciones.PROGRAMADA);
        vacaciones.agregar("V-FUTURO", "E002", AHORA.plusSeconds(1), AHORA.plusDays(1), EstadoVacaciones.PROGRAMADA);

        ciclo.ejecutar();

        assertThat(estado("V-EXACTO")).isEqualTo(EstadoVacaciones.EN_CURSO);
        assertThat(estado("V-FUTURO")).isEqualTo(EstadoVacaciones.PROGRAMADA);
        assertThat(publicador.publicados).hasSize(1);
    }

    @Test
    void finalizaElPeriodoCuandoLlegaSuFechaFinYPublicaVacacionesFinalizadasSegunElCatalogo() {
        vacaciones.agregar("V-1", "E001", AHORA.minusDays(3), AHORA, EstadoVacaciones.EN_CURSO);
        vacaciones.agregar("V-2", "E002", AHORA.minusDays(3), AHORA.plusSeconds(1), EstadoVacaciones.EN_CURSO);

        ProcesadorCicloVacaciones.Resumen resumen = ciclo.ejecutar();

        assertThat(resumen).isEqualTo(new ProcesadorCicloVacaciones.Resumen(0, 1, 0));
        assertThat(estado("V-1")).isEqualTo(EstadoVacaciones.FINALIZADA);
        assertThat(estado("V-2")).as("aún no termina").isEqualTo(EstadoVacaciones.EN_CURSO);
        assertThat(publicador.publicados).containsExactly(new Fakes.PublicadorEspia.Publicado("vacaciones.finalizadas",
                new VacacionesFinalizadas("V-1", "E001", "juan@empresa.com", "2026-10-15T10:00:00-05:00")));
    }

    @Test
    void unPeriodoCuyasDosFechasPasaronSeIniciaYFinalizaEnLaMismaPasadaEnOrden() {
        vacaciones.agregar("V-1", "E001", AHORA.minusMinutes(5), AHORA.minusMinutes(2), EstadoVacaciones.PROGRAMADA);

        ProcesadorCicloVacaciones.Resumen resumen = ciclo.ejecutar();

        assertThat(resumen).isEqualTo(new ProcesadorCicloVacaciones.Resumen(1, 1, 0));
        assertThat(estado("V-1")).isEqualTo(EstadoVacaciones.FINALIZADA);
        assertThat(publicador.publicados).extracting(Fakes.PublicadorEspia.Publicado::tipo)
                .containsExactly("vacaciones.iniciadas", "vacaciones.finalizadas");
    }

    @Test
    void ignoraLosCanceladosYLosYaFinalizados() {
        vacaciones.agregar("V-C", "E001", AHORA.minusDays(1), AHORA.plusDays(1), EstadoVacaciones.CANCELADA);
        vacaciones.agregar("V-F", "E002", AHORA.minusDays(5), AHORA.minusDays(1), EstadoVacaciones.FINALIZADA);

        assertThat(ciclo.ejecutar()).isEqualTo(new ProcesadorCicloVacaciones.Resumen(0, 0, 0));
        assertThat(publicador.publicados).isEmpty();
    }

    @Test
    void unaSegundaPasadaNoRepublica() {
        vacaciones.agregar("V-1", "E001", AHORA.minusMinutes(1), AHORA.plusDays(3), EstadoVacaciones.PROGRAMADA);

        ciclo.ejecutar();
        ciclo.ejecutar();

        assertThat(publicador.publicados).hasSize(1);
    }

    @Test
    void siOtraOperacionCambioElEstadoAntesNoSePublicaNada() {
        vacaciones.agregar("V-1", "E001", AHORA.minusMinutes(1), AHORA.plusDays(3), EstadoVacaciones.PROGRAMADA);
        // RR. HH. cancela justo entre la consulta del scheduler y su actualización.
        vacaciones.alCambiarEstado = () -> vacaciones.datos.computeIfPresent("V-1", (id, v) -> v.cancelada());

        ProcesadorCicloVacaciones.Resumen resumen = ciclo.ejecutar();

        assertThat(resumen).isEqualTo(new ProcesadorCicloVacaciones.Resumen(0, 0, 0));
        assertThat(estado("V-1")).isEqualTo(EstadoVacaciones.CANCELADA);
        assertThat(publicador.publicados).isEmpty();
    }

    @Test
    void unPeriodoQueFallaNoDetieneALosDemasYSeReintentaDespues() {
        vacaciones.agregar("V-MALO", "E001", AHORA.minusMinutes(3), AHORA.plusDays(3), EstadoVacaciones.PROGRAMADA);
        vacaciones.agregar("V-BUENO", "E002", AHORA.minusMinutes(1), AHORA.plusDays(3), EstadoVacaciones.PROGRAMADA);
        vacaciones.fallarCambioDe.add("V-MALO");

        ProcesadorCicloVacaciones.Resumen resumen = ciclo.ejecutar();

        assertThat(resumen).isEqualTo(new ProcesadorCicloVacaciones.Resumen(1, 0, 1));
        assertThat(estado("V-MALO")).isEqualTo(EstadoVacaciones.PROGRAMADA);
        assertThat(estado("V-BUENO")).isEqualTo(EstadoVacaciones.EN_CURSO);
        assertThat(publicador.publicados).extracting(p -> ((VacacionesIniciadas) p.data()).vacacionesId())
                .containsExactly("V-BUENO");

        // La BD vuelve: la siguiente pasada lo inicia.
        vacaciones.fallarCambioDe.clear();
        assertThat(ciclo.ejecutar()).isEqualTo(new ProcesadorCicloVacaciones.Resumen(1, 0, 0));
        assertThat(estado("V-MALO")).isEqualTo(EstadoVacaciones.EN_CURSO);
    }

    @Test
    void cadaPeriodoTieneSuTransaccionYSuEventoSePublicaDespuesDeCerrarla() {
        Fakes.TransaccionesRastreadas transacciones = new Fakes.TransaccionesRastreadas();
        publicador.transacciones = transacciones;
        Fakes.EmpleadosMem empleados = new Fakes.EmpleadosMem();
        empleados.registrar("E001", "juan@empresa.com");
        ciclo = new ProcesadorCicloVacaciones(vacaciones, empleados, publicador, transacciones, RELOJ);
        vacaciones.agregar("V-1", "E001", AHORA.minusMinutes(3), AHORA.plusDays(3), EstadoVacaciones.PROGRAMADA);
        vacaciones.agregar("V-2", "E001", AHORA.minusDays(5), AHORA.minusDays(4), EstadoVacaciones.PROGRAMADA);
        vacaciones.agregar("V-3", "E001", AHORA.minusDays(9), AHORA.minusDays(8), EstadoVacaciones.EN_CURSO);

        ciclo.ejecutar();

        // V-1 inicia; V-2 inicia y finaliza; V-3 finaliza: 4 transiciones = 4 transacciones.
        assertThat(publicador.publicados).hasSize(4);
        assertThat(transacciones.abiertas).isEqualTo(4);
        assertThat(publicador.dentroDeTransaccionAlPublicar).containsExactly(false, false, false, false);
    }

    @Test
    void unaPublicacionFallidaNoDetieneALosDemasNiDeshaceLaTransicion() {
        vacaciones.agregar("V-1", "E001", AHORA.minusMinutes(3), AHORA.plusDays(3), EstadoVacaciones.PROGRAMADA);
        vacaciones.agregar("V-2", "E002", AHORA.minusMinutes(1), AHORA.plusDays(3), EstadoVacaciones.PROGRAMADA);
        publicador.lanzarEnLaPublicacion = 1; // el publicador viola su contrato y lanza

        ProcesadorCicloVacaciones.Resumen resumen = ciclo.ejecutar();

        assertThat(resumen).isEqualTo(new ProcesadorCicloVacaciones.Resumen(2, 0, 0));
        assertThat(estado("V-1")).isEqualTo(EstadoVacaciones.EN_CURSO);
        assertThat(estado("V-2")).isEqualTo(EstadoVacaciones.EN_CURSO);
        assertThat(publicador.publicados).hasSize(2);
    }

    @Test
    void unaFinalizacionQueFallaSeCuentaYSeReintentaEnLaSiguientePasada() {
        vacaciones.agregar("V-1", "E001", AHORA.minusDays(3), AHORA.minusMinutes(1), EstadoVacaciones.EN_CURSO);
        vacaciones.fallarCambioDe.add("V-1");

        assertThat(ciclo.ejecutar()).isEqualTo(new ProcesadorCicloVacaciones.Resumen(0, 0, 1));
        assertThat(estado("V-1")).isEqualTo(EstadoVacaciones.EN_CURSO);
        assertThat(publicador.publicados).isEmpty();

        vacaciones.fallarCambioDe.clear();
        assertThat(ciclo.ejecutar()).isEqualTo(new ProcesadorCicloVacaciones.Resumen(0, 1, 0));
        assertThat(estado("V-1")).isEqualTo(EstadoVacaciones.FINALIZADA);
    }

    @Test
    void siOtraInstanciaYaLoFinalizoNoSePublicaDosVeces() {
        vacaciones.agregar("V-1", "E001", AHORA.minusDays(3), AHORA.minusMinutes(1), EstadoVacaciones.EN_CURSO);
        // Otra instancia del servicio gana la carrera (limitación documentada hasta el Reto 31).
        vacaciones.alCambiarEstado = () -> vacaciones.datos.computeIfPresent("V-1",
                (id, v) -> v.conEstado(EstadoVacaciones.FINALIZADA));

        assertThat(ciclo.ejecutar()).isEqualTo(new ProcesadorCicloVacaciones.Resumen(0, 0, 0));
        assertThat(publicador.publicados).isEmpty();
    }

    @Test
    void unEmpleadoQueNoEstaEnLaReplicaNoDetieneLaTransicionYElEventoSaleSinEmail() {
        vacaciones.agregar("V-1", "E999", AHORA.minusMinutes(1), AHORA.plusDays(3), EstadoVacaciones.PROGRAMADA);

        assertThat(ciclo.ejecutar()).isEqualTo(new ProcesadorCicloVacaciones.Resumen(1, 0, 0));
        assertThat(estado("V-1")).isEqualTo(EstadoVacaciones.EN_CURSO);
        assertThat(publicador.publicados).containsExactly(new Fakes.PublicadorEspia.Publicado("vacaciones.iniciadas",
                new VacacionesIniciadas("V-1", "E999", null, "2026-10-15T09:59:00-05:00", "2026-10-18T10:00:00-05:00")));
    }
}
