package com.talentflow.vacaciones.infraestructura.mensajeria;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.talentflow.vacaciones.aplicacion.PublicadorEventos;
import java.time.Clock;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.UUID;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageDeliveryMode;
import org.springframework.amqp.core.MessageProperties;
import org.springframework.amqp.rabbit.core.RabbitTemplate;

/**
 * Publica eventos en el exchange del ecosistema con el envelope del Catálogo de Eventos.
 * Usa confirmaciones del broker (publisher confirms): "publicado" significa que RabbitMQ lo
 * recibió y aceptó. Nunca lanza: si falla, lo registra y devuelve false.
 */
public class RabbitPublicadorEventos implements PublicadorEventos {

    private static final Logger log = LoggerFactory.getLogger(RabbitPublicadorEventos.class);

    private final RabbitTemplate rabbit;
    private final ObjectMapper json;
    private final String exchange;
    private final String producer;
    private final long timeoutConfirmacionMs;
    private final Clock reloj;

    public RabbitPublicadorEventos(RabbitTemplate rabbit, ObjectMapper json, String exchange, String producer,
            long timeoutConfirmacionMs, Clock reloj) {
        this.rabbit = rabbit;
        this.json = json;
        this.exchange = exchange;
        this.producer = producer;
        this.timeoutConfirmacionMs = timeoutConfirmacionMs;
        this.reloj = reloj;
    }

    @Override
    public boolean publicar(String tipo, Object data) {
        String id = UUID.randomUUID().toString();
        try {
            Map<String, Object> envelope = new LinkedHashMap<>();
            envelope.put("id", id);
            envelope.put("type", tipo);
            envelope.put("version", 1);
            envelope.put("occurredAt", Instant.now(reloj).truncatedTo(ChronoUnit.SECONDS).toString());
            envelope.put("producer", producer);
            envelope.put("data", data);

            MessageProperties propiedades = new MessageProperties();
            propiedades.setContentType(MessageProperties.CONTENT_TYPE_JSON);
            propiedades.setDeliveryMode(MessageDeliveryMode.PERSISTENT);
            propiedades.setMessageId(id);
            propiedades.setType(tipo);
            propiedades.setAppId(producer);
            Message mensaje = new Message(json.writeValueAsBytes(envelope), propiedades);

            // Routing key = tipo de evento: el exchange topic lo enruta a cada cola suscrita.
            boolean confirmado = Boolean.TRUE.equals(rabbit.invoke(operaciones -> {
                operaciones.send(exchange, tipo, mensaje);
                return operaciones.waitForConfirms(timeoutConfirmacionMs);
            }));
            if (!confirmado) {
                log.error("❌ El broker no confirmó {} (id {})", tipo, id);
                return false;
            }
            log.info("📤 Evento publicado: {} (id {})", tipo, id);
            return true;
        } catch (Exception e) {
            log.error("❌ No se pudo publicar {} (id {}): {}", tipo, id, e.getMessage());
            return false;
        }
    }
}
