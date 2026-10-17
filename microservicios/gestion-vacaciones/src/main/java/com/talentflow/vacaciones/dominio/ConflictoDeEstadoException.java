package com.talentflow.vacaciones.dominio;

/** La operación no aplica al estado actual del período (p. ej. cancelar uno ya iniciado) → 409. */
public class ConflictoDeEstadoException extends RuntimeException {

    public ConflictoDeEstadoException(String mensaje) {
        super(mensaje);
    }
}
