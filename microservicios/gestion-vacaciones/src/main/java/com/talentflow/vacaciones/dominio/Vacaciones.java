package com.talentflow.vacaciones.dominio;

import java.time.Instant;
import java.time.LocalDate;

/** Período de vacaciones (reto4.pdf, "Estructura de un período de vacaciones"). */
public record Vacaciones(
        String id,
        String empleadoId,
        LocalDate fechaInicio,
        LocalDate fechaFin,
        EstadoVacaciones estado,
        Instant fechaCreacion) {

    /** Solo se cancela un período programado que todavía no ha iniciado. */
    public boolean esCancelable(LocalDate hoy) {
        return estado == EstadoVacaciones.PROGRAMADA && fechaInicio.isAfter(hoy);
    }

    /** Períodos que ocupan fechas del empleado (los cancelados o finalizados no). */
    public boolean estaVigente() {
        return estado == EstadoVacaciones.PROGRAMADA || estado == EstadoVacaciones.EN_CURSO;
    }

    public Vacaciones cancelada() {
        return new Vacaciones(id, empleadoId, fechaInicio, fechaFin, EstadoVacaciones.CANCELADA, fechaCreacion);
    }
}
