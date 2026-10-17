package com.talentflow.vacaciones.dominio;

import java.time.DayOfWeek;
import java.time.LocalDateTime;

/**
 * Reglas de negocio de los períodos de vacaciones (sin dependencias de infraestructura).
 * Las fechas llevan hora (Reto 5, D6): son instantes locales en la zona horaria del negocio.
 */
public final class ReglasVacaciones {

    private ReglasVacaciones() {
    }

    /**
     * Validaciones 1 y 2 del reto4.pdf, con precisión de fecha y hora.
     * 1. fechaFin debe ser POSTERIOR a fechaInicio.
     * 2. fechaInicio no puede estar en el pasado respecto de ahora.
     */
    public static void validarFechas(LocalDateTime fechaInicio, LocalDateTime fechaFin, LocalDateTime ahora) {
        if (!fechaFin.isAfter(fechaInicio)) {
            throw new SolicitudInvalidaException("fechaFin",
                    "La fechaFin (" + fechaFin + ") debe ser posterior a la fechaInicio (" + fechaInicio + ")");
        }
        if (fechaInicio.isBefore(ahora)) {
            throw new SolicitudInvalidaException("fechaInicio",
                    "La fechaInicio (" + fechaInicio + ") no puede estar en el pasado (ahora: " + ahora + ")");
        }
    }

    /**
     * Días hábiles del período: lunes a viernes entre la fecha de inicio y la de fin, ambas
     * incluidas, sin festivos. La hora no cuenta: un día con algunas horas de vacaciones es un día.
     * (El ejemplo del Catálogo de Eventos, 12 días del 15 al 30 de marzo de 2026, no cuadra
     * con ninguna regla estándar: de lunes a viernes son 11. Se documenta la regla elegida.)
     */
    public static int diasHabiles(LocalDateTime fechaInicio, LocalDateTime fechaFin) {
        return (int) fechaInicio.toLocalDate()
                .datesUntil(fechaFin.toLocalDate().plusDays(1))
                .filter(dia -> dia.getDayOfWeek() != DayOfWeek.SATURDAY && dia.getDayOfWeek() != DayOfWeek.SUNDAY)
                .count();
    }

    /**
     * Rangos semiabiertos [inicio, fin): se solapan si cada uno empieza antes de que el otro
     * termine. Un período puede empezar justo cuando termina otro.
     */
    public static boolean seSolapan(LocalDateTime inicioA, LocalDateTime finA, LocalDateTime inicioB, LocalDateTime finB) {
        return inicioA.isBefore(finB) && inicioB.isBefore(finA);
    }
}
