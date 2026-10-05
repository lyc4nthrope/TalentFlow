package com.talentflow.vacaciones.dominio;

/** Regla de negocio violada por la solicitud del cliente → 400 Bad Request. */
public class SolicitudInvalidaException extends RuntimeException {

    private final String campo;
    private final transient Vacaciones periodoEnConflicto;

    public SolicitudInvalidaException(String campo, String mensaje) {
        this(campo, mensaje, null);
    }

    public SolicitudInvalidaException(String campo, String mensaje, Vacaciones periodoEnConflicto) {
        super(mensaje);
        this.campo = campo;
        this.periodoEnConflicto = periodoEnConflicto;
    }

    public String campo() {
        return campo;
    }

    /** Validación 3 (solapamiento): el reto exige incluir el período en conflicto. */
    public Vacaciones periodoEnConflicto() {
        return periodoEnConflicto;
    }
}
