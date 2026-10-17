package com.talentflow.vacaciones.aplicacion;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.core.JsonToken;
import com.fasterxml.jackson.databind.DeserializationContext;
import com.fasterxml.jackson.databind.JsonDeserializer;
import java.io.IOException;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.time.LocalTime;
import java.time.format.DateTimeParseException;
import java.time.temporal.ChronoUnit;
import java.util.Optional;

/**
 * Interpreta las fechas de POST /vacaciones (Reto 5, D6). Se acepta fecha y hora local
 * ({@code YYYY-MM-DDTHH:mm[:ss]}, en la zona horaria del negocio) y, por compatibilidad con el
 * Reto 4, solo la fecha ({@code YYYY-MM-DD}): como inicio es el comienzo del día y como fin su
 * último segundo. Se descartan las fracciones de segundo.
 */
public final class FechaHoraEntrada {

    public static final String FORMATO = "YYYY-MM-DD o YYYY-MM-DDTHH:mm[:ss]";

    private static final LocalTime ULTIMO_SEGUNDO_DEL_DIA = LocalTime.of(23, 59, 59);

    private FechaHoraEntrada() {
    }

    public static Optional<FechaSolicitada> comoInicio(String texto) {
        return interpretar(texto, LocalTime.MIDNIGHT);
    }

    public static Optional<FechaSolicitada> comoFin(String texto) {
        return interpretar(texto, ULTIMO_SEGUNDO_DEL_DIA);
    }

    /**
     * Inicio con el que se valida y se guarda el período. Con la fecha sola, "hoy es válido" (como
     * en el Reto 4): si es hoy, empieza ahora (el máximo entre el comienzo del día y ahora). Con
     * fecha y hora explícitas se respeta lo pedido, y si ya pasó la validación lo rechaza.
     * (Con la fecha sola de hoy, el valor es las 00:00 de hoy, que nunca es posterior a ahora:
     * el máximo siempre es ahora.)
     */
    public static LocalDateTime inicioEfectivo(FechaSolicitada inicio, LocalDateTime ahora) {
        if (inicio.soloFecha() && inicio.valor().toLocalDate().equals(ahora.toLocalDate())) {
            return ahora;
        }
        return inicio.valor();
    }

    private static Optional<FechaSolicitada> interpretar(String texto, LocalTime horaSiSoloFecha) {
        if (texto == null || texto.isBlank()) {
            return Optional.empty();
        }
        try {
            if (texto.length() == 10) {
                return Optional.of(new FechaSolicitada(LocalDate.parse(texto).atTime(horaSiSoloFecha), true));
            }
            return Optional.of(new FechaSolicitada(LocalDateTime.parse(texto).truncatedTo(ChronoUnit.SECONDS), false));
        } catch (DateTimeParseException e) {
            return Optional.empty();
        }
    }

    /** Deserializador de fechaInicio. */
    public static final class Inicio extends JsonDeserializer<FechaSolicitada> {
        @Override
        public FechaSolicitada deserialize(JsonParser parser, DeserializationContext ctxt) throws IOException {
            return leer(parser, ctxt, true);
        }
    }

    /** Deserializador de fechaFin. */
    public static final class Fin extends JsonDeserializer<FechaSolicitada> {
        @Override
        public FechaSolicitada deserialize(JsonParser parser, DeserializationContext ctxt) throws IOException {
            return leer(parser, ctxt, false);
        }
    }

    private static FechaSolicitada leer(JsonParser parser, DeserializationContext ctxt, boolean esInicio)
            throws IOException {
        String texto = parser.currentToken() == JsonToken.VALUE_STRING ? parser.getText() : null;
        Optional<FechaSolicitada> valor = esInicio ? comoInicio(texto) : comoFin(texto);
        if (valor.isEmpty()) {
            // InvalidFormatException: el manejador de errores responde 400 indicando el campo.
            throw ctxt.weirdStringException(String.valueOf(texto), LocalDateTime.class, "Debe tener el formato " + FORMATO);
        }
        return valor.get();
    }
}
