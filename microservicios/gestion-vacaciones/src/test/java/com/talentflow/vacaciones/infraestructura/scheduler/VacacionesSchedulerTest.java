package com.talentflow.vacaciones.infraestructura.scheduler;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;

import com.talentflow.vacaciones.aplicacion.ProcesadorCicloVacaciones;
import org.junit.jupiter.api.Test;

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
}
