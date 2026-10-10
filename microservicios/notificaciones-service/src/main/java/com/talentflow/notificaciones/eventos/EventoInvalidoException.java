package com.talentflow.notificaciones.eventos;

/** El mensaje no cumple el contrato del catálogo: reintentarlo no lo arregla, se descarta. */
public class EventoInvalidoException extends RuntimeException {

    public EventoInvalidoException(String mensaje) {
        super(mensaje);
    }

    public EventoInvalidoException(String mensaje, Throwable causa) {
        super(mensaje, causa);
    }
}
