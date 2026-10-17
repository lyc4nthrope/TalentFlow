package com.talentflow.vacaciones.infraestructura.scheduler;

import com.talentflow.vacaciones.aplicacion.ProcesadorCicloVacaciones;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;

/**
 * Dispara el ciclo de vida por fecha según VACACIONES_CRON (por defecto cada minuto, Reto 5 D7).
 * Nunca deja escapar una excepción: un fallo (p. ej. BD caída al listar) se registra y la
 * siguiente pasada lo vuelve a intentar.
 */
public class VacacionesScheduler {

    private static final Logger log = LoggerFactory.getLogger(VacacionesScheduler.class);

    private final ProcesadorCicloVacaciones ciclo;

    public VacacionesScheduler(ProcesadorCicloVacaciones ciclo) {
        this.ciclo = ciclo;
    }

    @Scheduled(cron = "${vacaciones.cron}", zone = "${vacaciones.zona-horaria}")
    public void ejecutar() {
        try {
            ciclo.ejecutar();
        } catch (RuntimeException e) {
            log.error("la pasada del scheduler de vacaciones falló; se reintentará: {}", e.getMessage(), e);
        }
    }
}
