package com.talentflow.vacaciones.infraestructura.scheduler;

import com.talentflow.vacaciones.aplicacion.ProcesadorCicloVacaciones;
import net.javacrumbs.shedlock.spring.annotation.SchedulerLock;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;

/**
 * Dispara el ciclo de vida por fecha según VACACIONES_CRON (por defecto cada minuto, Reto 5 D7).
 * Coordinado con ShedLock (V8) para que solo una réplica ejecute cada pasada.
 * Nunca deja escapar una excepción: un fallo (p. ej. BD caída al listar) se registra y la
 * siguiente pasada lo vuelve a intentar.
 */
public class VacacionesScheduler {

    private static final Logger log = LoggerFactory.getLogger(VacacionesScheduler.class);

    private final ProcesadorCicloVacaciones ciclo;

    public VacacionesScheduler(ProcesadorCicloVacaciones ciclo) {
        this.ciclo = ciclo;
    }

    // En producción el cron corre cada minuto (60 s) y en E2E cada 5 s (E2E_SCHEDULER_SEGUNDOS=5).
    // lockAtMostFor = "PT50S" libera el bloqueo antes del siguiente disparo de producción si una
    // instancia cae a mitad de pasada; lockAtLeastFor = "PT2S" evita que otra réplica con un ligero
    // desfase de reloj vuelva a ejecutar el mismo minuto cuando la primera termina en milisegundos,
    // sin bloquear el intervalo de 5 s usado en las pruebas E2E.
    @Scheduled(cron = "${vacaciones.cron}", zone = "${vacaciones.zona-horaria}")
    @SchedulerLock(name = "ciclo-vacaciones", lockAtMostFor = "PT50S", lockAtLeastFor = "PT2S")
    public void ejecutar() {
        try {
            ciclo.ejecutar();
        } catch (RuntimeException e) {
            log.error("la pasada del scheduler de vacaciones falló; se reintentará: {}", e.getMessage(), e);
        }
    }
}
