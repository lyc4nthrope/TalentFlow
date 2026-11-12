package com.talentflow.vacaciones.dominio;

import java.time.Instant;
import java.time.LocalDateTime;

/**
 * Período de vacaciones (reto4.pdf, "Estructura de un período de vacaciones").
 * fechaInicio y fechaFin son fecha y hora locales en la zona horaria del negocio (Reto 5, D6).
 */
public record Vacaciones(
        String id,
        String empleadoId,
        LocalDateTime fechaInicio,
        LocalDateTime fechaFin,
        EstadoVacaciones estado,
        Instant fechaCreacion) {

    /** Solo se cancela un período programado que todavía no ha iniciado. */
    public boolean esCancelable(LocalDateTime ahora) {
        return estado == EstadoVacaciones.PROGRAMADA && fechaInicio.isAfter(ahora);
    }

    /** Mismas condiciones que cancelar: solo se cambian las fechas de lo que aún no empieza. */
    public boolean esReprogramable(LocalDateTime ahora) {
        return esCancelable(ahora);
    }

    /** Períodos que ocupan fechas del empleado (los cancelados o finalizados no). */
    public boolean estaVigente() {
        return estado == EstadoVacaciones.PROGRAMADA || estado == EstadoVacaciones.EN_CURSO;
    }

    public Vacaciones cancelada() {
        return conEstado(EstadoVacaciones.CANCELADA);
    }

    public Vacaciones conFechas(LocalDateTime nuevoInicio, LocalDateTime nuevoFin) {
        return new Vacaciones(id, empleadoId, nuevoInicio, nuevoFin, estado, fechaCreacion);
    }

    /** Copia del período con otro estado (el record es inmutable). */
    public Vacaciones conEstado(EstadoVacaciones nuevoEstado) {
        return new Vacaciones(id, empleadoId, fechaInicio, fechaFin, nuevoEstado, fechaCreacion);
    }
}
