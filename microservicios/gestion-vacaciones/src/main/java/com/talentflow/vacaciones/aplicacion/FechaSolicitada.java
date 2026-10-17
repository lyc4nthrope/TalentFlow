package com.talentflow.vacaciones.aplicacion;

import java.time.LocalDateTime;

/**
 * Fecha recibida en POST /vacaciones ya interpretada, recordando si el cliente envió solo la
 * fecha (formato del Reto 4). Hace falta para respetar "hoy es válido" con la fecha sola.
 */
public record FechaSolicitada(LocalDateTime valor, boolean soloFecha) {
}
