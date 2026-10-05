package com.talentflow.vacaciones.aplicacion;

import java.util.function.Supplier;

/** Puerto para ejecutar un bloque en una transacción (la infraestructura decide cómo). */
public interface Transacciones {

    <T> T ejecutar(Supplier<T> bloque);
}
