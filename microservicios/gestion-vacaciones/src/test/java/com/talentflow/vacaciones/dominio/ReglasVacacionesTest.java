package com.talentflow.vacaciones.dominio;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.time.LocalDateTime;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;

class ReglasVacacionesTest {

    private static final LocalDateTime AHORA = LocalDateTime.of(2026, 10, 15, 10, 30, 0);

    // Validación 1 (fechaFin > fechaInicio) y 2 (sin fechas pasadas). "Ahora" = jueves 15/10/2026 10:30.
    @ParameterizedTest(name = "[{index}] {0} → 400 en {3}")
    @CsvSource(delimiter = '|', textBlock = """
            fin antes del inicio                          | 2026-10-25T10:30:00 | 2026-10-20T10:30:00 | fechaFin    | posterior
            fin igual al inicio (mismo instante)          | 2026-10-15T11:30:00 | 2026-10-15T11:30:00 | fechaFin    | posterior
            fin un segundo antes del inicio               | 2026-10-16T00:00:01 | 2026-10-16T00:00:00 | fechaFin    | posterior
            inicio ayer                                   | 2026-10-14T10:30:00 | 2026-10-20T10:30:00 | fechaInicio | pasado
            inicio hoy pero una hora antes de ahora       | 2026-10-15T09:30:00 | 2026-10-20T10:30:00 | fechaInicio | pasado
            inicio un segundo antes de ahora              | 2026-10-15T10:29:59 | 2026-10-20T10:30:00 | fechaInicio | pasado
            fin incoherente y en el pasado: gana fechaFin | 2026-10-01T00:00:00 | 2026-09-30T00:00:00 | fechaFin    | posterior
            """)
    void rechazaFechasIncoherentesOPasadas(String caso, LocalDateTime inicio, LocalDateTime fin,
            String campo, String fragmento) {
        assertThatThrownBy(() -> ReglasVacaciones.validarFechas(inicio, fin, AHORA))
                .isInstanceOfSatisfying(SolicitudInvalidaException.class, e -> {
                    assertThat(e.campo()).isEqualTo(campo);
                    assertThat(e.getMessage()).contains(fragmento);
                    assertThat(e.periodoEnConflicto()).isNull();
                });
    }

    // Con hora, un período de un mismo día es válido (D6): así se demuestra el ciclo en minutos.
    @ParameterizedTest(name = "[{index}] {0} → válido")
    @CsvSource(delimiter = '|', textBlock = """
            empieza ahora mismo                    | 2026-10-15T10:30:00 | 2026-10-20T10:30:00
            período de minutos el mismo día        | 2026-10-15T10:32:00 | 2026-10-15T10:35:00
            fin un segundo después del inicio      | 2026-10-16T00:00:00 | 2026-10-16T00:00:01
            """)
    void aceptaFechasCoherentesDesdeAhora(String caso, LocalDateTime inicio, LocalDateTime fin) {
        assertThatCode(() -> ReglasVacaciones.validarFechas(inicio, fin, AHORA)).doesNotThrowAnyException();
    }

    // Lunes a viernes, ambas fechas incluidas; la hora no cuenta.
    @ParameterizedTest(name = "[{index}] {0} → {3} día(s) hábil(es)")
    @CsvSource(delimiter = '|', textBlock = """
            lunes 08:00 a domingo 18:00                | 2026-12-14T08:00:00 | 2026-12-20T18:00:00 | 5
            ejemplo del catálogo: 15 al 30 de marzo    | 2026-03-15T00:00:00 | 2026-03-30T23:59:59 | 11
            unos minutos de un jueves                  | 2026-10-15T10:32:00 | 2026-10-15T10:35:00 | 1
            solo fin de semana                         | 2026-12-19T00:00:00 | 2026-12-20T23:00:00 | 0
            viernes a lunes                            | 2026-12-18T09:00:00 | 2026-12-21T09:00:00 | 2
            dos semanas completas                      | 2026-12-07T00:00:00 | 2026-12-20T23:59:59 | 10
            """)
    void cuentaDiasHabilesDeLunesAViernesInclusiveSinImportarLaHora(String caso, LocalDateTime inicio,
            LocalDateTime fin, int dias) {
        assertThat(ReglasVacaciones.diasHabiles(inicio, fin)).isEqualTo(dias);
    }

    /*
     * Validación 3: rangos semiabiertos [inicio, fin). Período existente fijo salvo en las filas
     * "fecha sola", que usan la conversión de FechaHoraEntrada (D6): inicio 00:00:00, fin 23:59:59.
     * Se comprueba en ambos órdenes: el cruce es simétrico.
     */
    @ParameterizedTest(name = "[{index}] {0} → se solapan: {5}")
    @CsvSource(delimiter = '|', textBlock = """
            nuevo dentro del existente                      | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | 2026-12-12T00:00:00 | 2026-12-15T00:00:00 | true
            nuevo contiene al existente                     | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | 2026-12-09T00:00:00 | 2026-12-21T00:00:00 | true
            idéntico                                        | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | true
            mismo inicio, termina antes                     | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | 2026-12-10T08:00:00 | 2026-12-11T00:00:00 | true
            mismo fin, empieza después                      | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | 2026-12-15T00:00:00 | 2026-12-20T18:00:00 | true
            cruce parcial al inicio del existente           | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | 2026-12-05T00:00:00 | 2026-12-12T00:00:00 | true
            cruce parcial al final del existente            | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | 2026-12-18T00:00:00 | 2026-12-25T00:00:00 | true
            un segundo en común al final                    | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | 2026-12-20T17:59:59 | 2026-12-25T00:00:00 | true
            un segundo en común al inicio                   | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | 2026-12-05T00:00:00 | 2026-12-10T08:00:01 | true
            contiguo después: empieza cuando termina el otro | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | 2026-12-20T18:00:00 | 2026-12-25T00:00:00 | false
            contiguo antes: termina cuando empieza el otro  | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | 2026-12-05T00:00:00 | 2026-12-10T08:00:00 | false
            un día de separación después                    | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | 2026-12-21T18:00:00 | 2026-12-25T00:00:00 | false
            un día de separación antes                      | 2026-12-10T08:00:00 | 2026-12-20T18:00:00 | 2026-12-01T00:00:00 | 2026-12-09T08:00:00 | false
            fecha sola: empieza el día en que termina el otro | 2026-12-10T00:00:00 | 2026-12-20T23:59:59 | 2026-12-20T00:00:00 | 2026-12-25T23:59:59 | true
            fecha sola: empieza el día siguiente            | 2026-12-10T00:00:00 | 2026-12-20T23:59:59 | 2026-12-21T00:00:00 | 2026-12-25T23:59:59 | false
            fecha sola: termina el día en que empieza el otro | 2026-12-10T00:00:00 | 2026-12-20T23:59:59 | 2026-12-05T00:00:00 | 2026-12-10T23:59:59 | true
            fecha sola: termina el día anterior             | 2026-12-10T00:00:00 | 2026-12-20T23:59:59 | 2026-12-05T00:00:00 | 2026-12-09T23:59:59 | false
            """)
    void detectaSolapamientoDeRangosSemiabiertos(String caso, LocalDateTime inicioExistente, LocalDateTime finExistente,
            LocalDateTime inicioNuevo, LocalDateTime finNuevo, boolean esperado) {
        assertThat(ReglasVacaciones.seSolapan(inicioExistente, finExistente, inicioNuevo, finNuevo)).isEqualTo(esperado);
        assertThat(ReglasVacaciones.seSolapan(inicioNuevo, finNuevo, inicioExistente, finExistente))
                .as("simétrico").isEqualTo(esperado);
    }
}
