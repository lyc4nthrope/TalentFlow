package com.talentflow.vacaciones.aplicacion;

import com.talentflow.vacaciones.dominio.ReglasVacaciones;
import com.talentflow.vacaciones.dominio.Vacaciones;
import java.time.ZoneId;

/** Carga útil de vacaciones.programadas, campo por campo según el Catálogo de Eventos (3.8). */
public record VacacionesProgramadas(
        String vacacionesId,
        String empleadoId,
        String email,
        String fechaInicio,
        String fechaFin,
        int diasHabiles) {

    public static final String TIPO = "vacaciones.programadas";

    public static VacacionesProgramadas de(Vacaciones v, String email, ZoneId zona) {
        return new VacacionesProgramadas(v.id(), v.empleadoId(), email,
                FormatoFechaEvento.conDesfase(v.fechaInicio(), zona),
                FormatoFechaEvento.conDesfase(v.fechaFin(), zona),
                ReglasVacaciones.diasHabiles(v.fechaInicio(), v.fechaFin()));
    }
}
