package com.talentflow.vacaciones.aplicacion;

import java.time.LocalDateTime;

/** Carga útil de vacaciones.programadas, campo por campo según el Catálogo de Eventos (3.8). */
public record VacacionesProgramadas(
        String vacacionesId,
        String empleadoId,
        String email,
        LocalDateTime fechaInicio,
        LocalDateTime fechaFin,
        int diasHabiles) {

    public static final String TIPO = "vacaciones.programadas";
}
