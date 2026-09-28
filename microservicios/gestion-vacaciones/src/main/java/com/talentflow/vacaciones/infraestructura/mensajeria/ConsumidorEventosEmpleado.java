package com.talentflow.vacaciones.infraestructura.mensajeria;

import com.rabbitmq.client.Channel;
import com.talentflow.vacaciones.aplicacion.ProcesadorEventosEmpleado;
import com.talentflow.vacaciones.aplicacion.Resultado;
import java.io.IOException;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;

/**
 * Consume la cola vacaciones.eventos con confirmación MANUAL. Por defecto Spring AMQP
 * reencola al instante ante cualquier excepción, lo que con un mensaje que siempre falla
 * produce un bucle infinito; aquí cada resultado decide explícitamente ack o nack.
 */
public class ConsumidorEventosEmpleado {

    private final ProcesadorEventosEmpleado procesador;
    private final long esperaReintentoMs;

    public ConsumidorEventosEmpleado(ProcesadorEventosEmpleado procesador, long esperaReintentoMs) {
        this.procesador = procesador;
        this.esperaReintentoMs = esperaReintentoMs;
    }

    @RabbitListener(queues = "${vacaciones.eventos.cola}", ackMode = "MANUAL")
    public void recibir(Message mensaje, Channel canal) throws IOException, InterruptedException {
        long etiqueta = mensaje.getMessageProperties().getDeliveryTag();
        if (procesador.procesar(mensaje.getBody()) == Resultado.ERROR_TRANSITORIO) {
            Thread.sleep(esperaReintentoMs); // no reintentar en un bucle cerrado
            canal.basicNack(etiqueta, false, true);
        } else {
            // PROCESADO, DUPLICADO o DESCARTADO: el mensaje sale de la cola.
            canal.basicAck(etiqueta, false);
        }
    }
}
