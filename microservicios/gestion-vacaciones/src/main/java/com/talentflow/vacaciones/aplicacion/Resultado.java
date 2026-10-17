package com.talentflow.vacaciones.aplicacion;

/** Qué debe hacer el consumidor con el mensaje en el broker. */
public enum Resultado {
    /** Efecto aplicado → ack. */
    PROCESADO,
    /** Ese id ya se había procesado → ack sin repetir el efecto. */
    DUPLICADO,
    /** Nunca podrá procesarse (mensaje corrupto, tipo no soportado, dato rechazado) → ack + log. DLQ: Reto 29. */
    DESCARTADO,
    /** Fallo recuperable (p. ej. BD caída) → nack con reencolado tras una espera. */
    ERROR_TRANSITORIO
}
