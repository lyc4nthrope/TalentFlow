package com.talentflow.vacaciones.infraestructura.scheduler;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;

import com.talentflow.vacaciones.aplicacion.ProcesadorCicloVacaciones;
import org.junit.jupiter.api.Test;
import org.springframework.scheduling.annotation.Scheduled;

class VacacionesSchedulerTest {

    @Test
    void unaPasadaFallidaNoDejaEscaparLaExcepcionYLaSiguienteVuelveAIntentar() {
        int[] pasadas = {0};
        ProcesadorCicloVacaciones cicloQueFalla = new ProcesadorCicloVacaciones(null, null, null, null, null) {
            @Override
            public Resumen ejecutar() {
                pasadas[0]++;
                throw new IllegalStateException("BD caída al listar");
            }
        };
        VacacionesScheduler scheduler = new VacacionesScheduler(cicloQueFalla);

        assertThatCode(scheduler::ejecutar).doesNotThrowAnyException();
        assertThatCode(scheduler::ejecutar).doesNotThrowAnyException();
        assertThat(pasadas[0]).isEqualTo(2);
    }

    // D6: sin zone, un VACACIONES_CRON con hora fija (p. ej. "0 0 0 * * *", medianoche) se
    // evaluaría en la zona del contenedor (UTC), 5 horas antes que en America/Bogota.
    @Test
    void elCronSeEvaluaEnLaZonaHorariaDelNegocio() throws NoSuchMethodException {
        Scheduled programacion = VacacionesScheduler.class.getDeclaredMethod("ejecutar").getAnnotation(Scheduled.class);

        assertThat(programacion.cron()).isEqualTo("${vacaciones.cron}");
        assertThat(programacion.zone()).isEqualTo("${vacaciones.zona-horaria}");
    }
}
