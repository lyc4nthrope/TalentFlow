package com.talentflow.notificaciones.eventos;

import com.fasterxml.jackson.databind.JsonNode;

/** Envelope común del Catálogo de Eventos (sección 2). */
public record Envelope(String id, String type, int version, String occurredAt, String producer, JsonNode data) {
}
