package com.talentflow.notificaciones.eventos;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import org.springframework.stereotype.Component;

@Component
public class EventoParser {

    private final ObjectMapper mapper;

    public EventoParser(ObjectMapper mapper) {
        this.mapper = mapper;
    }

    public Envelope parsear(byte[] cuerpo) {
        JsonNode raiz;
        try {
            raiz = mapper.readTree(cuerpo);
        } catch (IOException e) {
            throw new EventoInvalidoException("el mensaje no es JSON válido: " + e.getMessage(), e);
        }
        if (raiz == null || !raiz.isObject()) {
            throw new EventoInvalidoException("el mensaje no es un objeto JSON");
        }

        List<String> faltan = new ArrayList<>();
        if (!textoNoVacio(raiz, "id")) faltan.add("id");
        if (!textoNoVacio(raiz, "type")) faltan.add("type");
        if (!raiz.path("version").isInt() || raiz.path("version").asInt() < 1) faltan.add("version");
        if (!textoNoVacio(raiz, "occurredAt")) faltan.add("occurredAt");
        if (!textoNoVacio(raiz, "producer")) faltan.add("producer");
        if (!raiz.path("data").isObject()) faltan.add("data");
        if (!faltan.isEmpty()) {
            throw new EventoInvalidoException("faltan o son inválidos campos del envelope: " + String.join(", ", faltan));
        }

        return new Envelope(
                raiz.get("id").asText().trim(),
                raiz.get("type").asText().trim(),
                raiz.get("version").asInt(),
                raiz.get("occurredAt").asText(),
                raiz.get("producer").asText(),
                raiz.get("data"));
    }

    private static boolean textoNoVacio(JsonNode nodo, String campo) {
        JsonNode valor = nodo.get(campo);
        return valor != null && valor.isTextual() && !valor.asText().isBlank();
    }
}
