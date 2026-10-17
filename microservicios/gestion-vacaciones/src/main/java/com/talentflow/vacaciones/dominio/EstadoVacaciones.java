package com.talentflow.vacaciones.dominio;

/**
 * Ciclo de vida de un período: PROGRAMADA → EN_CURSO → FINALIZADA, o CANCELADA.
 * RR. HH. programa y cancela; las transiciones por fecha (EN_CURSO, FINALIZADA) las hace
 * el scheduler del Reto 5 al llegar fechaInicio y fechaFin.
 */
public enum EstadoVacaciones {
    PROGRAMADA,
    EN_CURSO,
    FINALIZADA,
    CANCELADA
}
