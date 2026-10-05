package com.talentflow.vacaciones.infraestructura.Scheduler;


import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import com.talentflow.vacaciones.aplicacion.ProcesarCicloVacacionesUseCase;

@Component 
public class VacacionesScheduler {

private final ProcesarCicloVacacionesUseCase procesarCicloVacacionesUseCase;

    public VacacionesScheduler(ProcesarCicloVacacionesUseCase procesarCicloVacacionesUseCase) {
        this.procesarCicloVacacionesUseCase = procesarCicloVacacionesUseCase;
    }

    @Scheduled(cron = "${vacaciones.cron}")
    public void procesarCicloVacaciones() {
        procesarCicloVacacionesUseCase.ejecutar();
    }
}