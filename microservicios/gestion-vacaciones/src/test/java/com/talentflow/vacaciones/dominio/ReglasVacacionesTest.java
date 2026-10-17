package com.talentflow.vacaciones.dominio;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.time.LocalDateTime;
import org.junit.jupiter.api.Test;

class ReglasVacacionesTest {

    private static final LocalDateTime AHORA = LocalDateTime.of(2026, 10, 15, 10, 30, 0);

    @Test
    void rechazaFechaFinAnteriorOIgualAlInicio() {
        assertThatThrownBy(() -> ReglasVacaciones.validarFechas(AHORA.plusDays(10), AHORA.plusDays(5), AHORA))
                .isInstanceOfSatisfying(SolicitudInvalidaException.class, e -> {
                    assertThat(e.campo()).isEqualTo("fechaFin");
                    assertThat(e.getMessage()).contains("posterior");
                });
        // El mismo instante no es posterior.
        assertThatThrownBy(() -> ReglasVacaciones.validarFechas(AHORA.plusHours(1), AHORA.plusHours(1), AHORA))
                .isInstanceOf(SolicitudInvalidaException.class);
    }

    @Test
    void aceptaUnPeriodoDeMinutosElMismoDia() {
        // Con hora, un período de un mismo día es válido (D6): así se demuestra el ciclo en minutos.
        assertThatCode(() -> ReglasVacaciones.validarFechas(AHORA.plusMinutes(2), AHORA.plusMinutes(5), AHORA))
                .doesNotThrowAnyException();
    }

    @Test
    void rechazaInicioEnElPasadoAunqueSeaHoyPeroAceptaAhora() {
        // Una hora antes, el mismo día: ya es pasado.
        assertThatThrownBy(() -> ReglasVacaciones.validarFechas(AHORA.minusHours(1), AHORA.plusDays(5), AHORA))
                .isInstanceOfSatisfying(SolicitudInvalidaException.class, e -> {
                    assertThat(e.campo()).isEqualTo("fechaInicio");
                    assertThat(e.getMessage()).contains("pasado");
                });
        assertThatThrownBy(() -> ReglasVacaciones.validarFechas(AHORA.minusSeconds(1), AHORA.plusDays(5), AHORA))
                .isInstanceOf(SolicitudInvalidaException.class);
        assertThatCode(() -> ReglasVacaciones.validarFechas(AHORA, AHORA.plusDays(5), AHORA)).doesNotThrowAnyException();
    }

    @Test
    void cuentaDiasHabilesDeLunesAViernesInclusiveSinImportarLaHora() {
        // Lunes 14 (08:00) a domingo 20 (18:00) de diciembre de 2026: 5 días hábiles.
        assertThat(ReglasVacaciones.diasHabiles(LocalDateTime.of(2026, 12, 14, 8, 0), LocalDateTime.of(2026, 12, 20, 18, 0)))
                .isEqualTo(5);
        // Ejemplo del catálogo (15 al 30 de marzo de 2026): 11 de lunes a viernes.
        assertThat(ReglasVacaciones.diasHabiles(LocalDateTime.of(2026, 3, 15, 0, 0), LocalDateTime.of(2026, 3, 30, 23, 59, 59)))
                .isEqualTo(11);
        // Unos minutos de un jueves: 1 día.
        assertThat(ReglasVacaciones.diasHabiles(AHORA.plusMinutes(2), AHORA.plusMinutes(5))).isEqualTo(1);
        // Solo fin de semana: 0.
        assertThat(ReglasVacaciones.diasHabiles(LocalDateTime.of(2026, 12, 19, 0, 0), LocalDateTime.of(2026, 12, 20, 23, 0)))
                .isZero();
    }

    @Test
    void detectaSolapamientoDeRangosSemiabiertos() {
        LocalDateTime d1 = LocalDateTime.of(2026, 12, 1, 8, 0);
        LocalDateTime d10 = LocalDateTime.of(2026, 12, 10, 18, 0);
        assertThat(ReglasVacaciones.seSolapan(d1, d10, LocalDateTime.of(2026, 12, 5, 0, 0), LocalDateTime.of(2026, 12, 20, 0, 0)))
                .isTrue();
        // Uno contiene al otro.
        assertThat(ReglasVacaciones.seSolapan(d1, d10, d1.plusHours(1), d10.minusHours(1))).isTrue();
        // Un segundo en común ya es cruce.
        assertThat(ReglasVacaciones.seSolapan(d1, d10, d10.minusSeconds(1), d10.plusDays(3))).isTrue();
        // Contiguos: uno termina justo cuando empieza el otro → no se cruzan (en ambos órdenes).
        assertThat(ReglasVacaciones.seSolapan(d1, d10, d10, d10.plusDays(3))).isFalse();
        assertThat(ReglasVacaciones.seSolapan(d10, d10.plusDays(3), d1, d10)).isFalse();
        assertThat(ReglasVacaciones.seSolapan(d1, d10, d10.plusDays(1), d10.plusDays(3))).isFalse();
    }

    @Test
    void soloSeCancelaUnPeriodoProgramadoQueNoHaIniciado() {
        Vacaciones futura = new Vacaciones("V-1", "E1", AHORA.plusMinutes(1), AHORA.plusDays(5), EstadoVacaciones.PROGRAMADA, null);
        Vacaciones yaInicio = new Vacaciones("V-2", "E1", AHORA, AHORA.plusDays(5), EstadoVacaciones.PROGRAMADA, null);
        assertThat(futura.esCancelable(AHORA)).isTrue();
        assertThat(yaInicio.esCancelable(AHORA)).isFalse();
        assertThat(futura.cancelada().esCancelable(AHORA)).isFalse();
    }
}
