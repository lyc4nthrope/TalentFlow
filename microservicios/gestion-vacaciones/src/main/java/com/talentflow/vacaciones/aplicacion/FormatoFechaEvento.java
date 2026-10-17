package com.talentflow.vacaciones.aplicacion;

import java.time.LocalDateTime;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;

/**
 * Fechas de los eventos vacaciones.*: ISO-8601 con el desfase de la zona horaria del negocio
 * (p. ej. {@code 2026-10-20T08:00:00-05:00}), para que ningún consumidor interprete la hora en
 * otra zona. Desviación documentada del ejemplo del catálogo, que muestra solo la fecha (D6).
 */
public final class FormatoFechaEvento {

    private FormatoFechaEvento() {
    }

    public static String conDesfase(LocalDateTime fecha, ZoneId zona) {
        return fecha.atZone(zona).toOffsetDateTime().format(DateTimeFormatter.ISO_OFFSET_DATE_TIME);
    }
}
