package com.talentflow.notificaciones.mensajeria;

import com.talentflow.notificaciones.eventos.EventoInvalidoException;
import com.talentflow.notificaciones.servicio.ProcesadorEventos;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.amqp.AmqpRejectAndDontRequeueException;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.stereotype.Component;

/**
 * Consumidor de la cola del servicio. Política de confirmación:
 *  - procesado / duplicado / ignorado -> el método termina normal  -> ack
 *  - mensaje inválido (no tiene arreglo) -> AmqpRejectAndDontRequeueException -> reject sin reencolar
 *  - fallo transitorio (p. ej. BD caída) -> pausa y se relanza -> el broker reencola y se reintenta
 */
@Component
public class EventoListener {

    private static final Logger log = LoggerFactory.getLogger(EventoListener.class);
    private static final long PAUSA_REINTENTO_MS = 2000;

    private final ProcesadorEventos procesador;

    public EventoListener(ProcesadorEventos procesador) {
        this.procesador = procesador;
    }

    @RabbitListener(id = "notificaciones-listener", queues = "${app.broker.queue}")
    public void recibir(Message mensaje) {
        String messageId = mensaje.getMessageProperties().getMessageId();
        try {
            ProcesadorEventos.Resultado resultado = procesador.procesar(mensaje.getBody());
            log.info("Mensaje {} ({}) -> {}", messageId,
                    mensaje.getMessageProperties().getReceivedRoutingKey(), resultado);
        } catch (EventoInvalidoException e) {
            log.error("Mensaje descartado por inválido: {}", e.getMessage());
            throw new AmqpRejectAndDontRequeueException(e.getMessage(), e);
        } catch (RuntimeException e) {
            log.error("Fallo procesando el mensaje {}; se reencola", messageId, e);
            pausar();
            throw e;
        }
    }

    private static void pausar() {
        try {
            Thread.sleep(PAUSA_REINTENTO_MS);
        } catch (InterruptedException ie) {
            Thread.currentThread().interrupt();
        }
    }
}
