package com.talentflow.vacaciones.dominio;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.time.LocalDateTime;
import org.junit.jupiter.api.Test;

class ReglasVacacionesTest {

    private static final LocalDateTime HOY = LocalDateTime.of(2026, 9, 28, 0, 0);

    @Test
    void rechazaFechaFinAnteriorOIgualAlInicio() {
        assertThatThrownBy(() -> ReglasVacaciones.validarFechas(LocalDateTime.of(2026, 12, 30, 0, 0), LocalDateTime.of(2026, 12, 15, 0, 0), HOY))
                .isInstanceOf(SolicitudInvalidaException.class)
                .hasMessageContaining("posterior");
        // "posterior" es estricto: el mismo día no lo es (texto literal del reto).
        assertThatThrownBy(() -> ReglasVacaciones.validarFechas(LocalDateTime.of(2026, 12, 15, 0, 0), LocalDateTime.of(2026, 12, 15, 0, 0), HOY))
                .isInstanceOf(SolicitudInvalidaException.class);
    }

    @Test
    void rechazaInicioEnElPasadoPeroAceptaHoy() {
        assertThatThrownBy(() -> ReglasVacaciones.validarFechas(HOY.minusDays(1), HOY.plusDays(5), HOY))
                .isInstanceOf(SolicitudInvalidaException.class)
                .hasMessageContaining("anterior a hoy");
        assertThatCode(() -> ReglasVacaciones.validarFechas(HOY, HOY.plusDays(5), HOY)).doesNotThrowAnyException();
    }

    @Test
    void cuentaDiasHabilesDeLunesAViernesInclusive() {
        // Semana completa lunes 14 a domingo 20 de diciembre de 2026: 5 días hábiles.
        assertThat(ReglasVacaciones.diasHabiles(LocalDateTime.of(2026, 12, 14, 0, 0), LocalDateTime.of(2026, 12, 20, 0, 0))).isEqualTo(5);
        // Ejemplo del catálogo (15 al 30 de marzo de 2026): 11 de lunes a viernes.
        assertThat(ReglasVacaciones.diasHabiles(LocalDateTime.of(2026, 3, 15, 0, 0), LocalDateTime.of(2026, 3, 30, 0, 0))).isEqualTo(11);
        // Solo fin de semana: 0.
        assertThat(ReglasVacaciones.diasHabiles(LocalDateTime.of(2026, 12, 19, 0, 0), LocalDateTime.of(2026, 12, 20, 0, 0))).isZero();
    }

    @Test
    void detectaSolapamientoDeRangosCerrados() {
        LocalDateTime d1 = LocalDateTime.of(2026, 12, 1, 0, 0);
        LocalDateTime d10 = LocalDateTime.of(2026, 12, 10, 0, 0);
        assertThat(ReglasVacaciones.seSolapan(d1, d10, LocalDateTime.of(2026, 12, 5, 0, 0), LocalDateTime.of(2026, 12, 20, 0, 0))).isTrue();
        assertThat(ReglasVacaciones.seSolapan(d1, d10, d10, LocalDateTime.of(2026, 12, 20, 0, 0))).isTrue(); // comparten el día 10
        assertThat(ReglasVacaciones.seSolapan(d1, d10, LocalDateTime.of(2026, 12, 11, 0, 0), LocalDateTime.of(2026, 12, 20, 0, 0))).isFalse();
    }

    @Test
    void soloSeCancelaUnPeriodoProgramadoQueNoHaIniciado() {
        Vacaciones futura = new Vacaciones("V-1", "E1", HOY.plusDays(1), HOY.plusDays(5), EstadoVacaciones.PROGRAMADA, null);
        Vacaciones deHoy = new Vacaciones("V-2", "E1", HOY, HOY.plusDays(5), EstadoVacaciones.PROGRAMADA, null);
        assertThat(futura.esCancelable(HOY)).isTrue();
        assertThat(deHoy.esCancelable(HOY)).isFalse();
        assertThat(futura.cancelada().esCancelable(HOY)).isFalse();
    }
}
