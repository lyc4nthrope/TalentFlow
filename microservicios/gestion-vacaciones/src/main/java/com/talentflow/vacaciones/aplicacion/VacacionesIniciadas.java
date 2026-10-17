package com.talentflow.vacaciones.aplicacion;

import com.talentflow.vacaciones.dominio.Vacaciones;
import java.time.ZoneId;

/** Carga útil de vacaciones.iniciadas, campo por campo según el Catálogo de Eventos (3.9). */
public record VacacionesIniciadas(
        String vacacionesId,
        String empleadoId,
        String email,
        String fechaInicio,
        String fechaFin) {

    public static final String TIPO = "vacaciones.iniciadas";

    public static VacacionesIniciadas de(Vacaciones v, String email, ZoneId zona) {
        return new VacacionesIniciadas(v.id(), v.empleadoId(), email,
                FormatoFechaEvento.conDesfase(v.fechaInicio(), zona),
                FormatoFechaEvento.conDesfase(v.fechaFin(), zona));
    }
}
