package com.talentflow.vacaciones.dominio;

/**
 * Ciclo de vida de un período: PROGRAMADA → EN_CURSO → FINALIZADA, o CANCELADA.
 * En el Reto 4 solo se alcanza PROGRAMADA (y CANCELADA); las transiciones por fecha
 * las hará el scheduler del Reto 5. Se declaran todos desde ya para no migrar el esquema.
 */
public enum EstadoVacaciones {
    PROGRAMADA,
    EN_CURSO,
    FINALIZADA,
    CANCELADA
}
