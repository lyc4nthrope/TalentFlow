package com.talentflow.vacaciones.aplicacion;

import com.talentflow.vacaciones.dominio.EmpleadoReplica;
import com.talentflow.vacaciones.dominio.EstadoVacaciones;
import com.talentflow.vacaciones.dominio.Vacaciones;
import java.time.Clock;
import java.time.LocalDateTime;
import java.util.Optional;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Ciclo de vida por fecha (Reto 5, D7). Cada pasada del scheduler:
 * 1. PROGRAMADA con fechaInicio <= ahora → EN_CURSO y publica vacaciones.iniciadas;
 * 2. EN_CURSO con fechaFin <= ahora → FINALIZADA y publica vacaciones.finalizadas.
 * Un período cuyas dos fechas ya pasaron recorre ambas transiciones en la misma pasada, en orden.
 *
 * Cada período va en su propia transacción y su evento se publica después del commit (misma
 * regla que POST /vacaciones). Un período que falla no detiene a los demás: queda en su estado y
 * se reintenta en la siguiente pasada. Limitación conocida: con varias instancias del servicio,
 * cada una ejecuta el job (la actualización condicional evita la doble transición; la
 * coordinación entre instancias es el Reto 31, ShedLock).
 */
public class ProcesadorCicloVacaciones {

    private static final Logger log = LoggerFactory.getLogger(ProcesadorCicloVacaciones.class);

    private final RepositorioVacaciones vacaciones;
    private final RepositorioEmpleados empleados;
    private final PublicadorEventos publicador;
    private final Transacciones transacciones;
    private final Clock reloj;

    /** Cuántos períodos cambiaron de estado en una pasada y cuántos fallaron. */
    public record Resumen(int iniciados, int finalizados, int fallidos) {
    }

    private record Transicion(String tipo, Object data) {
    }

    public ProcesadorCicloVacaciones(RepositorioVacaciones vacaciones, RepositorioEmpleados empleados,
            PublicadorEventos publicador, Transacciones transacciones, Clock reloj) {
        this.vacaciones = vacaciones;
        this.empleados = empleados;
        this.publicador = publicador;
        this.transacciones = transacciones;
        this.reloj = reloj;
    }

    public Resumen ejecutar() {
        LocalDateTime ahora = LocalDateTime.now(reloj);
        int fallidos = 0;

        int iniciados = 0;
        for (Vacaciones v : vacaciones.listarPorIniciar(ahora)) {
            switch (transicionar(v, EstadoVacaciones.PROGRAMADA, EstadoVacaciones.EN_CURSO)) {
                case APLICADA -> iniciados++;
                case FALLIDA -> fallidos++;
                case OMITIDA -> { }
            }
        }

        // Se consulta después de iniciar: incluye a los que acaban de pasar a EN_CURSO.
        int finalizados = 0;
        for (Vacaciones v : vacaciones.listarPorFinalizar(ahora)) {
            switch (transicionar(v, EstadoVacaciones.EN_CURSO, EstadoVacaciones.FINALIZADA)) {
                case APLICADA -> finalizados++;
                case FALLIDA -> fallidos++;
                case OMITIDA -> { }
            }
        }

        Resumen resumen = new Resumen(iniciados, finalizados, fallidos);
        if (iniciados + finalizados + fallidos > 0) {
            log.info("ciclo de vacaciones: {} iniciado(s), {} finalizado(s), {} con error", iniciados, finalizados, fallidos);
        }
        return resumen;
    }

    private enum Desenlace { APLICADA, OMITIDA, FALLIDA }

    private Desenlace transicionar(Vacaciones v, EstadoVacaciones esperado, EstadoVacaciones nuevo) {
        Optional<Transicion> transicion;
        try {
            transicion = transacciones.ejecutar(() -> {
                if (!vacaciones.cambiarEstado(v.id(), esperado, nuevo)) {
                    return Optional.<Transicion>empty(); // otra operación lo cambió antes
                }
                String email = empleados.buscar(v.empleadoId()).map(EmpleadoReplica::email).orElse(null);
                if (email == null) {
                    log.warn("el empleado {} del período {} no está en la réplica: el evento sale sin email",
                            v.empleadoId(), v.id());
                }
                Vacaciones actualizado = v.conEstado(nuevo);
                return Optional.of(nuevo == EstadoVacaciones.EN_CURSO
                        ? new Transicion(VacacionesIniciadas.TIPO, VacacionesIniciadas.de(actualizado, email, reloj.getZone()))
                        : new Transicion(VacacionesFinalizadas.TIPO, VacacionesFinalizadas.de(actualizado, email, reloj.getZone())));
            });
        } catch (RuntimeException e) {
            log.error("no se pudo pasar el período {} de {} a {}; se reintentará en la siguiente pasada: {}",
                    v.id(), esperado, nuevo, e.getMessage(), e);
            return Desenlace.FALLIDA;
        }

        if (transicion.isEmpty()) {
            log.info("período {} omitido: ya no estaba {}", v.id(), esperado);
            return Desenlace.OMITIDA;
        }
        // Después del commit. El publicador no lanza por contrato; si aun así lo hace, la
        // transición ya quedó registrada y el error no debe detener a los demás períodos.
        try {
            publicador.publicar(transicion.get().tipo(), transicion.get().data());
        } catch (RuntimeException e) {
            log.error("período {} pasó a {} pero no se pudo publicar {}: {}",
                    v.id(), nuevo, transicion.get().tipo(), e.getMessage(), e);
        }
        return Desenlace.APLICADA;
    }
}
