package com.talentflow.vacaciones.aplicacion;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.exc.InvalidFormatException;
import java.time.LocalDateTime;
import org.junit.jupiter.api.Test;

class FechaHoraEntradaTest {

    private final ObjectMapper json = new ObjectMapper().findAndRegisterModules();

    @Test
    void aceptaFechaYHoraConYSinSegundos() {
        assertThat(FechaHoraEntrada.comoInicio("2026-10-20T08:30").map(FechaSolicitada::valor)).contains(LocalDateTime.of(2026, 10, 20, 8, 30, 0));
        assertThat(FechaHoraEntrada.comoFin("2026-10-20T18:45:15").map(FechaSolicitada::valor)).contains(LocalDateTime.of(2026, 10, 20, 18, 45, 15));
        // Las fracciones de segundo se descartan.
        assertThat(FechaHoraEntrada.comoInicio("2026-10-20T08:30:00.987").map(FechaSolicitada::valor)).contains(LocalDateTime.of(2026, 10, 20, 8, 30, 0));
    }

    @Test
    void soloLaFechaEsElComienzoDelDiaComoInicioYSuUltimoSegundoComoFin() {
        // Compatibilidad con el Reto 4.
        assertThat(FechaHoraEntrada.comoInicio("2026-12-14"))
                .contains(new FechaSolicitada(LocalDateTime.of(2026, 12, 14, 0, 0, 0), true));
        assertThat(FechaHoraEntrada.comoFin("2026-12-20"))
                .contains(new FechaSolicitada(LocalDateTime.of(2026, 12, 20, 23, 59, 59), true));
        assertThat(FechaHoraEntrada.comoInicio("2026-12-14T00:00").orElseThrow().soloFecha()).isFalse();
    }

    @Test
    void rechazaTextosQueNoSonFechas() {
        assertThat(FechaHoraEntrada.comoInicio("2026-02-30")).isEmpty();
        assertThat(FechaHoraEntrada.comoInicio("2026-10-20T25:00")).isEmpty();
        assertThat(FechaHoraEntrada.comoInicio("2026-10-20T08:00:00-05:00")).as("con desfase no").isEmpty();
        assertThat(FechaHoraEntrada.comoFin("mañana")).isEmpty();
        assertThat(FechaHoraEntrada.comoFin("")).isEmpty();
        assertThat(FechaHoraEntrada.comoFin(null)).isEmpty();
    }

    @Test
    void elCuerpoDeLaSolicitudUsaCadaRegla() throws Exception {
        SolicitudVacaciones s = json.readValue(
                "{\"empleadoId\":\"E001\",\"fechaInicio\":\"2026-12-14\",\"fechaFin\":\"2026-12-20\"}", SolicitudVacaciones.class);

        assertThat(s.fechaInicio().valor()).isEqualTo(LocalDateTime.of(2026, 12, 14, 0, 0, 0));
        assertThat(s.fechaFin().valor()).isEqualTo(LocalDateTime.of(2026, 12, 20, 23, 59, 59));
    }

    @Test
    void conLaFechaSolaHoyEmpiezaAhoraYOtroDiaAlComienzoDelDia() {
        LocalDateTime ahora = LocalDateTime.of(2026, 10, 15, 10, 30, 0);
        FechaSolicitada hoy = FechaHoraEntrada.comoInicio("2026-10-15").orElseThrow();
        FechaSolicitada manana = FechaHoraEntrada.comoInicio("2026-10-16").orElseThrow();
        FechaSolicitada ayer = FechaHoraEntrada.comoInicio("2026-10-14").orElseThrow();
        FechaSolicitada hoyALaMedianocheExplicita = FechaHoraEntrada.comoInicio("2026-10-15T00:00").orElseThrow();

        assertThat(FechaHoraEntrada.inicioEfectivo(hoy, ahora)).isEqualTo(ahora);
        assertThat(FechaHoraEntrada.inicioEfectivo(manana, ahora)).isEqualTo(LocalDateTime.of(2026, 10, 16, 0, 0));
        // Ayer sigue siendo ayer (la validación lo rechaza); una hora explícita se respeta.
        assertThat(FechaHoraEntrada.inicioEfectivo(ayer, ahora)).isEqualTo(LocalDateTime.of(2026, 10, 14, 0, 0));
        assertThat(FechaHoraEntrada.inicioEfectivo(hoyALaMedianocheExplicita, ahora))
                .isEqualTo(LocalDateTime.of(2026, 10, 15, 0, 0));
    }

    @Test
    void unaFechaInvalidaEnElCuerpoIndicaElCampo() {
        assertThatThrownBy(() -> json.readValue(
                "{\"empleadoId\":\"E001\",\"fechaInicio\":\"2026-12-14\",\"fechaFin\":20261220}", SolicitudVacaciones.class))
                .isInstanceOfSatisfying(InvalidFormatException.class,
                        e -> assertThat(e.getPath().get(e.getPath().size() - 1).getFieldName()).isEqualTo("fechaFin"));
    }
}
