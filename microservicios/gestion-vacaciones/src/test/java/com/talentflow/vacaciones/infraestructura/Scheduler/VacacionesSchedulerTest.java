package com.talentflow.vacaciones.infraestructura.Scheduler;

import static org.assertj.core.api.Assertions.assertThat;

import java.lang.reflect.Method;
import org.junit.jupiter.api.Test;
import org.springframework.scheduling.annotation.Scheduled;

class VacacionesSchedulerTest {

    @Test
    void debeUsarLaZonaHorariaDelNegocioEnElCron() throws Exception {
        Method metodo = VacacionesScheduler.class.getDeclaredMethod("procesarCicloVacaciones");
        Scheduled scheduled = metodo.getAnnotation(Scheduled.class);

        assertThat(scheduled).isNotNull();
        assertThat(scheduled.cron()).isEqualTo("${vacaciones.cron}");
        assertThat(scheduled.zone()).isEqualTo("${vacaciones.zona-horaria}");
    }
}
