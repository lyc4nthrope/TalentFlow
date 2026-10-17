package com.talentflow.vacaciones.aplicacion;

import com.talentflow.vacaciones.dominio.Vacaciones;
import java.time.ZoneId;

/** Carga útil de vacaciones.finalizadas, campo por campo según el Catálogo de Eventos (3.10). */
public record VacacionesFinalizadas(
        String vacacionesId,
        String empleadoId,
        String email,
        String fechaFin) {

    public static final String TIPO = "vacaciones.finalizadas";

    public static VacacionesFinalizadas de(Vacaciones v, String email, ZoneId zona) {
        return new VacacionesFinalizadas(v.id(), v.empleadoId(), email,
                FormatoFechaEvento.conDesfase(v.fechaFin(), zona));
    }
}
