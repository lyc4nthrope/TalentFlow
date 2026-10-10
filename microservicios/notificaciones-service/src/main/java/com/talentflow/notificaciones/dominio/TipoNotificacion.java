package com.talentflow.notificaciones.dominio;

public enum TipoNotificacion {
    BIENVENIDA("BIENVENIDA"),
    DESVINCULACION("DESVINCULACIÓN"),
    VACACIONES("VACACIONES");

    private final String etiquetaLog;

    TipoNotificacion(String etiquetaLog) {
        this.etiquetaLog = etiquetaLog;
    }

    /** Texto usado en el log simulado (con tilde, como en el enunciado). El JSON usa name(). */
    public String etiquetaLog() {
        return etiquetaLog;
    }
}
