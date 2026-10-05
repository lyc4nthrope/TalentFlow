package com.talentflow.vacaciones.aplicacion;

/** Puerto de deduplicación (Catálogo de Eventos, 2.1): tabla eventos_procesados. */
public interface Deduplicador {

    /** Registra el id del mensaje. Devuelve false si ya estaba (evento duplicado). */
    boolean registrarSiEsNuevo(String eventoId);
}
