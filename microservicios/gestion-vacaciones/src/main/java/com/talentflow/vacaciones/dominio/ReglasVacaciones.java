package com.talentflow.vacaciones.dominio;

import java.time.DayOfWeek;
import java.time.LocalDateTime;

/** Reglas de negocio de los períodos de vacaciones (sin dependencias de infraestructura). */
public final class ReglasVacaciones {

    private ReglasVacaciones() {
    }

    /**
     * Validaciones 1 y 2 del reto4.pdf.
     * 1. fechaFin debe ser POSTERIOR a fechaInicio (texto literal del reto: el mismo día no lo es).
     * 2. fechaInicio no puede estar en el pasado; hoy sí es válido.
     */
    public static void validarFechas(LocalDateTime fechaInicio, LocalDateTime fechaFin, LocalDateTime hoy) {
        if (!fechaFin.isAfter(fechaInicio)) {
            throw new SolicitudInvalidaException("fechaFin",
                    "La fechaFin (" + fechaFin + ") debe ser posterior a la fechaInicio (" + fechaInicio + ")");
        }
        if (fechaInicio.isBefore(hoy)) {
            throw new SolicitudInvalidaException("fechaInicio",
                    "La fechaInicio (" + fechaInicio + ") no puede ser anterior a hoy (" + hoy + ")");
        }
    }

    /**
     * Días hábiles del período: lunes a viernes, ambos extremos incluidos, sin festivos.
     * (El ejemplo del Catálogo de Eventos, 12 días del 15 al 30 de marzo de 2026, no cuadra
     * con ninguna regla estándar: de lunes a viernes son 11. Se documenta la regla elegida.)
     */
    public static int diasHabiles(LocalDateTime fechaInicio, LocalDateTime fechaFin) {
        return (int) fechaInicio.toLocalDate()
        .datesUntil(fechaFin.toLocalDate().plusDays(1))
                .filter(dia -> dia.getDayOfWeek() != DayOfWeek.SATURDAY && dia.getDayOfWeek() != DayOfWeek.SUNDAY)
                .count();
    }

    /** Dos rangos cerrados [inicio, fin] se solapan si comparten al menos un día. */
    public static boolean seSolapan(LocalDateTime inicioA, LocalDateTime finA, LocalDateTime inicioB, LocalDateTime finB) {
        return !inicioA.isAfter(finB) && !inicioB.isAfter(finA);
    }
}
