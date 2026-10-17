package com.talentflow.vacaciones.dominio;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.LocalDateTime;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;

class VacacionesTest {

    private static final LocalDateTime AHORA = LocalDateTime.of(2026, 10, 15, 10, 30, 0);

    private static Vacaciones periodo(EstadoVacaciones estado, LocalDateTime inicio) {
        return new Vacaciones("V-1", "E1", inicio, inicio.plusDays(5), estado, null);
    }

    // Solo un período PROGRAMADA que todavía no ha iniciado se puede cancelar.
    @ParameterizedTest(name = "[{index}] {0}, inicio en {1} min → cancelable: {2}")
    @CsvSource(delimiter = '|', textBlock = """
            PROGRAMADA | 1  | true
            PROGRAMADA | 0  | false
            PROGRAMADA | -1 | false
            EN_CURSO   | 1  | false
            FINALIZADA | 1  | false
            CANCELADA  | 1  | false
            """)
    void soloSeCancelaUnPeriodoProgramadoQueNoHaIniciado(EstadoVacaciones estado, long minutosHastaInicio,
            boolean cancelable) {
        assertThat(periodo(estado, AHORA.plusMinutes(minutosHastaInicio)).esCancelable(AHORA)).isEqualTo(cancelable);
    }

    // Los cancelados y finalizados liberan las fechas del empleado.
    @ParameterizedTest(name = "[{index}] {0} → ocupa fechas: {1}")
    @CsvSource(delimiter = '|', textBlock = """
            PROGRAMADA | true
            EN_CURSO   | true
            FINALIZADA | false
            CANCELADA  | false
            """)
    void soloLosProgramadosYEnCursoOcupanFechas(EstadoVacaciones estado, boolean vigente) {
        assertThat(periodo(estado, AHORA).estaVigente()).isEqualTo(vigente);
    }

    @Test
    void cancelarDevuelveUnaCopiaCanceladaSinTocarElResto() {
        Vacaciones original = periodo(EstadoVacaciones.PROGRAMADA, AHORA.plusDays(1));

        Vacaciones cancelada = original.cancelada();

        assertThat(cancelada.estado()).isEqualTo(EstadoVacaciones.CANCELADA);
        assertThat(cancelada).usingRecursiveComparison().ignoringFields("estado").isEqualTo(original);
        assertThat(original.estado()).as("el record es inmutable").isEqualTo(EstadoVacaciones.PROGRAMADA);
    }
}
