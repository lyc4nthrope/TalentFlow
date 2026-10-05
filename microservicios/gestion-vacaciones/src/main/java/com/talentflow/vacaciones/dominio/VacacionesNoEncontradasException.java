package com.talentflow.vacaciones.dominio;

/** → 404 Not Found. */
public class VacacionesNoEncontradasException extends RuntimeException {

    public VacacionesNoEncontradasException(String id) {
        super("El período de vacaciones " + id + " no existe");
    }
}
